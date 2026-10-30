package service

import (
	"context"
	"hash/fnv"
	"math"
	"strconv"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// TryoutService menjalankan mesin tryout: daftar paket, sesi, autosave,
// penilaian, riwayat, dan papan peringkat.
type TryoutService struct {
	tryouts     *repository.TryoutRepository
	sessions    *repository.SessionRepository
	attempts    *repository.AttemptRepository
	enrollments *repository.EnrollmentRepository
	// licenses dipakai menegakkan izin soal: soal tanpa izin aktif tidak
	// dikirim ke peserta (lihat `bankQuestions`).
	licenses *repository.LicenseRepository
	cache    *cache.Cache
	cfg      *config.Config
}

// NewTryoutService membentuk service tryout.
func NewTryoutService(
	tryouts *repository.TryoutRepository,
	sessions *repository.SessionRepository,
	attempts *repository.AttemptRepository,
	enrollments *repository.EnrollmentRepository,
	licenses *repository.LicenseRepository,
	cacheLayer *cache.Cache,
	cfg *config.Config,
) *TryoutService {
	return &TryoutService{
		tryouts:     tryouts,
		sessions:    sessions,
		attempts:    attempts,
		enrollments: enrollments,
		licenses:    licenses,
		cache:       cacheLayer,
		cfg:         cfg,
	}
}

// bankQuestions mengambil soal sebuah bank MILIK satu tryout dari cache.
//
// Bank soal jarang berubah tetapi dibaca setiap kali peserta memulai sesi, jadi
// ini titik caching paling berdampak saat tryout serentak. Kunci cache memuat
// `tryoutID` karena soal sekarang dimiliki per paket tryout.
//
// Dua penyaringan diterapkan sebelum soal dipakai sesi:
//
//  1. Izin soal (bila `QUESTION_LICENSE_ENFORCE` menyala) — soal tanpa izin aktif
//     tidak dikirim ke peserta. Ini penegakan "harus dipastikan izinnya".
//  2. Grup varian — dari beberapa soal sekeluarga (mis. angka berbeda) hanya satu
//     yang dipakai, dipilih secara deterministik per (tryout, bank, grup) agar
//     seluruh peserta satu paket mengerjakan susunan yang sama.
func (s *TryoutService) bankQuestions(ctx context.Context, tryoutID, bankKey string) ([]models.Question, error) {
	return cache.Remember(ctx, s.cache, cache.KeyQuestionBank(tryoutID, bankKey), s.cfg.Redis.QuestionBankTTL,
		func(ctx context.Context) ([]models.Question, error) {
			questions, err := s.tryouts.QuestionsByBanks(ctx, tryoutID, []string{bankKey})
			if err != nil {
				return nil, err
			}
			if s.cfg.Business.QuestionLicenseEnforce {
				questions, err = s.onlyLicensed(ctx, questions)
				if err != nil {
					return nil, err
				}
			}
			return pickVariants(tryoutID, bankKey, questions), nil
		})
}

// onlyLicensed menyaring soal yang izinnya masih berlaku.
//
// Soal tanpa baris izin ikut dibuang (fail-closed): lebih baik satu soal tidak
// terpakai daripada soal tanpa dasar hak pakai dipakai pada produk berbayar.
func (s *TryoutService) onlyLicensed(ctx context.Context, questions []models.Question) ([]models.Question, error) {
	if len(questions) == 0 || s.licenses == nil {
		return questions, nil
	}

	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}

	usable, err := s.licenses.UsableQuestionIDs(ctx, ids, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	out := make([]models.Question, 0, len(questions))
	for _, question := range questions {
		if _, ok := usable[question.ID]; ok {
			out = append(out, question)
		}
	}
	return out, nil
}

// pickVariants memilih satu soal per grup varian.
//
// Soal tanpa grup varian selalu ikut. Pemilihan bersifat deterministik
// (hash dari tryout+bank+grup) sehingga susunan soal satu paket tetap sama untuk
// semua peserta — penting agar penilaian dan papan peringkat tetap adil.
func pickVariants(tryoutID, bankKey string, questions []models.Question) []models.Question {
	hasGroup := false
	for _, question := range questions {
		if question.VariantGroup != nil && *question.VariantGroup != "" {
			hasGroup = true
			break
		}
	}
	if !hasGroup {
		return questions
	}

	grouped := map[string][]models.Question{}
	order := make([]string, 0, len(questions))
	out := make([]models.Question, 0, len(questions))
	for _, question := range questions {
		if question.VariantGroup == nil || *question.VariantGroup == "" {
			out = append(out, question)
			continue
		}
		group := *question.VariantGroup
		if _, ok := grouped[group]; !ok {
			order = append(order, group)
		}
		grouped[group] = append(grouped[group], question)
	}

	for _, group := range order {
		members := grouped[group]
		index := stableVariantIndex(tryoutID, bankKey, group, len(members))
		out = append(out, members[index])
	}
	return out
}

// stableVariantIndex memilih indeks varian yang stabil untuk kombinasi kunci.
func stableVariantIndex(tryoutID, bankKey, group string, size int) int {
	if size <= 1 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(tryoutID + "\x00" + bankKey + "\x00" + group))
	return int(hash.Sum32() % uint32(size))
}

