package repository

import (
	"context"
	"errors"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
)

// CatalogRepository mengakses konten katalog publik.
type CatalogRepository struct {
	db *gorm.DB
}

// NewCatalogRepository membentuk repository.
func NewCatalogRepository(db *gorm.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *CatalogRepository) DB() *gorm.DB { return r.db }

/* -------------------------------- program -------------------------------- */

// ListPrograms mengambil seluruh program terurut.
func (r *CatalogRepository) ListPrograms(ctx context.Context) ([]models.Program, error) {
	var out []models.Program
	if err := r.db.WithContext(ctx).Order("sort_order ASC, name ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar program.")
	}
	return out, nil
}

// UpsertProgram menyisipkan/memperbarui program berdasarkan slug.
func (r *CatalogRepository) UpsertProgram(ctx context.Context, program *models.Program) error {
	err := r.db.WithContext(ctx).
		Where("slug = ?", program.Slug).
		Assign(map[string]any{
			"name":        program.Name,
			"tagline":     program.Tagline,
			"audience":    program.Audience,
			"format":      program.Format,
			"duration":    program.Duration,
			"features":    program.Features,
			"highlighted": program.Highlighted,
			"sort_order":  program.SortOrder,
		}).
		FirstOrCreate(program).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan program.")
	}
	return nil
}

/* --------------------------------- materi -------------------------------- */

// ListMateriCategories mengambil seluruh kategori materi.
func (r *CatalogRepository) ListMateriCategories(ctx context.Context) ([]models.MateriCategory, error) {
	var out []models.MateriCategory
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kategori materi.")
	}
	return out, nil
}

// UpsertMateriCategory menyisipkan/memperbarui kategori materi.
func (r *CatalogRepository) UpsertMateriCategory(ctx context.Context, category *models.MateriCategory) error {
	err := r.db.WithContext(ctx).
		Where("id = ?", category.ID).
		Assign(map[string]any{
			"label":       category.Label,
			"description": category.Description,
			"topics":      category.Topics,
			"sort_order":  category.SortOrder,
		}).
		FirstOrCreate(category).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan kategori materi.")
	}
	return nil
}

/* -------------------------------- produk --------------------------------- */

// ListProducts mengambil produk aktif (atau semua bila includeInactive).
func (r *CatalogRepository) ListProducts(ctx context.Context, includeInactive bool) ([]models.Product, error) {
	query := r.db.WithContext(ctx).Model(&models.Product{})
	if !includeInactive {
		query = query.Where("active = ?", true)
	}
	var out []models.Product
	if err := query.Order("sort_order ASC, price ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar produk.")
	}
	return out, nil
}

// FindProductBySlug mencari produk berdasarkan slug.
func (r *CatalogRepository) FindProductBySlug(ctx context.Context, slug string) (*models.Product, error) {
	var product models.Product
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Produk tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat produk.")
	}
	return &product, nil
}

// CreateProduct menyimpan produk baru.
func (r *CatalogRepository) CreateProduct(ctx context.Context, product *models.Product) error {
	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan produk.")
	}
	return nil
}

// UpdateProduct menyimpan perubahan produk.
func (r *CatalogRepository) UpdateProduct(ctx context.Context, product *models.Product) error {
	if err := r.db.WithContext(ctx).Save(product).Error; err != nil {
		return apperror.Internal(err, "Gagal memperbarui produk.")
	}
	return nil
}

// DeleteProduct menghapus produk berdasarkan slug.
func (r *CatalogRepository) DeleteProduct(ctx context.Context, slug string) error {
	result := r.db.WithContext(ctx).Where("slug = ?", slug).Delete(&models.Product{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus produk.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Produk tidak ditemukan.")
	}
	return nil
}

/* ---------------------------- metode pembayaran --------------------------- */

// ListPaymentMethods mengambil metode pembayaran (aktif saja bila includeInactive=false).
func (r *CatalogRepository) ListPaymentMethods(ctx context.Context, includeInactive bool) ([]models.PaymentMethod, error) {
	query := r.db.WithContext(ctx).Model(&models.PaymentMethod{})
	if !includeInactive {
		query = query.Where("active = ?", true)
	}
	var out []models.PaymentMethod
	if err := query.Order("sort_order ASC, id ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat metode pembayaran.")
	}
	return out, nil
}

