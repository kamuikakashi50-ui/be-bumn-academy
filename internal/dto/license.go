package dto

/* ------------------------------- izin soal ------------------------------- */

// CreateLicenseRequest adalah payload tambah izin soal.
type CreateLicenseRequest struct {
	QuestionID     string   `json:"questionId" validate:"required,max=64"`
	Scope          string   `json:"scope" validate:"required,oneof=internal vendor public_domain client_owned"`
	Owner          string   `json:"owner" validate:"omitempty,max=160"`
	AgreementNo    string   `json:"agreementNo" validate:"omitempty,max=80"`
	AgreementURL   string   `json:"agreementUrl" validate:"omitempty,max=300"`
	CommercialUse  *bool    `json:"commercialUse"`
	SelectionUse   *bool    `json:"selectionUse"`
	AllowedSectors []string `json:"allowedSectors" validate:"omitempty,max=13,dive,max=40"`
	ValidFrom      string   `json:"validFrom" validate:"omitempty"`
	ValidUntil     string   `json:"validUntil" validate:"omitempty"`
	Status         string   `json:"status" validate:"omitempty,oneof=aktif kedaluwarsa dicabut"`
	Notes          string   `json:"notes" validate:"omitempty,max=500"`
}

// UpdateLicenseRequest adalah payload ubah izin soal.
type UpdateLicenseRequest struct {
	Scope          *string   `json:"scope" validate:"omitempty,oneof=internal vendor public_domain client_owned"`
	Owner          *string   `json:"owner" validate:"omitempty,max=160"`
	AgreementNo    *string   `json:"agreementNo" validate:"omitempty,max=80"`
	AgreementURL   *string   `json:"agreementUrl" validate:"omitempty,max=300"`
	CommercialUse  *bool     `json:"commercialUse"`
	SelectionUse   *bool     `json:"selectionUse"`
	AllowedSectors *[]string `json:"allowedSectors" validate:"omitempty,max=13,dive,max=40"`
	ValidFrom      *string   `json:"validFrom" validate:"omitempty"`
	ValidUntil     *string   `json:"validUntil" validate:"omitempty"`
	Status         *string   `json:"status" validate:"omitempty,oneof=aktif kedaluwarsa dicabut"`
	Notes          *string   `json:"notes" validate:"omitempty,max=500"`
}

