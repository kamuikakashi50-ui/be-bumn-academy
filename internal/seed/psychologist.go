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
		Key:              "gita",
		FullName:         "Gita Maharani, M.Psi., Psikolog",
		Email:            "gita.maharani@mitrapsikologi.id",
		Phone:            "081200000103",
		SippNumber:       "SIPP-2024-00231",
		StrNumber:        "STR-2024-771455",
		Specs:            models.StringList{"Psikologi klinis", "Manajemen stres", "Resiliensi"},
		Status:           models.PartnerTerverifikasi,
		ValidDaysFromNow: 365,
		// Tanpa akun login: mendemokan mitra yang tetap boleh ditugaskan & dibuka
		// slotnya meski belum punya akun sendiri.
		Notes: "Mitra terverifikasi tanpa akun; dipakai mendemokan penjadwalan lintas mitra.",
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
	{
		Key:              "hadi",
		FullName:         "Hadi Nugroho, M.Psi.",
		Email:            "hadi.nugroho@mitrapsikologi.id",
		Phone:            "081200000104",
		SippNumber:       "SIPP-2022-00088",
		StrNumber:        "STR-2022-660311",
		Specs:            models.StringList{"Asesmen kepribadian", "Coaching"},
		Status:           models.PartnerNonaktif,
		ValidDaysFromNow: -30,
		Notes:            "Izin praktik sudah lewat masa berlaku; tidak boleh ditugaskan sampai diperbarui.",
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
		// Nilai negatif dipakai untuk mendemokan izin yang sudah lewat masa berlaku.
		if def.ValidDaysFromNow != 0 {
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
//
// `StartInHr` boleh negatif: slot lampau dipakai sesi yang sudah `selesai` atau
// `tidak_hadir`, sehingga agenda psikolog & rekap admin tidak hanya berisi jadwal
// mendatang.
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
	// Sesi lampau (riwayat).
	{Key: "p14-0900", Partner: "anindya", StartInHr: -336, Duration: 60 * time.Minute, Mode: "online", Quota: 1, Channel: "https://meet.bumnacademy.id/psikolog/anindya-0900"},
	{Key: "p7-1330", Partner: "anindya", StartInHr: -168, Duration: 45 * time.Minute, Mode: "online", Quota: 2, Channel: "https://meet.bumnacademy.id/psikolog/anindya-1330"},
	{Key: "p3-1000", Partner: "gita", StartInHr: -72, Duration: 60 * time.Minute, Mode: "online", Quota: 1, Channel: "https://meet.bumnacademy.id/psikolog/gita-1000"},

	// Jadwal mendatang yang masih bisa dipesan.
	{Key: "d2-0900", Partner: "anindya", StartInHr: 48, Duration: 60 * time.Minute, Mode: "online", Quota: 3, Channel: "https://meet.bumnacademy.id/psikolog/anindya-0900"},
	{Key: "d5-1300", Partner: "anindya", StartInHr: 120, Duration: 60 * time.Minute, Mode: "online", Quota: 2, Channel: "https://meet.bumnacademy.id/psikolog/anindya-1300"},
	{Key: "d9-1000", Partner: "anindya", StartInHr: 216, Duration: 45 * time.Minute, Mode: "offline", Quota: 1, Channel: ""},
	{Key: "d3-0900", Partner: "gita", StartInHr: 72, Duration: 60 * time.Minute, Mode: "online", Quota: 2, Channel: "https://meet.bumnacademy.id/psikolog/gita-0900"},
	{Key: "d6-1500", Partner: "gita", StartInHr: 144, Duration: 45 * time.Minute, Mode: "online", Quota: 1, Channel: "https://meet.bumnacademy.id/psikolog/gita-1500"},
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

/* --------------------------- sesi konsultasi demo -------------------------- */

// consultationDemoDef adalah satu sesi konsultasi demo.
//
// Sesi sengaja mencakup **seluruh status** (`terpesan`, `selesai`, `batal`,
// `tidak_hadir`) agar rekap pemanfaatan layanan admin dan riwayat peserta punya
// isi sejak database baru.
type consultationDemoDef struct {
	Key         string
	Slot        string
	Participant string
	Status      models.BookingStatus
	Agenda      string
	// CreatedDaysAgo menentukan waktu pemesanan (riwayat yang masuk akal).
	CreatedDaysAgo int
}

var consultationDemoDefs = []consultationDemoDef{
	{
		Key: "siti-d2", Slot: "d2-0900", Participant: "siti.nurhaliza@bumnacademy.id",
		Status: models.BookingTerpesan, CreatedDaysAgo: 3,
		Agenda: "Membahas hasil asesmen minat–bakat dan rencana belajar lanjutan.",
	},
	{
		Key: "dewi-p14", Slot: "p14-0900", Participant: "dewi.lestari@bumnacademy.id",
		Status: models.BookingSelesai, CreatedDaysAgo: 16,
		Agenda: "Menyusun strategi belajar untuk tryout lanjutan.",
	},
	{
		Key: "rangga-p7", Slot: "p7-1330", Participant: "rangga.pratama@bumnacademy.id",
		Status: models.BookingTidakHadir, CreatedDaysAgo: 10,
		Agenda: "Membahas kesiapan menghadapi tahap seleksi berikutnya.",
	},
	{
		Key: "fajar-p3", Slot: "p3-1000", Participant: "fajar.ramadhan@bumnacademy.id",
		Status: models.BookingSelesai, CreatedDaysAgo: 6,
		Agenda: "Pendampingan intensif penalaran numerik.",
	},
	{
		Key: "intan-d3", Slot: "d3-0900", Participant: "intan.permata@bumnacademy.id",
		Status: models.BookingTerpesan, CreatedDaysAgo: 2,
		Agenda: "Membahas hasil asesmen dan kesiapan kerja.",
	},
	{
		Key: "bayu-d6", Slot: "d6-1500", Participant: "bayu.saputra@bumnacademy.id",
		Status: models.BookingTerpesan, CreatedDaysAgo: 1,
		Agenda: "Diskusi penempatan lintas klaster BUMN.",
	},
	{
		Key: "yoga-d9", Slot: "d9-1000", Participant: "yoga.pratama@bumnacademy.id",
		Status: models.BookingBatal, CreatedDaysAgo: 4,
		Agenda: "Dibatalkan peserta karena jadwal bentrok.",
	},
}

// consultationNoteDef adalah satu catatan sesi demo.
type consultationNoteDef struct {
	Key            string
	Booking        string
	Author         string
	Summary        string
	Recommendation string
	Visibility     models.NoteVisibility
	// RetentionDaysFromNow negatif berarti masa retensi sudah lewat → isi
	// catatan disamarkan otomatis (contoh perilaku §4.5).
	RetentionDaysFromNow int
}

var consultationNoteDefs = []consultationNoteDef{
	{
		Key: "siti", Booking: "siti-d2", Author: "psikolog@bumnacademy.id",
		Summary:        "Peserta kuat pada penalaran analitis; ketelitian perlu latihan bertahap.",
		Recommendation: "Fokus modul penalaran numerik lanjutan dan simulasi berwaktu.",
		Visibility:     models.NoteSharedToUser,
	},
	{
		Key: "dewi-shared", Booking: "dewi-p14", Author: "psikolog@bumnacademy.id",
		Summary:        "Konsistensi belajar baik dan hasil tryout naik stabil dalam tiga bulan terakhir.",
		Recommendation: "Pertahankan ritme latihan; tambah simulasi berwaktu penuh.",
		Visibility:     models.NoteSharedToUser,
	},
	{
		Key: "dewi-private", Booking: "dewi-p14", Author: "psikolog@bumnacademy.id",
		Summary:    "Catatan kerja psikolog: peserta sempat cemas sebelum sesi, sudah tenang setelah dijelaskan alur seleksi.",
		Visibility: models.NotePrivate,
	},
	{
		Key: "rangga", Booking: "rangga-p7", Author: "psikolog@bumnacademy.id",
		Summary:    "Peserta tidak menghadiri sesi; perlu dihubungi ulang untuk menjadwalkan kembali.",
		Visibility: models.NotePrivate,
	},
	{
		Key: "fajar-hr", Booking: "fajar-p3", Author: "psikolog@bumnacademy.id",
		Summary:        "Pendampingan numerik menunjukkan kemajuan; skor latihan naik setelah dua pekan.",
		Recommendation: "Lanjutkan modul bertahap dan evaluasi tiap dua pekan.",
		Visibility:     models.NoteSharedToHR,
	},
	{
		Key: "bayu", Booking: "bayu-d6", Author: "psikolog@bumnacademy.id",
		Summary:    "Catatan internal: peserta mempertimbangkan dua klaster berbeda, perlu pemetaan minat ulang.",
		Visibility: models.NotePrivate,
	},
	{
		Key: "siti-retensi", Booking: "siti-d2", Author: "psikolog@bumnacademy.id",
		Summary:    "Catatan sesi awal sebelum peserta mengerjakan ulang instrumen.",
		Visibility: models.NoteSharedToUser,
		// Masa retensi sudah lewat → tampil sebagai "isi disamarkan".
		RetentionDaysFromNow: -5,
	},
}

/* ------------------------------ fungsi seed ------------------------------- */

// seedConsultationDemo menyimpan sesi konsultasi demo, catatan berlapis, satu
// permohonan koreksi, dan satu interpretasi instrumen (idempoten).
//
// Semua penulisan memakai `DO NOTHING` pada id yang deterministik, sehingga
// keputusan nyata pengguna (mis. peserta membatalkan sesi demo lewat UI) tidak
// dibatalkan saat seed dijalankan ulang.
func seedConsultationDemo(ctx context.Context, db *gorm.DB, userIDs map[string]string) error {
	now := time.Now().UTC()

	// Persetujuan konsultasi per peserta yang punya sesi.
	consentByParticipant := map[string]string{}
	for _, def := range consultationDemoDefs {
		participantID, ok := userIDs[def.Participant]
		if !ok {
			continue
		}
		if _, done := consentByParticipant[def.Participant]; done {
			continue
		}
		consent := models.ConsentRecord{
			IDBase:      models.IDBase{ID: stableID("consent/consultation/" + def.Participant)},
			UserID:      participantID,
			Purpose:     models.ConsentPurposeConsultation,
			TextVersion: "konsultasi-v1",
			GrantedAt:   now.AddDate(0, 0, -def.CreatedDaysAgo-2),
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&consent).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan persetujuan konsultasi %s: %w", def.Participant, err)
		}
		consentByParticipant[def.Participant] = consent.ID
	}

	bookings := make([]models.ConsultationBooking, 0, len(consultationDemoDefs))
	bookingIDByKey := map[string]string{}
	for _, def := range consultationDemoDefs {
		participantID, ok := userIDs[def.Participant]
		if !ok {
			continue
		}
		bookingID := stableID("consultation-booking/" + def.Key)
		bookingIDByKey[def.Key] = bookingID
		createdAt := now.AddDate(0, 0, -def.CreatedDaysAgo)
		booking := models.ConsultationBooking{
			ID:        bookingID,
			SlotID:    stableID("consultation-slot/" + def.Slot),
			UserID:    participantID,
			Status:    def.Status,
			Agenda:    def.Agenda,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		}
		if consentID, ok := consentByParticipant[def.Participant]; ok {
			booking.ConsentID = &consentID
		}
		bookings = append(bookings, booking)
	}

	if len(bookings) > 0 {
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&bookings).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan sesi konsultasi demo: %w", err)
		}
	}

	if err := seedConsultationNotes(ctx, db, userIDs, bookingIDByKey, now); err != nil {
		return err
	}
	if err := seedAssessmentDispute(ctx, db, userIDs, now); err != nil {
		return err
	}
	if err := seedInstrumentInterpretation(ctx, db, userIDs, now); err != nil {
		return err
	}
	return syncConsultationSlotBooked(ctx, db)
}