// bankCounts mengembalikan jumlah soal per (tryout, bank).
//
// Dipakai daftar & detail tryout untuk menampilkan jumlah soal tiap subtes
// tanpa query per paket.
func (s *TryoutService) bankCounts(ctx context.Context) (map[string]map[string]int, error) {
	type cached struct {
		Counts map[string]map[string]int
	}
	result, err := cache.Remember(ctx, s.cache, "bank:counts", s.cfg.Redis.QuestionBankTTL,
		func(ctx context.Context) (cached, error) {
			counts, err := s.tryouts.CountQuestionsByTryout(ctx)
			if err != nil {
				return cached{}, err
			}
			return cached{Counts: counts}, nil
		})
	if err != nil {
		return nil, err
	}
	if result.Counts == nil {
		return map[string]map[string]int{}, nil
	}
	return result.Counts, nil
}

// InvalidateBankCache menghapus cache bank soal setelah admin mengubah soal.
func (s *TryoutService) InvalidateBankCache(ctx context.Context) {
	if s.cache == nil {
		return
	}
	s.cache.ForgetPrefix(ctx, cache.PrefixQuestionBank())
	s.cache.ForgetPrefix(ctx, cache.PrefixTryout())
}

// GetPackage memuat satu paket tryout beserta subtesnya.
//
// Dipakai service lain (mis. mentor AI) agar tidak mengakses repository
// tryout secara langsung dan tetap mendapat validasi yang sama.
func (s *TryoutService) GetPackage(ctx context.Context, id string) (*models.TryoutPackage, error) {
	return s.tryouts.FindPackage(ctx, id)
}

// QuestionsForPackage memuat seluruh soal yang dipakai sebuah paket, di-key
// oleh id soal. Bank soal diambil dari cache sehingga pemakaian berulang murah.
func (s *TryoutService) QuestionsForPackage(ctx context.Context, pkg *models.TryoutPackage) (map[string]models.Question, error) {
	return s.questionsForSession(ctx, pkg)
}

// subtestQuestionCount menghitung jumlah soal sebuah subtes dari jumlah soal per
// (tryout, bank) — soal milik tryout lain tidak ikut dihitung walau bank-nya sama.
func subtestQuestionCount(tryoutID string, subtest models.Subtest, counts map[string]map[string]int) int {
	perBank := counts[tryoutID]
	total := 0
	for _, bankKey := range subtest.Banks {
		total += perBank[bankKey]
	}
	return total
}

// List mengembalikan daftar paket tryout beserta status akses peserta.
//
// Semua informasi akses (kepemilikan produk, jumlah percobaan) diambil dalam
// beberapa query agregat, bukan per paket, supaya daftar tetap cepat walau
// paketnya banyak.
func (s *TryoutService) List(ctx context.Context, userID string, isAdmin bool) ([]dto.TryoutSummary, error) {
	packages, err := cache.Remember(ctx, s.cache, cache.KeyTryoutList(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) ([]models.TryoutPackage, error) {
			return s.tryouts.ListPackages(ctx, true)
		})
	if err != nil {
		return nil, err
	}

	counts, err := s.bankCounts(ctx)
	if err != nil {
		return nil, err
	}

	owned := map[string]bool{}
	usedPerTryout := map[string]int{}
	activeSessions := map[string]string{}

	if userID != "" {
		slugs, err := s.enrollments.OwnedSlugs(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, slug := range slugs {
			owned[slug] = true
		}

		usedPerTryout, err = s.attempts.CountByUserPerTryout(ctx, userID)
		if err != nil {
			return nil, err
		}
	}

	out := make([]dto.TryoutSummary, 0, len(packages))
	for _, pkg := range packages {
		summary := s.toSummary(pkg, counts, owned, usedPerTryout, activeSessions, isAdmin)
		out = append(out, summary)
	}
	return out, nil
}

// AdminList mengembalikan daftar paket tryout untuk panel admin.
//
// Berbeda dari `List`, daftar ini tidak memakai cache dan dapat menyertakan paket
// yang dinonaktifkan (soft delete) sehingga admin tetap melihatnya di panel.
func (s *TryoutService) AdminList(ctx context.Context, includeInactive bool) ([]dto.TryoutSummary, error) {
	packages, err := s.tryouts.ListPackages(ctx, !includeInactive)
	if err != nil {
		return nil, err
	}

	counts, err := s.bankCounts(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]dto.TryoutSummary, 0, len(packages))
	for _, pkg := range packages {
		out = append(out, s.toSummary(pkg, counts, map[string]bool{}, map[string]int{}, nil, true))
	}
	return out, nil
}

