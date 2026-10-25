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
