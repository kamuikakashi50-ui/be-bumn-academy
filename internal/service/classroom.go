package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/ebook"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// CompletedThreshold adalah persentase minimum agar sebuah pelajaran dianggap
// selesai otomatis (peserta juga bisa menandai selesai secara manual).
const CompletedThreshold = 90

// ClassroomService melayani ruang kelas digital: daftar materi per produk,
// detail pelajaran, progres menonton/membaca, dan e-book PDF.
type ClassroomService struct {
	classroom   *repository.ClassroomRepository
	enrollments *repository.EnrollmentRepository
	catalog     *repository.CatalogRepository
	cfg         *config.Config
	// certificates menerbitkan sertifikat saat materi terakhir sebuah jalur
	// diselesaikan (opsional; diisi wiring, lihat SetCertificateIssuer).
	certificates *CertificateService
}

// SetCertificateIssuer memasang penerbit sertifikat yang dipanggil setelah progres
// materi tersimpan.
//
// Dipisah dari konstruktor agar service ruang kelas tetap bisa dipakai tanpa
// sertifikat (mis. pada paket uji) dan agar tidak ada ketergantungan melingkar.
func (s *ClassroomService) SetCertificateIssuer(issuer *CertificateService) {
	s.certificates = issuer
}

// NewClassroomService membentuk service ruang kelas.
func NewClassroomService(
	classroom *repository.ClassroomRepository,
	enrollments *repository.EnrollmentRepository,
	catalog *repository.CatalogRepository,
	cfg *config.Config,
) *ClassroomService {
	return &ClassroomService{
		classroom:   classroom,
		enrollments: enrollments,
		catalog:     catalog,
		cfg:         cfg,
	}
}

// EbookResult adalah hasil penyiapan e-book (berkas PDF atau tautan luar).
type EbookResult struct {
	Data        []byte
	Filename    string
	Title       string
	ExternalURL string
}

/* --------------------------------- peserta -------------------------------- */

// Courses mengembalikan ringkasan ruang kelas untuk seluruh produk yang dimiliki
// peserta (produk tanpa materi tetap ditampilkan agar tidak membingungkan).
func (s *ClassroomService) Courses(ctx context.Context, userID string) ([]dto.CourseView, error) {
	slugs, err := s.enrollments.OwnedSlugs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(slugs) == 0 {
		return []dto.CourseView{}, nil
	}

	modules, err := s.classroom.ListModules(ctx, "", false)
	if err != nil {
		return nil, err
	}
	progress, err := s.progressMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	products, err := s.catalog.ListProducts(ctx, true)
	if err != nil {
		return nil, err
	}
	byName := map[string]models.Product{}
	for _, product := range products {
		byName[product.Slug] = product
	}

	grouped := map[string][]models.CourseModule{}
	for _, module := range modules {
		grouped[module.ProductSlug] = append(grouped[module.ProductSlug], module)
	}

	out := make([]dto.CourseView, 0, len(slugs))
	for _, slug := range slugs {
		course := s.buildCourse(slug, grouped[slug], progress)
		if product, ok := byName[slug]; ok {
			course.ProductName = product.Name
			course.Description = product.Description
			course.Badge = strPtrOrEmpty(product.Badge)
		} else {
			course.ProductName = slug
		}
		out = append(out, course)
	}

	// Produk bermateri lebih dulu agar peserta langsung melihat kelas aktifnya.
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].TotalLessons > 0) != (out[j].TotalLessons > 0) {
			return out[i].TotalLessons > 0
		}
		return out[i].ProductSlug < out[j].ProductSlug
	})
	return out, nil
}

