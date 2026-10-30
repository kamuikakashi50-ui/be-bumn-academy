package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/certificate"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// wib adalah zona waktu Indonesia Barat untuk penomoran & tanggal sertifikat.
//
// Sengaja tidak memakai zona server: nomor sertifikat harus mengikuti tahun
// Indonesia agar tidak "mundur" saat server berada di UTC.
var wib = time.FixedZone("WIB", 7*60*60)

// CertificateService menerbitkan, mencabut, dan memverifikasi sertifikat jalur
// belajar (Fase 1 langkah 3).
//
// Aturan yang dipegang (spesifikasi §4.3):
//   - syarat terbit: seluruh langkah **wajib** selesai dan seluruh kuis aktif pada
//     pelajaran modul wajib pernah lulus minimal sekali;
//   - penerbitan idempoten: satu peserta satu sertifikat per jalur; pencabutan
//     tidak dibatalkan otomatis oleh penerbitan ulang;
//   - nomor `BA-<tahun>-<6 digit>` diambil dalam transaksi terkunci;
//   - verifikasi publik hanya menampilkan nama, jalur, tanggal, dan status.
type CertificateService struct {
	paths     *repository.LearningPathRepository
	classroom *repository.ClassroomRepository
	users     *repository.UserRepository
	access    *AccessChecker
	cfg       *config.Config
}

// NewCertificateService membentuk service sertifikat.
func NewCertificateService(
	paths *repository.LearningPathRepository,
	classroom *repository.ClassroomRepository,
	users *repository.UserRepository,
	access *AccessChecker,
	cfg *config.Config,
) *CertificateService {
	return &CertificateService{
		paths:     paths,
		classroom: classroom,
		users:     users,
		access:    access,
		cfg:       cfg,
	}
}

// CertificateFile adalah berkas PDF sertifikat siap dikirim ke klien.
type CertificateFile struct {
	Filename string
	Data     []byte
}

/* --------------------------------- peserta --------------------------------- */

// ListMine mengambil sertifikat milik peserta (terbaru lebih dulu).
func (s *CertificateService) ListMine(ctx context.Context, userID string) ([]dto.CertificateView, error) {
	rows, err := s.paths.ListCertificatesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.CertificateView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.view(ctx, row))
	}
	return out, nil
}

// Verify memverifikasi sertifikat lewat nomornya (publik, tanpa login).
func (s *CertificateService) Verify(ctx context.Context, number string) (*dto.CertificateVerificationView, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(number))
	if trimmed == "" {
		return nil, apperror.Validation("Nomor sertifikat wajib diisi.", nil)
	}

	row, err := s.paths.FindCertificateByNumber(ctx, trimmed)
	if err != nil {
		return nil, err
	}

	view := &dto.CertificateVerificationView{
		Number:      row.Number,
		Status:      certificateStatus(row),
		IssuedAt:    row.IssuedAt.Format(time.RFC3339),
		Valid:       row.RevokedAt == nil,
		Participant: s.participantName(ctx, row.UserID),
	}
	if row.RevokedAt != nil {
		view.RevokedAt = row.RevokedAt.Format(time.RFC3339)
	}
	if row.PathID != nil {
		if path, err := s.paths.FindPath(ctx, *row.PathID); err == nil {
			view.PathTitle = path.Title
			view.TestTypeLabel = bank.TestTypeLabel(path.TestType)
			view.SectorLabel = bank.SectorLabel(path.SectorKey)
		}
	}
	view.Organization = metadataString(row.Metadata, "organization")

	return view, nil
}

// Issue menerbitkan sertifikat peserta untuk satu jalur (idempoten).
//
// Dipakai endpoint eksplisit `POST /learning/paths/{slug}/certificate`: peserta
// bisa memicu ulang bila penerbitan otomatis terlewat, dan bila syaratnya belum
// lengkap balasannya memuat rincian yang kurang (spesifikasi §5.6).
func (s *CertificateService) Issue(ctx context.Context, userID, slug string) (*dto.CertificateView, error) {
	path, err := s.paths.FindPathBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !path.Active {
		return nil, apperror.NotFound("Jalur belajar tidak ditemukan.")
	}
	if err := s.access.RequireProduct(ctx, userID, path.ProductSlug); err != nil {
		return nil, err
	}
	if !path.Certificate {
		return nil, apperror.Validation("Jalur belajar ini tidak menerbitkan sertifikat.", nil)
	}

	eligibility, err := s.eligibility(ctx, userID, path)
	if err != nil {
		return nil, err
	}
	if eligibility.Issued {
		return eligibility.Certificate, nil
	}
	if !eligibility.Eligible {
		return nil, apperror.Validation(
			"Syarat sertifikat belum lengkap.",
			map[string]any{
				"missingSteps":   eligibility.MissingSteps,
				"missingQuizzes": eligibility.MissingQuizzes,
			},
		)
	}

	issued, err := s.issue(ctx, userID, path)
	if err != nil {
		return nil, err
	}
	view := s.view(ctx, *issued)
	return &view, nil
}

