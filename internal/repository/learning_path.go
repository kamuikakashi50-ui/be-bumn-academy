package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LearningPathRepository mengakses jalur belajar, kuis per pelajaran, dan
// sertifikat penyelesaian (Fase 1).
type LearningPathRepository struct {
	db *gorm.DB
}

// NewLearningPathRepository membentuk repository.
func NewLearningPathRepository(db *gorm.DB) *LearningPathRepository {
	return &LearningPathRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *LearningPathRepository) DB() *gorm.DB { return r.db }

/* ------------------------------ jalur belajar ------------------------------ */

// PathFilter adalah filter daftar jalur belajar.
type PathFilter struct {
	SectorKey   string
	TestType    string
	ProductSlug string
	Search      string
	ActiveOnly  bool
}

// ListPaths mengambil daftar jalur belajar beserta langkah-langkahnya.
//
// Memakai Preload (2 query, bukan N+1) agar siap dipakai handler.
func (r *LearningPathRepository) ListPaths(ctx context.Context, filter PathFilter) ([]models.LearningPath, error) {
	query := r.db.WithContext(ctx).Model(&models.LearningPath{})
	if filter.SectorKey != "" {
		query = query.Where("sector_key = ?", filter.SectorKey)
	}
	if filter.TestType != "" {
		query = query.Where("test_type = ?", filter.TestType)
	}
	if filter.ProductSlug != "" {
		query = query.Where("product_slug = ?", filter.ProductSlug)
	}
	if filter.Search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	if filter.ActiveOnly {
		query = query.Where("active = ?", true)
	}

	var out []models.LearningPath
	err := query.
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, module_id ASC")
		}).
		Order("sort_order ASC, title ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jalur belajar.")
	}
	return out, nil
}

// FindPath mengambil satu jalur belajar beserta langkahnya.
func (r *LearningPathRepository) FindPath(ctx context.Context, id string) (*models.LearningPath, error) {
	var path models.LearningPath
	err := r.db.WithContext(ctx).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, module_id ASC")
		}).
		First(&path, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Jalur belajar tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat jalur belajar.")
	}
	return &path, nil
}

// FindPathBySlug mengambil satu jalur belajar berdasarkan slug-nya.
func (r *LearningPathRepository) FindPathBySlug(ctx context.Context, slug string) (*models.LearningPath, error) {
	var path models.LearningPath
	err := r.db.WithContext(ctx).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, module_id ASC")
		}).
		First(&path, "slug = ?", slug).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Jalur belajar tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat jalur belajar.")
	}
	return &path, nil
}

// UpsertPath menyimpan jalur belajar beserta langkahnya.
//
// Langkah lama diganti seluruhnya di dalam satu transaksi agar susunan yang
// tersimpan selalu konsisten dengan yang dikirim admin.
//
// `Omit("Steps")` wajib: tanpa itu GORM ikut meng-upsert asosiasi `Steps` saat
// `Create`, sehingga baris langkah lama bertabrakan dengan constraint unik
// `(path_id, module_id)` — penyimpanan jalur yang sudah punya langkah akan gagal.
func (r *LearningPathRepository) UpsertPath(ctx context.Context, path *models.LearningPath, steps []models.LearningPathStep) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Steps").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"slug", "title", "description", "sector_key", "test_type",
				"product_slug", "certificate", "sort_order", "active", "updated_at",
			}),
		}).Create(path).Error; err != nil {
			return err
		}

		if err := tx.Where("path_id = ?", path.ID).Delete(&models.LearningPathStep{}).Error; err != nil {
			return err
		}
		if len(steps) == 0 {
			return nil
		}
		for i := range steps {
			steps[i].PathID = path.ID
			steps[i].SortOrder = i
			if steps[i].ID == "" {
				steps[i].ID = models.NewID()
			}
		}
		return tx.Create(&steps).Error
	})
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan jalur belajar.")
	}
	return nil
}

