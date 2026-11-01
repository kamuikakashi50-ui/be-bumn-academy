package models

import (
	"time"

	"gorm.io/gorm"
)

// Mitra psikolog, konsultasi daring, jejak akses data asesmen, dan koreksi hasil
// (Fase 3 langkah 6–7).

/* ------------------------------ mitra psikolog ----------------------------- */

// PartnerStatus adalah status verifikasi izin praktik mitra psikolog.
type PartnerStatus string

const (
	// PartnerPending menunggu verifikasi admin.
	PartnerPending PartnerStatus = "pending"
	// PartnerTerverifikasi boleh ditugaskan dan membuka data peserta.
	PartnerTerverifikasi PartnerStatus = "terverifikasi"
	// PartnerNonaktif izinnya tidak berlaku lagi.
	PartnerNonaktif PartnerStatus = "nonaktif"
)

// Valid memeriksa apakah status dikenal.
func (s PartnerStatus) Valid() bool {
	switch s {
	case PartnerPending, PartnerTerverifikasi, PartnerNonaktif:
		return true
	default:
		return false
	}
}

// PartnerStatusLabel mengembalikan label status siap tampil.
func PartnerStatusLabel(status PartnerStatus) string {
	switch status {
	case PartnerTerverifikasi:
		return "Terverifikasi"
	case PartnerNonaktif:
		return "Nonaktif"
	default:
		return "Menunggu verifikasi"
	}
}

// PsychologistPartner adalah mitra psikolog beserta izin praktiknya (§4.7).
type PsychologistPartner struct {
	Base
	// UserID menautkan mitra ke akun login (opsional; mitra boleh dicatat lebih
	// dulu sebelum akunnya dibuat).
	UserID          *string       `gorm:"column:user_id;type:uuid" json:"userId,omitempty"`
	FullName        string        `gorm:"column:full_name;not null" json:"fullName"`
	Email           string        `gorm:"column:email;not null;default:''" json:"email"`
	Phone           string        `gorm:"column:phone;not null;default:''" json:"phone"`
	SippNumber      string        `gorm:"column:sipp_number;not null;default:''" json:"sippNumber"`
	SippkNumber     *string       `gorm:"column:sippk_number" json:"sippkNumber,omitempty"`
	StrNumber       *string       `gorm:"column:str_number" json:"strNumber,omitempty"`
	Specializations StringList    `gorm:"column:specializations;type:jsonb;not null;default:'[]'" json:"specializations"`
	Status          PartnerStatus `gorm:"column:verification_status;not null;default:pending" json:"verificationStatus"`
	ValidUntil      *time.Time    `gorm:"column:valid_until;type:date" json:"validUntil,omitempty"`
	EvidenceURL     *string       `gorm:"column:evidence_url" json:"evidenceUrl,omitempty"`
	Notes           string        `gorm:"column:notes;not null;default:''" json:"notes"`
}

// TableName menetapkan nama tabel.
func (PsychologistPartner) TableName() string { return "psychologist_partners" }

// StatusLabel mengembalikan label status siap tampil.
func (p PsychologistPartner) StatusLabel() string { return PartnerStatusLabel(p.Status) }

// Assignable menandai mitra boleh ditugaskan pada tanggal tertentu.
//
// Dua syarat (§4.7): status `terverifikasi` **dan** masa berlaku izin belum
// lewat. `valid_until` kosong dianggap tanpa batas (mis. STR seumur hidup).
func (p PsychologistPartner) Assignable(at time.Time) bool {
	if p.Status != PartnerTerverifikasi {
		return false
	}
	if p.ValidUntil == nil {
		return true
	}
	// Bandingkan per hari: izin berlaku sampai akhir tanggal `valid_until`.
	return !at.Truncate(24 * time.Hour).After(p.ValidUntil.Truncate(24 * time.Hour))
}

/* ----------------------------- slot konsultasi ----------------------------- */

