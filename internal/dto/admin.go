package dto

/* --------------------------- manajemen konten --------------------------- */

// CreateQuestionRequest adalah payload tambah soal.
type CreateQuestionRequest struct {
	ID      string `json:"id" validate:"omitempty,max=64"`
	BankKey string `json:"bankKey" validate:"required,max=40"`
	// TryoutID wajib: satu soal dimiliki satu paket tryout.
	TryoutID string `json:"tryoutId" validate:"required,max=64"`
	// Dimensi taksonomi. Kosong berarti memakai nilai bawaan
	// (umum / tkd / umum), sehingga klien lama tetap bisa mengirim payload
	// tanpa kolom baru.
	SectorKey      string `json:"sectorKey" validate:"omitempty,max=40"`
	CompanyKey     string `json:"companyKey" validate:"omitempty,max=40"`
	TestType       string `json:"testType" validate:"omitempty,max=40"`
	PositionFamily string `json:"positionFamily" validate:"omitempty,max=40"`
	// VariantGroup menautkan soal ke grup varian (soal sekeluarga).
	VariantGroup string `json:"variantGroup" validate:"omitempty,max=80"`
	// RetentionUntil adalah batas masa simpan soal (format tanggal RFC3339).
	RetentionUntil string   `json:"retentionUntil" validate:"omitempty"`
	Text           string   `json:"text" validate:"required,min=5"`
	Options        []string `json:"options" validate:"required,min=2,max=6,dive,required"`
	AnswerIndex    int      `json:"answerIndex" validate:"gte=0,lte=5"`
	Explanation    string   `json:"explanation" validate:"required,min=5"`
	Difficulty     string   `json:"difficulty" validate:"omitempty,oneof=mudah sedang sulit"`
	Active         *bool    `json:"active"`
	// License adalah bukti hak pakai soal. Bila dikosongkan, soal dianggap
	// dibuat sendiri (scope `internal`) sehingga pengerjaan tryout tidak
	// terhalang; soal dari vendor wajib mengisi scope & nomor perjanjian.
	License *QuestionLicenseInput `json:"license" validate:"omitempty"`
}

// QuestionLicenseInput adalah data izin yang boleh dikirim bersama soal.
type QuestionLicenseInput struct {
	Scope          string   `json:"scope" validate:"omitempty,oneof=internal vendor public_domain client_owned"`
	Owner          string   `json:"owner" validate:"omitempty,max=160"`
	AgreementNo    string   `json:"agreementNo" validate:"omitempty,max=80"`
	AgreementURL   string   `json:"agreementUrl" validate:"omitempty,max=300"`
	CommercialUse  *bool    `json:"commercialUse"`
	SelectionUse   *bool    `json:"selectionUse"`
	AllowedSectors []string `json:"allowedSectors" validate:"omitempty,max=13,dive,max=40"`
	ValidUntil     string   `json:"validUntil" validate:"omitempty"`
	Notes          string   `json:"notes" validate:"omitempty,max=500"`
}

// UpdateQuestionRequest adalah payload ubah soal.
type UpdateQuestionRequest struct {
	BankKey        *string   `json:"bankKey" validate:"omitempty,max=40"`
	TryoutID       *string   `json:"tryoutId" validate:"omitempty,max=64"`
	SectorKey      *string   `json:"sectorKey" validate:"omitempty,max=40"`
	CompanyKey     *string   `json:"companyKey" validate:"omitempty,max=40"`
	TestType       *string   `json:"testType" validate:"omitempty,max=40"`
	PositionFamily *string   `json:"positionFamily" validate:"omitempty,max=40"`
	VariantGroup   *string   `json:"variantGroup" validate:"omitempty,max=80"`
	RetentionUntil *string   `json:"retentionUntil" validate:"omitempty"`
	Text           *string   `json:"text" validate:"omitempty,min=5"`
	Options        *[]string `json:"options" validate:"omitempty,min=2,max=6,dive,required"`
	AnswerIndex    *int      `json:"answerIndex" validate:"omitempty,gte=0,lte=5"`
	Explanation    *string   `json:"explanation" validate:"omitempty,min=5"`
	Difficulty     *string   `json:"difficulty" validate:"omitempty,oneof=mudah sedang sulit"`
	Active         *bool     `json:"active"`
}

