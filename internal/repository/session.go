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

// SessionRepository mengakses sesi tryout dan jawabannya.
//
// Bagian ini adalah jalur terpanas aplikasi: saat tryout serentak, ribuan
// peserta menulis jawaban dan mengumpulkan sesi hampir bersamaan. Karena itu
// seluruh penulisan memakai satu statement (upsert / bulk insert) dan
// pengumpulan sesi memakai row lock agar idempotent.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository membentuk repository.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *SessionRepository) DB() *gorm.DB { return r.db }

// FindActiveSession mencari sesi aktif peserta untuk sebuah tryout.
// Mengembalikan (nil, nil) bila tidak ada sesi aktif.
func (r *SessionRepository) FindActiveSession(ctx context.Context, userID, tryoutID string) (*models.TryoutSession, error) {
	var session models.TryoutSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tryout_id = ? AND status = ?", userID, tryoutID, models.SessionActive).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memeriksa sesi tryout.")
	}
	return &session, nil
}

// FindSessionByID mengambil sesi berdasarkan id.
func (r *SessionRepository) FindSessionByID(ctx context.Context, id string) (*models.TryoutSession, error) {
	var session models.TryoutSession
	err := r.db.WithContext(ctx).First(&session, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Sesi tryout tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat sesi tryout.")
	}
	return &session, nil
}

// CreateSession menyimpan sesi baru.
//
// Bila peserta menekan "mulai" dua kali (double click / retry jaringan), index
// unik parsial `tryout_sessions_active_uidx` akan menolak baris kedua. Kondisi
// itu bukan error bagi pengguna: sesi aktif yang sudah ada dikembalikan.
func (r *SessionRepository) CreateSession(ctx context.Context, session *models.TryoutSession) (*models.TryoutSession, error) {
	err := r.db.WithContext(ctx).Create(session).Error
	if err == nil {
		return session, nil
	}

	if isUniqueViolation(err) {
		existing, findErr := r.FindActiveSession(ctx, session.UserID, session.TryoutID)
		if findErr == nil && existing != nil {
			return existing, nil
		}
	}
	return nil, apperror.Internal(err, "Gagal memulai sesi tryout.")
}

// isUniqueViolation memeriksa kode error unique violation PostgreSQL (23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	// Fallback berbasis teks untuk driver yang tidak mengekspos SQLState.
	message := err.Error()
	return strings.Contains(message, "23505") ||
		strings.Contains(message, "duplicate key value violates unique constraint")
}

// UpdateSessionProgress menyimpan subtes yang sedang dibuka.
func (r *SessionRepository) UpdateSessionProgress(ctx context.Context, sessionID string, currentSubtest int) error {
	err := r.db.WithContext(ctx).Model(&models.TryoutSession{}).
		Where("id = ? AND status = ?", sessionID, models.SessionActive).
		Updates(map[string]any{
			"current_subtest": currentSubtest,
			"updated_at":      time.Now().UTC(),
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal memperbarui progres sesi.")
	}
	return nil
}

// UpdateSessionDeadlines menyimpan tenggat subtes dan subtes aktif.
func (r *SessionRepository) UpdateSessionDeadlines(
	ctx context.Context,
	sessionID string,
	deadlines models.TimeList,
	currentSubtest int,
) error {
	err := r.db.WithContext(ctx).Model(&models.TryoutSession{}).
		Where("id = ? AND status = ?", sessionID, models.SessionActive).
		Updates(map[string]any{
			"subtest_deadlines": deadlines,
			"current_subtest":   currentSubtest,
			"updated_at":        time.Now().UTC(),
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal memperbarui tenggat subtes.")
	}
	return nil
}

// SaveAnswers menyimpan jawaban secara massal (upsert).
//
// Satu statement INSERT ... ON CONFLICT DO UPDATE menangani seluruh jawaban
// dalam permintaan, sehingga autosave tetap murah walau dikirim setiap
// beberapa detik oleh ribuan peserta.
func (r *SessionRepository) SaveAnswers(ctx context.Context, answers []models.TryoutAnswer) error {
	if len(answers) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "session_id"}, {Name: "question_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"selected_index", "flagged", "subtest_index", "answered_at",
			}),
		}).
		CreateInBatches(answers, 200).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan jawaban.")
	}
	return nil
}