// Course mengambil isi ruang kelas satu produk (wajib dimiliki peserta).
func (s *ClassroomService) Course(ctx context.Context, userID, productSlug string) (*dto.ClassroomView, error) {
	if err := s.requireOwnership(ctx, userID, productSlug); err != nil {
		return nil, err
	}

	modules, err := s.classroom.ListModules(ctx, productSlug, false)
	if err != nil {
		return nil, err
	}
	progress, err := s.progressMap(ctx, userID)
	if err != nil {
		return nil, err
	}

	course, err := s.courseWithProduct(ctx, productSlug, modules, progress)
	if err != nil {
		return nil, err
	}

	return &dto.ClassroomView{
		Course:   course,
		Modules:  s.buildModules(modules, progress),
		Sequence: s.sequence(modules, progress),
	}, nil
}

// Lesson mengambil detail satu pelajaran beserta navigasi sebelumnya/berikutnya.
func (s *ClassroomService) Lesson(ctx context.Context, userID, lessonID string) (*dto.LessonDetailView, error) {
	lesson, err := s.classroom.FindLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwnership(ctx, userID, lesson.ProductSlug); err != nil {
		return nil, err
	}

	modules, err := s.classroom.ListModules(ctx, lesson.ProductSlug, false)
	if err != nil {
		return nil, err
	}
	progress, err := s.progressMap(ctx, userID)
	if err != nil {
		return nil, err
	}
	course, err := s.courseWithProduct(ctx, lesson.ProductSlug, modules, progress)
	if err != nil {
		return nil, err
	}

	titles := s.moduleTitles(modules)
	sequence := s.sequence(modules, progress)
	detail := &dto.LessonDetailView{
		Lesson: s.buildLesson(*lesson, titles, progress[lesson.ID], true),
		Course: course,
	}
	for index, item := range sequence {
		if item.LessonID != lesson.ID {
			continue
		}
		if index > 0 {
			previous := sequence[index-1]
			detail.Previous = &previous
		}
		if index < len(sequence)-1 {
			next := sequence[index+1]
			detail.Next = &next
		}
		break
	}
	return detail, nil
}

// SaveProgress menyimpan progres menonton/membaca satu pelajaran.
//
// Persentase dihitung ulang di server dari durasi pelajaran (bila diketahui) dan
// status "selesai" bersifat menetap: sekali selesai, tetap selesai.
func (s *ClassroomService) SaveProgress(ctx context.Context, userID, lessonID string, req dto.SaveProgressRequest) (*dto.LessonView, error) {
	lesson, err := s.classroom.FindLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwnership(ctx, userID, lesson.ProductSlug); err != nil {
		return nil, err
	}

	existing, err := s.classroom.FindProgress(ctx, userID, lessonID)
	if err != nil {
		return nil, err
	}

	position, percent, completed := 0, 0, false
	if existing != nil {
		position, percent, completed = existing.PositionSeconds, existing.ProgressPercent, existing.Completed
	}

	if req.PositionSeconds != nil {
		position = *req.PositionSeconds
	}
	if lesson.DurationSeconds > 0 && position > lesson.DurationSeconds {
		position = lesson.DurationSeconds
	}

	// Persentase: dari posisi bila durasi diketahui, selain itu dari klien.
	if lesson.DurationSeconds > 0 {
		percent = int(float64(position) / float64(lesson.DurationSeconds) * 100)
	} else if req.ProgressPercent != nil {
		percent = *req.ProgressPercent
	}
	percent = clampInt(percent, 0, 100)

	if percent >= CompletedThreshold {
		completed = true
	}
	if req.Completed != nil && *req.Completed {
		completed = true
		percent = 100
		if lesson.DurationSeconds > 0 {
			position = lesson.DurationSeconds
		}
	}

	progress := &models.LessonProgress{
		UserID:          userID,
		LessonID:        lessonID,
		PositionSeconds: position,
		ProgressPercent: percent,
		Completed:       completed,
		LastViewedAt:    time.Now().UTC(),
	}
	if err := s.classroom.SaveProgress(ctx, progress); err != nil {
		return nil, err
	}

	// Materi selesai bisa melengkapi langkah wajib sebuah jalur. Penerbitan
	// sertifikat dijalankan setelah progres tersimpan, dan kegagalannya sengaja
	// diabaikan: progres peserta tidak boleh hilang karena masalah sertifikat.
	// Peserta bisa memicu ulang lewat POST /learning/paths/{slug}/certificate.
	if completed && s.certificates != nil {
		_ = s.certificates.IssueEligibleForModule(ctx, userID, lesson.ModuleID)
	}

	modules, err := s.classroom.ListModules(ctx, lesson.ProductSlug, false)
	if err != nil {
		return nil, err
	}
	view := s.buildLesson(*lesson, s.moduleTitles(modules), progress, true)
	return &view, nil
}