// QuestionView adalah soal lengkap (khusus admin, termasuk kunci jawaban).
type QuestionView struct {
	ID          string   `json:"id"`
	BankKey     string   `json:"bankKey"`
	BankLabel   string   `json:"bankLabel,omitempty"`
	TryoutID    string   `json:"tryoutId,omitempty"`
	TryoutTitle string   `json:"tryoutTitle,omitempty"`
	Text        string   `json:"text"`
	Options     []string `json:"options"`
	AnswerIndex int      `json:"answerIndex"`
	Explanation string   `json:"explanation"`
	Difficulty  string   `json:"difficulty"`
	Active      bool     `json:"active"`
	// Dimensi taksonomi beserta labelnya agar panel admin tidak perlu
	// memetakan kunci sendiri.
	SectorKey           string `json:"sectorKey"`
	SectorLabel         string `json:"sectorLabel,omitempty"`
	CompanyKey          string `json:"companyKey,omitempty"`
	TestType            string `json:"testType"`
	TestTypeLabel       string `json:"testTypeLabel,omitempty"`
	PositionFamily      string `json:"positionFamily"`
	PositionFamilyLabel string `json:"positionFamilyLabel,omitempty"`
	VariantGroup        string `json:"variantGroup,omitempty"`
	RetentionUntil      string `json:"retentionUntil,omitempty"`
	// Ringkasan izin: panel bank soal menandai soal yang izinnya belum lengkap
	// tanpa perlu membuka halaman izin.
	LicenseStatus     string `json:"licenseStatus"`
	LicenseScope      string `json:"licenseScope,omitempty"`
	LicenseOwner      string `json:"licenseOwner,omitempty"`
	LicenseValidUntil string `json:"licenseValidUntil,omitempty"`
	LicenseCount      int    `json:"licenseCount"`
}

// SubtestInput adalah definisi subtes pada payload admin.
type SubtestInput struct {
	Name    string   `json:"name" validate:"required,max=120"`
	Minutes int      `json:"minutes" validate:"required,gt=0,lte=180"`
	Banks   []string `json:"banks" validate:"required,min=1,dive,required,max=40"`
}

// CreateTryoutRequest adalah payload tambah paket tryout.
type CreateTryoutRequest struct {
	ID              string         `json:"id" validate:"omitempty,max=64"`
	Title           string         `json:"title" validate:"required,min=3,max=180"`
	Description     string         `json:"description" validate:"omitempty,max=1000"`
	Tier            string         `json:"tier" validate:"required,oneof=Gratis Premium"`
	RequiredProduct string         `json:"requiredProduct" validate:"omitempty,max=80"`
	Attempts        int            `json:"attempts" validate:"required,gt=0,lte=100"`
	AccessUntil     string         `json:"accessUntil" validate:"omitempty,max=80"`
	Subtests        []SubtestInput `json:"subtests" validate:"required,min=1,dive"`
	Active          *bool          `json:"active"`
}

// UpdateTryoutRequest adalah payload ubah paket tryout.
type UpdateTryoutRequest struct {
	Title           *string        `json:"title" validate:"omitempty,min=3,max=180"`
	Description     *string        `json:"description" validate:"omitempty,max=1000"`
	Tier            *string        `json:"tier" validate:"omitempty,oneof=Gratis Premium"`
	RequiredProduct *string        `json:"requiredProduct" validate:"omitempty,max=80"`
	Attempts        *int           `json:"attempts" validate:"omitempty,gt=0,lte=100"`
	AccessUntil     *string        `json:"accessUntil" validate:"omitempty,max=80"`
	Subtests        []SubtestInput `json:"subtests" validate:"omitempty,min=1,dive"`
	Active          *bool          `json:"active"`
}

// CreateProductRequest adalah payload tambah produk.
type CreateProductRequest struct {
	Slug        string   `json:"slug" validate:"required,min=3,max=80"`
	Name        string   `json:"name" validate:"required,min=3,max=160"`
	Description string   `json:"description" validate:"omitempty,max=1000"`
	Price       int64    `json:"price" validate:"gte=0"`
	Type        string   `json:"type" validate:"required,oneof=program tryout"`
	Includes    []string `json:"includes" validate:"omitempty,max=20,dive,max=200"`
	Badge       string   `json:"badge" validate:"omitempty,max=40"`
	Active      *bool    `json:"active"`
}

// UpdateProductRequest adalah payload ubah produk.
type UpdateProductRequest struct {
	Name        *string   `json:"name" validate:"omitempty,min=3,max=160"`
	Description *string   `json:"description" validate:"omitempty,max=1000"`
	Price       *int64    `json:"price" validate:"omitempty,gte=0"`
	Type        *string   `json:"type" validate:"omitempty,oneof=program tryout"`
	Includes    *[]string `json:"includes" validate:"omitempty,max=20,dive,max=200"`
	Badge       *string   `json:"badge" validate:"omitempty,max=40"`
	Active      *bool     `json:"active"`
}

// CreateVoucherRequest adalah payload tambah voucher.
type CreateVoucherRequest struct {
	Code        string `json:"code" validate:"required,min=3,max=40"`
	Label       string `json:"label" validate:"omitempty,max=200"`
	Type        string `json:"type" validate:"required,oneof=percent fixed"`
	Value       int64  `json:"value" validate:"required,gt=0"`
	MinPurchase int64  `json:"minPurchase" validate:"gte=0"`
	MaxUses     *int   `json:"maxUses" validate:"omitempty,gt=0"`
	Active      *bool  `json:"active"`
}

