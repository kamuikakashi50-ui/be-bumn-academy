package service

import (
	"context"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

/* ------------------------------ kuis (peserta) ----------------------------- */

// quizQuestion adalah soal kuis lengkap dengan kunci jawaban.
//
// Struktur ini hanya hidup di server: versi yang dikirim ke klien adalah
// dto.QuizQuestionView tanpa `AnswerIndex`/`Explanation`.
type quizQuestion struct {
	ID          string
	Text        string
	Options     []string
	AnswerIndex int
	Explanation string
}

// LessonQuiz mengambil kuis aktif satu pelajaran untuk dikerjakan peserta.
func (s *LearningPathService) LessonQuiz(ctx context.Context, userID, lessonID string) (*dto.LessonQuizView, error) {
	lesson, err := s.classroom.FindLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.access.RequireLesson(ctx, userID, *lesson); err != nil {
		return nil, err
	}

	quiz, err := s.activeQuiz(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	return s.quizView(ctx, quiz, lesson.Title, userID)
}

// SubmitQuiz menilai jawaban peserta dan menyimpan percobaan kuis.
func (s *LearningPathService) SubmitQuiz(ctx context.Context, userID, lessonID string, req dto.SubmitQuizRequest) (*dto.QuizResultView, error) {
	lesson, err := s.classroom.FindLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.access.RequireLesson(ctx, userID, *lesson); err != nil {
		return nil, err
	}

	quiz, err := s.activeQuiz(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	used, err := s.finishedAttempts(ctx, quiz.ID, userID)
	if err != nil {
		return nil, err
	}
	if !quiz.AttemptsAllowed(used) {
		return nil, apperror.Conflict("Jatah percobaan kuis ini sudah habis.")
	}

	questions, _, err := s.quizQuestions(ctx, *quiz)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return nil, apperror.BadRequest("Kuis belum siap dikerjakan. Hubungi tim akademik.")
	}

	answers := normalizeAnswers(req.Answers)
	correct, review := gradeQuiz(questions, answers)
	score := quiz.ScoreFor(correct, len(questions))
	passed := quiz.PassedWith(score)

	now := time.Now().UTC()
	attempt := &models.QuizAttempt{
		IDBase:     models.IDBase{ID: models.NewID()},
		QuizID:     quiz.ID,
		UserID:     userID,
		Score:      score,
		Correct:    correct,
		Total:      len(questions),
		Passed:     passed,
		Answers:    models.JSONList[models.QuizAnswer](answers),
		StartedAt:  now,
		FinishedAt: &now,
	}
	if err := s.paths.CreateAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	// Kuis yang lulus bisa melengkapi syarat sertifikat sebuah jalur. Kegagalan
	// penerbitan sengaja diabaikan agar hasil kuis tetap tersimpan; peserta bisa
	// memicu ulang lewat POST /learning/paths/{slug}/certificate.
	if passed && s.certificates != nil {
		_ = s.certificates.IssueEligibleForAll(ctx, userID)
	}

	used++
	return &dto.QuizResultView{
		AttemptID:    attempt.ID,
		Score:        score,
		Correct:      correct,
		Total:        len(questions),
		Passed:       passed,
		PassScore:    quiz.PassScore,
		AttemptsUsed: used,
		AttemptsLeft: attemptsLeft(quiz.MaxAttempts, used),
		Review:       review,
	}, nil
}

// activeQuiz mengambil kuis aktif yang dipakai peserta pada sebuah pelajaran.
//
// Satu pelajaran bisa memuat beberapa kuis (mis. versi lama & versi baru).
// Yang dipakai adalah kuis aktif **terbaru** (lihat urutan pada repository),
// sehingga admin cukup membuat kuis baru untuk menggantikan kuis lama tanpa
// perlu mematikan yang lama lebih dulu.
func (s *LearningPathService) activeQuiz(ctx context.Context, lessonID string) (*models.LessonQuiz, error) {
	quizzes, err := s.paths.ListQuizzesByLesson(ctx, lessonID, true)
	if err != nil {
		return nil, err
	}
	if len(quizzes) == 0 {
		return nil, apperror.NotFound("Belum ada kuis untuk pelajaran ini.")
	}
	return &quizzes[0], nil
}

// quizView membentuk tampilan kuis untuk peserta (tanpa kunci jawaban).
func (s *LearningPathService) quizView(ctx context.Context, quiz *models.LessonQuiz, lessonTitle, userID string) (*dto.LessonQuizView, error) {
	used, err := s.finishedAttempts(ctx, quiz.ID, userID)
	if err != nil {
		return nil, err
	}
	canAttempt := quiz.AttemptsAllowed(used)

	view := &dto.LessonQuizView{
		ID:           quiz.ID,
		LessonID:     quiz.LessonID,
		LessonTitle:  lessonTitle,
		Title:        quiz.Title,
		PassScore:    quiz.PassScore,
		MaxAttempts:  quiz.MaxAttempts,
		TimeLimit:    quiz.TimeLimit,
		AttemptsUsed: used,
		AttemptsLeft: attemptsLeft(quiz.MaxAttempts, used),
		CanAttempt:   canAttempt,
		Questions:    []dto.QuizQuestionView{},
	}

	// Soal hanya dikirim bila peserta masih boleh mengerjakan. Dengan begitu
	// jatah percobaan yang habis tidak bisa dipakai untuk memanen soal berulang.
	if canAttempt {
		questions, broken, err := s.quizQuestions(ctx, *quiz)
		if err != nil {
			return nil, err
		}
		view.NeedsReview = broken > 0
		for index, question := range questions {
			view.Questions = append(view.Questions, dto.QuizQuestionView{
				ID:      question.ID,
				Number:  index + 1,
				Text:    question.Text,
				Options: question.Options,
			})
		}
	} else {
		broken, err := s.paths.QuizHasBrokenQuestions(ctx, quiz.ID)
		if err != nil {
			return nil, err
		}
		view.NeedsReview = broken
	}

	last, err := s.lastAttempt(ctx, quiz.ID, userID)
	if err != nil {
		return nil, err
	}
	view.LastAttempt = last
	return view, nil
}

// lastAttempt mengambil percobaan terakhir peserta yang sudah dikumpulkan.
func (s *LearningPathService) lastAttempt(ctx context.Context, quizID, userID string) (*dto.QuizAttemptSummaryView, error) {
	attempts, err := s.paths.ListAttemptsByUser(ctx, quizID, userID, 1)
	if err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		return nil, nil
	}
	return attemptSummary(attempts[0]), nil
}

// finishedAttempts menghitung percobaan yang sudah dikumpulkan peserta.
func (s *LearningPathService) finishedAttempts(ctx context.Context, quizID, userID string) (int, error) {
	total, err := s.paths.CountFinishedAttempts(ctx, quizID, userID)
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

// quizQuestions memuat soal kuis berurutan beserta jumlah posisi bermasalah.
//
// Sebuah posisi dihitung bermasalah bila soalnya sudah tidak ada, tidak aktif,
// atau **tidak punya izin aktif** (penegakan Fase 0). Soal bermasalah tidak
// dikirim ke peserta — peserta tidak boleh dirugikan oleh soal yang tidak layak
// dipakai — dan kuisnya ditandai perlu ditinjau.
func (s *LearningPathService) quizQuestions(ctx context.Context, quiz models.LessonQuiz) ([]quizQuestion, int, error) {
	positions := make([]*string, 0, len(quiz.Questions))
	ids := make([]string, 0, len(quiz.Questions))
	for _, slot := range quiz.Questions {
		positions = append(positions, slot.QuestionID)
		if slot.QuestionID != nil && strings.TrimSpace(*slot.QuestionID) != "" {
			ids = append(ids, *slot.QuestionID)
		}
	}

	questions, err := s.tryouts.ListQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	byID := make(map[string]models.Question, len(questions))
	for _, question := range questions {
		byID[question.ID] = question
	}

	usable, err := s.licenses.UsableQuestionIDs(ctx, ids, time.Now().UTC())
	if err != nil {
		return nil, 0, err
	}

	out := make([]quizQuestion, 0, len(positions))
	broken := 0
	for _, questionID := range positions {
		if questionID == nil {
			broken++
			continue
		}
		question, ok := byID[*questionID]
		if !ok || !question.Active {
			broken++
			continue
		}
		if _, ok := usable[*questionID]; !ok {
			broken++
			continue
		}
		out = append(out, quizQuestion{
			ID:          question.ID,
			Text:        question.Text,
			Options:     question.Options,
			AnswerIndex: question.AnswerIndex,
			Explanation: question.Explanation,
		})
	}
	return out, broken, nil
}

// normalizeAnswers merapikan jawaban peserta: duplikat diambil yang terakhir
// (jawaban terbaru menang). Jawaban tanpa pilihan tetap disimpan sebagai "tidak
// dijawab" sehingga penilaian mencakup seluruh soal kuis.
func normalizeAnswers(inputs []dto.QuizAnswerInput) []models.QuizAnswer {
	index := map[string]int{}
	out := make([]models.QuizAnswer, 0, len(inputs))
	for _, input := range inputs {
		questionID := strings.TrimSpace(input.QuestionID)
		if questionID == "" {
			continue
		}
		answer := models.QuizAnswer{QuestionID: questionID, SelectedIndex: input.SelectedIndex}
		if position, ok := index[questionID]; ok {
			out[position] = answer
			continue
		}
		index[questionID] = len(out)
		out = append(out, answer)
	}
	return out
}

// gradeQuiz menilai jawaban peserta dan menyusun pembahasan.
//
// Soal yang tidak dijawab tetap muncul di pembahasan supaya peserta melihat kunci
// jawabannya, dan pembagi nilai adalah jumlah soal kuis (bukan jumlah jawaban
// yang dikirim) agar nilai tidak bisa digelembungkan dengan mengirim jawaban
// sebagian.
func gradeQuiz(questions []quizQuestion, answers []models.QuizAnswer) (int, []dto.QuizAnswerReview) {
	selected := make(map[string]*int, len(answers))
	for _, answer := range answers {
		selected[answer.QuestionID] = answer.SelectedIndex
	}

	correct := 0
	review := make([]dto.QuizAnswerReview, 0, len(questions))
	for index, question := range questions {
		choice := selected[question.ID]
		isCorrect := choice != nil && *choice == question.AnswerIndex
		if isCorrect {
			correct++
		}
		review = append(review, dto.QuizAnswerReview{
			QuestionID:    question.ID,
			Number:        index + 1,
			Text:          question.Text,
			Options:       question.Options,
			SelectedIndex: choice,
			AnswerIndex:   question.AnswerIndex,
			Correct:       isCorrect,
			Explanation:   question.Explanation,
		})
	}
	return correct, review
}

// attemptsLeft menghitung sisa percobaan; -1 berarti tanpa batas.
func attemptsLeft(maxAttempts, used int) int {
	if maxAttempts <= 0 {
		return -1
	}
	left := maxAttempts - used
	if left < 0 {
		return 0
	}
	return left
}

// attemptSummary membentuk ringkasan satu percobaan.
func attemptSummary(attempt models.QuizAttempt) *dto.QuizAttemptSummaryView {
	view := &dto.QuizAttemptSummaryView{
		ID:        attempt.ID,
		Score:     attempt.Score,
		Correct:   attempt.Correct,
		Total:     attempt.Total,
		Passed:    attempt.Passed,
		StartedAt: attempt.StartedAt.Format(time.RFC3339),
	}
	if attempt.FinishedAt != nil {
		view.FinishedAt = attempt.FinishedAt.Format(time.RFC3339)
	}
	return view
}

/* ------------------------------- kuis (admin) ------------------------------ */

// AdminQuizzes mengambil kuis sebuah pelajaran untuk panel admin.
//
// Termasuk kuis nonaktif: admin perlu melihat & menghidupkan kembali kuis yang
// pernah dimatikan, bukan hanya yang sedang tayang.
func (s *LearningPathService) AdminQuizzes(ctx context.Context, lessonID string) ([]dto.AdminQuizView, error) {
	lesson, err := s.classroom.FindLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	quizzes, err := s.paths.ListQuizzesByLesson(ctx, lessonID, false)
	if err != nil {
		return nil, err
	}

	out := make([]dto.AdminQuizView, 0, len(quizzes))
	for _, quiz := range quizzes {
		questionIDs := make([]string, 0, len(quiz.Questions))
		broken := false
		for _, slot := range quiz.Questions {
			if slot.QuestionID == nil {
				broken = true
				continue
			}
			questionIDs = append(questionIDs, *slot.QuestionID)
		}

		view := dto.AdminQuizView{
			LessonQuizView: dto.LessonQuizView{
				ID:          quiz.ID,
				LessonID:    quiz.LessonID,
				LessonTitle: lesson.Title,
				Title:       quiz.Title,
				PassScore:   quiz.PassScore,
				MaxAttempts: quiz.MaxAttempts,
				TimeLimit:   quiz.TimeLimit,
				// Percobaan bersifat per-peserta (lihat rekap), jadi angka sisa
				// percobaan di tampilan admin diisi -1 (tidak berlaku) agar tidak
				// terbaca sebagai "peserta tidak punya percobaan".
				AttemptsLeft: -1,
				NeedsReview:  broken,
				Questions:    []dto.QuizQuestionView{},
			},
			Active:        quiz.Active,
			QuestionCount: len(questionIDs),
			QuestionIDs:   questionIDs,
		}
		out = append(out, view)
	}
	return out, nil
}

// Batas/default kuis yang ditetapkan service saat admin tidak mengisinya.
const (
	defaultQuizPassScore   = 70
	defaultQuizMaxAttempts = 1
)

// quizDefaults menetapkan nilai bawaan kuis.
//
// Default sengaja tidak dipasang lewat tag `default:` model: GORM akan menimpa
// nilai nol yang dikirim klien dengan nilai tag, sehingga `maxAttempts: 0`
// (tanpa batas percobaan) mustahil disimpan.
func quizDefaults(req dto.CreateQuizRequest) (passScore, maxAttempts, timeLimit int) {
	return intOr(req.PassScore, defaultQuizPassScore),
		intOr(req.MaxAttempts, defaultQuizMaxAttempts),
		intOr(req.TimeLimit, 0)
}

// CreateQuiz menambah kuis baru pada sebuah pelajaran.
func (s *LearningPathService) CreateQuiz(ctx context.Context, req dto.CreateQuizRequest) (*dto.AdminQuizView, error) {
	quizID := strings.TrimSpace(req.ID)
	if quizID != "" {
		if _, err := s.paths.FindQuiz(ctx, quizID); err == nil {
			return nil, apperror.Conflict("ID kuis sudah dipakai.")
		} else if !isNotFound(err) {
			return nil, err
		}
	} else {
		quizID = models.NewID()
	}

	lesson, err := s.classroom.FindLesson(ctx, strings.TrimSpace(req.LessonID))
	if err != nil {
		return nil, err
	}
	questionIDs, err := s.validQuizQuestionIDs(ctx, req.QuestionIDs)
	if err != nil {
		return nil, err
	}

	passScore, maxAttempts, timeLimit := quizDefaults(req)
	quiz := &models.LessonQuiz{
		Base:        models.Base{ID: quizID},
		LessonID:    lesson.ID,
		Title:       strings.TrimSpace(req.Title),
		PassScore:   passScore,
		MaxAttempts: maxAttempts,
		TimeLimit:   timeLimit,
		Active:      boolOr(req.Active, true),
	}
	if err := s.paths.UpsertQuiz(ctx, quiz, questionIDs); err != nil {
		return nil, err
	}

	views, err := s.AdminQuizzes(ctx, lesson.ID)
	if err != nil {
		return nil, err
	}
	for index := range views {
		if views[index].ID == quizID {
			return &views[index], nil
		}
	}
	return nil, apperror.Internal(nil, "Kuis tersimpan tetapi gagal dibaca kembali.")
}

// UpdateQuiz mengubah sebagian kuis.
//
// `questionIds` kosong berarti susunan soal dipertahankan; bila diisi, seluruh
// susunan diganti sesuai urutan yang dikirim admin.
func (s *LearningPathService) UpdateQuiz(ctx context.Context, id string, req dto.UpdateQuizRequest) (*dto.AdminQuizView, error) {
	existing, err := s.paths.FindQuiz(ctx, id)
	if err != nil {
		return nil, err
	}

	existing.Title = valueOr(req.Title, existing.Title)
	existing.PassScore = intOr(req.PassScore, existing.PassScore)
	existing.MaxAttempts = intOr(req.MaxAttempts, existing.MaxAttempts)
	existing.TimeLimit = intOr(req.TimeLimit, existing.TimeLimit)
	existing.Active = boolOr(req.Active, existing.Active)

	questionIDs := make([]string, 0, len(existing.Questions))
	for _, slot := range existing.Questions {
		if slot.QuestionID == nil {
			// Posisi yang kehilangan soal (NULL) dibuang saat kuis disimpan ulang —
			// `UpsertQuiz` hanya menerima daftar id soal. Admin menambahkan soal
			// pengganti lewat `questionIds`; sampai itu terjadi, kuis tetap ditandai
			// "perlu ditinjau" dan soal yang tersisa tetap bisa dikerjakan peserta.
			continue
		}
		questionIDs = append(questionIDs, *slot.QuestionID)
	}
	if req.QuestionIDs != nil {
		questionIDs, err = s.validQuizQuestionIDs(ctx, *req.QuestionIDs)
		if err != nil {
			return nil, err
		}
	}

	if err := s.paths.UpsertQuiz(ctx, existing, questionIDs); err != nil {
		return nil, err
	}

	views, err := s.AdminQuizzes(ctx, existing.LessonID)
	if err != nil {
		return nil, err
	}
	for index := range views {
		if views[index].ID == id {
			return &views[index], nil
		}
	}
	return nil, apperror.Internal(nil, "Kuis tersimpan tetapi gagal dibaca kembali.")
}

// SetQuizActive mengaktifkan/menonaktifkan kuis.
func (s *LearningPathService) SetQuizActive(ctx context.Context, id string, active bool) error {
	return s.paths.SetQuizActive(ctx, id, active)
}

// QuizAttempts mengambil rekap percobaan peserta pada satu kuis.
func (s *LearningPathService) QuizAttempts(ctx context.Context, quizID string, limit int) ([]dto.QuizAttemptView, error) {
	if _, err := s.paths.FindQuiz(ctx, quizID); err != nil {
		return nil, err
	}

	rows, err := s.paths.ListAttemptsByQuiz(ctx, quizID, limit)
	if err != nil {
		return nil, err
	}

	out := make([]dto.QuizAttemptView, 0, len(rows))
	for _, row := range rows {
		view := dto.QuizAttemptView{
			ID:        row.ID,
			UserID:    row.UserID,
			UserName:  row.Name,
			Initials:  initialsOf(row.Name),
			Email:     row.Email,
			Score:     row.Score,
			Correct:   row.Correct,
			Total:     row.Total,
			Passed:    row.Passed,
			StartedAt: row.StartedAt.Format(time.RFC3339),
		}
		if row.FinishedAt != nil {
			view.FinishedAt = row.FinishedAt.Format(time.RFC3339)
		}
		out = append(out, view)
	}
	return out, nil
}

// validQuizQuestionIDs memvalidasi soal yang dipilih admin.
//
// Kuis **fail-closed**: soal tanpa izin aktif tidak boleh masuk kuis, karena
// peserta adalah pihak yang dirugikan bila soal berlisensi dicabut di tengah
// jalan. Duplikat dirapikan (urutan pertama dipertahankan).
func (s *LearningPathService) validQuizQuestionIDs(ctx context.Context, inputs []string) ([]string, error) {
	ids := make([]string, 0, len(inputs))
	seen := map[string]struct{}{}
	for _, input := range inputs {
		id := strings.TrimSpace(input)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, apperror.Validation("Kuis wajib memuat minimal satu soal.", nil)
	}

	questions, err := s.tryouts.ListQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	known := make(map[string]models.Question, len(questions))
	for _, question := range questions {
		known[question.ID] = question
	}

	usable, err := s.licenses.UsableQuestionIDs(ctx, ids, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	missing := make([]string, 0, len(ids))
	unlicensed := make([]string, 0, len(ids))
	inactive := make([]string, 0, len(ids))
	for _, id := range ids {
		question, ok := known[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		if !question.Active {
			inactive = append(inactive, id)
			continue
		}
		if _, ok := usable[id]; !ok {
			unlicensed = append(unlicensed, id)
		}
	}

	details := map[string]string{}
	if len(missing) > 0 {
		details["missing"] = strings.Join(missing, ", ")
	}
	if len(inactive) > 0 {
		details["inactive"] = strings.Join(inactive, ", ")
	}
	if len(unlicensed) > 0 {
		details["unlicensed"] = strings.Join(unlicensed, ", ")
	}
	if len(details) > 0 {
		return nil, apperror.Validation(
			"Sebagian soal tidak bisa dipakai pada kuis (tidak ada, nonaktif, atau tanpa izin aktif).",
			details,
		)
	}
	return ids, nil
}
