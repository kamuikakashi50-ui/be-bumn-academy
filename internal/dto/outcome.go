package dto

/* --------------------------- referensi tahapan ------------------------------ */

// SelectionStageView adalah satu tahap seleksi BUMN untuk mengisi formulir peserta
// maupun panel admin.
type SelectionStageView struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Phase     int    `json:"phase"`
	IsFinal   bool   `json:"isFinal"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

// SelectionStagePatch adalah payload ubah label/urutan/status aktif tahapan.
type SelectionStagePatch struct {
	Label     *string `json:"label" validate:"omitempty,min=3,max=120"`
	Phase     *int    `json:"phase" validate:"omitempty,gte=1,lte=20"`
	IsFinal   *bool   `json:"isFinal"`
	SortOrder *int    `json:"sortOrder" validate:"omitempty,gte=0,lte=10000"`
	Active    *bool   `json:"active"`
}

/* ---------------------------- hasil seleksi peserta ------------------------ */

// OutcomeView adalah satu laporan hasil seleksi beserta label siap tampil.
//
// `source`/`sourceLabel`/`verified` dikirim eksplisit supaya UI **wajib** bisa
// membedakan angka terverifikasi dari yang dilaporkan sendiri (§2.3), sedangkan
// label taksonomi memakai kunci Fase 0 (`internal/pkg/bank`).
type OutcomeView struct {
	ID                  string `json:"id"`
	UserID              string `json:"userId"`
	UserName            string `json:"userName,omitempty"`
	UserEmail           string `json:"userEmail,omitempty"`
	UserProgram         string `json:"userProgram,omitempty"`
	CohortID            string `json:"cohortId,omitempty"`
	CohortName          string `json:"cohortName,omitempty"`
	StageKey            string `json:"stageKey"`
	StageLabel          string `json:"stageLabel"`
	StageFinal          bool   `json:"stageFinal"`
	CompanyName         string `json:"companyName"`
	CompanySector       string `json:"companySector"`
	CompanySectorLabel  string `json:"companySectorLabel"`
	Position            string `json:"position"`
	PositionFamily      string `json:"positionFamily"`
	PositionFamilyLabel string `json:"positionFamilyLabel"`
	SelectionYear       int    `json:"selectionYear"`
	Status              string `json:"status"`
	StatusLabel         string `json:"statusLabel"`
	AnnouncedAt         string `json:"announcedAt,omitempty"`
	Source              string `json:"source"`
	SourceLabel         string `json:"sourceLabel"`
	Verified            bool   `json:"verified"`
	EvidenceURL         string `json:"evidenceUrl,omitempty"`
	Note                string `json:"note"`
	AggregateConsent    bool   `json:"aggregateConsent"`
	VerifiedAt          string `json:"verifiedAt,omitempty"`
	CreatedAt           string `json:"createdAt,omitempty"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
}

// OutcomeInput adalah payload lapor hasil seleksi (peserta).
type OutcomeInput struct {
	StageKey       string `json:"stageKey" validate:"required,max=60"`
	CompanyName    string `json:"companyName" validate:"omitempty,max=180"`
	CompanySector  string `json:"companySector" validate:"omitempty,max=60"`
	Position       string `json:"position" validate:"omitempty,max=180"`
	PositionFamily string `json:"positionFamily" validate:"omitempty,max=60"`
	SelectionYear  int    `json:"selectionYear" validate:"required,gte=2020,lte=2100"`
	Status         string `json:"status" validate:"required,oneof=lolos gagal menunggu mengundurkan_diri"`
	AnnouncedAt    string `json:"announcedAt" validate:"omitempty"`
	Note           string `json:"note" validate:"omitempty,max=500"`
	// AggregateConsent nil berarti memakai bawaan (aktif) — peserta yang
	// mematikannya harus benar-benar tersimpan sebagai false.
	AggregateConsent *bool `json:"aggregateConsent"`
}

// OutcomePatch adalah payload perbaikan laporan sendiri (semua opsional).
type OutcomePatch struct {
	StageKey         *string `json:"stageKey" validate:"omitempty,max=60"`
	CompanyName      *string `json:"companyName" validate:"omitempty,max=180"`
	CompanySector    *string `json:"companySector" validate:"omitempty,max=60"`
	Position         *string `json:"position" validate:"omitempty,max=180"`
	PositionFamily   *string `json:"positionFamily" validate:"omitempty,max=60"`
	SelectionYear    *int    `json:"selectionYear" validate:"omitempty,gte=2020,lte=2100"`
	Status           *string `json:"status" validate:"omitempty,oneof=lolos gagal menunggu mengundurkan_diri"`
	AnnouncedAt      *string `json:"announcedAt" validate:"omitempty"`
	Note             *string `json:"note" validate:"omitempty,max=500"`
	AggregateConsent *bool   `json:"aggregateConsent"`
}

// OutcomeEvidenceInput adalah payload penautan bukti lewat URL (JSON).
type OutcomeEvidenceInput struct {
	EvidenceURL string `json:"evidenceUrl" validate:"required,url,max=500"`
}

// OutcomeVerifyInput adalah payload verifikasi admin.
//
// `documentVerified` dipakai saat bukti berupa dokumen pengumuman; `evidenceUrl`
// opsional untuk menautkan bukti yang diunggah admin sendiri.
type OutcomeVerifyInput struct {
	DocumentVerified bool   `json:"documentVerified"`
	EvidenceURL      string `json:"evidenceUrl" validate:"omitempty,url,max=500"`
	Note             string `json:"note" validate:"omitempty,max=500"`
}

// OutcomeRejectInput adalah payload penolakan bukti (alasan wajib).
type OutcomeRejectInput struct {
	Note string `json:"note" validate:"required,min=5,max=500"`
}

/* -------------------------------- target KPI ------------------------------- */

// KPITargetView adalah satu target KPI beserta nama kelompoknya.
type KPITargetView struct {
	ID          string  `json:"id"`
	OrgID       string  `json:"orgId,omitempty"`
	OrgName     string  `json:"orgName,omitempty"`
	CohortID    string  `json:"cohortId,omitempty"`
	CohortName  string  `json:"cohortName,omitempty"`
	PeriodYear  int     `json:"periodYear"`
	Metric      string  `json:"metric"`
	MetricLabel string  `json:"metricLabel"`
	StageKey    string  `json:"stageKey,omitempty"`
	TargetValue float64 `json:"targetValue"`
	Note        string  `json:"note"`
	CreatedAt   string  `json:"createdAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt,omitempty"`
}

// KPITargetInput adalah payload tambah/ubah target KPI.
type KPITargetInput struct {
	OrgID       string  `json:"orgId" validate:"omitempty"`
	CohortID    string  `json:"cohortId" validate:"omitempty"`
	PeriodYear  int     `json:"periodYear" validate:"required,gte=2020,lte=2100"`
	Metric      string  `json:"metric" validate:"required,oneof=final_pass_rate stage_pass_rate participants curriculum_completion_rate"`
	StageKey    string  `json:"stageKey" validate:"omitempty,max=60"`
	TargetValue float64 `json:"targetValue" validate:"required,gte=0,lte=100000"`
	Note        string  `json:"note" validate:"omitempty,max=300"`
}

/* ------------------------------ jejak audit KPI ---------------------------- */

// OutcomeAuditView adalah satu baris jejak audit laporan hasil seleksi.
type OutcomeAuditView struct {
	ID        string `json:"id"`
	OutcomeID string `json:"outcomeId"`
	ActorID   string `json:"actorId,omitempty"`
	ActorName string `json:"actorName,omitempty"`
	Action    string `json:"action"`
	Note      string `json:"note,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}
