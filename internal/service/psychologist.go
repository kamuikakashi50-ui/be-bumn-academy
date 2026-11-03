package service

import (
	"context"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// PsychologistService mengelola mitra psikolog, konsultasi daring, jejak akses
// data asesmen, dan koreksi hasil (Fase 3 langkah 6–7).
//
// Aturan yang dipegang:
//   - mitra hanya boleh ditugaskan bila izinnya **terverifikasi & belum lewat**
//     (§4.7), dan akses data peserta diperiksa per-resource, bukan hanya per-peran;
//   - data psikologis butuh persetujuan eksplisit sebelum diproses (§4.7, UU PDP);
//   - catatan konsultasi berlapis visibilitas; admin tidak pernah membaca isinya
//     (§4.5, §4.6);
//   - setiap akses pihak ketiga ke data asesmen dicatat (`assessment_access_logs`).
type PsychologistService struct {
	repo       *repository.PsychologistRepository
	talent     *repository.TalentRepository
	competency *repository.CompetencyRepository
	users      *repository.UserRepository
	now        func() time.Time
}

// NewPsychologistService membentuk service psikolog & konsultasi.
func NewPsychologistService(
	repo *repository.PsychologistRepository,
	talent *repository.TalentRepository,
	competency *repository.CompetencyRepository,
	users *repository.UserRepository,
) *PsychologistService {
	return &PsychologistService{
		repo:       repo,
		talent:     talent,
		competency: competency,
		users:      users,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

/* ---------------------------- helper murni -------------------------------- */

// consultationRejection menentukan alasan sebuah pemesanan ditolak (§4.5).
//
// Mengembalikan pesan kosong bila pemesanan boleh dilanjutkan. Aturan ini murni
// supaya bisa diuji tanpa database.
func consultationRejection(slot models.ConsultationSlot, alreadyBooked bool, now time.Time) string {
	if !slot.Active {
		return "Slot ini sudah ditutup."
	}
	if !slot.StartsAt.After(now) {
		return "Jadwal slot ini sudah lewat."
	}
	if alreadyBooked {
		return "Anda sudah memesan slot ini."
	}
	if !slot.HasRoom() {
		return "Kuota slot ini sudah penuh."
	}
	return ""
}

// consultationCancelRejection menentukan alasan pembatalan ditolak (§4.5).
func consultationCancelRejection(booking models.ConsultationBooking, startsAt, now time.Time) string {
	if booking.Status != models.BookingTerpesan {
		return "Pemesanan ini tidak bisa dibatalkan lagi."
	}
	if !booking.Cancellable(now, startsAt) {
		return "Pembatalan hanya bisa dilakukan minimal 12 jam sebelum jadwal."
	}
	return ""
}

// noteIsMasked menandai isi catatan/interpretasi yang harus disamarkan karena
// masa retensi sudah lewat (§4.5, §4.7).
func noteIsMasked(retentionUntil *time.Time, at time.Time) bool {
	if retentionUntil == nil {
		return false
	}
	return at.Truncate(24 * time.Hour).After(retentionUntil.Truncate(24 * time.Hour))
}

// maskedSummary adalah teks pengganti untuk isi yang masa retensinya lewat.
const maskedSummary = "Isi catatan sudah disamarkan karena masa retensi berakhir."

// parseOptionalDate membaca tanggal `YYYY-MM-DD` (opsional).
//
// Dipakai tanggal izin praktik & masa retensi catatan, yang boleh dikosongkan.
func parseOptionalDate(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, trimmed)
	if err != nil {
		return nil, apperror.Validation("Tanggal harus berformat YYYY-MM-DD.", nil)
	}
	return &parsed, nil
}

// partnerExpired menandai izin mitra sudah lewat masa berlakunya.
func partnerExpired(partner models.PsychologistPartner, at time.Time) bool {
	if partner.ValidUntil == nil {
		return false
	}
	return at.Truncate(24 * time.Hour).After(partner.ValidUntil.Truncate(24 * time.Hour))
}

// consultationMode menormalkan mode konsultasi.
func consultationMode(mode string) string {
	if strings.TrimSpace(mode) == "offline" {
		return "offline"
	}
	return "online"
}

// consultationQuota menormalkan kuota slot (1–20).
func consultationQuota(quota int) int {
	if quota <= 0 {
		return 1
	}
	if quota > 20 {
		return 20
	}
	return quota
}

/* ------------------------------ mitra psikolog ----------------------------- */

// partnerView menyusun tampilan mitra untuk admin.
func (s *PsychologistService) partnerView(partner models.PsychologistPartner) dto.PsychologistPartnerView {
	now := s.now()
	view := dto.PsychologistPartnerView{
		ID:              partner.ID,
		FullName:        partner.FullName,
		Email:           partner.Email,
		Phone:           partner.Phone,
		SippNumber:      partner.SippNumber,
		Specializations: []string(partner.Specializations),
		Status:          string(partner.Status),
		StatusLabel:     partner.StatusLabel(),
		Notes:           partner.Notes,
		Assignable:      partner.Assignable(now),
		Expired:         partnerExpired(partner, now),
		HasAccount:      partner.UserID != nil && *partner.UserID != "",
	}
	if partner.UserID != nil {
		view.UserID = *partner.UserID
	}
	if partner.SippkNumber != nil {
		view.SippkNumber = *partner.SippkNumber
	}
	if partner.StrNumber != nil {
		view.StrNumber = *partner.StrNumber
	}
	if partner.EvidenceURL != nil {
		view.EvidenceURL = *partner.EvidenceURL
	}
	if partner.ValidUntil != nil {
		view.ValidUntil = partner.ValidUntil.Format("2006-01-02")
	}
	return view
}

// AdminPartners mengambil daftar mitra psikolog untuk admin.
func (s *PsychologistService) AdminPartners(ctx context.Context) ([]dto.PsychologistPartnerView, error) {
	partners, err := s.repo.ListPartners(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PsychologistPartnerView, 0, len(partners))
	for _, partner := range partners {
		out = append(out, s.partnerView(partner))
	}
	return out, nil
}

// UpsertPartner menambah/mengubah mitra psikolog (admin).
//
// `userEmail` opsional menautkan mitra ke akun login; akun harus ada dan belum
// tertaut ke mitra lain.
func (s *PsychologistService) UpsertPartner(ctx context.Context, in dto.PsychologistPartnerInput) ([]dto.PsychologistPartnerView, error) {
	var userID *string
	if email := strings.TrimSpace(in.UserEmail); email != "" {
		user, err := s.users.FindByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, apperror.Validation("Akun dengan email tersebut tidak ditemukan.", nil)
		}
		userID = &user.ID
	}

	specializations := make(models.StringList, 0, len(in.Specializations))
	for _, item := range in.Specializations {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			specializations = append(specializations, trimmed)
		}
	}

	if strings.TrimSpace(in.ID) == "" {
		partner := models.PsychologistPartner{
			UserID:          userID,
			FullName:        strings.TrimSpace(in.FullName),
			Email:           strings.TrimSpace(in.Email),
			Phone:           strings.TrimSpace(in.Phone),
			SippNumber:      strings.TrimSpace(in.SippNumber),
			SippkNumber:     optionalString(in.SippkNumber),
			StrNumber:       optionalString(in.StrNumber),
			Specializations: specializations,
			// Mitra baru selalu mulai dari `pending`: verifikasi izin adalah
			// tindakan sadar admin, bukan efek samping pembuatan data (§4.7).
			Status: models.PartnerPending,
			Notes:  strings.TrimSpace(in.Notes),
		}
		if err := s.repo.CreatePartner(ctx, &partner); err != nil {
			return nil, err
		}
		return s.AdminPartners(ctx)
	}

	found, err := s.repo.FindPartner(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, apperror.NotFound("Mitra psikolog tidak ditemukan.")
	}
	found.FullName = strings.TrimSpace(in.FullName)
	found.Email = strings.TrimSpace(in.Email)
	found.Phone = strings.TrimSpace(in.Phone)
	found.SippNumber = strings.TrimSpace(in.SippNumber)
	found.SippkNumber = optionalString(in.SippkNumber)
	found.StrNumber = optionalString(in.StrNumber)
	found.Specializations = specializations
	found.Notes = strings.TrimSpace(in.Notes)
	if userID != nil {
		found.UserID = userID
	}
	if err := s.repo.UpdatePartner(ctx, found); err != nil {
		return nil, err
	}
	return s.AdminPartners(ctx)
}

