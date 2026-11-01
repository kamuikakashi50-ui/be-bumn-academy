package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// maxEvidenceBytes adalah batas ukuran berkas bukti pengumuman (5 MB) — cukup
// untuk tangkapan layar atau PDF pengumuman, tanpa membuka peluang unggahan besar.
const maxEvidenceBytes = 5 << 20

// evidenceExtensions adalah jenis berkas bukti yang diterima. Daftarnya sengaja
// sama dengan ekstensi yang boleh disajikan `MediaHandler`, sehingga berkas yang
// tersimpan pasti bisa dibuka kembali.
var evidenceExtensions = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".webp": {},
	".pdf":  {},
}

// OutcomeService menjalankan tracer study hasil seleksi (Fase 2): laporan peserta,
// unggahan bukti, dan verifikasi admin.
//
// Aturan bisnis yang dipegang service ini (§4.1/§4.2):
//   - satu baris per kombinasi (peserta, tahun, tahap, instansi); laporan kedua
//     ditolak `CONFLICT` dan diperbaiki lewat `PATCH /outcomes/{id}`;
//   - peserta hanya boleh menyentuh laporannya sendiri;
//   - laporan yang sudah diverifikasi lalu diubah peserta **kembali** ke
//     `self_reported` dan masuk daftar tinjauan admin;
//   - setiap perubahan/verifikasi dicatat pada jejak audit agar perubahan angka
//     KPI bisa ditelusuri.
type OutcomeService struct {
	outcomes *repository.OutcomeRepository
	cohorts  *repository.CohortRepository
	cfg      *config.Config
}

// NewOutcomeService membentuk service KPI kelulusan.
func NewOutcomeService(
	outcomes *repository.OutcomeRepository,
	cohorts *repository.CohortRepository,
	cfg *config.Config,
) *OutcomeService {
	return &OutcomeService{outcomes: outcomes, cohorts: cohorts, cfg: cfg}
}

/* ---------------------------- helper murni -------------------------------- */

// parseDateOnly mengubah tanggal `YYYY-MM-DD` menjadi waktu UTC.
//
// Mengembalikan nil (tanpa galat) untuk nilai kosong: tanggal pengumuman bersifat
// opsional karena peserta boleh melapor sebelum pengumuman resmi keluar.
func parseDateOnly(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.DateOnly, trimmed)
	if err != nil {
		return nil, apperror.Validation("Tanggal pengumuman harus berformat YYYY-MM-DD.", nil)
	}
	return &parsed, nil
}

// validateEvidenceFile memeriksa jenis & ukuran berkas bukti.
func validateEvidenceFile(filename string, size int) error {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if _, ok := evidenceExtensions[ext]; !ok {
		return apperror.Validation(
			"Bukti harus berupa gambar (PNG/JPG/WebP) atau dokumen PDF.", nil)
	}
	if size <= 0 {
		return apperror.Validation("Berkas bukti kosong.", nil)
	}
	if size > maxEvidenceBytes {
		return apperror.Validation(
			fmt.Sprintf("Ukuran berkas bukti maksimal %d MB.", maxEvidenceBytes/(1<<20)), nil)
	}
	return nil
}

// formatDateOnly membentuk tanggal `YYYY-MM-DD` dari kolom DATE.
func formatDateOnly(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.DateOnly)
}

// outcomeStatusFromInput memvalidasi status hasil seleksi dari masukan klien.
func outcomeStatusFromInput(value string) (models.OutcomeStatus, error) {
	status := models.OutcomeStatus(strings.TrimSpace(value))
	if !status.Valid() {
		return "", apperror.Validation("Status hasil seleksi tidak dikenal.", nil)
	}
	return status, nil
}

// selectionYearValid memeriksa rentang tahun seleksi yang wajar (§4.1).
//
// Batasnya sama dengan constraint database & tag validasi DTO; pemeriksaan di
// service menjaga aturan tetap berlaku bila service dipanggil dari jalur lain.
func selectionYearValid(year int) bool { return year >= 2020 && year <= 2100 }

// verifySource menentukan tingkat verifikasi dari pilihan admin.
func verifySource(documentVerified bool) models.OutcomeSource {
	if documentVerified {
		return models.OutcomeDocumentVerified
	}
	return models.OutcomeAdminVerified
}