// Ebook menyiapkan e-book sebuah pelajaran.
//
// Bila admin mengisi `ebookUrl`, tautan itu dikembalikan apa adanya; bila tidak,
// PDF dibuat dari judul, ringkasan, dan konten pelajaran.
func (s *ClassroomService) Ebook(ctx context.Context, userID, lessonID string) (*EbookResult, error) {
	lesson, err := s.classroom.FindLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwnership(ctx, userID, lesson.ProductSlug); err != nil {
		return nil, err
	}
	if url := strings.TrimSpace(strPtrOrEmpty(lesson.EbookURL)); url != "" {
		return &EbookResult{ExternalURL: url, Title: lesson.Title}, nil
	}
	if strings.TrimSpace(lesson.Content) == "" && strings.TrimSpace(lesson.Summary) == "" {
		return nil, apperror.NotFound("E-book untuk pelajaran ini belum tersedia.")
	}

	product, err := s.catalog.FindProductBySlug(ctx, lesson.ProductSlug)
	if err != nil {
		return nil, err
	}
	module, err := s.classroom.FindModule(ctx, lesson.ModuleID)
	if err != nil {
		return nil, err
	}

	document := ebook.Document{
		Title:    lesson.Title,
		Subtitle: fmt.Sprintf("%s · %s", module.Title, product.Name),
		Footer:   "BUMN Academy — Ruang Kelas",
		Blocks:   ebookBlocks(*lesson),
	}

	return &EbookResult{
		Data:     ebook.Render(document),
		Filename: ebookFilename(lesson.Title),
		Title:    lesson.Title,
	}, nil
}

// ContinueLearning mengambil pelajaran terakhir yang belum selesai (dipakai
// kartu "lanjutkan belajar" pada dashboard).
func (s *ClassroomService) ContinueLearning(ctx context.Context, userID string, limit int) ([]dto.LessonRefView, error) {
	if limit <= 0 {
		limit = 3
	}

	rows, err := s.classroom.LastViewedProgress(ctx, userID, limit*3)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []dto.LessonRefView{}, nil
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.LessonID)
	}
	lessons, err := s.classroom.LessonsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := map[string]models.Lesson{}
	for _, lesson := range lessons {
		byID[lesson.ID] = lesson
	}

	modules, err := s.classroom.ListModules(ctx, "", false)
	if err != nil {
		return nil, err
	}
	titles := s.moduleTitles(modules)

	out := make([]dto.LessonRefView, 0, limit)
	for index := range rows {
		row := rows[index]
		lesson, ok := byID[row.LessonID]
		if !ok || !lesson.Active || row.Completed {
			continue
		}
		out = append(out, s.buildRef(lesson, titles, &row))
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

/* ---------------------------------- util ---------------------------------- */

// requireOwnership memastikan peserta memiliki produk pemilik pelajaran.
func (s *ClassroomService) requireOwnership(ctx context.Context, userID, productSlug string) error {
	owned, err := s.enrollments.OwnedSlugs(ctx, userID)
	if err != nil {
		return err
	}
	for _, slug := range owned {
		if slug == productSlug {
			return nil
		}
	}
	return apperror.Forbidden("Kamu belum memiliki akses ke materi ini.")
}

// progressMap memuat seluruh progres peserta dan memetakannya per pelajaran.
func (s *ClassroomService) progressMap(ctx context.Context, userID string) (map[string]*models.LessonProgress, error) {
	rows, err := s.classroom.ProgressByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*models.LessonProgress, len(rows))
	for index := range rows {
		row := rows[index]
		out[row.LessonID] = &row
	}
	return out, nil
}

