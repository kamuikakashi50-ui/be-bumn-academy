package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/midtrans"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// ProviderMidtrans adalah penanda jalur pembayaran Midtrans pada pesanan.
const ProviderMidtrans = "midtrans"

// BillingService menangani checkout, pesanan, dan callback pembayaran.
type BillingService struct {
	catalog     *repository.CatalogRepository
	orders      *repository.OrderRepository
	enrollments *repository.EnrollmentRepository
	users       *repository.UserRepository
	learning    *repository.LearningRepository
	midtrans    *midtrans.Client
	cfg         *config.Config
}

// NewBillingService membentuk service billing.
func NewBillingService(
	catalog *repository.CatalogRepository,
	orders *repository.OrderRepository,
	enrollments *repository.EnrollmentRepository,
	users *repository.UserRepository,
	learning *repository.LearningRepository,
	midtransClient *midtrans.Client,
	cfg *config.Config,
) *BillingService {
	return &BillingService{
		catalog:     catalog,
		orders:      orders,
		enrollments: enrollments,
		users:       users,
		learning:    learning,
		midtrans:    midtransClient,
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
// Keamanan: signature_key diverifikasi lebih dulu (SHA512 atas
// order_id+status_code+gross_amount+ServerKey). Bila Server Key belum diisi,
// kunci simulator lokal dipakai sehingga jalur ini tetap teruji.
//
// Idempotent: pesanan yang sudah lunas tidak diproses ulang, dan status
// "pending" hanya dicatat tanpa membuka akses.
func (s *BillingService) HandleWebhook(ctx context.Context, notif dto.MidtransNotification) (*dto.PaymentWebhookResponse, error) {
	if strings.TrimSpace(notif.OrderID) == "" {
		return nil, apperror.BadRequest("order_id wajib disertakan.")
	}
	if !s.midtrans.VerifySignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, notif.SignatureKey) {
		return nil, apperror.Unauthorized("Signature notifikasi Midtrans tidak valid.")
	}

	order, err := s.orders.FindByInvoiceNo(ctx, notif.OrderID)
	if err != nil {
		return nil, err
	}

	updated, message, err := s.applyProviderStatus(ctx, order, notif.TransactionStatus, notif.FraudStatus, notif.TransactionID)
	if err != nil {
		return nil, err
	}

	return &dto.PaymentWebhookResponse{
		OrderID:   order.InvoiceNo,
		Status:    strings.ToLower(strings.TrimSpace(notif.TransactionStatus)),
		Processed: updated.Status != models.OrderPending,
		Message:   message,
	}, nil
}

// applyProviderStatus menerapkan status dari Midtrans (webhook atau polling)
// pada sebuah pesanan, lalu mengembalikan pesanan terbaru + pesan untuk klien.
//
// Pemetaan status Midtrans → status pesanan:
//
//	settlement/capture (fraud accept) → lunas + akses produk diberikan
//	settlement/capture (fraud deny)   → kedaluwarsa (transaksi dianggap gagal)
//	pending                           → tetap menunggu, hanya status mentah dicatat
//	expire/cancel/deny/failure        → kedaluwarsa
//	refund/partial_refund             → dicatat, pesanan tetap lunas
func (s *BillingService) applyProviderStatus(
	ctx context.Context,
	order *models.Order,
	transactionStatus, fraudStatus, reference string,
) (*models.Order, string, error) {
	status := strings.ToLower(strings.TrimSpace(transactionStatus))
	fraud := strings.ToLower(strings.TrimSpace(fraudStatus))
	if status == "" {
		status = "unknown"
	}

	// Status mentah selalu dicatat agar panel admin bisa menelusuri.
	if err := s.orders.UpdatePaymentStatus(ctx, order.ID, status); err != nil {
		return nil, "", err
	}

	switch status {
	case "settlement", "capture":
		if fraud == "deny" {
			if _, err := s.orders.ExpireOrder(ctx, order.ID); err != nil {
				return nil, "", err
			}
			return s.reload(ctx, order.ID, "Transaksi ditolak verifikasi fraud Midtrans.")
		}
		if reference == "" {
			reference = "midtrans-" + order.InvoiceNo
		}
		paid, err := s.settleOrder(ctx, order, reference)
		if err != nil {
			return nil, "", err
		}
		return paid, "Pembayaran tercatat. Akses kelas sudah dibuka.", nil
	case "pending":
		return s.reload(ctx, order.ID, "Menunggu pembayaran dari peserta.")
	case "expire", "cancel", "deny", "failure":
		if order.Status == models.OrderPaid {
			// Pesanan yang sudah lunas tidak dibatalkan otomatis.
			return s.reload(ctx, order.ID, "Status pembayaran diabaikan karena pesanan sudah lunas.")
		}
		if _, err := s.orders.ExpireOrder(ctx, order.ID); err != nil {
			return nil, "", err
		}
		return s.reload(ctx, order.ID, "Pesanan ditandai kedaluwarsa.")
	case "refund", "partial_refund":
		return s.reload(ctx, order.ID, "Pengembalian dana dicatat; pesanan tetap lunas.")
	default:
		return s.reload(ctx, order.ID, "Status pembayaran diabaikan.")
	}
}

// reload mengambil ulang pesanan beserta pesan balasan.
func (s *BillingService) reload(ctx context.Context, orderID, message string) (*models.Order, string, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, "", err
	}
	return order, message, nil
}