// seedConsultationNotes menyimpan catatan sesi demo.
func seedConsultationNotes(
	ctx context.Context,
	db *gorm.DB,
	userIDs map[string]string,
	bookingIDByKey map[string]string,
	now time.Time,
) error {
	notes := make([]models.ConsultationNote, 0, len(consultationNoteDefs))
	for _, def := range consultationNoteDefs {
		bookingID, ok := bookingIDByKey[def.Booking]
		if !ok {
			continue
		}
		authorID, ok := userIDs[def.Author]
		if !ok {
			// Mitra tanpa akun tidak bisa menjadi penulis catatan.
			continue
		}
		note := models.ConsultationNote{
			ID:             stableID("consultation-note/" + def.Key),
			BookingID:      bookingID,
			AuthorID:       authorID,
			Summary:        def.Summary,
			Recommendation: def.Recommendation,
			Visibility:     def.Visibility,
			CreatedAt:      now.AddDate(0, 0, -3),
		}
		if def.RetentionDaysFromNow != 0 {
			retention := now.AddDate(0, 0, def.RetentionDaysFromNow)
			note.RetentionUntil = &retention
		}
		notes = append(notes, note)
	}
	if len(notes) == 0 {
		return nil
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&notes).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan catatan konsultasi demo: %w", err)
	}
	return nil
}

