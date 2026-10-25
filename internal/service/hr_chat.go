package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/ai"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// HRChatService melayani asisten HR: percakapan AI yang menjawab pertanyaan tim
// HR dari data analitik yang sudah terintegrasi di sistem.
//
// Berbeda dari mentor AI — yang terikat pada satu hasil tryout peserta dan
// menyimpan percakapan di server — asisten HR bekerja pada data kohort agregat
// dan tidak menyimpan percakapan: riwayat dikirim klien setiap permintaan lalu
// dibatasi dan disanitasi ulang di server.
type HRChatService struct {
	hr       *HRService
	aiRepo   *repository.AIRepository
	provider ai.Provider
	cfg      *config.Config
	logger   *slog.Logger
}

// NewHRChatService membentuk service asisten HR.
func NewHRChatService(
	hr *HRService,
	aiRepo *repository.AIRepository,
	provider ai.Provider,
	cfg *config.Config,
	logger *slog.Logger,
) *HRChatService {
	if logger == nil {
		logger = slog.Default()
	}
	return &HRChatService{hr: hr, aiRepo: aiRepo, provider: provider, cfg: cfg, logger: logger}
}

// promptOptions menurunkan batas prompt dari konfigurasi.
func (s *HRChatService) promptOptions() ai.PromptOptions {
	options := ai.DefaultPromptOptions()
	if s.cfg.AI.MaxTokens > 0 {
		options.MaxTokens = s.cfg.AI.MaxTokens
	}
	if s.cfg.AI.Temperature > 0 {
		options.Temperature = s.cfg.AI.Temperature
	}
	if s.cfg.AI.MaxHistoryMessages > 0 {
		options.MaxHistoryMessages = s.cfg.AI.MaxHistoryMessages
	}
	return options
}

// Chat menjawab satu pertanyaan tim HR.
func (s *HRChatService) Chat(
	ctx context.Context,
	adminID string,
	req dto.HRChatRequest,
) (*dto.HRChatResponse, error) {
	question, err := s.prepare(ctx, adminID, req)
	if err != nil {
		return nil, err
	}

	hrCtx, err := s.buildContext(ctx)
	if err != nil {
		return nil, err
	}

	request := ai.BuildHRChatRequest(hrCtx, hrHistory(req.History), question, s.promptOptions())
	response, err := s.provider.Chat(ctx, request)
	if err != nil {
		return nil, s.providerError(err)
	}

	content := strings.TrimSpace(response.Content)
	if content == "" {
		content = "Maaf, asisten tidak menghasilkan balasan. Coba ajukan pertanyaan dengan kalimat lain."
	}

	if err := s.aiRepo.IncrementUsage(ctx, adminID, time.Now().UTC(),
		response.Usage.PromptTokens, response.Usage.CompletionTokens); err != nil {
		s.logger.Warn("gagal mencatat pemakaian asisten HR", "error", err)
	}

	usage, err := s.usageView(ctx, adminID)
	if err != nil {
		return nil, err
	}

	message := dto.HRChatMessageView{
		ID:        models.NewID(),
		Role:      "assistant",
		Content:   content,
		Provider:  response.Provider,
		Model:     response.Model,
		LatencyMS: int(response.Latency.Milliseconds()),
		CreatedAt: formatTime(time.Now().UTC()),
	}

	return &dto.HRChatResponse{
		Message:     message,
		Usage:       usage,
		Degraded:    s.isDegraded(),
		Suggestions: ai.HRSuggestions(hrCtx),
	}, nil
}

// prepare memeriksa kesiapan fitur, kuota, dan membersihkan pertanyaan.
func (s *HRChatService) prepare(ctx context.Context, adminID string, req dto.HRChatRequest) (string, error) {
	if !s.cfg.AI.Enabled {
		return "", apperror.New(apperror.CodeInternal, 503,
			"Asisten HR belum diaktifkan. Hubungi tim akademik.")
	}

	question := ai.SanitizeUserInput(req.Message, 1500)
	if question == "" {
		return "", apperror.Validation("Pertanyaan tidak boleh kosong.", nil)
	}

	if err := s.ensureQuota(ctx, adminID); err != nil {
		return "", err
	}
	return question, nil
}

// buildContext memuat analitik HR terkini dan mengubahnya menjadi fakta prompt.
//
// Konteks dibangun ulang setiap permintaan supaya jawaban selalu mengikuti data
// terbaru — tidak ada angka yang di-cache antar percakapan.
func (s *HRChatService) buildContext(ctx context.Context) (ai.HRContext, error) {
	view, err := s.hr.Analytics(ctx)
	if err != nil {
		return ai.HRContext{}, err
	}
	return toHRContext(*view), nil
}

/* --------------------------------- streaming ------------------------------- */

// HRChatStream adalah aliran balasan asisten HR untuk endpoint SSE.
type HRChatStream struct {
	// Events mengalirkan potongan balasan lalu ditutup.
	Events <-chan HRChatEvent
}

