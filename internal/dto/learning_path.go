package dto

/* ------------------------ jalur belajar (peserta) ------------------------- */

// PathStepView adalah satu langkah (modul) pada jalur belajar beserta progresnya.
type PathStepView struct {
	ModuleID         string `json:"moduleId"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Required         bool   `json:"required"`
	SortOrder        int    `json:"sortOrder"`
	LessonCount      int    `json:"lessonCount"`
	CompletedLessons int    `json:"completedLessons"`
	ProgressPercent  int    `json:"progressPercent"`
	// Completed berarti seluruh pelajaran modul ini sudah selesai.
	Completed bool `json:"completed"`
}

// PathSummaryView adalah ringkasan satu jalur belajar.
type PathSummaryView struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	SectorKey     string `json:"sectorKey"`
	SectorLabel   string `json:"sectorLabel,omitempty"`
	TestType      string `json:"testType"`
	TestTypeLabel string `json:"testTypeLabel,omitempty"`
	ProductSlug   string `json:"productSlug"`
	// CertificateEnabled menandai jalur ini menerbitkan sertifikat saat selesai.
	//
	// Sengaja bukan `certificate`: pada detail jalur, `certificate` dipakai untuk
	// objek sertifikat yang sudah terbit (nama JSON yang sama akan menelan salah satu).
	CertificateEnabled bool `json:"certificateEnabled"`
	// StepCount/RequiredStepCount membedakan modul wajib dan opsional.
	StepCount         int  `json:"stepCount"`
	RequiredStepCount int  `json:"requiredStepCount"`
	CompletedSteps    int  `json:"completedSteps"`
	ProgressPercent   int  `json:"progressPercent"`
	Active            bool `json:"active"`
}

// PathDetailView adalah detail satu jalur belajar beserta langkahnya.
type PathDetailView struct {
	PathSummaryView
	Steps []PathStepView `json:"steps"`
	// NextStepModuleID menunjuk langkah berikutnya yang belum selesai (kosong
	// bila seluruh langkah wajib sudah selesai).
	NextStepModuleID string `json:"nextStepModuleId,omitempty"`
	// EligibleCertificate menandai syarat sertifikat sudah lengkap dan sertifikat
	// belum terbit (peserta bisa memintanya sekarang).
	EligibleCertificate bool `json:"eligibleCertificate"`
	// Certificate adalah sertifikat yang sudah terbit untuk jalur ini (bila ada).
	Certificate *CertificateView `json:"certificate,omitempty"`
	// CertificateEligibility menjelaskan syarat sertifikat yang belum lengkap
	// (mis. kuis yang belum lulus), sehingga peserta tahu apa yang harus dikerjakan.
	// Diisi hanya bila jalur menerbitkan sertifikat dan seluruh langkah wajib selesai.
	CertificateEligibility *CertificateEligibilityView `json:"certificateEligibility,omitempty"`
}

/* --------------------------- kuis (peserta) ------------------------------- */

// QuizQuestionView adalah soal kuis yang dikirim saat peserta mengerjakan.
//
// Sengaja TIDAK memuat kunci jawaban & pembahasan: keduanya baru dibuka setelah
// kuis dikumpulkan (pola yang sama dengan sesi tryout).
type QuizQuestionView struct {
	ID      string   `json:"id"`
	Number  int      `json:"number"`
	Text    string   `json:"text"`
	Options []string `json:"options"`
}

// QuizAttemptSummaryView adalah ringkasan satu percobaan kuis.
type QuizAttemptSummaryView struct {
	ID         string `json:"id"`
	Score      int    `json:"score"`
	Correct    int    `json:"correct"`
	Total      int    `json:"total"`
	Passed     bool   `json:"passed"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

// LessonQuizView adalah kuis satu pelajaran yang siap dikerjakan peserta.
type LessonQuizView struct {
	ID          string `json:"id"`
	LessonID    string `json:"lessonId"`
	LessonTitle string `json:"lessonTitle,omitempty"`
	Title       string `json:"title"`
	PassScore   int    `json:"passScore"`
	MaxAttempts int    `json:"maxAttempts"`
	TimeLimit   int    `json:"timeLimit"`
	// AttemptsLeft bernilai -1 bila percobaan tidak dibatasi.
	AttemptsLeft int `json:"attemptsLeft"`
	AttemptsUsed int `json:"attemptsUsed"`
	// NeedsReview menandai kuis yang sebagian posisinya kehilangan soal (soal
	// dihapus dari bank soal) sehingga perlu ditinjau tim akademik.
	NeedsReview bool `json:"needsReview"`
	// CanAttempt false berarti jatah percobaan habis (peserta tetap bisa melihat
	// riwayat & hasil terakhir).
	CanAttempt  bool                    `json:"canAttempt"`
	Questions   []QuizQuestionView      `json:"questions"`
	LastAttempt *QuizAttemptSummaryView `json:"lastAttempt,omitempty"`
}

// QuizAnswerInput adalah satu jawaban yang dikirim peserta.
type QuizAnswerInput struct {
	QuestionID    string `json:"questionId" validate:"required,max=64"`
	SelectedIndex *int   `json:"selectedIndex" validate:"omitempty,gte=0,lte=9"`
}

// SubmitQuizRequest adalah payload pengumpulan kuis.
type SubmitQuizRequest struct {
	Answers []QuizAnswerInput `json:"answers" validate:"required,min=1,max=100,dive"`
}

// QuizAnswerReview adalah pembahasan satu soal setelah kuis dikumpulkan.
type QuizAnswerReview struct {
	QuestionID    string   `json:"questionId"`
	Number        int      `json:"number"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	SelectedIndex *int     `json:"selectedIndex,omitempty"`
	AnswerIndex   int      `json:"answerIndex"`
	Correct       bool     `json:"correct"`
	Explanation   string   `json:"explanation"`
}

// QuizResultView adalah hasil pengumpulan kuis.
type QuizResultView struct {
	AttemptID    string             `json:"attemptId"`
	Score        int                `json:"score"`
	Correct      int                `json:"correct"`
	Total        int                `json:"total"`
	Passed       bool               `json:"passed"`
	PassScore    int                `json:"passScore"`
	AttemptsUsed int                `json:"attemptsUsed"`
	AttemptsLeft int                `json:"attemptsLeft"`
	Review       []QuizAnswerReview `json:"review"`
}

/* ---------------------------- jalur belajar (admin) ----------------------- */

// PathStepInput adalah satu langkah jalur pada payload admin.
type PathStepInput struct {
	ModuleID string `json:"moduleId" validate:"required,max=64"`
	// Required kosong berarti wajib (bawaan).
	Required *bool `json:"required"`
}

// CreatePathRequest adalah payload tambah jalur belajar.
type CreatePathRequest struct {
	ID          string          `json:"id" validate:"omitempty,max=64"`
	Slug        string          `json:"slug" validate:"required,min=3,max=80"`
	Title       string          `json:"title" validate:"required,min=3,max=180"`
	Description string          `json:"description" validate:"omitempty,max=1000"`
	SectorKey   string          `json:"sectorKey" validate:"omitempty,max=40"`
	TestType    string          `json:"testType" validate:"omitempty,max=40"`
	ProductSlug string          `json:"productSlug" validate:"required,max=80"`
	Certificate *bool           `json:"certificate"`
	SortOrder   int             `json:"sortOrder" validate:"gte=0,lte=9999"`
	Steps       []PathStepInput `json:"steps" validate:"omitempty,max=50,dive"`
	Active      *bool           `json:"active"`
}

// UpdatePathRequest adalah payload ubah sebagian jalur belajar.
type UpdatePathRequest struct {
	Slug        *string `json:"slug" validate:"omitempty,min=3,max=80"`
	Title       *string `json:"title" validate:"omitempty,min=3,max=180"`
	Description *string `json:"description" validate:"omitempty,max=1000"`
	SectorKey   *string `json:"sectorKey" validate:"omitempty,max=40"`
	TestType    *string `json:"testType" validate:"omitempty,max=40"`
	ProductSlug *string `json:"productSlug" validate:"omitempty,max=80"`
	Certificate *bool   `json:"certificate"`
	SortOrder   *int    `json:"sortOrder" validate:"omitempty,gte=0,lte=9999"`
	Active      *bool   `json:"active"`
}

// SetPathStepsRequest adalah payload penyusunan ulang langkah jalur.
type SetPathStepsRequest struct {
	Steps []PathStepInput `json:"steps" validate:"required,max=50,dive"`
}

/* ------------------------------ kuis (admin) ----------------------------- */

// CreateQuizRequest adalah payload tambah kuis pelajaran.
//
// PassScore, MaxAttempts, dan TimeLimit berupa pointer agar "tidak dikirim"
// (default service) bisa dibedakan dari "dikirim 0" — khususnya MaxAttempts 0
// yang berarti tanpa batas percobaan.
type CreateQuizRequest struct {
	ID          string   `json:"id" validate:"omitempty,max=64"`
	LessonID    string   `json:"lessonId" validate:"required,max=64"`
	Title       string   `json:"title" validate:"required,min=3,max=180"`
	PassScore   *int     `json:"passScore" validate:"omitempty,gte=0,lte=100"`
	MaxAttempts *int     `json:"maxAttempts" validate:"omitempty,gte=0,lte=100"`
	TimeLimit   *int     `json:"timeLimit" validate:"omitempty,gte=0,lte=600"`
	QuestionIDs []string `json:"questionIds" validate:"required,min=1,max=100,dive,required,max=64"`
	Active      *bool    `json:"active"`
}

// UpdateQuizRequest adalah payload ubah sebagian kuis.
type UpdateQuizRequest struct {
	Title       *string   `json:"title" validate:"omitempty,min=3,max=180"`
	PassScore   *int      `json:"passScore" validate:"omitempty,gte=0,lte=100"`
	MaxAttempts *int      `json:"maxAttempts" validate:"omitempty,gte=0,lte=100"`
	TimeLimit   *int      `json:"timeLimit" validate:"omitempty,gte=0,lte=600"`
	QuestionIDs *[]string `json:"questionIds" validate:"omitempty,min=1,max=100,dive,required,max=64"`
	Active      *bool     `json:"active"`
}

// AdminQuizView adalah kuis pada panel admin (termasuk status & susunan soal).
type AdminQuizView struct {
	LessonQuizView
	Active        bool     `json:"active"`
	QuestionCount int      `json:"questionCount"`
	QuestionIDs   []string `json:"questionIds"`
}

// QuizAttemptView adalah satu percobaan peserta pada rekap admin.
type QuizAttemptView struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	UserName   string `json:"userName"`
	Initials   string `json:"initials"`
	Email      string `json:"email"`
	Score      int    `json:"score"`
	Correct    int    `json:"correct"`
	Total      int    `json:"total"`
	Passed     bool   `json:"passed"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
}
