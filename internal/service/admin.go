package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
	"github.com/google/uuid"
)

// shortID menghasilkan id pendek acak (8 karakter heksadesimal) untuk soal
// yang dibuat tanpa id eksplisit.
func shortID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}

// AdminService menangani manajemen konten, pengguna, pesanan, broadcast, dan
// analitik untuk dashboard admin.
type AdminService struct {
	users       *repository.UserRepository
	orders      *repository.OrderRepository
	enrollments *repository.EnrollmentRepository
	catalog     *repository.CatalogRepository
	tryouts     *repository.TryoutRepository
	attempts    *repository.AttemptRepository
	learning    *repository.LearningRepository
	catalogSvc  *CatalogService
	tryoutSvc   *TryoutService
	cfg         *config.Config
}

// NewAdminService membentuk service admin.
func NewAdminService(
	users *repository.UserRepository,
	orders *repository.OrderRepository,
	enrollments *repository.EnrollmentRepository,
	catalog *repository.CatalogRepository,
	tryouts *repository.TryoutRepository,
	attempts *repository.AttemptRepository,
	learning *repository.LearningRepository,
	catalogSvc *CatalogService,
	tryoutSvc *TryoutService,
	cfg *config.Config,
) *AdminService {
	return &AdminService{
		users:       users,
		orders:      orders,
		enrollments: enrollments,
		catalog:     catalog,
		tryouts:     tryouts,
		attempts:    attempts,
		learning:    learning,
		catalogSvc:  catalogSvc,
		tryoutSvc:   tryoutSvc,
		cfg:         cfg,
	}
}

/* ------------------------------- analitik -------------------------------- */

// Analytics menyusun ringkasan analitik dashboard admin.
func (s *AdminService) Analytics(ctx context.Context) (*dto.AnalyticsView, error) {
	totalUsers, err := s.users.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	totalPeserta, err := s.users.CountByRole(ctx, models.RolePeserta)
	if err != nil {
		return nil, err
	}

	statusCounts, err := s.orders.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}
	revenue, err := s.orders.SumPaidRevenue(ctx)
	if err != nil {
		return nil, err
	}

	totalAttempts, err := s.attemptsCount(ctx)
	if err != nil {
		return nil, err
	}
	averageScore, err := s.attemptsAverage(ctx)
	if err != nil {
		return nil, err
	}

	questions, err := s.tryouts.CountQuestions(ctx)
	if err != nil {
		return nil, err
	}
	tryoutsTotal, err := s.tryouts.CountPackages(ctx)
	if err != nil {
		return nil, err
	}

	topProducts, err := s.orders.TopProducts(ctx, 5)
	if err != nil {
		return nil, err
	}
	productSales := make([]dto.ProductSales, 0, len(topProducts))
	for _, row := range topProducts {
		productSales = append(productSales, dto.ProductSales{
			ProductSlug: row.ProductSlug,
			ProductName: row.ProductName,
			Orders:      row.Orders,
			Revenue:     row.Revenue,
		})
	}

	distribution, err := s.attemptsDistribution(ctx)
	if err != nil {
		return nil, err
	}
	buckets := make([]dto.BucketCount, 0, len(distribution))
	for _, row := range distribution {
		buckets = append(buckets, dto.BucketCount{Label: row.Label, Count: row.Count})
	}

	totalOrders := int64(0)
	for _, count := range statusCounts {
		totalOrders += count
	}

	return &dto.AnalyticsView{
		TotalUsers:        totalUsers,
		TotalPeserta:      totalPeserta,
		TotalOrders:       totalOrders,
		PaidOrders:        statusCounts[models.OrderPaid],
		PendingOrders:     statusCounts[models.OrderPending],
		Revenue:           revenue,
		TotalAttempts:     totalAttempts,
		AverageScore:      averageScore,
		QuestionsTotal:    questions,
		TryoutsTotal:      tryoutsTotal,
		TopProducts:       productSales,
		ScoreDistribution: buckets,
	}, nil
}

