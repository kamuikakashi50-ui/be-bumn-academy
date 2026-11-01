package repository

import (
	"context"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TalentRepository mengakses instrumen minat–bakat, jawaban, skor, norma, profil
// lini bisnis, skor kecocokan, dan rekaman persetujuan (Fase 3 langkah 3–4).
type TalentRepository struct {
	db *gorm.DB
}

// NewTalentRepository membentuk repository.
func NewTalentRepository(db *gorm.DB) *TalentRepository {
	return &TalentRepository{db: db}
}

// DB mengembalikan handle GORM (dipakai uji query dry-run).
func (r *TalentRepository) DB() *gorm.DB { return r.db }

/* ------------------------------ instrumen --------------------------------- */

// ListInstruments mengambil daftar instrumen (opsional hanya yang aktif).
func (r *TalentRepository) ListInstruments(ctx context.Context, activeOnly bool) ([]models.TalentInstrument, error) {
	var out []models.TalentInstrument
	query := r.db.WithContext(ctx).Model(&models.TalentInstrument{}).Order("kind ASC, code ASC")
	if activeOnly {
		query = query.Where("active = ?", true)
	}
	if err := query.Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat instrumen minat–bakat.")
	}
	return out, nil
}

// FindInstrumentByCode mengambil instrumen berdasarkan kode.
func (r *TalentRepository) FindInstrumentByCode(ctx context.Context, code string) (*models.TalentInstrument, error) {
	var item models.TalentInstrument
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat instrumen.")
	}
	return &item, nil
}

// FindInstrument mengambil instrumen berdasarkan id.
func (r *TalentRepository) FindInstrument(ctx context.Context, id string) (*models.TalentInstrument, error) {
	var item models.TalentInstrument
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat instrumen.")
	}
	return &item, nil
}

// UpsertInstrument menyimpan (tambah/ubah) satu instrumen.
func (r *TalentRepository) UpsertInstrument(ctx context.Context, item *models.TalentInstrument) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"code", "name", "kind", "version", "instrument_id", "norm_version", "description", "active",
		}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan instrumen.")
	}
	return nil
}

// ListItems mengambil butir satu instrumen sesuai urutan tampil.
func (r *TalentRepository) ListItems(ctx context.Context, instrumentID string) ([]models.TalentItem, error) {
	var out []models.TalentItem
	err := r.db.WithContext(ctx).Model(&models.TalentItem{}).
		Where("instrument_id = ?", instrumentID).
		Order("sort_order ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat butir instrumen.")
	}
	return out, nil
}

// ListItemsByInstruments mengambil butir untuk sekumpulan instrumen sekaligus.
func (r *TalentRepository) ListItemsByInstruments(ctx context.Context, instrumentIDs []string) ([]models.TalentItem, error) {
	if len(instrumentIDs) == 0 {
		return nil, nil
	}
	var out []models.TalentItem
	err := r.db.WithContext(ctx).Model(&models.TalentItem{}).
		Where("instrument_id IN ?", instrumentIDs).
		Order("instrument_id ASC, sort_order ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat butir instrumen.")
	}
	return out, nil
}

// UpsertItem menyimpan (tambah/ubah) satu butir instrumen.
func (r *TalentRepository) UpsertItem(ctx context.Context, item *models.TalentItem) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"instrument_id", "text", "dimension", "weight", "reverse", "sort_order",
		}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan butir instrumen.")
	}
	return nil
}

/* -------------------------------- norma ----------------------------------- */

// ListNorms mengambil parameter norma satu versi.
func (r *TalentRepository) ListNorms(ctx context.Context, normVersion string) ([]models.TalentNorm, error) {
	var out []models.TalentNorm
	err := r.db.WithContext(ctx).Model(&models.TalentNorm{}).
		Where("norm_version = ?", normVersion).Order("dimension ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat norma instrumen.")
	}
	return out, nil
}

// UpsertNorm menyimpan parameter norma satu dimensi.
func (r *TalentRepository) UpsertNorm(ctx context.Context, item *models.TalentNorm) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "norm_version"}, {Name: "dimension"}},
		DoUpdates: clause.AssignmentColumns([]string{"mean", "sd"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan norma instrumen.")
	}
	return nil
}

