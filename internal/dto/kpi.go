package dto

/* ------------------------------ dasbor KPI --------------------------------- */

// KPIRateView adalah satu angka KPI beserta dasar perhitungannya.
//
// `n` selalu dikirim (§6.3): pembaca harus tahu berapa peserta yang mendasari
// angka itu, dan `passed` memungkinkan pemeriksaan silang dengan funnel.
type KPIRateView struct {
	Value  float64 `json:"value"`
	N      int     `json:"n"`
	Passed int     `json:"passed"`
}

// KPIRatePairView memisahkan angka **terverifikasi** dari seluruh laporan.
//
// Tidak ada angka gabungan (§2.3): halaman admin & ekspor selalu menampilkan
// keduanya, sehingga pembaca tahu asal datanya.
type KPIRatePairView struct {
	Verified KPIRateView `json:"verified"`
	Reported KPIRateView `json:"reported"`
}

// KPIStageRateView adalah capaian satu tahap seleksi.
type KPIStageRateView struct {
	StageKey   string          `json:"stageKey"`
	StageLabel string          `json:"stageLabel"`
	IsFinal    bool            `json:"isFinal"`
	Rates      KPIRatePairView `json:"rates"`
}

// KPIBreakdownView adalah capaian satu kelompok (klaster/kelas/program).
type KPIBreakdownView struct {
	Key   string          `json:"key"`
	Label string          `json:"label"`
	Rates KPIRatePairView `json:"rates"`
}

// KPIPracticeView membandingkan kebiasaan latihan peserta lolos vs tidak lolos.
type KPIPracticeView struct {
	PassedAverageScore    float64 `json:"passedAverageScore"`
	FailedAverageScore    float64 `json:"failedAverageScore"`
	PassedAverageAttempts float64 `json:"passedAverageAttempts"`
	FailedAverageAttempts float64 `json:"failedAverageAttempts"`
}

// KPIPracticePairView memisahkan efektivitas latihan per tingkat kualitas data.
type KPIPracticePairView struct {
	Verified KPIPracticeView `json:"verified"`
	Reported KPIPracticeView `json:"reported"`
}

// KPIFunnelStepView adalah satu langkah funnel (jumlah, bukan hanya persentase).
type KPIFunnelStepView struct {
	Stage    string `json:"stage"`
	Label    string `json:"label"`
	Verified int    `json:"verified"`
	Reported int    `json:"reported"`
}

// KPITargetProgressView adalah target KPI beserta realisasinya.
type KPITargetProgressView struct {
	Metric      string  `json:"metric"`
	MetricLabel string  `json:"metricLabel"`
	StageKey    string  `json:"stageKey,omitempty"`
	OrgName     string  `json:"orgName,omitempty"`
	CohortName  string  `json:"cohortName,omitempty"`
	Target      float64 `json:"target"`
	// ActualVerified & ActualReported memakai satuan metric: persentase untuk
	// metric tingkat kelulusan, jumlah untuk metric peserta.
	ActualVerified float64 `json:"actualVerified"`
	ActualReported float64 `json:"actualReported"`
	Achieved       bool    `json:"achieved"`
}

// KPIDashboardSummary adalah ringkasan populasi & tingkat kelulusan akhir.
type KPIDashboardSummary struct {
	TotalParticipants int               `json:"totalParticipants"`
	Reported          int               `json:"reported"`
	Verified          int               `json:"verified"`
	NotReported       int               `json:"notReported"`
	FinalPassRate     KPIRatePairView   `json:"finalPassRate"`
	CurriculumRate    KPIRatePairView   `json:"curriculumCompletionRate"`
	TimeToPassDays    KPITimeToPassView `json:"timeToPassDays"`
}

// KPITimeToPassView adalah median hari dari percobaan pertama sampai pengumuman.
type KPITimeToPassView struct {
	Verified float64 `json:"verified"`
	Reported float64 `json:"reported"`
}

// KPIDashboardPeriod mencatat periode & waktu pembuatan angka (§2.7).
type KPIDashboardPeriod struct {
	Year        int    `json:"year"`
	GeneratedAt string `json:"generatedAt"`
}

// KPIDashboardView adalah balasan `GET /admin/kpi/bumn`.
type KPIDashboardView struct {
	Period           KPIDashboardPeriod      `json:"period"`
	Summary          KPIDashboardSummary     `json:"summary"`
	StagePassRate    []KPIStageRateView      `json:"stagePassRate"`
	SectorBreakdown  []KPIBreakdownView      `json:"sectorBreakdown"`
	CohortBreakdown  []KPIBreakdownView      `json:"cohortBreakdown"`
	ProgramBreakdown []KPIBreakdownView      `json:"programBreakdown"`
	Practice         KPIPracticePairView     `json:"practiceEffectiveness"`
	Funnel           []KPIFunnelStepView     `json:"funnel"`
	Targets          []KPITargetProgressView `json:"targets"`
	// Notes memuat catatan pembacaan (mis. korelasi latihan bukan sebab-akibat).
	Notes []string `json:"notes"`
}

/* --------------------------- ringkasan publik ------------------------------ */

// PublicOutcomeInsightView adalah ringkasan agregat untuk halaman korporasi.
//
// Hanya memuat angka agregat tanpa identitas peserta, dan hanya kelompok dengan
// `n ≥ ambang` (§4.7). Kelompok kecil dilaporkan lewat `HiddenGroups`.
type PublicOutcomeInsightView struct {
	Period       KPIDashboardPeriod       `json:"period"`
	MinGroupSize int                      `json:"minGroupSize"`
	Summary      PublicOutcomeSummaryView `json:"summary"`
	BySector     []PublicOutcomeGroupView `json:"bySector"`
	HiddenGroups int                      `json:"hiddenGroups"`
	Notes        []string                 `json:"notes"`
}

// PublicOutcomeSummaryView adalah angka agregat lintas kelompok.
type PublicOutcomeSummaryView struct {
	Participants   int     `json:"participants"`
	Reported       int     `json:"reported"`
	Verified       int     `json:"verified"`
	FinalPassRate  float64 `json:"finalPassRate"`
	VerifiedRate   float64 `json:"verifiedPassRate"`
	FinalPassCount int     `json:"finalPassCount"`
}

// PublicOutcomeGroupView adalah angka satu kelompok (mis. klaster BUMN).
type PublicOutcomeGroupView struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	N            int     `json:"n"`
	Passed       int     `json:"passed"`
	PassRate     float64 `json:"passRate"`
	VerifiedRate float64 `json:"verifiedPassRate"`
}