func (s *AdminService) attemptsCount(ctx context.Context) (int64, error) {
	return s.attempts.CountAll(ctx)
}

func (s *AdminService) attemptsAverage(ctx context.Context) (float64, error) {
	return s.attempts.AverageScore(ctx)
}

func (s *AdminService) attemptsDistribution(ctx context.Context) ([]repository.ScoreBucket, error) {
	return s.attempts.ScoreDistribution(ctx)
}

/* -------------------------------- pengguna -------------------------------- */

// ListUsers mengambil daftar pengguna beserta ringkasan aktivitasnya.
func (s *AdminService) ListUsers(ctx context.Context, filter repository.UserFilter, page, perPage int) ([]dto.AdminUserView, int64, error) {
	users, total, err := s.users.List(ctx, filter, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]string, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}

	enrollments, err := s.enrollments.SlugsByUserIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.AdminUserView, 0, len(users))
	for _, user := range users {
		orderCount, paidTotal, err := s.userOrderStats(ctx, user.ID)
		if err != nil {
			return nil, 0, err
		}
		attemptCount, err := s.attempts.CountByUser(ctx, user.ID)
		if err != nil {
			return nil, 0, err
		}

		out = append(out, dto.AdminUserView{
			ID:           user.ID,
			Name:         user.Name,
			Email:        user.Email,
			Role:         string(user.Role),
			Program:      user.Program,
			ReferralCode: strPtrOrEmpty(user.ReferralCode),
			CreatedAt:    formatTime(user.CreatedAt),
			OrderCount:   orderCount,
			PaidTotal:    paidTotal,
			AttemptCount: attemptCount,
			Enrollments:  emptyIfNil(enrollments[user.ID]),
		})
	}
	return out, total, nil
}

func (s *AdminService) userOrderStats(ctx context.Context, userID string) (int64, int64, error) {
	type row struct {
		Total int64 `gorm:"column:total"`
		Paid  int64 `gorm:"column:paid"`
	}
	var stats row
	err := s.orders.DB().WithContext(ctx).Model(&models.Order{}).
		Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN status = 'paid' THEN total ELSE 0 END), 0) AS paid").
		Where("user_id = ?", userID).
		Scan(&stats).Error
	if err != nil {
		return 0, 0, apperror.Internal(err, "Gagal menghitung pesanan pengguna.")
	}
	return stats.Total, stats.Paid, nil
}

/* --------------------------------- pesanan -------------------------------- */

// ListOrders mengambil seluruh pesanan untuk dashboard admin.
func (s *AdminService) ListOrders(ctx context.Context, filter repository.OrderFilter, page, perPage int) ([]dto.AdminOrderView, int64, error) {
	orders, users, total, err := s.orders.ListAll(ctx, filter, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.AdminOrderView, 0, len(orders))
	for _, order := range orders {
		view := dto.AdminOrderView{OrderView: toOrderView(order)}
		if user, ok := users[order.UserID]; ok {
			view.UserName = user.Name
			view.UserEmail = user.Email
		}
		out = append(out, view)
	}
	return out, total, nil
}

// MarkOrderPaid menandai pesanan lunas dari dashboard admin (mis. pembayaran
// manual yang sudah diverifikasi) sekaligus memberi akses produk.
//
// Referensi pembayaran dibuat unik per pesanan karena kolom
// `orders.payment_reference` memiliki indeks unik parsial — referensi tetap
// ("admin-manual") akan membuat penandaan lunas kedua gagal.
func (s *AdminService) MarkOrderPaid(ctx context.Context, orderID string) (*dto.OrderView, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	paid, err := s.orders.MarkPaid(ctx, orderID, "admin-manual-"+order.InvoiceNo)
	if err != nil {
		return nil, err
	}
	if _, err := s.enrollments.Grant(ctx, paid.UserID, paid.ProductSlug, &paid.ID, "admin"); err != nil {
		return nil, err
	}
	view := toOrderView(*paid)
	return &view, nil
}

