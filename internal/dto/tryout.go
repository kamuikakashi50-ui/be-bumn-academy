package dto

/* ------------------------------- katalog -------------------------------- */

// SubtestView adalah ringkasan satu subtes pada paket tryout.
type SubtestView struct {
	Name          string   `json:"name"`
	Banks         []string `json:"banks"`
	Minutes       int      `json:"minutes"`
	QuestionCount int      `json:"questionCount"`
}

// TryoutSummary adalah satu baris pada daftar tryout.
//
// Field akses (unlocked/attemptsUsed/attemptsLeft) hanya bermakna bila pemanggil
// sudah login; untuk tamu, `unlocked` dihitung dari tier gratis.
type TryoutSummary struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Tier            string        `json:"tier"`
	RequiredProduct *string       `json:"requiredProduct"`
	Attempts        int           `json:"attempts"`
	AccessUntil     string        `json:"accessUntil"`
	Subtests        []SubtestView `json:"subtests"`
	QuestionCount   int           `json:"questionCount"`
	Minutes         int           `json:"minutes"`
	Unlocked        bool          `json:"unlocked"`
	AttemptsUsed    int           `json:"attemptsUsed"`
	AttemptsLeft    int           `json:"attemptsLeft"`
	LockedReason    string        `json:"lockedReason,omitempty"`
	ActiveSessionID string        `json:"activeSessionId,omitempty"`
}

// TryoutDetail adalah detail satu paket tryout beserta aturan pengerjaannya.
type TryoutDetail struct {
	TryoutSummary
	Rules []string `json:"rules"`
}

/* ---------------------------- sesi tryout ------------------------------- */

// SessionQuestionView adalah soal yang dikirim saat sesi berjalan.
//
// Sengaja TIDAK memuat `answerIndex` dan `explanation`: kunci jawaban baru
// dibuka pada endpoint review setelah submit.
type SessionQuestionView struct {
	ID              string   `json:"id"`
	Number          int      `json:"number"`
	NumberInSubtest int      `json:"numberInSubtest"`
	Text            string   `json:"text"`
	Options         []string `json:"options"`
	SubtestIndex    int      `json:"subtestIndex"`
	SubtestName     string   `json:"subtestName"`
	Flagged         bool     `json:"flagged"`
	SelectedIndex   *int     `json:"selectedIndex"`
}

// SessionSubtestView adalah satu subtes dalam sesi berjalan.
type SessionSubtestView struct {
	Name             string                `json:"name"`
	Minutes          int                   `json:"minutes"`
	Seconds          int                   `json:"seconds"`
	Banks            []string              `json:"banks"`
	Started          bool                  `json:"started"`
	RemainingSeconds int                   `json:"remainingSeconds"`
	Locked           bool                  `json:"locked"`
	AnsweredCount    int                   `json:"answeredCount"`
	QuestionCount    int                   `json:"questionCount"`
	Questions        []SessionQuestionView `json:"questions"`
}

// TryoutSessionView adalah keadaan lengkap sesi tryout.
//
// Semua penghitung waktu berasal dari tenggat yang tersimpan di server
// (`remainingSeconds`), sehingga klien hanya perlu menampilkan hitungan mundur.
type TryoutSessionView struct {
	ID                    string               `json:"id"`
	TryoutID              string               `json:"tryoutId"`
	Title                 string               `json:"title"`
	Status                string               `json:"status"`
	CurrentSubtest        int                  `json:"currentSubtest"`
	StartedAt             string               `json:"startedAt"`
	ExpiresAt             string               `json:"expiresAt"`
	ServerTime            string               `json:"serverTime"`
	TotalQuestions        int                  `json:"totalQuestions"`
	TotalSeconds          int                  `json:"totalSeconds"`
	RemainingTotalSeconds int                  `json:"remainingTotalSeconds"`
	AnsweredTotal         int                  `json:"answeredTotal"`
	AttemptID             string               `json:"attemptId,omitempty"`
	Subtests              []SessionSubtestView `json:"subtests"`
}

// AnswerInput adalah satu jawaban pada payload autosave.
type AnswerInput struct {
	QuestionID    string `json:"questionId" validate:"required"`
	SelectedIndex *int   `json:"selectedIndex" validate:"omitempty,gte=0,lte=9"`
	Flagged       *bool  `json:"flagged"`
}

// SaveAnswersRequest adalah payload autosave (boleh banyak jawaban sekaligus).
type SaveAnswersRequest struct {
	SubtestIndex *int          `json:"subtestIndex" validate:"omitempty,gte=0,lte=20"`
	Answers      []AnswerInput `json:"answers" validate:"required,min=1,max=200,dive"`
}

// SaveAnswersResponse mengembalikan ringkasan setelah autosave.
type SaveAnswersResponse struct {
	SavedCount            int    `json:"savedCount"`
	AnsweredTotal         int    `json:"answeredTotal"`
	RemainingTotalSeconds int    `json:"remainingTotalSeconds"`
	LockedSubtests        []int  `json:"lockedSubtests"`
	ServerTime            string `json:"serverTime"`
}