// SetPathActive mengaktifkan/menonaktifkan jalur belajar (soft delete).
func (r *LearningPathRepository) SetPathActive(ctx context.Context, id string, active bool) error {
	result := r.db.WithContext(ctx).Model(&models.LearningPath{}).
		Where("id = ?", id).
		Updates(map[string]any{"active": active, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal memperbarui jalur belajar.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Jalur belajar tidak ditemukan.")
	}
	return nil
}

/* --------------------------- kuis per pelajaran ---------------------------- */

// ListQuizzesByLesson mengambil kuis aktif sebuah pelajaran beserta susunan soalnya.
func (r *LearningPathRepository) ListQuizzesByLesson(ctx context.Context, lessonID string, activeOnly bool) ([]models.LessonQuiz, error) {
	query := r.db.WithContext(ctx).Model(&models.LessonQuiz{}).Where("lesson_id = ?", lessonID)
	if activeOnly {
		query = query.Where("active = ?", true)
	}

	var out []models.LessonQuiz
	// Terbaru lebih dulu: bila admin membuat kuis baru untuk pelajaran yang sama,
	// kuis itulah yang dipakai peserta (pola "versi baru menggantikan versi lama").
	// `id` dipakai sebagai pemecah seri agar urutannya stabil.
	err := query.
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Order("created_at DESC, id DESC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kuis pelajaran.")
	}
	return out, nil
}

// FindQuiz mengambil satu kuis beserta susunan soalnya.
func (r *LearningPathRepository) FindQuiz(ctx context.Context, id string) (*models.LessonQuiz, error) {
	var quiz models.LessonQuiz
	err := r.db.WithContext(ctx).
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&quiz, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Kuis tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat kuis.")
	}
	return &quiz, nil
}

// UpsertQuiz menyimpan kuis beserta susunan soalnya.
//
// `questionIDs` adalah snapshot soal yang dipilih admin; urutannya menjadi
// `position` di dalam kuis. Soal yang tidak lagi dikirim akan dihapus dari kuis.
//
// `Omit("Questions")` wajib dengan alasan yang sama seperti `UpsertPath`: susunan
// soal dikelola eksplisit di bawah, bukan lewat asosiasi GORM.
func (r *LearningPathRepository) UpsertQuiz(ctx context.Context, quiz *models.LessonQuiz, questionIDs []string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Questions").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"lesson_id", "title", "pass_score", "max_attempts",
				"time_limit", "active", "updated_at",
			}),
		}).Create(quiz).Error; err != nil {
			return err
		}

		if err := tx.Where("quiz_id = ?", quiz.ID).Delete(&models.LessonQuizQuestion{}).Error; err != nil {
			return err
		}
		if len(questionIDs) == 0 {
			return nil
		}

		rows := make([]models.LessonQuizQuestion, 0, len(questionIDs))
		for position, questionID := range questionIDs {
			id := questionID
			rows = append(rows, models.LessonQuizQuestion{
				QuizID:     quiz.ID,
				QuestionID: &id,
				Position:   position,
			})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan kuis.")
	}
	return nil
}

// SetQuizActive mengaktifkan/menonaktifkan kuis (soft delete).
func (r *LearningPathRepository) SetQuizActive(ctx context.Context, id string, active bool) error {
	result := r.db.WithContext(ctx).Model(&models.LessonQuiz{}).
		Where("id = ?", id).
		Updates(map[string]any{"active": active, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal memperbarui kuis.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Kuis tidak ditemukan.")
	}
	return nil
}

// QuizHasBrokenQuestions menandai kuis yang salah satu posisinya kehilangan soal.
//
// Terjadi bila soal dihapus dari bank soal (FK ON DELETE SET NULL). Panel admin
// memakai penanda ini untuk menyarankan peninjauan ulang kuis.
func (r *LearningPathRepository) QuizHasBrokenQuestions(ctx context.Context, quizID string) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.LessonQuizQuestion{}).
		Where("quiz_id = ? AND question_id IS NULL", quizID).
		Count(&total).Error
	if err != nil {
		return false, apperror.Internal(err, "Gagal memeriksa kelengkapan kuis.")
	}
	return total > 0, nil
}

/* ---------------------------- percobaan kuis ------------------------------ */

// CountFinishedAttempts menghitung percobaan yang sudah dikumpulkan.
//
// Percobaan yang belum dikumpulkan (`finished_at IS NULL`) tidak dihitung, agar
// peserta yang menutup halaman di tengah kuis tidak kehilangan jatah percobaan.
func (r *LearningPathRepository) CountFinishedAttempts(ctx context.Context, quizID, userID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.QuizAttempt{}).
		Where("quiz_id = ? AND user_id = ? AND finished_at IS NOT NULL", quizID, userID).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung percobaan kuis.")
	}
	return total, nil
}

// ListAttemptsByUser mengambil riwayat percobaan peserta pada satu kuis.
func (r *LearningPathRepository) ListAttemptsByUser(ctx context.Context, quizID, userID string, limit int) ([]models.QuizAttempt, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var out []models.QuizAttempt
	err := r.db.WithContext(ctx).
		Where("quiz_id = ? AND user_id = ?", quizID, userID).
		Order("started_at DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat riwayat percobaan kuis.")
	}
	return out, nil
}

