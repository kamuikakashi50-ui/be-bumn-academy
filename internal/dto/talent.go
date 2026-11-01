package dto

/* --------------------------- instrumen & butir ---------------------------- */

// TalentItemView adalah satu butir instrumen untuk peserta.
//
// `reverse` dan `weight` **tidak** dikirim ke peserta: arah item adalah detail
// penyusunan instrumen, bukan informasi yang perlu diketahui saat menjawab.
type TalentItemView struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Dimension      string `json:"dimension"`
	DimensionLabel string `json:"dimensionLabel"`
	SortOrder      int    `json:"sortOrder"`
}

// TalentInstrumentView adalah satu instrumen beserta status pengerjaan peserta.
type TalentInstrumentView struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	KindLabel   string `json:"kindLabel"`
	Version     string `json:"version"`
	NormVersion string `json:"normVersion"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	ItemCount   int    `json:"itemCount"`
	// Status: belum|berjalan|selesai.
	Status       string `json:"status"`
	StatusLabel  string `json:"statusLabel"`
	AssessmentID string `json:"assessmentId,omitempty"`
}

// TalentAssessmentView adalah sesi asesmen yang sedang dikerjakan.
type TalentAssessmentView struct {
	ID             string           `json:"id"`
	InstrumentID   string           `json:"instrumentId"`
	InstrumentName string           `json:"instrumentName"`
	Status         string           `json:"status"`
	StatusLabel    string           `json:"statusLabel"`
	ItemCount      int              `json:"itemCount"`
	AnsweredCount  int              `json:"answeredCount"`
	Items          []TalentItemView `json:"items"`
}

// TalentAnswerInput adalah jawaban satu butir (1–5).
type TalentAnswerInput struct {
	ItemID string `json:"itemId" validate:"required"`
	Value  int    `json:"value" validate:"required,gte=1,lte=5"`
}

// TalentAnswersInput adalah payload autosave jawaban bertahap.
type TalentAnswersInput struct {
	Answers []TalentAnswerInput `json:"answers" validate:"required,min=1,dive"`
}

// TalentStartInput adalah payload memulai asesmen.
type TalentStartInput struct {
	InstrumentCode string `json:"instrumentCode" validate:"required,max=60"`
	// ConsentGranted menandai peserta menyetujui pemrosesan data asesmen.
	ConsentGranted bool `json:"consentGranted"`
}

/* ---------------------------- profil & kecocokan -------------------------- */

// TalentDimensionView adalah skor satu dimensi beserta persentilnya.
type TalentDimensionView struct {
	Dimension      string  `json:"dimension"`
	DimensionLabel string  `json:"dimensionLabel"`
	Raw            float64 `json:"raw"`
	Percentile     float64 `json:"percentile"`
	NormVersion    string  `json:"normVersion"`
}

// TalentFitView adalah peringkat kecocokan satu lini bisnis.
type TalentFitView struct {
	BusinessLineID string  `json:"businessLineId"`
	SectorKey      string  `json:"sectorKey"`
	SectorLabel    string  `json:"sectorLabel"`
	Name           string  `json:"name"`
	Score          float64 `json:"score"`
}

// TalentProfileView adalah balasan `GET /talent/profile`.
type TalentProfileView struct {
	Dimensions      []TalentDimensionView `json:"dimensions"`
	Fit             []TalentFitView       `json:"fit"`
	NormVersion     string                `json:"normVersion"`
	AssessmentCount int                   `json:"assessmentCount"`
	Notes           []string              `json:"notes"`
}

/* ----------------------------- lini bisnis -------------------------------- */

// BusinessLineView adalah satu profil lini bisnis beserta bobotnya.
type BusinessLineView struct {
	ID          string             `json:"id"`
	SectorKey   string             `json:"sectorKey"`
	SectorLabel string             `json:"sectorLabel"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Weights     map[string]float64 `json:"weights"`
	Active      bool               `json:"active"`
}

// BusinessLineInput adalah payload tambah/ubah profil lini bisnis.
type BusinessLineInput struct {
	SectorKey   string             `json:"sectorKey" validate:"required,max=60"`
	Name        string             `json:"name" validate:"required,min=3,max=120"`
	Description string             `json:"description" validate:"omitempty,max=400"`
	Weights     map[string]float64 `json:"weights" validate:"omitempty"`
	Active      *bool              `json:"active"`
}

// BusinessLinePatch adalah payload ubah sebagian profil lini bisnis (semua opsional).
type BusinessLinePatch struct {
	SectorKey   *string            `json:"sectorKey" validate:"omitempty,max=60"`
	Name        *string            `json:"name" validate:"omitempty,min=3,max=120"`
	Description *string            `json:"description" validate:"omitempty,max=400"`
	Weights     map[string]float64 `json:"weights" validate:"omitempty"`
	Active      *bool              `json:"active"`
}

/* ------------------------------- admin ------------------------------------ */

// TalentInstrumentInput adalah payload tambah/ubah instrumen (admin).
type TalentInstrumentInput struct {
	ID          string `json:"id" validate:"required,min=3,max=60"`
	Code        string `json:"code" validate:"required,min=3,max=60"`
	Name        string `json:"name" validate:"required,min=3,max=160"`
	Kind        string `json:"kind" validate:"required,oneof=minat bakat"`
	Version     string `json:"version" validate:"omitempty,max=20"`
	NormVersion string `json:"normVersion" validate:"omitempty,max=40"`
	Description string `json:"description" validate:"omitempty,max=400"`
	Active      *bool  `json:"active"`
}

