// Package service — mentor AI ("MinSmart") untuk ulasan hasil tryout.
//
// Service ini menggabungkan tiga hal:
//   - fakta hasil tryout peserta (repository, hanya milik pemilik attempt),
//   - penyedia model bahasa (paket `ai`, termasuk mentor heuristik lokal),
//   - pagar pengaman: kuota harian, batas konteks, dan riwayat percakapan.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/ai"
	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// errNoJSONObject menandai balasan model yang tidak memuat objek JSON.
var errNoJSONObject = errors.New("service: balasan model tidak memuat objek JSON")

// MentorService melayani analisis AI dan percakapan ulasan hasil tryout.
type MentorService struct {
	attempts *repository.AttemptRepository
	sessions *repository.SessionRepository
	aiRepo   *repository.AIRepository
	tryouts  *TryoutService
	provider ai.Provider
	cache    *cache.Cache
	cfg      *config.Config
	logger   *slog.Logger
}

// NewMentorService membentuk service mentor AI.
func NewMentorService(
	attempts *repository.AttemptRepository,
	sessions *repository.SessionRepository,
	aiRepo *repository.AIRepository,
	tryouts *TryoutService,
	provider ai.Provider,
	cacheLayer *cache.Cache,
	cfg *config.Config,
	logger *slog.Logger,
) *MentorService {
	if logger == nil {
		logger = slog.Default()
	}
	return &MentorService{
		attempts: attempts,
		sessions: sessions,
		aiRepo:   aiRepo,
		tryouts:  tryouts,
		provider: provider,
		cache:    cacheLayer,
		cfg:      cfg,
		logger:   logger,
	}
}

// mentorContext menggabungkan attempt asli dengan konteks yang dikirim ke model.
type mentorContext struct {
	Attempt models.Attempt
	Package models.TryoutPackage
	Session *models.TryoutSession
	Context ai.AttemptContext
}

// promptOptions menurunkan batas prompt dari konfigurasi.
func (s *MentorService) promptOptions() ai.PromptOptions {
	options := ai.DefaultPromptOptions()
	if s.cfg.AI.MaxTokens > 0 {
		options.MaxTokens = s.cfg.AI.MaxTokens
	}
	if s.cfg.AI.Temperature > 0 {
		options.Temperature = s.cfg.AI.Temperature
	}
	if s.cfg.AI.MaxContextQuestions > 0 {
		options.MaxContextQuestions = s.cfg.AI.MaxContextQuestions
	}
	if s.cfg.AI.MaxHistoryMessages > 0 {
		options.MaxHistoryMessages = s.cfg.AI.MaxHistoryMessages
	}
	return options
}

// loadContext memuat hasil tryout beserta seluruh fakta yang boleh diketahui
// mentor. Kepemilikan attempt diperiksa di sini sehingga tidak ada endpoint
// yang bisa membaca hasil tryout peserta lain.
func (s *MentorService) loadContext(ctx context.Context, userID, attemptID string) (*mentorContext, error) {
	if strings.TrimSpace(attemptID) == "" {
		return nil, apperror.BadRequest("ID hasil tryout wajib disertakan.")
	}

	attempt, err := s.attempts.FindByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, apperror.Forbidden("Hasil tryout ini bukan milik akun Anda.")
	}

	pkg, err := s.tryouts.GetPackage(ctx, attempt.TryoutID)
	if err != nil {
		return nil, err
	}

	session, err := s.sessions.FindByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

	questions, err := s.tryouts.QuestionsForPackage(ctx, pkg)
	if err != nil {
		return nil, err
	}

	answers := map[string]models.TryoutAnswer{}
	if session != nil {
		rows, err := s.sessions.ListAnswers(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			answers[row.QuestionID] = row
		}
	}

	return &mentorContext{
		Attempt: *attempt,
		Package: *pkg,
		Session: session,
		Context: buildAttemptContext(*attempt, *pkg, session, questions, answers, s.cfg.Business.PassingThreshold),
	}, nil
}