// CreateAttempt menyimpan percobaan kuis baru.
func (r *LearningPathRepository) CreateAttempt(ctx context.Context, attempt *models.QuizAttempt) error {
	if err := r.db.WithContext(ctx).Create(attempt).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan percobaan kuis.")
	}
	return nil
}

// FinishAttempt menyimpan hasil akhir percobaan kuis.
func (r *LearningPathRepository) FinishAttempt(ctx context.Context, attempt *models.QuizAttempt) error {
	err := r.db.WithContext(ctx).Model(&models.QuizAttempt{}).
		Where("id = ?", attempt.ID).
		Updates(map[string]any{
			"score":       attempt.Score,
			"correct":     attempt.Correct,
			"total":       attempt.Total,
			"passed":      attempt.Passed,
			"answers":     attempt.Answers,
			"finished_at": attempt.FinishedAt,
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan hasil kuis.")
	}
	return nil
}

/* ------------------------------- sertifikat -------------------------------- */

// CreateCertificateForUser menyimpan sertifikat secara atomik dan idempoten.
//
// Seluruhnya dalam satu transaksi:
//  1. baris peserta dikunci (`SELECT … FOR UPDATE`) sehingga dua permintaan
//     bersamaan tidak menerbitkan dua sertifikat untuk (peserta, jalur) yang sama;
//  2. bila sertifikat (peserta, jalur) sudah ada, baris itu dikembalikan apa adanya
//     (termasuk yang sudah dicabut — pencabutan tidak boleh dibatalkan otomatis);
//  3. nomor diambil dari counter tahun berjalan di transaksi yang sama.
//
// `year` adalah tahun penomoran (dipakai pemanggil agar konsisten dengan zona waktu
// Indonesia, bukan zona server).
func (r *LearningPathRepository) CreateCertificateForUser(
	ctx context.Context,
	certificate *models.Certificate,
	year int,
) (*models.Certificate, error) {
	var result models.Certificate

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedUserID string
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&models.User{}).
			Where("id = ?", certificate.UserID).
			Pluck("id", &lockedUserID).Error; err != nil {
			return err
		}
		if lockedUserID == "" {
			return apperror.NotFound("Peserta tidak ditemukan.")
		}

		if certificate.PathID != nil {
			var existing models.Certificate
			err := tx.First(&existing, "user_id = ? AND path_id = ?",
				certificate.UserID, *certificate.PathID).Error
			switch {
			case err == nil:
				result = existing
				return nil
			case !errors.Is(err, gorm.ErrRecordNotFound):
				return err
			}
		}

		number, err := nextCertificateNumberTx(tx, year)
		if err != nil {
			return err
		}
		certificate.Number = number

		if err := tx.Create(certificate).Error; err != nil {
			return err
		}
		result = *certificate
		return nil
	})
	if err != nil {
		if appErr := apperror.From(err); appErr.Code != apperror.CodeInternal {
			return nil, appErr
		}
		return nil, apperror.Internal(err, "Gagal menerbitkan sertifikat.")
	}
	return &result, nil
}

// nextCertificateNumberTx mengambil nomor berikutnya di dalam transaksi pemanggil.
//
// Baris counter tahun ini dibuat bila belum ada, lalu dikunci (`FOR UPDATE`)
// sebelum dinaikkan, sehingga nomor tidak pernah kembar.
func nextCertificateNumberTx(tx *gorm.DB, year int) (string, error) {
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.CertificateCounter{Year: year}).Error; err != nil {
		return "", err
	}

	var counter models.CertificateCounter
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&counter, "year = ?", year).Error; err != nil {
		return "", err
	}

	counter.LastNumber++
	if err := tx.Model(&models.CertificateCounter{}).
		Where("year = ?", year).
		Updates(map[string]any{
			"last_number": counter.LastNumber,
			"updated_at":  time.Now().UTC(),
		}).Error; err != nil {
		return "", err
	}

	return models.CertificateNumber(year, counter.LastNumber), nil
}

// NextCertificateNumber mengambil nomor sertifikat berikutnya untuk satu tahun.
//
// Dipakai pengujian & pemakaian di luar alur penerbitan; alur normal memakai
// `CreateCertificateForUser` yang mengambil nomor di dalam transaksi yang sama
// dengan penyimpanan sertifikat.
func (r *LearningPathRepository) NextCertificateNumber(ctx context.Context, year int) (string, error) {
	var number string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		value, err := nextCertificateNumberTx(tx, year)
		if err != nil {
			return err
		}
		number = value
		return nil
	})
	if err != nil {
		return "", apperror.Internal(err, "Gagal menyiapkan nomor sertifikat.")
	}
	return number, nil
}

