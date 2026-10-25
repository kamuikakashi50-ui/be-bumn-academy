package seed

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Berkas ini mengisi data kohort demo untuk dasbor Analitik HR (/admin/hr):
// beberapa segmen program, riwayat latihan yang tersebar dalam enam bulan, dan
// progres ruang kelas yang beragam.
//
// Akun inti (peserta@bumnacademy.id & admin@bumnacademy.id) sengaja TIDAK diberi
// percobaan/progres di sini agar kontrak skrip verifikasi tetap berlaku
// (mis. `scripts/smoke-test.sh` menegaskan peserta inti punya tepat satu hasil
// tryout setelah submit, dan `verify-integration.sh` memilih materi video yang
// belum pernah dibuka).

// hrCohortPassword adalah kata sandi seluruh akun kohort demo.
const hrCohortPassword = "peserta123"

// hrCohortUsers adalah kohort peserta demo analitik HR.
//
// Segmen (kolom `users.program`) sengaja beragam agar panel "Sebaran per
// program" terisi; akun tanpa percobaan (Hana & Rian) membuat metrik "belum
// pernah latihan" dan daftar pendampingan terisi.
var hrCohortUsers = []UserDef{
	{Name: "Dewi Lestari", Email: "dewi.lestari@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Kelas Intensif BUMN"},
	{Name: "Rangga Pratama", Email: "rangga.pratama@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Kelas Intensif BUMN"},
	{Name: "Siti Nurhaliza", Email: "siti.nurhaliza@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Program Reguler"},
	{Name: "Bayu Saputra", Email: "bayu.saputra@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Program Reguler"},
	{Name: "Intan Permata", Email: "intan.permata@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Kelas Korporasi"},
	{Name: "Fajar Ramadhan", Email: "fajar.ramadhan@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Kelas Korporasi"},
	{Name: "Nadia Safira", Email: "nadia.safira@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Alumni & Retaker"},
	{Name: "Yoga Pratama", Email: "yoga.pratama@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Alumni & Retaker"},
	{Name: "Hana Maulida", Email: "hana.maulida@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Program Reguler"},
	{Name: "Rian Kusuma", Email: "rian.kusuma@bumnacademy.id", Password: hrCohortPassword, Role: "peserta", Program: "Kelas Intensif BUMN"},
}

// hrAttemptDef adalah satu percobaan tryout demo pada kohort analitik HR.
//
// `DaysAgo` berupa selisih hari dari waktu seed dijalankan agar tren enam bulan
// terakhir selalu terisi, sedangkan `Score` ditulis eksplisit supaya sebaran
// nilai (di bawah/atas ambang) dan peringkatnya stabil antar pemanggilan.
type hrAttemptDef struct {
	Email   string
	DaysAgo int
	Score   int
	Minutes int
}

