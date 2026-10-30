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
)

// pathMaxSteps membatasi jumlah langkah satu jalur belajar.
const pathMaxSteps = 50

// LearningPathService menjalankan jalur belajar & kuis per pelajaran.
//
// Aturan yang dipegang service ini:
//   - akses materi lewat AccessChecker (kepemilikan produk atau materi contoh);
//   - soal kuis diambil dari bank soal dengan **izin aktif** (penegakan Fase 0),
//     dan kunci jawaban tidak pernah dikirim sebelum kuis dikumpulkan;
//   - percobaan yang belum dikumpulkan tidak dihitung terhadap batas percobaan.
type LearningPathService struct {
	paths     *repository.LearningPathRepository
	classroom *repository.ClassroomRepository
	catalog   *repository.CatalogRepository
	tryouts   *repository.TryoutRepository
	licenses  *repository.LicenseRepository
	access    *AccessChecker
	cfg       *config.Config
	// certificates menerbitkan sertifikat saat kuis terakhir sebuah jalur lulus
	// (opsional; diisi wiring lewat SetCertificateIssuer).
	certificates *CertificateService
}

// NewLearningPathService membentuk service jalur belajar.
func NewLearningPathService(
	paths *repository.LearningPathRepository,
	classroom *repository.ClassroomRepository,
	catalog *repository.CatalogRepository,
	tryouts *repository.TryoutRepository,
	licenses *repository.LicenseRepository,
	access *AccessChecker,
	cfg *config.Config,
) *LearningPathService {
	return &LearningPathService{
		paths:     paths,
		classroom: classroom,
		catalog:   catalog,
		tryouts:   tryouts,
		licenses:  licenses,
		access:    access,
		cfg:       cfg,
	}
}

// SetCertificateIssuer memasang penerbit sertifikat untuk jalur belajar.
//
// Dipisah dari konstruktor agar paket uji bisa memakai service ini tanpa
// sertifikat dan agar tidak ada ketergantungan melingkar antara service.
func (s *LearningPathService) SetCertificateIssuer(issuer *CertificateService) {
	s.certificates = issuer
}

/* ------------------------------ jalur belajar ------------------------------ */

// Paths mengembalikan jalur belajar aktif yang bisa diakses peserta.
//
// Jalur yang produknya tidak dimiliki peserta tidak ditampilkan — konsisten
// dengan daftar ruang kelas pada `/classroom`, sehingga peserta tidak melihat
// kurikulum yang tidak bisa ia buka.
func (s *LearningPathService) Paths(ctx context.Context, userID string) ([]dto.PathSummaryView, error) {
	rows, err := s.paths.ListPaths(ctx, repository.PathFilter{ActiveOnly: true})
	if err != nil {
		return nil, err
	}

	owned, err := s.access.OwnedProducts(ctx, userID)
	if err != nil {
		return nil, err
	}

	visible := make([]models.LearningPath, 0, len(rows))
	for _, path := range rows {
		if _, ok := owned[path.ProductSlug]; !ok {
			continue
		}
		visible = append(visible, path)
	}
	return s.summaries(ctx, userID, visible)
}

