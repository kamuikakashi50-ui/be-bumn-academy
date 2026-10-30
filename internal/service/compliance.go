package service

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/fingerprint"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Batas keluaran laporan kemiripan: cukup untuk ditinjau manusia, tanpa membuat
// balasan API membengkak pada bank soal besar.
const (
	complianceMaxMatches = 20
	complianceMaxPairs   = 100
	complianceBackfill   = 500
)

// ComplianceService menjaga keunikan bank soal dan jejak auditnya.
//
// Latar belakangnya adalah perjanjian kerahasiaan soal (pakta): soal yang sudah
// dipakai tidak boleh muncul identik lagi, dan tim akademik perlu dibantu
// menemukan soal kembar sebelum soal itu terbit.
type ComplianceService struct {
	fingerprints *repository.ComplianceRepository
	questions    *repository.TryoutRepository
	cfg          *config.Config
	logger       *slog.Logger
}

// NewComplianceService membentuk service kepatuhan bank soal.
func NewComplianceService(
	fingerprints *repository.ComplianceRepository,
	questions *repository.TryoutRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *ComplianceService {
	return &ComplianceService{
		fingerprints: fingerprints,
		questions:    questions,
		cfg:          cfg,
		logger:       logger,
	}
}

// complianceFingerprint adalah sidik jari satu soal dalam bentuk yang siap
// dibandingkan (memuat metadata untuk pelaporan).
type complianceFingerprint struct {
	questionID string
	bankKey    string
	sectorKey  string
	testType   string
	tryoutID   string
	group      string
	text       string
	hash       string
	template   string
	tokens     []string
}

// newFingerprintFromQuestion menghitung sidik jari sebuah soal.
func newFingerprintFromQuestion(question models.Question) complianceFingerprint {
	item := complianceFingerprint{
		questionID: question.ID,
		bankKey:    question.BankKey,
		sectorKey:  question.SectorKey,
		testType:   question.TestType,
		text:       question.Text,
		hash:       fingerprint.Hash(question.Text),
		template:   fingerprint.TemplateHash(question.Text),
		tokens:     fingerprint.Tokens(question.Text),
	}
	if question.TryoutID != nil {
		item.tryoutID = *question.TryoutID
	}
	if question.VariantGroup != nil {
		item.group = *question.VariantGroup
	}
	return item
}

// newFingerprintFromRow mengubah baris laporan menjadi sidik jari.
func newFingerprintFromRow(row repository.FingerprintRow) complianceFingerprint {
	item := complianceFingerprint{
		questionID: row.QuestionID,
		bankKey:    row.BankKey,
		sectorKey:  row.SectorKey,
		testType:   row.TestType,
		text:       row.Text,
		hash:       row.Fingerprint,
		template:   row.TemplateHash,
		tokens:     row.Tokens,
	}
	if row.TryoutID != nil {
		item.tryoutID = *row.TryoutID
	}
	if row.VariantGroup != nil {
		item.group = *row.VariantGroup
	}
	return item
}

// model mengubah sidik jari menjadi baris tabel.
func (f complianceFingerprint) model() models.QuestionFingerprint {
	return models.QuestionFingerprint{
		QuestionID:   f.questionID,
		Fingerprint:  f.hash,
		TemplateHash: f.template,
		Tokens:       models.StringList(f.tokens),
		ComputedAt:   time.Now().UTC(),
	}
}

// classify membandingkan dua sidik jari dan mengembalikan jenis kemiripan.
//
//	identik  → teks sama persis setelah normalisasi
//	keluarga → teks sama persis kecuali angka (satu rumus, angka berbeda)
//	mirip    → kemiripan kata di atas ambang
//
// Jenis kosong berarti tidak mirip.
func classify(left, right complianceFingerprint, threshold float64) (string, float64) {
	switch {
	case left.hash != "" && left.hash == right.hash:
		return dto.DuplicateIdentical, 1
	case left.template != "" && left.template == right.template:
		return dto.DuplicateFamily, fingerprint.Similarity(left.tokens, right.tokens)
	}

	// Penolakan cepat sebelum menghitung kemiripan: Jaccard tidak pernah
	// melebihi min/max jumlah kata, sehingga pasangan yang panjangnya jauh
	// berbeda tidak perlu dihitung sama sekali.
	if !couldExceedThreshold(len(left.tokens), len(right.tokens), threshold) {
		return "", 0
	}
	similarity := fingerprint.Similarity(left.tokens, right.tokens)
	if similarity >= threshold {
		return dto.DuplicateSimilar, similarity
	}
	return "", 0
}

// couldExceedThreshold memeriksa apakah dua himpunan kata mungkin melewati
// ambang kemiripan (batas atas Jaccard adalah min/max).
func couldExceedThreshold(leftSize, rightSize int, threshold float64) bool {
	if leftSize == 0 || rightSize == 0 {
		return false
	}
	small, large := leftSize, rightSize
	if small > large {
		small, large = large, small
	}
	return float64(small)/float64(large) >= threshold
}

// sameVariantGroup menandai dua soal yang memang sengaja dijadikan varian satu
// sama lain (angka berbeda, rumus sama), sehingga bukan temuan duplikat.
func sameVariantGroup(left, right complianceFingerprint) bool {
	return left.group != "" && left.group == right.group
}

/* --------------------------- pemakaian sidik jari ------------------------- */

// EnsureFingerprint menghitung dan menyimpan sidik jari satu soal.
func (s *ComplianceService) EnsureFingerprint(ctx context.Context, question models.Question) error {
	item := newFingerprintFromQuestion(question).model()
	return s.fingerprints.UpsertFingerprint(ctx, &item)
}

// EnsureFingerprints menghitung dan menyimpan sidik jari banyak soal sekaligus.
func (s *ComplianceService) EnsureFingerprints(ctx context.Context, questions []models.Question) error {
	if len(questions) == 0 {
		return nil
	}
	items := make([]models.QuestionFingerprint, 0, len(questions))
	for _, question := range questions {
		items = append(items, newFingerprintFromQuestion(question).model())
	}
	return s.fingerprints.UpsertFingerprints(ctx, items)
}

// RemoveFingerprint menghapus sidik jari soal yang sudah dihapus.
func (s *ComplianceService) RemoveFingerprint(ctx context.Context, questionID string) error {
	return s.fingerprints.DeleteFingerprint(ctx, questionID)
}

// backfill melengkapi sidik jari soal yang belum dihitung.
//
// Perhitungan sengaja tidak dilakukan di migrasi SQL: normalisasi teks hanya
// hidup di internal/pkg/fingerprint, sehingga hash yang tersimpan selalu sama
// dengan yang dihitung aplikasi.
func (s *ComplianceService) backfill(ctx context.Context) (int, error) {
	total := 0
	for round := 0; round < 5; round++ {
		questions, err := s.fingerprints.ListQuestionsWithoutFingerprint(ctx, complianceBackfill)
		if err != nil {
			return total, err
		}
		if len(questions) == 0 {
			break
		}
		if err := s.EnsureFingerprints(ctx, questions); err != nil {
			return total, err
		}
		total += len(questions)
		if len(questions) < complianceBackfill {
			break
		}
	}
	return total, nil
}

// Check memeriksa apakah sebuah teks soal mirip dengan soal yang sudah ada.
//
// Dipakai panel bank soal sebelum menyimpan/mengimpor: admin melihat temuan
// kembar dan bisa memutuskan mengganti teks, menjadikannya varian, atau
// melanjutkan.
func (s *ComplianceService) Check(ctx context.Context, req dto.CheckDuplicateRequest) (*dto.CheckDuplicateResponse, error) {
	if _, err := s.backfill(ctx); err != nil {
		return nil, err
	}

	threshold := fingerprint.DefaultThreshold
	if req.Threshold != nil {
		threshold = *req.Threshold
	}

	target := complianceFingerprint{
		questionID: req.QuestionID,
		bankKey:    req.BankKey,
		group:      req.VariantGroup,
		text:       req.Text,
		hash:       fingerprint.Hash(req.Text),
		template:   fingerprint.TemplateHash(req.Text),
		tokens:     fingerprint.Tokens(req.Text),
	}

	filter := repository.FingerprintFilter{
		BankKey:   req.BankKey,
		SectorKey: req.SectorKey,
	}
	if filter.BankKey == "" && req.QuestionID != "" {
		// Soal yang sudah ada: batasi pembanding ke kategori yang sama agar
		// pemeriksaan tetap cepat.
		if question, err := s.questions.FindQuestion(ctx, req.QuestionID); err == nil {
			filter.BankKey = question.BankKey
		}
	}

	rows, err := s.fingerprints.ListFingerprints(ctx, filter, 0)
	if err != nil {
		return nil, err
	}

	response := &dto.CheckDuplicateResponse{
		Kind:      "unik",
		Threshold: threshold,
		Scanned:   len(rows),
	}

	matches := make([]dto.DuplicateMatch, 0, complianceMaxMatches)
	for _, row := range rows {
		candidate := newFingerprintFromRow(row)
		if candidate.questionID == target.questionID {
			continue
		}
		if sameVariantGroup(target, candidate) {
			continue
		}

		kind, similarity := classify(target, candidate, threshold)
		if kind == "" {
			continue
		}
		matches = append(matches, toDuplicateMatch(candidate, kind, similarity))
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Similarity > matches[j].Similarity
	})
	if len(matches) > complianceMaxMatches {
		matches = matches[:complianceMaxMatches]
	}

	if len(matches) > 0 {
		response.Kind = matches[0].Kind
		response.Matches = matches
	}
	return response, nil
}