// Get mengembalikan detail satu paket tryout.
func (s *TryoutService) Get(ctx context.Context, userID, id string, isAdmin bool) (*dto.TryoutDetail, error) {
	pkg, err := s.GetPackage(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := ensurePackageVisible(*pkg, isAdmin); err != nil {
		return nil, err
	}

	counts, err := s.bankCounts(ctx)
	if err != nil {
		return nil, err
	}

	owned := map[string]bool{}
	usedPerTryout := map[string]int{}
	if userID != "" {
		slugs, err := s.enrollments.OwnedSlugs(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, slug := range slugs {
			owned[slug] = true
		}
		usedPerTryout, err = s.attempts.CountByUserPerTryout(ctx, userID)
		if err != nil {
			return nil, err
		}
	}

	summary := s.toSummary(*pkg, counts, owned, usedPerTryout, nil, isAdmin)

	return &dto.TryoutDetail{
		TryoutSummary: summary,
		Rules: []string{
			"Sesi dikerjakan per subtes dengan alokasi waktu masing-masing.",
			"Waktu berjalan di server; menutup browser tidak menghentikan hitungan.",
			"Jawaban tersimpan otomatis setiap kali kamu memilih opsi.",
			"Setelah waktu subtes habis, jawaban subtes tersebut terkunci.",
			"Pembahasan tiap soal terbuka setelah sesi dikumpulkan.",
		},
	}, nil
}

// ensurePackageVisible menolak akses peserta ke paket tryout yang dinonaktifkan
// (soft delete) tanpa menyembunyikan paket itu dari panel admin.
func ensurePackageVisible(pkg models.TryoutPackage, isAdmin bool) error {
	if pkg.Active || isAdmin {
		return nil
	}
	return apperror.NotFound("Tryout tidak ditemukan.")
}

// sessionGraceMinutes adalah kelonggaran tambahan di atas total waktu subtes
// sebelum sesi dianggap kedaluwarsa keras (mis. koneksi terputus sesaat).
const sessionGraceMinutes = 10

// StartSession memulai sesi tryout baru atau melanjutkan sesi aktif.
func (s *TryoutService) StartSession(ctx context.Context, userID, tryoutID string, isAdmin bool) (*dto.TryoutSessionView, error) {
	pkg, err := s.tryouts.FindPackage(ctx, tryoutID)
	if err != nil {
		return nil, err
	}
	if err := ensurePackageVisible(*pkg, isAdmin); err != nil {
		return nil, err
	}

	if err := s.checkAccess(ctx, userID, *pkg, isAdmin); err != nil {
		return nil, err
	}

	// Sudah ada sesi aktif → lanjutkan (idempotent, aman untuk double click).
	if existing, err := s.sessions.FindActiveSession(ctx, userID, tryoutID); err != nil {
		return nil, err
	} else if existing != nil {
		return s.buildSessionView(ctx, existing, pkg)
	}

	order, err := s.buildQuestionOrder(ctx, *pkg)
	if err != nil {
		return nil, err
	}

	totalMinutes := 0
	for i, subtest := range pkg.Subtests {
		totalMinutes += subtest.Minutes
		if len(order[i]) == 0 {
			message := "Subtes " + subtest.Name + " belum memiliki soal. Hubungi tim akademik."
			if s.cfg.Business.QuestionLicenseEnforce {
				message += " Periksa juga izin soal: soal tanpa izin aktif tidak dikirim ke peserta."
			}
			return nil, apperror.BadRequest(message)
		}
	}

	now := time.Now().UTC()
	deadlines := make(models.TimeList, len(pkg.Subtests))
	// Subtes pertama langsung berjalan; subtes lain dimulai saat dibuka peserta.
	if len(pkg.Subtests) > 0 {
		deadlines[0] = now.Add(time.Duration(pkg.Subtests[0].Minutes) * time.Minute)
	}

	session := models.TryoutSession{
		UserID:           userID,
		TryoutID:         tryoutID,
		Status:           models.SessionActive,
		CurrentSubtest:   0,
		QuestionOrder:    models.IDList(order),
		SubtestDeadlines: deadlines,
		StartedAt:        now,
		ExpiresAt:        now.Add(time.Duration(totalMinutes+sessionGraceMinutes) * time.Minute),
	}

	created, err := s.sessions.CreateSession(ctx, &session)
	if err != nil {
		return nil, err
	}

	return s.buildSessionView(ctx, created, pkg)
}

// GetSession mengambil keadaan sesi yang sedang berjalan.
//
// Bila sesi sudah melewati tenggat keras, sesi otomatis dikumpulkan agar
// peserta tetap mendapatkan hasil dari jawaban yang sudah tersimpan.
func (s *TryoutService) GetSession(ctx context.Context, userID, sessionID string) (*dto.TryoutSessionView, error) {
	session, err := s.sessions.FindSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.Forbidden("Sesi ini bukan milik akun Anda.")
	}

	if _, err := s.autoSubmitIfExpired(ctx, session); err != nil {
		return nil, err
	}

	pkg, err := s.tryouts.FindPackage(ctx, session.TryoutID)
	if err != nil {
		return nil, err
	}
	return s.buildSessionView(ctx, session, pkg)
}

// checkAccess memastikan peserta boleh mengerjakan sebuah tryout.
func (s *TryoutService) checkAccess(ctx context.Context, userID string, pkg models.TryoutPackage, isAdmin bool) error {
	if isAdmin {
		return nil
	}
	if userID == "" {
		return apperror.Unauthorized("Masuk terlebih dahulu untuk memulai sesi tryout.")
	}

	if pkg.Tier == models.TierPremium && pkg.RequiredProduct != nil {
		owned, err := s.enrollments.HasProduct(ctx, userID, *pkg.RequiredProduct)
		if err != nil {
			return err
		}
		if !owned {
			return apperror.Forbidden("Tryout ini memerlukan paket " + *pkg.RequiredProduct + ".")
		}
	}

	used, err := s.attempts.CountByUserAndTryout(ctx, userID, pkg.ID)
	if err != nil {
		return err
	}
	if used >= pkg.Attempts {
		return apperror.Locked("Kuota percobaan tryout ini sudah habis.")
	}
	return nil
}

// buildQuestionOrder menyusun snapshot id soal per subtes.
//
// Urutan mengikuti urutan bank pada definisi subtes, lalu urutan posisi soal di
// dalam bank (kolom `position`), sama seperti perilaku front-end.
func (s *TryoutService) buildQuestionOrder(ctx context.Context, pkg models.TryoutPackage) ([][]string, error) {
	order := make([][]string, len(pkg.Subtests))
	for i, subtest := range pkg.Subtests {
		ids := make([]string, 0, 16)
		for _, bankKey := range subtest.Banks {
			questions, err := s.bankQuestions(ctx, pkg.ID, bankKey)
			if err != nil {
				return nil, err
			}
			for _, question := range questions {
				ids = append(ids, question.ID)
			}
		}
		order[i] = ids
	}
	return order, nil
}

// questionsForSession memuat seluruh soal yang dipakai paket ini, di-key oleh id.
func (s *TryoutService) questionsForSession(ctx context.Context, pkg *models.TryoutPackage) (map[string]models.Question, error) {
	out := map[string]models.Question{}
	for _, subtest := range pkg.Subtests {
		for _, bankKey := range subtest.Banks {
			questions, err := s.bankQuestions(ctx, pkg.ID, bankKey)
			if err != nil {
				return nil, err
			}
			for _, question := range questions {
				out[question.ID] = question
			}
		}
	}
	return out, nil
}

// buildSessionView menyusun keadaan sesi untuk klien.
//
// Sisa waktu setiap subtes dihitung dari `subtest_deadlines` yang tersimpan di
// server, bukan dari hitungan klien.
func (s *TryoutService) buildSessionView(
	ctx context.Context,
	session *models.TryoutSession,
	pkg *models.TryoutPackage,
) (*dto.TryoutSessionView, error) {
	answers, err := s.sessions.ListAnswers(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	answerByQuestion := make(map[string]models.TryoutAnswer, len(answers))
	for _, answer := range answers {
		answerByQuestion[answer.QuestionID] = answer
	}

	questions, err := s.questionsForSession(ctx, pkg)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	subtestViews := make([]dto.SessionSubtestView, 0, len(pkg.Subtests))
	totalQuestions := 0
	totalSeconds := 0
	remainingTotal := 0
	answeredTotal := 0

	for i, subtest := range pkg.Subtests {
		ids := []string{}
		if i < len(session.QuestionOrder) {
			ids = session.QuestionOrder[i]
		}

		started := i < len(session.SubtestDeadlines) && !session.SubtestDeadlines[i].IsZero()
		remaining := subtest.Minutes * 60
		if started {
			remaining = session.RemainingSeconds(i, now)
		}
		locked := started && remaining == 0

		questionViews := make([]dto.SessionQuestionView, 0, len(ids))
		answeredInSubtest := 0
		for position, questionID := range ids {
			question, ok := questions[questionID]
			if !ok {
				continue
			}
			answer, hasAnswer := answerByQuestion[questionID]

			view := dto.SessionQuestionView{
				ID:              question.ID,
				Number:          position + 1,
				NumberInSubtest: position + 1,
				Text:            question.Text,
				Options:         emptyIfNil(question.Options),
				SubtestIndex:    i,
				SubtestName:     subtest.Name,
			}
			if hasAnswer {
				view.Flagged = answer.Flagged
				view.SelectedIndex = answer.SelectedIndex
				if answer.SelectedIndex != nil {
					answeredInSubtest++
				}
			}
			questionViews = append(questionViews, view)
		}

		totalQuestions += len(questionViews)
		totalSeconds += subtest.Minutes * 60
		remainingTotal += remaining
		answeredTotal += answeredInSubtest

		subtestViews = append(subtestViews, dto.SessionSubtestView{
			Name:             subtest.Name,
			Minutes:          subtest.Minutes,
			Seconds:          subtest.Minutes * 60,
			Banks:            emptyIfNil(subtest.Banks),
			Started:          started,
			RemainingSeconds: remaining,
			Locked:           locked,
			AnsweredCount:    answeredInSubtest,
			QuestionCount:    len(questionViews),
			Questions:        questionViews,
		})
	}

	view := &dto.TryoutSessionView{
		ID:                    session.ID,
		TryoutID:              session.TryoutID,
		Title:                 pkg.Title,
		Status:                string(session.Status),
		CurrentSubtest:        session.CurrentSubtest,
		StartedAt:             formatTime(session.StartedAt),
		ExpiresAt:             formatTime(session.ExpiresAt),
		ServerTime:            formatTime(now),
		TotalQuestions:        totalQuestions,
		TotalSeconds:          totalSeconds,
		RemainingTotalSeconds: remainingTotal,
		AnsweredTotal:         answeredTotal,
		Subtests:              subtestViews,
	}
	if session.AttemptID != nil {
		view.AttemptID = *session.AttemptID
	}
	return view, nil
}

// SaveAnswers menyimpan jawaban peserta (autosave).
//
// Aturan yang dijaga di sini:
//   - soal harus benar-benar bagian dari sesi (mencegah injeksi id sembarangan),
//   - subtes yang waktunya sudah habis tidak bisa diubah lagi,
//   - subtes baru otomatis mulai berjalan saat pertama kali diisi.
func (s *TryoutService) SaveAnswers(
	ctx context.Context,
	userID, sessionID string,
	req dto.SaveAnswersRequest,
) (*dto.SaveAnswersResponse, error) {
	session, err := s.sessions.FindSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.Forbidden("Sesi ini bukan milik akun Anda.")
	}

	if attempt, err := s.autoSubmitIfExpired(ctx, session); err != nil {
		return nil, err
	} else if attempt != nil {
		return nil, apperror.Conflict("Waktu sesi sudah habis; jawaban otomatis dikumpulkan.").
			WithDetails(map[string]any{"attemptId": attempt.ID})
	}
	if session.Status != models.SessionActive {
		return nil, apperror.Conflict("Sesi ini sudah dikumpulkan.")
	}

	pkg, err := s.tryouts.FindPackage(ctx, session.TryoutID)
	if err != nil {
		return nil, err
	}

	subtestOfQuestion := make(map[string]int)
	for index, ids := range session.QuestionOrder {
		for _, id := range ids {
			subtestOfQuestion[id] = index
		}
	}

	now := time.Now().UTC()
	touched := map[int]bool{}
	answers := make([]models.TryoutAnswer, 0, len(req.Answers))

	for _, item := range req.Answers {
		index, ok := subtestOfQuestion[item.QuestionID]
		if !ok {
			return nil, apperror.BadRequest("Soal " + item.QuestionID + " bukan bagian dari sesi ini.")
		}
		if s.isSubtestLocked(session, index, now) {
			name := "subtes ini"
			if index < len(pkg.Subtests) {
				name = pkg.Subtests[index].Name
			}
			return nil, apperror.Locked("Waktu " + name + " sudah habis, jawaban terkunci.")
		}

		answers = append(answers, models.TryoutAnswer{
			SessionID:     session.ID,
			QuestionID:    item.QuestionID,
			SubtestIndex:  index,
			SelectedIndex: item.SelectedIndex,
			Flagged:       item.Flagged != nil && *item.Flagged,
			AnsweredAt:    now,
		})
		touched[index] = true
	}

	// Buka subtes berikutnya bila peserta mengirim jawaban untuk subtes itu.
	if req.SubtestIndex != nil {
		touched[*req.SubtestIndex] = true
	}
	if err := s.advanceSubtests(ctx, session, pkg, touched, now); err != nil {
		return nil, err
	}

	if err := s.sessions.SaveAnswers(ctx, answers); err != nil {
		return nil, err
	}

	answeredTotal, err := s.sessions.CountAnswered(ctx, session.ID)
	if err != nil {
		return nil, err
	}

	return &dto.SaveAnswersResponse{
		SavedCount:            len(answers),
		AnsweredTotal:         answeredTotal,
		RemainingTotalSeconds: s.remainingTotal(session, pkg, now),
		LockedSubtests:        s.lockedSubtests(session, pkg, now),
		ServerTime:            formatTime(now),
	}, nil
}

// advanceSubtests memulai subtes yang baru dibuka peserta.
//
// Hanya boleh maju (tidak bisa kembali ke subtes sebelumnya) agar tenggat waktu
// tidak bisa direset dari sisi klien.
func (s *TryoutService) advanceSubtests(
	ctx context.Context,
	session *models.TryoutSession,
	pkg *models.TryoutPackage,
	touched map[int]bool,
	now time.Time,
) error {
	target := session.CurrentSubtest
	for index := range touched {
		if index > target {
			target = index
		}
	}
	if target <= session.CurrentSubtest || target >= len(pkg.Subtests) {
		return nil
	}

	deadlines := make(models.TimeList, len(pkg.Subtests))
	copy(deadlines, session.SubtestDeadlines)
	for len(deadlines) < len(pkg.Subtests) {
		deadlines = append(deadlines, time.Time{})
	}

	for i := session.CurrentSubtest; i <= target; i++ {
		if deadlines[i].IsZero() {
			deadlines[i] = now.Add(time.Duration(pkg.Subtests[i].Minutes) * time.Minute)
		}
	}

	session.SubtestDeadlines = deadlines
	session.CurrentSubtest = target

	return s.sessions.UpdateSessionDeadlines(ctx, session.ID, deadlines, target)
}

// isSubtestLocked menandakan subtes sudah berjalan dan waktunya habis.
func (s *TryoutService) isSubtestLocked(session *models.TryoutSession, index int, now time.Time) bool {
	if index < 0 || index >= len(session.SubtestDeadlines) {
		return false
	}
	deadline := session.SubtestDeadlines[index]
	if deadline.IsZero() {
		return false
	}
	return !now.Before(deadline)
}

// lockedSubtests mengembalikan daftar index subtes yang sudah terkunci.
func (s *TryoutService) lockedSubtests(session *models.TryoutSession, pkg *models.TryoutPackage, now time.Time) []int {
	out := make([]int, 0, len(pkg.Subtests))
	for index := range pkg.Subtests {
		if s.isSubtestLocked(session, index, now) {
			out = append(out, index)
		}
	}
	return out
}

// remainingTotal menjumlahkan sisa waktu seluruh subtes.
func (s *TryoutService) remainingTotal(session *models.TryoutSession, pkg *models.TryoutPackage, now time.Time) int {
	total := 0
	for index, subtest := range pkg.Subtests {
		if index < len(session.SubtestDeadlines) && !session.SubtestDeadlines[index].IsZero() {
			total += session.RemainingSeconds(index, now)
			continue
		}
		total += subtest.Minutes * 60
	}
	return total
}

// Submit mengumpulkan sesi dan mengembalikan hasil lengkap beserta pembahasan.
func (s *TryoutService) Submit(ctx context.Context, userID, sessionID string) (*dto.AttemptResultView, error) {
	session, err := s.sessions.FindSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.Forbidden("Sesi ini bukan milik akun Anda.")
	}

	pkg, err := s.tryouts.FindPackage(ctx, session.TryoutID)
	if err != nil {
		return nil, err
	}

	attempt, err := s.submitAttempt(ctx, session, pkg)
	if err != nil {
		return nil, err
	}

	return s.buildResultView(ctx, session, pkg, attempt)
}

