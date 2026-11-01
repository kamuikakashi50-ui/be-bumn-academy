package dto

/* ------------------------------ mitra psikolog ----------------------------- */

// PsychologistPartnerView adalah satu mitra psikolog untuk admin (lengkap
// dengan nomor izin praktik & masa berlakunya).
type PsychologistPartnerView struct {
	ID              string   `json:"id"`
	UserID          string   `json:"userId,omitempty"`
	FullName        string   `json:"fullName"`
	Email           string   `json:"email"`
	Phone           string   `json:"phone"`
	SippNumber      string   `json:"sippNumber"`
	SippkNumber     string   `json:"sippkNumber,omitempty"`
	StrNumber       string   `json:"strNumber,omitempty"`
	Specializations []string `json:"specializations"`
	Status          string   `json:"status"`
	StatusLabel     string   `json:"statusLabel"`
	ValidUntil      string   `json:"validUntil,omitempty"`
	EvidenceURL     string   `json:"evidenceUrl,omitempty"`
	Notes           string   `json:"notes"`
	// Assignable menandai mitra boleh ditugaskan saat ini (status + masa berlaku).
	Assignable bool `json:"assignable"`
	// Expired menandai masa berlaku izin sudah lewat.
	Expired bool `json:"expired"`
	// HasAccount menandai mitra sudah tertaut ke akun login.
	HasAccount bool `json:"hasAccount"`
}

// PsychologistPublicView adalah mitra psikolog untuk peserta: **tanpa** nomor
// izin praktik, hanya nama, spesialisasi, dan jumlah slot tersedia (§4.6).
type PsychologistPublicView struct {
	ID              string   `json:"id"`
	FullName        string   `json:"fullName"`
	Specializations []string `json:"specializations"`
	// OpenSlots jumlah slot tersedia yang bisa dipesan.
	OpenSlots int `json:"openSlots"`
}

// PsychologistPartnerInput adalah payload tambah/ubah mitra psikolog (admin).
type PsychologistPartnerInput struct {
	// ID kosong berarti mitra baru.
	ID              string   `json:"id" validate:"omitempty,max=60"`
	UserEmail       string   `json:"userEmail" validate:"omitempty,email,max=160"`
	FullName        string   `json:"fullName" validate:"required,min=3,max=160"`
	Email           string   `json:"email" validate:"omitempty,email,max=160"`
	Phone           string   `json:"phone" validate:"omitempty,max=40"`
	SippNumber      string   `json:"sippNumber" validate:"required,min=3,max=60"`
	SippkNumber     string   `json:"sippkNumber" validate:"omitempty,max=60"`
	StrNumber       string   `json:"strNumber" validate:"omitempty,max=60"`
	Specializations []string `json:"specializations" validate:"omitempty,dive,max=80"`
	Notes           string   `json:"notes" validate:"omitempty,max=400"`
}

// PsychologistVerifyInput adalah payload verifikasi izin praktik mitra (§4.7).
type PsychologistVerifyInput struct {
	// Status: terverifikasi|nonaktif|pending.
	Status      string `json:"status" validate:"required,oneof=terverifikasi nonaktif pending"`
	ValidUntil  string `json:"validUntil" validate:"omitempty,datetime=2006-01-02"`
	EvidenceURL string `json:"evidenceUrl" validate:"omitempty,url,max=300"`
	Notes       string `json:"notes" validate:"omitempty,max=400"`
}

/* ------------------------------ slot & booking ----------------------------- */

// ConsultationSlotView adalah satu slot jadwal konsultasi.
type ConsultationSlotView struct {
	ID               string `json:"id"`
	PsychologistID   string `json:"psychologistId"`
	PsychologistName string `json:"psychologistName"`
	StartsAt         string `json:"startsAt"`
	EndsAt           string `json:"endsAt"`
	Mode             string `json:"mode"`
	Quota            int    `json:"quota"`
	Booked           int    `json:"booked"`
	Remaining        int    `json:"remaining"`
	ChannelURL       string `json:"channelUrl,omitempty"`
	Active           bool   `json:"active"`
}

// ConsultationSlotInput adalah payload membuka slot konsultasi (admin).
type ConsultationSlotInput struct {
	PsychologistID string `json:"psychologistId" validate:"required,uuid"`
	StartsAt       string `json:"startsAt" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndsAt         string `json:"endsAt" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Mode           string `json:"mode" validate:"omitempty,oneof=online offline"`
	Quota          int    `json:"quota" validate:"omitempty,gte=1,lte=20"`
	ChannelURL     string `json:"channelUrl" validate:"omitempty,url,max=300"`
}

