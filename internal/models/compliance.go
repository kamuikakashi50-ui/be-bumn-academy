package models

import "time"

/* --------------------------- kepatuhan bank soal -------------------------- */

// QuestionFingerprint adalah sidik jari teks satu soal.
//
// Dipakai untuk mencari soal kembar — kebutuhan yang muncul karena soal terikat
// perjanjian kerahasiaan sehingga tidak boleh dipakai ulang atau muncul identik
// di dua paket/klien. Tiga nilai disimpan sekaligus:
//
//   - Fingerprint   → hash teks ternormalisasi (kembar persis).
//   - TemplateHash  → hash teks dengan angka diganti "#" (satu keluarga soal).
//   - Tokens        → himpunan kata untuk menghitung kemiripan Jaccard.
type QuestionFingerprint struct {
	QuestionID   string     `gorm:"column:question_id;primaryKey" json:"questionId"`
	Fingerprint  string     `gorm:"column:fingerprint;not null" json:"fingerprint"`
	TemplateHash string     `gorm:"column:template_hash;not null" json:"templateHash"`
	Tokens       StringList `gorm:"column:tokens;type:jsonb;not null;default:'[]'" json:"tokens"`
	ComputedAt   time.Time  `gorm:"column:computed_at;not null;default:now()" json:"computedAt"`
}

// TableName menetapkan nama tabel.
func (QuestionFingerprint) TableName() string { return "question_fingerprints" }

// Aksi pada jejak akses soal.
const (
	// QuestionActionCreate menandai soal baru dibuat.
	QuestionActionCreate = "create"
	// QuestionActionUpdate menandai soal diubah.
	QuestionActionUpdate = "update"
	// QuestionActionDelete menandai soal dihapus.
	QuestionActionDelete = "delete"
	// QuestionActionImport menandai soal masuk lewat import berkas.
	QuestionActionImport = "import"
	// QuestionActionVariant menandai soal ditautkan ke grup varian.
	QuestionActionVariant = "variant"
)

// QuestionAccessLog adalah jejak audit akses/perubahan bank soal.
//
// Soal yang terikat perjanjian kerahasiaan perlu diketahui siapa menyentuhnya
// dan kapan — terutama saat ada dugaan kebocoran soal.
type QuestionAccessLog struct {
	Base
	QuestionID string  `gorm:"column:question_id;not null" json:"questionId"`
	ActorID    *string `gorm:"column:actor_id" json:"actorId,omitempty"`
	Action     string  `gorm:"column:action;not null" json:"action"`
	Detail     string  `gorm:"column:detail;not null;default:''" json:"detail"`
}

// TableName menetapkan nama tabel.
func (QuestionAccessLog) TableName() string { return "question_access_logs" }