// moduleTitles memetakan id modul ke judulnya.
func (s *ClassroomService) moduleTitles(modules []models.CourseModule) map[string]string {
	out := make(map[string]string, len(modules))
	for _, module := range modules {
		out[module.ID] = module.Title
	}
	return out
}

// courseWithProduct menyusun ringkasan ruang kelas beserta identitas produknya.
func (s *ClassroomService) courseWithProduct(
	ctx context.Context,
	productSlug string,
	modules []models.CourseModule,
	progress map[string]*models.LessonProgress,
) (dto.CourseView, error) {
	product, err := s.catalog.FindProductBySlug(ctx, productSlug)
	if err != nil {
		return dto.CourseView{}, err
	}
	course := s.buildCourse(productSlug, modules, progress)
	course.ProductName = product.Name
	course.Description = product.Description
	course.Badge = strPtrOrEmpty(product.Badge)
	return course, nil
}

// buildCourse menyusun ringkasan satu produk dari modul-modulnya.
func (s *ClassroomService) buildCourse(productSlug string, modules []models.CourseModule, progress map[string]*models.LessonProgress) dto.CourseView {
	course := dto.CourseView{ProductSlug: productSlug}
	var last *models.LessonProgress
	var lastLesson *models.Lesson

	for _, module := range modules {
		for _, lesson := range module.Lessons {
			course.TotalLessons++
			course.TotalDurationSeconds += lesson.DurationSeconds
			row := progress[lesson.ID]
			if row != nil && row.Completed {
				course.CompletedLessons++
			}
			if row != nil && (last == nil || row.LastViewedAt.After(last.LastViewedAt)) {
				copyLesson := lesson
				last, lastLesson = row, &copyLesson
			}
		}
	}
	course.ProgressPercent = percent(course.CompletedLessons, course.TotalLessons)

	if last != nil && lastLesson != nil {
		ref := s.buildRef(*lastLesson, s.moduleTitles(modules), last)
		course.LastLesson = &ref
	}
	return course
}

// buildModules menyusun daftar modul beserta pelajarannya.
func (s *ClassroomService) buildModules(modules []models.CourseModule, progress map[string]*models.LessonProgress) []dto.ModuleView {
	titles := s.moduleTitles(modules)
	out := make([]dto.ModuleView, 0, len(modules))

	for _, module := range modules {
		view := dto.ModuleView{
			ID:          module.ID,
			ProductSlug: module.ProductSlug,
			Title:       module.Title,
			Description: module.Description,
			SortOrder:   module.SortOrder,
			Active:      module.Active,
			Lessons:     make([]dto.LessonView, 0, len(module.Lessons)),
		}
		for _, lesson := range module.Lessons {
			row := progress[lesson.ID]
			view.Lessons = append(view.Lessons, s.buildLesson(lesson, titles, row, false))
			view.TotalLessons++
			if row != nil && row.Completed {
				view.CompletedLessons++
			}
		}
		view.ProgressPercent = percent(view.CompletedLessons, view.TotalLessons)
		out = append(out, view)
	}
	return out
}

// sequence menyusun urutan pelajaran lintas modul (navigasi & prev/next).
func (s *ClassroomService) sequence(modules []models.CourseModule, progress map[string]*models.LessonProgress) []dto.LessonRefView {
	titles := s.moduleTitles(modules)
	out := make([]dto.LessonRefView, 0, 32)
	for _, module := range modules {
		for _, lesson := range module.Lessons {
			out = append(out, s.buildRef(lesson, titles, progress[lesson.ID]))
		}
	}
	return out
}

