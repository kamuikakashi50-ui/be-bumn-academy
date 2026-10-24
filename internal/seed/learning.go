package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// liveSessionDef adalah definisi satu jadwal kelas live.
//
// Jadwal disimpan sebagai selisih jam dari waktu seed dijalankan (bukan tanggal
// tetap) supaya daftar kelas live selalu masuk akal pada instalasi demo.
type liveSessionDef struct {
	ID           string
	Title        string
	Topic        string
	Mentor       string
	MentorRole   string
	OffsetHours  int
	DurationMins int
	Status       models.LiveStatus
	Platform     string
	Note         string
}

var liveSessionDefs = []liveSessionDef{
	{
		ID:           "live-numerik-intensif",
		Title:        "Bedah Soal Penalaran Numerik",
		Topic:        "Penalaran Numerik",
		Mentor:       "Rani Prameswari",
		MentorRole:   "Pengajar Numerik",
		OffsetHours:  48,
		DurationMins: 90,
		Status:       models.LiveUpcoming,
		Platform:     "Ruang live BUMN Academy",
		Note:         "Siapkan soal deret dan perbandingan untuk dibahas bersama.",
	},
	{
		ID:           "live-analitis-silogisme",
		Title:        "Strategi Cepat Logika Analitis",
		Topic:        "Logika Analitis",
		Mentor:       "Bagas Nurhadi",
		MentorRole:   "Pengajar Logika",
		OffsetHours:  120,
		DurationMins: 75,
		Status:       models.LiveUpcoming,
		Platform:     "Ruang live BUMN Academy",
		Note:         "Latihan silogisme dengan pengujian tiap pilihan jawaban.",
	},
	{
		ID:           "live-akhlak-studi-kasus",
		Title:        "AKHLAK dalam Studi Kasus Seleksi",
		Topic:        "Nilai AKHLAK & Wawasan",
		Mentor:       "Sari Mahendra",
		MentorRole:   "Fasilitator Nilai & Etika",
		OffsetHours:  216,
		DurationMins: 60,
		Status:       models.LiveUpcoming,
		Platform:     "Ruang live BUMN Academy",
		Note:         "Membahas studi kasus enam nilai inti AKHLAK.",
	},
	{
		ID:           "live-inggris-reading",
		Title:        "Reading Comprehension Intensif",
		Topic:        "Bahasa Inggris",
		Mentor:       "Dimas Ardhana",
		MentorRole:   "Pengajar Bahasa Inggris",
		OffsetHours:  -72,
		DurationMins: 75,
		Status:       models.LiveEnded,
		Platform:     "Ruang live BUMN Academy",
		Note:         "Rekaman sesi tersedia pada daftar rekaman kelas.",
	},
}

// recordingDef adalah definisi satu rekaman kelas.
type recordingDef struct {
	ID           string
	Title        string
	Description  string
	Topic        string
	Category     string
	Mentor       string
	DurationMins int
	DaysAgo      int
	Accent       string
	VideoURL     string
}