// ListAnswers mengambil seluruh jawaban sebuah sesi.
func (r *SessionRepository) ListAnswers(ctx context.Context, sessionID string) ([]models.TryoutAnswer, error) {
	var out []models.TryoutAnswer
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("subtest_index ASC, question_id ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jawaban.")
	}
	return out, nil
}

// CountAnswered menghitung jumlah soal yang sudah dijawab pada sebuah sesi.
func (r *SessionRepository) CountAnswered(ctx context.Context, sessionID string) (int, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.TryoutAnswer{}).
		Where("session_id = ? AND selected_index IS NOT NULL", sessionID).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung jawaban.")
	}
	return int(total), nil
}

// ExpireStaleSessions menandai sesi aktif yang melewati tenggat sebagai
// kedaluwarsa. Dipanggil berkala oleh worker agar data tidak menumpuk.
func (r *SessionRepository) ExpireStaleSessions(ctx context.Context, limit int) (int64, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&models.TryoutSession{}).
		Where("status = ? AND expires_at < ?", models.SessionActive, time.Now().UTC()).
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal memuat sesi kedaluwarsa.")
	}
	if len(ids) == 0 {
		return 0, nil
	}

	result := r.db.WithContext(ctx).Model(&models.TryoutSession{}).
		Where("id IN ?", ids).
		Update("status", models.SessionExpired)
	if result.Error != nil {
		return 0, apperror.Internal(result.Error, "Gagal menandai sesi kedaluwarsa.")
	}
	return result.RowsAffected, nil
}

// FindByAttemptID mencari sesi berdasarkan attempt yang dihasilkan.
func (r *SessionRepository) FindByAttemptID(ctx context.Context, attemptID string) (*models.TryoutSession, error) {
	var session models.TryoutSession
	err := r.db.WithContext(ctx).First(&session, "attempt_id = ?", attemptID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat sesi tryout.")
	}
	return &session, nil
}

// AttemptBuilder menyusun attempt dari sesi + jawaban yang sudah tersimpan.
type AttemptBuilder func(session *models.TryoutSession, answers []models.TryoutAnswer) (*models.Attempt, error)

// SubmitSession mengumpulkan sesi secara atomik dan idempotent.
//
// Langkah:
//  1. Kunci baris sesi (SELECT ... FOR UPDATE) → mencegah dua request submit
//     bersamaan menghitung/menyimpan attempt dua kali.
//  2. Bila sesi sudah submitted, kembalikan attempt yang sudah ada
//     (idempotent: retry dari klien aman).
//  3. Panggil builder untuk menghitung skor dari jawaban di database.
//  4. Simpan attempt + kategori, lalu tandai sesi submitted.
func (r *SessionRepository) SubmitSession(ctx context.Context, sessionID string, build AttemptBuilder) (*models.Attempt, error) {
	var result *models.Attempt

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session models.TryoutSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperror.NotFound("Sesi tryout tidak ditemukan.")
			}
			return err
		}

		// Sudah dikumpulkan sebelumnya → kembalikan attempt lama.
		if session.Status == models.SessionSubmitted && session.AttemptID != nil {
			var existing models.Attempt
			if err := tx.Preload("Categories", func(db *gorm.DB) *gorm.DB {
				return db.Order("sort_order ASC")
			}).First(&existing, "id = ?", *session.AttemptID).Error; err == nil {
				result = &existing
				return nil
			}
		}

		var answers []models.TryoutAnswer
		if err := tx.Where("session_id = ?", sessionID).Find(&answers).Error; err != nil {
			return err
		}

		attempt, err := build(&session, answers)
		if err != nil {
			return err
		}

		categories := attempt.Categories
		attempt.Categories = nil
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		if len(categories) > 0 {
			for i := range categories {
				categories[i].AttemptID = attempt.ID
			}
			if err := tx.Create(&categories).Error; err != nil {
				return err
			}
		}
		attempt.Categories = categories

		now := time.Now().UTC()
		if err := tx.Model(&models.TryoutSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]any{
				"status":       models.SessionSubmitted,
				"submitted_at": now,
				"attempt_id":   attempt.ID,
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}

		result = attempt
		return nil
	})

	if err != nil {
		return nil, apperror.From(err)
	}
	return result, nil
}