// buildLesson menyusun tampilan satu pelajaran (konten hanya pada mode detail).
func (s *ClassroomService) buildLesson(lesson models.Lesson, titles map[string]string, progress *models.LessonProgress, withContent bool) dto.LessonView {
	ebookURL := strings.TrimSpace(strPtrOrEmpty(lesson.EbookURL))
	view := dto.LessonView{
		ID:              lesson.ID,
		ModuleID:        lesson.ModuleID,
		ModuleTitle:     titles[lesson.ModuleID],
		ProductSlug:     lesson.ProductSlug,
		Title:           lesson.Title,
		Summary:         lesson.Summary,
		Kind:            string(lesson.Kind),
		VideoURL:        strPtrOrEmpty(lesson.VideoURL),
		EbookURL:        ebookURL,
		PosterURL:       strPtrOrEmpty(lesson.PosterURL),
		EbookAvailable:  ebookURL != "" || strings.TrimSpace(lesson.Content) != "",
		DurationSeconds: lesson.DurationSeconds,
		SortOrder:       lesson.SortOrder,
		Active:          lesson.Active,
		Progress:        progressView(progress),
	}
	if withContent {
		view.Content = lesson.Content
	}
	return view
}

// buildRef menyusun rujukan ringkas satu pelajaran.
func (s *ClassroomService) buildRef(lesson models.Lesson, titles map[string]string, progress *models.LessonProgress) dto.LessonRefView {
	ref := dto.LessonRefView{
		LessonID:        lesson.ID,
		ModuleID:        lesson.ModuleID,
		ModuleTitle:     titles[lesson.ModuleID],
		ProductSlug:     lesson.ProductSlug,
		Title:           lesson.Title,
		Kind:            string(lesson.Kind),
		DurationSeconds: lesson.DurationSeconds,
	}
	if progress != nil {
		ref.PositionSeconds = progress.PositionSeconds
		ref.ProgressPercent = progress.ProgressPercent
		ref.Completed = progress.Completed
	}
	return ref
}

// progressView mengubah model progres menjadi tampilan (nol bila belum ada).
func progressView(progress *models.LessonProgress) dto.LessonProgressView {
	if progress == nil {
		return dto.LessonProgressView{}
	}
	return dto.LessonProgressView{
		PositionSeconds: progress.PositionSeconds,
		ProgressPercent: progress.ProgressPercent,
		Completed:       progress.Completed,
		LastViewedAt:    formatTime(progress.LastViewedAt),
	}
}

// clampInt membatasi nilai pada rentang [min, max].
func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ebookBlocks menyusun isi e-book dari pelajaran.
//
// Konvensi ringan pada konten: baris "## Judul" menjadi subjudul, baris "- poin"
// menjadi butir daftar, dan baris lain menjadi paragraf.
func ebookBlocks(lesson models.Lesson) []ebook.Block {
	blocks := make([]ebook.Block, 0, 16)

	if summary := strings.TrimSpace(lesson.Summary); summary != "" {
		blocks = append(blocks,
			ebook.Block{Heading: true, Text: "Ringkasan"},
			ebook.Block{Text: summary},
		)
	}

	content := strings.ReplaceAll(lesson.Content, "\r\n", "\n")
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			blocks = append(blocks, ebook.Block{})
		case strings.HasPrefix(line, "## "):
			blocks = append(blocks, ebook.Block{Heading: true, Text: strings.TrimSpace(line[3:])})
		case strings.HasPrefix(line, "- "):
			blocks = append(blocks, ebook.Block{Bullet: true, Text: strings.TrimSpace(line[2:])})
		default:
			blocks = append(blocks, ebook.Block{Text: line})
		}
	}

	blocks = append(blocks,
		ebook.Block{Heading: true, Text: "Catatan"},
		ebook.Block{Text: "Materi ini disusun tim pengajar BUMN Academy sebagai bahan latihan. Tidak ada klaim afiliasi resmi maupun jaminan kelulusan seleksi."},
	)
	return blocks
}

// ebookFilename membentuk nama berkas e-book dari judul pelajaran.
func ebookFilename(title string) string {
	var buf strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			buf.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			buf.WriteByte('-')
		}
	}
	slug := strings.Trim(buf.String(), "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if slug == "" {
		slug = "materi"
	}
	return slug + ".pdf"
}

/* ------------------------------ ruang kelas (admin) ------------------------ */