// Path mengembalikan detail satu jalur beserta langkah & progresnya.
func (s *LearningPathService) Path(ctx context.Context, userID, slug string) (*dto.PathDetailView, error) {
	path, err := s.paths.FindPathBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !path.Active {
		return nil, apperror.NotFound("Jalur belajar tidak ditemukan.")
	}
	// Akses jalur mengikuti kepemilikan produknya: kurikulum kelas khusus tidak
	// boleh terbaca oleh peserta yang belum terdaftar di produk itu.
	if err := s.access.RequireProduct(ctx, userID, path.ProductSlug); err != nil {
		return nil, err
	}

	stats, err := s.moduleStats(ctx, userID, []models.LearningPath{*path})
	if err != nil {
		return nil, err
	}
	modules, err := s.modulesOf(ctx, path.Steps)
	if err != nil {
		return nil, err
	}

	summary := s.summaryView(*path, stats)
	detail := &dto.PathDetailView{
		PathSummaryView: summary,
		Steps:           make([]dto.PathStepView, 0, len(path.Steps)),
	}

	for _, step := range path.Steps {
		view := s.stepView(step, stats[step.ModuleID], modules[step.ModuleID])
		detail.Steps = append(detail.Steps, view)
		if detail.NextStepModuleID == "" && !view.Completed {
			detail.NextStepModuleID = step.ModuleID
		}
	}
	// Kelayakan sertifikat memakai aturan yang sama dengan penerbitannya (langkah
	// wajib + seluruh kuis wajib lulus). Pemeriksaan hanya dijalankan bila jalur
	// memang menerbitkan sertifikat dan seluruh langkah wajib sudah selesai, agar
	// tampilan jalur yang masih berjalan tidak menambah query kuis.
	if path.Certificate && summary.ProgressPercent >= 100 && s.certificates != nil {
		eligibility, err := s.certificates.Eligibility(ctx, userID, path)
		if err != nil {
			return nil, err
		}
		detail.EligibleCertificate = eligibility.Eligible && !eligibility.Issued
		detail.Certificate = eligibility.Certificate
		// Alasan belum layak dibuka ke peserta (mis. kuis yang belum lulus),
		// supaya sertifikat yang "tidak muncul" bisa ditindaklanjuti.
		detail.CertificateEligibility = eligibility
	}

	return detail, nil
}

// summaries menghitung ringkasan banyak jalur dengan satu query progres.
func (s *LearningPathService) summaries(ctx context.Context, userID string, rows []models.LearningPath) ([]dto.PathSummaryView, error) {
	stats, err := s.moduleStats(ctx, userID, rows)
	if err != nil {
		return nil, err
	}

	out := make([]dto.PathSummaryView, 0, len(rows))
	for _, path := range rows {
		out = append(out, s.summaryView(path, stats))
	}
	return out, nil
}

// moduleStats memuat progres peserta untuk seluruh modul yang dipakai jalur.
func (s *LearningPathService) moduleStats(ctx context.Context, userID string, rows []models.LearningPath) (map[string]repository.LessonStat, error) {
	// Tanpa peserta (mis. tampilan admin) tidak ada progres yang bisa dihitung.
	// Penjagaan ini juga mencegah query dengan `user_id = ''` pada kolom uuid.
	if strings.TrimSpace(userID) == "" {
		return map[string]repository.LessonStat{}, nil
	}

	seen := map[string]struct{}{}
	ids := make([]string, 0, 16)
	for _, path := range rows {
		for _, step := range path.Steps {
			if _, ok := seen[step.ModuleID]; ok {
				continue
			}
			seen[step.ModuleID] = struct{}{}
			ids = append(ids, step.ModuleID)
		}
	}
	return s.paths.ModuleLessonStats(ctx, userID, ids)
}

// summaryView membentuk ringkasan satu jalur dari statistik modulnya.
func (s *LearningPathService) summaryView(path models.LearningPath, stats map[string]repository.LessonStat) dto.PathSummaryView {
	counts := countPathSteps(path, stats)
	return dto.PathSummaryView{
		ID:                 path.ID,
		Slug:               path.Slug,
		Title:              path.Title,
		Description:        path.Description,
		SectorKey:          path.SectorKey,
		SectorLabel:        bank.SectorLabel(path.SectorKey),
		TestType:           path.TestType,
		TestTypeLabel:      bank.TestTypeLabel(path.TestType),
		ProductSlug:        path.ProductSlug,
		CertificateEnabled: path.Certificate,
		StepCount:          counts.total,
		RequiredStepCount:  counts.required,
		CompletedSteps:     counts.completed,
		ProgressPercent:    pathProgressPercent(counts),
		Active:             path.Active,
	}
}

// stepView membentuk tampilan satu langkah jalur.
func (s *LearningPathService) stepView(step models.LearningPathStep, stat repository.LessonStat, module *models.CourseModule) dto.PathStepView {
	view := dto.PathStepView{
		ModuleID:         step.ModuleID,
		Required:         step.Required,
		SortOrder:        step.SortOrder,
		LessonCount:      stat.Total,
		CompletedLessons: stat.Completed,
		ProgressPercent:  percentOf(stat.Completed, stat.Total),
		Completed:        moduleCompleted(stat),
	}
	if module != nil {
		view.Title = module.Title
		view.Description = module.Description
	}
	return view
}