var hrAttemptDefs = []hrAttemptDef{
	// Dewi — peserta kuat yang terus naik.
	{"dewi.lestari@bumnacademy.id", 170, 72, 45},
	{"dewi.lestari@bumnacademy.id", 140, 75, 48},
	{"dewi.lestari@bumnacademy.id", 105, 80, 44},
	{"dewi.lestari@bumnacademy.id", 70, 84, 42},
	{"dewi.lestari@bumnacademy.id", 38, 86, 40},
	{"dewi.lestari@bumnacademy.id", 14, 88, 38},

	// Rangga — mulai dari bawah, baru melewati ambang.
	{"rangga.pratama@bumnacademy.id", 162, 48, 55},
	{"rangga.pratama@bumnacademy.id", 128, 53, 58},
	{"rangga.pratama@bumnacademy.id", 92, 59, 52},
	{"rangga.pratama@bumnacademy.id", 60, 63, 49},
	{"rangga.pratama@bumnacademy.id", 26, 68, 45},
	{"rangga.pratama@bumnacademy.id", 4, 71, 42},

	// Siti — stabil di atas ambang.
	{"siti.nurhaliza@bumnacademy.id", 150, 64, 50},
	{"siti.nurhaliza@bumnacademy.id", 112, 68, 47},
	{"siti.nurhaliza@bumnacademy.id", 78, 72, 45},
	{"siti.nurhaliza@bumnacademy.id", 45, 76, 43},
	{"siti.nurhaliza@bumnacademy.id", 17, 79, 40},

	// Bayu — skor teratas, tetapi sudah 38 hari tidak berlatih.
	{"bayu.saputra@bumnacademy.id", 140, 90, 36},
	{"bayu.saputra@bumnacademy.id", 100, 93, 34},
	{"bayu.saputra@bumnacademy.id", 56, 95, 33},
	{"bayu.saputra@bumnacademy.id", 38, 92, 35},

	// Intan — perlu pendampingan intensif.
	{"intan.permata@bumnacademy.id", 132, 57, 58},
	{"intan.permata@bumnacademy.id", 95, 61, 54},
	{"intan.permata@bumnacademy.id", 60, 65, 51},
	{"intan.permata@bumnacademy.id", 30, 69, 48},
	{"intan.permata@bumnacademy.id", 10, 72, 45},

	// Fajar — skor terakhir masih di bawah ambang.
	{"fajar.ramadhan@bumnacademy.id", 122, 51, 60},
	{"fajar.ramadhan@bumnacademy.id", 80, 55, 56},
	{"fajar.ramadhan@bumnacademy.id", 47, 59, 53},
	{"fajar.ramadhan@bumnacademy.id", 20, 64, 50},

	// Nadia — alumni yang kembali berlatih.
	{"nadia.safira@bumnacademy.id", 108, 81, 42},
	{"nadia.safira@bumnacademy.id", 70, 85, 40},
	{"nadia.safira@bumnacademy.id", 24, 89, 38},

	// Yoga — masih jauh di bawah ambang.
	{"yoga.pratama@bumnacademy.id", 90, 46, 62},
	{"yoga.pratama@bumnacademy.id", 52, 50, 58},
	{"yoga.pratama@bumnacademy.id", 28, 54, 55},
}

// hrTryoutRotation adalah urutan paket tryout yang dipakai bergantian oleh
// kohort agar `distinctTryouts`, profil subtes, dan papan peringkat terisi.
var hrTryoutRotation = []string{
	"to-demo-nasional",
	"to-tkd-01",
	"to-tkd-02",
	"to-akhlak-01",
	"to-inggris-01",
}

// hrProductAccess adalah produk yang diberikan ke kohort agar paket tryout
// premium pada rotasi di atas benar-benar bisa dibuka peserta.
var hrProductAccess = []string{
	"kelas-intensif",
	"paket-latihan",
	"kelas-live",
	"tryout-demo",
}

// hrSubtestPattern menggeser skor subtes relatif terhadap skor total sehingga
// profil kompetensi antar peserta berbeda (ada yang kuat numerik, ada yang kuat
// verbal). Nilai dipakai bergilir memakai indeks peserta + indeks subtes.
var hrSubtestPattern = []int{6, -4, 2, -8, 4, -2}

// hrProgressPattern adalah sebaran persentase progres materi yang dipakai
// bergilir antar peserta.
var hrProgressPattern = []int{100, 90, 60, 100, 45, 100, 75, 100, 30, 95, 100, 55, 100, 20, 80}

// hrCompletedThreshold harus sama dengan service.CompletedThreshold (90%) agar
// progres hasil seed konsisten dengan perhitungan backend.
const hrCompletedThreshold = 90

// seedHRActivity mengisi akses produk, riwayat latihan, dan progres materi untuk
// kohort demo analitik HR.
func seedHRActivity(
	ctx context.Context,
	db *gorm.DB,
	data *Dataset,
	classroom *ClassroomDataset,
	userIDs map[string]string,
	threshold int,
) error {
	tryouts := make(map[string]TryoutDef, len(data.Tryouts))
	for _, tryout := range data.Tryouts {
		tryouts[tryout.ID] = tryout
	}

	rotation := make([]TryoutDef, 0, len(hrTryoutRotation))
	for _, id := range hrTryoutRotation {
		tryout, ok := tryouts[id]
		if !ok {
			return fmt.Errorf("seed: rotasi tryout analitik HR menunjuk paket %s yang tidak ada", id)
		}
		rotation = append(rotation, tryout)
	}

	if err := seedHRProductAccess(ctx, db, userIDs); err != nil {
		return err
	}
	if err := seedHRAttempts(ctx, db, hrAttemptDefs, rotation, userIDs, time.Now().UTC(), threshold); err != nil {
		return err
	}
	return seedHRLessonProgress(ctx, db, classroom, hrCohortUsers, userIDs, time.Now().UTC())
}