// AdminModules mengambil seluruh modul (termasuk yang nonaktif) beserta pelajarannya.
func (s *ClassroomService) AdminModules(ctx context.Context, productSlug string) ([]dto.ModuleView, error) {
	modules, err := s.classroom.ListModules(ctx, productSlug, true)
	if err != nil {
		return nil, err
	}
	return s.buildModules(modules, map[string]*models.LessonProgress{}), nil
}

// AdminLessons mengambil seluruh pelajaran (termasuk nonaktif) satu produk.
func (s *ClassroomService) AdminLessons(ctx context.Context, productSlug string) ([]dto.LessonView, error) {
	lessons, err := s.classroom.ListLessonsByProduct(ctx, productSlug, true)
	if err != nil {
		return nil, err
	}
	modules, err := s.classroom.ListModules(ctx, productSlug, true)
	if err != nil {
		return nil, err
	}
	titles := s.moduleTitles(modules)

	out := make([]dto.LessonView, 0, len(lessons))
	for _, lesson := range lessons {
		out = append(out, s.buildLesson(lesson, titles, nil, true))
	}
	return out, nil
}

// UpsertModule menambah atau memperbarui modul ruang kelas.
func (s *ClassroomService) UpsertModule(ctx context.Context, req dto.ModuleInput, id string) (*dto.ModuleView, error) {
	if _, err := s.catalog.FindProductBySlug(ctx, req.ProductSlug); err != nil {
		return nil, err
	}

	moduleID := strings.TrimSpace(id)
	if moduleID == "" {
		moduleID = strings.TrimSpace(req.ID)
	}
	if moduleID == "" {
		moduleID = "mod-" + shortID()
	}

	module := models.CourseModule{
		ID:          moduleID,
		ProductSlug: strings.TrimSpace(req.ProductSlug),
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		SortOrder:   req.SortOrder,
		Active:      req.Active == nil || *req.Active,
	}
	if err := s.classroom.SaveModule(ctx, &module); err != nil {
		return nil, err
	}
	return moduleView(module), nil
}