// NormVersionActive mengembalikan versi norma yang dipakai instrumen aktif.
func (r *TalentRepository) NormVersionActive(ctx context.Context) (string, error) {
	versions, err := r.ListInstruments(ctx, true)
	if err != nil {
		return "", err
	}
	for _, item := range versions {
		if strings.TrimSpace(item.NormVersion) != "" {
			return item.NormVersion, nil
		}
	}
	return "", nil
}

/* ------------------------------ asesmen ----------------------------------- */

// CreateAssessment membuat sesi asesmen baru.
func (r *TalentRepository) CreateAssessment(ctx context.Context, item *models.TalentAssessment) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal memulai asesmen.")
	}
	return nil
}

// FindAssessment mengambil satu asesmen.
func (r *TalentRepository) FindAssessment(ctx context.Context, id string) (*models.TalentAssessment, error) {
	var item models.TalentAssessment
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat asesmen.")
	}
	return &item, nil
}

// FindRunningAssessment mencari asesmen yang belum dikumpulkan untuk satu instrumen.
func (r *TalentRepository) FindRunningAssessment(ctx context.Context, userID, instrumentID string) (*models.TalentAssessment, error) {
	var item models.TalentAssessment
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND instrument_id = ? AND status = ?", userID, instrumentID, models.TalentAssessmentRunning).
		Order("started_at DESC").First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat asesmen berjalan.")
	}
	return &item, nil
}

// CompleteAssessment menandai asesmen selesai.
func (r *TalentRepository) CompleteAssessment(ctx context.Context, id string, finishedAt time.Time) error {
	err := r.db.WithContext(ctx).Model(&models.TalentAssessment{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": models.TalentAssessmentCompleted, "finished_at": finishedAt}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyelesaikan asesmen.")
	}
	return nil
}

// ListAssessmentsByUser mengambil riwayat asesmen seorang peserta.
func (r *TalentRepository) ListAssessmentsByUser(ctx context.Context, userID string) ([]models.TalentAssessment, error) {
	var out []models.TalentAssessment
	err := r.db.WithContext(ctx).Model(&models.TalentAssessment{}).
		Where("user_id = ?", userID).Order("started_at DESC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat riwayat asesmen.")
	}
	return out, nil
}

/* ------------------------------- jawaban ---------------------------------- */

// UpsertAnswers menyimpan jawaban bertahap (autosave).
func (r *TalentRepository) UpsertAnswers(ctx context.Context, answers []models.TalentAnswer) error {
	if len(answers) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "assessment_id"}, {Name: "item_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&answers).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan jawaban asesmen.")
	}
	return nil
}

// ListAnswers mengambil seluruh jawaban satu asesmen.
func (r *TalentRepository) ListAnswers(ctx context.Context, assessmentID string) ([]models.TalentAnswer, error) {
	var out []models.TalentAnswer
	err := r.db.WithContext(ctx).Model(&models.TalentAnswer{}).
		Where("assessment_id = ?", assessmentID).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jawaban asesmen.")
	}
	return out, nil
}

/* -------------------------------- skor ------------------------------------ */

// UpsertScores menyimpan skor dimensi peserta (per versi norma).
func (r *TalentRepository) UpsertScores(ctx context.Context, scores []models.TalentScore) error {
	if len(scores) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, {Name: "dimension"}, {Name: "norm_version"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"raw", "percentile", "assessment_id", "computed_at"}),
	}).Create(&scores).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan skor instrumen.")
	}
	return nil
}

// ListScores mengambil skor dimensi seorang peserta.
func (r *TalentRepository) ListScores(ctx context.Context, userID string) ([]models.TalentScore, error) {
	var out []models.TalentScore
	err := r.db.WithContext(ctx).Model(&models.TalentScore{}).
		Where("user_id = ?", userID).Order("dimension ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat skor instrumen.")
	}
	return out, nil
}