// autoSubmitIfExpired mengumpulkan sesi yang sudah melewati tenggat keras.
func (s *TryoutService) autoSubmitIfExpired(ctx context.Context, session *models.TryoutSession) (*models.Attempt, error) {
	if session.Status != models.SessionActive {
		return nil, nil
	}
	if time.Now().UTC().Before(session.ExpiresAt) {
		return nil, nil
	}

	pkg, err := s.tryouts.FindPackage(ctx, session.TryoutID)
	if err != nil {
		return nil, err
	}
	return s.submitAttempt(ctx, session, pkg)
}

// submitAttempt menjalankan penilaian di server dan menyimpan attempt.
//
// Seluruh skor dihitung dari jawaban yang tersimpan di database (bukan dari
// payload klien), sehingga hasil tidak bisa dipalsukan.
func (s *TryoutService) submitAttempt(
	ctx context.Context,
	session *models.TryoutSession,
	pkg *models.TryoutPackage,
) (*models.Attempt, error) {
	questions, err := s.questionsForSession(ctx, pkg)
	if err != nil {
		return nil, err
	}

	builder := func(locked *models.TryoutSession, answers []models.TryoutAnswer) (*models.Attempt, error) {
		answerByQuestion := make(map[string]models.TryoutAnswer, len(answers))
		for _, answer := range answers {
			answerByQuestion[answer.QuestionID] = answer
		}

		categories := make([]models.AttemptCategory, 0, len(pkg.Subtests))
		totalCorrect := 0
		totalQuestions := 0

		for index, subtest := range pkg.Subtests {
			var ids []string
			if index < len(locked.QuestionOrder) {
				ids = locked.QuestionOrder[index]
			}

			correct := 0
			for _, questionID := range ids {
				question, ok := questions[questionID]
				if !ok {
					continue
				}
				answer, hasAnswer := answerByQuestion[questionID]
				if hasAnswer && answer.SelectedIndex != nil && *answer.SelectedIndex == question.AnswerIndex {
					correct++
				}
			}

			totalQuestions += len(ids)
			totalCorrect += correct
			categories = append(categories, models.AttemptCategory{
				Name:      subtest.Name,
				Correct:   correct,
				Total:     len(ids),
				Score:     percent(correct, len(ids)),
				SortOrder: index,
			})
		}

		score := percent(totalCorrect, totalQuestions)
		status := models.AttemptFailed
		if score >= s.cfg.Business.PassingThreshold {
			status = models.AttemptPassed
		}

		return &models.Attempt{
			UserID:          locked.UserID,
			TryoutID:        locked.TryoutID,
			Title:           pkg.Title,
			Score:           score,
			Correct:         totalCorrect,
			Total:           totalQuestions,
			Status:          status,
			DurationSeconds: durationSeconds(locked),
			TakenAt:         time.Now().UTC(),
			Categories:      categories,
		}, nil
	}

	return s.sessions.SubmitSession(ctx, session.ID, builder)
}