// HRChatEvent adalah satu kejadian pada aliran balasan asisten HR.
type HRChatEvent struct {
	Delta    string                 `json:"delta,omitempty"`
	Done     bool                   `json:"done"`
	Message  *dto.HRChatMessageView `json:"message,omitempty"`
	Usage    *dto.AIUsageView       `json:"usage,omitempty"`
	Degraded bool                   `json:"degraded,omitempty"`
	// Suggestions adalah pertanyaan lanjutan yang relevan dengan data terkini.
	Suggestions []string `json:"suggestions,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// StreamChat menjalankan satu giliran percakapan dengan balasan bertahap.
func (s *HRChatService) StreamChat(
	ctx context.Context,
	adminID string,
	req dto.HRChatRequest,
) (*HRChatStream, error) {
	if !s.cfg.AI.StreamEnabled {
		return nil, apperror.BadRequest("Mode streaming asisten HR sedang dimatikan.")
	}

	question, err := s.prepare(ctx, adminID, req)
	if err != nil {
		return nil, err
	}

	hrCtx, err := s.buildContext(ctx)
	if err != nil {
		return nil, err
	}

	request := ai.BuildHRChatRequest(hrCtx, hrHistory(req.History), question, s.promptOptions())
	chunks, err := s.provider.Stream(ctx, request)
	if err != nil {
		return nil, s.providerError(err)
	}

	events := make(chan HRChatEvent, 16)
	go s.forwardStream(ctx, adminID, hrCtx, chunks, events)

	return &HRChatStream{Events: events}, nil
}

// forwardStream meneruskan potongan balasan lalu menutup dengan pesan lengkap.
func (s *HRChatService) forwardStream(
	ctx context.Context,
	adminID string,
	hrCtx ai.HRContext,
	chunks <-chan ai.Chunk,
	events chan<- HRChatEvent,
) {
	defer close(events)

	// Pencatatan kuota memakai context terpisah supaya pemakaian tetap tercatat
	// walau admin menutup halaman di tengah aliran balasan.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	var builder strings.Builder
	var usage ai.Usage
	degraded := false

	for chunk := range chunks {
		if chunk.Err != nil {
			s.logger.Warn("aliran balasan asisten HR terputus", "error", chunk.Err)
			events <- HRChatEvent{
				Error: streamErrorMessage(s.providerError(chunk.Err), "Asisten HR gagal menjawab. Coba lagi sebentar."),
				Done:  true,
			}
			return
		}
		if chunk.Delta != "" {
			builder.WriteString(chunk.Delta)
			events <- HRChatEvent{Delta: chunk.Delta}
		}
		if chunk.Done {
			usage = chunk.Usage
			degraded = s.isDegraded()
			break
		}
	}

	content := strings.TrimSpace(builder.String())
	if content == "" {
		content = "Maaf, asisten tidak menghasilkan balasan. Coba ajukan pertanyaan dengan kalimat lain."
	}

	if err := s.aiRepo.IncrementUsage(writeCtx, adminID, time.Now().UTC(),
		usage.PromptTokens, usage.CompletionTokens); err != nil {
		s.logger.Warn("gagal mencatat pemakaian asisten HR", "error", err)
	}

	usageView, usageErr := s.usageView(writeCtx, adminID)
	if usageErr != nil {
		s.logger.Warn("gagal memuat kuota asisten HR", "error", usageErr)
	}

	message := dto.HRChatMessageView{
		ID:        models.NewID(),
		Role:      "assistant",
		Content:   content,
		Provider:  s.provider.Name(),
		Model:     s.provider.Model(),
		CreatedAt: formatTime(time.Now().UTC()),
	}

	event := HRChatEvent{
		Done:        true,
		Message:     &message,
		Degraded:    degraded,
		Suggestions: ai.HRSuggestions(hrCtx),
	}
	if usageErr == nil {
		event.Usage = &usageView
	}
	events <- event
}

/* ---------------------------------- kuota ---------------------------------- */

// Usage mengembalikan pemakaian kuota asisten HR untuk admin yang meminta.
func (s *HRChatService) Usage(ctx context.Context, adminID string) (dto.AIUsageView, error) {
	return s.usageView(ctx, adminID)
}

// ensureQuota menolak permintaan yang melampaui kuota harian.
func (s *HRChatService) ensureQuota(ctx context.Context, adminID string) error {
	if s.cfg.AI.DailyQuota <= 0 {
		return nil
	}

	usage, err := s.aiRepo.GetUsage(ctx, adminID, time.Now().UTC())
	if err != nil {
		return err
	}
	if usage.Requests >= s.cfg.AI.DailyQuota {
		return apperror.New(apperror.CodeRateLimited, 429,
			"Kuota harian asisten AI sudah habis. Coba lagi besok.")
	}
	return nil
}

// usageView menyusun tampilan kuota harian admin.
func (s *HRChatService) usageView(ctx context.Context, adminID string) (dto.AIUsageView, error) {
	now := time.Now().UTC()
	usage, err := s.aiRepo.GetUsage(ctx, adminID, now)
	if err != nil {
		return dto.AIUsageView{}, err
	}

	remaining := -1
	if s.cfg.AI.DailyQuota > 0 {
		remaining = s.cfg.AI.DailyQuota - usage.Requests
		if remaining < 0 {
			remaining = 0
		}
	}

	return dto.AIUsageView{
		Date:             now.Format("2006-01-02"),
		Requests:         usage.Requests,
		DailyQuota:       s.cfg.AI.DailyQuota,
		Remaining:        remaining,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
	}, nil
}

// providerError mengubah kegagalan penyedia menjadi error aplikasi yang jelas.
func (s *HRChatService) providerError(err error) error {
	return providerError(s.logger, "Asisten HR", err)
}

// isDegraded menandakan jawaban berasal dari asisten heuristik lokal.
func (s *HRChatService) isDegraded() bool {
	return isDegradedProvider(s.provider)
}

/* --------------------------------- pemetaan -------------------------------- */

// hrHistory mengubah riwayat dari klien menjadi pesan model.
//
// Isi pesan disanitasi ulang di server: klien tidak boleh bisa menyisipkan
// penanda internal atau membanjiri konteks.
func hrHistory(items []dto.HRChatHistoryItem) []ai.Message {
	out := make([]ai.Message, 0, len(items))
	for _, item := range items {
		content := ai.SanitizeUserInput(item.Content, 2000)
		if strings.TrimSpace(content) == "" {
			continue
		}

		role := ai.RoleUser
		if item.Role == "assistant" {
			role = ai.RoleAssistant
		}
		out = append(out, ai.Message{Role: role, Content: content})
	}
	return out
}

// toHRContext mengubah analitik HR menjadi fakta agregat untuk prompt.
func toHRContext(view dto.HRAnalyticsView) ai.HRContext {
	ctx := ai.HRContext{
		GeneratedAt:       view.GeneratedAt,
		TotalPeserta:      view.TotalPeserta,
		ActivePeserta:     view.ActivePeserta,
		InactivePeserta:   view.InactivePeserta,
		ParticipationRate: view.ParticipationRate,
		ActiveWindowDays:  view.ActiveWindowDays,
		TotalAttempts:     view.TotalAttempts,
		AverageScore:      view.AverageScore,
		TopScore:          view.TopScore,
		PassingThreshold:  view.PassingThreshold,
		PassingRate:       view.PassingRate,
		PracticeHours:     view.PracticeHours,
		DistinctTryouts:   view.DistinctTryouts,
		TotalLessons:      view.TotalLessons,
		CompletedLessons:  view.CompletedLessons,
		ActiveLearners:    view.ActiveLearners,
		AverageProgress:   view.AverageProgress,
		CompletionRate:    view.CompletionRate,
		ScoreDistribution: make([]ai.HRBucket, 0, len(view.ScoreDistribution)),
		SkillProfile:      make([]ai.HRSkill, 0, len(view.SkillProfile)),
		MonthlyTrend:      make([]ai.HRMonth, 0, len(view.MonthlyTrend)),
		ProgramBreakdown:  make([]ai.HRProgram, 0, len(view.ProgramBreakdown)),
		TopLearners:       make([]ai.HRLearner, 0, len(view.TopLearners)),
		AtRiskLearners:    make([]ai.HRLearner, 0, len(view.AtRiskLearners)),
	}

	for _, bucket := range view.ScoreDistribution {
		ctx.ScoreDistribution = append(ctx.ScoreDistribution, ai.HRBucket{Label: bucket.Label, Count: bucket.Count})
	}
	for _, skill := range view.SkillProfile {
		ctx.SkillProfile = append(ctx.SkillProfile, ai.HRSkill{
			Name: skill.Name, AverageScore: skill.AverageScore, Attempts: skill.Attempts,
		})
	}
	for _, month := range view.MonthlyTrend {
		ctx.MonthlyTrend = append(ctx.MonthlyTrend, ai.HRMonth{
			Label: month.Label, Attempts: month.Attempts, AverageScore: month.AverageScore,
		})
	}
	for _, program := range view.ProgramBreakdown {
		ctx.ProgramBreakdown = append(ctx.ProgramBreakdown, ai.HRProgram{
			Program: program.Program, Peserta: program.Peserta,
			Attempts: program.Attempts, AverageScore: program.AverageScore,
		})
	}
	for _, learner := range view.TopLearners {
		ctx.TopLearners = append(ctx.TopLearners, ai.HRLearner{
			Name: learner.Name, Program: learner.Program, AttemptCount: learner.AttemptCount,
			Score: learner.BestScore, HasScore: true, Activity: learner.LastActivity, Note: learner.Note,
		})
	}
	for _, learner := range view.AtRiskLearners {
		item := ai.HRLearner{
			Name: learner.Name, Program: learner.Program, AttemptCount: learner.AttemptCount,
			Activity: learner.LastActivity, Note: learner.Reason,
		}
		if learner.LastScore != nil {
			item.Score = *learner.LastScore
			item.HasScore = true
		}
		ctx.AtRiskLearners = append(ctx.AtRiskLearners, item)
	}

	return ctx
}
