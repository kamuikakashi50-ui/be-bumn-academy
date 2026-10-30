package repository

import (
	"context"
	"errors"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TryoutRepository mengakses paket tryout, subtes, dan bank soal.
type TryoutRepository struct {
	db *gorm.DB
}

// NewTryoutRepository membentuk repository.
func NewTryoutRepository(db *gorm.DB) *TryoutRepository {
	return &TryoutRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *TryoutRepository) DB() *gorm.DB { return r.db }

/* ------------------------------ paket tryout ----------------------------- */

// ListPackages mengambil paket tryout beserta subtesnya.
//
// Memakai Preload (2 query, bukan N+1) dan mengurutkan subtes agar siap
// dipakai langsung oleh handler.
func (r *TryoutRepository) ListPackages(ctx context.Context, activeOnly bool) ([]models.TryoutPackage, error) {
	query := r.db.WithContext(ctx).Model(&models.TryoutPackage{})
	if activeOnly {
		query = query.Where("active = ?", true)
	}

	var out []models.TryoutPackage
	err := query.
		Preload("Subtests", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, name ASC")
		}).
		Order("sort_order ASC, title ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar tryout.")
	}
	return out, nil
}

// FindPackage mengambil satu paket tryout beserta subtesnya.
func (r *TryoutRepository) FindPackage(ctx context.Context, id string) (*models.TryoutPackage, error) {
	var pkg models.TryoutPackage
	err := r.db.WithContext(ctx).
		Preload("Subtests", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, name ASC")
		}).
		First(&pkg, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Paket tryout tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat paket tryout.")
	}
	return &pkg, nil
}

// UpsertPackage menyisipkan/memperbarui paket tryout beserta subtesnya.
//
// Subtes lama diganti seluruhnya di dalam satu transaksi agar konsisten.
func (r *TryoutRepository) UpsertPackage(ctx context.Context, pkg *models.TryoutPackage, subtests []models.Subtest) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "description", "tier", "required_product", "attempts", "access_until", "active", "sort_order", "updated_at"}),
		}).Create(pkg).Error; err != nil {
			return err
		}

		if err := tx.Where("tryout_id = ?", pkg.ID).Delete(&models.Subtest{}).Error; err != nil {
			return err
		}
		if len(subtests) > 0 {
			for i := range subtests {
				subtests[i].TryoutID = pkg.ID
				subtests[i].SortOrder = i
				if subtests[i].ID == "" {
					subtests[i].ID = models.NewID()
				}
			}
			if err := tx.Create(&subtests).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan paket tryout.")
	}
	return nil
}

// DeletePackage menghapus paket tryout.
func (r *TryoutRepository) DeletePackage(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.TryoutPackage{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus paket tryout.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Paket tryout tidak ditemukan.")
	}
	return nil
}

// CountPackages menghitung jumlah paket tryout.
func (r *TryoutRepository) CountPackages(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.TryoutPackage{}).Count(&total).Error; err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung paket tryout.")
	}
	return total, nil
}

/* -------------------------------- soal ----------------------------------- */

// QuestionFilter adalah filter daftar soal.
type QuestionFilter struct {
	BankKey    string
	TryoutID   string
	Search     string
	Difficulty string
	ActiveOnly bool
	// Dimensi taksonomi: memfilter soal per kategori BUMN, jenis tes, dan
	// keluarga jabatan (mis. soal TKB untuk sektor jasa keuangan).
	SectorKey      string
	TestType       string
	PositionFamily string
	// VariantGroup memfilter soal yang tergabung dalam satu grup varian.
	VariantGroup string
}

// ListQuestions mengambil daftar soal dengan paginasi (khusus admin).
func (r *TryoutRepository) ListQuestions(ctx context.Context, filter QuestionFilter, page, perPage int) ([]models.Question, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Question{})
	if filter.BankKey != "" {
		query = query.Where("bank_key = ?", filter.BankKey)
	}
	if filter.TryoutID != "" {
		query = query.Where("tryout_id = ?", filter.TryoutID)
	}
	if filter.Difficulty != "" {
		query = query.Where("difficulty = ?", filter.Difficulty)
	}
	if filter.SectorKey != "" {
		query = query.Where("sector_key = ?", filter.SectorKey)
	}
	if filter.TestType != "" {
		query = query.Where("test_type = ?", filter.TestType)
	}
	if filter.PositionFamily != "" {
		query = query.Where("position_family = ?", filter.PositionFamily)
	}
	if filter.VariantGroup != "" {
		query = query.Where("variant_group = ?", filter.VariantGroup)
	}
	if filter.ActiveOnly {
		query = query.Where("active = ?", true)
	}
	if filter.Search != "" {
		query = query.Where("text ILIKE ?", "%"+filter.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err, "Gagal menghitung soal.")
	}

	var out []models.Question
	err := query.Order("bank_key ASC, position ASC, id ASC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out).Error
	if err != nil {
		return nil, 0, apperror.Internal(err, "Gagal memuat daftar soal.")
	}
	return out, total, nil
}

// FindQuestion mengambil satu soal.
func (r *TryoutRepository) FindQuestion(ctx context.Context, id string) (*models.Question, error) {
	var question models.Question
	err := r.db.WithContext(ctx).First(&question, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Soal tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat soal.")
	}
	return &question, nil
}