// buildAttemptContext menyusun konteks yang dikirim ke model.
//
// Urutan soal mengikuti snapshot sesi (`question_order`) agar nomor soal pada
// ulasan sama dengan yang dilihat peserta saat mengerjakan.
func buildAttemptContext(
	attempt models.Attempt,
	pkg models.TryoutPackage,
	session *models.TryoutSession,
	questions map[string]models.Question,
	answers map[string]models.TryoutAnswer,
	passingThreshold int,
) ai.AttemptContext {
	out := ai.AttemptContext{
		Title:            attempt.Title,
		TryoutID:         attempt.TryoutID,
		TakenAt:          formatTime(attempt.TakenAt),
		Score:            attempt.Score,
		Correct:          attempt.Correct,
		Total:            attempt.Total,
		PassingThreshold: passingThreshold,
		Status:           string(attempt.Status),
		DurationSeconds:  attempt.DurationSeconds,
		Subtests:         make([]ai.SubtestContext, 0, len(pkg.Subtests)),
		Questions:        make([]ai.QuestionContext, 0, attempt.Total),
	}

	// Ringkasan subtes diambil dari skor tersimpan (attempt_categories) supaya
	// cocok dengan yang ditampilkan di halaman hasil.
	for _, category := range attempt.Categories {
		out.Subtests = append(out.Subtests, ai.SubtestContext{
			Name:    category.Name,
			Score:   category.Score,
			Correct: category.Correct,
			Total:   category.Total,
		})
	}
	// Lengkapi subtes yang belum punya kategori (mis. attempt lama).
	for _, subtest := range pkg.Subtests {
		if !hasSubtest(out.Subtests, subtest.Name) {
			out.Subtests = append(out.Subtests, ai.SubtestContext{Name: subtest.Name})
		}
	}

	number := 0
	for subtestIndex, ids := range questionOrder(pkg, session) {
		subtestName := ""
		if subtestIndex < len(pkg.Subtests) {
			subtestName = pkg.Subtests[subtestIndex].Name
		}

		for _, questionID := range ids {
			question, ok := questions[questionID]
			if !ok {
				continue
			}
			number++

			answer, hasAnswer := answers[questionID]
			var selected *int
			flagged := false
			if hasAnswer {
				selected = answer.SelectedIndex
				flagged = answer.Flagged
			}

			out.Questions = append(out.Questions, ai.QuestionContext{
				ID:            question.ID,
				Number:        number,
				Subtest:       subtestName,
				BankLabel:     bankLabelOf(question.BankKey),
				Text:          question.Text,
				Options:       emptyIfNil(question.Options),
				CorrectIndex:  question.AnswerIndex,
				SelectedIndex: selected,
				IsCorrect:     selected != nil && *selected == question.AnswerIndex,
				IsUnanswered:  selected == nil,
				Flagged:       flagged,
				Difficulty:    string(question.Difficulty),
				Explanation:   question.Explanation,
			})
		}
	}

	return out
}

// questionOrder menentukan urutan id soal per subtes.
//
// Snapshot sesi diprioritaskan; bila sesi sudah dibersihkan, seluruh soal paket
// disusun ulang berdasarkan urutan bank agar nomor soal tetap konsisten.
func questionOrder(pkg models.TryoutPackage, session *models.TryoutSession) [][]string {
	if session != nil && len(session.QuestionOrder) > 0 {
		return session.QuestionOrder
	}

	order := make([][]string, len(pkg.Subtests))
	for index := range pkg.Subtests {
		order[index] = []string{}
	}
	return order
}

func hasSubtest(subtests []ai.SubtestContext, name string) bool {
	for _, subtest := range subtests {
		if subtest.Name == name {
			return true
		}
	}
	return false
}

/* --------------------------------- analisis -------------------------------- */

