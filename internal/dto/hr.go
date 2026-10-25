package dto

/* --------------------------- analitik HR (people) -------------------------- */

// HRAnalyticsView adalah ringkasan analitik SDM (people analytics) panel admin:
// partisipasi peserta, kelengkapan belajar, kesiapan kompetensi, tren aktivitas,
// sebaran per program, dan daftar peserta yang perlu pendampingan.
//
// Berbeda dari insight korporasi publik (yang menyamarkan identitas), panel ini
// hanya bisa dibuka admin sehingga nama peserta ditampilkan apa adanya.
type HRAnalyticsView struct {
	// Headcount & partisipasi.
	TotalPeserta      int64   `json:"totalPeserta"`
	ActivePeserta     int64   `json:"activePeserta"`
	InactivePeserta   int64   `json:"inactivePeserta"`
	ParticipationRate float64 `json:"participationRate"`
	ActiveWindowDays  int     `json:"activeWindowDays"`

	// Aktivitas latihan & kesiapan kompetensi.
	TotalAttempts    int64   `json:"totalAttempts"`
	AverageScore     float64 `json:"averageScore"`
	TopScore         int     `json:"topScore"`
	PassingThreshold int     `json:"passingThreshold"`
	PassingRate      float64 `json:"passingRate"`
	PracticeHours    float64 `json:"practiceHours"`
	DistinctTryouts  int64   `json:"distinctTryouts"`

	// Kelengkapan belajar (ruang kelas digital).
	TotalLessons     int64   `json:"totalLessons"`
	CompletedLessons int64   `json:"completedLessons"`
	ActiveLearners   int64   `json:"activeLearners"`
	AverageProgress  float64 `json:"averageProgress"`
	CompletionRate   float64 `json:"completionRate"`

	// Rincian.
	ScoreDistribution []BucketCount       `json:"scoreDistribution"`
	SkillProfile      []SkillScoreView    `json:"skillProfile"`
	MonthlyTrend      []MonthlyPoint      `json:"monthlyTrend"`
	ProgramBreakdown  []ProgramBreakdown  `json:"programBreakdown"`
	TopLearners       []TopLearnerView    `json:"topLearners"`
	AtRiskLearners    []AtRiskLearnerView `json:"atRiskLearners"`
	GeneratedAt       string              `json:"generatedAt"`
}

// ProgramBreakdown adalah partisipasi & capaian peserta pada satu program/segmen
// (kolom `users.program`).
type ProgramBreakdown struct {
	Program      string  `json:"program"`
	Peserta      int64   `json:"peserta"`
	Attempts     int64   `json:"attempts"`
	AverageScore float64 `json:"averageScore"`
}

// AtRiskLearnerView adalah peserta yang perlu pendampingan: belum pernah
// mengerjakan tryout atau skor terakhirnya masih di bawah ambang.
type AtRiskLearnerView struct {
	Name         string `json:"name"`
	Initials     string `json:"initials"`
	Email        string `json:"email"`
	Program      string `json:"program"`
	AttemptCount int64  `json:"attemptCount"`
	LastScore    *int   `json:"lastScore,omitempty"`
	LastActivity string `json:"lastActivity,omitempty"`
	Reason       string `json:"reason"`
}

// TopLearnerView adalah peserta berprestasi: pemilik skor terbaik tertinggi,
// diurutkan menurun. Kebalikan dari daftar pendampingan, daftar ini dipakai
// untuk mengapresiasi sekaligus melihat praktik baik yang bisa dibagikan.
type TopLearnerView struct {
	Name         string  `json:"name"`
	Initials     string  `json:"initials"`
	Email        string  `json:"email"`
	Program      string  `json:"program"`
	AttemptCount int64   `json:"attemptCount"`
	AverageScore float64 `json:"averageScore"`
	BestScore    int     `json:"bestScore"`
	// BestTitle adalah judul tryout tempat skor terbaik diperoleh.
	BestTitle    string `json:"bestTitle"`
	LastActivity string `json:"lastActivity,omitempty"`
	Note         string `json:"note"`
}

/* ------------------------------- asisten HR (AI) --------------------------- */

// HRChatHistoryItem adalah satu pesan riwayat percakapan yang dikirim klien.
//
// Asisten HR tidak menyimpan percakapan di server — konteksnya adalah data
// kohort, bukan satu hasil tryout — sehingga kesinambungan percakapan dijaga
// oleh klien dan diverifikasi ulang di server.
type HRChatHistoryItem struct {
	Role    string `json:"role" validate:"required,oneof=user assistant"`
	Content string `json:"content" validate:"required,min=1,max=2000"`
}

// HRChatRequest adalah pertanyaan tim HR pada asisten HR.
type HRChatRequest struct {
	Message string `json:"message" validate:"required,min=2,max=1500"`
	// History dibatasi agar konteks prompt tidak bisa dibanjiri klien.
	History []HRChatHistoryItem `json:"history" validate:"omitempty,max=12,dive"`
}

// HRChatMessageView adalah satu balasan asisten HR.
type HRChatMessageView struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
	LatencyMS int    `json:"latencyMs,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// HRChatResponse adalah balasan asisten HR untuk satu pertanyaan.
type HRChatResponse struct {
	Message     HRChatMessageView `json:"message"`
	Usage       AIUsageView       `json:"usage"`
	Degraded    bool              `json:"degraded"`
	Suggestions []string          `json:"suggestions"`
}