// seedHRProductAccess memberi kohort akses ke seluruh produk demo agar paket
// tryout premium pada rotasi bisa dibuka (idempoten, tidak menimpa pembelian).
func seedHRProductAccess(ctx context.Context, db *gorm.DB, userIDs map[string]string) error {
	for _, user := range hrCohortUsers {
		userID, ok := userIDs[user.Email]
		if !ok {
			return fmt.Errorf("seed: akun kohort %s belum tersedia", user.Email)
		}
		for _, slug := range hrProductAccess {
			enrollment := models.Enrollment{
				ID:          stableID("enrollment/" + user.Email + "/" + slug),
				UserID:      userID,
				ProductSlug: slug,
				Source:      "seed",
			}
			if err := db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}, {Name: "product_slug"}},
				DoNothing: true,
			}).Create(&enrollment).Error; err != nil {
				return fmt.Errorf("seed: gagal memberi akses %s ke %s: %w", slug, user.Email, err)
			}
		}
	}
	return nil
}

// seedHRAttempts menulis riwayat percobaan tryout kohort beserta rincian skor
// per subtes.
//
// ID percobaan deterministik (`stableID`) sehingga seed berulang memperbarui
// baris yang sama, bukan menumpuk riwayat baru.
func seedHRAttempts(
	ctx context.Context,
	db *gorm.DB,
	defs []hrAttemptDef,
	rotation []TryoutDef,
	userIDs map[string]string,
	now time.Time,
	threshold int,
) error {
	if len(rotation) == 0 {
		return fmt.Errorf("seed: rotasi tryout analitik HR kosong")
	}

	// Indeks peserta dipakai agar pola subtes & rotasi tryout berbeda per orang.
	learnerIndex := make(map[string]int)
	for _, def := range defs {
		if _, ok := learnerIndex[def.Email]; !ok {
			learnerIndex[def.Email] = len(learnerIndex)
		}
	}

	slots := make(map[string]int)
	for _, def := range defs {
		userID, ok := userIDs[def.Email]
		if !ok {
			return fmt.Errorf("seed: percobaan menunjuk akun %s yang tidak dikenal", def.Email)
		}

		index := learnerIndex[def.Email]
		slot := slots[def.Email]
		slots[def.Email] = slot + 1

		tryout := rotation[(index+slot)%len(rotation)]
		takenAt := now.AddDate(0, 0, -def.DaysAgo)

		attemptID := stableID(fmt.Sprintf("attempt/%s/%s/%d", def.Email, tryout.ID, def.DaysAgo))
		categories, correct, total := buildAttemptCategories(attemptID, def.Score, index, tryout.Subtests)

		attempt := models.Attempt{
			ID:              attemptID,
			UserID:          userID,
			TryoutID:        tryout.ID,
			Title:           tryout.Title,
			Score:           def.Score,
			Correct:         correct,
			Total:           total,
			Status:          attemptStatus(def.Score, threshold),
			DurationSeconds: def.Minutes * 60,
			TakenAt:         takenAt,
			CreatedAt:       takenAt,
		}

		if err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id", "tryout_id", "title", "score", "correct", "total",
				"status", "duration_seconds", "taken_at",
			}),
		}).Create(&attempt).Error; err != nil {
			return fmt.Errorf("seed: gagal menyimpan percobaan %s: %w", attemptID, err)
		}

		// Rincian per subtes tidak punya kunci unik alami → ditulis ulang agar
		// seed berulang tidak menumpuk baris.
		if err := db.WithContext(ctx).Where("attempt_id = ?", attemptID).
			Delete(&models.AttemptCategory{}).Error; err != nil {
			return fmt.Errorf("seed: gagal membersihkan rincian subtes %s: %w", attemptID, err)
		}
		if len(categories) > 0 {
			if err := db.WithContext(ctx).Create(&categories).Error; err != nil {
				return fmt.Errorf("seed: gagal menyimpan rincian subtes %s: %w", attemptID, err)
			}
		}
	}
	return nil
}

