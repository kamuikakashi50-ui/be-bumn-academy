package models

import "time"

// Instrumen minat–bakat, skoring, dan skor kecocokan (Fase 3 langkah 3–4).

// Dimensi minat memakai kerangka RIASEC/Holland (§2.1).
const (
	TalentDimensionRealistic     = "realistic"
	TalentDimensionInvestigative = "investigative"
	TalentDimensionArtistic      = "artistic"
	TalentDimensionSocial        = "social"
	TalentDimensionEnterprising  = "enterprising"
	TalentDimensionConventional  = "conventional"
)

// Dimensi bakat (aptitude) yang relevan dengan seleksi BUMN.
const (
	TalentDimensionNumerik         = "numerik"
	TalentDimensionVerbal          = "verbal"
	TalentDimensionFigural         = "figural"
	TalentDimensionLearningAgility = "learning_agility"
)

// talentPotentialDimensions adalah dimensi bakat yang menjadi sumbu *potential*
// pada matriks 9-box (§4.4).
var talentPotentialDimensions = []string{
	TalentDimensionNumerik,
	TalentDimensionVerbal,
	TalentDimensionFigural,
	TalentDimensionLearningAgility,
}

// TalentPotentialDimensions mengembalikan salinan daftar dimensi potensi.
func TalentPotentialDimensions() []string {
	out := make([]string, len(talentPotentialDimensions))
	copy(out, talentPotentialDimensions)
	return out
}

// IsTalentPotentialDimension menandai apakah sebuah dimensi termasuk potensi.
func IsTalentPotentialDimension(dimension string) bool {
	for _, item := range talentPotentialDimensions {
		if item == dimension {
			return true
		}
	}
	return false
}

// Kind instrumen.
const (
	TalentKindMinat = "minat"
	TalentKindBakat = "bakat"
)

// Status asesmen peserta.
const (
	TalentAssessmentRunning   = "berjalan"
	TalentAssessmentCompleted = "selesai"
	TalentAssessmentCancelled = "dibatalkan"
)

// Purpose rekaman persetujuan (consent).
const (
	ConsentPurposeAssessment       = "assessment"
	ConsentPurposeConsultation     = "consultation"
	ConsentPurposeHRSharing        = "hr_sharing"
	ConsentPurposeAggregatePublish = "aggregate_publish"
)

// ConsentPurposes adalah seluruh tujuan persetujuan yang dikenali sistem.
//
// Dipakai untuk memvalidasi permintaan penarikan persetujuan (§4.7) supaya
// peserta tidak bisa "menarik" tujuan yang tidak pernah ada.
var ConsentPurposes = []string{
	ConsentPurposeAssessment,
	ConsentPurposeConsultation,
	ConsentPurposeHRSharing,
	ConsentPurposeAggregatePublish,
}

// ConsentPurposeValid menandai tujuan persetujuan yang dikenali sistem.
func ConsentPurposeValid(purpose string) bool {
	for _, item := range ConsentPurposes {
		if item == purpose {
			return true
		}
	}
	return false
}

// ConsentPurposeLabel mengembalikan label tujuan persetujuan siap tampil.
func ConsentPurposeLabel(purpose string) string {
	switch purpose {
	case ConsentPurposeAssessment:
		return "Pemrosesan data asesmen minat–bakat"
	case ConsentPurposeConsultation:
		return "Pemrosesan data konsultasi psikolog"
	case ConsentPurposeHRSharing:
		return "Berbagi ringkasan hasil ke instansi (HR)"
	case ConsentPurposeAggregatePublish:
		return "Publikasi angka agregat (tanpa identitas)"
	default:
		return purpose
	}
}

