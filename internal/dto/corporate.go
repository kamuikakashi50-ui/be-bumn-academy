package dto

/* --------------------- insight agregat (B2B / HR BUMN) -------------------- */

// CorporateInsightsView adalah ringkasan agregat kohort peserta untuk halaman
// "Solusi Korporasi".
//
// Seluruh angka bersifat agregat: tidak ada nama, email, atau ID peserta yang
// ikut dikirim. Nama pada daftar skor teratas sudah disamarkan di server.
type CorporateInsightsView struct {
	TotalPeserta      int64            `json:"totalPeserta"`
	ActivePeserta     int64            `json:"activePeserta"`
	TotalAttempts     int64            `json:"totalAttempts"`
	DistinctTryouts   int64            `json:"distinctTryouts"`
	AverageScore      float64          `json:"averageScore"`
	TopScore          int              `json:"topScore"`
	PassingRate       float64          `json:"passingRate"`
	PassingThreshold  int              `json:"passingThreshold"`
	ScoreDistribution []BucketCount    `json:"scoreDistribution"`
	SkillProfile      []SkillScoreView `json:"skillProfile"`
	TopPerformers     []PerformerView  `json:"topPerformers"`
	MonthlyTrend      []MonthlyPoint   `json:"monthlyTrend"`
	GeneratedAt       string           `json:"generatedAt"`
}

// SkillScoreView adalah rata-rata skor satu kategori/subtes kohort.
type SkillScoreView struct {
	Name         string  `json:"name"`
	AverageScore float64 `json:"averageScore"`
	Attempts     int64   `json:"attempts"`
}

// PerformerView adalah satu baris skor teratas dengan identitas disamarkan.
type PerformerView struct {
	Rank            int    `json:"rank"`
	DisplayName     string `json:"displayName"`
	Initials        string `json:"initials"`
	TryoutTitle     string `json:"tryoutTitle"`
	Score           int    `json:"score"`
	Correct         int    `json:"correct"`
	Total           int    `json:"total"`
	DurationSeconds int    `json:"durationSeconds"`
	TakenAt         string `json:"takenAt"`
}

// MonthlyPoint adalah satu titik tren bulanan (jumlah percobaan & rata-rata skor).
type MonthlyPoint struct {
	Label        string  `json:"label"`
	Attempts     int64   `json:"attempts"`
	AverageScore float64 `json:"averageScore"`
}