// CreateCertificate menyimpan sertifikat baru.
func (r *LearningPathRepository) CreateCertificate(ctx context.Context, certificate *models.Certificate) error {
	if err := r.db.WithContext(ctx).Create(certificate).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan sertifikat.")
	}
	return nil
}

// ListCertificatesByUser mengambil sertifikat milik satu peserta.
func (r *LearningPathRepository) ListCertificatesByUser(ctx context.Context, userID string) ([]models.Certificate, error) {
	var out []models.Certificate
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("issued_at DESC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat sertifikat.")
	}
	return out, nil
}

// FindCertificateByID mengambil satu sertifikat berdasarkan id.
func (r *LearningPathRepository) FindCertificateByID(ctx context.Context, id string) (*models.Certificate, error) {
	var certificate models.Certificate
	err := r.db.WithContext(ctx).First(&certificate, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Sertifikat tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat sertifikat.")
	}
	return &certificate, nil
}

// FindCertificateByNumber mengambil sertifikat berdasarkan nomor (verifikasi publik).
func (r *LearningPathRepository) FindCertificateByNumber(ctx context.Context, number string) (*models.Certificate, error) {
	var certificate models.Certificate
	err := r.db.WithContext(ctx).First(&certificate, "number = ?", number).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Sertifikat tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat sertifikat.")
	}
	return &certificate, nil
}

// FindCertificateByUserAndPath mencari sertifikat peserta pada satu jalur.
//
// Dipakai agar penerbitan bersifat idempoten: satu peserta satu sertifikat per
// jalur (sertifikat yang dicabut tetap dianggap sudah pernah terbit).
func (r *LearningPathRepository) FindCertificateByUserAndPath(ctx context.Context, userID, pathID string) (*models.Certificate, error) {
	var certificate models.Certificate
	err := r.db.WithContext(ctx).
		First(&certificate, "user_id = ? AND path_id = ?", userID, pathID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memeriksa sertifikat peserta.")
	}
	return &certificate, nil
}

// RevokeCertificate mencabut sertifikat (halaman verifikasi menampilkan statusnya).
func (r *LearningPathRepository) RevokeCertificate(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&models.Certificate{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": now})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mencabut sertifikat.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Sertifikat tidak ditemukan atau sudah dicabut.")
	}
	return nil
}

/* ------------------------------- progres ---------------------------------- */

// LessonStat adalah jumlah pelajaran sebuah modul beserta yang sudah selesai.
type LessonStat struct {
	ModuleID  string
	Total     int
	Completed int
}

// ModuleLessonStats menghitung progres peserta per modul.
//
// Hasilnya dipakai pemeriksaan kelayakan sertifikat: ambang "selesai" ditentukan
// service (`CompletedThreshold`), bukan di query, supaya aturannya satu tempat.
func (r *LearningPathRepository) ModuleLessonStats(ctx context.Context, userID string, moduleIDs []string) (map[string]LessonStat, error) {
	if len(moduleIDs) == 0 {
		return map[string]LessonStat{}, nil
	}

	type row struct {
		ModuleID  string
		Total     int
		Completed int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.Lesson{}).
		Select(`lessons.module_id AS module_id,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE lesson_progress.completed) AS completed`).
		Joins("LEFT JOIN lesson_progress ON lesson_progress.lesson_id = lessons.id AND lesson_progress.user_id = ?", userID).
		Where("lessons.module_id IN ?", moduleIDs).
		Where("lessons.active = ?", true).
		Group("lessons.module_id").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung progres modul.")
	}

	out := make(map[string]LessonStat, len(rows))
	for _, item := range rows {
		out[item.ModuleID] = LessonStat{
			ModuleID:  item.ModuleID,
			Total:     item.Total,
			Completed: item.Completed,
		}
	}
	return out, nil
}

// QuizAttemptRow adalah percobaan kuis beserta identitas peserta untuk rekap admin.
type QuizAttemptRow struct {
	ID         string
	UserID     string
	Name       string
	Email      string
	Score      int
	Correct    int
	Total      int
	Passed     bool
	StartedAt  time.Time
	FinishedAt *time.Time
}