// VerifyPartner menetapkan status verifikasi izin praktik seorang mitra (§4.7).
func (s *PsychologistService) VerifyPartner(ctx context.Context, id string, in dto.PsychologistVerifyInput) ([]dto.PsychologistPartnerView, error) {
	found, err := s.repo.FindPartner(ctx, id)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, apperror.NotFound("Mitra psikolog tidak ditemukan.")
	}

	validUntil, err := parseOptionalDate(in.ValidUntil)
	if err != nil {
		return nil, err
	}
	status := models.PartnerStatus(in.Status)
	if !status.Valid() {
		return nil, apperror.Validation("Status verifikasi tidak dikenal.", nil)
	}
	// Mitra terverifikasi tanpa masa berlaku diizinkan (mis. STR seumur hidup),
	// tetapi bila tanggalnya diisi harus di masa depan agar tidak langsung mati.
	if status == models.PartnerTerverifikasi && validUntil != nil && validUntil.Before(s.now()) {
		return nil, apperror.Validation("Masa berlaku izin sudah lewat; perbarui tanggalnya.", nil)
	}

	var evidence *string
	if trimmed := strings.TrimSpace(in.EvidenceURL); trimmed != "" {
		evidence = &trimmed
	}
	if err := s.repo.VerifyPartner(ctx, id, status, validUntil, evidence, strings.TrimSpace(in.Notes)); err != nil {
		return nil, err
	}
	return s.AdminPartners(ctx)
}

/* ------------------------------ slot & booking ----------------------------- */

// slotView menyusun tampilan satu slot.
func slotView(slot models.ConsultationSlot, psychologistName string) dto.ConsultationSlotView {
	view := dto.ConsultationSlotView{
		ID:               slot.ID,
		PsychologistID:   slot.PsychologistID,
		PsychologistName: psychologistName,
		StartsAt:         formatTime(slot.StartsAt),
		EndsAt:           formatTime(slot.EndsAt),
		Mode:             slot.Mode,
		Quota:            slot.Quota,
		Booked:           slot.Booked,
		Remaining:        slot.Remaining(),
		Active:           slot.Active,
	}
	if slot.ChannelURL != nil {
		view.ChannelURL = *slot.ChannelURL
	}
	return view
}

// bookingView menyusun tampilan satu pemesanan beserta catatan yang boleh dibaca.
func (s *PsychologistService) bookingView(
	row repository.ConsultationBookingRow,
	notes []dto.ConsultationNoteView,
) dto.ConsultationBookingView {
	view := dto.ConsultationBookingView{
		ID:               row.BookingID,
		SlotID:           row.SlotID,
		PsychologistID:   row.PsychologistID,
		PsychologistName: row.PsychologistName,
		StartsAt:         formatTime(row.StartsAt),
		EndsAt:           formatTime(row.EndsAt),
		Mode:             row.Mode,
		Status:           row.Status,
		StatusLabel:      models.BookingStatusLabel(models.BookingStatus(row.Status)),
		Agenda:           row.Agenda,
		CreatedAt:        formatTime(row.CreatedAt),
		Cancellable: models.ConsultationBooking{
			Status: models.BookingStatus(row.Status),
		}.Cancellable(s.now(), row.StartsAt),
		Notes: notes,
	}
	if row.ChannelURL != nil {
		view.ChannelURL = *row.ChannelURL
	}
	return view
}

