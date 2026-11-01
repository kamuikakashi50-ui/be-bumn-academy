package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Sisi admin KPI kelulusan (Fase 2 langkah 3): daftar tinjauan laporan, verifikasi
// /penolakan bukti, pengelolaan tahapan seleksi, dan target KPI instansi.

// OutcomeReviewResult adalah daftar laporan untuk panel admin.
type OutcomeReviewResult struct {
	Items []dto.OutcomeView `json:"items"`
	// Pending adalah jumlah laporan yang belum diverifikasi (tanpa filter), dipakai
	// sebagai lencana "perlu ditinjau" pada menu admin.
	Pending int `json:"pending"`
	Total   int `json:"total"`
}

// AdminOutcomes mengambil daftar laporan hasil seleksi untuk admin.
func (s *OutcomeService) AdminOutcomes(
	ctx context.Context,
	filter repository.OutcomeFilter,
) (OutcomeReviewResult, error) {
	rows, err := s.outcomes.ListOutcomes(ctx, filter)
	if err != nil {
		return OutcomeReviewResult{}, err
	}
	total, err := s.outcomes.CountOutcomes(ctx, filter)
	if err != nil {
		return OutcomeReviewResult{}, err
	}

	// Jumlah tinjauan dihitung tanpa filter agar admin tetap melihat ada berapa
	// laporan yang menunggu, walau sedang menyaring satu kelas/klaster.
	pending, err := s.outcomes.CountOutcomes(ctx, repository.OutcomeFilter{OnlyPending: true})
	if err != nil {
		return OutcomeReviewResult{}, err
	}

	items := make([]dto.OutcomeView, 0, len(rows))
	for _, row := range rows {
		items = append(items, outcomeViewFromRow(row))
	}

	return OutcomeReviewResult{Items: items, Pending: pending, Total: total}, nil
}

// OutcomeAudits mengambil jejak audit satu laporan.
func (s *OutcomeService) OutcomeAudits(ctx context.Context, outcomeID string, limit int) ([]dto.OutcomeAuditView, error) {
	rows, err := s.outcomes.ListOutcomeAudits(ctx, outcomeID, limit)
	if err != nil {
		return nil, err
	}

	out := make([]dto.OutcomeAuditView, 0, len(rows))
	for _, row := range rows {
		view := dto.OutcomeAuditView{
			ID:        row.ID,
			OutcomeID: row.OutcomeID,
			ActorName: row.ActorName,
			Action:    row.Action,
			Note:      row.Note,
			CreatedAt: formatTime(row.CreatedAt),
		}
		if row.ActorID != nil {
			view.ActorID = *row.ActorID
		}
		out = append(out, view)
	}
	return out, nil
}

// VerifyOutcome menandai laporan sudah diperiksa admin (§4.2).
//
// Verifikasi ulang atas laporan yang sudah terverifikasi dijawab `CONFLICT` tanpa
// mengubah apa pun (idempoten), supaya angka KPI tidak berubah karena klik ganda.
func (s *OutcomeService) VerifyOutcome(
	ctx context.Context,
	actorID string,
	id string,
	in dto.OutcomeVerifyInput,
) (dto.OutcomeView, error) {
	row, err := s.outcomes.FindOutcome(ctx, id)
	if err != nil {
		return dto.OutcomeView{}, err
	}
	if row.Verified() {
		return dto.OutcomeView{}, apperror.Conflict("Laporan ini sudah terverifikasi.")
	}

	source := verifySource(in.DocumentVerified)

	evidence := strings.TrimSpace(in.EvidenceURL)
	if evidence != "" {
		if err := s.outcomes.SetOutcomeEvidence(ctx, id, evidence); err != nil {
			return dto.OutcomeView{}, err
		}
	}

	note := strings.TrimSpace(in.Note)
	if note == "" {
		note = row.Note
	}
	verifiedAt := time.Now().UTC()
	actor := actorID
	if err := s.outcomes.SetOutcomeVerification(ctx, id, source, note, &actor, &verifiedAt); err != nil {
		return dto.OutcomeView{}, err
	}

	s.recordAudit(ctx, id, actorID, models.SelectionActionVerified,
		fmt.Sprintf("Laporan diverifikasi sebagai %s.", source.Label()))

	return s.reloadView(ctx, id)
}