// ListAttemptsByQuiz mengambil percobaan yang sudah dikumpulkan pada satu kuis
// beserta identitas peserta (satu query dengan JOIN, bukan N+1).
//
// Percobaan yang belum dikumpulkan tidak ditampilkan: yang dibutuhkan rekap admin
// adalah hasil, bukan aktivitas yang belum selesai.
func (r *LearningPathRepository) ListAttemptsByQuiz(ctx context.Context, quizID string, limit int) ([]QuizAttemptRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var rows []QuizAttemptRow
	err := r.db.WithContext(ctx).Model(&models.QuizAttempt{}).
		Select(`quiz_attempts.id, quiz_attempts.user_id, users.name, users.email,
		        quiz_attempts.score, quiz_attempts.correct, quiz_attempts.total,
		        quiz_attempts.passed, quiz_attempts.started_at, quiz_attempts.finished_at`).
		Joins("JOIN users ON users.id = quiz_attempts.user_id").
		Where("quiz_attempts.quiz_id = ? AND quiz_attempts.finished_at IS NOT NULL", quizID).
		Order("quiz_attempts.started_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat rekap percobaan kuis.")
	}
	return rows, nil
}

/* ------------------- bantuan penerbitan sertifikat (langkah 3) ------------- */

// CountPassedAttempts menghitung percobaan kuis yang sudah dikumpulkan DAN lulus.
//
// Dipakai pemeriksaan syarat sertifikat: satu kali lulus sudah cukup.
func (r *LearningPathRepository) CountPassedAttempts(ctx context.Context, quizID, userID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.QuizAttempt{}).
		Where("quiz_id = ? AND user_id = ? AND passed = ? AND finished_at IS NOT NULL",
			quizID, userID, true).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung kuis yang lulus.")
	}
	return total, nil
}

// ListActiveQuizzesByLessons mengambil kuis aktif dari sekumpulan pelajaran.
//
// Satu query untuk seluruh pelajaran agar pemeriksaan syarat sertifikat tidak
// menjadi N+1 (jalur bisa memuat banyak modul & pelajaran).
func (r *LearningPathRepository) ListActiveQuizzesByLessons(ctx context.Context, lessonIDs []string) ([]models.LessonQuiz, error) {
	if len(lessonIDs) == 0 {
		return nil, nil
	}

	var out []models.LessonQuiz
	err := r.db.WithContext(ctx).
		Where("lesson_id IN ? AND active = ?", lessonIDs, true).
		Order("created_at ASC, id ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kuis pelajaran jalur.")
	}
	return out, nil
}

// CertificateRow adalah sertifikat beserta identitas peserta & jalur (rekap admin).
type CertificateRow struct {
	ID        string
	Number    string
	UserID    string
	Name      string
	Email     string
	PathID    *string
	PathTitle *string
	IssuedAt  time.Time
	RevokedAt *time.Time
}

// CertificateFilter adalah filter daftar sertifikat pada panel admin.
type CertificateFilter struct {
	Search string
	PathID string
	// Status: "sah" (belum dicabut) atau "dicabut".
	Status string
}

// ListCertificates mengambil daftar sertifikat untuk panel admin.
//
// Memakai JOIN (bukan N+1) karena panel menampilkan nama peserta & jalur.
func (r *LearningPathRepository) ListCertificates(ctx context.Context, filter CertificateFilter, limit int) ([]CertificateRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	query := r.db.WithContext(ctx).Model(&models.Certificate{}).
		Select(`certificates.id, certificates.number, certificates.user_id,
		        users.name, users.email, certificates.path_id, learning_paths.title AS path_title,
		        certificates.issued_at, certificates.revoked_at`).
		Joins("JOIN users ON users.id = certificates.user_id").
		Joins("LEFT JOIN learning_paths ON learning_paths.id = certificates.path_id")

	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(certificates.number) LIKE ? OR LOWER(users.name) LIKE ? OR LOWER(users.email) LIKE ?",
			like, like, like)
	}
	if filter.PathID != "" {
		query = query.Where("certificates.path_id = ?", filter.PathID)
	}
	switch filter.Status {
	case "sah":
		query = query.Where("certificates.revoked_at IS NULL")
	case "dicabut":
		query = query.Where("certificates.revoked_at IS NOT NULL")
	}

	var rows []CertificateRow
	err := query.Order("certificates.issued_at DESC, certificates.number DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar sertifikat.")
	}
	return rows, nil
}

// RestoreCertificate membatalkan pencabutan sertifikat (salah cabut bisa dipulihkan).
func (r *LearningPathRepository) RestoreCertificate(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&models.Certificate{}).
		Where("id = ? AND revoked_at IS NOT NULL", id).
		Update("revoked_at", nil)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal memulihkan sertifikat.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Sertifikat tidak ditemukan atau belum dicabut.")
	}
	return nil
}
