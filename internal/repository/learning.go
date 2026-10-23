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

// LearningRepository mengakses kelas live, rekaman, pengingat, dan broadcast.
type LearningRepository struct {
	db *gorm.DB
}

// NewLearningRepository membentuk repository.
func NewLearningRepository(db *gorm.DB) *LearningRepository {
	return &LearningRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *LearningRepository) DB() *gorm.DB { return r.db }

// ListLiveSessions mengambil jadwal kelas live.
func (r *LearningRepository) ListLiveSessions(ctx context.Context, upcomingOnly bool) ([]models.LiveSession, error) {
	query := r.db.WithContext(ctx).Model(&models.LiveSession{})
	if upcomingOnly {
		query = query.Where("status IN ?", []models.LiveStatus{models.LiveOngoing, models.LiveUpcoming})
	}
	var out []models.LiveSession
	if err := query.Order("starts_at ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jadwal live.")
	}
	return out, nil
}

// UpsertLiveSession menyisipkan/memperbarui jadwal live.
func (r *LearningRepository) UpsertLiveSession(ctx context.Context, session *models.LiveSession) error {
	err := r.db.WithContext(ctx).
		Where("id = ?", session.ID).
		Assign(map[string]any{
			"title":            session.Title,
			"topic":            session.Topic,
			"mentor":           session.Mentor,
			"mentor_role":      session.MentorRole,
			"starts_at":        session.StartsAt,
			"duration_minutes": session.DurationMinutes,
			"status":           session.Status,
			"platform":         session.Platform,
			"note":             session.Note,
			"meeting_url":      session.MeetingURL,
		}).
		FirstOrCreate(session).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan jadwal live.")
	}
	return nil
}

// ListRecordings mengambil koleksi rekaman dengan filter opsional.
func (r *LearningRepository) ListRecordings(ctx context.Context, category, search string) ([]models.Recording, error) {
	query := r.db.WithContext(ctx).Model(&models.Recording{})
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ? OR topic ILIKE ?", like, like, like)
	}
	var out []models.Recording
	if err := query.Order("published_at DESC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat rekaman.")
	}
	return out, nil
}

// UpsertRecording menyisipkan/memperbarui rekaman.
func (r *LearningRepository) UpsertRecording(ctx context.Context, recording *models.Recording) error {
	err := r.db.WithContext(ctx).
		Where("id = ?", recording.ID).
		Assign(map[string]any{
			"title":            recording.Title,
			"description":      recording.Description,
			"topic":            recording.Topic,
			"category":         recording.Category,
			"mentor":           recording.Mentor,
			"duration_minutes": recording.DurationMinutes,
			"published_at":     recording.PublishedAt,
			"accent":           recording.Accent,
			"video_url":        recording.VideoURL,
		}).
		FirstOrCreate(recording).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan rekaman.")
	}
	return nil
}

// ReminderIDsByUser mengambil id sesi live yang diingatkan oleh peserta.
func (r *LearningRepository) ReminderIDsByUser(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&models.SessionReminder{}).
		Where("user_id = ?", userID).
		Pluck("live_session_id", &ids).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat pengingat.")
	}
	return ids, nil
}

// ToggleReminder menyalakan/mematikan pengingat sesi live dan mengembalikan
// status terbaru. Idempotent karena memakai primary key gabungan.
func (r *LearningRepository) ToggleReminder(ctx context.Context, userID, liveSessionID string) (bool, error) {
	var existing models.SessionReminder
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND live_session_id = ?", userID, liveSessionID).
		First(&existing).Error

	if err == nil {
		result := r.db.WithContext(ctx).
			Where("user_id = ? AND live_session_id = ?", userID, liveSessionID).
			Delete(&models.SessionReminder{})
		if result.Error != nil {
			return false, apperror.Internal(result.Error, "Gagal mematikan pengingat.")
		}
		return false, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, apperror.Internal(err, "Gagal memeriksa pengingat.")
	}

	reminder := models.SessionReminder{UserID: userID, LiveSessionID: liveSessionID}
	createErr := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&reminder).Error
	if createErr != nil {
		return false, apperror.Internal(createErr, "Gagal menyimpan pengingat.")
	}
	return true, nil
}

// CreateBroadcastLog mencatat broadcast admin.
func (r *LearningRepository) CreateBroadcastLog(ctx context.Context, log *models.BroadcastLog) error {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat broadcast.")
	}
	return nil
}

// ListBroadcastLogs mengambil riwayat broadcast terbaru.
func (r *LearningRepository) ListBroadcastLogs(ctx context.Context, limit int) ([]models.BroadcastLog, error) {
	var out []models.BroadcastLog
	err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat riwayat broadcast.")
	}
	return out, nil
}

// CreateNotificationLog mencatat notifikasi transaksi.
func (r *LearningRepository) CreateNotificationLog(ctx context.Context, log *models.NotificationLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat notifikasi.")
	}
	return nil
}