// ListQuestionsByIDs mengambil banyak soal sekaligus (dipakai panel izin &
// kepatuhan untuk melengkapi metadata soal tanpa query per baris).
func (r *TryoutRepository) ListQuestionsByIDs(ctx context.Context, ids []string) ([]models.Question, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var out []models.Question
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat soal.")
	}
	return out, nil
}

// QuestionsByBanks mengambil seluruh soal aktif sebuah tryout dari sekumpulan
// bank, terurut mengikuti urutan bank pada subtes.
//
// `tryoutID` wajib: satu soal dimiliki satu tryout, sehingga soal milik tryout
// lain tidak boleh ikut terambil walau kategorinya sama. Hasilnya di-cache di
// Redis oleh service karena bank soal jarang berubah sementara fungsi ini
// dipanggil setiap kali peserta memulai sesi.
func (r *TryoutRepository) QuestionsByBanks(ctx context.Context, tryoutID string, banks []string) ([]models.Question, error) {
	if tryoutID == "" || len(banks) == 0 {
		return nil, nil
	}
	var out []models.Question
	err := r.db.WithContext(ctx).
		Where("tryout_id = ? AND bank_key IN ? AND active = ?", tryoutID, banks, true).
		Order("bank_key ASC, position ASC, id ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat bank soal.")
	}
	return out, nil
}

// CountQuestionsByTryout menghitung jumlah soal aktif per (tryout, bank).
//
// Dipakai daftar & detail tryout untuk menampilkan jumlah soal tiap subtes
// tanpa query per paket. Soal yang belum ditautkan ke tryout (tryout_id NULL)
// tidak ikut dihitung karena tidak dipakai sesi mana pun.
//
// Soal dalam satu grup varian (`variant_group`) dihitung sekali: sesi tryout
// hanya memakai satu soal per grup, jadi angka pada kartu tryout tetap sama
// dengan jumlah soal yang benar-benar dikerjakan peserta.
func (r *TryoutRepository) CountQuestionsByTryout(ctx context.Context) (map[string]map[string]int, error) {
	type row struct {
		TryoutID string
		BankKey  string
		Total    int
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("(?) AS soal_unik", r.db.WithContext(ctx).
		Model(&models.Question{}).
		Select("DISTINCT ON (tryout_id, bank_key, COALESCE(variant_group, id)) tryout_id, bank_key").
		Where("active = ? AND tryout_id IS NOT NULL", true).
		Order("tryout_id, bank_key, COALESCE(variant_group, id), position, id")).
		Select("tryout_id, bank_key, count(*) AS total").
		Group("tryout_id, bank_key").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung soal per kategori.")
	}

	out := make(map[string]map[string]int, len(rows))
	for _, item := range rows {
		if out[item.TryoutID] == nil {
			out[item.TryoutID] = make(map[string]int)
		}
		out[item.TryoutID][item.BankKey] = item.Total
	}
	return out, nil
}

// CreateQuestion menyimpan soal baru.
func (r *TryoutRepository) CreateQuestion(ctx context.Context, question *models.Question) error {
	if err := r.db.WithContext(ctx).Create(question).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan soal.")
	}
	return nil
}

// UpdateQuestion menyimpan perubahan soal.
func (r *TryoutRepository) UpdateQuestion(ctx context.Context, question *models.Question) error {
	if err := r.db.WithContext(ctx).Save(question).Error; err != nil {
		return apperror.Internal(err, "Gagal memperbarui soal.")
	}
	return nil
}

// DeleteQuestion menghapus soal.
func (r *TryoutRepository) DeleteQuestion(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Question{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus soal.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Soal tidak ditemukan.")
	}
	return nil
}

// BulkUpsertQuestions menyimpan banyak soal sekaligus (dipakai import Excel).
//
// Memakai ON CONFLICT DO UPDATE dalam satu statement dengan batch 200 baris,
// jauh lebih cepat daripada menyimpan satu per satu.
func (r *TryoutRepository) BulkUpsertQuestions(ctx context.Context, questions []models.Question) error {
	if len(questions) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"bank_key", "tryout_id", "text", "options", "answer_index",
				"explanation", "difficulty", "position", "active",
				"sector_key", "company_key", "test_type", "position_family",
				"variant_group", "retention_until", "updated_at",
			}),
		}).
		CreateInBatches(questions, 200).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan batch soal.")
	}
	return nil
}

// CountQuestions menghitung seluruh soal.
func (r *TryoutRepository) CountQuestions(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.Question{}).Count(&total).Error; err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung soal.")
	}
	return total, nil
}

// NextQuestionPosition menghitung posisi berikutnya pada sebuah bank di dalam
// satu tryout (posisi bersifat lokal per tryout).
func (r *TryoutRepository) NextQuestionPosition(ctx context.Context, tryoutID, bankKey string) (int, error) {
	query := r.db.WithContext(ctx).Model(&models.Question{}).
		Where("bank_key = ?", bankKey)
	if tryoutID != "" {
		query = query.Where("tryout_id = ?", tryoutID)
	}

	var maxPos *int
	err := query.Select("MAX(position)").Scan(&maxPos).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung urutan soal.")
	}
	if maxPos == nil {
		return 1, nil
	}
	return *maxPos + 1, nil
}