// toDuplicateMatch membentuk satu temuan untuk balasan API.
func toDuplicateMatch(candidate complianceFingerprint, kind string, similarity float64) dto.DuplicateMatch {
	return dto.DuplicateMatch{
		QuestionID:   candidate.questionID,
		Text:         candidate.text,
		BankKey:      candidate.bankKey,
		BankLabel:    bankLabelOf(candidate.bankKey),
		SectorKey:    candidate.sectorKey,
		TestType:     candidate.testType,
		TryoutID:     candidate.tryoutID,
		VariantGroup: candidate.group,
		Kind:         kind,
		Similarity:   roundTo2(similarity),
	}
}

/* ------------------------------ laporan kembar --------------------------- */

// Report menyusun laporan soal kembar untuk halaman kepatuhan.
//
// Perbandingan hanya dilakukan antar soal berkategori bank yang sama, sehingga
// biayanya tetap wajar walau bank soal berisi ribuan baris.
func (s *ComplianceService) Report(ctx context.Context, filter repository.FingerprintFilter, limit int) (*dto.UniquenessReportView, error) {
	fingerprinted, err := s.backfill(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.fingerprints.ListFingerprints(ctx, filter, limit)
	if err != nil {
		return nil, err
	}

	byBank := map[string][]complianceFingerprint{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		item := newFingerprintFromRow(row)
		if _, ok := byBank[item.bankKey]; !ok {
			order = append(order, item.bankKey)
		}
		byBank[item.bankKey] = append(byBank[item.bankKey], item)
	}

	pairs := make([]dto.DuplicatePair, 0, complianceMaxPairs)
	for _, bankKey := range order {
		group := byBank[bankKey]
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				if sameVariantGroup(group[i], group[j]) {
					continue
				}
				kind, similarity := classify(group[i], group[j], fingerprint.DefaultThreshold)
				if kind == "" {
					continue
				}
				pairs = append(pairs, dto.DuplicatePair{
					Kind:       kind,
					Similarity: roundTo2(similarity),
					Left:       toDuplicateMatch(group[i], kind, similarity),
					Right:      toDuplicateMatch(group[j], kind, similarity),
				})
			}
		}
	}

	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].Similarity > pairs[j].Similarity
	})
	truncated := len(pairs) > complianceMaxPairs
	if truncated {
		pairs = pairs[:complianceMaxPairs]
	}

	return &dto.UniquenessReportView{
		Pairs:         pairs,
		Scanned:       len(rows),
		Fingerprinted: fingerprinted,
		Truncated:     truncated,
		Threshold:     fingerprint.DefaultThreshold,
		GeneratedAt:   formatTime(time.Now().UTC()),
	}, nil
}

