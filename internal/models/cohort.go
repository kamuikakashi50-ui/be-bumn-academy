package models

import (
	"time"
)

/* ------------------------------- instansi ---------------------------------- */

// Organization adalah instansi mitra (BUMN atau unit di dalamnya).
//
// Satu instansi bisa punya beberapa kohort (batch/kelas). `SectorKey` memakai
// kunci taksonomi Fase 0 sehingga laporan instansi bisa disilangkan dengan
// klaster bank soal.
type Organization struct {
	ID        string  `gorm:"column:id;primaryKey" json:"id"`
	Name      string  `gorm:"column:name;not null" json:"name"`
	Slug      string  `gorm:"column:slug;not null" json:"slug"`
	SectorKey string  `gorm:"column:sector_key;not null;default:umum" json:"sectorKey"`
	LogoURL   *string `gorm:"column:logo_url" json:"logoUrl,omitempty"`
	PICName   string  `gorm:"column:pic_name;not null;default:''" json:"picName"`
	PICEmail  string  `gorm:"column:pic_email;not null;default:''" json:"picEmail"`
	PICPhone  string  `gorm:"column:pic_phone;not null;default:''" json:"picPhone"`
	// Active tanpa tag `default:` — instansi yang dinonaktifkan (`false`) harus
	// benar-benar tersimpan, bukan ditimpa nilai tag saat INSERT.
	Active bool `gorm:"column:active;not null" json:"active"`
}

// TableName menetapkan nama tabel.
func (Organization) TableName() string { return "organizations" }

/* -------------------------------- kohort ----------------------------------- */

// CohortStatus adalah status daur hidup sebuah kelas khusus.
type CohortStatus string

const (
	// CohortDraft kelas masih disusun, belum bisa diikuti peserta.
	CohortDraft CohortStatus = "draft"
	// CohortAktif kelas berjalan dan bisa diikuti lewat kode undangan.
	CohortAktif CohortStatus = "aktif"
	// CohortSelesai kelas sudah berakhir (akses materi mengikuti `AccessUntil`).
	CohortSelesai CohortStatus = "selesai"
	// CohortArsip kelas diarsipkan, tidak lagi ditampilkan sebagai kelas berjalan.
	CohortArsip CohortStatus = "arsip"
)

// Valid memeriksa apakah status kohort dikenal.
func (s CohortStatus) Valid() bool {
	switch s {
	case CohortDraft, CohortAktif, CohortSelesai, CohortArsip:
		return true
	default:
		return false
	}
}

// Label mengembalikan label status untuk ditampilkan di UI/laporan.
func (s CohortStatus) Label() string {
	switch s {
	case CohortDraft:
		return "Draf"
	case CohortAktif:
		return "Aktif"
	case CohortSelesai:
		return "Selesai"
	case CohortArsip:
		return "Arsip"
	default:
		return string(s)
	}
}

// Cohort adalah satu batch/kelas khusus untuk sebuah instansi.
//
// Kohort tidak memiliki materi sendiri: modulnya diambil dari ruang kelas yang
// sudah ada lewat `cohort_curriculum`, sehingga perbaikan materi otomatis berlaku
// untuk semua kelas.
type Cohort struct {
	ID string `gorm:"column:id;primaryKey" json:"id"`
	// OrgID NULL berarti kelas internal (bukan kelas milik instansi).
	OrgID     *string      `gorm:"column:org_id" json:"orgId,omitempty"`
	Name      string       `gorm:"column:name;not null" json:"name"`
	Code      string       `gorm:"column:code;not null" json:"code"`
	SectorKey string       `gorm:"column:sector_key;not null;default:umum" json:"sectorKey"`
	Status    CohortStatus `gorm:"column:status;not null" json:"status"`
	StartsAt  *time.Time   `gorm:"column:starts_at" json:"startsAt,omitempty"`
	EndsAt    *time.Time   `gorm:"column:ends_at" json:"endsAt,omitempty"`
	// Quota 0 berarti tanpa batas peserta.
	Quota int `gorm:"column:quota;not null;default:0" json:"quota"`
	// AccessUntil membatasi masa akses materi peserta (NULL = tanpa batas).
	AccessUntil  *time.Time `gorm:"column:access_until" json:"accessUntil,omitempty"`
	MentorUserID *string    `gorm:"column:mentor_user_id" json:"mentorUserId,omitempty"`
	Notes        string     `gorm:"column:notes;not null;default:''" json:"notes"`

	// Organization diisi lewat Preload saat memuat detail kohort.
	Organization *Organization `gorm:"foreignKey:OrgID" json:"organization,omitempty"`
	// Curriculum diisi lewat Preload saat memuat detail kohort.
	Curriculum []CohortCurriculumItem `gorm:"foreignKey:CohortID" json:"curriculum,omitempty"`
}

