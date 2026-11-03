package repository

import (
	"context"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
)

// PsychologistRepository mengakses mitra psikolog, slot konsultasi, pemesanan,
// catatan sesi, jejak akses data asesmen, dan permohonan koreksi (Fase 3).
type PsychologistRepository struct {
	db *gorm.DB
}

// NewPsychologistRepository membentuk repository.
func NewPsychologistRepository(db *gorm.DB) *PsychologistRepository {
	return &PsychologistRepository{db: db}
}

// DB mengembalikan handle GORM (dipakai uji query dry-run).
func (r *PsychologistRepository) DB() *gorm.DB { return r.db }

/* ------------------------------ mitra psikolog ----------------------------- */

// ListPartners mengambil daftar mitra psikolog; `verifiedOnly` menyaring ke mitra
// yang sudah terverifikasi (dipakai katalog publik peserta).
func (r *PsychologistRepository) ListPartners(ctx context.Context, verifiedOnly bool) ([]models.PsychologistPartner, error) {
	var out []models.PsychologistPartner
	query := r.db.WithContext(ctx).Model(&models.PsychologistPartner{}).Order("full_name ASC")
	if verifiedOnly {
		query = query.Where("verification_status = ?", models.PartnerTerverifikasi)
	}
	if err := query.Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat mitra psikolog.")
	}
	return out, nil
}

// FindPartner mengambil satu mitra berdasarkan id.
func (r *PsychologistRepository) FindPartner(ctx context.Context, id string) (*models.PsychologistPartner, error) {
	var item models.PsychologistPartner
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat mitra psikolog.")
	}
	return &item, nil
}

// FindPartnerByUser mencari mitra yang ditautkan ke sebuah akun pengguna.
//
// Dipakai gerbang area `/psikolog`: akun psikolog hanya boleh membuka data
// peserta yang ditugaskan kepadanya (§2.6, §2.8).
func (r *PsychologistRepository) FindPartnerByUser(ctx context.Context, userID string) (*models.PsychologistPartner, error) {
	var item models.PsychologistPartner
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat mitra psikolog.")
	}
	return &item, nil
}

// CreatePartner menyimpan mitra psikolog baru.
func (r *PsychologistRepository) CreatePartner(ctx context.Context, item *models.PsychologistPartner) error {
	if item.ID == "" {
		item.ID = models.NewID()
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		if isUniqueViolation(err) {
			return apperror.Conflict("Akun ini sudah tertaut ke mitra psikolog lain.")
		}
		return apperror.Internal(err, "Gagal menyimpan mitra psikolog.")
	}
	return nil
}

// UpdatePartner mengubah data mitra psikolog.
func (r *PsychologistRepository) UpdatePartner(ctx context.Context, item *models.PsychologistPartner) error {
	updates := map[string]any{
		"user_id":             item.UserID,
		"full_name":           item.FullName,
		"email":               item.Email,
		"phone":               item.Phone,
		"sipp_number":         item.SippNumber,
		"sippk_number":        item.SippkNumber,
		"str_number":          item.StrNumber,
		"specializations":     item.Specializations,
		"verification_status": item.Status,
		"valid_until":         item.ValidUntil,
		"evidence_url":        item.EvidenceURL,
		"notes":               item.Notes,
	}
	if err := r.db.WithContext(ctx).Model(&models.PsychologistPartner{}).
		Where("id = ?", item.ID).Updates(updates).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan mitra psikolog.")
	}
	return nil
}

// VerifyPartner mengubah status verifikasi izin praktik seorang mitra (§4.7).
func (r *PsychologistRepository) VerifyPartner(
	ctx context.Context,
	id string,
	status models.PartnerStatus,
	validUntil *time.Time,
	evidenceURL *string,
	notes string,
) error {
	updates := map[string]any{
		"verification_status": status,
		"valid_until":         validUntil,
		"evidence_url":        evidenceURL,
		"notes":               notes,
	}
	err := r.db.WithContext(ctx).Model(&models.PsychologistPartner{}).
		Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return apperror.Internal(err, "Gagal memperbarui verifikasi mitra psikolog.")
	}
	return nil
}

/* ------------------------------- slot jadwal ------------------------------- */

