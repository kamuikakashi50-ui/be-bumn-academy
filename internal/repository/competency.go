package repository

import (
	"context"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CompetencyRepository mengakses kamus kompetensi, standar jabatan, penilaian
// peserta, dan pemetaan kategori bank soal → kompetensi (Fase 3).
type CompetencyRepository struct {
	db *gorm.DB
}

// NewCompetencyRepository membentuk repository.
func NewCompetencyRepository(db *gorm.DB) *CompetencyRepository {
	return &CompetencyRepository{db: db}
}

// DB mengembalikan handle GORM (dipakai uji query dry-run).
func (r *CompetencyRepository) DB() *gorm.DB { return r.db }

/* -------------------------------- kamus ----------------------------------- */

// ListCompetencies mengambil kamus kompetensi (opsional hanya yang aktif).
func (r *CompetencyRepository) ListCompetencies(ctx context.Context, activeOnly bool) ([]models.Competency, error) {
	var out []models.Competency
	query := r.db.WithContext(ctx).Model(&models.Competency{}).Order("cluster ASC, code ASC")
	if activeOnly {
		query = query.Where("active = ?", true)
	}
	if err := query.Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kamus kompetensi.")
	}
	return out, nil
}

// FindCompetency mengambil satu kompetensi berdasarkan kode.
func (r *CompetencyRepository) FindCompetency(ctx context.Context, code string) (*models.Competency, error) {
	var item models.Competency
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat kompetensi.")
	}
	return &item, nil
}

// UpsertCompetency menyimpan (tambah/ubah) satu kompetensi.
func (r *CompetencyRepository) UpsertCompetency(ctx context.Context, item *models.Competency) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "cluster", "definition", "active"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan kompetensi.")
	}
	return nil
}

// ListLevelsByCode mengambil deskriptor level sebuah kompetensi.
func (r *CompetencyRepository) ListLevelsByCode(ctx context.Context, code string) ([]models.CompetencyLevel, error) {
	var out []models.CompetencyLevel
	err := r.db.WithContext(ctx).Model(&models.CompetencyLevel{}).
		Where("competency_code = ?", code).Order("level ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat level kompetensi.")
	}
	return out, nil
}

// ListAllLevels mengambil seluruh deskriptor level (untuk memuat framework sekaligus).
func (r *CompetencyRepository) ListAllLevels(ctx context.Context) ([]models.CompetencyLevel, error) {
	var out []models.CompetencyLevel
	err := r.db.WithContext(ctx).Model(&models.CompetencyLevel{}).
		Order("competency_code ASC, level ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat level kompetensi.")
	}
	return out, nil
}

// UpsertLevel menyimpan deskriptor satu level kompetensi.
func (r *CompetencyRepository) UpsertLevel(ctx context.Context, item *models.CompetencyLevel) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "competency_code"}, {Name: "level"}},
		DoUpdates: clause.AssignmentColumns([]string{"descriptor"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan level kompetensi.")
	}
	return nil
}

/* ---------------------------- standar jabatan ----------------------------- */

// ListRequirements mengambil standar kompetensi satu keluarga jabatan
// (opsional disaring per klaster BUMN).
func (r *CompetencyRepository) ListRequirements(ctx context.Context, family, sector string) ([]models.PositionRequirement, error) {
	var out []models.PositionRequirement
	query := r.db.WithContext(ctx).Model(&models.PositionRequirement{}).
		Where("position_family = ?", family)
	if strings.TrimSpace(sector) != "" {
		query = query.Where("sector_key = ?", sector)
	}
	if err := query.Order("sector_key ASC, competency_code ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat standar kompetensi jabatan.")
	}
	return out, nil
}

// ListAllRequirements mengambil seluruh standar kompetensi jabatan.
func (r *CompetencyRepository) ListAllRequirements(ctx context.Context) ([]models.PositionRequirement, error) {
	var out []models.PositionRequirement
	err := r.db.WithContext(ctx).Model(&models.PositionRequirement{}).
		Order("position_family ASC, sector_key ASC, competency_code ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat standar kompetensi jabatan.")
	}
	return out, nil
}

// UpsertRequirement menyimpan satu standar kompetensi jabatan.
func (r *CompetencyRepository) UpsertRequirement(ctx context.Context, item *models.PositionRequirement) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "position_family"}, {Name: "sector_key"}, {Name: "competency_code"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"min_level", "weight"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan standar kompetensi jabatan.")
	}
	return nil
}

// DeleteRequirement menghapus satu standar kompetensi jabatan.
func (r *CompetencyRepository) DeleteRequirement(ctx context.Context, family, sector, code string) error {
	err := r.db.WithContext(ctx).Where(
		"position_family = ? AND sector_key = ? AND competency_code = ?", family, sector, code,
	).Delete(&models.PositionRequirement{}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menghapus standar kompetensi jabatan.")
	}
	return nil
}

/* ------------------------- pemetaan bank soal ----------------------------- */

// ListBankMap mengambil pemetaan kategori bank soal → kompetensi.
func (r *CompetencyRepository) ListBankMap(ctx context.Context) ([]models.BankCompetencyMap, error) {
	var out []models.BankCompetencyMap
	err := r.db.WithContext(ctx).Model(&models.BankCompetencyMap{}).
		Order("bank_key ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat pemetaan bank soal.")
	}
	return out, nil
}