// ConsultationSlot adalah satu slot jadwal konsultasi yang dibuka mitra.
type ConsultationSlot struct {
	ID             string    `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	PsychologistID string    `gorm:"column:psychologist_id;type:uuid;not null" json:"psychologistId"`
	StartsAt       time.Time `gorm:"column:starts_at;not null" json:"startsAt"`
	EndsAt         time.Time `gorm:"column:ends_at;not null" json:"endsAt"`
	Mode           string    `gorm:"column:mode;not null;default:online" json:"mode"`
	Quota          int       `gorm:"column:quota;not null;default:1" json:"quota"`
	Booked         int       `gorm:"column:booked;not null;default:0" json:"booked"`
	ChannelURL     *string   `gorm:"column:channel_url" json:"channelUrl,omitempty"`
	Active         bool      `gorm:"column:active;not null;default:true" json:"active"`
}

// TableName menetapkan nama tabel.
func (ConsultationSlot) TableName() string { return "consultation_slots" }

// BeforeCreate memastikan ID slot terisi saat dibuat langsung di Go.
func (s *ConsultationSlot) BeforeCreate(*gorm.DB) error {
	if s.ID == "" {
		s.ID = NewID()
	}
	return nil
}

// HasRoom menandai slot masih punya kuota tersisa (§4.5).
func (s ConsultationSlot) HasRoom() bool { return s.Booked < s.Quota }

// Remaining mengembalikan sisa kuota (tidak pernah negatif).
func (s ConsultationSlot) Remaining() int {
	if s.Booked >= s.Quota {
		return 0
	}
	return s.Quota - s.Booked
}

/* ------------------------------ pemesanan ---------------------------------- */

// BookingStatus adalah status satu pemesanan konsultasi.
type BookingStatus string

const (
	// BookingTerpesan sudah dipesan dan belum berlangsung.
	BookingTerpesan BookingStatus = "terpesan"
	// BookingSelesai sesi sudah berlangsung.
	BookingSelesai BookingStatus = "selesai"
	// BookingBatal dibatalkan peserta sesuai aturan 12 jam.
	BookingBatal BookingStatus = "batal"
	// BookingTidakHadir peserta tidak hadir tanpa pembatalan.
	BookingTidakHadir BookingStatus = "tidak_hadir"
)

// Valid memeriksa apakah status pemesanan dikenal.
func (s BookingStatus) Valid() bool {
	switch s {
	case BookingTerpesan, BookingSelesai, BookingBatal, BookingTidakHadir:
		return true
	default:
		return false
	}
}

// BookingStatusLabel mengembalikan label status siap tampil.
func BookingStatusLabel(status BookingStatus) string {
	switch status {
	case BookingSelesai:
		return "Selesai"
	case BookingBatal:
		return "Dibatalkan"
	case BookingTidakHadir:
		return "Tidak hadir"
	default:
		return "Terpesan"
	}
}

// CancelWindow adalah batas waktu pembatalan mandiri oleh peserta (§4.5).
//
// Peserta boleh batal ≥ 12 jam sebelum jadwal; setelah itu pemesanan ditandai
// `tidak_hadir` agar rekap pemanfaatan layanan tetap jujur.
const CancelWindow = 12 * time.Hour

// ConsultationBooking adalah satu pemesanan slot oleh peserta.
type ConsultationBooking struct {
	ID        string        `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	SlotID    string        `gorm:"column:slot_id;type:uuid;not null" json:"slotId"`
	UserID    string        `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	CohortID  *string       `gorm:"column:cohort_id;type:uuid" json:"cohortId,omitempty"`
	Status    BookingStatus `gorm:"column:status;not null;default:terpesan" json:"status"`
	Agenda    string        `gorm:"column:agenda;not null;default:''" json:"agenda"`
	ConsentID *string       `gorm:"column:consent_id;type:uuid" json:"consentId,omitempty"`
	CreatedAt time.Time     `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time     `gorm:"column:updated_at;not null;autoUpdateTime" json:"updatedAt"`
}

// TableName menetapkan nama tabel.
func (ConsultationBooking) TableName() string { return "consultation_bookings" }

// BeforeCreate memastikan ID pemesanan terisi saat dibuat langsung di Go.
func (b *ConsultationBooking) BeforeCreate(*gorm.DB) error {
	if b.ID == "" {
		b.ID = NewID()
	}
	return nil
}

// StatusLabel mengembalikan label status siap tampil.
func (b ConsultationBooking) StatusLabel() string { return BookingStatusLabel(b.Status) }

// Cancellable menentukan apakah peserta masih boleh membatalkan sendiri.
//
// Hanya pemesanan yang masih `terpesan` dan jadwalnya ≥ `CancelWindow` dari
// sekarang yang bisa dibatalkan (§4.5).
func (b ConsultationBooking) Cancellable(now, startsAt time.Time) bool {
	if b.Status != BookingTerpesan {
		return false
	}
	return startsAt.Sub(now) >= CancelWindow
}

/* ------------------------------ catatan sesi ------------------------------- */