// CreateSlot membuka satu slot konsultasi baru.
func (r *PsychologistRepository) CreateSlot(ctx context.Context, item *models.ConsultationSlot) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal membuka slot konsultasi.")
	}
	return nil
}

// FindSlot mengambil satu slot berdasarkan id.
func (r *PsychologistRepository) FindSlot(ctx context.Context, id string) (*models.ConsultationSlot, error) {
	var item models.ConsultationSlot
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat slot konsultasi.")
	}
	return &item, nil
}

// ListSlots mengambil slot milik seorang psikolog (opsional hanya yang tersedia).
func (r *PsychologistRepository) ListSlots(
	ctx context.Context,
	psychologistID string,
	from time.Time,
	availableOnly bool,
) ([]models.ConsultationSlot, error) {
	var out []models.ConsultationSlot
	query := r.db.WithContext(ctx).Model(&models.ConsultationSlot{}).
		Where("psychologist_id = ?", psychologistID).
		Where("starts_at >= ?", from).
		Where("active = ?", true).
		Order("starts_at ASC")
	if availableOnly {
		query = query.Where("booked < quota")
	}
	if err := query.Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat slot konsultasi.")
	}
	return out, nil
}

// ReserveSlot menaikkan jumlah pemesan satu slot secara atomik.
//
// Syarat kuota (`booked < quota`) ikut di dalam `WHERE`, sehingga dua pemesanan
// berbarengan tidak bisa melampaui kuota. Mengembalikan `false` bila slot sudah
// penuh atau tidak aktif (§4.5).
func (r *PsychologistRepository) ReserveSlot(ctx context.Context, slotID string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&models.ConsultationSlot{}).
		Where("id = ? AND active = ? AND booked < quota", slotID, true).
		UpdateColumn("booked", gorm.Expr("booked + 1"))
	if result.Error != nil {
		return false, apperror.Internal(result.Error, "Gagal memesan slot konsultasi.")
	}
	return result.RowsAffected == 1, nil
}

// ReleaseSlot menurunkan jumlah pemesan satu slot (dipakai saat pembatalan).
func (r *PsychologistRepository) ReleaseSlot(ctx context.Context, slotID string) error {
	err := r.db.WithContext(ctx).Model(&models.ConsultationSlot{}).
		Where("id = ? AND booked > 0", slotID).
		UpdateColumn("booked", gorm.Expr("booked - 1")).Error
	if err != nil {
		return apperror.Internal(err, "Gagal memperbarui kuota slot konsultasi.")
	}
	return nil
}

/* ------------------------------- pemesanan --------------------------------- */

// CreateBooking menyimpan satu pemesanan konsultasi.
//
// Konflik unik (slot, peserta dengan status aktif) diterjemahkan menjadi
// `CONFLICT` agar peserta tahu ia sudah memesan slot itu (§5.5). Pemesanan yang
// sudah dibatalkan tidak menghalangi pemesanan ulang (unique index parsial).
func (r *PsychologistRepository) CreateBooking(ctx context.Context, item *models.ConsultationBooking) error {
	err := r.db.WithContext(ctx).Create(item).Error
	if err != nil {
		if isUniqueViolation(err) {
			return apperror.Conflict("Anda sudah memesan slot ini.")
		}
		return apperror.Internal(err, "Gagal menyimpan pemesanan konsultasi.")
	}
	return nil
}

// FindBooking mengambil satu pemesanan berdasarkan id.
func (r *PsychologistRepository) FindBooking(ctx context.Context, id string) (*models.ConsultationBooking, error) {
	var item models.ConsultationBooking
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat pemesanan konsultasi.")
	}
	return &item, nil
}

// UpdateBookingStatus mengubah status satu pemesanan.
func (r *PsychologistRepository) UpdateBookingStatus(ctx context.Context, id string, status models.BookingStatus) error {
	err := r.db.WithContext(ctx).Model(&models.ConsultationBooking{}).
		Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return apperror.Internal(err, "Gagal memperbarui status pemesanan konsultasi.")
	}
	return nil
}