/* ---------------------------------- soal ---------------------------------- */

// ListQuestions mengambil daftar soal untuk dashboard admin.
//
// Setiap soal disertai judul tryout pemiliknya agar panel bank soal bisa
// dikelompokkan per tryout tanpa permintaan tambahan dari klien.
func (s *AdminService) ListQuestions(ctx context.Context, filter repository.QuestionFilter, page, perPage int) ([]dto.QuestionView, int64, error) {
	rows, total, err := s.tryouts.ListQuestions(ctx, filter, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	titles, err := s.tryoutTitles(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.QuestionView, 0, len(rows))
	for _, row := range rows {
		view := toQuestionView(row)
		if row.TryoutID != nil {
			view.TryoutTitle = titles[*row.TryoutID]
		}
		out = append(out, view)
	}
	return out, total, nil
}

// tryoutTitles memetakan id paket tryout → judulnya (dipakai panel bank soal
// untuk menampilkan pengelompokan per tryout).
func (s *AdminService) tryoutTitles(ctx context.Context) (map[string]string, error) {
	packages, err := s.tryouts.ListPackages(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(packages))
	for _, pkg := range packages {
		out[pkg.ID] = pkg.Title
	}
	return out, nil
}

// ensureTryoutExists memastikan paket tryout yang dikirim admin benar-benar ada,
// karena satu soal wajib dimiliki satu tryout. Id beserta judulnya dikembalikan
// agar balasan create/update bisa langsung menampilkan pengelompokannya.
func (s *AdminService) ensureTryoutExists(ctx context.Context, tryoutID string) (string, string, error) {
	id := strings.TrimSpace(tryoutID)
	if id == "" {
		return "", "", apperror.Validation("Tryout soal wajib dipilih.", nil)
	}
	pkg, err := s.tryouts.FindPackage(ctx, id)
	if err != nil {
		return "", "", err
	}
	return id, pkg.Title, nil
}

// CreateQuestion menambahkan satu soal baru.
func (s *AdminService) CreateQuestion(ctx context.Context, req dto.CreateQuestionRequest) (*dto.QuestionView, error) {
	if err := validateQuestion(req.Options, req.AnswerIndex); err != nil {
		return nil, err
	}

	tryoutID, tryoutTitle, err := s.ensureTryoutExists(ctx, req.TryoutID)
	if err != nil {
		return nil, err
	}

	position, err := s.tryouts.NextQuestionPosition(ctx, tryoutID, req.BankKey)
	if err != nil {
		return nil, err
	}

	question := models.Question{
		ID:          strings.TrimSpace(req.ID),
		BankKey:     req.BankKey,
		TryoutID:    &tryoutID,
		Text:        req.Text,
		Options:     models.StringList(req.Options),
		AnswerIndex: req.AnswerIndex,
		Explanation: req.Explanation,
		Difficulty:  difficultyOr(req.Difficulty),
		Position:    position,
		Active:      req.Active == nil || *req.Active,
	}
	if question.ID == "" {
		question.ID = fmt.Sprintf("%s-%s", req.BankKey, shortID())
	}

	if err := s.tryouts.CreateQuestion(ctx, &question); err != nil {
		return nil, err
	}
	s.tryoutSvc.InvalidateBankCache(ctx)

	view := toQuestionView(question)
	view.TryoutTitle = tryoutTitle
	return &view, nil
}

// UpdateQuestion mengubah sebagian data soal.
func (s *AdminService) UpdateQuestion(ctx context.Context, id string, req dto.UpdateQuestionRequest) (*dto.QuestionView, error) {
	question, err := s.tryouts.FindQuestion(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.BankKey != nil {
		question.BankKey = *req.BankKey
	}
	if req.TryoutID != nil {
		tryoutID, _, err := s.ensureTryoutExists(ctx, *req.TryoutID)
		if err != nil {
			return nil, err
		}
		question.TryoutID = &tryoutID
	}
	if req.Text != nil {
		question.Text = *req.Text
	}
	if req.Options != nil {
		question.Options = models.StringList(*req.Options)
	}
	if req.AnswerIndex != nil {
		question.AnswerIndex = *req.AnswerIndex
	}
	if req.Explanation != nil {
		question.Explanation = *req.Explanation
	}
	if req.Difficulty != nil {
		question.Difficulty = models.Difficulty(*req.Difficulty)
	}
	if req.Active != nil {
		question.Active = *req.Active
	}

	if err := validateQuestion(question.Options, question.AnswerIndex); err != nil {
		return nil, err
	}

	if err := s.tryouts.UpdateQuestion(ctx, question); err != nil {
		return nil, err
	}
	s.tryoutSvc.InvalidateBankCache(ctx)

	view := toQuestionView(*question)
	return &view, nil
}

// DeleteQuestion menghapus soal.
func (s *AdminService) DeleteQuestion(ctx context.Context, id string) error {
	if err := s.tryouts.DeleteQuestion(ctx, id); err != nil {
		return err
	}
	s.tryoutSvc.InvalidateBankCache(ctx)
	return nil
}

// validateQuestion memastikan kunci jawaban berada di dalam rentang opsi.
func validateQuestion(options []string, answerIndex int) error {
	if len(options) < 2 {
		return apperror.Validation("Soal minimal punya dua pilihan jawaban.", nil)
	}
	if answerIndex < 0 || answerIndex >= len(options) {
		return apperror.Validation("Indeks jawaban benar di luar rentang pilihan.", nil)
	}
	return nil
}

func difficultyOr(value string) models.Difficulty {
	if value == "" {
		return models.DifficultySedang
	}
	return models.Difficulty(value)
}

/* ------------------------------ paket tryout ------------------------------ */

// UpsertTryout membuat atau memperbarui paket tryout beserta subtesnya.
func (s *AdminService) UpsertTryout(ctx context.Context, req dto.CreateTryoutRequest, id string) (*dto.TryoutDetail, error) {
	pkgID := strings.TrimSpace(id)
	if pkgID == "" {
		pkgID = strings.TrimSpace(req.ID)
	}
	if pkgID == "" {
		pkgID = "to-" + shortID()
	}

	for _, subtest := range req.Subtests {
		for _, bankKey := range subtest.Banks {
			if !bank.IsValid(bankKey) {
				return nil, apperror.Validation("Kategori soal tidak dikenal: "+bankKey, nil)
			}
		}
	}

	accessUntil := req.AccessUntil
	if accessUntil == "" {
		accessUntil = "Tanpa batas"
	}

	pkg := models.TryoutPackage{
		ID:              pkgID,
		Title:           req.Title,
		Description:     req.Description,
		Tier:            models.Tier(req.Tier),
		RequiredProduct: optionalString(req.RequiredProduct),
		Attempts:        req.Attempts,
		AccessUntil:     accessUntil,
		Active:          req.Active == nil || *req.Active,
	}

	subtests := make([]models.Subtest, 0, len(req.Subtests))
	for index, subtest := range req.Subtests {
		subtests = append(subtests, models.Subtest{
			TryoutID:  pkgID,
			Name:      subtest.Name,
			Minutes:   subtest.Minutes,
			Banks:     models.StringList(subtest.Banks),
			SortOrder: index,
		})
	}

	if err := s.tryouts.UpsertPackage(ctx, &pkg, subtests); err != nil {
		return nil, err
	}
	s.tryoutSvc.InvalidateBankCache(ctx)

	return s.tryoutSvc.Get(ctx, "", pkgID, true)
}

// UpdateTryout mengubah sebagian data paket tryout.
func (s *AdminService) UpdateTryout(ctx context.Context, id string, req dto.UpdateTryoutRequest) (*dto.TryoutDetail, error) {
	pkg, err := s.tryouts.FindPackage(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		pkg.Title = *req.Title
	}
	if req.Description != nil {
		pkg.Description = *req.Description
	}
	if req.Tier != nil {
		pkg.Tier = models.Tier(*req.Tier)
	}
	if req.RequiredProduct != nil {
		pkg.RequiredProduct = optionalString(*req.RequiredProduct)
	}
	if req.Attempts != nil {
		pkg.Attempts = *req.Attempts
	}
	if req.AccessUntil != nil {
		pkg.AccessUntil = *req.AccessUntil
	}
	if req.Active != nil {
		pkg.Active = *req.Active
	}

	subtests := pkg.Subtests
	if len(req.Subtests) > 0 {
		subtests = make([]models.Subtest, 0, len(req.Subtests))
		for index, subtest := range req.Subtests {
			for _, bankKey := range subtest.Banks {
				if !bank.IsValid(bankKey) {
					return nil, apperror.Validation("Kategori soal tidak dikenal: "+bankKey, nil)
				}
			}
			subtests = append(subtests, models.Subtest{
				TryoutID:  pkg.ID,
				Name:      subtest.Name,
				Minutes:   subtest.Minutes,
				Banks:     models.StringList(subtest.Banks),
				SortOrder: index,
			})
		}
	}

	if err := s.tryouts.UpsertPackage(ctx, pkg, subtests); err != nil {
		return nil, err
	}
	s.tryoutSvc.InvalidateBankCache(ctx)

	return s.tryoutSvc.Get(ctx, "", pkg.ID, true)
}

// DeleteTryout menghapus paket tryout.
func (s *AdminService) DeleteTryout(ctx context.Context, id string) error {
	if err := s.tryouts.DeletePackage(ctx, id); err != nil {
		return err
	}
	s.tryoutSvc.InvalidateBankCache(ctx)
	return nil
}

/* --------------------------------- produk --------------------------------- */

// CreateProduct menambahkan produk baru.
func (s *AdminService) CreateProduct(ctx context.Context, req dto.CreateProductRequest) (*dto.ProductView, error) {
	product := models.Product{
		Slug:        strings.ToLower(strings.TrimSpace(req.Slug)),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Type:        models.ProductType(req.Type),
		Includes:    models.StringList(req.Includes),
		Badge:       optionalString(req.Badge),
		Active:      req.Active == nil || *req.Active,
	}
	if err := s.catalog.CreateProduct(ctx, &product); err != nil {
		return nil, err
	}
	s.catalogSvc.Invalidate(ctx)

	view := toProductView(product, false)
	return &view, nil
}

// UpdateProduct mengubah sebagian data produk.
func (s *AdminService) UpdateProduct(ctx context.Context, slug string, req dto.UpdateProductRequest) (*dto.ProductView, error) {
	product, err := s.catalog.FindProductBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.Price != nil {
		product.Price = *req.Price
	}
	if req.Type != nil {
		product.Type = models.ProductType(*req.Type)
	}
	if req.Includes != nil {
		product.Includes = models.StringList(*req.Includes)
	}
	if req.Badge != nil {
		product.Badge = optionalString(*req.Badge)
	}
	if req.Active != nil {
		product.Active = *req.Active
	}

	if err := s.catalog.UpdateProduct(ctx, product); err != nil {
		return nil, err
	}
	s.catalogSvc.Invalidate(ctx)

	view := toProductView(*product, false)
	return &view, nil
}

// DeleteProduct menghapus produk.
func (s *AdminService) DeleteProduct(ctx context.Context, slug string) error {
	if err := s.catalog.DeleteProduct(ctx, slug); err != nil {
		return err
	}
	s.catalogSvc.Invalidate(ctx)
	return nil
}

/* -------------------------------- voucher -------------------------------- */

// CreateVoucher menambahkan voucher baru.
func (s *AdminService) CreateVoucher(ctx context.Context, req dto.CreateVoucherRequest) (*dto.VoucherView, error) {
	if req.Type == string(models.VoucherPercent) && req.Value > 100 {
		return nil, apperror.Validation("Nilai voucher persentase tidak boleh lebih dari 100.", nil)
	}

	voucher := models.Voucher{
		Code:        strings.ToUpper(strings.TrimSpace(req.Code)),
		Label:       req.Label,
		Type:        models.VoucherType(req.Type),
		Value:       req.Value,
		MinPurchase: req.MinPurchase,
		MaxUses:     req.MaxUses,
		Active:      req.Active == nil || *req.Active,
	}
	if err := s.catalog.UpsertVoucher(ctx, &voucher); err != nil {
		return nil, err
	}
	view := toVoucherView(voucher)
	return &view, nil
}

// UpdateVoucher mengubah sebagian data voucher.
func (s *AdminService) UpdateVoucher(ctx context.Context, code string, req dto.UpdateVoucherRequest) (*dto.VoucherView, error) {
	voucher, err := s.catalog.FindVoucher(ctx, code)
	if err != nil {
		return nil, err
	}

	if req.Label != nil {
		voucher.Label = *req.Label
	}
	if req.Type != nil {
		voucher.Type = models.VoucherType(*req.Type)
	}
	if req.Value != nil {
		voucher.Value = *req.Value
	}
	if req.MinPurchase != nil {
		voucher.MinPurchase = *req.MinPurchase
	}
	if req.MaxUses != nil {
		voucher.MaxUses = req.MaxUses
	}
	if req.Active != nil {
		voucher.Active = *req.Active
	}

	if voucher.Type == models.VoucherPercent && voucher.Value > 100 {
		return nil, apperror.Validation("Nilai voucher persentase tidak boleh lebih dari 100.", nil)
	}

	if err := s.catalog.UpdateVoucher(ctx, voucher); err != nil {
		return nil, err
	}
	view := toVoucherView(*voucher)
	return &view, nil
}

// DeleteVoucher menghapus voucher.
func (s *AdminService) DeleteVoucher(ctx context.Context, code string) error {
	return s.catalog.DeleteVoucher(ctx, code)
}

/* -------------------------------- broadcast ------------------------------- */

// Broadcast mencatat broadcast WhatsApp ke segmen peserta tertentu.
//
// Pengiriman nyata dilakukan worker terpisah; di sini kita hanya menyiapkan
// daftar penerima dan mencatat log agar tidak terkirim dobel.
func (s *AdminService) Broadcast(ctx context.Context, adminID string, req dto.BroadcastRequest) (*dto.BroadcastResponse, error) {
	recipients, err := s.enrollments.UserIDsByAudience(ctx, req.Audience)
	if err != nil {
		return nil, err
	}

	log := models.BroadcastLog{
		CreatedBy:  &adminID,
		Audience:   req.Audience,
		Message:    req.Message,
		Recipients: len(recipients),
		Status:     "queued",
	}
	if err := s.learning.CreateBroadcastLog(ctx, &log); err != nil {
		return nil, err
	}

	return &dto.BroadcastResponse{
		Audience:   req.Audience,
		Recipients: len(recipients),
		Status:     log.Status,
		Preview:    preview(req.Message, 120),
	}, nil
}

// ListBroadcasts mengambil riwayat broadcast.
func (s *AdminService) ListBroadcasts(ctx context.Context, limit int) ([]models.BroadcastLog, error) {
	return s.learning.ListBroadcastLogs(ctx, limit)
}

/* --------------------------------- helper --------------------------------- */

// optionalString mengubah string kosong menjadi nil (kolom NULL).
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func preview(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