// FindPaymentMethod mencari metode pembayaran aktif berdasarkan id.
func (r *CatalogRepository) FindPaymentMethod(ctx context.Context, id string) (*models.PaymentMethod, error) {
	var method models.PaymentMethod
	err := r.db.WithContext(ctx).Where("id = ? AND active = ?", id, true).First(&method).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.BadRequest("Metode pembayaran tidak dikenali.")
		}
		return nil, apperror.Internal(err, "Gagal memuat metode pembayaran.")
	}
	return &method, nil
}

// UpsertPaymentMethod menyisipkan/memperbarui metode pembayaran.
func (r *CatalogRepository) UpsertPaymentMethod(ctx context.Context, method *models.PaymentMethod) error {
	err := r.db.WithContext(ctx).
		Where("id = ?", method.ID).
		Assign(map[string]any{
			"label":      method.Label,
			"channel":    method.Channel,
			"fee":        method.Fee,
			"active":     method.Active,
			"sort_order": method.SortOrder,
		}).
		FirstOrCreate(method).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan metode pembayaran.")
	}
	return nil
}

/* -------------------------------- voucher -------------------------------- */

// ListVouchers mengambil seluruh voucher (aktif saja bila includeInactive=false).
func (r *CatalogRepository) ListVouchers(ctx context.Context, includeInactive bool) ([]models.Voucher, error) {
	query := r.db.WithContext(ctx).Model(&models.Voucher{})
	if !includeInactive {
		query = query.Where("active = ?", true)
	}
	var out []models.Voucher
	if err := query.Order("code ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat voucher.")
	}
	return out, nil
}

// FindVoucher mencari voucher aktif berdasarkan kode (case-insensitive).
func (r *CatalogRepository) FindVoucher(ctx context.Context, code string) (*models.Voucher, error) {
	var voucher models.Voucher
	err := r.db.WithContext(ctx).
		Where("upper(code) = upper(?) AND active = ?", code, true).
		First(&voucher).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Kode voucher tidak dikenali.")
		}
		return nil, apperror.Internal(err, "Gagal memeriksa voucher.")
	}
	return &voucher, nil
}

// UpsertVoucher menyisipkan/memperbarui voucher.
func (r *CatalogRepository) UpsertVoucher(ctx context.Context, voucher *models.Voucher) error {
	err := r.db.WithContext(ctx).
		Where("code = ?", voucher.Code).
		Assign(map[string]any{
			"label":        voucher.Label,
			"type":         voucher.Type,
			"value":        voucher.Value,
			"min_purchase": voucher.MinPurchase,
			"max_uses":     voucher.MaxUses,
			"active":       voucher.Active,
		}).
		FirstOrCreate(voucher).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan voucher.")
	}
	return nil
}

// UpdateVoucher menyimpan perubahan voucher.
func (r *CatalogRepository) UpdateVoucher(ctx context.Context, voucher *models.Voucher) error {
	if err := r.db.WithContext(ctx).Save(voucher).Error; err != nil {
		return apperror.Internal(err, "Gagal memperbarui voucher.")
	}
	return nil
}

// DeleteVoucher menghapus voucher berdasarkan kode.
func (r *CatalogRepository) DeleteVoucher(ctx context.Context, code string) error {
	result := r.db.WithContext(ctx).Where("upper(code) = upper(?)", code).Delete(&models.Voucher{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus voucher.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Voucher tidak ditemukan.")
	}
	return nil
}

// IncrementVoucherUsage menaikkan pemakaian voucher secara atomik.
//
// UPDATE ... SET used_count = used_count + 1 dijalankan langsung di database
// sehingga tidak ada race condition saat banyak peserta checkout bersamaan.
// Bila max_uses sudah terlampaui, RowsAffected = 0.
func (r *CatalogRepository) IncrementVoucherUsage(ctx context.Context, code string) (bool, error) {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE vouchers
		   SET used_count = used_count + 1,
		       updated_at = now()
		 WHERE upper(code) = upper(?)
		   AND active = true
		   AND (max_uses IS NULL OR used_count < max_uses)`, code)
	if result.Error != nil {
		return false, apperror.Internal(result.Error, "Gagal mencatat pemakaian voucher.")
	}
	return result.RowsAffected > 0, nil
}
