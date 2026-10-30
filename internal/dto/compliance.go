package dto

/* ------------------------- kepatuhan & taksonomi -------------------------- */

// TaxonomyOption adalah satu pilihan taksonomi (kunci + label).
type TaxonomyOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// TaxonomyView adalah daftar pilihan taksonomi yang dipakai panel bank soal.
//
// Dikirim sekali oleh backend supaya label kategori, sektor, jenis tes, dan
// keluarga jabatan tidak perlu diduplikasi di front-end (sumber kebenarannya
// ada di internal/pkg/bank).
type TaxonomyView struct {
	Banks            []TaxonomyOption `json:"banks"`
	Sectors          []TaxonomyOption `json:"sectors"`
	TestTypes        []TaxonomyOption `json:"testTypes"`
	PositionFamilies []TaxonomyOption `json:"positionFamilies"`
	Defaults         TaxonomyDefaults `json:"defaults"`
}

// TaxonomyDefaults adalah nilai bawaan tiap dimensi.
type TaxonomyDefaults struct {
	SectorKey      string `json:"sectorKey"`
	TestType       string `json:"testType"`
	PositionFamily string `json:"positionFamily"`
}

// CheckDuplicateRequest adalah payload pemeriksaan soal kembar.
type CheckDuplicateRequest struct {
	// QuestionID diisi saat memeriksa soal yang sudah ada (soal itu sendiri
	// tidak ikut dihitung sebagai kembarannya).
	QuestionID string `json:"questionId" validate:"omitempty,max=64"`
	Text       string `json:"text" validate:"required,min=5"`
	// BankKey & SectorKey mempersempit pembanding; kosong berarti seluruh bank.
	BankKey      string `json:"bankKey" validate:"omitempty,max=40"`
	SectorKey    string `json:"sectorKey" validate:"omitempty,max=40"`
	VariantGroup string `json:"variantGroup" validate:"omitempty,max=80"`
	// Threshold mengubah ambang kemiripan (0..1); kosong memakai ambang bawaan.
	Threshold *float64 `json:"threshold" validate:"omitempty,gt=0,lt=1"`
}

// DuplicateMatch adalah satu soal yang mirip dengan soal yang diperiksa.
type DuplicateMatch struct {
	QuestionID   string `json:"questionId"`
	Text         string `json:"text"`
	BankKey      string `json:"bankKey"`
	BankLabel    string `json:"bankLabel,omitempty"`
	SectorKey    string `json:"sectorKey"`
	TestType     string `json:"testType"`
	TryoutID     string `json:"tryoutId,omitempty"`
	VariantGroup string `json:"variantGroup,omitempty"`
	// Kind menjelaskan tingkat kemiripan: identik | keluarga | mirip.
	Kind       string  `json:"kind"`
	Similarity float64 `json:"similarity"`
}

// CheckDuplicateResponse adalah hasil pemeriksaan soal kembar.
type CheckDuplicateResponse struct {
	// Kind adalah tingkat kemiripan tertinggi yang ditemukan, atau "unik".
	Kind      string           `json:"kind"`
	Matches   []DuplicateMatch `json:"matches"`
	Threshold float64          `json:"threshold"`
	Scanned   int              `json:"scanned"`
}

// DuplicatePair adalah satu pasangan soal kembar pada laporan kepatuhan.
type DuplicatePair struct {
	Kind       string         `json:"kind"`
	Similarity float64        `json:"similarity"`
	Left       DuplicateMatch `json:"left"`
	Right      DuplicateMatch `json:"right"`
}

// UniquenessReportView adalah laporan soal kembar satu bank soal.
type UniquenessReportView struct {
	Pairs         []DuplicatePair `json:"pairs"`
	Scanned       int             `json:"scanned"`
	Fingerprinted int             `json:"fingerprinted"`
	// Truncated menandai laporan dihentikan karena batas jumlah pasangan.
	Truncated   bool    `json:"truncated"`
	Threshold   float64 `json:"threshold"`
	GeneratedAt string  `json:"generatedAt"`
}

// QuestionAccessLogView adalah satu baris jejak audit bank soal.
type QuestionAccessLogView struct {
	ID         string `json:"id"`
	QuestionID string `json:"questionId"`
	ActorID    string `json:"actorId,omitempty"`
	Action     string `json:"action"`
	Detail     string `json:"detail"`
	CreatedAt  string `json:"createdAt"`
}