// modulesOf memuat modul yang dipakai langkah jalur.
//
// Modul yang sudah dihapus tidak menggagalkan seluruh halaman: langkahnya tampil
// tanpa judul dan tetap terlihat agar bisa ditinjau tim akademik.
func (s *LearningPathService) modulesOf(ctx context.Context, steps []models.LearningPathStep) (map[string]*models.CourseModule, error) {
	out := make(map[string]*models.CourseModule, len(steps))
	for _, step := range steps {
		if _, ok := out[step.ModuleID]; ok {
			continue
		}
		module, err := s.classroom.FindModule(ctx, step.ModuleID)
		if err != nil {
			out[step.ModuleID] = nil
			continue
		}
		out[step.ModuleID] = module
	}
	return out, nil
}

// pathCounts adalah hitungan langkah jalur untuk perhitungan progres.
type pathCounts struct {
	required          int
	completedRequired int
	total             int
	completed         int
}

// pathCounts menghitung langkah wajib/seluruhnya beserta yang sudah selesai.
func countPathSteps(path models.LearningPath, stats map[string]repository.LessonStat) pathCounts {
	var out pathCounts
	for _, step := range path.Steps {
		done := moduleCompleted(stats[step.ModuleID])

		out.total++
		if done {
			out.completed++
		}
		if step.Required {
			out.required++
			if done {
				out.completedRequired++
			}
		}
	}
	return out
}

// moduleCompleted menandai seluruh pelajaran aktif sebuah modul sudah selesai.
//
// Modul tanpa pelajaran aktif tidak dianggap selesai (tidak ada yang dipelajari),
// sehingga jalur kosong tidak pernah dianggap tuntas.
func moduleCompleted(stat repository.LessonStat) bool {
	return stat.Total > 0 && stat.Completed >= stat.Total
}

// pathProgressPercent menghitung persentase progres jalur.
//
// Basisnya langkah wajib; bila jalur tidak punya langkah wajib, seluruh langkah
// dipakai sebagai basis agar angkanya tetap bermakna. Pembagi nol menghasilkan 0.
func pathProgressPercent(counts pathCounts) int {
	if counts.required > 0 {
		return percentOf(counts.completedRequired, counts.required)
	}
	return percentOf(counts.completed, counts.total)
}

// percentOf menghitung persentase dengan aman (nol bila pembagi nol).
func percentOf(part, total int) int {
	if total <= 0 || part <= 0 {
		return 0
	}
	value := int(float64(part) / float64(total) * 100)
	if value > 100 {
		return 100
	}
	return value
}

// ensureProduct memastikan produk jalur benar-benar ada.
func (s *LearningPathService) ensureProduct(ctx context.Context, productSlug string) error {
	slug := strings.TrimSpace(productSlug)
	if slug == "" {
		return apperror.Validation("Produk jalur wajib dipilih.", nil)
	}
	if _, err := s.catalog.FindProductBySlug(ctx, slug); err != nil {
		return err
	}
	return nil
}

// pathStepsFrom membentuk langkah jalur dari payload admin.
//
// Semua modul wajib berasal dari produk yang sama dengan jalurnya, sehingga
// kurikulum tidak bisa mencampur materi milik produk lain (yang aturan aksesnya
// berbeda). Duplikasi modul ditolak agar progres tidak dihitung dua kali.
func (s *LearningPathService) pathStepsFrom(ctx context.Context, productSlug string, inputs []dto.PathStepInput) ([]models.LearningPathStep, error) {
	if len(inputs) > pathMaxSteps {
		return nil, apperror.Validation(
			fmt.Sprintf("Jalur maksimal %d langkah.", pathMaxSteps), nil)
	}
	if len(inputs) == 0 {
		return nil, nil
	}

	modules, err := s.classroom.ListModules(ctx, productSlug, true)
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(modules))
	for _, module := range modules {
		known[module.ID] = struct{}{}
	}

	seen := map[string]struct{}{}
	steps := make([]models.LearningPathStep, 0, len(inputs))
	for index, input := range inputs {
		moduleID := strings.TrimSpace(input.ModuleID)
		if moduleID == "" {
			return nil, apperror.Validation("Setiap langkah wajib memilih modul.", nil)
		}
		if _, ok := known[moduleID]; !ok {
			return nil, apperror.Validation(
				fmt.Sprintf("Modul %q tidak ada pada produk %q.", moduleID, productSlug),
				map[string]string{"steps": "Pilih modul dari produk yang sama dengan jalur."},
			)
		}
		if _, duplicate := seen[moduleID]; duplicate {
			return nil, apperror.Validation(
				fmt.Sprintf("Modul %q muncul lebih dari sekali pada jalur.", moduleID), nil)
		}
		seen[moduleID] = struct{}{}

		steps = append(steps, models.LearningPathStep{
			ModuleID:  moduleID,
			Required:  input.Required == nil || *input.Required,
			SortOrder: index,
		})
	}
	return steps, nil
}

