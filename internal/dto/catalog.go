package dto

/* ------------------------------- katalog -------------------------------- */

// ProgramView adalah program pada katalog publik.
type ProgramView struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	Audience    string   `json:"audience"`
	Format      string   `json:"format"`
	Duration    string   `json:"duration"`
	Features    []string `json:"features"`
	Highlighted bool     `json:"highlighted"`
}

// MateriCategoryView adalah kategori materi.
type MateriCategoryView struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Topics      []string `json:"topics"`
}

// ProductView adalah produk pada halaman pembayaran.
type ProductView struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	Type        string   `json:"type"`
	Includes    []string `json:"includes"`
	Badge       *string  `json:"badge,omitempty"`
	Owned       bool     `json:"owned"`
}

// PaymentMethodView adalah metode pembayaran.
type PaymentMethodView struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Channel string `json:"channel"`
	Fee     int64  `json:"fee"`
}

/* ------------------------------- billing -------------------------------- */

// QuoteRequest adalah permintaan simulasi harga checkout.
type QuoteRequest struct {
	ProductSlug  string `json:"productSlug" validate:"required"`
	VoucherCode  string `json:"voucherCode" validate:"omitempty,max=40"`
	ReferralCode string `json:"referralCode" validate:"omitempty,max=40"`
	MethodID     string `json:"methodId" validate:"omitempty,max=40"`
}

// QuoteResponse adalah rincian harga setelah voucher & referral.
type QuoteResponse struct {
	ProductSlug      string `json:"productSlug"`
	ProductName      string `json:"productName"`
	Price            int64  `json:"price"`
	VoucherDiscount  int64  `json:"voucherDiscount"`
	ReferralDiscount int64  `json:"referralDiscount"`
	MethodFee        int64  `json:"methodFee"`
	Discount         int64  `json:"discount"`
	Total            int64  `json:"total"`
	VoucherCode      string `json:"voucherCode,omitempty"`
	ReferralCode     string `json:"referralCode,omitempty"`
	MethodID         string `json:"methodId,omitempty"`
	VoucherLabel     string `json:"voucherLabel,omitempty"`
	ReferralPercent  int    `json:"referralPercent"`
	Free             bool   `json:"free"`
}

// CreateOrderRequest adalah permintaan pembuatan pesanan.
type CreateOrderRequest struct {
	ProductSlug  string `json:"productSlug" validate:"required"`
	VoucherCode  string `json:"voucherCode" validate:"omitempty,max=40"`
	ReferralCode string `json:"referralCode" validate:"omitempty,max=40"`
	MethodID     string `json:"methodId" validate:"omitempty,max=40"`
}

// OrderView adalah satu pesanan.
type OrderView struct {
	ID               string `json:"id"`
	InvoiceNo        string `json:"invoiceNo"`
	ProductSlug      string `json:"productSlug"`
	ProductName      string `json:"productName"`
	Price            int64  `json:"price"`
	Discount         int64  `json:"discount"`
	Total            int64  `json:"total"`
	VoucherCode      string `json:"voucherCode,omitempty"`
	ReferralCode     string `json:"referralCode,omitempty"`
	Method           string `json:"method"`
	MethodID         string `json:"methodId,omitempty"`
	Status           string `json:"status"`
	CreatedAt        string `json:"createdAt"`
	PaidAt           string `json:"paidAt,omitempty"`
	ExpiresAt        string `json:"expiresAt,omitempty"`
	PaymentReference string `json:"paymentReference,omitempty"`
	// PaymentProvider & PaymentStatus menjelaskan jalur pembayaran yang dipakai.
	PaymentProvider string `json:"paymentProvider,omitempty"`
	PaymentStatus   string `json:"paymentStatus,omitempty"`
}

// PaymentIntentResponse adalah hasil pembuatan sesi pembayaran sebuah pesanan.
//
// Bila `enabled` bernilai false, kredensial Midtrans belum diisi sehingga
// front-end menampilkan simulator sandbox lokal (`sandboxSimulator`).
type PaymentIntentResponse struct {
	OrderID          string   `json:"orderId"`
	InvoiceNo        string   `json:"invoiceNo"`
	Status           string   `json:"status"`
	Total            int64    `json:"total"`
	Provider         string   `json:"provider"`
	Mode             string   `json:"mode"`
	Enabled          bool     `json:"enabled"`
	IsProduction     bool     `json:"isProduction"`
	SnapToken        string   `json:"snapToken,omitempty"`
	SnapRedirectURL  string   `json:"snapRedirectUrl,omitempty"`
	ClientKey        string   `json:"clientKey,omitempty"`
	SnapScriptURL    string   `json:"snapScriptUrl,omitempty"`
	ExpiresAt        string   `json:"expiresAt,omitempty"`
	EnabledPayments  []string `json:"enabledPayments,omitempty"`
	SandboxSimulator bool     `json:"sandboxSimulator"`
	Instructions     []string `json:"instructions"`
}

// MidtransConfigResponse adalah konfigurasi publik Midtrans untuk front-end.
type MidtransConfigResponse struct {
	Enabled          bool   `json:"enabled"`
	IsProduction     bool   `json:"isProduction"`
	ClientKey        string `json:"clientKey,omitempty"`
	SnapScriptURL    string `json:"snapScriptUrl"`
	SandboxSimulator bool   `json:"sandboxSimulator"`
	Message          string `json:"message"`
}

// SandboxPaymentRequest adalah payload simulator pembayaran lokal.
//
// `outcome` meniru status yang dikirim Midtrans: settlement (berhasil),
// pending (menunggu), expire (kedaluwarsa), deny (ditolak FDS).
type SandboxPaymentRequest struct {
	Outcome string `json:"outcome" validate:"required,oneof=settlement pending expire deny"`
}

// SandboxPaymentResponse adalah balasan simulator pembayaran lokal.
type SandboxPaymentResponse struct {
	OrderID       string    `json:"orderId"`
	InvoiceNo     string    `json:"invoiceNo"`
	Outcome       string    `json:"outcome"`
	SignatureKey  string    `json:"signatureKey"`
	Order         OrderView `json:"order"`
	WebhookStatus string    `json:"webhookStatus"`
	Message       string    `json:"message"`
}

// PayOrderResponse adalah balasan saat pesanan ditandai lunas.
type PayOrderResponse struct {
	Order        OrderView `json:"order"`
	EnrolledSlug string    `json:"enrolledSlug"`
}

// MidtransNotification adalah payload webhook pembayaran (subset Midtrans).
type MidtransNotification struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	PaymentType       string `json:"payment_type"`
	FraudStatus       string `json:"fraud_status"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	StatusCode        string `json:"status_code"`
}

// VoucherView adalah voucher pada dashboard admin.
type VoucherView struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Value       int64  `json:"value"`
	MinPurchase int64  `json:"minPurchase"`
	MaxUses     *int   `json:"maxUses,omitempty"`
	UsedCount   int    `json:"usedCount"`
	Active      bool   `json:"active"`
}

// PaymentWebhookResponse adalah balasan webhook.
type PaymentWebhookResponse struct {
	OrderID   string `json:"orderId"`
	Status    string `json:"status"`
	Processed bool   `json:"processed"`
	Message   string `json:"message"`
}

/* ------------------------------ enrollment ------------------------------ */

// EnrollmentView adalah kepemilikan produk oleh peserta.
type EnrollmentView struct {
	ProductSlug string `json:"productSlug"`
	ProductName string `json:"productName,omitempty"`
	Source      string `json:"source"`
	CreatedAt   string `json:"createdAt"`
}