// seedAssessmentDispute menyimpan satu permohonan koreksi hasil asesmen.
func seedAssessmentDispute(ctx context.Context, db *gorm.DB, userIDs map[string]string, now time.Time) error {
	participantID, ok := userIDs["rangga.pratama@bumnacademy.id"]
	if !ok {
		return nil
	}
	dispute := models.AssessmentDispute{
		ID:           stableID("assessment-dispute/rangga"),
		AssessmentID: stableID("talent-assessment/rangga.pratama@bumnacademy.id/aptitude-v1"),
		UserID:       participantID,
		Reason:       "Hasil dimensi penalaran numerik saya rasa belum mencerminkan kemampuan saat mengerjakan instrumen.",
		Status:       models.DisputeDitinjau,
		CreatedAt:    now.AddDate(0, 0, -2),
	}
	if adminID, ok := userIDs["admin@bumnacademy.id"]; ok {
		resolvedAt := now.AddDate(0, 0, -1)
		dispute.ResolvedBy = &adminID
		dispute.ResolvedAt = &resolvedAt
		dispute.Resolution = "Sedang ditinjau bersama mitra psikolog; peserta dijadwalkan sesi ulang."
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&dispute).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan permohonan koreksi demo: %w", err)
	}
	return nil
}

// seedInstrumentInterpretation menyimpan satu catatan interpretasi instrumen.
//
// Interpretasi hanya boleh ditulis psikolog yang ditugaskan pada peserta (§2.6).
func seedInstrumentInterpretation(ctx context.Context, db *gorm.DB, userIDs map[string]string, now time.Time) error {
	subjectID, ok := userIDs["siti.nurhaliza@bumnacademy.id"]
	if !ok {
		return nil
	}
	interpretation := models.InstrumentInterpretation{
		ID:             stableID("instrument-interpretation/siti-aptitude"),
		InstrumentCode: "aptitude-v1",
		PsychologistID: stableID("psychologist/anindya"),
		SubjectID:      subjectID,
		AssessmentID:   stringPtr(stableID("talent-assessment/siti.nurhaliza@bumnacademy.id/aptitude-v1")),
		Interpretation: "Profil bakat menunjukkan kekuatan pada penalaran verbal; numerik masih dapat ditingkatkan dengan latihan bertahap.",
		CreatedAt:      now.AddDate(0, 0, -2),
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&interpretation).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan interpretasi instrumen demo: %w", err)
	}
	return nil
}

// syncConsultationSlotBooked menyelaraskan `booked` setiap slot dengan jumlah
// pemesanan nyata yang belum dibatalkan.
//
// Dihitung ulang (bukan angka tetap) supaya pembatalan oleh pengguna tidak
// "dihidupkan kembali" oleh seed berikutnya.
func syncConsultationSlotBooked(ctx context.Context, db *gorm.DB) error {
	err := db.WithContext(ctx).Exec(`
		UPDATE consultation_slots s
		SET booked = (
			SELECT COUNT(*) FROM consultation_bookings b
			WHERE b.slot_id = s.id AND b.status <> ?
		)`, models.BookingBatal).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyelaraskan kuota slot konsultasi: %w", err)
	}
	return nil
}
