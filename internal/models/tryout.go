package models

import "time"

// Difficulty adalah tingkat kesulitan soal.
type Difficulty string

const (
	// DifficultyMudah soal mudah.
	DifficultyMudah Difficulty = "mudah"
	// DifficultySedang soal sedang.
	DifficultySedang Difficulty = "sedang"
	// DifficultySulit soal sulit.
	DifficultySulit Difficulty = "sulit"
)

// Valid memeriksa apakah tingkat kesulitan dikenal.
func (d Difficulty) Valid() bool {
	return d == DifficultyMudah || d == DifficultySedang || d == DifficultySulit
}

// Question adalah satu butir soal pada bank soal.
//
// `BankKey` menentukan subtes mana yang memakai soal ini (lewat `Subtest.Banks`),
// sedangkan `TryoutID` menentukan paket tryout pemiliknya — satu soal dimiliki
// SATU tryout, sehingga panel bank soal bisa dikelompokkan per tryout. Nilai
// NULL berarti soal belum ditautkan ke tryout mana pun (warisan data lama).
//
// Tiga dimensi tambahan mengelompokkan soal sesuai kebutuhan seleksi BUMN:
// `SectorKey` (klaster/sektor BUMN), `TestType` (jenis tes resmi), dan
// `PositionFamily` (keluarga jabatan). Semuanya bernilai bawaan sehingga soal
// lama tetap valid tanpa diisi ulang.
//
// `AnswerIndex` dan `Explanation` TIDAK PERNAH dikirim ke klien selama sesi
// tryout berjalan — keduanya baru dibuka pada endpoint review setelah submit.
type Question struct {
	ID       string  `gorm:"column:id;primaryKey" json:"id"`
	BankKey  string  `gorm:"column:bank_key;not null" json:"bankKey"`
	TryoutID *string `gorm:"column:tryout_id" json:"tryoutId,omitempty"`
	// SectorKey menandai soal sesuai kategori/klaster BUMN (mis. jasa_keuangan).
	SectorKey string `gorm:"column:sector_key;not null;default:umum" json:"sectorKey"`
	// CompanyKey menandai soal yang khusus untuk satu BUMN (opsional).
	CompanyKey *string `gorm:"column:company_key" json:"companyKey,omitempty"`
	// TestType adalah jenis tes seleksi BUMN (tkd, akhlaq, twk, bing, tkb, …).
	TestType string `gorm:"column:test_type;not null;default:tkd" json:"testType"`
	// PositionFamily adalah keluarga jabatan yang dituju soal (khusus TKB).
	PositionFamily string `gorm:"column:position_family;not null;default:umum" json:"positionFamily"`
	// VariantGroup menautkan soal-soal yang merupakan varian satu sama lain
	// (mis. angka berbeda, rumus sama). Sesi tryout hanya memakai SATU soal per
	// grup sehingga dua peserta tidak mengerjakan soal yang identik.
	VariantGroup *string `gorm:"column:variant_group" json:"variantGroup,omitempty"`
	// RetentionUntil membatasi masa simpan soal mengikuti perjanjian
	// kerahasiaan; NULL berarti tanpa batas.
	RetentionUntil *time.Time `gorm:"column:retention_until" json:"retentionUntil,omitempty"`
	Text           string     `gorm:"column:text;not null" json:"text"`
	Options        StringList `gorm:"column:options;type:jsonb;not null;default:'[]'" json:"options"`
	AnswerIndex    int        `gorm:"column:answer_index;not null" json:"answerIndex"`
	Explanation    string     `gorm:"column:explanation;not null;default:''" json:"explanation"`
	Difficulty     Difficulty `gorm:"column:difficulty;not null;default:sedang" json:"difficulty"`
	Position       int        `gorm:"column:position;not null;default:0" json:"position"`
	Active         bool       `gorm:"column:active;not null" json:"active"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (Question) TableName() string { return "questions" }

// Tier adalah tingkat akses paket tryout.
type Tier string

const (
	// TierGratis dapat diakses semua peserta.
	TierGratis Tier = "Gratis"
	// TierPremium memerlukan produk tertentu.
	TierPremium Tier = "Premium"
)

// TryoutPackage adalah paket tryout (kumpulan subtes).
type TryoutPackage struct {
	ID              string  `gorm:"column:id;primaryKey" json:"id"`
	Title           string  `gorm:"column:title;not null" json:"title"`
	Description     string  `gorm:"column:description;not null;default:''" json:"description"`
	Tier            Tier    `gorm:"column:tier;not null;default:Gratis" json:"tier"`
	RequiredProduct *string `gorm:"column:required_product" json:"requiredProduct"`
	Attempts        int     `gorm:"column:attempts;not null;default:1" json:"attempts"`
	AccessUntil     string  `gorm:"column:access_until;not null;default:Tanpa batas" json:"accessUntil"`
	Active          bool    `gorm:"column:active;not null" json:"active"`
	SortOrder       int     `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	Timestamps

	Subtests []Subtest `gorm:"foreignKey:TryoutID;references:ID" json:"subtests"`
}

// TableName menetapkan nama tabel.
func (TryoutPackage) TableName() string { return "tryout_packages" }

// Subtest adalah bagian dari paket tryout dengan alokasi waktu sendiri.
type Subtest struct {
	Base
	TryoutID  string     `gorm:"column:tryout_id;not null" json:"tryoutId"`
	Name      string     `gorm:"column:name;not null" json:"name"`
	Minutes   int        `gorm:"column:minutes;not null;default:5" json:"minutes"`
	Banks     StringList `gorm:"column:banks;type:jsonb;not null;default:'[]'" json:"banks"`
	SortOrder int        `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (Subtest) TableName() string { return "subtests" }
