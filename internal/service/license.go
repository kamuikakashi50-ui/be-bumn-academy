package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// licenseExpiringWindowDays adalah rentang peringatan izin yang akan berakhir.
const licenseExpiringWindowDays = 30

// licenseScopeLabels memetakan cakupan izin ke label yang ditampilkan.
var licenseScopeLabels = map[models.LicenseScope]string{
	models.LicenseInternal:     "Dibuat sendiri",
	models.LicenseVendor:       "Lisensi vendor",
	models.LicensePublicDomain: "Domain publik",
	models.LicenseClientOwned:  "Milik instansi",
}

// instrumentKindLabels memetakan jenis instrumen psikologi ke labelnya.
var instrumentKindLabels = map[models.InstrumentKind]string{
	models.InstrumentPractice: "Latihan peserta",
	models.InstrumentLicensed: "Berlisensi (psikolog)",
}

// LicenseService mengelola izin/hak pakai soal dan instrumen tes psikologi.
//
// Ini inti jawaban atas permintaan klien "harus dipastikan izinnya": setiap soal
// punya bukti izin, dan izin yang tidak aktif membuat soal berhenti dipakai
// peserta (lihat TryoutService.onlyLicensed).
type LicenseService struct {
	licenses  *repository.LicenseRepository
	questions *repository.TryoutRepository
	tryoutSvc *TryoutService
	cfg       *config.Config
	logger    *slog.Logger
}

// NewLicenseService membentuk service izin soal.
func NewLicenseService(
	licenses *repository.LicenseRepository,
	questions *repository.TryoutRepository,
	tryoutSvc *TryoutService,
	cfg *config.Config,
	logger *slog.Logger,
) *LicenseService {
	return &LicenseService{
		licenses:  licenses,
		questions: questions,
		tryoutSvc: tryoutSvc,
		cfg:       cfg,
		logger:    logger,
	}
}

/* ------------------------------- izin soal ------------------------------- */

// List mengambil daftar izin soal beserta ringkasan soal terkait.
func (s *LicenseService) List(ctx context.Context, filter repository.LicenseFilter, page, perPage int) ([]dto.LicenseView, int64, error) {
	rows, total, err := s.licenses.ListLicenses(ctx, filter, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	questions, err := s.questionIndex(ctx, licenseQuestionIDs(rows))
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.LicenseView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toLicenseView(row, questions[row.QuestionID]))
	}
	return out, total, nil
}

// ValidateLicenseInput memeriksa data izin sebelum soal disimpan.
//
// Dipanggil lebih dulu oleh AdminService agar soal tidak tersimpan setengah
// jalan ketika datanya ternyata belum lengkap.
func (s *LicenseService) ValidateLicenseInput(input *dto.QuestionLicenseInput) error {
	if input == nil {
		return nil
	}
	if input.Scope != "" && !models.LicenseScope(input.Scope).Valid() {
		return apperror.Validation("Cakupan izin tidak dikenal.",
			map[string]string{"scope": "Pilih cakupan izin dari daftar."})
	}
	if input.Scope == string(models.LicenseVendor) && strings.TrimSpace(input.AgreementNo) == "" {
		return apperror.Validation("Soal lisensi vendor wajib mencantumkan nomor perjanjian.",
			map[string]string{"agreementNo": "Isi nomor PKS/perjanjian."})
	}
	if _, err := parseOptionalTime(input.ValidUntil); err != nil {
		return err
	}
	return nil
}

// LicensesByQuestion mengelompokkan izin per soal.
//
// Dipakai panel bank soal untuk menampilkan ringkasan izin tiap baris soal
// tanpa permintaan tambahan per soal.
func (s *LicenseService) LicensesByQuestion(ctx context.Context, questionIDs []string) (map[string][]models.QuestionLicense, error) {
	return s.licenses.LicensesByQuestion(ctx, questionIDs)
}