// TableName menetapkan nama tabel.
func (Cohort) TableName() string { return "cohorts" }

// AcceptsMembers menentukan kelas masih menerima anggota baru.
//
// `active` adalah jumlah anggota berstatus aktif saat ini. Kuota 0 berarti tanpa
// batas.
func (c Cohort) AcceptsMembers(active int) bool {
	return c.Quota <= 0 || active < c.Quota
}

// ActiveAt menentukan kelas sedang berjalan pada waktu tertentu.
//
// Status wajib `aktif`; periode kelas (bila diisi) juga harus mencakup `now`.
// Dipakai saat memutuskan apakah kode undangan boleh dipakai dan apakah materi
// kelas bisa dibuka.
func (c Cohort) ActiveAt(now time.Time) bool {
	if c.Status != CohortAktif {
		return false
	}
	if c.StartsAt != nil && now.Before(*c.StartsAt) {
		return false
	}
	if c.EndsAt != nil && now.After(*c.EndsAt) {
		return false
	}
	return true
}

// AccessExpired menentukan masa akses materi peserta sudah lewat.
//
// Setelah ini, materi ditutup tetapi riwayat belajar & sertifikat tetap bisa
// dilihat (aturan §4.1).
func (c Cohort) AccessExpired(now time.Time) bool {
	return c.AccessUntil != nil && now.After(*c.AccessUntil)
}

/* ------------------------------ keanggotaan -------------------------------- */

// CohortMemberRole adalah peran anggota di dalam sebuah kelas.
type CohortMemberRole string

const (
	// CohortRolePeserta anggota yang mengikuti kelas.
	CohortRolePeserta CohortMemberRole = "peserta"
	// CohortRoleMentor pendamping/mentor kelas.
	CohortRoleMentor CohortMemberRole = "mentor"
	// CohortRoleAdminInstansi pengelola dari pihak instansi (menyusul di Fase 3).
	CohortRoleAdminInstansi CohortMemberRole = "admin_instansi"
)

// Valid memeriksa apakah peran anggota dikenal.
func (r CohortMemberRole) Valid() bool {
	switch r {
	case CohortRolePeserta, CohortRoleMentor, CohortRoleAdminInstansi:
		return true
	default:
		return false
	}
}

// CohortMemberStatus adalah status keanggotaan peserta di kelas.
type CohortMemberStatus string

const (
	// CohortMemberAktif masih mengikuti kelas (dihitung pada kuota).
	CohortMemberAktif CohortMemberStatus = "aktif"
	// CohortMemberKeluar keluar sebelum kelas selesai.
	CohortMemberKeluar CohortMemberStatus = "keluar"
	// CohortMemberLulus menyelesaikan kelas (tidak dihitung pada kuota).
	CohortMemberLulus CohortMemberStatus = "lulus"
)

// Valid memeriksa apakah status keanggotaan dikenal.
func (s CohortMemberStatus) Valid() bool {
	switch s {
	case CohortMemberAktif, CohortMemberKeluar, CohortMemberLulus:
		return true
	default:
		return false
	}
}

// CountsTowardsQuota menandai status yang memakai jatah kuota kelas.
func (s CohortMemberStatus) CountsTowardsQuota() bool { return s == CohortMemberAktif }

// CohortMember adalah keanggotaan satu pengguna pada satu kelas.
type CohortMember struct {
	CohortID string             `gorm:"column:cohort_id;primaryKey" json:"cohortId"`
	UserID   string             `gorm:"column:user_id;type:uuid;primaryKey" json:"userId"`
	Role     CohortMemberRole   `gorm:"column:role;not null" json:"role"`
	Status   CohortMemberStatus `gorm:"column:status;not null" json:"status"`
	JoinedAt time.Time          `gorm:"column:joined_at;not null;default:now()" json:"joinedAt"`
}

// TableName menetapkan nama tabel.
func (CohortMember) TableName() string { return "cohort_members" }

// IsActive menandai anggota masih mengikuti kelas.
func (m CohortMember) IsActive() bool { return m.Status == CohortMemberAktif }

