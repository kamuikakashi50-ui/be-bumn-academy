package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderStatus adalah status pesanan.
type OrderStatus string

const (
	// OrderPending menunggu pembayaran.
	OrderPending OrderStatus = "pending"
	// OrderPaid sudah dibayar.
	OrderPaid OrderStatus = "paid"
	// OrderExpired kedaluwarsa/dibatalkan.
	OrderExpired OrderStatus = "expired"
)

// Order adalah transaksi pembelian produk.
type Order struct {
	Base
	InvoiceNo        string      `gorm:"column:invoice_no;not null" json:"invoiceNo"`
	UserID           string      `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	ProductSlug      string      `gorm:"column:product_slug;not null" json:"productSlug"`
	ProductName      string      `gorm:"column:product_name;not null;default:''" json:"productName"`
	Price            int64       `gorm:"column:price;not null;default:0" json:"price"`
	Discount         int64       `gorm:"column:discount;not null;default:0" json:"discount"`
	Total            int64       `gorm:"column:total;not null;default:0" json:"total"`
	VoucherCode      *string     `gorm:"column:voucher_code" json:"voucherCode,omitempty"`
	ReferralCode     *string     `gorm:"column:referral_code" json:"referralCode,omitempty"`
	Method           string      `gorm:"column:method;not null;default:''" json:"method"`
	Status           OrderStatus `gorm:"column:status;not null;default:pending" json:"status"`
	PaymentReference *string     `gorm:"column:payment_reference" json:"paymentReference,omitempty"`
	PaidAt           *time.Time  `gorm:"column:paid_at" json:"paidAt,omitempty"`
	ExpiresAt        *time.Time  `gorm:"column:expires_at" json:"expiresAt,omitempty"`
}

// TableName menetapkan nama tabel.
func (Order) TableName() string { return "orders" }

// Enrollment menandakan peserta memiliki akses ke sebuah produk.
//
// Tidak memakai `Base` karena tabelnya sengaja tanpa kolom `updated_at`
// (kepemilikan produk bersifat append-only).
type Enrollment struct {
	ID          string    `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID      string    `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	ProductSlug string    `gorm:"column:product_slug;not null" json:"productSlug"`
	OrderID     *string   `gorm:"column:order_id;type:uuid" json:"orderId,omitempty"`
	Source      string    `gorm:"column:source;not null;default:purchase" json:"source"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (Enrollment) TableName() string { return "enrollments" }

// BeforeCreate memastikan ID terisi.
func (e *Enrollment) BeforeCreate(*gorm.DB) error {
	if e.ID == "" {
		e.ID = NewID()
	}
	return nil
}

// NotificationLog mencatat notifikasi transaksi (WhatsApp/email).
type NotificationLog struct {
	ID        string     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID    *string    `gorm:"column:user_id;type:uuid" json:"userId,omitempty"`
	OrderID   *string    `gorm:"column:order_id;type:uuid" json:"orderId,omitempty"`
	Channel   string     `gorm:"column:channel;not null;default:whatsapp" json:"channel"`
	Event     string     `gorm:"column:event;not null" json:"event"`
	Status    string     `gorm:"column:status;not null;default:queued" json:"status"`
	Message   string     `gorm:"column:message;not null;default:''" json:"message"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	SentAt    *time.Time `gorm:"column:sent_at" json:"sentAt,omitempty"`
}

// TableName menetapkan nama tabel.
func (NotificationLog) TableName() string { return "notification_logs" }

// BeforeCreate memastikan ID terisi.
func (n *NotificationLog) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = NewID()
	}
	return nil
}
