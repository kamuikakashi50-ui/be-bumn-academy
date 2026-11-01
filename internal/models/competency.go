package models

import "time"

// Kamus kompetensi & penilaiannya (Fase 3 langkah 1).
//
// Tabel-tabel di sini sengaja **tanpa** `created_at`/`updated_at`: kamus
// kompetensi & standar jabatan adalah referensi yang diperbarui lewat upsert,
// dan yang penting untuk jejak adalah `assessed_at` pada penilaian peserta.

// Klaster kompetensi.
const (
	CompetencyClusterTeknis         = "teknis"
	CompetencyClusterManajerial     = "manajerial"
	CompetencyClusterSosialKultural = "sosial_kultural"
)

// Sumber penilaian kompetensi (§4.3). `tryout` adalah proksi dari latihan;
// sisanya penilaian manusia. Nilai ini disimpan apa adanya di kolom `source`
// dan selalu ditampilkan di UI.
const (
	CompetencySourceTryout   = "tryout"
	CompetencySourceSelf     = "self"
	CompetencySourceAsesor   = "asesor"
	CompetencySourceManager  = "manager"
	CompetencySourcePsikotes = "psikotes"
)

// Competency adalah satu entri kamus kompetensi.
type Competency struct {
	Code       string `gorm:"column:code;primaryKey" json:"code"`
	Name       string `gorm:"column:name;not null" json:"name"`
	Cluster    string `gorm:"column:cluster;not null;default:teknis" json:"cluster"`
	Definition string `gorm:"column:definition;not null;default:''" json:"definition"`
	Active     bool   `gorm:"column:active;not null;default:true" json:"active"`
}

// TableName menetapkan nama tabel.
func (Competency) TableName() string { return "competencies" }

// CompetencyLevel adalah deskriptor perilaku satu level kompetensi (1–5).
type CompetencyLevel struct {
	CompetencyCode string `gorm:"column:competency_code;primaryKey" json:"competencyCode"`
	Level          int    `gorm:"column:level;primaryKey" json:"level"`
	Descriptor     string `gorm:"column:descriptor;not null" json:"descriptor"`
}

// TableName menetapkan nama tabel.
func (CompetencyLevel) TableName() string { return "competency_levels" }

// PositionRequirement adalah standar kompetensi minimum sebuah keluarga jabatan.
type PositionRequirement struct {
	PositionFamily string  `gorm:"column:position_family;primaryKey" json:"positionFamily"`
	SectorKey      string  `gorm:"column:sector_key;primaryKey;default:umum" json:"sectorKey"`
	CompetencyCode string  `gorm:"column:competency_code;primaryKey" json:"competencyCode"`
	MinLevel       int     `gorm:"column:min_level;not null" json:"minLevel"`
	Weight         float64 `gorm:"column:weight;not null;default:1" json:"weight"`
}

// TableName menetapkan nama tabel.
func (PositionRequirement) TableName() string { return "position_requirements" }

// UserCompetency adalah satu penilaian kompetensi seorang peserta dari satu sumber.
type UserCompetency struct {
	UserID         string    `gorm:"column:user_id;primaryKey" json:"userId"`
	CompetencyCode string    `gorm:"column:competency_code;primaryKey" json:"competencyCode"`
	Level          int       `gorm:"column:level;not null" json:"level"`
	Source         string    `gorm:"column:source;primaryKey" json:"source"`
	Evidence       string    `gorm:"column:evidence;not null;default:''" json:"evidence"`
	AssessedBy     *string   `gorm:"column:assessed_by" json:"assessedBy,omitempty"`
	AssessedAt     time.Time `gorm:"column:assessed_at;not null;default:now()" json:"assessedAt"`
}

// TableName menetapkan nama tabel.
func (UserCompetency) TableName() string { return "user_competencies" }

// BankCompetencyMap memetakan kategori bank soal (Fase 0) ke kompetensi.
type BankCompetencyMap struct {
	BankKey        string  `gorm:"column:bank_key;primaryKey" json:"bankKey"`
	CompetencyCode string  `gorm:"column:competency_code;not null" json:"competencyCode"`
	Weight         float64 `gorm:"column:weight;not null;default:1" json:"weight"`
}

// TableName menetapkan nama tabel.
func (BankCompetencyMap) TableName() string { return "bank_competency_map" }

// CompetencySourceLabel mengembalikan label sumber penilaian siap tampil.
func CompetencySourceLabel(source string) string {
	switch source {
	case CompetencySourceTryout:
		return "Proksi dari latihan"
	case CompetencySourceSelf:
		return "Penilaian diri"
	case CompetencySourceAsesor:
		return "Asesmen formal"
	case CompetencySourceManager:
		return "Penilaian atasan"
	case CompetencySourcePsikotes:
		return "Psikotes"
	default:
		return source
	}
}

// CompetencyClusterLabel mengembalikan label klaster kompetensi.
func CompetencyClusterLabel(cluster string) string {
	switch cluster {
	case CompetencyClusterTeknis:
		return "Teknis"
	case CompetencyClusterManajerial:
		return "Manajerial"
	case CompetencyClusterSosialKultural:
		return "Sosial Kultural"
	default:
		return cluster
	}
}