// RejectOutcome menolak bukti laporan dan mengembalikannya ke peserta (§4.2).
//
// Laporan tidak dihapus: status verifikasinya dikosongkan dan alasan penolakan
// disimpan pada `note`, sehingga peserta bisa memperbaikinya.
func (s *OutcomeService) RejectOutcome(
	ctx context.Context,
	actorID string,
	id string,
	in dto.OutcomeRejectInput,
) (dto.OutcomeView, error) {
	if _, err := s.outcomes.FindOutcome(ctx, id); err != nil {
		return dto.OutcomeView{}, err
	}

	reason, err := validateRejectReason(in.Note)
	if err != nil {
		return dto.OutcomeView{}, err
	}

	// `verified_by`/`verified_at` dikosongkan agar laporan kembali masuk daftar
	// tinjauan dan tidak lagi dihitung pada angka terverifikasi.
	err = s.outcomes.SetOutcomeVerification(ctx, id, models.OutcomeSelfReported, reason, nil, nil)
	if err != nil {
		return dto.OutcomeView{}, err
	}

	s.recordAudit(ctx, id, actorID, models.SelectionActionRejected, reason)
	return s.reloadView(ctx, id)
}

/* --------------------------- tahapan & target KPI --------------------------- */

// AdminStages mengambil seluruh tahapan seleksi (termasuk yang nonaktif).
func (s *OutcomeService) AdminStages(ctx context.Context) ([]dto.SelectionStageView, error) {
	stages, err := s.outcomes.ListStages(ctx, false)
	if err != nil {
		return nil, err
	}

	out := make([]dto.SelectionStageView, 0, len(stages))
	for _, stage := range stages {
		out = append(out, stageViewFromModel(stage))
	}
	return out, nil
}

// UpdateStage mengubah label/urutan/status aktif sebuah tahapan seleksi.
//
// Tahapan bersifat data (§2.2), jadi perubahan alur seleksi tidak memerlukan
// migrasi baru — cukup lewat panel admin.
func (s *OutcomeService) UpdateStage(
	ctx context.Context,
	key string,
	patch dto.SelectionStagePatch,
) (dto.SelectionStageView, error) {
	stage, err := s.outcomes.FindStage(ctx, strings.TrimSpace(key))
	if err != nil {
		return dto.SelectionStageView{}, err
	}

	if patch.Label != nil {
		label := strings.TrimSpace(*patch.Label)
		if len(label) < 3 {
			return dto.SelectionStageView{}, apperror.Validation("Label tahapan minimal 3 karakter.", nil)
		}
		stage.Label = label
	}
	if patch.Phase != nil {
		stage.Phase = *patch.Phase
	}
	if patch.IsFinal != nil {
		stage.IsFinal = *patch.IsFinal
	}
	if patch.SortOrder != nil {
		stage.SortOrder = *patch.SortOrder
	}
	if patch.Active != nil {
		stage.Active = *patch.Active
	}

	if err := s.outcomes.UpdateStage(ctx, stage); err != nil {
		return dto.SelectionStageView{}, err
	}
	return stageViewFromModel(*stage), nil
}

// Targets mengambil daftar target KPI beserta nama kelompoknya.
func (s *OutcomeService) Targets(
	ctx context.Context,
	filter repository.KPITargetFilter,
) ([]dto.KPITargetView, error) {
	rows, err := s.outcomes.ListTargets(ctx, filter)
	if err != nil {
		return nil, err
	}

	out := make([]dto.KPITargetView, 0, len(rows))
	for _, row := range rows {
		out = append(out, kpiTargetView(row))
	}
	return out, nil
}

// kpiTargetView membentuk tampilan target KPI dari baris hasil join.
func kpiTargetView(row repository.KPITargetRow) dto.KPITargetView {
	target := row.KPITarget

	view := dto.KPITargetView{
		ID:          target.ID,
		OrgName:     row.OrgName,
		CohortName:  row.CohortName,
		PeriodYear:  target.PeriodYear,
		Metric:      string(target.Metric),
		MetricLabel: target.Metric.Label(),
		TargetValue: target.TargetValue,
		Note:        target.Note,
		CreatedAt:   formatTime(target.CreatedAt),
		UpdatedAt:   formatTime(target.UpdatedAt),
	}
	if target.OrgID != nil {
		view.OrgID = *target.OrgID
	}
	if target.CohortID != nil {
		view.CohortID = *target.CohortID
	}
	if target.StageKey != nil {
		view.StageKey = *target.StageKey
	}
	return view
}