// LicenseView adalah satu izin soal untuk panel admin.
type LicenseView struct {
	ID           string `json:"id"`
	QuestionID   string `json:"questionId"`
	QuestionText string `json:"questionText,omitempty"`
	BankKey      string `json:"bankKey,omitempty"`
	SectorKey    string `json:"sectorKey,omitempty"`
	Scope        string `json:"scope"`
	ScopeLabel   string `json:"scopeLabel,omitempty"`
	Owner        string `json:"owner"`
	AgreementNo  string `json:"agreementNo"`
	AgreementURL string `json:"agreementUrl,omitempty"`
	// CommercialUse & SelectionUse menentukan izin pemakaian produk berbayar
	// dan asesmen/seleksi.
	CommercialUse  bool     `json:"commercialUse"`
	SelectionUse   bool     `json:"selectionUse"`
	AllowedSectors []string `json:"allowedSectors"`
	ValidFrom      string   `json:"validFrom"`
	ValidUntil     string   `json:"validUntil,omitempty"`
	// Status adalah status tersimpan; StatusEffective memperhitungkan masa
	// berlaku yang sudah lewat (izin kedaluwarsa otomatis berhenti dipakai).
	Status          string `json:"status"`
	StatusEffective string `json:"statusEffective"`
	Notes           string `json:"notes"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// UnlicensedQuestionView adalah soal tanpa izin aktif.
type UnlicensedQuestionView struct {
	ID            string `json:"id"`
	BankKey       string `json:"bankKey"`
	BankLabel     string `json:"bankLabel,omitempty"`
	SectorKey     string `json:"sectorKey"`
	SectorLabel   string `json:"sectorLabel,omitempty"`
	TestType      string `json:"testType"`
	TestTypeLabel string `json:"testTypeLabel,omitempty"`
	Text          string `json:"text"`
	// Status kosong berarti soal tidak punya izin yang masih berlaku; lihat
	// HasAnyLicense untuk membedakan "belum dibuat" dan "sudah tidak berlaku".
	Status        string `json:"status"`
	ValidUntil    string `json:"validUntil,omitempty"`
	HasAnyLicense bool   `json:"hasAnyLicense"`
	UpdatedAt     string `json:"updatedAt"`
}

// LicenseSummaryView adalah ringkasan kepatuhan izin bank soal.
type LicenseSummaryView struct {
	TotalLicenses       int64                    `json:"totalLicenses"`
	UnlicensedQuestions int64                    `json:"unlicensedQuestions"`
	ExpiringSoon        int                      `json:"expiringSoon"`
	ProblemLicenses     int                      `json:"problemLicenses"`
	ByScope             map[string]int64         `json:"byScope"`
	Unlicensed          []UnlicensedQuestionView `json:"unlicensed"`
	Expiring            []LicenseView            `json:"expiring"`
	// EnforcementEnabled menandai apakah izin ditegakkan saat peserta memulai
	// tryout (soal tanpa izin aktif tidak dikirim ke peserta).
	EnforcementEnabled bool   `json:"enforcementEnabled"`
	ExpiringWindowDays int    `json:"expiringWindowDays"`
	GeneratedAt        string `json:"generatedAt"`
}

/* --------------------------- instrumen psikologi -------------------------- */

// CreateInstrumentRequest adalah payload tambah instrumen psikologi.
type CreateInstrumentRequest struct {
	Code           string `json:"code" validate:"required,min=2,max=40"`
	Name           string `json:"name" validate:"required,min=3,max=160"`
	Kind           string `json:"kind" validate:"required,oneof=latihan berlisensi"`
	Vendor         string `json:"vendor" validate:"omitempty,max=160"`
	LicenseNo      string `json:"licenseNo" validate:"omitempty,max=80"`
	LicenseURL     string `json:"licenseUrl" validate:"omitempty,max=300"`
	LicenseFrom    string `json:"licenseFrom" validate:"omitempty"`
	LicenseUntil   string `json:"licenseUntil" validate:"omitempty"`
	Status         string `json:"status" validate:"omitempty,oneof=aktif kedaluwarsa dicabut"`
	AdministeredBy string `json:"administeredBy" validate:"omitempty,uuid4"`
	ScoringNotes   string `json:"scoringNotes" validate:"omitempty,max=1000"`
	Active         *bool  `json:"active"`
}

// UpdateInstrumentRequest adalah payload ubah instrumen psikologi.
type UpdateInstrumentRequest struct {
	Name           *string `json:"name" validate:"omitempty,min=3,max=160"`
	Kind           *string `json:"kind" validate:"omitempty,oneof=latihan berlisensi"`
	Vendor         *string `json:"vendor" validate:"omitempty,max=160"`
	LicenseNo      *string `json:"licenseNo" validate:"omitempty,max=80"`
	LicenseURL     *string `json:"licenseUrl" validate:"omitempty,max=300"`
	LicenseFrom    *string `json:"licenseFrom" validate:"omitempty"`
	LicenseUntil   *string `json:"licenseUntil" validate:"omitempty"`
	Status         *string `json:"status" validate:"omitempty,oneof=aktif kedaluwarsa dicabut"`
	AdministeredBy *string `json:"administeredBy" validate:"omitempty,uuid4"`
	ScoringNotes   *string `json:"scoringNotes" validate:"omitempty,max=1000"`
	Active         *bool   `json:"active"`
}

// InstrumentView adalah satu instrumen psikologi untuk panel admin.
type InstrumentView struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	KindLabel       string `json:"kindLabel,omitempty"`
	Vendor          string `json:"vendor"`
	LicenseNo       string `json:"licenseNo"`
	LicenseURL      string `json:"licenseUrl,omitempty"`
	LicenseFrom     string `json:"licenseFrom,omitempty"`
	LicenseUntil    string `json:"licenseUntil,omitempty"`
	Status          string `json:"status"`
	StatusEffective string `json:"statusEffective"`
	AdministeredBy  string `json:"administeredBy,omitempty"`
	ScoringNotes    string `json:"scoringNotes"`
	Active          bool   `json:"active"`
	// Usable menandai instrumen boleh dipakai sekarang: izin aktif dan, untuk
	// instrumen berlisensi, ada psikolog penanggung jawab.
	Usable     bool   `json:"usable"`
	UsableNote string `json:"usableNote,omitempty"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}