// TalentInstrument adalah definisi satu instrumen minat–bakat.
type TalentInstrument struct {
	ID      string `gorm:"column:id;primaryKey" json:"id"`
	Code    string `gorm:"column:code;not null" json:"code"`
	Name    string `gorm:"column:name;not null" json:"name"`
	Kind    string `gorm:"column:kind;not null;default:minat" json:"kind"`
	Version string `gorm:"column:version;not null;default:v1" json:"version"`
	// InstrumentID menautkan ke registri instrumen berlisensi (Fase 0) bila ada.
	InstrumentID *string `gorm:"column:instrument_id" json:"instrumentId,omitempty"`
	NormVersion  string  `gorm:"column:norm_version;not null;default:norm-2026" json:"normVersion"`
	Description  string  `gorm:"column:description;not null;default:''" json:"description"`
	Active       bool    `gorm:"column:active;not null;default:true" json:"active"`
}

// TableName menetapkan nama tabel.
func (TalentInstrument) TableName() string { return "talent_instruments" }

// TalentItem adalah satu butir instrumen.
type TalentItem struct {
	IDBase
	InstrumentID string  `gorm:"column:instrument_id;not null" json:"instrumentId"`
	Text         string  `gorm:"column:text;not null" json:"text"`
	Dimension    string  `gorm:"column:dimension;not null" json:"dimension"`
	Weight       float64 `gorm:"column:weight;not null;default:1" json:"weight"`
	Reverse      bool    `gorm:"column:reverse;not null;default:false" json:"reverse"`
	SortOrder    int     `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (TalentItem) TableName() string { return "talent_items" }

// TalentNorm adalah parameter norma satu dimensi pada satu versi norma.
type TalentNorm struct {
	NormVersion string  `gorm:"column:norm_version;primaryKey" json:"normVersion"`
	Dimension   string  `gorm:"column:dimension;primaryKey" json:"dimension"`
	Mean        float64 `gorm:"column:mean;not null" json:"mean"`
	SD          float64 `gorm:"column:sd;not null" json:"sd"`
}

// TableName menetapkan nama tabel.
func (TalentNorm) TableName() string { return "talent_norms" }

// TalentAssessment adalah satu sesi pengerjaan instrumen oleh peserta.
type TalentAssessment struct {
	IDBase
	UserID         string     `gorm:"column:user_id;not null" json:"userId"`
	InstrumentID   string     `gorm:"column:instrument_id;not null" json:"instrumentId"`
	CohortID       *string    `gorm:"column:cohort_id" json:"cohortId,omitempty"`
	Status         string     `gorm:"column:status;not null;default:berjalan" json:"status"`
	AdministeredBy *string    `gorm:"column:administered_by" json:"administeredBy,omitempty"`
	StartedAt      time.Time  `gorm:"column:started_at;not null;default:now()" json:"startedAt"`
	FinishedAt     *time.Time `gorm:"column:finished_at" json:"finishedAt,omitempty"`
	ConsentID      *string    `gorm:"column:consent_id" json:"consentId,omitempty"`
}

// TableName menetapkan nama tabel.
func (TalentAssessment) TableName() string { return "talent_assessments" }

// TalentAnswer adalah jawaban satu butir pada satu asesmen.
type TalentAnswer struct {
	AssessmentID string `gorm:"column:assessment_id;primaryKey" json:"assessmentId"`
	ItemID       string `gorm:"column:item_id;primaryKey" json:"itemId"`
	Value        int    `gorm:"column:value;not null" json:"value"`
}

// TableName menetapkan nama tabel.
func (TalentAnswer) TableName() string { return "talent_answers" }

// TalentScore adalah skor satu dimensi milik peserta (per versi norma).
type TalentScore struct {
	UserID       string    `gorm:"column:user_id;primaryKey" json:"userId"`
	Dimension    string    `gorm:"column:dimension;primaryKey" json:"dimension"`
	Raw          float64   `gorm:"column:raw;not null" json:"raw"`
	Percentile   float64   `gorm:"column:percentile;not null" json:"percentile"`
	NormVersion  string    `gorm:"column:norm_version;primaryKey" json:"normVersion"`
	AssessmentID *string   `gorm:"column:assessment_id" json:"assessmentId,omitempty"`
	ComputedAt   time.Time `gorm:"column:computed_at;not null;default:now()" json:"computedAt"`
}

// TableName menetapkan nama tabel.
func (TalentScore) TableName() string { return "talent_scores" }

// BusinessLineProfile adalah bobot dimensi untuk sebuah lini bisnis/klaster BUMN.
//
// Bobot disimpan sebagai data (JSONB) sehingga klien bisa menyesuaikan tanpa
// mengubah kode (§2.3).
type BusinessLineProfile struct {
	IDBase
	SectorKey   string  `gorm:"column:sector_key;not null" json:"sectorKey"`
	Name        string  `gorm:"column:name;not null" json:"name"`
	Weights     JSONMap `gorm:"column:weights;type:jsonb;not null;default:'{}'" json:"weights"`
	Description string  `gorm:"column:description;not null;default:''" json:"description"`
	OrgID       *string `gorm:"column:org_id" json:"orgId,omitempty"`
	Active      bool    `gorm:"column:active;not null;default:true" json:"active"`
}

// TableName menetapkan nama tabel.
func (BusinessLineProfile) TableName() string { return "business_line_profiles" }

// TalentFitScore adalah skor kecocokan peserta terhadap satu lini bisnis.
type TalentFitScore struct {
	UserID         string    `gorm:"column:user_id;primaryKey" json:"userId"`
	BusinessLineID string    `gorm:"column:business_line_id;primaryKey" json:"businessLineId"`
	Score          float64   `gorm:"column:score;not null" json:"score"`
	ComputedAt     time.Time `gorm:"column:computed_at;not null;default:now()" json:"computedAt"`
}

// TableName menetapkan nama tabel.
func (TalentFitScore) TableName() string { return "talent_fit_scores" }

// ConsentRecord adalah rekaman persetujuan pemrosesan data (UU PDP).
type ConsentRecord struct {
	IDBase
	UserID      string     `gorm:"column:user_id;not null" json:"userId"`
	Purpose     string     `gorm:"column:purpose;not null" json:"purpose"`
	TextVersion string     `gorm:"column:text_version;not null" json:"textVersion"`
	GrantedAt   time.Time  `gorm:"column:granted_at;not null;default:now()" json:"grantedAt"`
	RevokedAt   *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	IPAddress   string     `gorm:"column:ip_address;not null;default:''" json:"ipAddress"`
	UserAgent   string     `gorm:"column:user_agent;not null;default:''" json:"userAgent"`
}

// TableName menetapkan nama tabel.
func (ConsentRecord) TableName() string { return "consent_records" }

// TalentDimensionLabel mengembalikan label dimensi siap tampil.
func TalentDimensionLabel(dimension string) string {
	switch dimension {
	case TalentDimensionRealistic:
		return "Realistic"
	case TalentDimensionInvestigative:
		return "Investigative"
	case TalentDimensionArtistic:
		return "Artistic"
	case TalentDimensionSocial:
		return "Social"
	case TalentDimensionEnterprising:
		return "Enterprising"
	case TalentDimensionConventional:
		return "Conventional"
	case TalentDimensionNumerik:
		return "Numerik"
	case TalentDimensionVerbal:
		return "Verbal"
	case TalentDimensionFigural:
		return "Figural"
	case TalentDimensionLearningAgility:
		return "Learning Agility"
	default:
		return dimension
	}
}

// TalentKindLabel mengembalikan label jenis instrumen.
func TalentKindLabel(kind string) string {
	switch kind {
	case TalentKindMinat:
		return "Minat"
	case TalentKindBakat:
		return "Bakat"
	default:
		return kind
	}
}

// TalentAssessmentStatusLabel mengembalikan label status asesmen.
func TalentAssessmentStatusLabel(status string) string {
	switch status {
	case TalentAssessmentRunning:
		return "Berjalan"
	case TalentAssessmentCompleted:
		return "Selesai"
	case TalentAssessmentCancelled:
		return "Dibatalkan"
	default:
		return status
	}
}
