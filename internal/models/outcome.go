package models

import (
	"strings"
	"time"
)

/* --------------------------- referensi tahapan ------------------------------ */

// SelectionStage adalah satu tahap seleksi BUMN (Fase 2 §2.2).
//
// Tahapan disimpan sebagai data (bukan konstanta di kode) supaya alur seleksi
// yang berubah antar tahun tidak memaksa migrasi baru, dan KPI per tahap selalu
// memakai definisi yang sama untuk semua peserta.
type SelectionStage struct {
	Key       string `gorm:"column:key;primaryKey" json:"key"`
	Label     string `gorm:"column:label;not null" json:"label"`
	Phase     int    `gorm:"column:phase;not null" json:"phase"`
	IsFinal   bool   `gorm:"column:is_final;not null" json:"isFinal"`
	SortOrder int    `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	// Active tanpa tag `default:` — tahap yang dinonaktifkan (`false`) harus
	// benar-benar tersimpan, bukan ditimpa nilai tag saat INSERT.
	Active bool `gorm:"column:active;not null" json:"active"`
}

// TableName menetapkan nama tabel.
func (SelectionStage) TableName() string { return "selection_stages" }

/* ---------------------------- hasil seleksi peserta ------------------------ */

// OutcomeStatus adalah status hasil seleksi pada satu tahap.
type OutcomeStatus string

const (
	// OutcomeLolos peserta dinyatakan lolos tahap itu.
	OutcomeLolos OutcomeStatus = "lolos"
	// OutcomeGagal peserta tidak lolos tahap itu.
	OutcomeGagal OutcomeStatus = "gagal"
	// OutcomeMenunggu pengumuman belum keluar (tidak dihitung sebagai lolos).
	OutcomeMenunggu OutcomeStatus = "menunggu"
	// OutcomeMengundurkanDiri peserta mundur dari proses seleksi.
	OutcomeMengundurkanDiri OutcomeStatus = "mengundurkan_diri"
)

// Valid memeriksa apakah status dikenal.
func (s OutcomeStatus) Valid() bool {
	switch s {
	case OutcomeLolos, OutcomeGagal, OutcomeMenunggu, OutcomeMengundurkanDiri:
		return true
	default:
		return false
	}
}

// Label mengembalikan label status untuk UI.
func (s OutcomeStatus) Label() string {
	switch s {
	case OutcomeLolos:
		return "Lolos"
	case OutcomeGagal:
		return "Belum lolos"
	case OutcomeMenunggu:
		return "Menunggu pengumuman"
	case OutcomeMengundurkanDiri:
		return "Mengundurkan diri"
	default:
		return string(s)
	}
}

// Passed menandai status yang dihitung sebagai kelulusan pada tahap itu.
//
// `menunggu` & `mengundurkan_diri` sengaja bukan kelulusan: keduanya bukan hasil
// final, sehingga memasukkan salah satunya akan menaikkan angka KPI secara palsu.
func (s OutcomeStatus) Passed() bool { return s == OutcomeLolos }

// Counted menandai status yang ikut menjadi penyebut tingkat kelulusan tahap.
//
// Hanya peserta yang **mengikuti** tahap itu (lolos/gagal) yang dihitung, sesuai
// §4.3: peserta yang belum ada pengumuman tidak menekan angka kelulusan.
func (s OutcomeStatus) Counted() bool { return s == OutcomeLolos || s == OutcomeGagal }

// OutcomeSource adalah tingkat kepercayaan sebuah laporan.
type OutcomeSource string

const (
	// OutcomeSelfReported dilaporkan peserta, belum diperiksa admin.
	OutcomeSelfReported OutcomeSource = "self_reported"
	// OutcomeAdminVerified sudah diverifikasi admin.
	OutcomeAdminVerified OutcomeSource = "admin_verified"
	// OutcomeDocumentVerified diverifikasi admin dengan bukti dokumen pengumuman.
	OutcomeDocumentVerified OutcomeSource = "document_verified"
)

// Valid memeriksa apakah sumber laporan dikenal.
func (s OutcomeSource) Valid() bool {
	switch s {
	case OutcomeSelfReported, OutcomeAdminVerified, OutcomeDocumentVerified:
		return true
	default:
		return false
	}
}

// Label mengembalikan label kualitas data untuk lencana UI (§6.3).
func (s OutcomeSource) Label() string {
	switch s {
	case OutcomeAdminVerified:
		return "Terverifikasi"
	case OutcomeDocumentVerified:
		return "Terverifikasi (dokumen)"
	default:
		return "Dilaporkan sendiri"
	}
}

// Verified menandai laporan yang sudah diperiksa admin.
func (s OutcomeSource) Verified() bool {
	return s == OutcomeAdminVerified || s == OutcomeDocumentVerified
}

// IsDocumentEvidence menandai bukti berupa berkas dokumen (PDF), sehingga
// verifikasi admin dapat dinaikkan ke `document_verified` (§4.2 butir 3).
func IsDocumentEvidence(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	return lower != "" && strings.HasSuffix(lower, ".pdf")
}

// SelectionOutcome adalah laporan hasil seleksi seorang peserta pada satu tahap.
type SelectionOutcome struct {
	Base
	UserID string `gorm:"column:user_id;not null" json:"userId"`
	// CohortID diisi dari keanggotaan kelas saat laporan dibuat (Fase 1); nil
	// berarti peserta bukan anggota kelas khusus mana pun.
	CohortID       *string       `gorm:"column:cohort_id" json:"cohortId,omitempty"`
	StageKey       string        `gorm:"column:stage_key;not null" json:"stageKey"`
	CompanyName    string        `gorm:"column:company_name;not null;default:''" json:"companyName"`
	CompanySector  string        `gorm:"column:company_sector;not null;default:umum" json:"companySector"`
	Position       string        `gorm:"column:position;not null;default:''" json:"position"`
	PositionFamily string        `gorm:"column:position_family;not null;default:umum" json:"positionFamily"`
	SelectionYear  int           `gorm:"column:selection_year;not null" json:"selectionYear"`
	Status         OutcomeStatus `gorm:"column:status;not null" json:"status"`
	AnnouncedAt    *time.Time    `gorm:"column:announced_at;type:date" json:"announcedAt,omitempty"`
	Source         OutcomeSource `gorm:"column:source;not null" json:"source"`
	EvidenceURL    *string       `gorm:"column:evidence_url" json:"evidenceUrl,omitempty"`
	Note           string        `gorm:"column:note;not null;default:''" json:"note"`
	// AggregateConsent tanpa tag `default:` — peserta yang mematikan persetujuan
	// (`false`) harus benar-benar tersimpan.
	AggregateConsent bool       `gorm:"column:aggregate_consent;not null" json:"aggregateConsent"`
	VerifiedBy       *string    `gorm:"column:verified_by" json:"verifiedBy,omitempty"`
	VerifiedAt       *time.Time `gorm:"column:verified_at" json:"verifiedAt,omitempty"`
}

// TableName menetapkan nama tabel.
func (SelectionOutcome) TableName() string { return "selection_outcomes" }

// Verified menandai laporan sudah diperiksa admin.
func (o SelectionOutcome) Verified() bool { return o.Source.Verified() }

/* ------------------------------ jejak audit KPI ---------------------------- */

// Aksi yang dicatat pada jejak audit hasil seleksi (§4.2 butir 5).
const (
	SelectionActionReported = "outcome_reported"
	SelectionActionUpdated  = "outcome_updated"
	SelectionActionDeleted  = "outcome_deleted"
	SelectionActionVerified = "outcome_verified"
	SelectionActionRejected = "outcome_rejected"
)

// SelectionOutcomeAudit mencatat siapa mengubah/memverifikasi laporan mana.
//
// Tanpa catatan ini, perubahan angka KPI (mis. satu laporan yang diverifikasi
// lalu ditolak) tidak bisa ditelusuri saat angkanya dipertanyakan instansi.
type SelectionOutcomeAudit struct {
	// IDBase: tabel ini hanya punya `created_at`, bukan created/updated.
	IDBase
	OutcomeID string    `gorm:"column:outcome_id;not null" json:"outcomeId"`
	ActorID   *string   `gorm:"column:actor_id" json:"actorId,omitempty"`
	Action    string    `gorm:"column:action;not null" json:"action"`
	Note      string    `gorm:"column:note;not null;default:''" json:"note"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (SelectionOutcomeAudit) TableName() string { return "selection_outcome_audits" }

/* -------------------------------- target KPI ------------------------------- */

// KPIMetric adalah jenis target KPI yang bisa ditetapkan admin (§4.3).
type KPIMetric string

const (
	// KPIMetricFinalPassRate target tingkat kelulusan akhir.
	KPIMetricFinalPassRate KPIMetric = "final_pass_rate"
	// KPIMetricStagePassRate target tingkat kelulusan satu tahap.
	KPIMetricStagePassRate KPIMetric = "stage_pass_rate"
	// KPIMetricParticipants target jumlah peserta yang melapor.
	KPIMetricParticipants KPIMetric = "participants"
	// KPIMetricCurriculumCompletion target kelengkapan kurikulum peserta lolos.
	KPIMetricCurriculumCompletion KPIMetric = "curriculum_completion_rate"
)

// Valid memeriksa apakah metric dikenal.
func (m KPIMetric) Valid() bool {
	switch m {
	case KPIMetricFinalPassRate, KPIMetricStagePassRate, KPIMetricParticipants, KPIMetricCurriculumCompletion:
		return true
	default:
		return false
	}
}

// Label mengembalikan label metric untuk UI.
func (m KPIMetric) Label() string {
	switch m {
	case KPIMetricFinalPassRate:
		return "Tingkat kelulusan akhir"
	case KPIMetricStagePassRate:
		return "Tingkat kelulusan per tahap"
	case KPIMetricParticipants:
		return "Jumlah peserta melapor"
	case KPIMetricCurriculumCompletion:
		return "Kelengkapan kurikulum peserta lolos"
	default:
		return string(m)
	}
}

// KPITarget adalah target KPI untuk sebuah kelompok pada satu periode.
//
// Kelompok ditentukan pasangan `OrgID`/`CohortID`; keduanya nil berarti target
// lintas program (semua peserta).
type KPITarget struct {
	Base
	OrgID      *string   `gorm:"column:org_id" json:"orgId,omitempty"`
	CohortID   *string   `gorm:"column:cohort_id" json:"cohortId,omitempty"`
	PeriodYear int       `gorm:"column:period_year;not null" json:"periodYear"`
	Metric     KPIMetric `gorm:"column:metric;not null" json:"metric"`
	StageKey   *string   `gorm:"column:stage_key" json:"stageKey,omitempty"`
	// TargetValue memakai NUMERIC(6,2) di database: persentase 0–100 atau jumlah.
	TargetValue float64 `gorm:"column:target_value;type:numeric(6,2);not null" json:"targetValue"`
	Note        string  `gorm:"column:note;not null;default:''" json:"note"`
}

// TableName menetapkan nama tabel.
func (KPITarget) TableName() string { return "kpi_targets" }