// percent menghitung persentase bulat; 0 bila pembagi nol.
func percent(part, total int) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(part) / float64(total) * 100))
}

// durationSeconds menghitung durasi pengerjaan, dibatasi oleh tenggat sesi.
func durationSeconds(session *models.TryoutSession) int {
	end := time.Now().UTC()
	if end.After(session.ExpiresAt) {
		end = session.ExpiresAt
	}
	seconds := int(end.Sub(session.StartedAt).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

// buildResultView menyusun hasil + pembahasan lengkap per soal.
//
// Pembahasan hanya tersedia setelah sesi dikumpulkan — inilah satu-satunya
// tempat kunci jawaban (`AnswerIndex`) dikirim ke klien.
func (s *TryoutService) buildResultView(
	ctx context.Context,
	session *models.TryoutSession,
	pkg *models.TryoutPackage,
	attempt *models.Attempt,
) (*dto.AttemptResultView, error) {
	answerByQuestion := map[string]models.TryoutAnswer{}
	if session.ID != "" {
		answers, err := s.sessions.ListAnswers(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		for _, answer := range answers {
			answerByQuestion[answer.QuestionID] = answer
		}
	}

	questions, err := s.questionsForSession(ctx, pkg)
	if err != nil {
		return nil, err
	}

	scoreByIndex := make(map[int]models.AttemptCategory, len(attempt.Categories))
	for _, category := range attempt.Categories {
		scoreByIndex[category.SortOrder] = category
	}

	review := make([]dto.ReviewSubtestView, 0, len(pkg.Subtests))
	for index, subtest := range pkg.Subtests {
		var ids []string
		if index < len(session.QuestionOrder) {
			ids = session.QuestionOrder[index]
		}

		questionViews := make([]dto.ReviewQuestionView, 0, len(ids))
		for position, questionID := range ids {
			question, ok := questions[questionID]
			if !ok {
				continue
			}
			answer, hasAnswer := answerByQuestion[questionID]

			var selected *int
			flagged := false
			if hasAnswer {
				selected = answer.SelectedIndex
				flagged = answer.Flagged
			}

			questionViews = append(questionViews, dto.ReviewQuestionView{
				ID:            question.ID,
				Number:        position + 1,
				Text:          question.Text,
				Options:       emptyIfNil(question.Options),
				AnswerIndex:   question.AnswerIndex,
				SelectedIndex: selected,
				Correct:       selected != nil && *selected == question.AnswerIndex,
				Flagged:       flagged,
				Explanation:   question.Explanation,
			})
		}

		category := scoreByIndex[index]
		review = append(review, dto.ReviewSubtestView{
			Name:      subtest.Name,
			Score:     category.Score,
			Correct:   category.Correct,
			Total:     category.Total,
			Questions: questionViews,
		})
	}

	return &dto.AttemptResultView{
		AttemptView:      toAttemptView(*attempt),
		PassingThreshold: s.cfg.Business.PassingThreshold,
		Review:           review,
	}, nil
}

// ListAttempts mengambil riwayat tryout peserta.
func (s *TryoutService) ListAttempts(ctx context.Context, userID string, page, perPage int) ([]dto.AttemptView, int64, error) {
	rows, total, err := s.attempts.ListByUser(ctx, userID, page, perPage)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dto.AttemptView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAttemptView(row))
	}
	return out, total, nil
}

// GetAttempt mengambil detail satu hasil tryout beserta pembahasannya.
func (s *TryoutService) GetAttempt(ctx context.Context, userID, attemptID string) (*dto.AttemptDetailView, error) {
	attempt, err := s.attempts.FindByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, apperror.Forbidden("Hasil tryout ini bukan milik akun Anda.")
	}

	pkg, err := s.tryouts.FindPackage(ctx, attempt.TryoutID)
	if err != nil {
		return nil, err
	}

	// Snapshot susunan soal diambil dari sesi; bila sesi sudah dibersihkan,
	// susunan dibangun ulang dari bank soal terkini.
	session, err := s.sessions.FindByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		order, orderErr := s.buildQuestionOrder(ctx, *pkg)
		if orderErr != nil {
			return nil, orderErr
		}
		session = &models.TryoutSession{
			UserID:        userID,
			TryoutID:      attempt.TryoutID,
			QuestionOrder: models.IDList(order),
		}
	}

	result, err := s.buildResultView(ctx, session, pkg, attempt)
	if err != nil {
		return nil, err
	}

	return &dto.AttemptDetailView{
		AttemptResultView: *result,
		TryoutTitle:       pkg.Title,
	}, nil
}

