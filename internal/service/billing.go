package service

import (
	"context"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// BillingService menangani checkout, pesanan, dan callback pembayaran.
type BillingService struct {
	catalog     *repository.CatalogRepository
	orders      *repository.OrderRepository
	enrollments *repository.EnrollmentRepository
	users       *repository.UserRepository
	learning    *repository.LearningRepository
	cfg         *config.Config
}

// NewBillingService membentuk service billing.
func NewBillingService(
	catalog *repository.CatalogRepository,
	orders *repository.OrderRepository,
	enrollments *repository.EnrollmentRepository,
	users *repository.UserRepository,
	learning *repository.LearningRepository,
	cfg *config.Config,
) *BillingService {
	return &BillingService{
		catalog:     catalog,
		orders:      orders,
		enrollments: enrollments,
		users:       users,
		learning:    learning,
		cfg:         cfg,
	}
}

// Quote menghitung harga akhir (voucher + referral + biaya metode bayar).
//
// Seluruh perhitungan dilakukan di server agar nominal yang dibayar tidak bisa
// diubah dari sisi klien.
func (s *BillingService) Quote(ctx context.Context, userID string, req dto.QuoteRequest) (*dto.QuoteResponse, error) {
	product, err := s.catalog.FindProductBySlug(ctx, req.ProductSlug)
	if err != nil {
		return nil, err
	}

	quote := dto.QuoteResponse{
		ProductSlug:     product.Slug,
		ProductName:     product.Name,
		Price:           product.Price,
		ReferralPercent: s.cfg.Business.ReferralDiscountPercent,
	}

	// Produk gratis: tidak perlu voucher/metode pembayaran.
	if product.IsFree() {
		quote.Free = true
		quote.Discount = product.Price
		quote.Total = 0
		return &quote, nil
	}

	// Voucher.
	if code := strings.TrimSpace(req.VoucherCode); code != "" {
		voucher, err := s.catalog.FindVoucher(ctx, code)
		if err != nil {
			return nil, err
		}
		if product.Price < voucher.MinPurchase {
			return nil, apperror.BadRequest("Nilai transaksi belum memenuhi syarat minimum voucher ini.")
		}
		if voucher.MaxUses != nil && voucher.UsedCount >= *voucher.MaxUses {
			return nil, apperror.BadRequest("Kuota voucher sudah habis.")
		}

		switch voucher.Type {
		case models.VoucherPercent:
			quote.VoucherDiscount = product.Price * voucher.Value / 100
		case models.VoucherFixed:
			quote.VoucherDiscount = voucher.Value
		}
		quote.VoucherCode = voucher.Code
		quote.VoucherLabel = voucher.Label
	}

	// Referral: hanya kode milik pengguna lain yang dikenali.
	if code := strings.TrimSpace(req.ReferralCode); code != "" {
		owner, err := s.users.FindByReferralCode(ctx, code)
		if err != nil {
			return nil, err
		}
		quote.ReferralCode = strPtrOrEmpty(owner.ReferralCode)
		quote.ReferralDiscount = product.Price * int64(s.cfg.Business.ReferralDiscountPercent) / 100
	}

	// Biaya metode pembayaran.
	if methodID := strings.TrimSpace(req.MethodID); methodID != "" {
		method, err := s.catalog.FindPaymentMethod(ctx, methodID)
		if err != nil {
			return nil, err
		}
		quote.MethodID = method.ID
		quote.MethodFee = method.Fee
	}

	quote.Discount = quote.VoucherDiscount + quote.ReferralDiscount
	if quote.Discount > product.Price {
		quote.Discount = product.Price
	}
	quote.Total = product.Price - quote.Discount + quote.MethodFee
	if quote.Total < 0 {
		quote.Total = 0
	}
	quote.Free = quote.Total == 0
	return &quote, nil
}

// CreateOrder membuat pesanan dari produk + voucher/referral yang dipilih.
//
// Pesanan gratis langsung ditandai lunas dan aksesnya langsung diberikan,
// meniru alur "produk gratis → otomatis masuk ruang kelas" pada front-end.
func (s *BillingService) CreateOrder(ctx context.Context, userID string, req dto.CreateOrderRequest) (*dto.OrderView, error) {
	quote, err := s.Quote(ctx, userID, dto.QuoteRequest{
		ProductSlug:  req.ProductSlug,
		VoucherCode:  req.VoucherCode,
		ReferralCode: req.ReferralCode,
		MethodID:     req.MethodID,
	})
	if err != nil {
		return nil, err
	}

	invoiceNo, err := s.orders.NextInvoiceNo(ctx)
	if err != nil {
		return nil, err
	}

	order := models.Order{
		InvoiceNo:   invoiceNo,
		UserID:      userID,
		ProductSlug: quote.ProductSlug,
		ProductName: quote.ProductName,
		Price:       quote.Price,
		Discount:    quote.Discount,
		Total:       quote.Total,
		Method:      quote.MethodID,
		Status:      models.OrderPending,
	}
	if quote.VoucherCode != "" {
		order.VoucherCode = &quote.VoucherCode
	}
	if quote.ReferralCode != "" {
		order.ReferralCode = &quote.ReferralCode
	}
	if !quote.Free {
		expiresAt := time.Now().UTC().Add(24 * time.Hour)
		order.ExpiresAt = &expiresAt
	}

	if err := s.orders.Create(ctx, &order); err != nil {
		return nil, err
	}

	// Produk gratis: langsung lunas + dapat akses.
	if quote.Free {
		paid, err := s.settleOrder(ctx, &order, "free-checkout")
		if err != nil {
			return nil, err
		}
		view := toOrderView(*paid)
		return &view, nil
	}

	// Voucher berbayar: kuota dicatat saat pesanan benar-benar dibuat.
	if order.VoucherCode != nil {
		ok, err := s.catalog.IncrementVoucherUsage(ctx, *order.VoucherCode)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperror.BadRequest("Kuota voucher sudah habis.")
		}
	}

	view := toOrderView(order)
	return &view, nil
}