// resetVerificationAfterParticipantEdit mengembalikan laporan ke status belum
// terverifikasi bila sebelumnya sudah diperiksa admin (§4.1).
//
// Mengembalikan true bila verifikasi direset. Dipakai agar angka KPI tidak lagi
// memakai laporan yang isinya sudah berbeda dari yang diperiksa admin.
func resetVerificationAfterParticipantEdit(outcome *models.SelectionOutcome) bool {
	if !outcome.Verified() {
		return false
	}
	outcome.Source = models.OutcomeSelfReported
	outcome.VerifiedBy = nil
	outcome.VerifiedAt = nil
	return true
}

// validateRejectReason memeriksa alasan penolakan bukti.
//
// Alasan wajib diisi: peserta harus tahu apa yang perlu diperbaiki, dan angka KPI
// tidak boleh turun tanpa penjelasan yang bisa dibaca peserta.
func validateRejectReason(note string) (string, error) {
	reason := strings.TrimSpace(note)
	if len(reason) < 5 {
		return "", apperror.Validation("Alasan penolakan wajib diisi (minimal 5 karakter).", nil)
	}
	return reason, nil
}

// outcomeViewFromRow membentuk tampilan laporan dari baris hasil join.
func outcomeViewFromRow(row repository.OutcomeListRow) dto.OutcomeView {
	outcome := row.SelectionOutcome

	view := dto.OutcomeView{
		ID:                  outcome.ID,
		UserID:              outcome.UserID,
		UserName:            row.UserName,
		UserEmail:           row.UserEmail,
		UserProgram:         row.UserProgram,
		CohortName:          row.CohortName,
		StageKey:            outcome.StageKey,
		StageLabel:          row.StageLabel,
		StageFinal:          row.StageFinal,
		CompanyName:         outcome.CompanyName,
		CompanySector:       outcome.CompanySector,
		CompanySectorLabel:  bank.SectorLabel(outcome.CompanySector),
		Position:            outcome.Position,
		PositionFamily:      outcome.PositionFamily,
		PositionFamilyLabel: bank.PositionFamilyLabel(outcome.PositionFamily),
		SelectionYear:       outcome.SelectionYear,
		Status:              string(outcome.Status),
		StatusLabel:         outcome.Status.Label(),
		AnnouncedAt:         formatDateOnly(outcome.AnnouncedAt),
		Source:              string(outcome.Source),
		SourceLabel:         outcome.Source.Label(),
		Verified:            outcome.Verified(),
		EvidenceURL:         strPtrOrEmpty(outcome.EvidenceURL),
		Note:                outcome.Note,
		AggregateConsent:    outcome.AggregateConsent,
		VerifiedAt:          formatTimePtr(outcome.VerifiedAt),
		CreatedAt:           formatTime(outcome.CreatedAt),
		UpdatedAt:           formatTime(outcome.UpdatedAt),
	}
	if outcome.CohortID != nil {
		view.CohortID = *outcome.CohortID
	}
	return view
}

// outcomeViewFromModel membentuk tampilan laporan dari model + tahapannya.
//
// Dipakai setelah penulisan (create/update) ketika relasi belum ikut dibaca.
func outcomeViewFromModel(outcome models.SelectionOutcome, stage models.SelectionStage) dto.OutcomeView {
	view := dto.OutcomeView{
		ID:                  outcome.ID,
		UserID:              outcome.UserID,
		StageKey:            outcome.StageKey,
		StageLabel:          stage.Label,
		StageFinal:          stage.IsFinal,
		CompanyName:         outcome.CompanyName,
		CompanySector:       outcome.CompanySector,
		CompanySectorLabel:  bank.SectorLabel(outcome.CompanySector),
		Position:            outcome.Position,
		PositionFamily:      outcome.PositionFamily,
		PositionFamilyLabel: bank.PositionFamilyLabel(outcome.PositionFamily),
		SelectionYear:       outcome.SelectionYear,
		Status:              string(outcome.Status),
		StatusLabel:         outcome.Status.Label(),
		AnnouncedAt:         formatDateOnly(outcome.AnnouncedAt),
		Source:              string(outcome.Source),
		SourceLabel:         outcome.Source.Label(),
		Verified:            outcome.Verified(),
		EvidenceURL:         strPtrOrEmpty(outcome.EvidenceURL),
		Note:                outcome.Note,
		AggregateConsent:    outcome.AggregateConsent,
		VerifiedAt:          formatTimePtr(outcome.VerifiedAt),
		CreatedAt:           formatTime(outcome.CreatedAt),
		UpdatedAt:           formatTime(outcome.UpdatedAt),
	}
	if outcome.CohortID != nil {
		view.CohortID = *outcome.CohortID
	}
	return view
}