// CreateTarget menambahkan target KPI baru (idempoten per kelompok + periode + metric).
func (s *OutcomeService) CreateTarget(ctx context.Context, in dto.KPITargetInput) (dto.KPITargetView, error) {
	metric := models.KPIMetric(in.Metric)
	if !metric.Valid() {
		return dto.KPITargetView{}, apperror.Validation("Metric target KPI tidak dikenal.", nil)
	}

	stageKey := strings.TrimSpace(in.StageKey)
	if metric == models.KPIMetricStagePassRate {
		if stageKey == "" {
			return dto.KPITargetView{}, apperror.Validation(
				"Target tingkat kelulusan per tahap wajib menyebut tahapnya.", nil)
		}
		if _, err := s.outcomes.FindStage(ctx, stageKey); err != nil {
			return dto.KPITargetView{}, err
		}
	} else {
		// Tahap hanya bermakna untuk metric per tahap (dijaga juga oleh constraint).
		stageKey = ""
	}

	target := models.KPITarget{
		PeriodYear:  in.PeriodYear,
		Metric:      metric,
		TargetValue: in.TargetValue,
		Note:        strings.TrimSpace(in.Note),
	}
	if orgID := strings.TrimSpace(in.OrgID); orgID != "" {
		target.OrgID = &orgID
	}
	if cohortID := strings.TrimSpace(in.CohortID); cohortID != "" {
		target.CohortID = &cohortID
	}
	if stageKey != "" {
		target.StageKey = &stageKey
	}

	// Kelompok + periode + metric bersifat unik: target kedua memperbarui yang ada.
	existing, err := s.outcomes.FindTargetByKey(
		ctx, strPtrOrEmpty(target.OrgID), strPtrOrEmpty(target.CohortID), target.PeriodYear, metric, stageKey)
	if err != nil {
		return dto.KPITargetView{}, err
	}
	if existing != nil {
		target.ID = existing.ID
		if err := s.outcomes.UpdateTarget(ctx, &target); err != nil {
			return dto.KPITargetView{}, err
		}
		return s.reloadTargetView(ctx, target.ID)
	}

	if err := s.outcomes.CreateTarget(ctx, &target); err != nil {
		return dto.KPITargetView{}, err
	}
	return s.reloadTargetView(ctx, target.ID)
}

// UpdateTarget mengubah nilai/catatan sebuah target KPI.
func (s *OutcomeService) UpdateTarget(ctx context.Context, id string, in dto.KPITargetInput) (dto.KPITargetView, error) {
	target, err := s.outcomes.FindTarget(ctx, id)
	if err != nil {
		return dto.KPITargetView{}, err
	}

	target.TargetValue = in.TargetValue
	target.Note = strings.TrimSpace(in.Note)
	if err := s.outcomes.UpdateTarget(ctx, target); err != nil {
		return dto.KPITargetView{}, err
	}
	return s.reloadTargetView(ctx, target.ID)
}

// DeleteTarget menghapus sebuah target KPI.
func (s *OutcomeService) DeleteTarget(ctx context.Context, id string) error {
	if _, err := s.outcomes.FindTarget(ctx, id); err != nil {
		return err
	}
	return s.outcomes.DeleteTarget(ctx, id)
}

// reloadTargetView membaca ulang satu target KPI beserta nama kelompoknya.
func (s *OutcomeService) reloadTargetView(ctx context.Context, id string) (dto.KPITargetView, error) {
	rows, err := s.outcomes.ListTargets(ctx, repository.KPITargetFilter{})
	if err != nil {
		return dto.KPITargetView{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return kpiTargetView(row), nil
		}
	}
	return dto.KPITargetView{}, apperror.NotFound("Target KPI tidak ditemukan.")
}