// leaderboardCacheTTL adalah masa simpan cache papan peringkat.
//
// Sengaja pendek: papan peringkat harus terasa "hidup" saat tryout serentak,
// tetapi tetap dilindungi agar query agregat tidak dijalankan tiap request.
const leaderboardCacheTTL = 10 * time.Second

// Leaderboard mengambil papan peringkat sebuah tryout.
func (s *TryoutService) Leaderboard(ctx context.Context, tryoutID, userID string, limit int) (*dto.LeaderboardResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	cacheKey := cache.KeyLeaderboard(tryoutID) + ":" + strconv.Itoa(limit)
	rows, err := cache.Remember(ctx, s.cache, cacheKey, leaderboardCacheTTL,
		func(ctx context.Context) ([]repository.LeaderboardRow, error) {
			return s.attempts.Leaderboard(ctx, tryoutID, limit)
		})
	if err != nil {
		return nil, err
	}

	entries := make([]dto.LeaderboardEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, toLeaderboardEntry(row, userID))
	}

	response := &dto.LeaderboardResponse{TryoutID: tryoutID, Entries: entries}

	// Peringkat pribadi selalu diambil langsung agar tidak tertinggal cache.
	if userID != "" {
		myRow, err := s.attempts.LeaderboardRank(ctx, tryoutID, userID)
		if err != nil {
			return nil, err
		}
		if myRow != nil {
			mine := toLeaderboardEntry(*myRow, userID)
			response.MyRank = &mine
		}
	}

	return response, nil
}