// NoteVisibility adalah tingkat visibilitas catatan konsultasi (§4.5).
type NoteVisibility string

const (
	// NotePrivate hanya penulis (psikolog).
	NotePrivate NoteVisibility = "private"
	// NoteSharedToUser juga dibagikan ke peserta.
	NoteSharedToUser NoteVisibility = "shared_to_user"
	// NoteSharedToHR juga dibagikan ke HR instansi.
	NoteSharedToHR NoteVisibility = "shared_to_hr"
)

// Valid memeriksa apakah visibilitas dikenal.
func (v NoteVisibility) Valid() bool {
	switch v {
	case NotePrivate, NoteSharedToUser, NoteSharedToHR:
		return true
	default:
		return false
	}
}

// NoteVisibilityLabel mengembalikan label visibilitas siap tampil.
func NoteVisibilityLabel(visibility NoteVisibility) string {
	switch visibility {
	case NoteSharedToHR:
		return "Dibagikan ke peserta & HR"
	case NoteSharedToUser:
		return "Dibagikan ke peserta"
	default:
		return "Hanya psikolog"
	}
}

// ConsultationNote adalah catatan satu sesi konsultasi.
type ConsultationNote struct {
	ID             string         `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	BookingID      string         `gorm:"column:booking_id;type:uuid;not null" json:"bookingId"`
	AuthorID       string         `gorm:"column:author_id;type:uuid;not null" json:"authorId"`
	Summary        string         `gorm:"column:summary;not null" json:"summary"`
	Recommendation string         `gorm:"column:recommendation;not null;default:''" json:"recommendation"`
	Visibility     NoteVisibility `gorm:"column:visibility;not null;default:private" json:"visibility"`
	RetentionUntil *time.Time     `gorm:"column:retention_until;type:date" json:"retentionUntil,omitempty"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (ConsultationNote) TableName() string { return "consultation_notes" }

// BeforeCreate memastikan ID catatan terisi saat dibuat langsung di Go.
func (n *ConsultationNote) BeforeCreate(*gorm.DB) error {
	if n.ID == "" {
		n.ID = NewID()
	}
	return nil
}

// VisibilityLabel mengembalikan label visibilitas siap tampil.
func (n ConsultationNote) VisibilityLabel() string { return NoteVisibilityLabel(n.Visibility) }

// SharedToUser menandai catatan boleh dibaca peserta yang bersangkutan.
func (n ConsultationNote) SharedToUser() bool {
	return n.Visibility == NoteSharedToUser || n.Visibility == NoteSharedToHR
}

// SharedToHR menandai catatan boleh muncul di rekap HR instansi (§4.6).
func (n ConsultationNote) SharedToHR() bool { return n.Visibility == NoteSharedToHR }

// Expired menandai masa retensi catatan sudah lewat (isi disamarkan otomatis).
func (n ConsultationNote) Expired(at time.Time) bool {
	if n.RetentionUntil == nil {
		return false
	}
	return at.Truncate(24 * time.Hour).After(n.RetentionUntil.Truncate(24 * time.Hour))
}

/* ---------------------------- jejak akses data ----------------------------- */

// AccessResource adalah jenis data asesmen yang diakses (§4.6).
type AccessResource string

const (
	// AccessResourceTalentProfile profil minat–bakat seorang peserta.
	AccessResourceTalentProfile AccessResource = "talent_profile"
	// AccessResourceCompetency profil/matriks kompetensi.
	AccessResourceCompetency AccessResource = "competency"
	// AccessResourceConsultationNote catatan konsultasi.
	AccessResourceConsultationNote AccessResource = "consultation_note"
	// AccessResourceItemAnswers jawaban item mentah (hanya psikolog).
	AccessResourceItemAnswers AccessResource = "item_answers"
)

// AccessAction adalah jenis aksi yang dicatat.
type AccessAction string

const (
	// AccessActionView membaca data.
	AccessActionView AccessAction = "view"
	// AccessActionExport mengunduh/menyalin data.
	AccessActionExport AccessAction = "export"
	// AccessActionUpdate mengubah data.
	AccessActionUpdate AccessAction = "update"
)

// AssessmentAccessLog mencatat siapa membuka data asesmen siapa (§4.6, UU PDP).
type AssessmentAccessLog struct {
	ID         string         `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	ActorID    *string        `gorm:"column:actor_id;type:uuid" json:"actorId,omitempty"`
	SubjectID  string         `gorm:"column:subject_id;type:uuid;not null" json:"subjectId"`
	Resource   AccessResource `gorm:"column:resource;not null" json:"resource"`
	ResourceID string         `gorm:"column:resource_id;not null;default:''" json:"resourceId"`
	Action     AccessAction   `gorm:"column:action;not null" json:"action"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (AssessmentAccessLog) TableName() string { return "assessment_access_logs" }

// BeforeCreate memastikan ID log terisi saat dibuat langsung di Go.
func (l *AssessmentAccessLog) BeforeCreate(*gorm.DB) error {
	if l.ID == "" {
		l.ID = NewID()
	}
	return nil
}

/* --------------------------- koreksi hasil peserta ------------------------- */

// DisputeStatus adalah status satu permohonan koreksi hasil.
type DisputeStatus string

const (
	// DisputeTerbuka belum ditinjau.
	DisputeTerbuka DisputeStatus = "terbuka"
	// DisputeDitinjau sedang ditinjau.
	DisputeDitinjau DisputeStatus = "ditinjau"
	// DisputeSelesai diterima dan ditindaklanjuti.
	DisputeSelesai DisputeStatus = "selesai"
	// DisputeDitolak ditolak dengan alasan.
	DisputeDitolak DisputeStatus = "ditolak"
)

// DisputeStatusLabel mengembalikan label status siap tampil.
func DisputeStatusLabel(status DisputeStatus) string {
	switch status {
	case DisputeDitinjau:
		return "Sedang ditinjau"
	case DisputeSelesai:
		return "Selesai ditindaklanjuti"
	case DisputeDitolak:
		return "Ditolak"
	default:
		return "Terbuka"
	}
}

// AssessmentDispute adalah permohonan koreksi hasil asesmen oleh peserta (§2.9).
type AssessmentDispute struct {
	ID           string        `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	AssessmentID string        `gorm:"column:assessment_id;type:uuid;not null" json:"assessmentId"`
	UserID       string        `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	Reason       string        `gorm:"column:reason;not null" json:"reason"`
	Status       DisputeStatus `gorm:"column:status;not null;default:terbuka" json:"status"`
	Resolution   string        `gorm:"column:resolution;not null;default:''" json:"resolution"`
	ResolvedBy   *string       `gorm:"column:resolved_by;type:uuid" json:"resolvedBy,omitempty"`
	ResolvedAt   *time.Time    `gorm:"column:resolved_at" json:"resolvedAt,omitempty"`
	CreatedAt    time.Time     `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (AssessmentDispute) TableName() string { return "assessment_disputes" }

// BeforeCreate memastikan ID permohonan terisi saat dibuat langsung di Go.
func (d *AssessmentDispute) BeforeCreate(*gorm.DB) error {
	if d.ID == "" {
		d.ID = NewID()
	}
	return nil
}

// StatusLabel mengembalikan label status siap tampil.
func (d AssessmentDispute) StatusLabel() string { return DisputeStatusLabel(d.Status) }

/* --------------------- interpretasi instrumen berlisensi ------------------- */

// InstrumentInterpretation adalah catatan interpretasi psikolog atas hasil
// instrumen berlisensi (§2.6).
type InstrumentInterpretation struct {
	ID             string     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	InstrumentCode string     `gorm:"column:instrument_code;not null" json:"instrumentCode"`
	PsychologistID string     `gorm:"column:psychologist_id;type:uuid;not null" json:"psychologistId"`
	SubjectID      string     `gorm:"column:subject_id;type:uuid;not null" json:"subjectId"`
	AssessmentID   *string    `gorm:"column:assessment_id;type:uuid" json:"assessmentId,omitempty"`
	Interpretation string     `gorm:"column:interpretation;not null" json:"interpretation"`
	RetentionUntil *time.Time `gorm:"column:retention_until;type:date" json:"retentionUntil,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (InstrumentInterpretation) TableName() string { return "instrument_interpretations" }

// BeforeCreate memastikan ID interpretasi terisi saat dibuat langsung di Go.
func (i *InstrumentInterpretation) BeforeCreate(*gorm.DB) error {
	if i.ID == "" {
		i.ID = NewID()
	}
	return nil
}

// Expired menandai masa retensi interpretasi sudah lewat.
func (i InstrumentInterpretation) Expired(at time.Time) bool {
	if i.RetentionUntil == nil {
		return false
	}
	return at.Truncate(24 * time.Hour).After(i.RetentionUntil.Truncate(24 * time.Hour))
}
