package repository

import (
	"context"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnrollmentRepository mengakses tabel enrollments.
type EnrollmentRepository struct {
	db *gorm.DB
}

// NewEnrollmentRepository membentuk repository.
func NewEnrollmentRepository(db *gorm.DB) *EnrollmentRepository {
	return &EnrollmentRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *EnrollmentRepository) DB() *gorm.DB { return r.db }

// Grant memberikan akses produk ke peserta.
//
// Memakai INSERT ... ON CONFLICT DO NOTHING pada unique index
// (user_id, product_slug) sehingga callback pembayaran boleh dipanggil berulang
// tanpa menghasilkan baris ganda. Nilai balik `created` menandakan apakah baris
// baru benar-benar dibuat (dipakai untuk notifikasi sekali saja).
func (r *EnrollmentRepository) Grant(ctx context.Context, userID, productSlug string, orderID *string, source string) (bool, error) {
	enrollment := models.Enrollment{
		UserID:      userID,
		ProductSlug: productSlug,
		OrderID:     orderID,
		Source:      source,
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "product_slug"}}, DoNothing: true}).
		Create(&enrollment)
	if result.Error != nil {
		return false, apperror.Internal(result.Error, "Gagal memberikan akses produk.")
	}
	return result.RowsAffected > 0, nil
}

// ListByUser mengambil seluruh kepemilikan produk seorang peserta.
func (r *EnrollmentRepository) ListByUser(ctx context.Context, userID string) ([]models.Enrollment, error) {
	var out []models.Enrollment
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar kelas peserta.")
	}
	return out, nil
}

// OwnedSlugs mengambil daftar slug produk yang dimiliki peserta.
func (r *EnrollmentRepository) OwnedSlugs(ctx context.Context, userID string) ([]string, error) {
	var slugs []string
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("user_id = ?", userID).
		Distinct().
		Pluck("product_slug", &slugs).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat akses produk peserta.")
	}
	return slugs, nil
}

// HasProduct memeriksa kepemilikan satu produk.
func (r *EnrollmentRepository) HasProduct(ctx context.Context, userID, productSlug string) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
		Where("user_id = ? AND product_slug = ?", userID, productSlug).
		Count(&total).Error
	if err != nil {
		return false, apperror.Internal(err, "Gagal memeriksa akses produk.")
	}
	return total > 0, nil
}

// SlugsByUserIDs mengambil kepemilikan produk untuk sekumpulan pengguna
// (dipakai dashboard admin agar tidak terjadi N+1).
func (r *EnrollmentRepository) SlugsByUserIDs(ctx context.Context, userIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []models.Enrollment
	err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kepemilikan produk.")
	}
	for _, row := range rows {
		out[row.UserID] = append(out[row.UserID], row.ProductSlug)
	}
	return out, nil
}

// UserIDsByAudience mengambil daftar user id berdasarkan segmen broadcast.
func (r *EnrollmentRepository) UserIDsByAudience(ctx context.Context, audience string) ([]string, error) {
	var ids []string

	switch audience {
	case "enrolled":
		err := r.db.WithContext(ctx).Model(&models.Enrollment{}).
			Distinct().Pluck("user_id", &ids).Error
		if err != nil {
			return nil, apperror.Internal(err, "Gagal memuat penerima broadcast.")
		}
	case "paid":
		err := r.db.WithContext(ctx).Model(&models.Order{}).
			Where("status = ?", models.OrderPaid).
			Distinct().Pluck("user_id", &ids).Error
		if err != nil {
			return nil, apperror.Internal(err, "Gagal memuat penerima broadcast.")
		}
	case "pending":
		err := r.db.WithContext(ctx).Model(&models.Order{}).
			Where("status = ?", models.OrderPending).
			Distinct().Pluck("user_id", &ids).Error
		if err != nil {
			return nil, apperror.Internal(err, "Gagal memuat penerima broadcast.")
		}
	default: // all
		err := r.db.WithContext(ctx).Model(&models.User{}).
			Where("role = ?", models.RolePeserta).
			Pluck("id", &ids).Error
		if err != nil {
			return nil, apperror.Internal(err, "Gagal memuat penerima broadcast.")
		}
	}

	return ids, nil
}