// Create menyimpan izin soal baru.
func (s *LicenseService) Create(ctx context.Context, req dto.CreateLicenseRequest) (*dto.LicenseView, error) {
	question, err := s.questions.FindQuestion(ctx, req.QuestionID)
	if err != nil {
		return nil, err
	}

	validFrom, err := parseOptionalTime(req.ValidFrom)
	if err != nil {
		return nil, apperror.Validation("Tanggal mulai izin tidak valid.", nil)
	}
	validUntil, err := parseOptionalTime(req.ValidUntil)
	if err != nil {
		return nil, apperror.Validation("Tanggal berakhir izin tidak valid.", nil)
	}
	if err := validateLicensePeriod(validFrom, validUntil); err != nil {
		return nil, err
	}

	license := models.QuestionLicense{
		QuestionID:     question.ID,
		Scope:          models.LicenseScope(req.Scope),
		Owner:          strings.TrimSpace(req.Owner),
		AgreementNo:    strings.TrimSpace(req.AgreementNo),
		AgreementURL:   optionalString(req.AgreementURL),
		CommercialUse:  boolOr(req.CommercialUse, true),
		SelectionUse:   boolOr(req.SelectionUse, false),
		AllowedSectors: models.StringList(req.AllowedSectors),
		Status:         licenseStatusOr(req.Status),
		Notes:          strings.TrimSpace(req.Notes),
		ValidUntil:     validUntil,
	}
	if validFrom != nil {
		license.ValidFrom = *validFrom
	} else {
		license.ValidFrom = time.Now().UTC()
	}

	if err := s.licenses.CreateLicense(ctx, &license); err != nil {
		return nil, err
	}
	s.invalidate(ctx)

	view := toLicenseView(license, *question)
	return &view, nil
}