// stageViewFromModel membentuk tampilan tahapan seleksi.
func stageViewFromModel(stage models.SelectionStage) dto.SelectionStageView {
	return dto.SelectionStageView{
		Key:       stage.Key,
		Label:     stage.Label,
		Phase:     stage.Phase,
		IsFinal:   stage.IsFinal,
		SortOrder: stage.SortOrder,
		Active:    stage.Active,
	}
}

// recordAudit mencatat satu aksi pada jejak audit hasil seleksi.
//
// Kegagalan pencatatan tidak menggagalkan aksi utama: jejak audit penting untuk
// penelusuran, tetapi menolak laporan peserta karena audit gagal justru merugikan.
func (s *OutcomeService) recordAudit(ctx context.Context, outcomeID, actorID, action, note string) {
	audit := &models.SelectionOutcomeAudit{
		OutcomeID: outcomeID,
		Action:    action,
		Note:      note,
	}
	if strings.TrimSpace(actorID) != "" {
		actor := actorID
		audit.ActorID = &actor
	}
	_ = s.outcomes.AppendOutcomeAudit(ctx, audit)
}

// evidenceDir mengembalikan direktori penyimpanan bukti pengumuman.
func (s *OutcomeService) evidenceDir() string {
	return filepath.Join(s.cfg.App.MediaDir, "outcomes")
}

// saveEvidence menulis berkas bukti ke direktori media dan mengembalikan URL relatif.
//
// URL memakai bentuk `/media/...` sehingga dilayani `MediaHandler` (yang sudah
// menyajikan ekstensi ini) tanpa endpoint tambahan.
func (s *OutcomeService) saveEvidence(outcomeID, filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	name := fmt.Sprintf("%s-%d%s", outcomeID, time.Now().UTC().Unix(), ext)

	dir := s.evidenceDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", apperror.Internal(err, "Gagal menyiapkan penyimpanan bukti.")
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", apperror.Internal(err, "Gagal menyimpan berkas bukti.")
	}
	return "/media/outcomes/" + name, nil
}

/* --------------------------------- peserta --------------------------------- */

// Stages mengambil daftar tahapan seleksi aktif untuk mengisi formulir peserta.
func (s *OutcomeService) Stages(ctx context.Context) ([]dto.SelectionStageView, error) {
	stages, err := s.outcomes.ListStages(ctx, true)
	if err != nil {
		return nil, err
	}

	out := make([]dto.SelectionStageView, 0, len(stages))
	for _, stage := range stages {
		out = append(out, stageViewFromModel(stage))
	}
	return out, nil
}

// MyOutcomes mengambil laporan hasil seleksi milik satu peserta.
func (s *OutcomeService) MyOutcomes(ctx context.Context, userID string) ([]dto.OutcomeView, error) {
	rows, err := s.outcomes.ListOutcomes(ctx, repository.OutcomeFilter{UserID: userID})
	if err != nil {
		return nil, err
	}

	out := make([]dto.OutcomeView, 0, len(rows))
	for _, row := range rows {
		out = append(out, outcomeViewFromRow(row))
	}
	return out, nil
}