/* ------------------------------ sesi bayar -------------------------------- */

// MidtransConfig mengembalikan konfigurasi publik Midtrans untuk front-end.
func (s *BillingService) MidtransConfig() *dto.MidtransConfigResponse {
	response := &dto.MidtransConfigResponse{
		Enabled:          s.midtrans.Enabled(),
		IsProduction:     s.midtrans.IsProduction(),
		ClientKey:        s.midtrans.ClientKey(),
		SnapScriptURL:    s.midtrans.SnapScriptURL(),
		SandboxSimulator: s.cfg.SandboxSimulatorAllowed(),
	}
	switch {
	case !response.Enabled:
		response.Message = "Kredensial Midtrans belum diisi (MIDTRANS_SERVER_KEY). Pembayaran memakai simulator sandbox lokal."
	case !response.IsProduction:
		response.Message = "Midtrans berjalan pada mode sandbox."
	default:
		response.Message = "Midtrans berjalan pada mode produksi."
	}
	return response
}

// PaymentIntent menyiapkan sesi pembayaran sebuah pesanan.
//
// Bila kredensial Midtrans tersedia, transaksi Snap dibuat (token + redirect
// URL). Bila belum, balasan menandai mode simulator lokal sehingga front-end
// bisa menampilkan tombol simulasi pembayaran. `refresh=true` memaksa token baru.
func (s *BillingService) PaymentIntent(ctx context.Context, userID, orderID string, refresh bool) (*dto.PaymentIntentResponse, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperror.Forbidden("Pesanan ini bukan milik akun Anda.")
	}
	if order.Status == models.OrderPaid {
		return nil, apperror.Conflict("Pesanan ini sudah lunas.")
	}
	if order.Status == models.OrderExpired {
		return nil, apperror.Conflict("Pesanan sudah kedaluwarsa. Silakan buat pesanan baru.")
	}
	if order.Total <= 0 {
		return nil, apperror.BadRequest("Pesanan tanpa nominal tidak memerlukan pembayaran Midtrans.")
	}
	if order.ExpiresAt != nil && order.ExpiresAt.Before(time.Now().UTC()) {
		if _, err := s.orders.ExpireOrder(ctx, order.ID); err != nil {
			return nil, err
		}
		return nil, apperror.Conflict("Batas waktu pembayaran sudah lewat. Silakan buat pesanan baru.")
	}

	channels := midtrans.ChannelsFor(order.Method)

	// Simulator lokal: kredensial Midtrans belum diisi.
	if !s.midtrans.Enabled() {
		return &dto.PaymentIntentResponse{
			OrderID:          order.ID,
			InvoiceNo:        order.InvoiceNo,
			Status:           string(order.Status),
			Total:            order.Total,
			Provider:         ProviderMidtrans,
			Mode:             "simulator",
			Enabled:          false,
			SandboxSimulator: s.cfg.SandboxSimulatorAllowed(),
			ExpiresAt:        formatTimePtr(order.ExpiresAt),
			EnabledPayments:  emptyIfNil(channels),
			Instructions: []string{
				"Kredensial Midtrans belum diisi, sehingga pembayaran dijalankan lewat simulator sandbox lokal.",
				"Simulator mengirim notifikasi tiruan bertanda tangan kunci sandbox, sehingga jalur webhook yang sama tetap teruji.",
			},
		}, nil
	}

	// Token lama dipakai ulang selama pesanan masih menunggu pembayaran.
	if !refresh && order.PaymentProvider == ProviderMidtrans && order.SnapToken != nil && *order.SnapToken != "" {
		return s.intentFromStored(order, *order.SnapToken, strPtrOrEmpty(order.SnapRedirectURL), channels), nil
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	transaction, err := s.midtrans.CreateSnapTransaction(ctx, s.snapPayload(order, user, channels))
	if err != nil {
		return nil, err
	}
	if err := s.orders.SavePaymentIntent(ctx, order.ID, ProviderMidtrans, transaction.Token, transaction.RedirectURL); err != nil {
		return nil, err
	}

	return s.intentFromStored(order, transaction.Token, transaction.RedirectURL, channels), nil
}

