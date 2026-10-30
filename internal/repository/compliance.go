package repository

import (
	"context"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ComplianceRepository mengakses sidik jari soal dan jejak audit bank soal.
type ComplianceRepository struct {
	db *gorm.DB
}

// NewComplianceRepository membentuk repository.
func NewComplianceRepository(db *gorm.DB) *ComplianceRepository {
	return &ComplianceRepository{db: db}
}

/* ------------------------------ sidik jari ------------------------------- */

// UpsertFingerprint menyimpan sidik jari satu soal (menimpa bila sudah ada).
func (r *ComplianceRepository) UpsertFingerprint(ctx context.Context, item *models.QuestionFingerprint) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "question_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"fingerprint", "template_hash", "tokens", "computed_at"}),
	}).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan sidik jari soal.")
	}
	return nil
}

// UpsertFingerprints menyimpan banyak sidik jari sekaligus.
func (r *ComplianceRepository) UpsertFingerprints(ctx context.Context, items []models.QuestionFingerprint) error {
	if len(items) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "question_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"fingerprint", "template_hash", "tokens", "computed_at"}),
	}).CreateInBatches(items, 200).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan sidik jari soal.")
	}
	return nil
}

// DeleteFingerprint menghapus sidik jari sebuah soal.
func (r *ComplianceRepository) DeleteFingerprint(ctx context.Context, questionID string) error {
	if err := r.db.WithContext(ctx).Where("question_id = ?", questionID).
		Delete(&models.QuestionFingerprint{}).Error; err != nil {
		return apperror.Internal(err, "Gagal menghapus sidik jari soal.")
	}
	return nil
}

// ListQuestionsWithoutFingerprint mengambil soal yang belum punya sidik jari.
//
// Dipakai laporan duplikat: sebelum membandingkan, sidik jari soal yang belum
// dihitung dilengkapi lebih dulu sehingga laporan mencakup seluruh bank soal.
func (r *ComplianceRepository) ListQuestionsWithoutFingerprint(ctx context.Context, limit int) ([]models.Question, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}

	var out []models.Question
	err := r.db.WithContext(ctx).Model(&models.Question{}).
		Select("questions.*").
		Joins("LEFT JOIN question_fingerprints ON question_fingerprints.question_id = questions.id").
		Where("question_fingerprints.question_id IS NULL").
		Order("questions.updated_at DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat soal tanpa sidik jari.")
	}
	return out, nil
}

// FingerprintRow adalah sidik jari soal beserta metadata soal untuk laporan.
type FingerprintRow struct {
	QuestionID   string            `json:"questionId"`
	BankKey      string            `json:"bankKey"`
	SectorKey    string            `json:"sectorKey"`
	TestType     string            `json:"testType"`
	TryoutID     *string           `json:"tryoutId,omitempty"`
	VariantGroup *string           `json:"variantGroup,omitempty"`
	Text         string            `json:"text"`
	Fingerprint  string            `json:"fingerprint"`
	TemplateHash string            `json:"templateHash"`
	Tokens       models.StringList `json:"tokens"`
}

// FingerprintFilter adalah filter laporan duplikat.
type FingerprintFilter struct {
	BankKey    string
	SectorKey  string
	TestType   string
	TryoutID   string
	ActiveOnly bool
}

// ListFingerprints mengambil sidik jari soal beserta metadatanya.
//
// Hasilnya dikelompokkan per kategori bank di service, sehingga perbandingan
// kemiripan hanya dilakukan antar soal sekategori (jauh lebih murah daripada
// membandingkan seluruh bank soal satu sama lain).
func (r *ComplianceRepository) ListFingerprints(ctx context.Context, filter FingerprintFilter, limit int) ([]FingerprintRow, error) {
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}

	query := r.db.WithContext(ctx).Table("question_fingerprints").
		Select(`questions.id AS question_id, questions.bank_key, questions.sector_key,
		        questions.test_type, questions.tryout_id, questions.variant_group,
		        questions.text, question_fingerprints.fingerprint,
		        question_fingerprints.template_hash, question_fingerprints.tokens`).
		Joins("JOIN questions ON questions.id = question_fingerprints.question_id")

	if filter.BankKey != "" {
		query = query.Where("questions.bank_key = ?", filter.BankKey)
	}
	if filter.SectorKey != "" {
		query = query.Where("questions.sector_key = ?", filter.SectorKey)
	}
	if filter.TestType != "" {
		query = query.Where("questions.test_type = ?", filter.TestType)
	}
	if filter.TryoutID != "" {
		query = query.Where("questions.tryout_id = ?", filter.TryoutID)
	}
	if filter.ActiveOnly {
		query = query.Where("questions.active = ?", true)
	}

	var out []FingerprintRow
	err := query.Order("questions.bank_key ASC, questions.id ASC").Limit(limit).Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat sidik jari soal.")
	}
	return out, nil
}

/* ------------------------------ jejak audit ------------------------------ */

// LogAccess menyimpan satu jejak akses/perubahan soal.
//
// Kegagalan pencatatan tidak boleh menggagalkan aksi utama admin, sehingga
// pemanggil cukup mencatat error-nya ke log server.
func (r *ComplianceRepository) LogAccess(ctx context.Context, entry *models.QuestionAccessLog) error {
	if entry.ID == "" {
		entry.ID = models.NewID()
	}
	if err := r.db.WithContext(ctx).Create(entry).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat jejak akses soal.")
	}
	return nil
}

// AccessLogFilter adalah filter jejak akses soal.
type AccessLogFilter struct {
	QuestionID string
	ActorID    string
	Action     string
}

// ListAccessLogs mengambil jejak akses soal terbaru.
func (r *ComplianceRepository) ListAccessLogs(ctx context.Context, filter AccessLogFilter, limit int) ([]models.QuestionAccessLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := r.db.WithContext(ctx).Model(&models.QuestionAccessLog{})
	if filter.QuestionID != "" {
		query = query.Where("question_id = ?", filter.QuestionID)
	}
	if filter.ActorID != "" {
		query = query.Where("actor_id = ?", filter.ActorID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}

	var out []models.QuestionAccessLog
	if err := query.Order("created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jejak akses soal.")
	}
	return out, nil
}

// CountAccessLogsSince menghitung jejak akses sejak waktu tertentu.
func (r *ComplianceRepository) CountAccessLogsSince(ctx context.Context, since time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.QuestionAccessLog{}).
		Where("created_at >= ?", since).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung jejak akses soal.")
	}
	return total, nil
}
