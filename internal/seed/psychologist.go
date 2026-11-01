package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Seed mitra psikolog, slot konsultasi, dan satu sesi demo (Fase 3 langkah 6–7).
//
// Dua mitra disiapkan agar kedua sisi aturan §4.7 terlihat di UI:
//   - **Anindya Prameswari** — terverifikasi, tertaut ke akun
//     `psikolog@bumnacademy.id`, izin berlaku sampai 2 tahun ke depan;
//   - **Rendra Wibowo** — masih `pending` (tanpa akun), sehingga tidak muncul di
//     katalog peserta dan tidak bisa ditugaskan sampai admin memverifikasi.
//
// Instrumen & alat tes berlisensi tetap menjadi prasyarat **operasional**
// (perjanjian kerja sama); seed ini hanya menyiapkan kerangka layanannya.

// psychologistDef adalah definisi mitra psikolog demo.
type psychologistDef struct {
	Key         string
	FullName    string
	Email       string
	Phone       string
	SippNumber  string
	SippkNumber string
	StrNumber   string
	Specs       models.StringList
	Status      models.PartnerStatus
	// ValidDaysFromNow 0 berarti tanpa masa berlaku.
	ValidDaysFromNow int
	UserEmail        string
	Notes            string
}

var psychologistDefs = []psychologistDef{
	{
		Key:              "anindya",
		FullName:         "Anindya Prameswari, M.Psi.",
		Email:            "anindya.prameswari@mitrapsikologi.id",
		Phone:            "081200000101",
		SippNumber:       "SIPP-2024-00117",
		SippkNumber:      "SIPPK-2024-00042",
		StrNumber:        "STR-2024-778102",
		Specs:            models.StringList{"Asesmen minat–bakat", "Karier & penempatan", "Kesiapan kerja"},
		Status:           models.PartnerTerverifikasi,
		ValidDaysFromNow: 730,
		UserEmail:        "psikolog@bumnacademy.id",
		Notes:            "Mitra demo untuk alur konsultasi daring; izin praktik terverifikasi.",
	},
	{
		Key:        "rendra",
		FullName:   "Rendra Wibowo, M.Psi.",
		Email:      "rendra.wibowo@mitrapsikologi.id",
		Phone:      "081200000102",
		SippNumber: "SIPP-2025-00301",
		Specs:      models.StringList{"Psikologi klinis", "Manajemen stres"},
		Status:     models.PartnerPending,
		UserEmail:  "",
		Notes:      "Menunggu verifikasi dokumen izin praktik.",
	},
}

// seedPsychologists menyimpan mitra psikolog demo (idempoten).
//
// Kolom `verification_status` & `valid_until` sengaja **tidak** ditimpa ulang
// setelah dibuat, supaya keputusan verifikasi admin tidak dibatalkan seed —
// pola yang sama dipakai pada kolom `active` instrumen & soal.
func seedPsychologists(ctx context.Context, db *gorm.DB, userIDs map[string]string) error {
	now := time.Now().UTC()
	for _, def := range psychologistDefs {
		partner := models.PsychologistPartner{
			Base:            models.Base{ID: stableID("psychologist/" + def.Key)},
			FullName:        def.FullName,
			Email:           def.Email,
			Phone:           def.Phone,
			SippNumber:      def.SippNumber,
			SippkNumber:     stringPtr(def.SippkNumber),
			StrNumber:       stringPtr(def.StrNumber),
			Specializations: def.Specs,
			Status:          def.Status,
			Notes:           def.Notes,
		}
		if def.UserEmail != "" {
			if id, ok := userIDs[def.UserEmail]; ok {
				partner.UserID = &id
			}
		}
		if def.ValidDaysFromNow > 0 {
			valid := now.AddDate(0, 0, def.ValidDaysFromNow)
			partner.ValidUntil = &valid
		}

		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			// `verification_status` & `valid_until` tidak ikut diperbarui.
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id", "full_name", "email", "phone", "sipp_number",
				"sippk_number", "str_number", "specializations", "notes",
			}),
		}).Create(&partner).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan mitra psikolog %s: %w", def.FullName, err)
		}
	}
	return nil
}

// slotDef adalah satu slot konsultasi demo.
type slotDef struct {
	Key       string
	Partner   string
	StartInHr int
	Duration  time.Duration
	Mode      string
	Quota     int
	Channel   string
}

var psychologistSlotDefs = []slotDef{
	{Key: "d2-0900", Partner: "anindya", StartInHr: 48, Duration: 60 * time.Minute, Mode: "online", Quota: 3, Channel: "https://meet.bumnacademy.id/psikolog/anindya-0900"},
	{Key: "d5-1300", Partner: "anindya", StartInHr: 120, Duration: 60 * time.Minute, Mode: "online", Quota: 2, Channel: "https://meet.bumnacademy.id/psikolog/anindya-1300"},
	{Key: "d9-1000", Partner: "anindya", StartInHr: 216, Duration: 45 * time.Minute, Mode: "offline", Quota: 1, Channel: ""},
}

