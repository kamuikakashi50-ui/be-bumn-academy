package seed

import (
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji data mitra psikolog & konsultasi demo (Fase 3 langkah 6–7).
//
// Data ini menggerakkan gerbang izin praktik (§4.7) dan seluruh status
// pemesanan (§4.5), jadi konsistensinya diuji sebelum menyentuh database.

// daysFromNow mengubah selisih hari menjadi waktu (nil untuk 0).
func daysFromNow(now time.Time, days int) *time.Time {
	if days == 0 {
		return nil
	}
	value := now.AddDate(0, 0, days)
	return &value
}

func TestMitraPsikologDefKonsisten(t *testing.T) {
	seen := map[string]bool{}
	statuses := map[models.PartnerStatus]int{}
	accounts := seededUserEmails(t)
	now := time.Now().UTC()

	for _, def := range psychologistDefs {
		if def.Key == "" || def.FullName == "" || def.SippNumber == "" {
			t.Errorf("mitra %q belum lengkap (kunci/nama/SIPP wajib)", def.Key)
		}
		if seen[def.Key] {
			t.Errorf("kunci mitra ganda: %q", def.Key)
		}
		seen[def.Key] = true

		if !def.Status.Valid() {
			t.Errorf("mitra %s memakai status tidak dikenal: %q", def.Key, def.Status)
		}
		statuses[def.Status]++

		if len(def.Specs) == 0 {
			t.Errorf("mitra %s tidak punya spesialisasi", def.Key)
		}
		if def.UserEmail != "" && !accounts[def.UserEmail] {
			t.Errorf("mitra %s menautkan akun yang tidak dibuat seed: %s", def.Key, def.UserEmail)
		}

		// Gerbang §4.7: hanya mitra terverifikasi dengan izin berlaku.
		partner := models.PsychologistPartner{
			Status:     def.Status,
			ValidUntil: daysFromNow(now, def.ValidDaysFromNow),
		}
		if got, want := partner.Assignable(now), def.Status == models.PartnerTerverifikasi; got != want {
			t.Errorf("Assignable mitra %s = %v, harap %v", def.Key, got, want)
		}
	}

	// Ketiga status harus terwakili agar UI memperlihatkan ketiga bentuknya.
	for _, status := range []models.PartnerStatus{
		models.PartnerTerverifikasi, models.PartnerPending, models.PartnerNonaktif,
	} {
		if statuses[status] == 0 {
			t.Errorf("tidak ada mitra demo berstatus %q", status)
		}
	}

	// Mitra terverifikasi wajib punya masa berlaku di masa depan; mitra nonaktif
	// mendemokan izin yang sudah lewat.
	for _, def := range psychologistDefs {
		switch def.Status {
		case models.PartnerTerverifikasi:
			if def.ValidDaysFromNow <= 0 {
				t.Errorf("mitra terverifikasi %s tidak punya masa berlaku di masa depan", def.Key)
			}
		case models.PartnerNonaktif:
			if def.ValidDaysFromNow >= 0 {
				t.Errorf("mitra nonaktif %s sebaiknya mendemokan izin yang sudah lewat", def.Key)
			}
		}
	}
}

func TestSlotKonsultasiDefKonsisten(t *testing.T) {
	partners := map[string]bool{}
	for _, def := range psychologistDefs {
		partners[def.Key] = true
	}

	seen := map[string]bool{}
	past, future := 0, 0
	for _, def := range psychologistSlotDefs {
		if def.Key == "" {
			t.Fatal("slot konsultasi tanpa kunci")
		}
		if seen[def.Key] {
			t.Errorf("kunci slot ganda: %q", def.Key)
		}
		seen[def.Key] = true

		if !partners[def.Partner] {
			t.Errorf("slot %s menunjuk mitra tak dikenal: %s", def.Key, def.Partner)
		}
		if def.Quota < 1 {
			t.Errorf("slot %s punya kuota tidak valid: %d", def.Key, def.Quota)
		}
		if def.Duration <= 0 {
			t.Errorf("slot %s punya durasi tidak valid: %v", def.Key, def.Duration)
		}
		if def.Mode != "online" && def.Mode != "offline" {
			t.Errorf("slot %s memakai mode tak dikenal: %q", def.Key, def.Mode)
		}
		if def.StartInHr < 0 {
			past++
		} else {
			future++
		}
	}

	// Riwayat (agenda & rekap) butuh sesi lampau; peserta butuh slot mendatang.
	if past == 0 {
		t.Error("tidak ada slot lampau; agenda & rekap konsultasi hanya akan berisi jadwal kosong")
	}
	if future == 0 {
		t.Error("tidak ada slot mendatang; peserta tidak bisa memesan apa pun")
	}
}

func TestSesiKonsultasiDemoKonsisten(t *testing.T) {
	slots := map[string]bool{}
	for _, def := range psychologistSlotDefs {
		slots[def.Key] = true
	}
	accounts := seededUserEmails(t)

	seen := map[string]bool{}
	statuses := map[models.BookingStatus]int{}
	// Unique index pemesanan bersifat parsial: satu peserta hanya boleh punya
	// satu pemesanan **aktif** per slot.
	activePairs := map[string]bool{}

	for _, def := range consultationDemoDefs {
		if def.Key == "" {
			t.Fatal("sesi konsultasi tanpa kunci")
		}
		if seen[def.Key] {
			t.Errorf("kunci sesi ganda: %q", def.Key)
		}
		seen[def.Key] = true

		if !slots[def.Slot] {
			t.Errorf("sesi %s menunjuk slot tak dikenal: %s", def.Key, def.Slot)
		}
		if !accounts[def.Participant] {
			t.Errorf("sesi %s memakai peserta yang tidak dibuat seed: %s", def.Key, def.Participant)
		}
		if !def.Status.Valid() {
			t.Errorf("sesi %s memakai status tak dikenal: %q", def.Key, def.Status)
		}
		statuses[def.Status]++

		if def.CreatedDaysAgo < 0 {
			t.Errorf("sesi %s punya CreatedDaysAgo negatif", def.Key)
		}
		if def.Status != models.BookingBatal {
			pair := def.Slot + "/" + def.Participant
			if activePairs[pair] {
				t.Errorf("sesi %s membuat pemesanan aktif ganda pada slot+peserta yang sama (%s)", def.Key, pair)
			}
			activePairs[pair] = true
		}
	}

	// Seluruh status harus terwakili agar rekap pemanfaatan layanan berisi.
	for _, status := range []models.BookingStatus{
		models.BookingTerpesan, models.BookingSelesai, models.BookingBatal, models.BookingTidakHadir,
	} {
		if statuses[status] == 0 {
			t.Errorf("tidak ada sesi demo berstatus %q", status)
		}
	}
}

func TestCatatanKonsultasiDefKonsisten(t *testing.T) {
	bookings := map[string]bool{}
	for _, def := range consultationDemoDefs {
		bookings[def.Key] = true
	}
	accounts := seededUserEmails(t)

	seen := map[string]bool{}
	visibilities := map[models.NoteVisibility]int{}
	masked := 0

	for _, def := range consultationNoteDefs {
		if seen[def.Key] {
			t.Errorf("kunci catatan ganda: %q", def.Key)
		}
		seen[def.Key] = true

		if !bookings[def.Booking] {
			t.Errorf("catatan %s menunjuk sesi tak dikenal: %s", def.Key, def.Booking)
		}
		if !accounts[def.Author] {
			t.Errorf("catatan %s ditulis akun yang tidak dibuat seed: %s", def.Key, def.Author)
		}
		if !def.Visibility.Valid() {
			t.Errorf("catatan %s memakai visibilitas tak dikenal: %q", def.Key, def.Visibility)
		}
		visibilities[def.Visibility]++

		if def.RetentionDaysFromNow < 0 {
			masked++
		}
	}

	// Ketiga tingkat visibilitas harus ada: peserta & HR hanya melihat sebagian.
	for _, visibility := range []models.NoteVisibility{
		models.NotePrivate, models.NoteSharedToUser, models.NoteSharedToHR,
	} {
		if visibilities[visibility] == 0 {
			t.Errorf("tidak ada catatan demo dengan visibilitas %q", visibility)
		}
	}
	if masked == 0 {
		t.Error("tidak ada catatan demo dengan masa retensi lewat; perilaku penyamaran tidak terlihat")
	}
}