// Analysis mengembalikan analisis hasil tryout.
//
// Analisis dihitung sekali lalu disimpan: attempt bersifat final sehingga
// hasilnya tidak perlu dihitung ulang setiap halaman dibuka. Gunakan
// `force=true` untuk menghitung ulang (mis. setelah model diganti).
func (s *MentorService) Analysis(ctx context.Context, userID, attemptID string, force bool) (*dto.AIAnalysisView, error) {
	mc, err := s.loadContext(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	if !force {
		cached, err := s.aiRepo.FindAnalysis(ctx, mc.Attempt.ID)
		if err != nil {
			return nil, err
		}
		if cached != nil && s.cacheStillValid(cached) {
			view := s.toAnalysisView(*cached, mc.Context)
			view.Cached = true
			return &view, nil
		}
	}

	result, provider, model, degraded := s.generateAnalysis(ctx, mc.Context)

	analysis := &models.AttemptAnalysis{
		AttemptID:      mc.Attempt.ID,
		UserID:         mc.Attempt.UserID,
		Provider:       provider,
		Model:          model,
		Headline:       result.Headline,
		Summary:        result.Summary,
		Readiness:      models.AIReadiness(ai.Readiness(mc.Context)),
		ReadinessScore: ai.ReadinessScore(mc.Context),
		Strengths:      toNotes(result.Strengths),
		Weaknesses:     toNotes(result.Weaknesses),
		FocusAreas:     toNotes(result.FocusAreas),
		StudyPlan:      toNotes(result.StudyPlan),
		QuestionNotes:  toNotes(result.QuestionNotes),
		GeneratedAt:    time.Now().UTC(),
	}

	if err := s.aiRepo.SaveAnalysis(ctx, analysis); err != nil {
		// Analisis tetap bisa dikembalikan walau penyimpanan gagal.
		s.logger.Warn("gagal menyimpan analisis mentor AI", "error", err, "attempt_id", mc.Attempt.ID)
	}

	view := s.toAnalysisView(*analysis, mc.Context)
	view.Degraded = degraded
	return &view, nil
}

// generateAnalysis menghasilkan analisis dari baseline deterministik, lalu
// mencoba memperkayanya dengan model bahasa.
//
// Baseline selalu tersedia, sehingga kegagalan model tidak pernah membuat
// halaman hasil tryout kosong.
func (s *MentorService) generateAnalysis(
	ctx context.Context,
	attemptCtx ai.AttemptContext,
) (ai.AnalysisResult, string, string, bool) {
	baseline := ai.BaselineAnalysis(attemptCtx)

	if !s.cfg.AI.Enabled {
		return baseline, "disabled", "", false
	}

	response, err := s.provider.Chat(ctx, ai.BuildAnalysisRequest(attemptCtx, s.promptOptions()))
	if err != nil {
		s.logger.Warn("mentor AI gagal membuat analisis, memakai baseline lokal",
			"error", err, "provider", s.provider.Name())
		return baseline, s.provider.Name(), s.provider.Model(), true
	}

	parsed, err := parseAnalysisJSON(response.Content)
	if err != nil {
		s.logger.Warn("balasan analisis mentor AI tidak bisa diurai, memakai baseline lokal",
			"error", err, "provider", response.Provider)
		return baseline, response.Provider, response.Model, true
	}

	return sanitizeAnalysis(parsed, baseline, attemptCtx), response.Provider, response.Model, false
}

// parseAnalysisJSON mengurai balasan model menjadi AnalysisResult.
//
// Model kadang membungkus JSON dengan pagar markdown atau menambahkan kalimat
// pembuka; karena itu objek JSON diambil dari kurung kurawal pertama sampai
// terakhir sebelum diurai.
func parseAnalysisJSON(raw string) (ai.AnalysisResult, error) {
	content := strings.TrimSpace(raw)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return ai.AnalysisResult{}, errNoJSONObject
	}

	var result ai.AnalysisResult
	if err := json.Unmarshal([]byte(content[start:end+1]), &result); err != nil {
		return ai.AnalysisResult{}, err
	}
	return result, nil
}

// sanitizeAnalysis memastikan hasil model tetap masuk akal.
//
// Butir kosong atau yang menunjuk soal di luar attempt dibuang, lalu bagian yang
// jadi kosong diisi baseline agar analisis selalu lengkap.
func sanitizeAnalysis(parsed, baseline ai.AnalysisResult, attemptCtx ai.AttemptContext) ai.AnalysisResult {
	knownQuestion := make(map[string]bool, len(attemptCtx.Questions))
	for _, question := range attemptCtx.Questions {
		knownQuestion[question.ID] = true
	}

	out := ai.AnalysisResult{
		Headline:      clampText(parsed.Headline, 200),
		Summary:       clampText(parsed.Summary, 2000),
		Strengths:     cleanNotes(parsed.Strengths, 4, knownQuestion),
		Weaknesses:    cleanNotes(parsed.Weaknesses, 4, knownQuestion),
		FocusAreas:    cleanNotes(parsed.FocusAreas, 4, knownQuestion),
		StudyPlan:     cleanNotes(parsed.StudyPlan, 6, knownQuestion),
		QuestionNotes: cleanNotes(parsed.QuestionNotes, 8, knownQuestion),
	}

	if out.Headline == "" {
		out.Headline = baseline.Headline
	}
	if out.Summary == "" {
		out.Summary = baseline.Summary
	}
	if len(out.Strengths) == 0 {
		out.Strengths = baseline.Strengths
	}
	if len(out.Weaknesses) == 0 {
		out.Weaknesses = baseline.Weaknesses
	}
	if len(out.FocusAreas) == 0 {
		out.FocusAreas = baseline.FocusAreas
	}
	if len(out.StudyPlan) == 0 {
		out.StudyPlan = baseline.StudyPlan
	}
	if len(out.QuestionNotes) == 0 {
		out.QuestionNotes = baseline.QuestionNotes
	}

	return out
}