// seedPsychologistSlots menyimpan slot konsultasi demo (idempoten).
//
// Waktu slot dihitung dari "sekarang" dan ikut diperbarui saat seed dijalankan
// ulang, sehingga data demo tidak pernah basi. `booked` tidak ditimpa agar
// pemesanan nyata tidak hilang.
func seedPsychologistSlots(ctx context.Context, db *gorm.DB) error {
	now := time.Now().UTC()
	slots := make([]models.ConsultationSlot, 0, len(psychologistSlotDefs))
	for _, def := range psychologistSlotDefs {
		startsAt := now.Add(time.Duration(def.StartInHr) * time.Hour).Truncate(time.Hour)
		slot := models.ConsultationSlot{
			ID:             stableID("consultation-slot/" + def.Key),
			PsychologistID: stableID("psychologist/" + def.Partner),
			StartsAt:       startsAt,
			EndsAt:         startsAt.Add(def.Duration),
			Mode:           def.Mode,
			Quota:          def.Quota,
			Active:         true,
		}
		if def.Channel != "" {
			slot.ChannelURL = stringPtr(def.Channel)
		}
		slots = append(slots, slot)
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"psychologist_id", "starts_at", "ends_at", "mode", "quota", "channel_url", "active",
		}),
	}).Create(&slots).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan slot konsultasi: %w", err)
	}
	return nil
}

// seedPsychologistDemoSession membuat satu sesi konsultasi demo beserta catatan
// yang dibagikan ke peserta (idempoten).
//
// Sesi ini membuat alur §4.5–§4.6 dapat dicoba tanpa langkah manual: peserta
// punya riwayat konsultasi, psikolog punya agenda & peserta yang ditugaskan,
// dan catatan `shared_to_user` terlihat di kedua sisi.
func seedPsychologistDemoSession(ctx context.Context, db *gorm.DB, userIDs map[string]string) error {
	participantID, ok := userIDs["siti.nurhaliza@bumnacademy.id"]
	if !ok {
		// Tanpa akun demo kohort, sesi demo dilewati (seed tetap berhasil).
		return nil
	}
	partnerID := stableID("psychologist/anindya")
	slotID := stableID("consultation-slot/d2-0900")

	consent := models.ConsentRecord{
		IDBase:      models.IDBase{ID: stableID("consent/consultation/siti")},
		UserID:      participantID,
		Purpose:     models.ConsentPurposeConsultation,
		TextVersion: "konsultasi-v1",
		GrantedAt:   time.Now().UTC().AddDate(0, 0, -7),
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&consent).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan persetujuan konsultasi demo: %w", err)
	}

	booking := models.ConsultationBooking{
		ID:        stableID("consultation-booking/siti"),
		SlotID:    slotID,
		UserID:    participantID,
		Status:    models.BookingTerpesan,
		Agenda:    "Membahas hasil asesmen minat–bakat dan rencana belajar lanjutan.",
		ConsentID: &consent.ID,
		CreatedAt: time.Now().UTC().AddDate(0, 0, -3),
		UpdatedAt: time.Now().UTC().AddDate(0, 0, -3),
	}
	// Konflik pada id (pemesanan demo sudah ada) diabaikan, bukan gagal.
	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&booking).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan pemesanan konsultasi demo: %w", err)
	}

	authorID, ok := userIDs["psikolog@bumnacademy.id"]
	if ok {
		note := models.ConsultationNote{
			ID:             stableID("consultation-note/siti"),
			BookingID:      booking.ID,
			AuthorID:       authorID,
			Summary:        "Peserta kuat pada penalaran analitis; ketelitian perlu latihan bertahap.",
			Recommendation: "Fokus modul penalaran numerik lanjutan dan simulasi berwaktu.",
			Visibility:     models.NoteSharedToUser,
			CreatedAt:      time.Now().UTC().AddDate(0, 0, -2),
		}
		err = db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&note).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan catatan konsultasi demo: %w", err)
		}
	}

	// `booked` dihitung ulang dari pemesanan nyata agar selalu konsisten —
	// termasuk bila peserta membatalkan pemesanan demo lewat UI.
	err = db.WithContext(ctx).Exec(`
		UPDATE consultation_slots s
		SET booked = (
			SELECT COUNT(*) FROM consultation_bookings b
			WHERE b.slot_id = s.id AND b.status <> ?
		)
		WHERE s.psychologist_id = ?`, models.BookingBatal, partnerID).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyelaraskan kuota slot konsultasi: %w", err)
	}
	return nil
}
