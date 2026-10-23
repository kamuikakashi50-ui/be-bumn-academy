package service

import (
	"context"

	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// CatalogService melayani konten katalog publik dengan caching.
//
// Katalog hampir tidak pernah berubah tetapi dibaca setiap kali halaman publik
// dibuka. Karena itu hasilnya disimpan di Redis dengan TTL pendek dan
// dilindungi singleflight, sehingga lonjakan trafik tidak menyentuh database.
type CatalogService struct {
	catalog     *repository.CatalogRepository
	enrollments *repository.EnrollmentRepository
	cache       *cache.Cache
	cfg         *config.Config
}

// NewCatalogService membentuk service katalog.
func NewCatalogService(
	catalog *repository.CatalogRepository,
	enrollments *repository.EnrollmentRepository,
	cacheLayer *cache.Cache,
	cfg *config.Config,
) *CatalogService {
	return &CatalogService{catalog: catalog, enrollments: enrollments, cache: cacheLayer, cfg: cfg}
}

// Programs mengembalikan daftar program.
func (s *CatalogService) Programs(ctx context.Context) ([]dto.ProgramView, error) {
	rows, err := cache.Remember(ctx, s.cache, cache.KeyCatalogPrograms(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) ([]models.Program, error) {
			return s.catalog.ListPrograms(ctx)
		})
	if err != nil {
		return nil, err
	}

	out := make([]dto.ProgramView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProgramView(row))
	}
	return out, nil
}

// MateriCategories mengembalikan kategori materi.
func (s *CatalogService) MateriCategories(ctx context.Context) ([]dto.MateriCategoryView, error) {
	rows, err := cache.Remember(ctx, s.cache, cache.KeyCatalogMateri(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) ([]models.MateriCategory, error) {
			return s.catalog.ListMateriCategories(ctx)
		})
	if err != nil {
		return nil, err
	}

	out := make([]dto.MateriCategoryView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMateriView(row))
	}
	return out, nil
}

// Products mengembalikan produk beserta status kepemilikan peserta.
//
// Daftar produk di-cache, sedangkan status kepemilikan dihitung per request
// dengan satu query (bukan N+1).
func (s *CatalogService) Products(ctx context.Context, userID string) ([]dto.ProductView, error) {
	rows, err := cache.Remember(ctx, s.cache, cache.KeyCatalogProducts(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) ([]models.Product, error) {
			return s.catalog.ListProducts(ctx, false)
		})
	if err != nil {
		return nil, err
	}

	owned := map[string]bool{}
	if userID != "" {
		slugs, err := s.enrollments.OwnedSlugs(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, slug := range slugs {
			owned[slug] = true
		}
	}

	out := make([]dto.ProductView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProductView(row, owned[row.Slug]))
	}
	return out, nil
}

// PaymentMethods mengembalikan metode pembayaran aktif.
func (s *CatalogService) PaymentMethods(ctx context.Context) ([]dto.PaymentMethodView, error) {
	rows, err := cache.Remember(ctx, s.cache, cache.KeyCatalogPaymentMethods(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) ([]models.PaymentMethod, error) {
			return s.catalog.ListPaymentMethods(ctx, false)
		})
	if err != nil {
		return nil, err
	}

	out := make([]dto.PaymentMethodView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPaymentMethodView(row))
	}
	return out, nil
}

// Vouchers mengembalikan daftar voucher (khusus admin, tidak di-cache).
func (s *CatalogService) Vouchers(ctx context.Context, includeInactive bool) ([]dto.VoucherView, error) {
	rows, err := s.catalog.ListVouchers(ctx, includeInactive)
	if err != nil {
		return nil, err
	}
	out := make([]dto.VoucherView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toVoucherView(row))
	}
	return out, nil
}

// Invalidate menghapus cache katalog setelah admin mengubah konten.
func (s *CatalogService) Invalidate(ctx context.Context) {
	if s.cache == nil {
		return
	}
	s.cache.ForgetPrefix(ctx, cache.PrefixCatalog())
}
