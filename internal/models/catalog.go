package models

import "time"

// Program adalah program bimbel pada katalog publik.
type Program struct {
	Base
	Slug        string     `gorm:"column:slug;not null" json:"slug"`
	Name        string     `gorm:"column:name;not null" json:"name"`
	Tagline     string     `gorm:"column:tagline;not null;default:''" json:"tagline"`
	Audience    string     `gorm:"column:audience;not null;default:''" json:"audience"`
	Format      string     `gorm:"column:format;not null;default:''" json:"format"`
	Duration    string     `gorm:"column:duration;not null;default:''" json:"duration"`
	Features    StringList `gorm:"column:features;type:jsonb;not null;default:'[]'" json:"features"`
	Highlighted bool       `gorm:"column:highlighted;not null;default:false" json:"highlighted"`
	SortOrder   int        `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (Program) TableName() string { return "programs" }

// MateriCategory adalah kategori materi pada halaman /materi.
type MateriCategory struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	Label       string     `gorm:"column:label;not null" json:"label"`
	Description string     `gorm:"column:description;not null;default:''" json:"description"`
	Topics      StringList `gorm:"column:topics;type:jsonb;not null;default:'[]'" json:"topics"`
	SortOrder   int        `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (MateriCategory) TableName() string { return "materi_categories" }

// ProductType membedakan produk program dan produk tryout.
type ProductType string

const (
	// ProductProgram adalah paket belajar/program.
	ProductProgram ProductType = "program"
	// ProductTryout adalah produk berupa paket tryout.
	ProductTryout ProductType = "tryout"
)

// Product adalah produk yang dijual pada halaman pembayaran.
type Product struct {
	Base
	Slug        string      `gorm:"column:slug;not null" json:"slug"`
	Name        string      `gorm:"column:name;not null" json:"name"`
	Description string      `gorm:"column:description;not null;default:''" json:"description"`
	Price       int64       `gorm:"column:price;not null;default:0" json:"price"`
	Type        ProductType `gorm:"column:type;not null;default:program" json:"type"`
	Includes    StringList  `gorm:"column:includes;type:jsonb;not null;default:'[]'" json:"includes"`
	Badge       *string     `gorm:"column:badge" json:"badge,omitempty"`
	Active      bool        `gorm:"column:active;not null;default:true" json:"active"`
	SortOrder   int         `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (Product) TableName() string { return "products" }

// IsFree menandakan produk gratis.
func (p Product) IsFree() bool { return p.Price <= 0 }

// PaymentMethod adalah metode pembayaran yang tersedia.
type PaymentMethod struct {
	ID        string `gorm:"column:id;primaryKey" json:"id"`
	Label     string `gorm:"column:label;not null" json:"label"`
	Channel   string `gorm:"column:channel;not null;default:''" json:"channel"`
	Fee       int64  `gorm:"column:fee;not null;default:0" json:"fee"`
	Active    bool   `gorm:"column:active;not null;default:true" json:"active"`
	SortOrder int    `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (PaymentMethod) TableName() string { return "payment_methods" }

// VoucherType adalah tipe potongan voucher.
type VoucherType string

const (
	// VoucherPercent adalah potongan persentase.
	VoucherPercent VoucherType = "percent"
	// VoucherFixed adalah potongan nominal tetap.
	VoucherFixed VoucherType = "fixed"
)

// Voucher adalah kode promo pada checkout.
type Voucher struct {
	Code        string      `gorm:"column:code;primaryKey" json:"code"`
	Label       string      `gorm:"column:label;not null;default:''" json:"label"`
	Type        VoucherType `gorm:"column:type;not null" json:"type"`
	Value       int64       `gorm:"column:value;not null;default:0" json:"value"`
	MinPurchase int64       `gorm:"column:min_purchase;not null;default:0" json:"minPurchase"`
	MaxUses     *int        `gorm:"column:max_uses" json:"maxUses,omitempty"`
	UsedCount   int         `gorm:"column:used_count;not null;default:0" json:"usedCount"`
	StartsAt    *time.Time  `gorm:"column:starts_at" json:"startsAt,omitempty"`
	EndsAt      *time.Time  `gorm:"column:ends_at" json:"endsAt,omitempty"`
	Active      bool        `gorm:"column:active;not null;default:true" json:"active"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (Voucher) TableName() string { return "vouchers" }
