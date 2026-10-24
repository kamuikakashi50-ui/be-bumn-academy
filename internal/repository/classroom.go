package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClassroomRepository mengakses modul, pelajaran, dan progres menonton.
type ClassroomRepository struct {
	db *gorm.DB
}

// NewClassroomRepository membentuk repository ruang kelas.
func NewClassroomRepository(db *gorm.DB) *ClassroomRepository {
	return &ClassroomRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *ClassroomRepository) DB() *gorm.DB { return r.db }

/* --------------------------------- modul ---------------------------------- */

// ListModules mengambil modul milik satu produk (semua produk bila kosong)
// beserta pelajarannya.
func (r *ClassroomRepository) ListModules(ctx context.Context, productSlug string, includeInactive bool) ([]models.CourseModule, error) {
	query := r.db.WithContext(ctx).Model(&models.CourseModule{})
	if productSlug != "" {
		query = query.Where("product_slug = ?", productSlug)
	}
	if !includeInactive {
		query = query.Where("active = ?", true)
	}

	var out []models.CourseModule
	err := query.
		Preload("Lessons", func(db *gorm.DB) *gorm.DB {
			if !includeInactive {
				db = db.Where("active = ?", true)
			}
			return db.Order("sort_order ASC, title ASC")
		}).
		Order("product_slug ASC, sort_order ASC, title ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat modul ruang kelas.")
	}
	return out, nil
}

// FindModule mengambil satu modul.
func (r *ClassroomRepository) FindModule(ctx context.Context, id string) (*models.CourseModule, error) {
	var module models.CourseModule
	err := r.db.WithContext(ctx).First(&module, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Modul tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat modul.")
	}
	return &module, nil
}

// SaveModule menyisipkan atau memperbarui modul.
func (r *ClassroomRepository) SaveModule(ctx context.Context, module *models.CourseModule) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"product_slug", "title", "description", "sort_order", "active", "updated_at",
		}),
	}).Create(module).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan modul.")
	}
	return nil
}

// DeleteModule menghapus modul beserta pelajarannya (cascade).
func (r *ClassroomRepository) DeleteModule(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.CourseModule{}, "id = ?", id)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus modul.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Modul tidak ditemukan.")
	}
	return nil
}

/* -------------------------------- pelajaran -------------------------------- */

// ListLessonsByProduct mengambil pelajaran satu produk (semua produk bila kosong).
func (r *ClassroomRepository) ListLessonsByProduct(ctx context.Context, productSlug string, includeInactive bool) ([]models.Lesson, error) {
	query := r.db.WithContext(ctx).Model(&models.Lesson{})
	if productSlug != "" {
		query = query.Where("product_slug = ?", productSlug)
	}
	if !includeInactive {
		query = query.Where("active = ?", true)
	}

	var out []models.Lesson
	if err := query.Order("product_slug ASC, sort_order ASC, title ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar pelajaran.")
	}
	return out, nil
}

// FindLesson mengambil satu pelajaran.
func (r *ClassroomRepository) FindLesson(ctx context.Context, id string) (*models.Lesson, error) {
	var lesson models.Lesson
	err := r.db.WithContext(ctx).First(&lesson, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Pelajaran tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat pelajaran.")
	}
	return &lesson, nil
}

// SaveLesson menyisipkan atau memperbarui pelajaran.
func (r *ClassroomRepository) SaveLesson(ctx context.Context, lesson *models.Lesson) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"module_id", "product_slug", "title", "summary", "kind", "video_url",
			"ebook_url", "poster_url", "content", "duration_seconds", "sort_order", "active", "updated_at",
		}),
	}).Create(lesson).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan pelajaran.")
	}
	return nil
}

// DeleteLesson menghapus pelajaran beserta progresnya (cascade).
func (r *ClassroomRepository) DeleteLesson(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.Lesson{}, "id = ?", id)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus pelajaran.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Pelajaran tidak ditemukan.")
	}
	return nil
}

/* --------------------------------- progres --------------------------------- */

// ProgressByUser mengambil seluruh progres seorang peserta.
func (r *ClassroomRepository) ProgressByUser(ctx context.Context, userID string) ([]models.LessonProgress, error) {
	var out []models.LessonProgress
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat progres belajar.")
	}
	return out, nil
}

// FindProgress mengambil progres satu pelajaran milik peserta (nil bila belum ada).
func (r *ClassroomRepository) FindProgress(ctx context.Context, userID, lessonID string) (*models.LessonProgress, error) {
	var progress models.LessonProgress
	err := r.db.WithContext(ctx).
		First(&progress, "user_id = ? AND lesson_id = ?", userID, lessonID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat progres pelajaran.")
	}
	return &progress, nil
}

// SaveProgress menyimpan (upsert) progres menonton satu pelajaran.
func (r *ClassroomRepository) SaveProgress(ctx context.Context, progress *models.LessonProgress) error {
	progress.LastViewedAt = time.Now().UTC()
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "lesson_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"position_seconds", "progress_percent", "completed", "last_viewed_at", "updated_at",
		}),
	}).Create(progress).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan progres pelajaran.")
	}
	return nil
}

// LastViewedProgress mengambil pelajaran terakhir yang dibuka peserta (terbaru
// lebih dulu) — dipakai kartu "lanjutkan belajar" di dashboard.
func (r *ClassroomRepository) LastViewedProgress(ctx context.Context, userID string, limit int) ([]models.LessonProgress, error) {
	var rows []models.LessonProgress
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_viewed_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat pelajaran terakhir.")
	}
	return rows, nil
}

// LessonsByIDs mengambil beberapa pelajaran sekaligus.
func (r *ClassroomRepository) LessonsByIDs(ctx context.Context, ids []string) ([]models.Lesson, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []models.Lesson
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat detail pelajaran.")
	}
	return out, nil
}