// PDF membentuk berkas PDF satu sertifikat.
//
// Peserta hanya boleh mengunduh sertifikatnya sendiri; admin boleh mengunduh
// semua (dipakai panel admin & laporan kelas).
func (s *CertificateService) PDF(ctx context.Context, requesterID, certificateID string, isAdmin bool) (*CertificateFile, error) {
	row, err := s.paths.FindCertificateByID(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	if !isAdmin && row.UserID != requesterID {
		return nil, apperror.Forbidden("Sertifikat ini bukan milikmu.")
	}

	doc := certificate.Document{
		Number:       row.Number,
		Participant:  s.participantName(ctx, row.UserID),
		IssuedAt:     row.IssuedAt,
		Revoked:      row.RevokedAt != nil,
		VerifyURL:    s.verifyURL(row.Number),
		Organization: metadataString(row.Metadata, "organization"),
	}

	if row.PathID != nil {
		if path, err := s.paths.FindPath(ctx, *row.PathID); err == nil {
			doc.PathTitle = path.Title
			doc.TestTypeLabel = bank.TestTypeLabel(path.TestType)
			doc.SectorLabel = bank.SectorLabel(path.SectorKey)
		}
	}
	if doc.PathTitle == "" {
		// Jalur bisa saja sudah dihapus (path_id menjadi NULL); judul disimpan
		// sebagai cadangan di metadata agar sertifikat lama tetap bisa dicetak.
		doc.PathTitle = metadataString(row.Metadata, "pathTitle")
	}

	return &CertificateFile{
		Filename: fmt.Sprintf("sertifikat-%s.pdf", row.Number),
		Data:     certificate.Render(doc),
	}, nil
}

/* ---------------------------------- admin ---------------------------------- */

// AdminList mengambil daftar sertifikat untuk panel admin.
func (s *CertificateService) AdminList(ctx context.Context, filter repository.CertificateFilter, limit int) ([]dto.AdminCertificateView, error) {
	rows, err := s.paths.ListCertificates(ctx, filter, limit)
	if err != nil {
		return nil, err
	}

	out := make([]dto.AdminCertificateView, 0, len(rows))
	for _, row := range rows {
		view := dto.AdminCertificateView{
			CertificateView: dto.CertificateView{
				ID:          row.ID,
				Number:      row.Number,
				IssuedAt:    row.IssuedAt.Format(time.RFC3339),
				Status:      certificateStatus(&models.Certificate{RevokedAt: row.RevokedAt}),
				DownloadURL: "/admin/certificates/" + row.ID + "/pdf",
				VerifyURL:   s.verifyURL(row.Number),
			},
			UserID: row.UserID,
			Name:   row.Name,
			Email:  row.Email,
		}
		if row.PathID != nil {
			view.PathID = *row.PathID
		}
		if row.PathTitle != nil {
			view.PathTitle = *row.PathTitle
		}
		if row.RevokedAt != nil {
			view.RevokedAt = row.RevokedAt.Format(time.RFC3339)
		}
		out = append(out, view)
	}
	return out, nil
}

// Revoke mencabut sertifikat (mis. temuan kecurangan atau permintaan peserta).
func (s *CertificateService) Revoke(ctx context.Context, id string) error {
	return s.paths.RevokeCertificate(ctx, id)
}

// Restore membatalkan pencabutan sertifikat (salah cabut bisa dipulihkan).
func (s *CertificateService) Restore(ctx context.Context, id string) error {
	return s.paths.RestoreCertificate(ctx, id)
}

/* ------------------------------ perhitungan ------------------------------- */

// Eligibility memeriksa syarat sertifikat seorang peserta pada satu jalur.
//
// Dipakai juga oleh detail jalur peserta (`eligibleCertificate`), sehingga aturan
// "kapan sertifikat layak" hanya hidup di satu tempat.
func (s *CertificateService) Eligibility(ctx context.Context, userID string, path *models.LearningPath) (*dto.CertificateEligibilityView, error) {
	return s.eligibility(ctx, userID, path)
}

// eligibility adalah implementasi pemeriksaan syarat sertifikat.
func (s *CertificateService) eligibility(ctx context.Context, userID string, path *models.LearningPath) (*dto.CertificateEligibilityView, error) {
	view := &dto.CertificateEligibilityView{
		CertificateEnabled: path.Certificate,
		MissingSteps:       []dto.CertificateBlockerView{},
		MissingQuizzes:     []dto.CertificateBlockerView{},
	}

	// Sertifikat yang sudah terbit membuat pemeriksaan syarat tidak perlu diulang.
	existing, err := s.paths.FindCertificateByUserAndPath(ctx, userID, path.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		issued := s.view(ctx, *existing)
		view.Issued = true
		view.Eligible = existing.RevokedAt == nil
		view.Certificate = &issued
		return view, nil
	}

	required := path.RequiredModuleIDs()
	stats, err := s.paths.ModuleLessonStats(ctx, userID, required)
	if err != nil {
		return nil, err
	}

	for _, step := range path.Steps {
		if !step.Required {
			continue
		}
		stat := stats[step.ModuleID]
		if moduleCompleted(stat) {
			continue
		}
		view.MissingSteps = append(view.MissingSteps, dto.CertificateBlockerView{
			ID:              step.ModuleID,
			Title:           s.moduleTitle(ctx, step.ModuleID),
			ProgressPercent: percentOf(stat.Completed, stat.Total),
		})
	}

	// Kuis hanya diperiksa bila langkah wajib sudah selesai: pemeriksaan ini butuh
	// query tambahan, dan hasilnya tidak relevan selama masih ada modul tertinggal.
	if len(view.MissingSteps) == 0 {
		quizzes, err := s.requiredQuizzes(ctx, path, required)
		if err != nil {
			return nil, err
		}
		for _, item := range quizzes {
			passed, err := s.paths.CountPassedAttempts(ctx, item.quiz.ID, userID)
			if err != nil {
				return nil, err
			}
			if passed > 0 {
				continue
			}
			view.MissingQuizzes = append(view.MissingQuizzes, dto.CertificateBlockerView{
				ID:    item.quiz.ID,
				Title: item.label,
			})
		}
	}

	view.Eligible = len(view.MissingSteps) == 0 && len(view.MissingQuizzes) == 0
	return view, nil
}

// requiredQuiz adalah kuis wajib pada jalur beserta label untuk pesan peserta.
type requiredQuiz struct {
	quiz  models.LessonQuiz
	label string
}

// requiredQuizzes mengambil kuis aktif pada pelajaran modul wajib sebuah jalur.
func (s *CertificateService) requiredQuizzes(ctx context.Context, path *models.LearningPath, moduleIDs []string) ([]requiredQuiz, error) {
	if len(moduleIDs) == 0 {
		return nil, nil
	}

	lessons, err := s.classroom.ListLessonsByProduct(ctx, path.ProductSlug, true)
	if err != nil {
		return nil, err
	}

	wanted := make(map[string]struct{}, len(moduleIDs))
	for _, moduleID := range moduleIDs {
		wanted[moduleID] = struct{}{}
	}

	lessonTitles := map[string]string{}
	lessonIDs := make([]string, 0, len(lessons))
	for _, lesson := range lessons {
		if _, ok := wanted[lesson.ModuleID]; !ok {
			continue
		}
		lessonIDs = append(lessonIDs, lesson.ID)
		lessonTitles[lesson.ID] = lesson.Title
	}

	quizzes, err := s.paths.ListActiveQuizzesByLessons(ctx, lessonIDs)
	if err != nil {
		return nil, err
	}

	out := make([]requiredQuiz, 0, len(quizzes))
	for _, quiz := range quizzes {
		label := quiz.Title
		if title := lessonTitles[quiz.LessonID]; title != "" {
			label = fmt.Sprintf("%s (%s)", quiz.Title, title)
		}
		out = append(out, requiredQuiz{quiz: quiz, label: label})
	}
	return out, nil
}

// issue menyimpan sertifikat peserta (nomor diambil dalam transaksi terkunci).
func (s *CertificateService) issue(ctx context.Context, userID string, path *models.LearningPath) (*models.Certificate, error) {
	participant := s.participantName(ctx, userID)
	now := time.Now().UTC()

	metadata := models.JSONMap{
		"participantName": participant,
		"pathTitle":       path.Title,
		"testTypeLabel":   bank.TestTypeLabel(path.TestType),
		"sectorLabel":     bank.SectorLabel(path.SectorKey),
		"issuedDate":      now.In(wib).Format("2006-01-02"),
	}

	pathID := path.ID
	certificate := &models.Certificate{
		IDBase:   models.IDBase{ID: models.NewID()},
		UserID:   userID,
		PathID:   &pathID,
		IssuedAt: now,
		Metadata: metadata,
	}

	return s.paths.CreateCertificateForUser(ctx, certificate, now.In(wib).Year())
}

// view membentuk tampilan satu sertifikat peserta.
func (s *CertificateService) view(ctx context.Context, row models.Certificate) dto.CertificateView {
	view := dto.CertificateView{
		ID:           row.ID,
		Number:       row.Number,
		IssuedAt:     row.IssuedAt.Format(time.RFC3339),
		Status:       certificateStatus(&row),
		Organization: metadataString(row.Metadata, "organization"),
		DownloadURL:  "/certificates/" + row.ID + "/pdf",
		VerifyURL:    s.verifyURL(row.Number),
	}
	if row.RevokedAt != nil {
		view.RevokedAt = row.RevokedAt.Format(time.RFC3339)
	}
	if row.PathID != nil {
		view.PathID = *row.PathID
		if path, err := s.paths.FindPath(ctx, *row.PathID); err == nil {
			view.PathTitle = path.Title
			view.SectorKey = path.SectorKey
			view.SectorLabel = bank.SectorLabel(path.SectorKey)
			view.TestType = path.TestType
			view.TestTypeLabel = bank.TestTypeLabel(path.TestType)
		}
	}
	if view.PathTitle == "" {
		view.PathTitle = metadataString(row.Metadata, "pathTitle")
	}
	return view
}

// participantName mengambil nama peserta (dengan pengaman bila akun sudah hilang).
func (s *CertificateService) participantName(ctx context.Context, userID string) string {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return "Peserta BUMN Academy"
	}
	return user.Name
}