// intentFromStored menyusun balasan sesi pembayaran dari token yang tersimpan.
func (s *BillingService) intentFromStored(order *models.Order, token, redirectURL string, channels []string) *dto.PaymentIntentResponse {
	return &dto.PaymentIntentResponse{
		OrderID:          order.ID,
		InvoiceNo:        order.InvoiceNo,
		Status:           string(order.Status),
		Total:            order.Total,
		Provider:         ProviderMidtrans,
		Mode:             "snap",
		Enabled:          true,
		IsProduction:     s.midtrans.IsProduction(),
		SnapToken:        token,
		SnapRedirectURL:  redirectURL,
		ClientKey:        s.midtrans.ClientKey(),
		SnapScriptURL:    s.midtrans.SnapScriptURL(),
		ExpiresAt:        formatTimePtr(order.ExpiresAt),
		EnabledPayments:  emptyIfNil(channels),
		SandboxSimulator: s.cfg.SandboxSimulatorAllowed(),
		Instructions:     s.snapInstructions(order, channels),
	}
}

// snapInstructions menyusun petunjuk singkat yang ditampilkan di checkout.
func (s *BillingService) snapInstructions(order *models.Order, channels []string) []string {
	steps := []string{
		"Tekan tombol bayar untuk membuka halaman pembayaran Midtrans (Snap).",
		"Selesaikan pembayaran, lalu kembali ke halaman ini.",
	}
	if s.cfg.SandboxSimulatorAllowed() {
		steps = append(steps, "Di sandbox, gunakan menu Simulator pada halaman Snap untuk menandai pembayaran berhasil.")
	}
	if len(channels) == 0 {
		steps = append(steps, "Semua kanal pembayaran yang aktif pada akun Midtrans akan ditampilkan.")
	}
	if order.ExpiresAt != nil {
		steps = append(steps, "Status diperbarui otomatis dari notifikasi Midtrans; tombol cek status tersedia bila notifikasi belum sampai.")
	}
	return steps
}

// snapPayload menyusun payload transaksi Snap.
//
// Rincian item harus berjumlah tepat sama dengan gross_amount, karena Midtrans
// menolak transaksi yang nominalnya tidak konsisten.
func (s *BillingService) snapPayload(order *models.Order, user *models.User, channels []string) midtrans.SnapRequest {
	items := []midtrans.SnapItem{{
		ID:       order.ProductSlug,
		Price:    order.Price,
		Quantity: 1,
		Name:     truncateRunes(order.ProductName, 50),
	}}
	if order.Discount > 0 {
		items = append(items, midtrans.SnapItem{
			ID:       "DISCOUNT",
			Price:    -order.Discount,
			Quantity: 1,
			Name:     truncateRunes(discountLabel(order), 50),
		})
	}
	if fee := order.Total - (order.Price - order.Discount); fee > 0 {
		items = append(items, midtrans.SnapItem{
			ID:       "METHOD-FEE",
			Price:    fee,
			Quantity: 1,
			Name:     "Biaya metode pembayaran",
		})
	}

	// Jaring pengaman: bila rincian tidak berjumlah pas, kirim satu baris saja.
	sum := int64(0)
	for _, item := range items {
		sum += item.Price * int64(item.Quantity)
	}
	if sum != order.Total {
		items = []midtrans.SnapItem{{
			ID:       order.InvoiceNo,
			Price:    order.Total,
			Quantity: 1,
			Name:     truncateRunes(order.ProductName, 50),
		}}
	}

	firstName, lastName := splitName(user.Name)
	payload := midtrans.SnapRequest{
		ItemDetails: items,
		CustomerDetails: midtrans.SnapCustomer{
			FirstName: firstName,
			LastName:  lastName,
			Email:     user.Email,
		},
		Callbacks:       s.snapCallbacks(order),
		EnabledPayments: channels,
		CustomField1:    "bumn-academy",
	}
	payload.TransactionDetails.OrderID = order.InvoiceNo
	payload.TransactionDetails.GrossAmount = order.Total

	// Masa berlaku halaman Snap mengikuti sisa waktu pesanan (maksimal 24 jam).
	if order.ExpiresAt != nil {
		minutes := int(math.Ceil(time.Until(*order.ExpiresAt).Minutes()))
		if minutes < 30 {
			minutes = 30
		}
		if minutes > 24*60 {
			minutes = 24 * 60
		}
		payload.Expiry = &midtrans.SnapExpiry{Unit: "minute", Duration: minutes}
	}

	return payload
}

// snapCallbacks mengarahkan peserta kembali ke halaman pembayaran dashboard.
func (s *BillingService) snapCallbacks(order *models.Order) *midtrans.SnapCallbacks {
	base := s.cfg.Business.FrontendBaseURL
	if base == "" {
		return nil
	}
	link := func(status string) string {
		return fmt.Sprintf("%s/dashboard/pembayaran?order=%s&status=%s", base, order.ID, status)
	}
	return &midtrans.SnapCallbacks{
		Finish:   link("finish"),
		Unfinish: link("unfinish"),
		Error:    link("error"),
	}
}