// buildAttemptCategories menurunkan skor tiap subtes dari skor total percobaan.
//
// Setiap subtes memakai 10 soal; pola pergeseran digeser memakai indeks peserta
// sehingga profil kompetensi antar peserta berbeda. Skor dibatasi 0–100 agar
// tidak melanggar CHECK constraint `attempts_score_check`.
func buildAttemptCategories(
	attemptID string,
	score, learnerIndex int,
	subtests []SubtestDef,
) ([]models.AttemptCategory, int, int) {
	const perSubtest = 10

	out := make([]models.AttemptCategory, 0, len(subtests))
	correctSum, totalSum := 0, 0
	for index, subtest := range subtests {
		offset := hrSubtestPattern[(learnerIndex+index)%len(hrSubtestPattern)]
		subScore := clampScore(score + offset)
		correct := int(math.Round(float64(subScore) / 100 * perSubtest))
		out = append(out, models.AttemptCategory{
			AttemptID: attemptID,
			Name:      subtest.Name,
			Score:     subScore,
			Correct:   correct,
			Total:     perSubtest,
			SortOrder: index,
		})
		correctSum += correct
		totalSum += perSubtest
	}
	return out, correctSum, totalSum
}

// attemptStatus menentukan status hasil relatif terhadap ambang latihan.
func attemptStatus(score, threshold int) models.AttemptStatus {
	if score >= threshold {
		return models.AttemptPassed
	}
	return models.AttemptFailed
}

// clampScore membatasi skor ke rentang 0–100.
func clampScore(value int) int {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

// seedHRLessonProgress menulis progres ruang kelas kohort dengan sebaran
// persentase yang beragam (termasuk materi yang benar-benar selesai) sehingga
// metrik kelengkapan belajar & "progres materi" pada dasbor Analitik HR terisi.
func seedHRLessonProgress(
	ctx context.Context,
	db *gorm.DB,
	classroom *ClassroomDataset,
	users []UserDef,
	userIDs map[string]string,
	now time.Time,
) error {
	lessons := make([]LessonDef, 0, 16)
	for _, module := range classroom.Modules {
		lessons = append(lessons, module.Lessons...)
	}
	if len(lessons) == 0 {
		return nil
	}

	for index, user := range users {
		userID, ok := userIDs[user.Email]
		if !ok {
			return fmt.Errorf("seed: progres menunjuk akun %s yang tidak dikenal", user.Email)
		}

		// Tiap peserta membuka sejumlah materi berbeda (3–8) mulai dari titik
		// yang berbeda pula, sehingga "peserta aktif" tidak seragam.
		count := 3 + index%6
		for offset := 0; offset < count && offset < len(lessons); offset++ {
			lesson := lessons[(index+offset)%len(lessons)]
			percent := hrProgressPattern[(index*3+offset)%len(hrProgressPattern)]
			if percent <= 0 {
				continue
			}

			progress := models.LessonProgress{
				UserID:          userID,
				LessonID:        lesson.ID,
				PositionSeconds: lesson.DurationSeconds * percent / 100,
				ProgressPercent: percent,
				Completed:       percent >= hrCompletedThreshold,
				LastViewedAt:    now.AddDate(0, 0, -(index%5 + offset)),
			}

			if err := db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}, {Name: "lesson_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"position_seconds", "progress_percent", "completed",
					"last_viewed_at", "updated_at",
				}),
			}).Create(&progress).Error; err != nil {
				return fmt.Errorf("seed: gagal menyimpan progres %s/%s: %w", user.Email, lesson.ID, err)
			}
		}
	}
	return nil
}