// ExpireStaleSessions menyapu sesi aktif yang melewati tenggat (dipakai worker).
func (s *TryoutService) ExpireStaleSessions(ctx context.Context, limit int) (int64, error) {
	return s.sessions.ExpireStaleSessions(ctx, limit)
}

// RecentAttempts mengambil beberapa hasil terbaru (dipakai ringkasan dashboard).
func (s *TryoutService) RecentAttempts(ctx context.Context, userID string, limit int) ([]dto.AttemptView, error) {
	rows, err := s.attempts.ListRecentByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AttemptView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAttemptView(row))
	}
	return out, nil
}

// Stats mengembalikan ringkasan performa peserta.
func (s *TryoutService) Stats(ctx context.Context, userID string) (repository.Stats, error) {
	return s.attempts.StatsByUser(ctx, userID, s.cfg.Business.PassingThreshold)
}

// ScoreTrend mengambil tren skor peserta.
func (s *TryoutService) ScoreTrend(ctx context.Context, userID string, limit int) ([]repository.ScoreTrendPoint, error) {
	return s.attempts.ScoreTrend(ctx, userID, limit)
}

// PassingThreshold mengembalikan ambang batas kelulusan yang dikonfigurasi.
func (s *TryoutService) PassingThreshold() int { return s.cfg.Business.PassingThreshold }
func (s *TryoutService) toSummary(
	pkg models.TryoutPackage,
	counts map[string]map[string]int,
	owned map[string]bool,
	usedPerTryout map[string]int,
	activeSessions map[string]string,
	isAdmin bool,
) dto.TryoutSummary {
	subtests := make([]dto.SubtestView, 0, len(pkg.Subtests))
	questionCount := 0
	minutes := 0
	for _, subtest := range pkg.Subtests {
		subCount := subtestQuestionCount(pkg.ID, subtest, counts)
		questionCount += subCount
		minutes += subtest.Minutes
		subtests = append(subtests, toSubtestView(subtest, subCount))
	}

	used := usedPerTryout[pkg.ID]
	unlocked := true
	reason := ""

	if pkg.Tier == models.TierPremium && pkg.RequiredProduct != nil && !isAdmin {
		if !owned[*pkg.RequiredProduct] {
			unlocked = false
			reason = "Perlu memiliki paket " + *pkg.RequiredProduct + " untuk membuka tryout ini."
		}
	}

	attemptsLeft := pkg.Attempts - used
	if attemptsLeft < 0 {
		attemptsLeft = 0
	}
	if unlocked && attemptsLeft == 0 {
		unlocked = false
		reason = "Kuota percobaan tryout ini sudah habis."
	}

	summary := dto.TryoutSummary{
		ID:              pkg.ID,
		Title:           pkg.Title,
		Description:     pkg.Description,
		Tier:            string(pkg.Tier),
		RequiredProduct: pkg.RequiredProduct,
		Attempts:        pkg.Attempts,
		AccessUntil:     pkg.AccessUntil,
		Subtests:        subtests,
		QuestionCount:   questionCount,
		Minutes:         minutes,
		Unlocked:        unlocked,
		AttemptsUsed:    used,
		AttemptsLeft:    attemptsLeft,
		LockedReason:    reason,
		Active:          pkg.Active,
	}
	if activeSessions != nil {
		summary.ActiveSessionID = activeSessions[pkg.ID]
	}
	return summary
}