// ConsultationBookingRow adalah satu pemesanan beserta slot & mitranya.
type ConsultationBookingRow struct {
	BookingID        string    `gorm:"column:booking_id"`
	UserID           string    `gorm:"column:user_id"`
	UserName         string    `gorm:"column:user_name"`
	PsychologistID   string    `gorm:"column:psychologist_id"`
	PsychologistName string    `gorm:"column:psychologist_name"`
	SlotID           string    `gorm:"column:slot_id"`
	StartsAt         time.Time `gorm:"column:starts_at"`
	EndsAt           time.Time `gorm:"column:ends_at"`
	Mode             string    `gorm:"column:mode"`
	ChannelURL       *string   `gorm:"column:channel_url"`
	Status           string    `gorm:"column:status"`
	Agenda           string    `gorm:"column:agenda"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

// bookingSelect adalah daftar kolom standar untuk query pemesanan.
const bookingSelect = `b.id AS booking_id, b.user_id AS user_id, u.name AS user_name,
	p.id AS psychologist_id, p.full_name AS psychologist_name, b.slot_id AS slot_id,
	s.starts_at AS starts_at, s.ends_at AS ends_at, s.mode AS mode, s.channel_url AS channel_url,
	b.status AS status, b.agenda AS agenda, b.created_at AS created_at`

// bookingJoins menyambungkan pemesanan → slot → mitra → peserta.
const bookingJoins = `JOIN consultation_slots s ON s.id = b.slot_id
	JOIN psychologist_partners p ON p.id = s.psychologist_id
	JOIN users u ON u.id = b.user_id`

// ListBookingsByUser mengambil riwayat konsultasi seorang peserta.
func (r *PsychologistRepository) ListBookingsByUser(ctx context.Context, userID string) ([]ConsultationBookingRow, error) {
	var out []ConsultationBookingRow
	err := r.db.WithContext(ctx).Table("consultation_bookings AS b").
		Select(bookingSelect).Joins(bookingJoins).
		Where("b.user_id = ?", userID).
		Order("s.starts_at DESC").Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat riwayat konsultasi.")
	}
	return out, nil
}

// ListBookingsByPsychologist mengambil agenda/riwayat seorang mitra psikolog.
func (r *PsychologistRepository) ListBookingsByPsychologist(ctx context.Context, psychologistID string) ([]ConsultationBookingRow, error) {
	var out []ConsultationBookingRow
	err := r.db.WithContext(ctx).Table("consultation_bookings AS b").
		Select(bookingSelect).Joins(bookingJoins).
		Where("p.id = ?", psychologistID).
		Order("s.starts_at DESC").Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat agenda psikolog.")
	}
	return out, nil
}

// ConsultationRecapFilter menyaring rekap pemanfaatan layanan konsultasi.
type ConsultationRecapFilter struct {
	PsychologistID string
	Status         string
	From           time.Time
	Limit          int
}

// ListConsultationRecap mengambil rekap pemesanan untuk admin.
//
// Rekap **tidak** memuat isi catatan konsultasi — admin tidak pernah membaca
// catatan mentah (§4.6). Yang tersedia hanya metadata jadwal & status.
func (r *PsychologistRepository) ListConsultationRecap(ctx context.Context, filter ConsultationRecapFilter) ([]ConsultationBookingRow, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query := r.db.WithContext(ctx).Table("consultation_bookings AS b").
		Select(bookingSelect).Joins(bookingJoins)
	if filter.PsychologistID != "" {
		query = query.Where("p.id = ?", filter.PsychologistID)
	}
	if filter.Status != "" {
		query = query.Where("b.status = ?", filter.Status)
	}
	if !filter.From.IsZero() {
		query = query.Where("s.starts_at >= ?", filter.From)
	}

	var out []ConsultationBookingRow
	if err := query.Order("s.starts_at DESC").Limit(limit).Scan(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat rekap konsultasi.")
	}
	return out, nil
}

// IsPsychologistAssigned menandai mitra memang ditugaskan pada seorang peserta.
//
// Dasar penugasan: ada pemesanan konsultasi antara keduanya. Aturan ini dipakai
// gerbang akses data peserta oleh psikolog (§2.6, §4.6).
func (r *PsychologistRepository) IsPsychologistAssigned(ctx context.Context, psychologistID, userID string) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Table("consultation_bookings AS b").
		Joins("JOIN consultation_slots s ON s.id = b.slot_id").
		Where("s.psychologist_id = ? AND b.user_id = ?", psychologistID, userID).
		Where("b.status <> ?", models.BookingBatal).
		Count(&total).Error
	if err != nil {
		return false, apperror.Internal(err, "Gagal memeriksa penugasan psikolog.")
	}
	return total > 0, nil
}

// BookingStatusCount adalah jumlah pemesanan per status (untuk rekap admin).
type BookingStatusCount struct {
	Status string `gorm:"column:status"`
	Total  int64  `gorm:"column:total"`
}

// CountBookingsByStatus menghitung pemesanan per status.
func (r *PsychologistRepository) CountBookingsByStatus(ctx context.Context, from time.Time) ([]BookingStatusCount, error) {
	var out []BookingStatusCount
	query := r.db.WithContext(ctx).Table("consultation_bookings AS b").
		Select("b.status AS status, COUNT(*) AS total").
		Joins("JOIN consultation_slots s ON s.id = b.slot_id").
		Group("b.status")
	if !from.IsZero() {
		query = query.Where("s.starts_at >= ?", from)
	}
	if err := query.Scan(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung pemesanan konsultasi.")
	}
	return out, nil
}

/* ------------------------------ catatan sesi ------------------------------- */

// CreateNote menyimpan catatan konsultasi.
func (r *PsychologistRepository) CreateNote(ctx context.Context, item *models.ConsultationNote) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan catatan konsultasi.")
	}
	return nil
}

// ListNotesByBooking mengambil catatan sebuah pemesanan (urut terlama dulu).
func (r *PsychologistRepository) ListNotesByBooking(ctx context.Context, bookingID string) ([]models.ConsultationNote, error) {
	var out []models.ConsultationNote
	err := r.db.WithContext(ctx).Model(&models.ConsultationNote{}).
		Where("booking_id = ?", bookingID).Order("created_at ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat catatan konsultasi.")
	}
	return out, nil
}

// ConsultationNoteRow adalah catatan konsultasi beserta konteks pemesanan.
type ConsultationNoteRow struct {
	NoteID         string     `gorm:"column:note_id"`
	BookingID      string     `gorm:"column:booking_id"`
	UserID         string     `gorm:"column:user_id"`
	UserName       string     `gorm:"column:user_name"`
	PsychologistID string     `gorm:"column:psychologist_id"`
	AuthorID       string     `gorm:"column:author_id"`
	Summary        string     `gorm:"column:summary"`
	Recommendation string     `gorm:"column:recommendation"`
	Visibility     string     `gorm:"column:visibility"`
	RetentionUntil *time.Time `gorm:"column:retention_until"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

// ListNotesSharedToUser mengambil catatan yang dibagikan ke seorang peserta.
//
// Hanya `shared_to_user`/`shared_to_hr` yang dikembalikan; catatan `private`
// tidak pernah keluar dari akun psikolog (§4.5).
func (r *PsychologistRepository) ListNotesSharedToUser(ctx context.Context, userID string) ([]ConsultationNoteRow, error) {
	return r.listNotesByUsers(ctx, []string{userID}, true)
}

// ListNotesSharedToHR mengambil catatan berbagi HR untuk kumpulan peserta.
func (r *PsychologistRepository) ListNotesSharedToHR(ctx context.Context, userIDs []string) ([]ConsultationNoteRow, error) {
	return r.listNotesByUsers(ctx, userIDs, false)
}

func (r *PsychologistRepository) listNotesByUsers(ctx context.Context, userIDs []string, sharedToUser bool) ([]ConsultationNoteRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	visibilities := []models.NoteVisibility{models.NoteSharedToHR}
	if sharedToUser {
		visibilities = append(visibilities, models.NoteSharedToUser)
	}
	var out []ConsultationNoteRow
	err := r.db.WithContext(ctx).Table("consultation_notes AS n").
		Select(`n.id AS note_id, n.booking_id AS booking_id, b.user_id AS user_id,
			u.name AS user_name, s.psychologist_id AS psychologist_id, n.author_id AS author_id,
			n.summary AS summary, n.recommendation AS recommendation, n.visibility AS visibility,
			n.retention_until AS retention_until, n.created_at AS created_at`).
		Joins("JOIN consultation_bookings b ON b.id = n.booking_id").
		Joins("JOIN consultation_slots s ON s.id = b.slot_id").
		Joins("JOIN users u ON u.id = b.user_id").
		Where("b.user_id IN ?", userIDs).
		Where("n.visibility IN ?", visibilities).
		Order("n.created_at DESC").Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat catatan konsultasi.")
	}
	return out, nil
}

