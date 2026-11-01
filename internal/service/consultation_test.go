package service

import (
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji aturan konsultasi daring (Fase 3 §4.5) dan gerbang izin praktik (§4.7).

func slotAt(startsIn time.Duration, quota, booked int) models.ConsultationSlot {
	now := time.Now().UTC()
	return models.ConsultationSlot{
		ID:       "slot-1",
		StartsAt: now.Add(startsIn),
		EndsAt:   now.Add(startsIn + time.Hour),
		Mode:     "online",
		Quota:    quota,
		Booked:   booked,
		Active:   true,
	}
}

func TestConsultationRejection(t *testing.T) {
	now := time.Now().UTC()
	closed := slotAt(48*time.Hour, 2, 0)
	closed.Active = false

	cases := []struct {
		name          string
		slot          models.ConsultationSlot
		alreadyBooked bool
		wantEmpty     bool
	}{
		{"slot tersedia diterima", slotAt(48*time.Hour, 2, 0), false, true},
		{"slot ditutup ditolak", closed, false, false},
		{"jadwal sudah lewat ditolak", slotAt(-2*time.Hour, 2, 0), false, false},
		{"sudah memesan slot itu ditolak", slotAt(48*time.Hour, 2, 0), true, false},
		{"kuota penuh ditolak", slotAt(48*time.Hour, 2, 2), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := consultationRejection(tc.slot, tc.alreadyBooked, now)
			if tc.wantEmpty && got != "" {
				t.Errorf("harap diterima, tapi ditolak: %s", got)
			}
			if !tc.wantEmpty && got == "" {
				t.Error("harap ditolak, tapi diterima")
			}
		})
	}
}

func TestConsultationRejectionUrutanPrioritas(t *testing.T) {
	// Slot penuh sekaligus sudah dipesan → alasan "sudah memesan" lebih dulu
	// supaya peserta tidak bingung dengan pesan kuota.
	got := consultationRejection(slotAt(48*time.Hour, 1, 1), true, time.Now().UTC())
	if got == "" || got == "Kuota slot ini sudah penuh." {
		t.Errorf("alasan = %q, harap alasan \"sudah memesan slot ini\"", got)
	}
}

func TestConsultationCancelRejection(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name      string
		status    models.BookingStatus
		startsIn  time.Duration
		wantEmpty bool
	}{
		{"lebih dari 12 jam boleh dibatalkan", models.BookingTerpesan, 48 * time.Hour, true},
		{"tepat 12 jam boleh dibatalkan", models.BookingTerpesan, 12 * time.Hour, true},
		{"kurang dari 12 jam ditolak", models.BookingTerpesan, 11*time.Hour + 59*time.Minute, false},
		{"sudah selesai tidak bisa dibatalkan", models.BookingSelesai, 48 * time.Hour, false},
		{"sudah batal tidak bisa dibatalkan lagi", models.BookingBatal, 48 * time.Hour, false},
		{"tidak hadir tidak bisa dibatalkan", models.BookingTidakHadir, 48 * time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			booking := models.ConsultationBooking{Status: tc.status}
			got := consultationCancelRejection(booking, now.Add(tc.startsIn), now)
			if tc.wantEmpty && got != "" {
				t.Errorf("harap boleh dibatalkan, tapi ditolak: %s", got)
			}
			if !tc.wantEmpty && got == "" {
				t.Error("harap ditolak, tapi boleh dibatalkan")
			}
		})
	}
}

func TestPartnerAssignableMenghormatiIzinPraktik(t *testing.T) {
	now := time.Now().UTC()
	future := now.AddDate(0, 0, 30)
	past := now.AddDate(0, 0, -1)

	cases := []struct {
		name    string
		partner models.PsychologistPartner
		want    bool
	}{
		{"pending tidak boleh ditugaskan", models.PsychologistPartner{Status: models.PartnerPending}, false},
		{"nonaktif tidak boleh ditugaskan", models.PsychologistPartner{Status: models.PartnerNonaktif, ValidUntil: &future}, false},
		{"terverifikasi tanpa masa berlaku boleh", models.PsychologistPartner{Status: models.PartnerTerverifikasi}, true},
		{"terverifikasi & izin berlaku boleh", models.PsychologistPartner{Status: models.PartnerTerverifikasi, ValidUntil: &future}, true},
		{"terverifikasi tapi izin lewat ditolak", models.PsychologistPartner{Status: models.PartnerTerverifikasi, ValidUntil: &past}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.partner.Assignable(now); got != tc.want {
				t.Errorf("Assignable = %v, harap %v", got, tc.want)
			}
		})
	}
}

func TestPartnerAssignableIzinBerlakuSampaiAkhirHari(t *testing.T) {
	// Izin berlaku sampai akhir tanggal `valid_until`, bukan awal hari.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	partner := models.PsychologistPartner{Status: models.PartnerTerverifikasi, ValidUntil: &today}
	if !partner.Assignable(today.Add(20 * time.Hour)) {
		t.Error("izin yang berakhir hari ini harus masih berlaku hari ini")
	}
	if partner.Assignable(today.AddDate(0, 0, 1).Add(time.Hour)) {
		t.Error("izin harus tidak berlaku setelah tanggal berakhir")
	}
}