// moduleTitle mengambil judul modul untuk pesan "langkah belum selesai".
func (s *CertificateService) moduleTitle(ctx context.Context, moduleID string) string {
	module, err := s.classroom.FindModule(ctx, moduleID)
	if err != nil || module == nil {
		return moduleID
	}
	return module.Title
}

// verifyURL membentuk tautan verifikasi publik sebuah nomor sertifikat.
func (s *CertificateService) verifyURL(number string) string {
	base := strings.TrimRight(s.cfg.App.PublicURL, "/")
	if base == "" {
		return "/verifikasi/" + number
	}
	return base + "/verifikasi/" + number
}

// certificateStatus menerjemahkan `revoked_at` menjadi status yang dibaca manusia.
func certificateStatus(row *models.Certificate) string {
	if row.RevokedAt != nil {
		return "dicabut"
	}
	return "sah"
}

// metadataString mengambil satu nilai teks dari metadata sertifikat.
func metadataString(metadata models.JSONMap, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

/* --------------------------- penerbitan otomatis --------------------------- */

// IssueEligibleForModule menerbitkan sertifikat untuk jalur yang memuat sebuah modul.
//
// Dipanggil setelah progres materi disimpan. Kegagalan di sini **tidak** boleh
// membatalkan penyimpanan progres peserta, jadi pemanggil mengabaikan error dan
// endpoint eksplisit tetap tersedia untuk memicu ulang.
func (s *CertificateService) IssueEligibleForModule(ctx context.Context, userID, moduleID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(moduleID) == "" {
		return nil
	}

	rows, err := s.paths.ListPaths(ctx, repository.PathFilter{ActiveOnly: true})
	if err != nil {
		return err
	}

	for index := range rows {
		path := rows[index]
		if !path.Certificate || !pathHasModule(path, moduleID) {
			continue
		}
		if err := s.issueIfEligible(ctx, userID, path); err != nil {
			return err
		}
	}
	return nil
}

// IssueEligibleForAll menerbitkan sertifikat untuk seluruh jalur yang syaratnya
// sudah terpenuhi (dipakai setelah peserta menyelesaikan kuis).
func (s *CertificateService) IssueEligibleForAll(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return nil
	}

	rows, err := s.paths.ListPaths(ctx, repository.PathFilter{ActiveOnly: true})
	if err != nil {
		return err
	}

	for index := range rows {
		path := rows[index]
		if !path.Certificate {
			continue
		}
		if err := s.issueIfEligible(ctx, userID, path); err != nil {
			return err
		}
	}
	return nil
}

// pathHasModule menandai sebuah jalur memuat modul tertentu.
func pathHasModule(path models.LearningPath, moduleID string) bool {
	for _, step := range path.Steps {
		if step.ModuleID == moduleID {
			return true
		}
	}
	return false
}

// issueIfEligible menerbitkan sertifikat bila syarat jalur sudah terpenuhi.
func (s *CertificateService) issueIfEligible(ctx context.Context, userID string, path models.LearningPath) error {
	eligibility, err := s.eligibility(ctx, userID, &path)
	if err != nil {
		return err
	}
	if eligibility.Issued || !eligibility.Eligible {
		return nil
	}
	_, err = s.issue(ctx, userID, &path)
	return err
}
