package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
)

// OrderRepository mengakses tabel orders.
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository membentuk repository.
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *OrderRepository) DB() *gorm.DB { return r.db }

// NextInvoiceNo mengambil nomor invoice berikutnya dari sequence PostgreSQL.
//
// Memakai `nextval` (bukan COUNT) sehingga tetap unik dan bebas race condition
// walau ratusan checkout terjadi pada detik yang sama.
func (r *OrderRepository) NextInvoiceNo(ctx context.Context) (string, error) {
	var invoice string
	err := r.db.WithContext(ctx).Raw(
		`SELECT 'BA-' || to_char(now(), 'YYYY') || '-' || lpad(nextval('order_invoice_seq')::text, 5, '0')`,
	).Scan(&invoice).Error
	if err != nil {
		return "", apperror.Internal(err, "Gagal membuat nomor invoice.")
	}
	return invoice, nil
}

// Create menyimpan pesanan baru.
func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	if err := r.db.WithContext(ctx).Create(order).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan pesanan.")
	}
	return nil
}

// FindByID mencari pesanan berdasarkan id.
func (r *OrderRepository) FindByID(ctx context.Context, id string) (*models.Order, error) {
	var order models.Order
	err := r.db.WithContext(ctx).First(&order, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Pesanan tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat pesanan.")
	}
	return &order, nil
}

// FindByInvoiceNo mencari pesanan berdasarkan nomor invoice.
func (r *OrderRepository) FindByInvoiceNo(ctx context.Context, invoiceNo string) (*models.Order, error) {
	var order models.Order
	err := r.db.WithContext(ctx).Where("invoice_no = ?", invoiceNo).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Pesanan tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat pesanan.")
	}
	return &order, nil
}

// ListByUser mengambil pesanan milik satu pengguna.
func (r *OrderRepository) ListByUser(ctx context.Context, userID string, page, perPage int) ([]models.Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Order{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err, "Gagal menghitung pesanan.")
	}

	var orders []models.Order
	err := query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&orders).Error
	if err != nil {
		return nil, 0, apperror.Internal(err, "Gagal memuat pesanan.")
	}
	return orders, total, nil
}

// OrderFilter adalah filter daftar pesanan untuk admin.
type OrderFilter struct {
	Status string
	Search string
}

// ListAll mengambil seluruh pesanan (khusus admin) beserta identitas pemesan.
func (r *OrderRepository) ListAll(ctx context.Context, filter OrderFilter, page, perPage int) ([]models.Order, map[string]models.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Order{})
	if filter.Status != "" {
		query = query.Where("orders.status = ?", filter.Status)
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		query = query.Where("orders.invoice_no ILIKE ? OR orders.product_name ILIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, apperror.Internal(err, "Gagal menghitung pesanan.")
	}

	var orders []models.Order
	err := query.Order("orders.created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&orders).Error
	if err != nil {
		return nil, nil, 0, apperror.Internal(err, "Gagal memuat pesanan.")
	}

	users, err := r.usersByIDs(ctx, orderUserIDs(orders))
	if err != nil {
		return nil, nil, 0, err
	}
	return orders, users, total, nil
}

func orderUserIDs(orders []models.Order) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(orders))
	for _, order := range orders {
		if _, ok := seen[order.UserID]; ok {
			continue
		}
		seen[order.UserID] = struct{}{}
		out = append(out, order.UserID)
	}
	return out
}

// usersByIDs mengambil sekumpulan pengguna dalam satu query (menghindari N+1).
func (r *OrderRepository) usersByIDs(ctx context.Context, ids []string) (map[string]models.User, error) {
	out := make(map[string]models.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var users []models.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat data pemesan.")
	}
	for _, user := range users {
		out[user.ID] = user
	}
	return out, nil
}

// MarkPaid menandai pesanan lunas secara atomik.
//
// WHERE status = 'pending' membuat operasi ini idempotent: callback pembayaran
// yang dikirim berulang tidak akan mengubah paid_at berkali-kali.
func (r *OrderRepository) MarkPaid(ctx context.Context, orderID, paymentReference string) (*models.Order, error) {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&models.Order{}).
		Where("id = ? AND status = ?", orderID, models.OrderPending).
		Updates(map[string]any{
			"status":            models.OrderPaid,
			"paid_at":           now,
			"payment_reference": paymentReference,
			"updated_at":        now,
		})
	if result.Error != nil {
		return nil, apperror.Internal(result.Error, "Gagal menandai pesanan lunas.")
	}
	return r.FindByID(ctx, orderID)
}

// ExpireOrder menandai pesanan kedaluwarsa/dibatalkan.
func (r *OrderRepository) ExpireOrder(ctx context.Context, orderID string) (*models.Order, error) {
	result := r.db.WithContext(ctx).Model(&models.Order{}).
		Where("id = ? AND status = ?", orderID, models.OrderPending).
		Update("status", models.OrderExpired)
	if result.Error != nil {
		return nil, apperror.Internal(result.Error, "Gagal membatalkan pesanan.")
	}
	return r.FindByID(ctx, orderID)
}

// CountByStatus menghitung pesanan per status.
func (r *OrderRepository) CountByStatus(ctx context.Context) (map[models.OrderStatus]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.Order{}).
		Select("status, count(*) AS total").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung pesanan.")
	}

	out := make(map[models.OrderStatus]int64, len(rows))
	for _, item := range rows {
		out[models.OrderStatus(item.Status)] = item.Total
	}
	return out, nil
}

// SumPaidRevenue menghitung total pendapatan dari pesanan lunas.
func (r *OrderRepository) SumPaidRevenue(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.Order{}).
		Where("status = ?", models.OrderPaid).
		Select("COALESCE(SUM(total), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung pendapatan.")
	}
	return total, nil
}

// ProductSalesRow adalah hasil agregasi penjualan per produk.
type ProductSalesRow struct {
	ProductSlug string `gorm:"column:product_slug"`
	ProductName string `gorm:"column:product_name"`
	Orders      int64  `gorm:"column:orders"`
	Revenue     int64  `gorm:"column:revenue"`
}

// TopProducts mengambil produk dengan penjualan terbaik.
func (r *OrderRepository) TopProducts(ctx context.Context, limit int) ([]ProductSalesRow, error) {
	var rows []ProductSalesRow
	err := r.db.WithContext(ctx).Model(&models.Order{}).
		Select("product_slug, MAX(product_name) AS product_name, COUNT(*) AS orders, COALESCE(SUM(total), 0) AS revenue").
		Where("status = ?", models.OrderPaid).
		Group("product_slug").
		Order("revenue DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat penjualan produk.")
	}
	return rows, nil
}