// UpdateVoucherRequest adalah payload ubah voucher.
type UpdateVoucherRequest struct {
	Label       *string `json:"label" validate:"omitempty,max=200"`
	Type        *string `json:"type" validate:"omitempty,oneof=percent fixed"`
	Value       *int64  `json:"value" validate:"omitempty,gt=0"`
	MinPurchase *int64  `json:"minPurchase" validate:"omitempty,gte=0"`
	MaxUses     *int    `json:"maxUses" validate:"omitempty,gt=0"`
	Active      *bool   `json:"active"`
}

// ImportResult adalah hasil import soal dari Excel/CSV.
type ImportResult struct {
	TotalRows int `json:"totalRows"`
	Imported  int `json:"imported"`
	Updated   int `json:"updated"`
	Failed    int `json:"failed"`
	// Unassigned adalah jumlah baris yang belum ditautkan ke tryout mana pun
	// (kolom tryoutId kosong dan tidak ada tryout tujuan dari formulir).
	Unassigned int              `json:"unassigned"`
	Errors     []ImportRowError `json:"errors,omitempty"`
	BankKeys   map[string]int   `json:"bankKeys"`
	// Duplicates menandai baris hasil import yang mirip dengan soal lain di bank
	// soal (termasuk sesama baris pada berkas yang sama). Import tetap berjalan —
	// temuan ini bahan tinjauan tim akademik, bukan penghalang.
	Duplicates []DuplicateHint `json:"duplicates,omitempty"`
}

// Jenis temuan soal mirip.
const (
	// DuplicateIdentical berarti teks soal sama persis (setelah normalisasi).
	DuplicateIdentical = "identik"
	// DuplicateFamily berarti teks sama persis kecuali angka.
	DuplicateFamily = "keluarga"
	// DuplicateSimilar berarti teks cukup mirip (di atas ambang kemiripan).
	DuplicateSimilar = "mirip"
)

// DuplicateHint adalah satu temuan soal yang mirip dengan soal lain.
type DuplicateHint struct {
	// Row adalah nomor baris pada berkas import (0 bila bukan dari import).
	Row        int     `json:"row,omitempty"`
	QuestionID string  `json:"questionId"`
	OtherID    string  `json:"otherId"`
	OtherText  string  `json:"otherText"`
	Kind       string  `json:"kind"`
	Similarity float64 `json:"similarity"`
}

// ImportRowError adalah kesalahan pada satu baris berkas import.
type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// AdminUserView adalah data pengguna untuk dashboard admin.
type AdminUserView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Role         string   `json:"role"`
	Program      string   `json:"program"`
	ReferralCode string   `json:"referralCode,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	OrderCount   int64    `json:"orderCount"`
	PaidTotal    int64    `json:"paidTotal"`
	AttemptCount int64    `json:"attemptCount"`
	Enrollments  []string `json:"enrollments"`
}

// AdminOrderView adalah data pesanan untuk dashboard admin.
type AdminOrderView struct {
	OrderView
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
}

// BroadcastRequest adalah payload broadcast WhatsApp.
type BroadcastRequest struct {
	Audience string `json:"audience" validate:"required,oneof=all enrolled paid pending"`
	Message  string `json:"message" validate:"required,min=5,max=1000"`
}

// BroadcastResponse adalah balasan broadcast.
type BroadcastResponse struct {
	Audience   string `json:"audience"`
	Recipients int    `json:"recipients"`
	Status     string `json:"status"`
	Preview    string `json:"preview"`
}

// ProductSales adalah penjualan per produk.
type ProductSales struct {
	ProductSlug string `json:"productSlug"`
	ProductName string `json:"productName"`
	Orders      int64  `json:"orders"`
	Revenue     int64  `json:"revenue"`
}

// BucketCount adalah hitungan per rentang nilai.
type BucketCount struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// AnalyticsView adalah ringkasan analitik dashboard admin.
type AnalyticsView struct {
	TotalUsers        int64          `json:"totalUsers"`
	TotalPeserta      int64          `json:"totalPeserta"`
	TotalOrders       int64          `json:"totalOrders"`
	PaidOrders        int64          `json:"paidOrders"`
	PendingOrders     int64          `json:"pendingOrders"`
	Revenue           int64          `json:"revenue"`
	TotalAttempts     int64          `json:"totalAttempts"`
	AverageScore      float64        `json:"averageScore"`
	QuestionsTotal    int64          `json:"questionsTotal"`
	TryoutsTotal      int64          `json:"tryoutsTotal"`
	TopProducts       []ProductSales `json:"topProducts"`
	ScoreDistribution []BucketCount  `json:"scoreDistribution"`
}
