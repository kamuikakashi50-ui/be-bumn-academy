package models

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
// `AnswerIndex` dan `Explanation` TIDAK PERNAH dikirim ke klien selama sesi
// tryout berjalan — keduanya baru dibuka pada endpoint review setelah submit.
type Question struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	BankKey     string     `gorm:"column:bank_key;not null" json:"bankKey"`
	Text        string     `gorm:"column:text;not null" json:"text"`
	Options     StringList `gorm:"column:options;type:jsonb;not null;default:'[]'" json:"options"`
	AnswerIndex int        `gorm:"column:answer_index;not null" json:"answerIndex"`
	Explanation string     `gorm:"column:explanation;not null;default:''" json:"explanation"`
	Difficulty  Difficulty `gorm:"column:difficulty;not null;default:sedang" json:"difficulty"`
	Position    int        `gorm:"column:position;not null;default:0" json:"position"`
	Active      bool       `gorm:"column:active;not null;default:true" json:"active"`
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
	Active          bool    `gorm:"column:active;not null;default:true" json:"active"`
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