/* --------------------------- jalur belajar (admin) ------------------------- */

// AdminPaths mengambil daftar jalur untuk panel admin (termasuk yang nonaktif).
//
// Progres peserta tidak dihitung di sini: angka itu milik peserta, bukan admin.
func (s *LearningPathService) AdminPaths(ctx context.Context, filter repository.PathFilter) ([]dto.PathSummaryView, error) {
	rows, err := s.paths.ListPaths(ctx, filter)
	if err != nil {
		return nil, err
	}

	empty := map[string]repository.LessonStat{}
	out := make([]dto.PathSummaryView, 0, len(rows))
	for _, path := range rows {
		out = append(out, s.summaryView(path, empty))
	}
	return out, nil
}

// AdminPath mengambil detail satu jalur untuk panel admin (termasuk langkahnya).
//
// Panel admin memakai ini untuk menyunting susunan langkah; endpoint peserta
// (`GET /learning/paths/{slug}`) tidak bisa dipakai admin karena terikat aturan
// kepemilikan produk.
func (s *LearningPathService) AdminPath(ctx context.Context, id string) (*dto.PathDetailView, error) {
	path, err := s.paths.FindPath(ctx, id)
	if err != nil {
		return nil, err
	}

	modules, err := s.modulesOf(ctx, path.Steps)
	if err != nil {
		return nil, err
	}

	empty := map[string]repository.LessonStat{}
	detail := &dto.PathDetailView{
		PathSummaryView: s.summaryView(*path, empty),
		Steps:           make([]dto.PathStepView, 0, len(path.Steps)),
	}
	for _, step := range path.Steps {
		detail.Steps = append(detail.Steps, s.stepView(step, empty[step.ModuleID], modules[step.ModuleID]))
	}
	return detail, nil
}

// CreatePath menambah jalur belajar baru.
func (s *LearningPathService) CreatePath(ctx context.Context, req dto.CreatePathRequest) (*dto.PathSummaryView, error) {
	pathID := strings.TrimSpace(req.ID)
	if pathID != "" {
		// ID yang sudah dipakai ditolak agar pembuatan ulang tidak diam-diam
		// menimpa jalur yang sedang berjalan.
		if _, err := s.paths.FindPath(ctx, pathID); err == nil {
			return nil, apperror.Conflict("ID jalur belajar sudah dipakai.")
		} else if !isNotFound(err) {
			return nil, err
		}
	} else {
		pathID = models.NewID()
	}

	path, err := s.buildPath(ctx, req.Slug, req.Title, req.Description, req.SectorKey,
		req.TestType, req.ProductSlug, boolOr(req.Certificate, true), boolOr(req.Active, true),
		req.SortOrder, req.Steps, "")
	if err != nil {
		return nil, err
	}
	path.ID = pathID

	if err := s.paths.UpsertPath(ctx, path, path.Steps); err != nil {
		return nil, err
	}

	view := s.summaryView(*path, map[string]repository.LessonStat{})
	return &view, nil
}