// UpdateModule mengubah sebagian data modul.
func (s *ClassroomService) UpdateModule(ctx context.Context, id string, req dto.ModulePatch) (*dto.ModuleView, error) {
	module, err := s.classroom.FindModule(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.ProductSlug != nil {
		if _, err := s.catalog.FindProductBySlug(ctx, *req.ProductSlug); err != nil {
			return nil, err
		}
		module.ProductSlug = strings.TrimSpace(*req.ProductSlug)
	}
	if req.Title != nil {
		module.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		module.Description = strings.TrimSpace(*req.Description)
	}
	if req.SortOrder != nil {
		module.SortOrder = *req.SortOrder
	}
	if req.Active != nil {
		module.Active = *req.Active
	}
	if err := s.classroom.SaveModule(ctx, module); err != nil {
		return nil, err
	}
	return moduleView(*module), nil
}

// DeleteModule menghapus modul beserta pelajarannya.
func (s *ClassroomService) DeleteModule(ctx context.Context, id string) error {
	return s.classroom.DeleteModule(ctx, id)
}

// moduleView menyusun tampilan ringkas sebuah modul (tanpa daftar pelajaran).
func moduleView(module models.CourseModule) *dto.ModuleView {
	return &dto.ModuleView{
		ID:          module.ID,
		ProductSlug: module.ProductSlug,
		Title:       module.Title,
		Description: module.Description,
		SortOrder:   module.SortOrder,
		Active:      module.Active,
		Lessons:     []dto.LessonView{},
	}
}

// UpsertLesson menambah atau memperbarui pelajaran ruang kelas.
func (s *ClassroomService) UpsertLesson(ctx context.Context, req dto.LessonInput, id string) (*dto.LessonView, error) {
	module, err := s.classroom.FindModule(ctx, req.ModuleID)
	if err != nil {
		return nil, err
	}
	kind := models.LessonKind(strings.ToLower(strings.TrimSpace(req.Kind)))
	if !kind.Valid() {
		return nil, apperror.Validation("Jenis pelajaran harus 'video' atau 'ebook'.", nil)
	}
	if kind == models.LessonVideo && strings.TrimSpace(req.VideoURL) == "" {
		return nil, apperror.Validation("Pelajaran video wajib memiliki tautan video.", nil)
	}

	lessonID := strings.TrimSpace(id)
	if lessonID == "" {
		lessonID = strings.TrimSpace(req.ID)
	}
	if lessonID == "" {
		lessonID = "les-" + shortID()
	}

	lesson := models.Lesson{
		ID:              lessonID,
		ModuleID:        module.ID,
		ProductSlug:     module.ProductSlug,
		Title:           strings.TrimSpace(req.Title),
		Summary:         strings.TrimSpace(req.Summary),
		Kind:            kind,
		VideoURL:        optionalString(req.VideoURL),
		EbookURL:        optionalString(req.EbookURL),
		PosterURL:       optionalString(req.PosterURL),
		Content:         req.Content,
		DurationSeconds: req.DurationSeconds,
		SortOrder:       req.SortOrder,
		Active:          req.Active == nil || *req.Active,
	}
	if err := s.classroom.SaveLesson(ctx, &lesson); err != nil {
		return nil, err
	}

	view := s.buildLesson(lesson, map[string]string{module.ID: module.Title}, nil, true)
	return &view, nil
}

// UpdateLesson mengubah sebagian data pelajaran.
func (s *ClassroomService) UpdateLesson(ctx context.Context, id string, req dto.LessonPatch) (*dto.LessonView, error) {
	lesson, err := s.classroom.FindLesson(ctx, id)
	if err != nil {
		return nil, err
	}

	titles := map[string]string{}
	if req.ModuleID != nil {
		module, err := s.classroom.FindModule(ctx, *req.ModuleID)
		if err != nil {
			return nil, err
		}
		lesson.ModuleID = module.ID
		lesson.ProductSlug = module.ProductSlug
		titles[module.ID] = module.Title
	} else if module, err := s.classroom.FindModule(ctx, lesson.ModuleID); err == nil {
		titles[module.ID] = module.Title
	}

	if req.Title != nil {
		lesson.Title = strings.TrimSpace(*req.Title)
	}
	if req.Summary != nil {
		lesson.Summary = strings.TrimSpace(*req.Summary)
	}
	if req.Kind != nil {
		kind := models.LessonKind(strings.ToLower(strings.TrimSpace(*req.Kind)))
		if !kind.Valid() {
			return nil, apperror.Validation("Jenis pelajaran harus 'video' atau 'ebook'.", nil)
		}
		lesson.Kind = kind
	}
	if req.VideoURL != nil {
		lesson.VideoURL = optionalString(*req.VideoURL)
	}
	if req.EbookURL != nil {
		lesson.EbookURL = optionalString(*req.EbookURL)
	}
	if req.PosterURL != nil {
		lesson.PosterURL = optionalString(*req.PosterURL)
	}
	if req.Content != nil {
		lesson.Content = *req.Content
	}
	if req.DurationSeconds != nil {
		lesson.DurationSeconds = *req.DurationSeconds
	}
	if req.SortOrder != nil {
		lesson.SortOrder = *req.SortOrder
	}
	if req.Active != nil {
		lesson.Active = *req.Active
	}
	if lesson.Kind == models.LessonVideo && strPtrOrEmpty(lesson.VideoURL) == "" {
		return nil, apperror.Validation("Pelajaran video wajib memiliki tautan video.", nil)
	}
	if err := s.classroom.SaveLesson(ctx, lesson); err != nil {
		return nil, err
	}

	view := s.buildLesson(*lesson, titles, nil, true)
	return &view, nil
}

// DeleteLesson menghapus pelajaran beserta progresnya.
func (s *ClassroomService) DeleteLesson(ctx context.Context, id string) error {
	return s.classroom.DeleteLesson(ctx, id)
}