// CreateSlot membuka slot konsultasi baru (admin).
//
// Mitra harus terverifikasi & izinnya belum lewat — menugaskan psikolog tanpa
// izin aktif ditolak (§4.7, §5.5).
func (s *PsychologistService) CreateSlot(ctx context.Context, in dto.ConsultationSlotInput) ([]dto.ConsultationSlotView, error) {
	partner, err := s.repo.FindPartner(ctx, in.PsychologistID)
	if err != nil {
		return nil, err
	}
	if partner == nil {
		return nil, apperror.NotFound("Mitra psikolog tidak ditemukan.")
	}
	if !partner.Assignable(s.now()) {
		return nil, apperror.Validation("Mitra psikolog belum terverifikasi atau izinnya sudah lewat.", nil)
	}

	startsAt, err := time.Parse(time.RFC3339, in.StartsAt)
	if err != nil {
		return nil, apperror.Validation("Waktu mulai harus berformat RFC3339.", nil)
	}
	endsAt, err := time.Parse(time.RFC3339, in.EndsAt)
	if err != nil {
		return nil, apperror.Validation("Waktu selesai harus berformat RFC3339.", nil)
	}
	if !endsAt.After(startsAt) {
		return nil, apperror.Validation("Waktu selesai harus setelah waktu mulai.", nil)
	}

	slot := models.ConsultationSlot{
		PsychologistID: partner.ID,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Mode:           consultationMode(in.Mode),
		Quota:          consultationQuota(in.Quota),
		ChannelURL:     optionalString(in.ChannelURL),
		Active:         true,
	}
	if err := s.repo.CreateSlot(ctx, &slot); err != nil {
		return nil, err
	}
	return s.AdminSlots(ctx, partner.ID)
}

// AdminSlots mengambil seluruh slot milik satu mitra (admin).
func (s *PsychologistService) AdminSlots(ctx context.Context, psychologistID string) ([]dto.ConsultationSlotView, error) {
	partner, err := s.repo.FindPartner(ctx, psychologistID)
	if err != nil {
		return nil, err
	}
	if partner == nil {
		return nil, apperror.NotFound("Mitra psikolog tidak ditemukan.")
	}
	slots, err := s.repo.ListSlots(ctx, psychologistID, time.Time{}, false)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ConsultationSlotView, 0, len(slots))
	for _, slot := range slots {
		out = append(out, slotView(slot, partner.FullName))
	}
	return out, nil
}

// Consultations membentuk rekap pemanfaatan layanan untuk admin.
//
// Rekap **tanpa** isi catatan konsultasi (§4.6).
func (s *PsychologistService) Consultations(ctx context.Context, filter repository.ConsultationRecapFilter) (dto.ConsultationRecapView, error) {
	rows, err := s.repo.ListConsultationRecap(ctx, filter)
	if err != nil {
		return dto.ConsultationRecapView{}, err
	}
	counts, err := s.repo.CountBookingsByStatus(ctx, time.Time{})
	if err != nil {
		return dto.ConsultationRecapView{}, err
	}

	bookings := make([]dto.ConsultationBookingView, 0, len(rows))
	for _, row := range rows {
		bookings = append(bookings, s.bookingView(row, []dto.ConsultationNoteView{}))
	}
	summary := make([]dto.ConsultationStatusSummary, 0, len(counts))
	var total int64
	for _, item := range counts {
		summary = append(summary, dto.ConsultationStatusSummary{
			Status:      item.Status,
			StatusLabel: models.BookingStatusLabel(models.BookingStatus(item.Status)),
			Total:       item.Total,
		})
		total += item.Total
	}

	return dto.ConsultationRecapView{
		Bookings: bookings,
		Summary:  summary,
		Total:    total,
		Notes: []string{
			"Rekap ini tidak memuat isi catatan konsultasi — catatan hanya dibuka psikolog penulis dan peserta pemiliknya.",
			"Jumlah pemesanan per status dipakai mengukur pemanfaatan layanan, bukan menilai peserta.",
		},
	}, nil
}

/* --------------------------------- peserta -------------------------------- */

// consultationConsentVersion adalah versi teks persetujuan konsultasi.
//
// Disimpan pada `consent_records.text_version` supaya isi persetujuan yang
// disetujui peserta bisa ditelusuri bila teksnya berubah (§4.7).
const consultationConsentVersion = "konsultasi-v1"

// PublicPsychologists mengambil mitra terverifikasi untuk peserta.
//
// Hanya mitra yang boleh ditugaskan saat ini yang ditampilkan; nomor izin praktik
// tidak ikut dikirim (§4.6).
func (s *PsychologistService) PublicPsychologists(ctx context.Context) ([]dto.PsychologistPublicView, error) {
	partners, err := s.repo.ListPartners(ctx, true)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]dto.PsychologistPublicView, 0, len(partners))
	for _, partner := range partners {
		if !partner.Assignable(now) {
			// Izin lewat masa berlaku → tidak ditawarkan ke peserta.
			continue
		}
		slots, err := s.repo.ListSlots(ctx, partner.ID, now, true)
		if err != nil {
			return nil, err
		}
		out = append(out, dto.PsychologistPublicView{
			ID:              partner.ID,
			FullName:        partner.FullName,
			Specializations: []string(partner.Specializations),
			OpenSlots:       len(slots),
		})
	}
	return out, nil
}