func TestSlotHasRoomDanRemaining(t *testing.T) {
	if !(models.ConsultationSlot{Quota: 3, Booked: 2}).HasRoom() {
		t.Error("slot 2/3 harus masih punya kuota")
	}
	if (models.ConsultationSlot{Quota: 3, Booked: 3}).HasRoom() {
		t.Error("slot 3/3 tidak boleh punya kuota")
	}
	if got := (models.ConsultationSlot{Quota: 3, Booked: 5}).Remaining(); got != 0 {
		t.Errorf("Remaining saat kelebihan = %d, harap 0", got)
	}
	if got := (models.ConsultationSlot{Quota: 3, Booked: 1}).Remaining(); got != 2 {
		t.Errorf("Remaining = %d, harap 2", got)
	}
}

func TestNoteIsMaskedMengikutiRetensi(t *testing.T) {
	now := time.Now().UTC()
	future := now.AddDate(0, 0, 30)
	past := now.AddDate(0, 0, -1)

	if noteIsMasked(nil, now) {
		t.Error("tanpa masa retensi tidak boleh disamarkan")
	}
	if noteIsMasked(&future, now) {
		t.Error("retensi di masa depan tidak boleh disamarkan")
	}
	if !noteIsMasked(&past, now) {
		t.Error("retensi yang sudah lewat harus disamarkan")
	}
}

func TestConsultationModeDanQuotaDinormalkan(t *testing.T) {
	if got := consultationMode(""); got != "online" {
		t.Errorf("mode kosong = %q, harap online", got)
	}
	if got := consultationMode("offline"); got != "offline" {
		t.Errorf("mode offline = %q", got)
	}
	if got := consultationMode("daring"); got != "online" {
		t.Errorf("mode tak dikenal = %q, harap online", got)
	}
	if got := consultationQuota(0); got != 1 {
		t.Errorf("kuota 0 = %d, harap 1", got)
	}
	if got := consultationQuota(50); got != 20 {
		t.Errorf("kuota 50 = %d, harap dibatasi 20", got)
	}
	if got := consultationQuota(4); got != 4 {
		t.Errorf("kuota 4 = %d", got)
	}
}

func TestParseOptionalDate(t *testing.T) {
	if value, err := parseOptionalDate(""); err != nil || value != nil {
		t.Errorf("tanggal kosong harus nil tanpa galat, dapat %v / %v", value, err)
	}
	if _, err := parseOptionalDate("31-12-2026"); err == nil {
		t.Error("format salah harus menghasilkan galat")
	}
	value, err := parseOptionalDate("2027-10-29")
	if err != nil {
		t.Fatalf("tanggal valid error: %v", err)
	}
	if value == nil || value.Format(time.DateOnly) != "2027-10-29" {
		t.Errorf("tanggal = %v", value)
	}
}

func TestNoteVisibilityHelpers(t *testing.T) {
	private := models.ConsultationNote{Visibility: models.NotePrivate}
	if private.SharedToUser() || private.SharedToHR() {
		t.Error("catatan private tidak boleh dibagikan ke siapa pun")
	}
	toUser := models.ConsultationNote{Visibility: models.NoteSharedToUser}
	if !toUser.SharedToUser() || toUser.SharedToHR() {
		t.Error("shared_to_user: peserta boleh, HR tidak")
	}
	toHR := models.ConsultationNote{Visibility: models.NoteSharedToHR}
	if !toHR.SharedToUser() || !toHR.SharedToHR() {
		t.Error("shared_to_hr: peserta & HR boleh")
	}
}

func TestBookingCancellableHanyaSaatTerpesan(t *testing.T) {
	now := time.Now().UTC()
	startsAt := now.Add(48 * time.Hour)
	if !(models.ConsultationBooking{Status: models.BookingTerpesan}).Cancellable(now, startsAt) {
		t.Error("pemesanan terpesan 48 jam lagi harus bisa dibatalkan")
	}
	if (models.ConsultationBooking{Status: models.BookingSelesai}).Cancellable(now, startsAt) {
		t.Error("pemesanan selesai tidak bisa dibatalkan")
	}
}

func TestBookingStatusDanVisibilityValid(t *testing.T) {
	for _, status := range []models.BookingStatus{
		models.BookingTerpesan, models.BookingSelesai, models.BookingBatal, models.BookingTidakHadir,
	} {
		if !status.Valid() {
			t.Errorf("status %q harus valid", status)
		}
	}
	if models.BookingStatus("entah").Valid() {
		t.Error("status tak dikenal harus tidak valid")
	}
	for _, visibility := range []models.NoteVisibility{
		models.NotePrivate, models.NoteSharedToUser, models.NoteSharedToHR,
	} {
		if !visibility.Valid() {
			t.Errorf("visibilitas %q harus valid", visibility)
		}
	}
	if models.NoteVisibility("entah").Valid() {
		t.Error("visibilitas tak dikenal harus tidak valid")
	}
}

func TestRoleBaruValid(t *testing.T) {
	for _, role := range []models.Role{
		models.RolePeserta, models.RoleAdmin, models.RolePsikolog, models.RoleAdminInstansi,
	} {
		if !role.Valid() {
			t.Errorf("peran %q harus valid", role)
		}
	}
	if models.Role("mentor").Valid() {
		t.Error("peran di luar daftar harus tidak valid")
	}
}