// ListOrders mengambil riwayat pesanan peserta.
func (s *BillingService) ListOrders(ctx context.Context, userID string, page, perPage int) ([]dto.OrderView, int64, error) {
	orders, total, err := s.orders.ListByUser(ctx, userID, page, perPage)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dto.OrderView, 0, len(orders))
	for _, order := range orders {
		out = append(out, toOrderView(order))
	}
	return out, total, nil
}

// GetOrder mengambil satu pesanan milik peserta.
func (s *BillingService) GetOrder(ctx context.Context, userID, orderID string) (*dto.OrderView, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperror.Forbidden("Pesanan ini bukan milik akun Anda.")
	}
	view := toOrderView(*order)
	return &view, nil
}

// CancelOrder membatalkan pesanan yang masih menunggu pembayaran.
func (s *BillingService) CancelOrder(ctx context.Context, userID, orderID string) (*dto.OrderView, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperror.Forbidden("Pesanan ini bukan milik akun Anda.")
	}
	if order.Status == models.OrderPaid {
		return nil, apperror.Conflict("Pesanan yang sudah dibayar tidak bisa dibatalkan.")
	}

	updated, err := s.orders.ExpireOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	view := toOrderView(*updated)
	return &view, nil
}

// PayOrder mensimulasikan pembayaran berhasil (dipakai tombol demo pada
// front-end). Idempotent: pemanggilan ulang mengembalikan hasil yang sama.
func (s *BillingService) PayOrder(ctx context.Context, userID, orderID string) (*dto.PayOrderResponse, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperror.Forbidden("Pesanan ini bukan milik akun Anda.")
	}
	if order.Status == models.OrderExpired {
		return nil, apperror.Conflict("Pesanan sudah kedaluwarsa. Silakan buat pesanan baru.")
	}

	paid, err := s.settleOrder(ctx, order, "manual-"+order.InvoiceNo)
	if err != nil {
		return nil, err
	}

	return &dto.PayOrderResponse{
		Order:        toOrderView(*paid),
		EnrolledSlug: paid.ProductSlug,
	}, nil
}

// settleOrder menandai pesanan lunas, memberi akses produk, dan mencatat
// notifikasi. Aman dipanggil berulang (idempotent) sehingga callback
// pembayaran yang dikirim ganda tidak menimbulkan efek samping ganda.
func (s *BillingService) settleOrder(ctx context.Context, order *models.Order, reference string) (*models.Order, error) {
	if order.Status == models.OrderPaid {
		return order, nil
	}

	paid, err := s.orders.MarkPaid(ctx, order.ID, reference)
	if err != nil {
		return nil, err
	}

	granted, err := s.enrollments.Grant(ctx, paid.UserID, paid.ProductSlug, &paid.ID, "purchase")
	if err != nil {
		return nil, err
	}

	// Notifikasi hanya dicatat saat akses benar-benar baru diberikan.
	if granted {
		message := "Pembayaran pesanan " + paid.InvoiceNo + " berhasil. Akses kelas sudah aktif."
		_ = s.learning.CreateNotificationLog(ctx, &models.NotificationLog{
			UserID:  &paid.UserID,
			OrderID: &paid.ID,
			Channel: "whatsapp",
			Event:   "order_paid",
			Status:  "queued",
			Message: message,
		})
	}

	return paid, nil
}

// HandleWebhook memproses notifikasi pembayaran Midtrans.
//
// Endpoint ini idempotent: status selain "settlement"/"capture" hanya dicatat,
// dan pesanan yang sudah lunas tidak diproses ulang.
func (s *BillingService) HandleWebhook(ctx context.Context, notif dto.MidtransNotification) (*dto.PaymentWebhookResponse, error) {
	if strings.TrimSpace(notif.OrderID) == "" {
		return nil, apperror.BadRequest("order_id wajib disertakan.")
	}

	order, err := s.orders.FindByInvoiceNo(ctx, notif.OrderID)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(notif.TransactionStatus))
	response := &dto.PaymentWebhookResponse{OrderID: order.InvoiceNo, Status: status}

	switch status {
	case "settlement", "capture":
		reference := notif.TransactionID
		if reference == "" {
			reference = "midtrans-" + order.InvoiceNo
		}
		if _, err := s.settleOrder(ctx, order, reference); err != nil {
			return nil, err
		}
		response.Processed = true
		response.Message = "Pembayaran tercatat. Akses kelas sudah dibuka."
	case "expire", "cancel", "deny":
		if _, err := s.orders.ExpireOrder(ctx, order.ID); err != nil {
			return nil, err
		}
		response.Processed = true
		response.Message = "Pesanan ditandai kedaluwarsa."
	default:
		response.Message = "Status pembayaran diabaikan."
	}

	return response, nil
}

// ListEnrollments mengambil daftar kelas/produk yang dimiliki peserta.
func (s *BillingService) ListEnrollments(ctx context.Context, userID string) ([]dto.EnrollmentView, error) {
	rows, err := s.enrollments.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EnrollmentView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toEnrollmentView(row))
	}
	return out, nil
}
