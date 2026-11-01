package dto

/* --------------------------- kamus & level -------------------------------- */

// CompetencyLevelView adalah deskriptor satu level kompetensi (1–5).
type CompetencyLevelView struct {
	Level      int    `json:"level"`
	Descriptor string `json:"descriptor"`
}

// CompetencyView adalah satu entri kamus kompetensi beserta levelnya.
type CompetencyView struct {
	Code         string                `json:"code"`
	Name         string                `json:"name"`
	Cluster      string                `json:"cluster"`
	ClusterLabel string                `json:"clusterLabel"`
	Definition   string                `json:"definition"`
	Active       bool                  `json:"active"`
	Levels       []CompetencyLevelView `json:"levels"`
}

// CompetencyFrameworkView adalah kamus lengkap (kompetensi + level + pemetaan bank).
type CompetencyFrameworkView struct {
	Competencies []CompetencyView        `json:"competencies"`
	BankMap      []BankCompetencyMapping `json:"bankMap"`
	Clusters     []string                `json:"clusters"`
}

// BankCompetencyMapping adalah satu pemetaan kategori bank soal → kompetensi.
type BankCompetencyMapping struct {
	BankKey        string  `json:"bankKey"`
	BankLabel      string  `json:"bankLabel"`
	CompetencyCode string  `json:"competencyCode"`
	CompetencyName string  `json:"competencyName"`
	Weight         float64 `json:"weight"`
}

// CompetencyInput adalah payload tambah/ubah satu kompetensi (admin).
type CompetencyInput struct {
	Code       string `json:"code" validate:"required,min=3,max=60"`
	Name       string `json:"name" validate:"required,min=3,max=120"`
	Cluster    string `json:"cluster" validate:"omitempty,oneof=teknis manajerial sosial_kultural"`
	Definition string `json:"definition" validate:"omitempty,max=500"`
	Active     *bool  `json:"active"`
}

// CompetencyLevelInput adalah payload tambah/ubah deskriptor satu level.
type CompetencyLevelInput struct {
	Level      int    `json:"level" validate:"required,gte=1,lte=5"`
	Descriptor string `json:"descriptor" validate:"required,min=3,max=500"`
}

/* ---------------------------- standar jabatan ----------------------------- */

// PositionRequirementView adalah satu standar kompetensi jabatan.
type PositionRequirementView struct {
	PositionFamily    string  `json:"positionFamily"`
	PositionLabel     string  `json:"positionLabel"`
	SectorKey         string  `json:"sectorKey"`
	SectorLabel       string  `json:"sectorLabel"`
	CompetencyCode    string  `json:"competencyCode"`
	CompetencyName    string  `json:"competencyName"`
	CompetencyCluster string  `json:"competencyCluster"`
	MinLevel          int     `json:"minLevel"`
	Weight            float64 `json:"weight"`
}

// PositionRequirementInput adalah payload tambah/ubah standar kompetensi jabatan.
type PositionRequirementInput struct {
	SectorKey      string  `json:"sectorKey" validate:"omitempty,max=60"`
	CompetencyCode string  `json:"competencyCode" validate:"required,max=60"`
	MinLevel       int     `json:"minLevel" validate:"required,gte=1,lte=5"`
	Weight         float64 `json:"weight" validate:"omitempty,gte=0,lte=100"`
}

/* --------------------------- profil peserta ------------------------------- */

// CompetencySourceView adalah satu penilaian kompetensi dari satu sumber.
type CompetencySourceView struct {
	Source      string `json:"source"`
	SourceLabel string `json:"sourceLabel"`
	Level       int    `json:"level"`
	Evidence    string `json:"evidence,omitempty"`
	AssessedAt  string `json:"assessedAt,omitempty"`
}

// CompetencyAssessmentView adalah kompetensi peserta + sumber penilaiannya.
type CompetencyAssessmentView struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Cluster      string `json:"cluster"`
	ClusterLabel string `json:"clusterLabel"`
	// BestLevel adalah level tertinggi lintas sumber (§4.4); 0 berarti belum dinilai.
	BestLevel int                    `json:"bestLevel"`
	HasData   bool                   `json:"hasData"`
	Sources   []CompetencySourceView `json:"sources"`
}

// CompetencyGapView adalah kesenjangan satu kompetensi terhadap standar jabatan.
type CompetencyGapView struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Need      int    `json:"need"`
	Have      int    `json:"have"`
	Gap       int    `json:"gap"`
	Satisfied bool   `json:"satisfied"`
}

// CompetencyProfileView adalah balasan `GET /me/competencies`.
type CompetencyProfileView struct {
	Assessments []CompetencyAssessmentView `json:"assessments"`
	Gap         []CompetencyGapView        `json:"gap"`
	// PositionFamily adalah keluarga jabatan yang dipakai menghitung gap.
	PositionFamily string   `json:"positionFamily"`
	PositionLabel  string   `json:"positionLabel"`
	Notes          []string `json:"notes"`
}

// SelfAssessmentInput adalah payload penilaian diri satu kompetensi.
type SelfAssessmentInput struct {
	CompetencyCode string `json:"competencyCode" validate:"required,max=60"`
	Level          int    `json:"level" validate:"required,gte=1,lte=5"`
	Evidence       string `json:"evidence" validate:"omitempty,max=300"`
}

/* ---------------------------- matriks admin ------------------------------- */

// CompetencyMatrixCellView adalah level satu kompetensi milik satu peserta.
type CompetencyMatrixCellView struct {
	Level       int    `json:"level"`
	Source      string `json:"source"`
	SourceLabel string `json:"sourceLabel"`
}

// CompetencyMatrixRowView adalah satu baris matriks (peserta × kompetensi).
type CompetencyMatrixRowView struct {
	UserID   string                              `json:"userId"`
	Name     string                              `json:"name"`
	Initials string                              `json:"initials"`
	Email    string                              `json:"email"`
	Program  string                              `json:"program"`
	Cells    map[string]CompetencyMatrixCellView `json:"cells"`
	Gap      []CompetencyGapView                 `json:"gap"`
	GapCount int                                 `json:"gapCount"`
}

// CompetencyMatrixView adalah balasan `GET /admin/competency-matrix`.
type CompetencyMatrixView struct {
	Competencies   []CompetencyView          `json:"competencies"`
	Rows           []CompetencyMatrixRowView `json:"rows"`
	PositionFamily string                    `json:"positionFamily"`
	PositionLabel  string                    `json:"positionLabel"`
	TotalPeserta   int                       `json:"totalPeserta"`
	Notes          []string                  `json:"notes"`
}