/* ---------------------------- jejak akses data ----------------------------- */

// CreateAccessLog mencatat satu akses data asesmen (§4.6).
func (r *PsychologistRepository) CreateAccessLog(ctx context.Context, item *models.AssessmentAccessLog) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat jejak akses data asesmen.")
	}
	return nil
}

// AccessLogRow adalah satu baris jejak akses beserta nama pelakunya.
type AccessLogRow struct {
	ID         string    `gorm:"column:id"`
	ActorID    *string   `gorm:"column:actor_id"`
	ActorName  string    `gorm:"column:actor_name"`
	ActorRole  string    `gorm:"column:actor_role"`
	SubjectID  string    `gorm:"column:subject_id"`
	Resource   string    `gorm:"column:resource"`
	ResourceID string    `gorm:"column:resource_id"`
	Action     string    `gorm:"column:action"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

// ListAccessLogs mengambil jejak akses untuk satu peserta (terbaru dulu).
func (r *PsychologistRepository) ListAccessLogs(ctx context.Context, subjectID string, limit int) ([]AccessLogRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []AccessLogRow
	err := r.db.WithContext(ctx).Table("assessment_access_logs AS l").
		Select(`l.id AS id, l.actor_id AS actor_id, COALESCE(u.name, '') AS actor_name,
			COALESCE(u.role::text, '') AS actor_role, l.subject_id AS subject_id,
			l.resource AS resource, l.resource_id AS resource_id, l.action AS action,
			l.created_at AS created_at`).
		Joins("LEFT JOIN users u ON u.id = l.actor_id").
		Where("l.subject_id = ?", subjectID).
		Order("l.created_at DESC").Limit(limit).Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jejak akses data asesmen.")
	}
	return out, nil
}

/* --------------------------- koreksi hasil peserta ------------------------- */

// CreateDispute menyimpan permohonan koreksi hasil asesmen.
func (r *PsychologistRepository) CreateDispute(ctx context.Context, item *models.AssessmentDispute) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan permohonan koreksi.")
	}
	return nil
}

// ListDisputesByUser mengambil permohonan koreksi milik seorang peserta.
func (r *PsychologistRepository) ListDisputesByUser(ctx context.Context, userID string) ([]models.AssessmentDispute, error) {
	var out []models.AssessmentDispute
	err := r.db.WithContext(ctx).Model(&models.AssessmentDispute{}).
		Where("user_id = ?", userID).Order("created_at DESC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat permohonan koreksi.")
	}
	return out, nil
}

// FindAssessmentForUser mengambil satu sesi asesmen milik peserta.
//
// Dipakai gerbang kepemilikan saat peserta mengajukan koreksi hasil (§2.9).
func (r *PsychologistRepository) FindAssessmentForUser(ctx context.Context, assessmentID, userID string) (*models.TalentAssessment, error) {
	var item models.TalentAssessment
	err := r.db.WithContext(ctx).Model(&models.TalentAssessment{}).
		Where("id = ? AND user_id = ?", assessmentID, userID).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat sesi asesmen.")
	}
	return &item, nil
}

/* -------------------------- akses kohort instansi -------------------------- */

// CohortIDsForAdminInstansi mengambil daftar kohort yang dikelola seorang
// `admin_instansi` (keanggotaan berperan admin_instansi).
//
// Dipakai membatasi HR instansi pada kohortnya sendiri (§4.6).
func (r *PsychologistRepository) CohortIDsForAdminInstansi(ctx context.Context, userID string) ([]string, error) {
	var out []string
	err := r.db.WithContext(ctx).Table("cohort_members").
		Select("cohort_id").
		Where("user_id = ? AND role = ?", userID, models.CohortRoleAdminInstansi).
		Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kohort instansi pengguna.")
	}
	return out, nil
}

// UserInCohorts menandai peserta berada di salah satu kohort yang diberikan.
func (r *PsychologistRepository) UserInCohorts(ctx context.Context, userID string, cohortIDs []string) (bool, error) {
	if len(cohortIDs) == 0 {
		return false, nil
	}
	var total int64
	err := r.db.WithContext(ctx).Table("cohort_members").
		Where("user_id = ? AND cohort_id IN ?", userID, cohortIDs).
		Count(&total).Error
	if err != nil {
		return false, apperror.Internal(err, "Gagal memeriksa keanggotaan kohort peserta.")
	}
	return total > 0, nil
}

/* --------------------- interpretasi instrumen berlisensi ------------------- */

// CreateInterpretation menyimpan catatan interpretasi psikolog.
func (r *PsychologistRepository) CreateInterpretation(ctx context.Context, item *models.InstrumentInterpretation) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan interpretasi instrumen.")
	}
	return nil
}

// ListInterpretations mengambil interpretasi instrumen seorang peserta.
func (r *PsychologistRepository) ListInterpretations(ctx context.Context, subjectID string) ([]models.InstrumentInterpretation, error) {
	var out []models.InstrumentInterpretation
	err := r.db.WithContext(ctx).Model(&models.InstrumentInterpretation{}).
		Where("subject_id = ?", subjectID).Order("created_at DESC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat interpretasi instrumen.")
	}
	return out, nil
}

/* ------------------------------- persetujuan ------------------------------- */

// LatestConsent mengambil rekaman persetujuan terakhir yang **masih berlaku**
// untuk satu tujuan.
//
// Dipakai gerbang consent konsultasi: pemesanan ditolak bila peserta belum
// menyetujui pemrosesan data psikologis (§4.5, §4.7). Persetujuan yang sudah
// ditarik (`revoked_at` terisi) tidak dihitung sebagai persetujuan aktif,
// sehingga penarikan persetujuan menghentikan pemrosesan baru.
func (r *PsychologistRepository) LatestConsent(ctx context.Context, userID, purpose string) (*models.ConsentRecord, error) {
	var item models.ConsentRecord
	err := r.db.WithContext(ctx).Model(&models.ConsentRecord{}).
		Where("user_id = ? AND purpose = ? AND revoked_at IS NULL", userID, purpose).
		Order("granted_at DESC").First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat rekaman persetujuan.")
	}
	return &item, nil
}

// ListConsents mengambil seluruh rekaman persetujuan seorang peserta (termasuk
// yang sudah ditarik) agar peserta bisa melihat riwayat persetujuannya (§4.7).
func (r *PsychologistRepository) ListConsents(ctx context.Context, userID string) ([]models.ConsentRecord, error) {
	var out []models.ConsentRecord
	err := r.db.WithContext(ctx).Model(&models.ConsentRecord{}).
		Where("user_id = ?", userID).
		Order("granted_at DESC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat riwayat persetujuan.")
	}
	return out, nil
}

// RevokeConsent menarik seluruh persetujuan aktif untuk satu tujuan.
//
// Mengembalikan jumlah rekaman yang ditarik. Riwayat yang sudah ditarik tidak
// dihapus (kewajiban penyimpanan bukti persetujuan) — hanya `revoked_at` diisi
// sehingga pemrosesan baru berhenti (§4.7).
func (r *PsychologistRepository) RevokeConsent(ctx context.Context, userID, purpose string, at time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ConsentRecord{}).
		Where("user_id = ? AND purpose = ? AND revoked_at IS NULL", userID, purpose).
		Update("revoked_at", at)
	if result.Error != nil {
		return 0, apperror.Internal(result.Error, "Gagal menarik persetujuan.")
	}
	return result.RowsAffected, nil
}

// CreateConsent mencatat persetujuan baru (dipakai alur konsultasi).
func (r *PsychologistRepository) CreateConsent(ctx context.Context, item *models.ConsentRecord) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan rekaman persetujuan.")
	}
	return nil
}