// ConsultationBookingInput adalah payload memesan slot (peserta).
type ConsultationBookingInput struct {
	SlotID string `json:"slotId" validate:"required,uuid"`
	Agenda string `json:"agenda" validate:"omitempty,max=400"`
	// ConsentGranted wajib true: data psikologis butuh persetujuan eksplisit.
	ConsentGranted bool `json:"consentGranted"`
}

// ConsultationNoteView adalah catatan konsultasi yang boleh dibaca pemanggil.
type ConsultationNoteView struct {
	ID              string `json:"id"`
	BookingID       string `json:"bookingId"`
	AuthorName      string `json:"authorName,omitempty"`
	Summary         string `json:"summary"`
	Recommendation  string `json:"recommendation,omitempty"`
	Visibility      string `json:"visibility"`
	VisibilityLabel string `json:"visibilityLabel"`
	RetentionUntil  string `json:"retentionUntil,omitempty"`
	CreatedAt       string `json:"createdAt"`
	// Masked menandai isi disamarkan karena masa retensi sudah lewat.
	Masked bool `json:"masked"`
}

// ConsultationBookingView adalah satu pemesanan beserta konteks jadwalnya.
type ConsultationBookingView struct {
	ID               string `json:"id"`
	SlotID           string `json:"slotId"`
	PsychologistID   string `json:"psychologistId"`
	PsychologistName string `json:"psychologistName"`
	StartsAt         string `json:"startsAt"`
	EndsAt           string `json:"endsAt"`
	Mode             string `json:"mode"`
	ChannelURL       string `json:"channelUrl,omitempty"`
	Status           string `json:"status"`
	StatusLabel      string `json:"statusLabel"`
	Agenda           string `json:"agenda,omitempty"`
	CreatedAt        string `json:"createdAt"`
	// Cancellable menandai peserta masih boleh membatalkan sendiri (≥ 12 jam).
	Cancellable bool                   `json:"cancellable"`
	Notes       []ConsultationNoteView `json:"notes"`
}

// ConsultationHistoryView adalah balasan `GET /me/consultations`.
type ConsultationHistoryView struct {
	Bookings []ConsultationBookingView `json:"bookings"`
	// Notes berisi catatan yang dibagikan psikolog ke peserta (yang `private`
	// tidak pernah dikirim).
	Notes []ConsultationNoteView `json:"notes"`
	Info  []string               `json:"info"`
}

// ConsultationStatusSummary adalah jumlah pemesanan per status.
type ConsultationStatusSummary struct {
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	Total       int64  `json:"total"`
}

// ConsultationRecapView adalah balasan `GET /admin/consultations`.
//
// Rekap **tidak** memuat isi catatan konsultasi (§4.6) — hanya metadata jadwal,
// status, dan jumlah per status.
type ConsultationRecapView struct {
	Bookings []ConsultationBookingView   `json:"bookings"`
	Summary  []ConsultationStatusSummary `json:"summary"`
	Total    int64                       `json:"total"`
	Notes    []string                    `json:"notes"`
}

/* --------------------------------- psikolog -------------------------------- */

// ConsultationNoteInput adalah payload menulis catatan konsultasi (psikolog).
type ConsultationNoteInput struct {
	Summary        string `json:"summary" validate:"required,min=10,max=1000"`
	Recommendation string `json:"recommendation" validate:"omitempty,max=1000"`
	Visibility     string `json:"visibility" validate:"required,oneof=private shared_to_user shared_to_hr"`
	RetentionUntil string `json:"retentionUntil" validate:"omitempty,datetime=2006-01-02"`
}

// PsychologistAgendaItemView adalah satu sesi pada agenda psikolog.
type PsychologistAgendaItemView struct {
	BookingID   string `json:"bookingId"`
	UserID      string `json:"userId"`
	UserName    string `json:"userName"`
	Initials    string `json:"initials"`
	StartsAt    string `json:"startsAt"`
	EndsAt      string `json:"endsAt"`
	Mode        string `json:"mode"`
	ChannelURL  string `json:"channelUrl,omitempty"`
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	Agenda      string `json:"agenda,omitempty"`
	NoteCount   int    `json:"noteCount"`
}