// TalentItemInput adalah payload tambah/ubah butir instrumen (admin).
type TalentItemInput struct {
	ID        string  `json:"id" validate:"omitempty,max=60"`
	Text      string  `json:"text" validate:"required,min=5,max=300"`
	Dimension string  `json:"dimension" validate:"required,max=60"`
	Weight    float64 `json:"weight" validate:"omitempty,gte=0,lte=10"`
	Reverse   bool    `json:"reverse"`
	SortOrder int     `json:"sortOrder" validate:"omitempty,gte=0,lte=10000"`
}

// TalentMappingRowView adalah satu baris heatmap peserta × lini bisnis.
type TalentMappingRowView struct {
	UserID       string             `json:"userId"`
	Name         string             `json:"name"`
	Initials     string             `json:"initials"`
	Program      string             `json:"program"`
	Percentiles  map[string]float64 `json:"percentiles"`
	Fit          map[string]float64 `json:"fit"`
	BestLineID   string             `json:"bestLineId,omitempty"`
	BestLineName string             `json:"bestLineName,omitempty"`
	BestScore    float64            `json:"bestScore"`
	HasData      bool               `json:"hasData"`
}

// TalentMappingView adalah balasan `GET /admin/talent/mapping`.
type TalentMappingView struct {
	Dimensions    []TalentDimensionView  `json:"dimensions"`
	BusinessLines []BusinessLineView     `json:"businessLines"`
	Rows          []TalentMappingRowView `json:"rows"`
	NormVersion   string                 `json:"normVersion"`
	Notes         []string               `json:"notes"`
}

// TalentAssessmentSummaryView adalah satu baris riwayat asesmen.
type TalentAssessmentSummaryView struct {
	ID             string `json:"id"`
	InstrumentName string `json:"instrumentName"`
	Status         string `json:"status"`
	StatusLabel    string `json:"statusLabel"`
	StartedAt      string `json:"startedAt,omitempty"`
	FinishedAt     string `json:"finishedAt,omitempty"`
}

// TalentMappingDetailView adalah profil satu peserta untuk admin.
type TalentMappingDetailView struct {
	UserID      string                        `json:"userId"`
	Name        string                        `json:"name"`
	Initials    string                        `json:"initials"`
	Program     string                        `json:"program"`
	Dimensions  []TalentDimensionView         `json:"dimensions"`
	Fit         []TalentFitView               `json:"fit"`
	Assessments []TalentAssessmentSummaryView `json:"assessments"`
	NormVersion string                        `json:"normVersion"`
	Notes       []string                      `json:"notes"`
}

/* ------------------------------- 9-box ------------------------------------ */

// NineBoxMemberView adalah satu peserta di dalam kotak matriks 9-box.
type NineBoxMemberView struct {
	UserID      string  `json:"userId"`
	Name        string  `json:"name"`
	Initials    string  `json:"initials"`
	Program     string  `json:"program"`
	Performance float64 `json:"performance"`
	Potential   float64 `json:"potential"`
	Attempts    int64   `json:"attempts"`
}

// NineBoxCellView adalah satu kotak matriks 9-box beserta saran tindak lanjutnya.
//
// `row`/`column` dipakai klien untuk menempatkan kotak pada grid 3×3
// (baris 0 = potensi tinggi, kolom 0 = kinerja rendah).
type NineBoxCellView struct {
	Key                  string              `json:"key"`
	Label                string              `json:"label"`
	ShortLabel           string              `json:"shortLabel"`
	Action               string              `json:"action"`
	PerformanceBand      string              `json:"performanceBand"`
	PerformanceBandLabel string              `json:"performanceBandLabel"`
	PotentialBand        string              `json:"potentialBand"`
	PotentialBandLabel   string              `json:"potentialBandLabel"`
	Row                  int                 `json:"row"`
	Column               int                 `json:"column"`
	Count                int                 `json:"count"`
	Users                []NineBoxMemberView `json:"users"`
}

// NineBoxThresholdView adalah ambang band yang dipakai pada kedua sumbu.
type NineBoxThresholdView struct {
	LowMax  float64 `json:"lowMax"`
	HighMin float64 `json:"highMin"`
}

// NineBoxSummaryView adalah ringkasan sebaran peserta.
type NineBoxSummaryView struct {
	TotalPeserta int `json:"totalPeserta"`
	WithData     int `json:"withData"`
	Insufficient int `json:"insufficient"`
}

// NineBoxView adalah balasan `GET /admin/talent/nine-box`.
type NineBoxView struct {
	Cells             []NineBoxCellView    `json:"cells"`
	Insufficient      []NineBoxMemberView  `json:"insufficient"`
	Summary           NineBoxSummaryView   `json:"summary"`
	PerformanceMetric string               `json:"performanceMetric"`
	PotentialMetric   string               `json:"potentialMetric"`
	Thresholds        NineBoxThresholdView `json:"thresholds"`
	Notes             []string             `json:"notes"`
}