// HintsFor mencari soal kembar untuk sekumpulan soal yang baru disimpan
// (dipakai import massal).
//
// Temuan dikembalikan sebagai catatan, bukan penghalang: import tetap berjalan
// dan tim akademik yang memutuskan tindak lanjutnya.
func (s *ComplianceService) HintsFor(ctx context.Context, questions []models.Question, threshold float64, max int) []dto.DuplicateHint {
	if len(questions) == 0 || max <= 0 {
		return nil
	}
	if err := s.EnsureFingerprints(ctx, questions); err != nil && s.logger != nil {
		s.logger.Warn("gagal menyimpan sidik jari soal import", "error", err)
	}

	banks := map[string]struct{}{}
	for _, question := range questions {
		banks[question.BankKey] = struct{}{}
	}

	candidates := make([]complianceFingerprint, 0, 64)
	for bankKey := range banks {
		rows, err := s.fingerprints.ListFingerprints(ctx, repository.FingerprintFilter{BankKey: bankKey}, 0)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("gagal memuat pembanding sidik jari", "bankKey", bankKey, "error", err)
			}
			continue
		}
		for _, row := range rows {
			candidates = append(candidates, newFingerprintFromRow(row))
		}
	}

	hints := make([]dto.DuplicateHint, 0, max)
	for _, question := range questions {
		target := newFingerprintFromQuestion(question)
		for _, candidate := range candidates {
			if candidate.questionID == target.questionID || sameVariantGroup(target, candidate) {
				continue
			}
			kind, similarity := classify(target, candidate, threshold)
			if kind == "" {
				continue
			}
			hints = append(hints, dto.DuplicateHint{
				QuestionID: target.questionID,
				OtherID:    candidate.questionID,
				OtherText:  candidate.text,
				Kind:       kind,
				Similarity: roundTo2(similarity),
			})
			if len(hints) >= max {
				return hints
			}
		}
	}
	return hints
}