// PsychologistAgendaView adalah balasan `GET /psikolog/agenda`.
type PsychologistAgendaView struct {
	PsychologistID   string                       `json:"psychologistId"`
	PsychologistName string                       `json:"psychologistName"`
	Status           string                       `json:"status"`
	StatusLabel      string                       `json:"statusLabel"`
	Verified         bool                         `json:"verified"`
	Sessions         []PsychologistAgendaItemView `json:"sessions"`
	Notes            []string                     `json:"notes"`
}

// PsychologistItemAnswerView adalah satu jawaban item mentah.
//
// Hanya psikolog yang ditugaskan yang menerima data ini (§4.6, Kode Etik).
type PsychologistItemAnswerView struct {
	ItemID    string `json:"itemId"`
	Dimension string `json:"dimension"`
	Value     int    `json:"value"`
}

// PsychologistAssessmentView adalah satu sesi asesmen peserta beserta jawabannya.
type PsychologistAssessmentView struct {
	ID             string                       `json:"id"`
	InstrumentName string                       `json:"instrumentName"`
	InstrumentKind string                       `json:"instrumentKind"`
	Status         string                       `json:"status"`
	StatusLabel    string                       `json:"statusLabel"`
	StartedAt      string                       `json:"startedAt,omitempty"`
	FinishedAt     string                       `json:"finishedAt,omitempty"`
	Answers        []PsychologistItemAnswerView `json:"answers"`
}

// PsychologistParticipantView adalah balasan `GET /psikolog/participants/{id}`.
type PsychologistParticipantView struct {
	UserID          string                         `json:"userId"`
	Name            string                         `json:"name"`
	Initials        string                         `json:"initials"`
	Program         string                         `json:"program"`
	Email           string                         `json:"email"`
	Assessments     []PsychologistAssessmentView   `json:"assessments"`
	Interpretations []InstrumentInterpretationView `json:"interpretations"`
	Notes           []string                       `json:"notes"`
}

// PsychologistCompetencyInput adalah payload penilaian kompetensi oleh psikolog.
type PsychologistCompetencyInput struct {
	CompetencyCode string `json:"competencyCode" validate:"required,max=60"`
	Level          int    `json:"level" validate:"required,gte=1,lte=5"`
	Evidence       string `json:"evidence" validate:"omitempty,max=400"`
}

// InstrumentInterpretationView adalah satu catatan interpretasi instrumen.
type InstrumentInterpretationView struct {
	ID             string `json:"id"`
	InstrumentCode string `json:"instrumentCode"`
	Interpretation string `json:"interpretation"`
	RetentionUntil string `json:"retentionUntil,omitempty"`
	CreatedAt      string `json:"createdAt"`
	Masked         bool   `json:"masked"`
}

// InstrumentInterpretationInput adalah payload catatan interpretasi psikolog.
type InstrumentInterpretationInput struct {
	UserID         string `json:"userId" validate:"required,uuid"`
	AssessmentID   string `json:"assessmentId" validate:"omitempty,uuid"`
	Interpretation string `json:"interpretation" validate:"required,min=10,max=2000"`
	RetentionUntil string `json:"retentionUntil" validate:"omitempty,datetime=2006-01-02"`
}

/* --------------------------- koreksi & jejak akses ------------------------- */

// AssessmentDisputeInput adalah payload mengajukan koreksi hasil asesmen.
type AssessmentDisputeInput struct {
	Reason string `json:"reason" validate:"required,min=10,max=600"`
}

// AssessmentDisputeView adalah satu permohonan koreksi milik peserta.
type AssessmentDisputeView struct {
	ID           string `json:"id"`
	AssessmentID string `json:"assessmentId"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
	StatusLabel  string `json:"statusLabel"`
	Resolution   string `json:"resolution,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

// AccessLogView adalah satu baris jejak akses data asesmen (§4.6).
type AccessLogView struct {
	ID         string `json:"id"`
	ActorName  string `json:"actorName"`
	ActorRole  string `json:"actorRole"`
	Resource   string `json:"resource"`
	ResourceID string `json:"resourceId,omitempty"`
	Action     string `json:"action"`
	CreatedAt  string `json:"createdAt"`
}

// AccessLogListView adalah balasan jejak akses untuk satu peserta.
type AccessLogListView struct {
	SubjectID string          `json:"subjectId"`
	Logs      []AccessLogView `json:"logs"`
	Notes     []string        `json:"notes"`
}