// UpdatePath mengubah sebagian jalur belajar (langkah dipertahankan).
func (s *LearningPathService) UpdatePath(ctx context.Context, id string, req dto.UpdatePathRequest) (*dto.PathSummaryView, error) {
	existing, err := s.paths.FindPath(ctx, id)
	if err != nil {
		return nil, err
	}

	steps := make([]dto.PathStepInput, 0, len(existing.Steps))
	for _, step := range existing.Steps {
		required := step.Required
		steps = append(steps, dto.PathStepInput{ModuleID: step.ModuleID, Required: &required})
	}

	path, err := s.buildPath(ctx,
		valueOr(req.Slug, existing.Slug),
		valueOr(req.Title, existing.Title),
		valueOr(req.Description, existing.Description),
		valueOr(req.SectorKey, existing.SectorKey),
		valueOr(req.TestType, existing.TestType),
		valueOr(req.ProductSlug, existing.ProductSlug),
		boolOr(req.Certificate, existing.Certificate),
		boolOr(req.Active, existing.Active),
		intOr(req.SortOrder, existing.SortOrder),
		steps,
		id,
	)
	if err != nil {
		return nil, err
	}
	path.ID = id

	if err := s.paths.UpsertPath(ctx, path, path.Steps); err != nil {
		return nil, err
	}

	view := s.summaryView(*path, map[string]repository.LessonStat{})
	return &view, nil
}

// SetPathSteps menyusun ulang langkah sebuah jalur belajar.
func (s *LearningPathService) SetPathSteps(ctx context.Context, id string, req dto.SetPathStepsRequest) (*dto.PathSummaryView, error) {
	existing, err := s.paths.FindPath(ctx, id)
	if err != nil {
		return nil, err
	}

	steps, err := s.pathStepsFrom(ctx, existing.ProductSlug, req.Steps)
	if err != nil {
		return nil, err
	}
	existing.Steps = steps

	if err := s.paths.UpsertPath(ctx, existing, steps); err != nil {
		return nil, err
	}

	view := s.summaryView(*existing, map[string]repository.LessonStat{})
	return &view, nil
}

// SetPathActive mengaktifkan/menonaktifkan jalur belajar.
func (s *LearningPathService) SetPathActive(ctx context.Context, id string, active bool) error {
	return s.paths.SetPathActive(ctx, id, active)
}

// buildPath memvalidasi payload admin menjadi model jalur belajar siap simpan.
//
// `selfID` adalah id jalur yang sedang diubah (kosong saat pembuatan baru) dan
// dipakai untuk mengabaikan bentrok slug dengan dirinya sendiri.
func (s *LearningPathService) buildPath(
	ctx context.Context,
	slug, title, description, sectorKey, testType, productSlug string,
	certificate, active bool, sortOrder int,
	stepInputs []dto.PathStepInput, selfID string,
) (*models.LearningPath, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	title = strings.TrimSpace(title)
	productSlug = strings.TrimSpace(productSlug)

	if slug == "" || title == "" {
		return nil, apperror.Validation("Judul dan slug jalur wajib diisi.", nil)
	}
	sector, err := validateSector(sectorKey)
	if err != nil {
		return nil, err
	}
	kind, err := validateTestType(testType)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProduct(ctx, productSlug); err != nil {
		return nil, err
	}

	// Slug adalah kunci URL jalur, jadi harus unik lintas jalur (bukan hanya
	// di dalam produk). Bentrok dicek di sini agar pesannya jelas untuk admin.
	if other, err := s.paths.FindPathBySlug(ctx, slug); err == nil {
		if other.ID != selfID {
			return nil, apperror.Validation(
				fmt.Sprintf("Slug %q sudah dipakai jalur lain.", slug),
				map[string]string{"slug": "Gunakan slug lain."})
		}
	} else if !isNotFound(err) {
		return nil, err
	}

	steps, err := s.pathStepsFrom(ctx, productSlug, stepInputs)
	if err != nil {
		return nil, err
	}

	path := &models.LearningPath{
		Slug:        slug,
		Title:       title,
		Description: strings.TrimSpace(description),
		SectorKey:   sector,
		TestType:    kind,
		ProductSlug: productSlug,
		Certificate: certificate,
		SortOrder:   sortOrder,
		Active:      active,
		Steps:       steps,
	}
	return path, nil
}

// valueOr mengembalikan nilai pengganti bila pointer kosong.
func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

// intOr mengembalikan nilai pengganti bila pointer kosong.
func intOr(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}