/* ------------------------------ jejak audit ------------------------------ */

// Refresh menghitung ulang sidik jari soal dan mencatat aksinya.
//
// Dipanggil setiap soal dibuat/diubah/diimpor. Kegagalan penghitungan sidik jari
// hanya dicatat ke log: soal tetap tersimpan, dan sidik jarinya akan dihitung
// ulang saat laporan duplikat dibuka.
func (s *ComplianceService) Refresh(ctx context.Context, question models.Question, actorID, action, detail string) {
	if err := s.EnsureFingerprint(ctx, question); err != nil && s.logger != nil {
		s.logger.Warn("gagal menghitung sidik jari soal",
			"questionId", question.ID, "error", err)
	}
	s.Record(ctx, actorID, question.ID, action, detail)
}

// Record mencatat satu aksi pada bank soal.
//
// Kegagalan pencatatan hanya dicatat ke log server: jejak audit penting, tetapi
// tidak boleh menggagalkan pekerjaan tim akademik.
func (s *ComplianceService) Record(ctx context.Context, actorID, questionID, action, detail string) {
	if questionID == "" {
		return
	}
	entry := &models.QuestionAccessLog{QuestionID: questionID, Action: action, Detail: detail}
	if actorID != "" {
		entry.ActorID = &actorID
	}
	if err := s.fingerprints.LogAccess(ctx, entry); err != nil && s.logger != nil {
		s.logger.Warn("gagal mencatat jejak akses soal",
			"questionId", questionID, "action", action, "error", err)
	}
}

// AccessLog mengambil jejak audit bank soal.
func (s *ComplianceService) AccessLog(ctx context.Context, filter repository.AccessLogFilter, limit int) ([]dto.QuestionAccessLogView, error) {
	rows, err := s.fingerprints.ListAccessLogs(ctx, filter, limit)
	if err != nil {
		return nil, err
	}

	out := make([]dto.QuestionAccessLogView, 0, len(rows))
	for _, row := range rows {
		view := dto.QuestionAccessLogView{
			ID:         row.ID,
			QuestionID: row.QuestionID,
			Action:     row.Action,
			Detail:     row.Detail,
			CreatedAt:  formatTime(row.CreatedAt),
		}
		if row.ActorID != nil {
			view.ActorID = *row.ActorID
		}
		out = append(out, view)
	}
	return out, nil
}

// roundTo2 membulatkan angka ke dua desimal (skor kemiripan).
func roundTo2(value float64) float64 {
	return math.Round(value*100) / 100
}

// trimDetail memotong detail jejak audit agar kolom tidak membengkak.
func trimDetail(value string) string {
	const max = 200
	trimmed := strings.TrimSpace(value)
	if len([]rune(trimmed)) <= max {
		return trimmed
	}
	return string([]rune(trimmed)[:max])
}