// cleanNotes menyaring butir analisis yang tidak layak ditampilkan.
func cleanNotes(notes []ai.Note, limit int, knownQuestion map[string]bool) []ai.Note {
	out := make([]ai.Note, 0, len(notes))
	for _, note := range notes {
		title := strings.TrimSpace(note.Title)
		detail := strings.TrimSpace(note.Detail)
		if title == "" || detail == "" {
			continue
		}
		// Catatan yang menunjuk soal di luar attempt ini dibuang: itu tanda
		// model mengarang soal yang tidak pernah dikerjakan peserta.
		if note.QuestionID != "" && !knownQuestion[note.QuestionID] {
			continue
		}
		note.Title = clampText(title, 160)
		note.Detail = clampText(detail, 900)
		out = append(out, note)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func clampText(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if max > 0 && len(trimmed) > max {
		return trimmed[:max]
	}
	return trimmed
}

func (s *MentorService) cacheStillValid(analysis *models.AttemptAnalysis) bool {
	if s.cfg.AI.CacheTTL <= 0 {
		return true
	}
	return time.Since(analysis.GeneratedAt) < s.cfg.AI.CacheTTL
}

func (s *MentorService) toAnalysisView(analysis models.AttemptAnalysis, attemptCtx ai.AttemptContext) dto.AIAnalysisView {
	return dto.AIAnalysisView{
		AttemptID:        analysis.AttemptID,
		Headline:         analysis.Headline,
		Summary:          analysis.Summary,
		Readiness:        string(analysis.Readiness),
		ReadinessScore:   analysis.ReadinessScore,
		PassingThreshold: attemptCtx.PassingThreshold,
		Strengths:        fromNotes(analysis.Strengths),
		Weaknesses:       fromNotes(analysis.Weaknesses),
		FocusAreas:       fromNotes(analysis.FocusAreas),
		StudyPlan:        fromNotes(analysis.StudyPlan),
		QuestionNotes:    fromNotes(analysis.QuestionNotes),
		GeneratedAt:      formatTime(analysis.GeneratedAt),
		Provider:         analysis.Provider,
		Model:            analysis.Model,
		Suggestions:      ai.Suggestions(attemptCtx, nil),
	}
}

/* -------------------------------- percakapan ------------------------------- */

// Conversation mengembalikan riwayat percakapan mentor untuk satu hasil tryout.
func (s *MentorService) Conversation(ctx context.Context, userID, attemptID string) (*dto.AIConversationView, error) {
	mc, err := s.loadContext(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	conversation, err := s.aiRepo.FindConversation(ctx, mc.Attempt.ID)
	if err != nil {
		return nil, err
	}

	view := &dto.AIConversationView{
		AttemptID:   mc.Attempt.ID,
		Title:       conversationTitle(mc.Attempt.Title),
		Messages:    []dto.AIMessageView{},
		Suggestions: ai.Suggestions(mc.Context, nil),
	}
	if conversation != nil {
		view.Title = conversation.Title
		view.MessageCount = conversation.MessageCount

		messages, err := s.aiRepo.ListMessages(ctx, conversation.ID, 0)
		if err != nil {
			return nil, err
		}
		view.Messages = toMessageViews(messages)
	}

	usage, err := s.usageView(ctx, userID)
	if err != nil {
		return nil, err
	}
	view.Usage = usage

	return view, nil
}

// ResetConversation mengosongkan riwayat percakapan mentor.
func (s *MentorService) ResetConversation(ctx context.Context, userID, attemptID string) error {
	mc, err := s.loadContext(ctx, userID, attemptID)
	if err != nil {
		return err
	}

	conversation, err := s.aiRepo.FindConversation(ctx, mc.Attempt.ID)
	if err != nil {
		return err
	}
	if conversation == nil {
		return nil
	}
	return s.aiRepo.ResetConversation(ctx, conversation.ID)
}

func conversationTitle(attemptTitle string) string {
	if strings.TrimSpace(attemptTitle) == "" {
		return "Ulasan Mentor AI"
	}
	return "Ulasan " + attemptTitle
}

/* ---------------------------------- chat ---------------------------------- */

// Chat menjawab pertanyaan peserta tentang hasil tryout.
func (s *MentorService) Chat(ctx context.Context, userID, attemptID string, req dto.AIChatRequest) (*dto.AIChatResponse, error) {
	mc, err := s.loadContext(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	focus, err := resolveFocus(mc.Context, req.QuestionID)
	if err != nil {
		return nil, err
	}

	question := ai.SanitizeUserInput(req.Message, 1500)
	if question == "" {
		return nil, apperror.Validation("Pertanyaan tidak boleh kosong.", nil)
	}

	return s.runTurn(ctx, mc, chatTurn{
		display:    question,
		focus:      focus,
		useHistory: true,
	})
}

// ExplainQuestion meminta mentor menjelaskan satu soal secara bertahap.
func (s *MentorService) ExplainQuestion(
	ctx context.Context,
	userID, attemptID, questionID string,
) (*dto.AIChatResponse, error) {
	mc, err := s.loadContext(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	focus, err := resolveFocus(mc.Context, questionID)
	if err != nil {
		return nil, err
	}
	if focus == nil {
		return nil, apperror.BadRequest("Pilih soal yang ingin dijelaskan terlebih dahulu.")
	}

	return s.runTurn(ctx, mc, chatTurn{
		display: fmt.Sprintf("%s Jelaskan soal nomor %d (%s).",
			ai.AutoPromptMarker(), focus.Number, focus.Subtest),
		focus:   focus,
		request: ai.BuildQuestionExplainRequest(mc.Context, *focus, s.promptOptions()),
	})
}

// chatTurn menggambarkan satu giliran percakapan.
type chatTurn struct {
	// display adalah teks yang disimpan sebagai pesan peserta.
	display string
	focus   *ai.QuestionContext
	// request boleh kosong; bila kosong, dibangun dari riwayat percakapan.
	request    ai.Request
	useHistory bool
}

// runTurn menjalankan satu giliran percakapan: simpan pesan peserta, panggil
// model, simpan balasan, lalu perbarui kuota.
func (s *MentorService) runTurn(ctx context.Context, mc *mentorContext, turn chatTurn) (*dto.AIChatResponse, error) {
	if err := s.ensureQuota(ctx, mc.Attempt.UserID); err != nil {
		return nil, err
	}

	conversation, err := s.aiRepo.EnsureConversation(
		ctx, mc.Attempt.ID, mc.Attempt.UserID, conversationTitle(mc.Attempt.Title),
	)
	if err != nil {
		return nil, err
	}

	request := turn.request
	if turn.useHistory || request.Messages == nil {
		history, err := s.historyForModel(ctx, conversation.ID, s.promptOptions().MaxHistoryMessages)
		if err != nil {
			return nil, err
		}
		// Bangun ulang request dengan riwayat agar percakapan berkesinambungan.
		request = ai.BuildChatRequest(mc.Context, turn.focus, history, turn.display, s.promptOptions())
	}

	userMessage := newMessage(conversation.ID, mc.Attempt.UserID, models.AIRoleUser, turn.display, turn.focus)
	if err := s.aiRepo.AppendMessages(ctx, conversation.ID, []*models.AIMessage{&userMessage}); err != nil {
		return nil, err
	}

	response, err := s.provider.Chat(ctx, request)
	if err != nil {
		return nil, s.providerError(err)
	}

	content := strings.TrimSpace(response.Content)
	if content == "" {
		content = "Maaf, mentor tidak menghasilkan balasan. Coba ajukan pertanyaan dengan kalimat lain."
	}

	assistantMessage := newMessage(conversation.ID, mc.Attempt.UserID, models.AIRoleAssistant, content, turn.focus)
	assistantMessage.Provider = response.Provider
	assistantMessage.Model = response.Model
	assistantMessage.PromptTokens = response.Usage.PromptTokens
	assistantMessage.CompletionTokens = response.Usage.CompletionTokens
	assistantMessage.LatencyMS = int(response.Latency.Milliseconds())

	if err := s.aiRepo.AppendMessages(ctx, conversation.ID, []*models.AIMessage{&assistantMessage}); err != nil {
		return nil, err
	}
	if err := s.aiRepo.IncrementUsage(ctx, mc.Attempt.UserID, time.Now().UTC(),
		response.Usage.PromptTokens, response.Usage.CompletionTokens); err != nil {
		s.logger.Warn("gagal mencatat pemakaian mentor AI", "error", err)
	}

	usage, err := s.usageView(ctx, mc.Attempt.UserID)
	if err != nil {
		return nil, err
	}

	return &dto.AIChatResponse{
		Message:     toMessageView(assistantMessage),
		Usage:       usage,
		Degraded:    s.isDegraded(),
		Suggestions: ai.Suggestions(mc.Context, turn.focus),
	}, nil
}

/* --------------------------------- streaming ------------------------------- */

// MentorStream adalah aliran balasan mentor untuk endpoint SSE.
type MentorStream struct {
	// UserMessage adalah pesan peserta yang sudah tersimpan.
	UserMessage dto.AIMessageView
	// Events mengalirkan potongan balasan lalu ditutup.
	Events <-chan MentorEvent
}

// MentorEvent adalah satu kejadian pada aliran balasan mentor.
type MentorEvent struct {
	Delta    string             `json:"delta,omitempty"`
	Done     bool               `json:"done"`
	Message  *dto.AIMessageView `json:"message,omitempty"`
	Usage    *dto.AIUsageView   `json:"usage,omitempty"`
	Degraded bool               `json:"degraded,omitempty"`
	Error    string             `json:"error,omitempty"`
}

// StreamChat menjalankan satu giliran percakapan dengan balasan bertahap.
//
// Balasan tetap disimpan walau koneksi klien terputus di tengah jalan, sehingga
// riwayat percakapan tidak kehilangan jawaban yang sudah dihasilkan model.
func (s *MentorService) StreamChat(
	ctx context.Context,
	userID, attemptID string,
	req dto.AIChatRequest,
) (*MentorStream, error) {
	if !s.cfg.AI.StreamEnabled {
		return nil, apperror.BadRequest("Mode streaming mentor AI sedang dimatikan.")
	}

	mc, err := s.loadContext(ctx, userID, attemptID)
	if err != nil {
		return nil, err
	}

	focus, err := resolveFocus(mc.Context, req.QuestionID)
	if err != nil {
		return nil, err
	}

	question := ai.SanitizeUserInput(req.Message, 1500)
	if question == "" {
		return nil, apperror.Validation("Pertanyaan tidak boleh kosong.", nil)
	}

	if err := s.ensureQuota(ctx, mc.Attempt.UserID); err != nil {
		return nil, err
	}

	conversation, err := s.aiRepo.EnsureConversation(
		ctx, mc.Attempt.ID, mc.Attempt.UserID, conversationTitle(mc.Attempt.Title),
	)
	if err != nil {
		return nil, err
	}

	history, err := s.historyForModel(ctx, conversation.ID, s.promptOptions().MaxHistoryMessages)
	if err != nil {
		return nil, err
	}
	request := ai.BuildChatRequest(mc.Context, focus, history, question, s.promptOptions())

	userMessage := newMessage(conversation.ID, mc.Attempt.UserID, models.AIRoleUser, question, focus)
	if err := s.aiRepo.AppendMessages(ctx, conversation.ID, []*models.AIMessage{&userMessage}); err != nil {
		return nil, err
	}

	chunks, err := s.provider.Stream(ctx, request)
	if err != nil {
		return nil, s.providerError(err)
	}

	events := make(chan MentorEvent, 16)
	go s.forwardStream(ctx, streamJob{
		conversationID: conversation.ID,
		userID:         mc.Attempt.UserID,
		focus:          focus,
		chunks:         chunks,
		events:         events,
	})

	return &MentorStream{
		UserMessage: toMessageView(userMessage),
		Events:      events,
	}, nil
}

// streamJob mengumpulkan parameter penerusan aliran balasan.
type streamJob struct {
	conversationID string
	userID         string
	focus          *ai.QuestionContext
	chunks         <-chan ai.Chunk
	events         chan<- MentorEvent
}

// forwardStream meneruskan potongan balasan dan menyimpan hasil akhirnya.
func (s *MentorService) forwardStream(ctx context.Context, job streamJob) {
	defer close(job.events)

	// Penulisan hasil akhir memakai context terpisah supaya jawaban yang sudah
	// dihasilkan model tidak hilang saat klien menutup koneksi lebih dulu.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	var builder strings.Builder
	var usage ai.Usage
	degraded := false

	for chunk := range job.chunks {
		if chunk.Err != nil {
			s.logger.Warn("aliran balasan mentor terputus", "error", chunk.Err)
			s.persistStreamedMessage(writeCtx, job, builder.String(), usage, true)
			job.events <- MentorEvent{Error: s.streamErrorMessage(chunk.Err), Done: true}
			return
		}
		if chunk.Delta != "" {
			builder.WriteString(chunk.Delta)
			job.events <- MentorEvent{Delta: chunk.Delta}
		}
		if chunk.Done {
			usage = chunk.Usage
			degraded = s.isDegraded()
			break
		}
	}

	content := strings.TrimSpace(builder.String())
	if content == "" {
		content = "Maaf, mentor tidak menghasilkan balasan. Coba ajukan pertanyaan dengan kalimat lain."
	}

	message := s.persistStreamedMessage(writeCtx, job, content, usage, false)
	usageView, err := s.usageView(writeCtx, job.userID)
	if err != nil {
		s.logger.Warn("gagal memuat kuota mentor AI", "error", err)
	}

	event := MentorEvent{
		Done:     true,
		Message:  message,
		Degraded: degraded,
	}
	if err == nil {
		event.Usage = &usageView
	}
	job.events <- event
}

// persistStreamedMessage menyimpan balasan mentor yang sudah lengkap.
func (s *MentorService) persistStreamedMessage(
	ctx context.Context,
	job streamJob,
	content string,
	usage ai.Usage,
	failed bool,
) *dto.AIMessageView {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	message := newMessage(job.conversationID, job.userID, models.AIRoleAssistant, content, job.focus)
	message.Provider = s.provider.Name()
	message.Model = s.provider.Model()
	message.PromptTokens = usage.PromptTokens
	message.CompletionTokens = usage.CompletionTokens

	if err := s.aiRepo.AppendMessages(ctx, job.conversationID, []*models.AIMessage{&message}); err != nil {
		s.logger.Warn("gagal menyimpan balasan streaming mentor", "error", err)
		return nil
	}
	if !failed {
		if err := s.aiRepo.IncrementUsage(ctx, job.userID, time.Now().UTC(),
			usage.PromptTokens, usage.CompletionTokens); err != nil {
			s.logger.Warn("gagal mencatat pemakaian mentor AI", "error", err)
		}
	}

	view := toMessageView(message)
	return &view
}

func (s *MentorService) streamErrorMessage(err error) string {
	appErr := s.providerError(err)
	if typed, ok := appErr.(*apperror.Error); ok {
		return typed.Message
	}
	return "Mentor AI gagal menjawab. Coba lagi sebentar."
}

/* ---------------------------------- kuota --------------------------------- */

// ensureQuota menolak permintaan yang melampaui kuota harian peserta.
//
// Kuota menjaga biaya tetap terkendali sekaligus mencegah penyalahgunaan.
func (s *MentorService) ensureQuota(ctx context.Context, userID string) error {
	if s.cfg.AI.DailyQuota <= 0 {
		return nil
	}

	usage, err := s.aiRepo.GetUsage(ctx, userID, time.Now().UTC())
	if err != nil {
		return err
	}
	if usage.Requests >= s.cfg.AI.DailyQuota {
		return apperror.New(apperror.CodeRateLimited, 429,
			fmt.Sprintf("Kuota mentor AI harianmu sudah habis (%d permintaan). Coba lagi besok.", s.cfg.AI.DailyQuota))
	}
	return nil
}

// Usage mengembalikan pemakaian kuota mentor peserta.
func (s *MentorService) Usage(ctx context.Context, userID string) (dto.AIUsageView, error) {
	return s.usageView(ctx, userID)
}

func (s *MentorService) usageView(ctx context.Context, userID string) (dto.AIUsageView, error) {
	now := time.Now().UTC()
	usage, err := s.aiRepo.GetUsage(ctx, userID, now)
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

// Status merangkum kesiapan fitur mentor AI.
func (s *MentorService) Status() dto.AIStatusView {
	status := dto.AIStatusView{
		Enabled:          s.cfg.AI.Enabled,
		Provider:         s.provider.Name(),
		Model:            s.provider.Model(),
		StreamingEnabled: s.cfg.AI.StreamEnabled,
		DailyQuota:       s.cfg.AI.DailyQuota,
	}
	if fallback, ok := s.provider.(*ai.FallbackProvider); ok {
		status.Degraded = fallback.Degraded()
		status.LastError = fallback.LastError()
	}
	return status
}

/* ---------------------------------- helper --------------------------------- */

// resolveFocus memvalidasi soal yang diminta peserta.
func resolveFocus(attemptCtx ai.AttemptContext, questionID string) (*ai.QuestionContext, error) {
	id := strings.TrimSpace(questionID)
	if id == "" {
		return nil, nil
	}

	question, ok := attemptCtx.QuestionByID(id)
	if !ok {
		return nil, apperror.BadRequest("Soal tersebut tidak ada pada hasil tryout ini.")
	}
	return &question, nil
}

// historyForModel mengambil riwayat percakapan dalam bentuk pesan model.
func (s *MentorService) historyForModel(ctx context.Context, conversationID string, limit int) ([]ai.Message, error) {
	if limit <= 0 {
		limit = 12
	}
	messages, err := s.aiRepo.ListMessages(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}

	out := make([]ai.Message, 0, len(messages))
	for _, message := range messages {
		role := ai.RoleAssistant
		if message.Role == models.AIRoleUser {
			role = ai.RoleUser
		}
		out = append(out, ai.Message{Role: role, Content: message.Content})
	}
	return out, nil
}

// newMessage membentuk model pesan percakapan.
func newMessage(
	conversationID, userID string,
	role models.AIRole,
	content string,
	focus *ai.QuestionContext,
) models.AIMessage {
	message := models.AIMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           role,
		Content:        content,
		Scope:          models.AIScopeAttempt,
	}
	if focus != nil {
		questionID := focus.ID
		number := focus.Number
		subtest := focus.Subtest
		message.Scope = models.AIScopeQuestion
		message.QuestionID = &questionID
		message.QuestionNumber = &number
		message.SubtestName = &subtest
	}
	return message
}

// providerError mengubah kegagalan penyedia menjadi error aplikasi yang jelas.
func (s *MentorService) providerError(err error) error {
	var upstream *ai.UpstreamError
	if !errors.As(err, &upstream) {
		return apperror.Internal(err, "Mentor AI sedang tidak bisa dihubungi. Coba lagi sebentar.")
	}

	if upstream.IsConfiguration() {
		s.logger.Error("konfigurasi mentor AI bermasalah",
			"provider", upstream.Provider, "code", upstream.Code, "status", upstream.StatusCode)
		return apperror.New(apperror.CodeInternal, 503,
			"Mentor AI belum siap dipakai (konfigurasi model). Hubungi tim akademik.")
	}
	if upstream.IsRetryable() {
		return apperror.New(apperror.CodeInternal, 503, "Mentor AI sedang sibuk. Coba lagi beberapa saat.")
	}
	return apperror.Internal(err, "Mentor AI gagal menjawab. Coba lagi sebentar.")
}

func (s *MentorService) isDegraded() bool {
	if fallback, ok := s.provider.(*ai.FallbackProvider); ok {
		return fallback.Degraded()
	}
	return false
}

/* ------------------------------ pemetaan tipe ------------------------------ */

func toNotes(notes []ai.Note) models.JSONList[models.AINote] {
	out := make(models.JSONList[models.AINote], 0, len(notes))
	for index, note := range notes {
		out = append(out, models.AINote{
			Title:      note.Title,
			Detail:     note.Detail,
			Subtest:    note.Subtest,
			QuestionID: note.QuestionID,
			Priority:   index + 1,
		})
	}
	return out
}

func fromNotes(notes []models.AINote) []dto.AINoteView {
	out := make([]dto.AINoteView, 0, len(notes))
	for _, note := range notes {
		out = append(out, dto.AINoteView{
			Title:      note.Title,
			Detail:     note.Detail,
			Subtest:    note.Subtest,
			QuestionID: note.QuestionID,
			Priority:   note.Priority,
		})
	}
	return out
}

func toMessageView(message models.AIMessage) dto.AIMessageView {
	view := dto.AIMessageView{
		ID:             message.ID,
		Role:           string(message.Role),
		Content:        message.Content,
		Scope:          string(message.Scope),
		QuestionNumber: message.QuestionNumber,
		Provider:       message.Provider,
		Model:          message.Model,
		LatencyMS:      message.LatencyMS,
		CreatedAt:      formatTime(message.CreatedAt),
	}
	if message.QuestionID != nil {
		view.QuestionID = *message.QuestionID
	}
	if message.SubtestName != nil {
		view.SubtestName = *message.SubtestName
	}
	return view
}

func toMessageViews(messages []models.AIMessage) []dto.AIMessageView {
	out := make([]dto.AIMessageView, 0, len(messages))
	for _, message := range messages {
		out = append(out, toMessageView(message))
	}
	return out
}