// UpsertBankMap menyimpan satu pemetaan bank soal → kompetensi.
func (r *CompetencyRepository) UpsertBankMap(ctx context.Context, item *models.BankCompetencyMap) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bank_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"competency_code", "weight"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan pemetaan bank soal.")
	}
	return nil
}

/* --------------------------- penilaian peserta ---------------------------- */

// UpsertUserCompetency menyimpan penilaian satu kompetensi (per sumber).
func (r *CompetencyRepository) UpsertUserCompetency(ctx context.Context, item *models.UserCompetency) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, {Name: "competency_code"}, {Name: "source"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"level", "evidence", "assessed_by", "assessed_at"}),
	}).Create(item).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan penilaian kompetensi.")
	}
	return nil
}

// ListUserCompetencies mengambil seluruh penilaian kompetensi seorang peserta.
func (r *CompetencyRepository) ListUserCompetencies(ctx context.Context, userID string) ([]models.UserCompetency, error) {
	var out []models.UserCompetency
	err := r.db.WithContext(ctx).Model(&models.UserCompetency{}).
		Where("user_id = ?", userID).
		Order("competency_code ASC, source ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kompetensi peserta.")
	}
	return out, nil
}

// ListUserCompetenciesByUsers mengambil penilaian untuk sekumpulan peserta.
func (r *CompetencyRepository) ListUserCompetenciesByUsers(ctx context.Context, userIDs []string) ([]models.UserCompetency, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var out []models.UserCompetency
	err := r.db.WithContext(ctx).Model(&models.UserCompetency{}).
		Where("user_id IN ?", userIDs).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kompetensi peserta.")
	}
	return out, nil
}

// DeleteUserCompetenciesBySource menghapus penilaian satu sumber (mis. menyegarkan
// proksi tryout sebelum menghitung ulang).
func (r *CompetencyRepository) DeleteUserCompetenciesBySource(ctx context.Context, userID, source string) error {
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND source = ?", userID, source).
		Delete(&models.UserCompetency{}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyegarkan penilaian kompetensi.")
	}
	return nil
}

/* ------------------------------ data pendukung ---------------------------- */

// UserSkillScoreRow adalah rata-rata skor satu kategori soal seorang peserta.
type UserSkillScoreRow struct {
	UserID       string  `gorm:"column:user_id"`
	Name         string  `gorm:"column:name"`
	AverageScore float64 `gorm:"column:average_score"`
	Attempts     int64   `gorm:"column:attempts"`
}

// UserSkillScores menghitung rata-rata skor per kategori soal seorang peserta
// (sumber proksi kompetensi dari hasil tryout).
func (r *CompetencyRepository) UserSkillScores(ctx context.Context, userID string) ([]UserSkillScoreRow, error) {
	return r.UserSkillScoresForUsers(ctx, []string{userID})
}

// UserSkillScoresForUsers menghitung rata-rata skor per kategori soal untuk
// sekumpulan peserta sekaligus (dipakai matriks kompetensi agar tidak N+1).
func (r *CompetencyRepository) UserSkillScoresForUsers(ctx context.Context, userIDs []string) ([]UserSkillScoreRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var out []UserSkillScoreRow
	err := r.db.WithContext(ctx).
		Table("attempt_categories AS ac").
		Select("a.user_id AS user_id, ac.name AS name, COALESCE(AVG(ac.score), 0) AS average_score, COUNT(*) AS attempts").
		Joins("JOIN attempts a ON a.id = ac.attempt_id").
		Where("a.user_id IN ?", userIDs).
		Group("a.user_id, ac.name").
		Scan(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung skor kategori peserta.")
	}
	return out, nil
}

// MatrixUserRow adalah satu peserta pada matriks kompetensi.
type MatrixUserRow struct {
	ID      string `gorm:"column:id"`
	Name    string `gorm:"column:name"`
	Email   string `gorm:"column:email"`
	Program string `gorm:"column:program"`
}

// MatrixFilter menyaring daftar peserta matriks kompetensi.
type MatrixFilter struct {
	// UserIDs membatasi ke anggota sebuah kelas khusus (opsional).
	UserIDs []string
	// UserIDsSet menandai bahwa `UserIDs` memang dipakai sebagai penyaring —
	// sehingga daftar kosong berarti "tidak ada peserta", bukan "semua peserta".
	UserIDsSet bool
	Program    string
	Search     string
	Limit      int
}

// ListPesertaForMatrix mengambil daftar peserta untuk matriks kompetensi.
func (r *CompetencyRepository) ListPesertaForMatrix(ctx context.Context, filter MatrixFilter) ([]MatrixUserRow, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := r.db.WithContext(ctx).Table("users").
		Select("id, name, email, program").
		Where("role = ?", models.RolePeserta)
	if filter.UserIDsSet {
		if len(filter.UserIDs) == 0 {
			return nil, nil
		}
		query = query.Where("id IN ?", filter.UserIDs)
	}
	if strings.TrimSpace(filter.Program) != "" {
		query = query.Where("program = ?", filter.Program)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ?", like, like)
	}

	var out []MatrixUserRow
	if err := query.Order("name ASC").Limit(limit).Scan(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat peserta matriks kompetensi.")
	}
	return out, nil
}