var recordingDefs = []recordingDef{
	{
		ID:           "rec-numerik-deret",
		Title:        "Deret Angka & Pola: Pembahasan Soal",
		Description:  "Membedah pola deret aritmetika, geometri, dan campuran dengan langkah pengerjaan ringkas.",
		Topic:        "Penalaran Numerik",
		Category:     "TKD",
		Mentor:       "Rani Prameswari",
		DurationMins: 48,
		DaysAgo:      21,
		Accent:       "from-brand/80 to-navy",
		VideoURL:     "/media/lessons/numerik-deret.mp4",
	},
	{
		ID:           "rec-numerik-perbandingan",
		Title:        "Perbandingan & Persentase Tanpa Kalkulator",
		Description:  "Trik menghitung perbandingan senilai dan berbalik nilai secara cepat dan akurat.",
		Topic:        "Penalaran Numerik",
		Category:     "TKD",
		Mentor:       "Rani Prameswari",
		DurationMins: 42,
		DaysAgo:      18,
		Accent:       "from-emerald-500/80 to-navy",
		VideoURL:     "/media/lessons/numerik-perbandingan.mp4",
	},
	{
		ID:           "rec-verbal-analogi",
		Title:        "Analogi: Menentukan Hubungan Kata",
		Description:  "Cara mengenali jenis hubungan kata dan menghindari jebakan pilihan jawaban.",
		Topic:        "Penalaran Verbal",
		Category:     "TKD",
		Mentor:       "Kartika Larasati",
		DurationMins: 38,
		DaysAgo:      15,
		Accent:       "from-amber-500/80 to-navy",
		VideoURL:     "/media/lessons/verbal-analogi.mp4",
	},
	{
		ID:           "rec-analitis-silogisme",
		Title:        "Silogisme & Penarikan Kesimpulan",
		Description:  "Latihan silogisme dengan diagram himpunan, termasuk modus ponens dan modus tollens.",
		Topic:        "Logika Analitis",
		Category:     "Kognitif",
		Mentor:       "Bagas Nurhadi",
		DurationMins: 45,
		DaysAgo:      12,
		Accent:       "from-sky-500/80 to-navy",
		VideoURL:     "/media/lessons/analitis-silogisme.mp4",
	},
	{
		ID:           "rec-akhlak-studi-kasus",
		Title:        "Studi Kasus Nilai AKHLAK",
		Description:  "Membahas studi kasus kerja untuk memilih nilai inti AKHLAK yang paling tepat.",
		Topic:        "Nilai AKHLAK & Wawasan",
		Category:     "AKHLAK",
		Mentor:       "Sari Mahendra",
		DurationMins: 36,
		DaysAgo:      9,
		Accent:       "from-rose-500/80 to-navy",
		VideoURL:     "/media/lessons/akhlaq-studi-kasus.mp4",
	},
	{
		ID:           "rec-inggris-reading",
		Title:        "Reading Comprehension: Strategi Cepat",
		Description:  "Strategi membaca teks bahasa Inggris secara efisien dan menentukan gagasan utama.",
		Topic:        "Bahasa Inggris",
		Category:     "Bahasa Inggris",
		Mentor:       "Dimas Ardhana",
		DurationMins: 40,
		DaysAgo:      6,
		Accent:       "from-indigo-500/80 to-navy",
		VideoURL:     "/media/lessons/inggris-reading.mp4",
	},
}

// seedLiveSessions mengisi jadwal kelas live.
//
// Baris yang sudah ada tidak ditimpa kecuali jadwalnya sudah lewat dan statusnya
// masih "upcoming" — pada kasus itu jadwal digeser ulang agar demo tetap
// menampilkan sesi yang akan datang.
func seedLiveSessions(ctx context.Context, db *gorm.DB) error {
	now := time.Now().UTC()

	for _, def := range liveSessionDefs {
		startsAt := now.Add(time.Duration(def.OffsetHours) * time.Hour)

		var existing models.LiveSession
		err := db.WithContext(ctx).First(&existing, "id = ?", def.ID).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			session := models.LiveSession{
				ID:              def.ID,
				Title:           def.Title,
				Topic:           def.Topic,
				Mentor:          def.Mentor,
				MentorRole:      def.MentorRole,
				StartsAt:        startsAt,
				DurationMinutes: def.DurationMins,
				Status:          def.Status,
				Platform:        def.Platform,
				Note:            stringPtr(def.Note),
			}
			if err := db.WithContext(ctx).Create(&session).Error; err != nil {
				return fmt.Errorf("seed: gagal membuat jadwal live %s: %w", def.ID, err)
			}
		case err != nil:
			return fmt.Errorf("seed: gagal membaca jadwal live %s: %w", def.ID, err)
		default:
			if def.Status != models.LiveUpcoming || existing.StartsAt.After(now) {
				continue
			}
			if err := db.WithContext(ctx).Model(&models.LiveSession{}).
				Where("id = ?", def.ID).
				Updates(map[string]any{"starts_at": startsAt, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("seed: gagal menggeser jadwal live %s: %w", def.ID, err)
			}
		}
	}
	return nil
}

// seedRecordings mengisi daftar rekaman kelas (idempoten).
func seedRecordings(ctx context.Context, db *gorm.DB) error {
	now := time.Now().UTC()

	for _, def := range recordingDefs {
		recording := models.Recording{
			ID:              def.ID,
			Title:           def.Title,
			Description:     def.Description,
			Topic:           def.Topic,
			Category:        def.Category,
			Mentor:          def.Mentor,
			DurationMinutes: def.DurationMins,
			PublishedAt:     now.Add(-time.Duration(def.DaysAgo) * 24 * time.Hour),
			Accent:          def.Accent,
			VideoURL:        stringPtr(def.VideoURL),
		}

		// `published_at` hanya dipakai saat baris pertama kali dibuat agar urutan
		// rekaman tidak berubah setiap kali server start.
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"title", "description", "topic", "category", "mentor",
				"duration_minutes", "accent", "video_url", "updated_at",
			}),
		}).Create(&recording).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan rekaman %s: %w", def.ID, err)
		}
	}
	return nil
}