// Report menyimpan laporan hasil seleksi baru milik peserta.
//
// Kombinasi (peserta, tahun, tahap, instansi) bersifat unik (§4.1), sehingga laporan
// kedua pada kombinasi yang sama ditolak `CONFLICT`: klien harus memakai
// `PATCH /outcomes/{id}` untuk memperbaiki baris yang sudah ada.
func (s *OutcomeService) Report(ctx context.Context, userID string, in dto.OutcomeInput) (dto.OutcomeView, error) {
	stage, err := s.outcomes.FindStage(ctx, strings.TrimSpace(in.StageKey))
	if err != nil {
		return dto.OutcomeView{}, err
	}

	status, err := outcomeStatusFromInput(in.Status)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	if !selectionYearValid(in.SelectionYear) {
		return dto.OutcomeView{}, apperror.Validation("Tahun seleksi harus di antara 2020–2100.", nil)
	}
	announcedAt, err := parseDateOnly(in.AnnouncedAt)
	if err != nil {
		return dto.OutcomeView{}, err
	}

	company := strings.TrimSpace(in.CompanyName)
	existing, err := s.outcomes.FindOutcomeByKey(ctx, userID, in.SelectionYear, stage.Key, company)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	if existing != nil {
		return dto.OutcomeView{}, apperror.Conflict(
			"Laporan untuk tahap dan instansi ini sudah ada — perbarui laporan yang ada.")
	}

	// Kelas khusus peserta diisi otomatis saat laporan dibuat (§4.8); bila peserta
	// keluar kelas nanti, nilai ini tetap tersimpan sebagai riwayat.
	cohortID := s.activeCohortID(ctx, userID)

	outcome := models.SelectionOutcome{
		UserID:           userID,
		CohortID:         cohortID,
		StageKey:         stage.Key,
		CompanyName:      company,
		CompanySector:    bank.SectorOrDefault(in.CompanySector),
		Position:         strings.TrimSpace(in.Position),
		PositionFamily:   bank.PositionFamilyOrDefault(in.PositionFamily),
		SelectionYear:    in.SelectionYear,
		Status:           status,
		AnnouncedAt:      announcedAt,
		Source:           models.OutcomeSelfReported,
		Note:             strings.TrimSpace(in.Note),
		AggregateConsent: in.AggregateConsent == nil || *in.AggregateConsent,
	}
	if err := s.outcomes.CreateOutcome(ctx, &outcome); err != nil {
		return dto.OutcomeView{}, err
	}

	s.recordAudit(ctx, outcome.ID, userID, models.SelectionActionReported,
		fmt.Sprintf("Laporan %s pada tahap %s dilaporkan peserta.", status.Label(), stage.Label))

	return outcomeViewFromModel(outcome, *stage), nil
}

// activeCohortID mengembalikan kelas khusus aktif peserta (bila ada).
func (s *OutcomeService) activeCohortID(ctx context.Context, userID string) *string {
	cohorts, err := s.cohorts.ActiveCohortsForUser(ctx, userID, time.Now().UTC())
	if err != nil || len(cohorts) == 0 {
		return nil
	}
	id := cohorts[0].ID
	return &id
}

// findOwnedOutcome mengambil laporan milik peserta tertentu.
//
// Laporan milik peserta lain tidak boleh terlihat maupun berubah — kesalahan ini
// biasanya berbentuk IDOR, jadi pemeriksaannya ada di satu tempat.
func (s *OutcomeService) findOwnedOutcome(ctx context.Context, userID, id string) (*repository.OutcomeListRow, error) {
	row, err := s.outcomes.FindOutcome(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, apperror.Forbidden("Laporan ini bukan milik Anda.")
	}
	return row, nil
}

// UpdateOutcome memperbaiki laporan milik peserta sendiri.
//
// Bila laporan itu sudah diverifikasi admin, perubahan peserta mengembalikannya ke
// `self_reported` dan mengosongkan jejak verifikasi (§4.1) supaya angka KPI tidak
// memakai data yang sudah berbeda dari yang diperiksa admin.
func (s *OutcomeService) UpdateOutcome(
	ctx context.Context,
	userID string,
	id string,
	patch dto.OutcomePatch,
) (dto.OutcomeView, error) {
	row, err := s.findOwnedOutcome(ctx, userID, id)
	if err != nil {
		return dto.OutcomeView{}, err
	}

	outcome := row.SelectionOutcome

	stage, err := s.outcomes.FindStage(ctx, outcome.StageKey)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	if patch.StageKey != nil {
		stage, err = s.outcomes.FindStage(ctx, strings.TrimSpace(*patch.StageKey))
		if err != nil {
			return dto.OutcomeView{}, err
		}
		outcome.StageKey = stage.Key
	}
	if patch.CompanyName != nil {
		outcome.CompanyName = strings.TrimSpace(*patch.CompanyName)
	}
	if patch.CompanySector != nil {
		outcome.CompanySector = bank.SectorOrDefault(*patch.CompanySector)
	}
	if patch.Position != nil {
		outcome.Position = strings.TrimSpace(*patch.Position)
	}
	if patch.PositionFamily != nil {
		outcome.PositionFamily = bank.PositionFamilyOrDefault(*patch.PositionFamily)
	}
	if patch.SelectionYear != nil {
		if !selectionYearValid(*patch.SelectionYear) {
			return dto.OutcomeView{}, apperror.Validation("Tahun seleksi harus di antara 2020–2100.", nil)
		}
		outcome.SelectionYear = *patch.SelectionYear
	}
	if patch.Status != nil {
		status, err := outcomeStatusFromInput(*patch.Status)
		if err != nil {
			return dto.OutcomeView{}, err
		}
		outcome.Status = status
	}
	if patch.AnnouncedAt != nil {
		announcedAt, err := parseDateOnly(*patch.AnnouncedAt)
		if err != nil {
			return dto.OutcomeView{}, err
		}
		outcome.AnnouncedAt = announcedAt
	}
	if patch.Note != nil {
		outcome.Note = strings.TrimSpace(*patch.Note)
	}
	if patch.AggregateConsent != nil {
		outcome.AggregateConsent = *patch.AggregateConsent
	}

	// Kombinasi baru tidak boleh bentrok dengan laporan lain milik peserta yang sama.
	other, err := s.outcomes.FindOutcomeByKey(
		ctx, userID, outcome.SelectionYear, outcome.StageKey, outcome.CompanyName)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	if other != nil && other.ID != outcome.ID {
		return dto.OutcomeView{}, apperror.Conflict(
			"Laporan untuk tahap dan instansi ini sudah ada.")
	}

	auditNote := "Laporan diperbarui peserta."
	if resetVerificationAfterParticipantEdit(&outcome) {
		auditNote = "Laporan yang sudah terverifikasi diubah peserta; status verifikasi direset."
	}

	if err := s.outcomes.SaveOutcome(ctx, &outcome); err != nil {
		return dto.OutcomeView{}, err
	}
	s.recordAudit(ctx, outcome.ID, userID, models.SelectionActionUpdated, auditNote)

	// Dibaca ulang agar `updatedAt` (dan nama kelas) pada balasan benar-benar
	// mencerminkan baris yang tersimpan.
	return s.reloadView(ctx, outcome.ID)
}

