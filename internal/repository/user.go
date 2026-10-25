// Package repository mengisolasi seluruh akses database.
//
// Setiap method menerima context sehingga query bisa dibatalkan tepat waktu
// (penting saat beban tinggi: request yang sudah ditinggalkan klien tidak lagi
// menghabiskan koneksi pool).
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
)

// UserRepository mengakses tabel users dan refresh_tokens.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository membentuk repository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// DB mengembalikan handle GORM (dipakai service untuk transaksi).
func (r *UserRepository) DB() *gorm.DB { return r.db }

// FindByEmail mencari pengguna berdasarkan email (case-insensitive).
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("lower(email) = lower(?)", email).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Akun dengan email tersebut tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat akun.")
	}
	return &user, nil
}

// FindByID mencari pengguna berdasarkan id.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Akun tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat akun.")
	}
	return &user, nil
}

// FindByReferralCode mencari pengguna pemilik kode referral.
func (r *UserRepository) FindByReferralCode(ctx context.Context, code string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("referral_code IS NOT NULL AND upper(referral_code) = upper(?)", code).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Kode referral tidak dikenali.")
		}
		return nil, apperror.Internal(err, "Gagal memeriksa kode referral.")
	}
	return &user, nil
}

// Create menyimpan pengguna baru.
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan akun.")
	}
	return nil
}

// Update menyimpan perubahan pengguna.
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		return apperror.Internal(err, "Gagal memperbarui akun.")
	}
	return nil
}

// UserFilter adalah filter daftar pengguna untuk admin.
type UserFilter struct {
	Search string
	Role   string
}

// List mengambil daftar pengguna dengan paginasi.
func (r *UserRepository) List(ctx context.Context, filter UserFilter, page, perPage int) ([]models.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.User{})
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err, "Gagal menghitung jumlah pengguna.")
	}

	var users []models.User
	err := query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&users).Error
	if err != nil {
		return nil, 0, apperror.Internal(err, "Gagal memuat daftar pengguna.")
	}
	return users, total, nil
}

// CountByRole menghitung jumlah pengguna dengan peran tertentu ("" = semua).
func (r *UserRepository) CountByRole(ctx context.Context, role models.Role) (int64, error) {
	query := r.db.WithContext(ctx).Model(&models.User{})
	if role != "" {
		query = query.Where("role = ?", role)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung pengguna.")
	}
	return total, nil
}

// CountAll menghitung seluruh pengguna.
func (r *UserRepository) CountAll(ctx context.Context) (int64, error) {
	return r.CountByRole(ctx, "")
}

// ProgramBreakdownRow adalah partisipasi & capaian peserta pada satu program.
type ProgramBreakdownRow struct {
	Program      string  `gorm:"column:program"`
	Peserta      int64   `gorm:"column:peserta"`
	Attempts     int64   `gorm:"column:attempts"`
	AverageScore float64 `gorm:"column:average_score"`
}

// ProgramBreakdown mengagregasi jumlah peserta, percobaan tryout, dan rata-rata
// skor per program/segmen peserta (kolom `users.program`).
//
// Dipakai analitik HR untuk melihat sebaran kohort antar program tanpa membuka
// identitas peserta satu per satu.
func (r *UserRepository) ProgramBreakdown(ctx context.Context) ([]ProgramBreakdownRow, error) {
	var rows []ProgramBreakdownRow
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select(`COALESCE(NULLIF(users.program, ''), 'Belum diisi') AS program,
		        COUNT(DISTINCT users.id) AS peserta,
		        COUNT(attempts.id) AS attempts,
		        COALESCE(AVG(attempts.score), 0) AS average_score`).
		Joins("LEFT JOIN attempts ON attempts.user_id = users.id").
		Where("users.role = ?", models.RolePeserta).
		Group("program").
		Order("peserta DESC, program ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung sebaran peserta per program.")
	}
	return rows, nil
}

/* ---------------------------- refresh token ----------------------------- */

// CreateRefreshToken menyimpan refresh token baru.
func (r *UserRepository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan sesi login.")
	}
	return nil
}

// FindRefreshToken mencari refresh token berdasarkan hash.
func (r *UserRepository) FindRefreshToken(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.Unauthorized("Sesi login tidak valid. Silakan masuk kembali.")
		}
		return nil, apperror.Internal(err, "Gagal memeriksa sesi login.")
	}
	return &token, nil
}

// RevokeRefreshToken mencabut satu refresh token.
func (r *UserRepository) RevokeRefreshToken(ctx context.Context, id string) error {
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now).Error
	if err != nil {
		return apperror.Internal(err, "Gagal mencabut sesi login.")
	}
	return nil
}

// RevokeAllUserTokens mencabut seluruh refresh token milik satu pengguna.
func (r *UserRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
	if err != nil {
		return apperror.Internal(err, "Gagal mencabut seluruh sesi login.")
	}
	return nil
}

// DeleteExpiredTokens membersihkan token kedaluwarsa (dipanggil berkala).
func (r *UserRepository) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now().UTC()).
		Delete(&models.RefreshToken{})
	if result.Error != nil {
		return 0, apperror.Internal(result.Error, "Gagal membersihkan sesi kedaluwarsa.")
	}
	return result.RowsAffected, nil
}