// ListScoresByUsers mengambil skor dimensi sekumpulan peserta.
func (r *TalentRepository) ListScoresByUsers(ctx context.Context, userIDs []string) ([]models.TalentScore, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var out []models.TalentScore
	err := r.db.WithContext(ctx).Model(&models.TalentScore{}).
		Where("user_id IN ?", userIDs).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat skor instrumen.")
	}
	return out, nil
}

// ListAllScores mengambil seluruh skor dimensi (dipakai menghitung ulang skor
// kecocokan saat bobot lini bisnis berubah).
func (r *TalentRepository) ListAllScores(ctx context.Context) ([]models.TalentScore, error) {
	var out []models.TalentScore
	err := r.db.WithContext(ctx).Model(&models.TalentScore{}).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat skor instrumen.")
	}
	return out, nil
}

/* ----------------------------- lini bisnis -------------------------------- */

// ListBusinessLines mengambil profil lini bisnis (opsional hanya yang aktif).
func (r *TalentRepository) ListBusinessLines(ctx context.Context, activeOnly bool) ([]models.BusinessLineProfile, error) {
	var out []models.BusinessLineProfile
	query := r.db.WithContext(ctx).Model(&models.BusinessLineProfile{}).Order("sector_key ASC, name ASC")
	if activeOnly {
		query = query.Where("active = ?", true)
	}
	if err := query.Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat profil lini bisnis.")
	}
	return out, nil
}

// FindBusinessLine mengambil satu profil lini bisnis.
func (r *TalentRepository) FindBusinessLine(ctx context.Context, id string) (*models.BusinessLineProfile, error) {
	var item models.BusinessLineProfile
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat profil lini bisnis.")
	}
	return &item, nil
}

// UpsertBusinessLine menyimpan (tambah/ubah) satu profil lini bisnis.
func (r *TalentRepository) UpsertBusinessLine(ctx context.Context, item *models.BusinessLineProfile) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"sector_key", "name", "weights", "description", "org_id", "active"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan profil lini bisnis.")
	}
	return nil
}

// UpdateBusinessLineFields mengubah sebagian kolom profil lini bisnis.
func (r *TalentRepository) UpdateBusinessLineFields(ctx context.Context, id string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Model(&models.BusinessLineProfile{}).
		Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return apperror.Internal(err, "Gagal mengubah profil lini bisnis.")
	}
	return nil
}

/* ---------------------------- skor kecocokan ------------------------------ */

// UpsertFitScores menyimpan skor kecocokan peserta terhadap lini bisnis.
func (r *TalentRepository) UpsertFitScores(ctx context.Context, scores []models.TalentFitScore) error {
	if len(scores) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "business_line_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"score", "computed_at"}),
	}).Create(&scores).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan skor kecocokan.")
	}
	return nil
}

// ListFitScores mengambil skor kecocokan seorang peserta (tertinggi lebih dulu).
func (r *TalentRepository) ListFitScores(ctx context.Context, userID string) ([]models.TalentFitScore, error) {
	var out []models.TalentFitScore
	err := r.db.WithContext(ctx).Model(&models.TalentFitScore{}).
		Where("user_id = ?", userID).Order("score DESC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat skor kecocokan.")
	}
	return out, nil
}

// ListFitScoresByUsers mengambil skor kecocokan sekumpulan peserta.
func (r *TalentRepository) ListFitScoresByUsers(ctx context.Context, userIDs []string) ([]models.TalentFitScore, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var out []models.TalentFitScore
	err := r.db.WithContext(ctx).Model(&models.TalentFitScore{}).
		Where("user_id IN ?", userIDs).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat skor kecocokan.")
	}
	return out, nil
}

/* ------------------------------- consent ---------------------------------- */

// CreateConsent mencatat persetujuan peserta.
func (r *TalentRepository) CreateConsent(ctx context.Context, item *models.ConsentRecord) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat persetujuan.")
	}
	return nil
}

// LatestConsent mengambil persetujuan terakhir yang masih berlaku untuk satu tujuan.
func (r *TalentRepository) LatestConsent(ctx context.Context, userID, purpose string) (*models.ConsentRecord, error) {
	var item models.ConsentRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND purpose = ? AND revoked_at IS NULL", userID, purpose).
		Order("granted_at DESC").First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat persetujuan.")
	}
	return &item, nil
}