// Update mengubah sebagian data izin soal.
func (s *LicenseService) Update(ctx context.Context, id string, req dto.UpdateLicenseRequest) (*dto.LicenseView, error) {
	license, err := s.licenses.FindLicense(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Scope != nil {
		license.Scope = models.LicenseScope(*req.Scope)
	}
	if req.Owner != nil {
		license.Owner = strings.TrimSpace(*req.Owner)
	}
	if req.AgreementNo != nil {
		license.AgreementNo = strings.TrimSpace(*req.AgreementNo)
	}
	if req.AgreementURL != nil {
		license.AgreementURL = optionalString(*req.AgreementURL)
	}
	if req.CommercialUse != nil {
		license.CommercialUse = *req.CommercialUse
	}
	if req.SelectionUse != nil {
		license.SelectionUse = *req.SelectionUse
	}
	if req.AllowedSectors != nil {
		license.AllowedSectors = models.StringList(*req.AllowedSectors)
	}
	if req.ValidFrom != nil {
		parsed, err := parseOptionalTime(*req.ValidFrom)
		if err != nil {
			return nil, apperror.Validation("Tanggal mulai izin tidak valid.", nil)
		}
		if parsed != nil {
			license.ValidFrom = *parsed
		}
	}
	if req.ValidUntil != nil {
		parsed, err := parseOptionalTime(*req.ValidUntil)
		if err != nil {
			return nil, apperror.Validation("Tanggal berakhir izin tidak valid.", nil)
		}
		license.ValidUntil = parsed
	}
	if req.Status != nil {
		license.Status = models.LicenseStatus(*req.Status)
	}
	if req.Notes != nil {
		license.Notes = strings.TrimSpace(*req.Notes)
	}

	if err := validateLicensePeriod(&license.ValidFrom, license.ValidUntil); err != nil {
		return nil, err
	}
	if err := s.licenses.UpdateLicense(ctx, license); err != nil {
		return nil, err
	}
	s.invalidate(ctx)

	question, err := s.questions.FindQuestion(ctx, license.QuestionID)
	if err != nil {
		return nil, err
	}
	view := toLicenseView(*license, *question)
	return &view, nil
}

// Delete menghapus izin soal.
func (s *LicenseService) Delete(ctx context.Context, id string) error {
	if err := s.licenses.DeleteLicense(ctx, id); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// EnsureForQuestion memastikan sebuah soal punya izin.
//
// Dipakai saat soal dibuat/diimpor: bila admin tidak mengirim data izin, soal
// dianggap dibuat sendiri (scope internal) sehingga pengerjaan tryout tidak
// terhalang oleh kolom yang belum diisi. Soal dari vendor tetap wajib memuat
// nomor perjanjian agar klaim izinnya bisa ditelusuri.
func (s *LicenseService) EnsureForQuestion(ctx context.Context, questionID string, input *dto.QuestionLicenseInput) (*models.QuestionLicense, error) {
	if questionID == "" {
		return nil, nil
	}

	license := models.QuestionLicense{
		QuestionID:    questionID,
		Scope:         models.LicenseInternal,
		Owner:         "BUMN Academy",
		CommercialUse: true,
		Status:        models.LicenseActive,
		ValidFrom:     time.Now().UTC(),
	}
	if input != nil {
		if input.Scope != "" {
			license.Scope = models.LicenseScope(input.Scope)
		}
		if input.Owner != "" {
			license.Owner = strings.TrimSpace(input.Owner)
		}
		license.AgreementNo = strings.TrimSpace(input.AgreementNo)
		license.AgreementURL = optionalString(input.AgreementURL)
		license.CommercialUse = boolOr(input.CommercialUse, true)
		license.SelectionUse = boolOr(input.SelectionUse, false)
		license.AllowedSectors = models.StringList(input.AllowedSectors)
		license.Notes = strings.TrimSpace(input.Notes)

		validUntil, err := parseOptionalTime(input.ValidUntil)
		if err != nil {
			return nil, apperror.Validation("Tanggal berakhir izin tidak valid.", nil)
		}
		license.ValidUntil = validUntil
	}

	if license.Scope == models.LicenseVendor && license.AgreementNo == "" {
		return nil, apperror.Validation("Soal lisensi vendor wajib mencantumkan nomor perjanjian.", nil)
	}

	if err := s.licenses.CreateLicense(ctx, &license); err != nil {
		return nil, err
	}
	// Cache bank soal dibersihkan oleh pemanggil: pembuatan soal tunggal maupun
	// import massal sama-sama memanggilnya sekali di akhir.
	return &license, nil
}

// EnsureLicensesForQuestions menyimpan izin untuk soal yang belum punya izin.
//
// Dipakai import massal: soal yang sudah berizin tidak diubah (izin yang sudah
// ditinjau manusia tidak boleh tertimpa oleh berkas import), dan invalidasi
// cache dilakukan sekali oleh pemanggil.
func (s *LicenseService) EnsureLicensesForQuestions(ctx context.Context, inputs map[string]*dto.QuestionLicenseInput) (int, error) {
	if len(inputs) == 0 {
		return 0, nil
	}

	ids := make([]string, 0, len(inputs))
	for id := range inputs {
		ids = append(ids, id)
	}

	existing, err := s.licenses.LicensesByQuestion(ctx, ids)
	if err != nil {
		return 0, err
	}

	created := 0
	for id, input := range inputs {
		if len(existing[id]) > 0 {
			continue
		}
		if _, err := s.EnsureForQuestion(ctx, id, input); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// Summary merangkum kepatuhan izin bank soal.
func (s *LicenseService) Summary(ctx context.Context) (*dto.LicenseSummaryView, error) {
	_, total, err := s.licenses.ListLicenses(ctx, repository.LicenseFilter{}, 1, 1)
	if err != nil {
		return nil, err
	}
	unlicensed, err := s.licenses.CountUnlicensedQuestions(ctx)
	if err != nil {
		return nil, err
	}
	byScope, err := s.licenses.CountLicensesByScope(ctx)
	if err != nil {
		return nil, err
	}
	unlicensedRows, err := s.licenses.ListUnlicensedQuestions(ctx, 20)
	if err != nil {
		return nil, err
	}
	_, problemTotal, err := s.licenses.ListLicenses(ctx, repository.LicenseFilter{OnlyProblem: true}, 1, 1)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	until := now.AddDate(0, 0, licenseExpiringWindowDays)
	expiring, err := s.licenses.ListExpiringLicenses(ctx, now, until, 20)
	if err != nil {
		return nil, err
	}

	view := &dto.LicenseSummaryView{
		TotalLicenses:       total,
		UnlicensedQuestions: unlicensed,
		ExpiringSoon:        len(expiring),
		ProblemLicenses:     int(problemTotal),
		ByScope:             byScope,
		EnforcementEnabled:  s.cfg.Business.QuestionLicenseEnforce,
		ExpiringWindowDays:  licenseExpiringWindowDays,
		GeneratedAt:         formatTime(now),
	}

	view.Unlicensed = make([]dto.UnlicensedQuestionView, 0, len(unlicensedRows))
	for _, row := range unlicensedRows {
		view.Unlicensed = append(view.Unlicensed, dto.UnlicensedQuestionView{
			ID:            row.ID,
			BankKey:       row.BankKey,
			BankLabel:     bank.Label(row.BankKey),
			SectorKey:     row.SectorKey,
			SectorLabel:   bank.SectorLabel(row.SectorKey),
			TestType:      row.TestType,
			TestTypeLabel: bank.TestTypeLabel(row.TestType),
			Text:          row.Text,
			Status:        row.Status,
			ValidUntil:    formatTimePtr(row.ValidUntil),
			HasAnyLicense: row.HasAnyLicense,
			UpdatedAt:     formatTime(row.UpdatedAt),
		})
	}

	questions, err := s.questionIndex(ctx, licenseQuestionIDs(expiring))
	if err != nil {
		return nil, err
	}
	view.Expiring = make([]dto.LicenseView, 0, len(expiring))
	for _, row := range expiring {
		view.Expiring = append(view.Expiring, toLicenseView(row, questions[row.QuestionID]))
	}
	return view, nil
}

/* --------------------------- instrumen psikologi -------------------------- */

// ListInstruments mengambil daftar instrumen psikologi.
func (s *LicenseService) ListInstruments(ctx context.Context, filter repository.InstrumentFilter) ([]dto.InstrumentView, error) {
	rows, err := s.licenses.ListInstruments(ctx, filter)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := make([]dto.InstrumentView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInstrumentView(row, now))
	}
	return out, nil
}

// CreateInstrument menyimpan instrumen psikologi baru.
func (s *LicenseService) CreateInstrument(ctx context.Context, req dto.CreateInstrumentRequest) (*dto.InstrumentView, error) {
	if _, err := s.licenses.FindInstrumentByCode(ctx, req.Code); err == nil {
		return nil, apperror.Conflict("Kode instrumen sudah dipakai.")
	}

	licenseFrom, err := parseOptionalTime(req.LicenseFrom)
	if err != nil {
		return nil, apperror.Validation("Tanggal mulai lisensi instrumen tidak valid.", nil)
	}
	licenseUntil, err := parseOptionalTime(req.LicenseUntil)
	if err != nil {
		return nil, apperror.Validation("Tanggal berakhir lisensi instrumen tidak valid.", nil)
	}

	instrument := models.PsychologicalInstrument{
		Code:           strings.TrimSpace(req.Code),
		Name:           strings.TrimSpace(req.Name),
		Kind:           models.InstrumentKind(req.Kind),
		Vendor:         strings.TrimSpace(req.Vendor),
		LicenseNo:      strings.TrimSpace(req.LicenseNo),
		LicenseURL:     optionalString(req.LicenseURL),
		LicenseFrom:    licenseFrom,
		LicenseUntil:   licenseUntil,
		Status:         licenseStatusOr(req.Status),
		AdministeredBy: optionalString(req.AdministeredBy),
		ScoringNotes:   strings.TrimSpace(req.ScoringNotes),
		Active:         req.Active == nil || *req.Active,
	}
	if err := s.validateInstrument(instrument); err != nil {
		return nil, err
	}
	if err := s.licenses.CreateInstrument(ctx, &instrument); err != nil {
		return nil, err
	}

	view := toInstrumentView(instrument, time.Now().UTC())
	return &view, nil
}

// UpdateInstrument mengubah sebagian data instrumen psikologi.
func (s *LicenseService) UpdateInstrument(ctx context.Context, id string, req dto.UpdateInstrumentRequest) (*dto.InstrumentView, error) {
	instrument, err := s.licenses.FindInstrument(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		instrument.Name = strings.TrimSpace(*req.Name)
	}
	if req.Kind != nil {
		instrument.Kind = models.InstrumentKind(*req.Kind)
	}
	if req.Vendor != nil {
		instrument.Vendor = strings.TrimSpace(*req.Vendor)
	}
	if req.LicenseNo != nil {
		instrument.LicenseNo = strings.TrimSpace(*req.LicenseNo)
	}
	if req.LicenseURL != nil {
		instrument.LicenseURL = optionalString(*req.LicenseURL)
	}
	if req.LicenseFrom != nil {
		parsed, err := parseOptionalTime(*req.LicenseFrom)
		if err != nil {
			return nil, apperror.Validation("Tanggal mulai lisensi instrumen tidak valid.", nil)
		}
		instrument.LicenseFrom = parsed
	}
	if req.LicenseUntil != nil {
		parsed, err := parseOptionalTime(*req.LicenseUntil)
		if err != nil {
			return nil, apperror.Validation("Tanggal berakhir lisensi instrumen tidak valid.", nil)
		}
		instrument.LicenseUntil = parsed
	}
	if req.Status != nil {
		instrument.Status = models.LicenseStatus(*req.Status)
	}
	if req.AdministeredBy != nil {
		instrument.AdministeredBy = optionalString(*req.AdministeredBy)
	}
	if req.ScoringNotes != nil {
		instrument.ScoringNotes = strings.TrimSpace(*req.ScoringNotes)
	}
	if req.Active != nil {
		instrument.Active = *req.Active
	}

	if err := s.validateInstrument(*instrument); err != nil {
		return nil, err
	}
	if err := s.licenses.UpdateInstrument(ctx, instrument); err != nil {
		return nil, err
	}

	view := toInstrumentView(*instrument, time.Now().UTC())
	return &view, nil
}

// DeleteInstrument menghapus instrumen psikologi.
func (s *LicenseService) DeleteInstrument(ctx context.Context, id string) error {
	return s.licenses.DeleteInstrument(ctx, id)
}

// validateInstrument menegakkan aturan izin instrumen.
//
// Instrumen berlisensi wajib punya vendor, nomor lisensi, dan psikolog
// penanggung jawab: tanpa itu instrumen tidak boleh dijalankan siapa pun.
func (s *LicenseService) validateInstrument(instrument models.PsychologicalInstrument) error {
	if !instrument.Kind.Valid() {
		return apperror.Validation("Jenis instrumen tidak dikenal.", nil)
	}
	if instrument.Kind != models.InstrumentLicensed {
		return nil
	}

	missing := make(map[string]string)
	if instrument.Vendor == "" {
		missing["vendor"] = "Vendor/pemilik alat tes wajib diisi."
	}
	if instrument.LicenseNo == "" {
		missing["licenseNo"] = "Nomor lisensi alat tes wajib diisi."
	}
	if instrument.AdministeredBy == nil || *instrument.AdministeredBy == "" {
		missing["administeredBy"] = "Psikolog penanggung jawab wajib diisi."
	}
	if len(missing) > 0 {
		return apperror.Validation("Instrumen berlisensi belum lengkap.", missing)
	}
	return nil
}

/* --------------------------------- helpers -------------------------------- */

// invalidate membersihkan cache bank soal setelah izin berubah, agar soal yang
// baru dicabut tidak tetap terkirim ke peserta.
func (s *LicenseService) invalidate(ctx context.Context) {
	if s.tryoutSvc != nil {
		s.tryoutSvc.InvalidateBankCache(ctx)
	}
}

// questionIndex memuat metadata soal yang dipakai melengkapi tampilan izin.
func (s *LicenseService) questionIndex(ctx context.Context, ids []string) (map[string]models.Question, error) {
	if len(ids) == 0 {
		return map[string]models.Question{}, nil
	}
	questions, err := s.questions.ListQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.Question, len(questions))
	for _, question := range questions {
		out[question.ID] = question
	}
	return out, nil
}

// licenseQuestionIDs mengumpulkan id soal dari daftar izin.
func licenseQuestionIDs(rows []models.QuestionLicense) []string {
	seen := make(map[string]struct{}, len(rows))
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.QuestionID]; ok {
			continue
		}
		seen[row.QuestionID] = struct{}{}
		out = append(out, row.QuestionID)
	}
	return out
}

