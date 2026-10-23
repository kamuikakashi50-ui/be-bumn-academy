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

// AIRepository mengakses analisis, percakapan, pesan, dan kuota mentor AI.
type AIRepository struct {
	db *gorm.DB
}

// NewAIRepository membentuk repository mentor AI.
func NewAIRepository(db *gorm.DB) *AIRepository {
	return &AIRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *AIRepository) DB() *gorm.DB { return r.db }

/* --------------------------------- analisis -------------------------------- */

// FindAnalysis mengambil analisis sebuah attempt (nil bila belum ada).
func (r *AIRepository) FindAnalysis(ctx context.Context, attemptID string) (*models.AttemptAnalysis, error) {
	var analysis models.AttemptAnalysis
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&analysis).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat analisis hasil tryout.")
	}
	return &analysis, nil
}

// SaveAnalysis menyimpan (upsert) analisis sebuah attempt.
//
// Unique index pada attempt_id membuat operasi ini aman dipanggil berulang,
// termasuk saat dua permintaan "buat analisis" datang bersamaan.
func (r *AIRepository) SaveAnalysis(ctx context.Context, analysis *models.AttemptAnalysis) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "attempt_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider", "model", "headline", "summary", "readiness", "readiness_score",
			"strengths", "weaknesses", "focus_areas", "study_plan", "question_notes",
			"generated_at", "updated_at",
		}),
	}).Create(analysis).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan analisis hasil tryout.")
	}
	return nil
}

// DeleteAnalysis menghapus analisis sebuah attempt.
func (r *AIRepository) DeleteAnalysis(ctx context.Context, attemptID string) error {
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).Delete(&models.AttemptAnalysis{}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menghapus analisis hasil tryout.")
	}
	return nil
}

/* -------------------------------- percakapan ------------------------------- */

// EnsureConversation mengambil percakapan sebuah attempt, membuatnya bila belum ada.
func (r *AIRepository) EnsureConversation(
	ctx context.Context,
	attemptID, userID, title string,
) (*models.AIConversation, error) {
	var conversation models.AIConversation
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&conversation).Error
	if err == nil {
		return &conversation, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.Internal(err, "Gagal memuat percakapan mentor.")
	}

	candidate := models.AIConversation{AttemptID: attemptID, UserID: userID, Title: title}
	// OnConflict menjaga dua permintaan pertama yang datang bersamaan tidak
	// menghasilkan dua percakapan untuk attempt yang sama.
	createErr := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "attempt_id"}}, DoNothing: true}).
		Create(&candidate).Error
	if createErr != nil {
		return nil, apperror.Internal(createErr, "Gagal membuat percakapan mentor.")
	}

	if err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&conversation).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat percakapan mentor.")
	}
	return &conversation, nil
}

// FindConversation mengambil percakapan sebuah attempt (nil bila belum ada).
func (r *AIRepository) FindConversation(ctx context.Context, attemptID string) (*models.AIConversation, error) {
	var conversation models.AIConversation
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&conversation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat percakapan mentor.")
	}
	return &conversation, nil
}

// AppendMessages menyimpan pesan baru dan memperbarui ringkasan percakapan.
//
// Menerima slice pointer agar hook `BeforeCreate` (pengisi ID) dan
// `autoCreateTime` GORM menulis ke struct milik pemanggil, sehingga ID serta
// waktu pesan bisa langsung dipakai untuk balasan API.
func (r *AIRepository) AppendMessages(ctx context.Context, conversationID string, messages []*models.AIMessage) error {
	if len(messages) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&messages).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan pesan percakapan.")
	}

	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Model(&models.AIConversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]any{
			"message_count":   gorm.Expr("message_count + ?", len(messages)),
			"last_message_at": now,
			"updated_at":      now,
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal memperbarui percakapan mentor.")
	}
	return nil
}

// ListMessages mengambil pesan percakapan (terlama → terbaru).
func (r *AIRepository) ListMessages(ctx context.Context, conversationID string, limit int) ([]models.AIMessage, error) {
	query := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID)

	if limit > 0 {
		// Ambil yang terbaru dulu agar pesan lama yang terpotong, bukan pesan baru.
		var recent []models.AIMessage
		if err := query.Order("created_at DESC").Limit(limit).Find(&recent).Error; err != nil {
			return nil, apperror.Internal(err, "Gagal memuat pesan percakapan.")
		}
		for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
			recent[i], recent[j] = recent[j], recent[i]
		}
		return recent, nil
	}

	var messages []models.AIMessage
	if err := query.Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat pesan percakapan.")
	}
	return messages, nil
}

// ResetConversation menghapus seluruh pesan pada sebuah percakapan.
func (r *AIRepository) ResetConversation(ctx context.Context, conversationID string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&models.AIMessage{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.AIConversation{}).
			Where("id = ?", conversationID).
			Updates(map[string]any{
				"message_count":   0,
				"last_message_at": nil,
				"updated_at":      time.Now().UTC(),
			}).Error
	})
	if err != nil {
		return apperror.Internal(err, "Gagal mengosongkan percakapan mentor.")
	}
	return nil
}

/* ---------------------------------- kuota ---------------------------------- */

// GetUsage mengambil pemakaian AI peserta pada satu tanggal.
func (r *AIRepository) GetUsage(ctx context.Context, userID string, date time.Time) (*models.AIUsageCounter, error) {
	var counter models.AIUsageCounter
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND usage_date = ?", userID, date.Format("2006-01-02")).
		First(&counter).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AIUsageCounter{UserID: userID, UsageDate: date}, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat kuota mentor AI.")
	}
	return &counter, nil
}

// IncrementUsage menaikkan pemakaian AI peserta secara atomik.
//
// Satu statement UPSERT membuat penghitungan tetap benar walau beberapa
// permintaan AI datang bersamaan.
func (r *AIRepository) IncrementUsage(
	ctx context.Context,
	userID string,
	date time.Time,
	promptTokens, completionTokens int,
) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO ai_usage_counters (user_id, usage_date, requests, prompt_tokens, completion_tokens, updated_at)
		VALUES (?, ?, 1, ?, ?, now())
		ON CONFLICT (user_id, usage_date) DO UPDATE
		   SET requests          = ai_usage_counters.requests + 1,
		       prompt_tokens     = ai_usage_counters.prompt_tokens + EXCLUDED.prompt_tokens,
		       completion_tokens = ai_usage_counters.completion_tokens + EXCLUDED.completion_tokens,
		       updated_at        = now()`,
		userID, date.Format("2006-01-02"), promptTokens, completionTokens,
	).Error
	if err != nil {
		return apperror.Internal(err, "Gagal mencatat pemakaian mentor AI.")
	}
	return nil
}