// SyncPaymentStatus menanyakan status transaksi langsung ke Midtrans.
//
// Berguna saat notifikasi webhook tidak bisa menjangkau server (mis. backend di
// localhost tanpa tunnel): front-end memicu pemeriksaan ini dari tombol
// "Cek status pembayaran".
func (s *BillingService) SyncPaymentStatus(ctx context.Context, userID, orderID string) (*dto.OrderView, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperror.Forbidden("Pesanan ini bukan milik akun Anda.")
	}
	if order.Status != models.OrderPending {
		view := toOrderView(*order)
		return &view, nil
	}
	if !s.midtrans.Enabled() {
		return nil, apperror.BadRequest("Midtrans belum dikonfigurasi, sehingga status tidak bisa diperiksa ke penyedia.")
	}

	status, err := s.midtrans.Status(ctx, order.InvoiceNo)
	if err != nil {
		return nil, err
	}
	updated, _, err := s.applyProviderStatus(ctx, order, status.TransactionStatus, status.FraudStatus, status.TransactionID)
	if err != nil {
		return nil, err
	}
	view := toOrderView(*updated)
	return &view, nil
}

// SandboxSimulate meniru notifikasi Midtrans untuk pengujian lokal.
//
// Notifikasi tiruan ditandatangani memakai kunci yang sama dengan verifikasi di
// HandleWebhook, sehingga jalur produksi (verifikasi signature → pelunasan →
// pemberian akses) benar-benar dijalankan. Hanya aktif saat
// MIDTRANS_SANDBOX_SIMULATOR=true dan aplikasi tidak berjalan di produksi.
func (s *BillingService) SandboxSimulate(ctx context.Context, userID, orderID, outcome string) (*dto.SandboxPaymentResponse, error) {
	if !s.cfg.SandboxSimulatorAllowed() {
		return nil, apperror.Forbidden("Simulator pembayaran tidak aktif pada konfigurasi ini.")
	}

	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperror.Forbidden("Pesanan ini bukan milik akun Anda.")
	}
	if order.Status == models.OrderPaid {
		return nil, apperror.Conflict("Pesanan ini sudah lunas.")
	}

	statusCode, transactionStatus, fraudStatus := sandboxOutcome(outcome)
	grossAmount := fmt.Sprintf("%d.00", order.Total)
	transactionID := "sandbox-" + order.InvoiceNo + "-" + transactionStatus

	notif := dto.MidtransNotification{
		OrderID:           order.InvoiceNo,
		TransactionID:     transactionID,
		TransactionStatus: transactionStatus,
		FraudStatus:       fraudStatus,
		PaymentType:       "sandbox-simulator",
		GrossAmount:       grossAmount,
		StatusCode:        statusCode,
	}
	notif.SignatureKey = midtrans.Signature(notif.OrderID, notif.StatusCode, notif.GrossAmount, s.midtrans.SigningKey())

	result, err := s.HandleWebhook(ctx, notif)
	if err != nil {
		return nil, err
	}
	if order.PaymentProvider == "" {
		// Menandai bahwa pesanan ini diproses lewat jalur Midtrans.
		_ = s.orders.SavePaymentIntent(ctx, order.ID, ProviderMidtrans, "", "")
	}

	updated, err := s.orders.FindByID(ctx, order.ID)
	if err != nil {
		return nil, err
	}

	return &dto.SandboxPaymentResponse{
		OrderID:       order.ID,
		InvoiceNo:     order.InvoiceNo,
		Outcome:       outcome,
		SignatureKey:  notif.SignatureKey,
		Order:         toOrderView(*updated),
		WebhookStatus: result.Status,
		Message:       result.Message,
	}, nil
}

// sandboxOutcome memetakan pilihan simulator ke status ala Midtrans.
func sandboxOutcome(outcome string) (statusCode, transactionStatus, fraudStatus string) {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "settlement":
		return "200", "settlement", "accept"
	case "pending":
		return "201", "pending", "accept"
	case "deny":
		return "202", "deny", "deny"
	default: // expire
		return "407", "expire", "accept"
	}
}

// discountLabel membentuk label baris diskon pada halaman Snap.
func discountLabel(order *models.Order) string {
	if order.VoucherCode != nil && *order.VoucherCode != "" {
		return "Diskon voucher " + *order.VoucherCode
	}
	if order.ReferralCode != nil && *order.ReferralCode != "" {
		return "Diskon referral " + *order.ReferralCode
	}
	return "Diskon"
}

// splitName memisahkan nama lengkap menjadi nama depan & sisa nama.
func splitName(name string) (string, string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "Peserta", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// truncateRunes memotong teks (per karakter) agar aman bagi batas field Midtrans.
func truncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
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