// CohortCurriculumItem adalah satu modul yang diikutkan ke sebuah kelas.
type CohortCurriculumItem struct {
	CohortID string `gorm:"column:cohort_id;primaryKey" json:"cohortId"`
	ModuleID string `gorm:"column:module_id;primaryKey" json:"moduleId"`
	// Required tanpa tag `default:` supaya modul opsional (`false`) tersimpan
	// apa adanya — kesalahan yang sama pernah terjadi pada langkah jalur belajar.
	Required  bool `gorm:"column:required;not null" json:"required"`
	SortOrder int  `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`

	// Module diisi lewat Preload saat memuat kurikulum kelas.
	Module *CourseModule `gorm:"foreignKey:ModuleID;references:ID" json:"module,omitempty"`
}

// TableName menetapkan nama tabel.
func (CohortCurriculumItem) TableName() string { return "cohort_curriculum" }

/* ------------------------------ kode undangan ------------------------------ */

// CohortInvite adalah kode undangan untuk bergabung ke sebuah kelas.
//
// Beberapa kode boleh menunjuk kohort yang sama, sehingga admin bisa mencabut satu
// kode tanpa mengganggu kode lain (mis. kode khusus satu batch).
type CohortInvite struct {
	Code string `gorm:"column:code;primaryKey" json:"code"`
	// CohortID wajib: kode undangan tanpa kelas tidak punya arti.
	CohortID  string  `gorm:"column:cohort_id;not null" json:"cohortId"`
	CreatedBy *string `gorm:"column:created_by" json:"createdBy,omitempty"`
	// MaxUses 0 berarti tanpa batas pemakaian.
	MaxUses   int        `gorm:"column:max_uses;not null;default:0" json:"maxUses"`
	UsedCount int        `gorm:"column:used_count;not null;default:0" json:"usedCount"`
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	// Active tanpa tag `default:` — kode yang dicabut (`false`) harus tetap
	// tersimpan sebagai tidak aktif.
	Active bool `gorm:"column:active;not null" json:"active"`

	// Cohort diisi lewat Preload saat peserta memakai kode undangan.
	Cohort *Cohort `gorm:"foreignKey:CohortID" json:"cohort,omitempty"`
}

// TableName menetapkan nama tabel.
func (CohortInvite) TableName() string { return "cohort_invites" }

// Exhausted menentukan kuota pemakaian kode sudah habis.
func (i CohortInvite) Exhausted() bool { return i.MaxUses > 0 && i.UsedCount >= i.MaxUses }

// UsableAt menentukan kode undangan masih bisa dipakai pada waktu tertentu.
func (i CohortInvite) UsableAt(now time.Time) bool { return i.RejectReason(now) == "" }

// RejectReason mengembalikan alasan kode undangan tidak bisa dipakai.
//
// Mengembalikan string kosong bila kode masih sah. Pesan ditulis di sini (bukan di
// handler) supaya alasan yang sama dipakai endpoint peserta maupun panel admin.
func (i CohortInvite) RejectReason(now time.Time) string {
	if !i.Active {
		return "Kode undangan sudah dicabut."
	}
	if i.ExpiresAt != nil && now.After(*i.ExpiresAt) {
		return "Kode undangan sudah kedaluwarsa."
	}
	if i.Exhausted() {
		return "Kuota kode undangan sudah habis."
	}
	return ""
}

/* ------------------------------- jejak audit ------------------------------- */

// Aksi yang dicatat pada jejak audit kohort.
const (
	CohortActionCreated       = "cohort_created"
	CohortActionInviteCreated = "invite_created"
	CohortActionInviteRevoked = "invite_revoked"
	CohortActionInviteUsed    = "invite_used"
	CohortActionMemberAdded   = "member_added"
	CohortActionMemberRemoved = "member_removed"
	CohortActionMemberStatus  = "member_status_changed"
	CohortActionCurriculumSet = "curriculum_updated"
	CohortActionStatusChanged = "status_changed"
)

// CohortAuditLog mencatat perubahan penting pada sebuah kelas.
//
// Dipakai menjawab "siapa mengundang/mengeluarkan siapa, kapan" ketika angka
// laporan kelas dipertanyakan.
type CohortAuditLog struct {
	// IDBase: tabel ini tidak punya updated_at, waktunya diwakili `created_at`.
	IDBase
	CohortID  string    `gorm:"column:cohort_id;not null" json:"cohortId"`
	ActorID   *string   `gorm:"column:actor_id" json:"actorId,omitempty"`
	Action    string    `gorm:"column:action;not null" json:"action"`
	Detail    string    `gorm:"column:detail;not null;default:''" json:"detail"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (CohortAuditLog) TableName() string { return "cohort_audit_logs" }