// PartnerSlots mengambil slot tersedia milik satu mitra (peserta).
func (s *PsychologistService) PartnerSlots(ctx context.Context, partnerID string) ([]dto.ConsultationSlotView, error) {
	partner, err := s.repo.FindPartner(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	if partner == nil || !partner.Assignable(s.now()) {
		return nil, apperror.NotFound("Mitra psikolog tidak ditemukan.")
	}
	slots, err := s.repo.ListSlots(ctx, partnerID, s.now(), true)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ConsultationSlotView, 0, len(slots))
	for _, slot := range slots {
		out = append(out, slotView(slot, partner.FullName))
	}
	return out, nil
}

// Book memesan slot konsultasi untuk peserta (§4.5).
//
// Persetujuan wajib: data psikologis adalah data pribadi spesifik (UU PDP),
// sehingga pemesanan tanpa persetujuan ditolak (§4.7).
func (s *PsychologistService) Book(ctx context.Context, userID string, in dto.ConsultationBookingInput) (dto.ConsultationHistoryView, error) {
	slot, err := s.repo.FindSlot(ctx, in.SlotID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	if slot == nil {
		return dto.ConsultationHistoryView{}, apperror.NotFound("Slot konsultasi tidak ditemukan.")
	}

	partner, err := s.repo.FindPartner(ctx, slot.PsychologistID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	if partner == nil || !partner.Assignable(s.now()) {
		return dto.ConsultationHistoryView{}, apperror.Validation(
			"Mitra psikolog belum terverifikasi atau izinnya sudah lewat.", nil)
	}

	existing, err := s.repo.ListBookingsByUser(ctx, userID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	alreadyBooked := false
	for _, row := range existing {
		if row.SlotID == slot.ID && row.Status != string(models.BookingBatal) {
			alreadyBooked = true
			break
		}
	}
	if reason := consultationRejection(*slot, alreadyBooked, s.now()); reason != "" {
		return dto.ConsultationHistoryView{}, apperror.Conflict(reason)
	}

	consent, err := s.ensureConsultationConsent(ctx, userID, in.ConsentGranted)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}

	reserved, err := s.repo.ReserveSlot(ctx, slot.ID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	if !reserved {
		return dto.ConsultationHistoryView{}, apperror.Conflict("Kuota slot ini sudah penuh.")
	}

	booking := models.ConsultationBooking{
		SlotID:    slot.ID,
		UserID:    userID,
		Status:    models.BookingTerpesan,
		Agenda:    strings.TrimSpace(in.Agenda),
		ConsentID: consent,
	}
	if err := s.repo.CreateBooking(ctx, &booking); err != nil {
		// Pemesanan gagal (mis. balapan data) → kuota dikembalikan agar tidak
		// ada slot yang tampak penuh tanpa pemesanan nyata.
		_ = s.repo.ReleaseSlot(ctx, slot.ID)
		return dto.ConsultationHistoryView{}, err
	}
	return s.MyConsultations(ctx, userID)
}

// consultationConsentRejection memutuskan apakah pemrosesan konsultasi boleh
// dilanjutkan tanpa persetujuan baru (§4.5, §4.7).
//
// Murni supaya bisa diuji tanpa database. Mengembalikan pesan kosong bila boleh
// lanjut. Persetujuan yang sudah ditarik (`revoked_at` terisi) atau belum ada
// sama sekali dianggap belum ada persetujuan — penarikan persetujuan
// menghentikan pemrosesan baru, tetapi tidak menghapus riwayatnya.
func consultationConsentRejection(latest *models.ConsentRecord, granted bool) string {
	if granted {
		return ""
	}
	if latest == nil || latest.RevokedAt != nil {
		return "Persetujuan pemrosesan data konsultasi diperlukan sebelum memesan slot."
	}
	return ""
}

// ensureConsultationConsent memastikan ada persetujuan aktif sebelum memproses.
//
// Mengembalikan id rekaman persetujuan yang dipakai. Bila peserta menyetujui pada
// permintaan ini, persetujuannya dicatat; bila tidak, persetujuan lama yang masih
// berlaku dipakai ulang.
func (s *PsychologistService) ensureConsultationConsent(ctx context.Context, userID string, granted bool) (*string, error) {
	if granted {
		record := models.ConsentRecord{
			UserID:      userID,
			Purpose:     models.ConsentPurposeConsultation,
			TextVersion: consultationConsentVersion,
			GrantedAt:   s.now(),
		}
		if err := s.repo.CreateConsent(ctx, &record); err != nil {
			return nil, err
		}
		return &record.ID, nil
	}

	latest, err := s.repo.LatestConsent(ctx, userID, models.ConsentPurposeConsultation)
	if err != nil {
		return nil, err
	}
	if reason := consultationConsentRejection(latest, granted); reason != "" {
		return nil, apperror.Validation(reason, nil)
	}
	return &latest.ID, nil
}

// MyConsultations mengambil riwayat konsultasi peserta beserta catatan yang
// dibagikan kepadanya (§4.5).
func (s *PsychologistService) MyConsultations(ctx context.Context, userID string) (dto.ConsultationHistoryView, error) {
	rows, err := s.repo.ListBookingsByUser(ctx, userID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	notes, err := s.repo.ListNotesSharedToUser(ctx, userID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}

	notesByBooking := make(map[string][]dto.ConsultationNoteView, len(rows))
	for _, note := range notes {
		notesByBooking[note.BookingID] = append(notesByBooking[note.BookingID], s.noteView(note))
	}

	bookings := make([]dto.ConsultationBookingView, 0, len(rows))
	for _, row := range rows {
		bookingNotes := notesByBooking[row.BookingID]
		if bookingNotes == nil {
			bookingNotes = []dto.ConsultationNoteView{}
		}
		bookings = append(bookings, s.bookingView(row, bookingNotes))
	}

	return dto.ConsultationHistoryView{
		Bookings: bookings,
		Notes:    s.flattenNotes(notes),
		Info: []string{
			"Catatan yang ditandai \"Hanya psikolog\" tidak ditampilkan di sini.",
			"Pembatalan mandiri hanya bisa dilakukan minimal 12 jam sebelum jadwal.",
		},
	}, nil
}

// noteView menyusun tampilan satu catatan konsultasi.
//
// Bila masa retensi sudah lewat, isinya disamarkan otomatis (§4.5).
func (s *PsychologistService) noteView(note repository.ConsultationNoteRow) dto.ConsultationNoteView {
	visibility := models.NoteVisibility(note.Visibility)
	view := dto.ConsultationNoteView{
		ID:              note.NoteID,
		BookingID:       note.BookingID,
		Summary:         note.Summary,
		Recommendation:  note.Recommendation,
		Visibility:      note.Visibility,
		VisibilityLabel: models.NoteVisibilityLabel(visibility),
		CreatedAt:       formatTime(note.CreatedAt),
	}
	if note.RetentionUntil != nil {
		view.RetentionUntil = note.RetentionUntil.Format(time.DateOnly)
	}
	if noteIsMasked(note.RetentionUntil, s.now()) {
		view.Summary = maskedSummary
		view.Recommendation = ""
		view.Masked = true
	}
	return view
}

// flattenNotes menggabungkan catatan per pemesanan menjadi satu daftar.
func (s *PsychologistService) flattenNotes(notes []repository.ConsultationNoteRow) []dto.ConsultationNoteView {
	out := make([]dto.ConsultationNoteView, 0, len(notes))
	for _, note := range notes {
		out = append(out, s.noteView(note))
	}
	return out
}

// Cancel membatalkan pemesanan peserta (§4.5).
func (s *PsychologistService) Cancel(ctx context.Context, userID, bookingID string) (dto.ConsultationHistoryView, error) {
	booking, err := s.repo.FindBooking(ctx, bookingID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	if booking == nil || booking.UserID != userID {
		return dto.ConsultationHistoryView{}, apperror.NotFound("Pemesanan konsultasi tidak ditemukan.")
	}
	slot, err := s.repo.FindSlot(ctx, booking.SlotID)
	if err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	if slot == nil {
		return dto.ConsultationHistoryView{}, apperror.NotFound("Slot konsultasi tidak ditemukan.")
	}
	if reason := consultationCancelRejection(*booking, slot.StartsAt, s.now()); reason != "" {
		return dto.ConsultationHistoryView{}, apperror.Conflict(reason)
	}

	if err := s.repo.UpdateBookingStatus(ctx, booking.ID, models.BookingBatal); err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	if err := s.repo.ReleaseSlot(ctx, slot.ID); err != nil {
		return dto.ConsultationHistoryView{}, err
	}
	return s.MyConsultations(ctx, userID)
}

// DisputeAssessment mencatat permohonan koreksi hasil asesmen (§2.9).
func (s *PsychologistService) DisputeAssessment(ctx context.Context, userID, assessmentID string, in dto.AssessmentDisputeInput) ([]dto.AssessmentDisputeView, error) {
	assessment, err := s.repo.FindAssessmentForUser(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if assessment == nil {
		return nil, apperror.NotFound("Sesi asesmen tidak ditemukan.")
	}
	dispute := models.AssessmentDispute{
		AssessmentID: assessment.ID,
		UserID:       userID,
		Reason:       strings.TrimSpace(in.Reason),
		Status:       models.DisputeTerbuka,
	}
	if err := s.repo.CreateDispute(ctx, &dispute); err != nil {
		return nil, err
	}
	return s.MyDisputes(ctx, userID)
}

// MyDisputes mengambil permohonan koreksi milik peserta.
func (s *PsychologistService) MyDisputes(ctx context.Context, userID string) ([]dto.AssessmentDisputeView, error) {
	items, err := s.repo.ListDisputesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AssessmentDisputeView, 0, len(items))
	for _, item := range items {
		out = append(out, dto.AssessmentDisputeView{
			ID:           item.ID,
			AssessmentID: item.AssessmentID,
			Reason:       item.Reason,
			Status:       string(item.Status),
			StatusLabel:  item.StatusLabel(),
			Resolution:   item.Resolution,
			CreatedAt:    formatTime(item.CreatedAt),
		})
	}
	return out, nil
}

// MyAccessLogs menampilkan jejak akses data asesmen peserta (§4.7).
//
// Peserta berhak tahu siapa membuka datanya — bagian hak akses & transparansi
// UU PDP, bukan sekadar audit internal.
func (s *PsychologistService) MyAccessLogs(ctx context.Context, userID string) (dto.AccessLogListView, error) {
	return s.accessLogs(ctx, userID)
}

// AdminAccessLogs menampilkan jejak akses untuk seorang peserta (admin).
func (s *PsychologistService) AdminAccessLogs(ctx context.Context, userID string) (dto.AccessLogListView, error) {
	return s.accessLogs(ctx, userID)
}

// accessLogs membentuk daftar jejak akses satu peserta.
func (s *PsychologistService) accessLogs(ctx context.Context, subjectID string) (dto.AccessLogListView, error) {
	rows, err := s.repo.ListAccessLogs(ctx, subjectID, 200)
	if err != nil {
		return dto.AccessLogListView{}, err
	}
	logs := make([]dto.AccessLogView, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, dto.AccessLogView{
			ID:         row.ID,
			ActorName:  row.ActorName,
			ActorRole:  row.ActorRole,
			Resource:   row.Resource,
			ResourceID: row.ResourceID,
			Action:     row.Action,
			CreatedAt:  formatTime(row.CreatedAt),
		})
	}
	return dto.AccessLogListView{
		SubjectID: subjectID,
		Logs:      logs,
		Notes: []string{
			"Setiap akses pihak ketiga ke data asesmen dicatat (UU PDP No. 27/2022).",
			"Akses peserta ke datanya sendiri tidak dicatat.",
		},
	}, nil
}

// Consents menampilkan riwayat persetujuan peserta beserta tujuan yang dikenali
// sistem (§4.7, UU PDP) agar peserta bisa melihat & menarik persetujuannya.
func (s *PsychologistService) Consents(ctx context.Context, userID string) (dto.ConsentListView, error) {
	rows, err := s.repo.ListConsents(ctx, userID)
	if err != nil {
		return dto.ConsentListView{}, err
	}
	return s.consentListView(rows), nil
}

// RevokeConsent menarik persetujuan peserta untuk satu tujuan (§4.7).
//
// Penarikan menghentikan pemrosesan **baru** untuk tujuan itu (asesmen atau
// konsultasi berikutnya wajib meminta persetujuan ulang), tetapi tidak menghapus
// riwayat yang wajib disimpan sebagai bukti persetujuan.
func (s *PsychologistService) RevokeConsent(ctx context.Context, userID, purpose string) (dto.ConsentListView, error) {
	purpose = strings.TrimSpace(purpose)
	if !models.ConsentPurposeValid(purpose) {
		return dto.ConsentListView{}, apperror.Validation("Tujuan persetujuan tidak dikenal.", nil)
	}
	if _, err := s.repo.RevokeConsent(ctx, userID, purpose, s.now()); err != nil {
		return dto.ConsentListView{}, err
	}
	rows, err := s.repo.ListConsents(ctx, userID)
	if err != nil {
		return dto.ConsentListView{}, err
	}
	return s.consentListView(rows), nil
}

// consentListView membentuk daftar persetujuan + status setiap tujuan.
func (s *PsychologistService) consentListView(rows []models.ConsentRecord) dto.ConsentListView {
	consents := make([]dto.ConsentView, 0, len(rows))
	active := make(map[string]bool, len(rows))
	for _, row := range rows {
		view := dto.ConsentView{
			ID:           row.ID,
			Purpose:      row.Purpose,
			PurposeLabel: models.ConsentPurposeLabel(row.Purpose),
			TextVersion:  row.TextVersion,
			GrantedAt:    formatTime(row.GrantedAt),
			Active:       row.RevokedAt == nil,
		}
		if row.RevokedAt != nil {
			view.RevokedAt = formatTime(*row.RevokedAt)
		} else {
			active[row.Purpose] = true
		}
		consents = append(consents, view)
	}

	purposes := make([]dto.ConsentPurposeView, 0, len(models.ConsentPurposes))
	for _, purpose := range models.ConsentPurposes {
		purposes = append(purposes, dto.ConsentPurposeView{
			Purpose: purpose,
			Label:   models.ConsentPurposeLabel(purpose),
			Active:  active[purpose],
		})
	}

	return dto.ConsentListView{
		Consents: consents,
		Purposes: purposes,
		Info: []string{
			"Penarikan persetujuan menghentikan pemrosesan baru; riwayat yang wajib disimpan tetap tersimpan.",
			"Asesmen atau konsultasi berikutnya akan meminta persetujuan ulang.",
		},
	}
}

// recordAccess mencatat satu akses data asesmen bila diperlukan (§4.6).
func (s *PsychologistService) recordAccess(
	ctx context.Context,
	actorID string,
	actorRole models.Role,
	subjectID string,
	resource models.AccessResource,
	resourceID string,
	action models.AccessAction,
) error {
	actor := AccessActor{UserID: actorID, Role: actorRole}
	if !AccessLogNeeded(actor, subjectID) {
		return nil
	}
	actorIDCopy := actorID
	log := models.AssessmentAccessLog{
		ActorID:    &actorIDCopy,
		SubjectID:  subjectID,
		Resource:   resource,
		ResourceID: resourceID,
		Action:     action,
	}
	return s.repo.CreateAccessLog(ctx, &log)
}

/* --------------------------------- psikolog ------------------------------- */

// partnerForUser mengambil mitra psikolog yang tertaut ke akun pemanggil.
//
// Akun berperan `psikolog` tanpa data mitra ditolak: tanpa itu tidak ada izin
// praktik yang bisa diverifikasi (§4.7).
func (s *PsychologistService) partnerForUser(ctx context.Context, userID string) (*models.PsychologistPartner, error) {
	partner, err := s.repo.FindPartnerByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if partner == nil {
		return nil, apperror.Forbidden("Akun ini belum tertaut ke data mitra psikolog.")
	}
	return partner, nil
}

// ensureAssigned memastikan mitra memang ditugaskan pada peserta (§4.6).
func (s *PsychologistService) ensureAssigned(ctx context.Context, partner *models.PsychologistPartner, subjectID string) error {
	assigned, err := s.repo.IsPsychologistAssigned(ctx, partner.ID, subjectID)
	if err != nil {
		return err
	}
	if !assigned {
		return apperror.Forbidden("Peserta ini tidak ditugaskan kepada Anda.")
	}
	return nil
}

// Agenda membentuk agenda sesi seorang psikolog (`GET /psikolog/agenda`).
func (s *PsychologistService) Agenda(ctx context.Context, userID string) (dto.PsychologistAgendaView, error) {
	partner, err := s.partnerForUser(ctx, userID)
	if err != nil {
		return dto.PsychologistAgendaView{}, err
	}
	rows, err := s.repo.ListBookingsByPsychologist(ctx, partner.ID)
	if err != nil {
		return dto.PsychologistAgendaView{}, err
	}

	sessions := make([]dto.PsychologistAgendaItemView, 0, len(rows))
	for _, row := range rows {
		notes, err := s.repo.ListNotesByBooking(ctx, row.BookingID)
		if err != nil {
			return dto.PsychologistAgendaView{}, err
		}
		item := dto.PsychologistAgendaItemView{
			BookingID:   row.BookingID,
			UserID:      row.UserID,
			UserName:    row.UserName,
			Initials:    competencyInitials(row.UserName),
			StartsAt:    formatTime(row.StartsAt),
			EndsAt:      formatTime(row.EndsAt),
			Mode:        row.Mode,
			Status:      row.Status,
			StatusLabel: models.BookingStatusLabel(models.BookingStatus(row.Status)),
			Agenda:      row.Agenda,
			NoteCount:   len(notes),
		}
		if row.ChannelURL != nil {
			item.ChannelURL = *row.ChannelURL
		}
		sessions = append(sessions, item)
	}

	verified := partner.Assignable(s.now())
	view := dto.PsychologistAgendaView{
		PsychologistID:   partner.ID,
		PsychologistName: partner.FullName,
		Status:           string(partner.Status),
		StatusLabel:      partner.StatusLabel(),
		Verified:         verified,
		Sessions:         sessions,
		Notes: []string{
			"Catatan sesi hanya dapat dibaca Anda sebagai penulis, peserta bila dibagikan, dan HR instansi bila Anda memilih \"dibagikan ke HR\".",
			"Jawaban item mentah hanya tersedia untuk peserta yang ditugaskan kepada Anda.",
		},
	}
	if !verified {
		view.Notes = append(view.Notes,
			"Izin praktik Anda belum terverifikasi atau sudah lewat masa berlaku; hubungi admin.")
	}
	return view, nil
}

// Participant mengambil data peserta yang ditugaskan, termasuk jawaban item
// mentah (`GET /psikolog/participants/{id}`).
//
// Akses ini dicatat di `assessment_access_logs` (§4.6).
func (s *PsychologistService) Participant(ctx context.Context, userID, subjectID string) (dto.PsychologistParticipantView, error) {
	partner, err := s.partnerForUser(ctx, userID)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	// Gerbang per-resource: peran `psikolog` saja tidak cukup (§2.8).
	if err := s.ensureAssigned(ctx, partner, subjectID); err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	subject, err := s.users.FindByID(ctx, subjectID)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	if subject == nil {
		return dto.PsychologistParticipantView{}, apperror.NotFound("Peserta tidak ditemukan.")
	}

	assessments, err := s.talent.ListAssessmentsByUser(ctx, subjectID)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	instruments, err := s.talent.ListInstruments(ctx, false)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	nameByInstrument := make(map[string]string, len(instruments))
	kindByInstrument := make(map[string]string, len(instruments))
	for _, instrument := range instruments {
		nameByInstrument[instrument.ID] = instrument.Name
		kindByInstrument[instrument.ID] = instrument.Kind
	}

	views := make([]dto.PsychologistAssessmentView, 0, len(assessments))
	for _, assessment := range assessments {
		items, err := s.talent.ListItems(ctx, assessment.InstrumentID)
		if err != nil {
			return dto.PsychologistParticipantView{}, err
		}
		dimensionByItem := make(map[string]string, len(items))
		for _, item := range items {
			dimensionByItem[item.ID] = item.Dimension
		}
		answers, err := s.talent.ListAnswers(ctx, assessment.ID)
		if err != nil {
			return dto.PsychologistParticipantView{}, err
		}
		answerViews := make([]dto.PsychologistItemAnswerView, 0, len(answers))
		for _, answer := range answers {
			answerViews = append(answerViews, dto.PsychologistItemAnswerView{
				ItemID:    answer.ItemID,
				Dimension: dimensionByItem[answer.ItemID],
				Value:     answer.Value,
			})
		}
		view := dto.PsychologistAssessmentView{
			ID:             assessment.ID,
			InstrumentName: nameByInstrument[assessment.InstrumentID],
			InstrumentKind: kindByInstrument[assessment.InstrumentID],
			Status:         assessment.Status,
			StatusLabel:    models.TalentAssessmentStatusLabel(assessment.Status),
			StartedAt:      formatTime(assessment.StartedAt),
			Answers:        answerViews,
		}
		if assessment.FinishedAt != nil {
			view.FinishedAt = formatTime(*assessment.FinishedAt)
		}
		views = append(views, view)
	}

	interpretations, err := s.repo.ListInterpretations(ctx, subjectID)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	interpretationViews := make([]dto.InstrumentInterpretationView, 0, len(interpretations))
	for _, item := range interpretations {
		interpretationViews = append(interpretationViews, s.interpretationView(item))
	}

	// Jejak akses dicatat setelah data berhasil dibaca.
	if err := s.recordAccess(ctx, userID, models.RolePsikolog, subjectID,
		models.AccessResourceItemAnswers, "", models.AccessActionView); err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	if err := s.recordAccess(ctx, userID, models.RolePsikolog, subjectID,
		models.AccessResourceTalentProfile, "", models.AccessActionView); err != nil {
		return dto.PsychologistParticipantView{}, err
	}

	return dto.PsychologistParticipantView{
		UserID:          subject.ID,
		Name:            subject.Name,
		Initials:        subject.Initials(),
		Program:         subject.Program,
		Email:           subject.Email,
		Assessments:     views,
		Interpretations: interpretationViews,
		Notes: []string{
			"Jawaban item mentah hanya dibuka untuk psikolog yang ditugaskan (Kode Etik Psikologi).",
			"Akses ini tercatat pada jejak akses data asesmen peserta.",
		},
	}, nil
}

// interpretationView menyusun tampilan satu interpretasi instrumen.
func (s *PsychologistService) interpretationView(item models.InstrumentInterpretation) dto.InstrumentInterpretationView {
	view := dto.InstrumentInterpretationView{
		ID:             item.ID,
		InstrumentCode: item.InstrumentCode,
		Interpretation: item.Interpretation,
		CreatedAt:      formatTime(item.CreatedAt),
	}
	if item.RetentionUntil != nil {
		view.RetentionUntil = item.RetentionUntil.Format(time.DateOnly)
	}
	if item.Expired(s.now()) {
		view.Interpretation = maskedSummary
		view.Masked = true
	}
	return view
}

// AddNote menyimpan catatan konsultasi beserta tingkat visibilitasnya (§4.5).
//
// Penulis harus mitra yang ditugaskan pada pemesanan itu; catatan `private`
// tidak pernah keluar dari akun psikolog.
func (s *PsychologistService) AddNote(ctx context.Context, userID, bookingID string, in dto.ConsultationNoteInput) (dto.PsychologistAgendaView, error) {
	partner, err := s.partnerForUser(ctx, userID)
	if err != nil {
		return dto.PsychologistAgendaView{}, err
	}
	booking, err := s.repo.FindBooking(ctx, bookingID)
	if err != nil {
		return dto.PsychologistAgendaView{}, err
	}
	if booking == nil {
		return dto.PsychologistAgendaView{}, apperror.NotFound("Pemesanan konsultasi tidak ditemukan.")
	}
	slot, err := s.repo.FindSlot(ctx, booking.SlotID)
	if err != nil {
		return dto.PsychologistAgendaView{}, err
	}
	if slot == nil || slot.PsychologistID != partner.ID {
		return dto.PsychologistAgendaView{}, apperror.Forbidden("Sesi ini bukan milik Anda.")
	}

	visibility := models.NoteVisibility(in.Visibility)
	if !visibility.Valid() {
		return dto.PsychologistAgendaView{}, apperror.Validation("Tingkat visibilitas catatan tidak dikenal.", nil)
	}
	retention, err := parseOptionalDate(in.RetentionUntil)
	if err != nil {
		return dto.PsychologistAgendaView{}, err
	}

	note := models.ConsultationNote{
		BookingID:      booking.ID,
		AuthorID:       userID,
		Summary:        strings.TrimSpace(in.Summary),
		Recommendation: strings.TrimSpace(in.Recommendation),
		Visibility:     visibility,
		RetentionUntil: retention,
	}
	if err := s.repo.CreateNote(ctx, &note); err != nil {
		return dto.PsychologistAgendaView{}, err
	}
	// Sesi ditandai selesai begitu catatan ditulis — sesi dianggap tuntas.
	if booking.Status == models.BookingTerpesan {
		if err := s.repo.UpdateBookingStatus(ctx, booking.ID, models.BookingSelesai); err != nil {
			return dto.PsychologistAgendaView{}, err
		}
	}
	if err := s.recordAccess(ctx, userID, models.RolePsikolog, booking.UserID,
		models.AccessResourceConsultationNote, note.ID, models.AccessActionUpdate); err != nil {
		return dto.PsychologistAgendaView{}, err
	}
	return s.Agenda(ctx, userID)
}

// AssessCompetency mencatat penilaian kompetensi oleh psikolog.
//
// Penilaian disimpan terpisah dari self-assessment & proksi tryout
// (`source = asesor`), sesuai §2.4 & §4.3.
func (s *PsychologistService) AssessCompetency(ctx context.Context, userID, subjectID string, in dto.PsychologistCompetencyInput) error {
	partner, err := s.partnerForUser(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.ensureAssigned(ctx, partner, subjectID); err != nil {
		return err
	}
	found, err := s.competency.FindCompetency(ctx, in.CompetencyCode)
	if err != nil {
		return err
	}
	if found == nil || !found.Active {
		return apperror.Validation("Kompetensi tidak dikenal atau tidak aktif.", nil)
	}

	item := models.UserCompetency{
		UserID:         subjectID,
		CompetencyCode: found.Code,
		Level:          in.Level,
		Source:         models.CompetencySourceAsesor,
		Evidence:       strings.TrimSpace(in.Evidence),
		AssessedAt:     s.now(),
	}
	if err := s.competency.UpsertUserCompetency(ctx, &item); err != nil {
		return err
	}
	return s.recordAccess(ctx, userID, models.RolePsikolog, subjectID,
		models.AccessResourceCompetency, found.Code, models.AccessActionUpdate)
}

// InterpretInstrument menyimpan catatan interpretasi instrumen berlisensi (§2.6).
//
// Hanya psikolog yang ditugaskan pada peserta itu yang boleh menginterpretasi.
func (s *PsychologistService) InterpretInstrument(ctx context.Context, userID, instrumentCode string, in dto.InstrumentInterpretationInput) (dto.PsychologistParticipantView, error) {
	partner, err := s.partnerForUser(ctx, userID)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	if err := s.ensureAssigned(ctx, partner, in.UserID); err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	code := strings.TrimSpace(instrumentCode)
	if code == "" {
		return dto.PsychologistParticipantView{}, apperror.Validation("Kode instrumen wajib diisi.", nil)
	}
	if !partner.Assignable(s.now()) {
		return dto.PsychologistParticipantView{}, apperror.Forbidden(
			"Izin praktik Anda belum terverifikasi atau sudah lewat masa berlaku.")
	}

	retention, err := parseOptionalDate(in.RetentionUntil)
	if err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	interpretation := models.InstrumentInterpretation{
		InstrumentCode: code,
		PsychologistID: partner.ID,
		SubjectID:      in.UserID,
		Interpretation: strings.TrimSpace(in.Interpretation),
		RetentionUntil: retention,
	}
	if trimmed := strings.TrimSpace(in.AssessmentID); trimmed != "" {
		interpretation.AssessmentID = &trimmed
	}
	if err := s.repo.CreateInterpretation(ctx, &interpretation); err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	if err := s.recordAccess(ctx, userID, models.RolePsikolog, in.UserID,
		models.AccessResourceItemAnswers, code, models.AccessActionUpdate); err != nil {
		return dto.PsychologistParticipantView{}, err
	}
	return s.Participant(ctx, userID, in.UserID)
}