// toLicenseView membentuk tampilan izin; `question` boleh kosong bila soalnya
// sudah terhapus.
func toLicenseView(license models.QuestionLicense, question models.Question) dto.LicenseView {
	now := time.Now().UTC()
	return dto.LicenseView{
		ID:              license.ID,
		QuestionID:      license.QuestionID,
		QuestionText:    question.Text,
		BankKey:         question.BankKey,
		SectorKey:       question.SectorKey,
		Scope:           string(license.Scope),
		ScopeLabel:      licenseScopeLabels[license.Scope],
		Owner:           license.Owner,
		AgreementNo:     license.AgreementNo,
		AgreementURL:    strPtrOrEmpty(license.AgreementURL),
		CommercialUse:   license.CommercialUse,
		SelectionUse:    license.SelectionUse,
		AllowedSectors:  emptyIfNil(license.AllowedSectors),
		ValidFrom:       formatTime(license.ValidFrom),
		ValidUntil:      formatTimePtr(license.ValidUntil),
		Status:          string(license.Status),
		StatusEffective: string(license.EffectiveStatus(now)),
		Notes:           license.Notes,
		CreatedAt:       formatTime(license.CreatedAt),
		UpdatedAt:       formatTime(license.UpdatedAt),
	}
}

// toInstrumentView membentuk tampilan instrumen psikologi.
func toInstrumentView(instrument models.PsychologicalInstrument, now time.Time) dto.InstrumentView {
	view := dto.InstrumentView{
		ID:              instrument.ID,
		Code:            instrument.Code,
		Name:            instrument.Name,
		Kind:            string(instrument.Kind),
		KindLabel:       instrumentKindLabels[instrument.Kind],
		Vendor:          instrument.Vendor,
		LicenseNo:       instrument.LicenseNo,
		LicenseURL:      strPtrOrEmpty(instrument.LicenseURL),
		LicenseFrom:     formatTimePtr(instrument.LicenseFrom),
		LicenseUntil:    formatTimePtr(instrument.LicenseUntil),
		Status:          string(instrument.Status),
		StatusEffective: string(instrument.EffectiveStatus(now)),
		AdministeredBy:  strPtrOrEmpty(instrument.AdministeredBy),
		ScoringNotes:    instrument.ScoringNotes,
		Active:          instrument.Active,
		Usable:          instrument.Usable(now),
		CreatedAt:       formatTime(instrument.CreatedAt),
		UpdatedAt:       formatTime(instrument.UpdatedAt),
	}
	if !view.Usable {
		switch {
		case !instrument.Active:
			view.UsableNote = "Instrumen sedang dinonaktifkan."
		case view.StatusEffective == string(models.LicenseRevoked):
			view.UsableNote = "Izin instrumen dicabut."
		case view.StatusEffective == string(models.LicenseExpired):
			view.UsableNote = "Masa berlaku lisensi sudah berakhir."
		default:
			view.UsableNote = "Instrumen berlisensi wajib punya psikolog penanggung jawab."
		}
	}
	return view
}

// parseOptionalTime membaca tanggal dari klien; menerima RFC3339 maupun format
// tanggal sederhana (YYYY-MM-DD). String kosong menghasilkan nil.
func parseOptionalTime(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			utc := parsed.UTC()
			return &utc, nil
		}
	}
	return nil, apperror.Validation("Format tanggal tidak dikenal.", nil)
}

// validateLicensePeriod memastikan tanggal berakhir tidak mendahului tanggal mulai.
func validateLicensePeriod(from *time.Time, until *time.Time) error {
	if from == nil || until == nil {
		return nil
	}
	if until.Before(*from) {
		return apperror.Validation("Tanggal berakhir izin tidak boleh mendahului tanggal mulai.", nil)
	}
	return nil
}

// licenseStatusOr mengembalikan status izin dari string; kosong berarti aktif.
func licenseStatusOr(value string) models.LicenseStatus {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return models.LicenseActive
	}
	return models.LicenseStatus(trimmed)
}

// boolOr mengembalikan nilai pointer, atau `fallback` bila nil.
func boolOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