// DeleteOutcome membatalkan laporan milik peserta sendiri.
func (s *OutcomeService) DeleteOutcome(ctx context.Context, userID, id string) error {
	if _, err := s.findOwnedOutcome(ctx, userID, id); err != nil {
		return err
	}

	// Jejak audit ditulis lebih dulu: baris audit ikut terhapus bersama laporannya
	// (ON DELETE CASCADE), tetapi jejaknya tetap ada bila kebijakan itu diubah.
	s.recordAudit(ctx, id, userID, models.SelectionActionDeleted, "Laporan dibatalkan peserta.")
	return s.outcomes.DeleteOutcome(ctx, id)
}

// SetEvidenceURL menautkan bukti pengumuman berupa URL milik peserta sendiri.
func (s *OutcomeService) SetEvidenceURL(ctx context.Context, userID, id, url string) (dto.OutcomeView, error) {
	if _, err := s.findOwnedOutcome(ctx, userID, id); err != nil {
		return dto.OutcomeView{}, err
	}

	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return dto.OutcomeView{}, apperror.Validation("Tautan bukti wajib diisi.", nil)
	}
	if err := s.outcomes.SetOutcomeEvidence(ctx, id, trimmed); err != nil {
		return dto.OutcomeView{}, err
	}
	s.recordAudit(ctx, id, userID, models.SelectionActionUpdated, "Tautan bukti pengumuman diperbarui.")
	return s.reloadView(ctx, id)
}

// UploadEvidence menyimpan berkas bukti pengumuman milik peserta sendiri.
//
// Berkas ditulis ke direktori media dan disajikan `MediaHandler` lewat `/media/...`,
// sehingga tidak perlu endpoint pengunduhan khusus.
func (s *OutcomeService) UploadEvidence(
	ctx context.Context,
	userID string,
	id string,
	filename string,
	data []byte,
) (dto.OutcomeView, error) {
	if _, err := s.findOwnedOutcome(ctx, userID, id); err != nil {
		return dto.OutcomeView{}, err
	}
	if err := validateEvidenceFile(filename, len(data)); err != nil {
		return dto.OutcomeView{}, err
	}

	url, err := s.saveEvidence(id, filename, data)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	if err := s.outcomes.SetOutcomeEvidence(ctx, id, url); err != nil {
		return dto.OutcomeView{}, err
	}
	s.recordAudit(ctx, id, userID, models.SelectionActionUpdated, "Bukti pengumuman diunggah peserta.")
	return s.reloadView(ctx, id)
}

// reloadView membaca ulang satu laporan beserta relasinya.
func (s *OutcomeService) reloadView(ctx context.Context, id string) (dto.OutcomeView, error) {
	row, err := s.outcomes.FindOutcome(ctx, id)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	return outcomeViewFromRow(*row), nil
}
