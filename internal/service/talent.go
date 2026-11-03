package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
	"github.com/google/uuid"
)

// TalentService mengelola instrumen minat–bakat, skoring & norma, bobot lini
// bisnis, dan skor kecocokan (Fase 3 langkah 3–4).
//
// Aturan yang dipegang:
//   - skor dimensi selalu menyimpan `norm_version`, sehingga hasil lama tetap
//     dapat dijelaskan dengan norma saat itu (§2.2);
//   - bobot lini bisnis adalah data, bukan hardcode (§2.3);
//   - skor kecocokan ditampilkan sebagai **peringkat kecocokan**, bukan keputusan:
//     "Anda cocok bekerja di X" dilarang (§4.2).
type TalentService struct {
	talent     *repository.TalentRepository
	competency *repository.CompetencyRepository
}

// NewTalentService membentuk service talent mapping.
func NewTalentService(
	talent *repository.TalentRepository,
	competency *repository.CompetencyRepository,
) *TalentService {
	return &TalentService{talent: talent, competency: competency}
}

/* ---------------------------- helper murni -------------------------------- */

// Skala jawaban instrumen (Likert 1–5).
const (
	talentMinValue = 1
	talentMaxValue = 5
)

// talentItemScore mengubah jawaban menjadi skor butir; item terbalik dibalik
// (skor = min + max − nilai) dan dikalikan bobot butir.
func talentItemScore(value int, reverse bool, weight float64) float64 {
	if value < talentMinValue {
		value = talentMinValue
	}
	if value > talentMaxValue {
		value = talentMaxValue
	}
	if reverse {
		value = talentMinValue + talentMaxValue - value
	}
	if weight <= 0 {
		weight = 1
	}
	return float64(value) * weight
}

// talentRawScores menghitung skor mentah per dimensi dari jawaban.
func talentRawScores(items []models.TalentItem, answers map[string]int) map[string]float64 {
	out := make(map[string]float64)
	for _, item := range items {
		value, ok := answers[item.ID]
		if !ok {
			continue
		}
		out[item.Dimension] += talentItemScore(value, item.Reverse, item.Weight)
	}
	return out
}

// talentPercentile mengubah skor mentah menjadi persentil terhadap norma.
//
// Memakai sebaran normal: z = (raw − mean) / sd, persentil = Φ(z) × 100.
func talentPercentile(raw, mean, sd float64) float64 {
	if sd <= 0 {
		return 50
	}
	z := (raw - mean) / (sd * math.Sqrt2)
	probability := 0.5 * (1 + math.Erf(z))
	return talentClampPercentile(math.Round(probability*1000) / 10)
}

// talentClampPercentile menjaga persentil tetap di rentang 0–100.
func talentClampPercentile(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// talentFitScore menghitung skor kecocokan = rata-rata persentil berbobot.
//
// Bobot kosong atau tanpa dimensi yang cocok → tidak ada skor (`ok=false`), supaya
// profil lini bisnis yang belum diisi tidak menghasilkan angka menyesatkan (§4.2).
func talentFitScore(percentiles, weights map[string]float64) (float64, bool) {
	var weighted, total float64
	for dimension, weight := range weights {
		if weight <= 0 {
			continue
		}
		percentile, ok := percentiles[dimension]
		if !ok {
			continue
		}
		weighted += percentile * weight
		total += weight
	}
	if total <= 0 {
		return 0, false
	}
	return math.Round(weighted/total*10) / 10, true
}

// talentWeightsFromJSON membaca bobot dari kolom JSONB.
func talentWeightsFromJSON(raw models.JSONMap) map[string]float64 {
	out := make(map[string]float64, len(raw))
	for key, value := range raw {
		switch v := value.(type) {
		case float64:
			out[key] = v
		case int:
			out[key] = float64(v)
		case int64:
			out[key] = float64(v)
		}
	}
	return out
}

// talentWeightsToJSON mengubah bobot menjadi bentuk JSONB.
func talentWeightsToJSON(weights map[string]float64) models.JSONMap {
	out := make(models.JSONMap, len(weights))
	for key, value := range weights {
		out[key] = value
	}
	return out
}

// talentAssessmentStatusLabel mengembalikan label status pengerjaan instrumen.
func talentAssessmentStatusLabel(status string) string {
	switch status {
	case "":
		return "Belum dikerjakan"
	case models.TalentAssessmentRunning:
		return "Sedang dikerjakan"
	case models.TalentAssessmentCompleted:
		return "Selesai"
	default:
		return models.TalentAssessmentStatusLabel(status)
	}
}

// talentMaxRawByDimension menghitung skor mentah maksimum per dimensi (dipakai
// sebagai cadangan bila norma satu dimensi belum tersedia).
func talentMaxRawByDimension(items []models.TalentItem) map[string]float64 {
	out := make(map[string]float64)
	for _, item := range items {
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		out[item.Dimension] += float64(talentMaxValue) * weight
	}
	return out
}

/* ------------------------- helper murni (diekspor) ------------------------- */

// Helper di bawah ini mengekspor perhitungan murni talent mapping agar **seed
// demo** memakai rumus yang sama dengan runtime. Tanpa ini, seed harus menyalin
// rumusnya sendiri dan dua implementasi itu bisa menyimpang diam-diam.

// TalentRawScores menghitung skor mentah per dimensi dari jawaban peserta.
func TalentRawScores(items []models.TalentItem, answers map[string]int) map[string]float64 {
	return talentRawScores(items, answers)
}

// TalentMaxRawByDimension menghitung skor mentah maksimum tiap dimensi
// (dipakai sebagai cadangan saat norma sebuah dimensi belum tersedia).
func TalentMaxRawByDimension(items []models.TalentItem) map[string]float64 {
	return talentMaxRawByDimension(items)
}

// TalentPercentile mengubah skor mentah menjadi persentil terhadap norma.
func TalentPercentile(raw, mean, sd float64) float64 {
	return talentPercentile(raw, mean, sd)
}

// TalentClampPercentile menjaga persentil tetap di rentang 0–100.
func TalentClampPercentile(value float64) float64 {
	return talentClampPercentile(value)
}

// TalentFitScore menghitung skor kecocokan lini bisnis dari persentil dimensi.
func TalentFitScore(percentiles, weights map[string]float64) (float64, bool) {
	return talentFitScore(percentiles, weights)
}

/* ------------------------------- peserta ---------------------------------- */

// Instruments mengembalikan instrumen aktif beserta status pengerjaan peserta.
func (s *TalentService) Instruments(ctx context.Context, userID string) ([]dto.TalentInstrumentView, error) {
	instruments, err := s.talent.ListInstruments(ctx, true)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(instruments))
	for _, item := range instruments {
		ids = append(ids, item.ID)
	}
	items, err := s.talent.ListItemsByInstruments(ctx, ids)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(instruments))
	for _, item := range items {
		counts[item.InstrumentID]++
	}

	assessments, err := s.talent.ListAssessmentsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	latest := make(map[string]models.TalentAssessment, len(instruments))
	for _, assessment := range assessments {
		if _, ok := latest[assessment.InstrumentID]; !ok {
			latest[assessment.InstrumentID] = assessment
		}
	}

	out := make([]dto.TalentInstrumentView, 0, len(instruments))
	for _, instrument := range instruments {
		status, assessmentID := "", ""
		if assessment, ok := latest[instrument.ID]; ok {
			status = assessment.Status
			assessmentID = assessment.ID
		}
		out = append(out, dto.TalentInstrumentView{
			ID:           instrument.ID,
			Code:         instrument.Code,
			Name:         instrument.Name,
			Kind:         instrument.Kind,
			KindLabel:    models.TalentKindLabel(instrument.Kind),
			Version:      instrument.Version,
			NormVersion:  instrument.NormVersion,
			Description:  instrument.Description,
			Active:       instrument.Active,
			ItemCount:    counts[instrument.ID],
			Status:       status,
			StatusLabel:  talentAssessmentStatusLabel(status),
			AssessmentID: assessmentID,
		})
	}
	return out, nil
}

// StartAssessment memulai (atau melanjutkan) asesmen satu instrumen.
func (s *TalentService) StartAssessment(ctx context.Context, userID string, in dto.TalentStartInput) (dto.TalentAssessmentView, error) {
	instrument, err := s.talent.FindInstrumentByCode(ctx, strings.TrimSpace(in.InstrumentCode))
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}
	if instrument == nil || !instrument.Active {
		return dto.TalentAssessmentView{}, apperror.Validation("Instrumen tidak dikenal atau tidak aktif.", nil)
	}

	// Asesmen berjalan dilanjutkan agar jawaban sebelumnya tidak hilang.
	assessment, err := s.talent.FindRunningAssessment(ctx, userID, instrument.ID)
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}

	if assessment == nil {
		consentID, err := s.ensureAssessmentConsent(ctx, userID, in.ConsentGranted)
		if err != nil {
			return dto.TalentAssessmentView{}, err
		}
		assessment = &models.TalentAssessment{
			UserID:       userID,
			InstrumentID: instrument.ID,
			Status:       models.TalentAssessmentRunning,
			StartedAt:    time.Now().UTC(),
			ConsentID:    consentID,
		}
		if err := s.talent.CreateAssessment(ctx, assessment); err != nil {
			return dto.TalentAssessmentView{}, err
		}
	}

	return s.assessmentView(ctx, *instrument, *assessment)
}

// assessmentConsentRejection memutuskan apakah pemrosesan asesmen boleh
// dilanjutkan tanpa persetujuan baru (§2.9, §4.7).
//
// Murni supaya bisa diuji tanpa database. Mengembalikan pesan kosong bila boleh
// lanjut. Persetujuan yang sudah ditarik tidak dihitung aktif — peserta harus
// menyetujui ulang sebelum memulai asesmen baru, sementara riwayatnya tetap
// tersimpan.
func assessmentConsentRejection(existing *models.ConsentRecord, granted bool) string {
	if existing != nil && existing.RevokedAt == nil {
		return ""
	}
	if granted {
		return ""
	}
	return "Persetujuan pemrosesan data asesmen wajib diberikan sebelum memulai."
}

// ensureAssessmentConsent memastikan peserta punya persetujuan pemrosesan data
// asesmen (UU PDP); persetujuan yang sudah ada dipakai ulang.
func (s *TalentService) ensureAssessmentConsent(ctx context.Context, userID string, granted bool) (*string, error) {
	existing, err := s.talent.LatestConsent(ctx, userID, models.ConsentPurposeAssessment)
	if err != nil {
		return nil, err
	}
	if reason := assessmentConsentRejection(existing, granted); reason != "" {
		return nil, apperror.Validation(reason, nil)
	}
	if existing != nil && existing.RevokedAt == nil {
		return &existing.ID, nil
	}

	record := models.ConsentRecord{
		UserID:      userID,
		Purpose:     models.ConsentPurposeAssessment,
		TextVersion: "2026-11-v1",
		GrantedAt:   time.Now().UTC(),
	}
	if err := s.talent.CreateConsent(ctx, &record); err != nil {
		return nil, err
	}
	return &record.ID, nil
}

// assessmentView membentuk tampilan satu sesi asesmen beserta butirnya.
func (s *TalentService) assessmentView(
	ctx context.Context,
	instrument models.TalentInstrument,
	assessment models.TalentAssessment,
) (dto.TalentAssessmentView, error) {
	items, err := s.talent.ListItems(ctx, instrument.ID)
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}
	answers, err := s.talent.ListAnswers(ctx, assessment.ID)
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}

	views := make([]dto.TalentItemView, 0, len(items))
	for _, item := range items {
		views = append(views, dto.TalentItemView{
			ID:             item.ID,
			Text:           item.Text,
			Dimension:      item.Dimension,
			DimensionLabel: models.TalentDimensionLabel(item.Dimension),
			SortOrder:      item.SortOrder,
		})
	}

	return dto.TalentAssessmentView{
		ID:             assessment.ID,
		InstrumentID:   instrument.ID,
		InstrumentName: instrument.Name,
		Status:         assessment.Status,
		StatusLabel:    talentAssessmentStatusLabel(assessment.Status),
		ItemCount:      len(items),
		AnsweredCount:  len(answers),
		Items:          views,
	}, nil
}

// SaveAnswers menyimpan jawaban bertahap (autosave) milik peserta.
func (s *TalentService) SaveAnswers(
	ctx context.Context,
	userID, assessmentID string,
	in dto.TalentAnswersInput,
) (dto.TalentAssessmentView, error) {
	assessment, err := s.talent.FindAssessment(ctx, assessmentID)
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}
	if assessment == nil || assessment.UserID != userID {
		return dto.TalentAssessmentView{}, apperror.NotFound("Asesmen tidak ditemukan.")
	}
	if assessment.Status != models.TalentAssessmentRunning {
		return dto.TalentAssessmentView{}, apperror.Conflict("Asesmen ini sudah dikumpulkan.")
	}

	instrument, err := s.talent.FindInstrument(ctx, assessment.InstrumentID)
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}
	if instrument == nil {
		return dto.TalentAssessmentView{}, apperror.NotFound("Instrumen tidak ditemukan.")
	}

	items, err := s.talent.ListItems(ctx, instrument.ID)
	if err != nil {
		return dto.TalentAssessmentView{}, err
	}
	valid := make(map[string]struct{}, len(items))
	for _, item := range items {
		valid[item.ID] = struct{}{}
	}

	answers := make([]models.TalentAnswer, 0, len(in.Answers))
	for _, answer := range in.Answers {
		if _, ok := valid[answer.ItemID]; !ok {
			continue
		}
		answers = append(answers, models.TalentAnswer{
			AssessmentID: assessment.ID,
			ItemID:       answer.ItemID,
			Value:        answer.Value,
		})
	}
	if err := s.talent.UpsertAnswers(ctx, answers); err != nil {
		return dto.TalentAssessmentView{}, err
	}
	return s.assessmentView(ctx, *instrument, *assessment)
}

// Submit menilai asesmen, menyimpan skor dimensi & skor kecocokan, lalu
// mengembalikan profil terbaru. Idempoten: asesmen yang sudah selesai tidak
// dinilai ulang.
func (s *TalentService) Submit(ctx context.Context, userID, assessmentID string) (dto.TalentProfileView, error) {
	assessment, err := s.talent.FindAssessment(ctx, assessmentID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	if assessment == nil || assessment.UserID != userID {
		return dto.TalentProfileView{}, apperror.NotFound("Asesmen tidak ditemukan.")
	}
	if assessment.Status == models.TalentAssessmentCompleted {
		return s.Profile(ctx, userID)
	}
	if assessment.Status != models.TalentAssessmentRunning {
		return dto.TalentProfileView{}, apperror.Conflict("Asesmen ini tidak bisa dikumpulkan.")
	}

	instrument, err := s.talent.FindInstrument(ctx, assessment.InstrumentID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	if instrument == nil {
		return dto.TalentProfileView{}, apperror.NotFound("Instrumen tidak ditemukan.")
	}

	items, err := s.talent.ListItems(ctx, instrument.ID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	rows, err := s.talent.ListAnswers(ctx, assessment.ID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	if len(rows) == 0 {
		return dto.TalentProfileView{}, apperror.Validation("Belum ada jawaban yang bisa dinilai.", nil)
	}

	answers := make(map[string]int, len(rows))
	for _, row := range rows {
		answers[row.ItemID] = row.Value
	}
	raw := talentRawScores(items, answers)
	maxRaw := talentMaxRawByDimension(items)

	norms, err := s.talent.ListNorms(ctx, instrument.NormVersion)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	normByDimension := make(map[string]models.TalentNorm, len(norms))
	for _, norm := range norms {
		normByDimension[norm.Dimension] = norm
	}

	now := time.Now().UTC()
	scores := make([]models.TalentScore, 0, len(raw))
	for dimension, value := range raw {
		percentile := 0.0
		if norm, ok := normByDimension[dimension]; ok {
			percentile = talentPercentile(value, norm.Mean, norm.SD)
		} else if maximum := maxRaw[dimension]; maximum > 0 {
			// Cadangan: persentase terhadap skor maksimum dimensi.
			percentile = talentClampPercentile(math.Round(value/maximum*1000) / 10)
		}
		scores = append(scores, models.TalentScore{
			UserID:       userID,
			Dimension:    dimension,
			Raw:          math.Round(value*100) / 100,
			Percentile:   percentile,
			NormVersion:  instrument.NormVersion,
			AssessmentID: &assessment.ID,
			ComputedAt:   now,
		})
	}
	if err := s.talent.UpsertScores(ctx, scores); err != nil {
		return dto.TalentProfileView{}, err
	}
	if err := s.recomputeFit(ctx, userID); err != nil {
		return dto.TalentProfileView{}, err
	}
	if err := s.talent.CompleteAssessment(ctx, assessment.ID, now); err != nil {
		return dto.TalentProfileView{}, err
	}
	return s.Profile(ctx, userID)
}

// recomputeFit menghitung ulang skor kecocokan seluruh lini bisnis aktif dari
// persentil terbaru peserta (hasil lama ditimpa, bukan dihapus).
func (s *TalentService) recomputeFit(ctx context.Context, userID string) error {
	scores, err := s.talent.ListScores(ctx, userID)
	if err != nil {
		return err
	}
	percentiles := make(map[string]float64, len(scores))
	for _, score := range scores {
		percentiles[score.Dimension] = score.Percentile
	}

	lines, err := s.talent.ListBusinessLines(ctx, true)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	fitScores := make([]models.TalentFitScore, 0, len(lines))
	for _, line := range lines {
		score, ok := talentFitScore(percentiles, talentWeightsFromJSON(line.Weights))
		if !ok {
			continue
		}
		fitScores = append(fitScores, models.TalentFitScore{
			UserID:         userID,
			BusinessLineID: line.ID,
			Score:          score,
			ComputedAt:     now,
		})
	}
	return s.talent.UpsertFitScores(ctx, fitScores)
}

// Profile membentuk profil talenta peserta: dimensi + peringkat kecocokan lini bisnis.
func (s *TalentService) Profile(ctx context.Context, userID string) (dto.TalentProfileView, error) {
	scores, err := s.talent.ListScores(ctx, userID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	assessments, err := s.talent.ListAssessmentsByUser(ctx, userID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	fitScores, err := s.talent.ListFitScores(ctx, userID)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	lines, err := s.talent.ListBusinessLines(ctx, false)
	if err != nil {
		return dto.TalentProfileView{}, err
	}
	lineByID := make(map[string]models.BusinessLineProfile, len(lines))
	for _, line := range lines {
		lineByID[line.ID] = line
	}

	normVersion := ""
	dimensions := make([]dto.TalentDimensionView, 0, len(scores))
	for _, score := range scores {
		if normVersion == "" {
			normVersion = score.NormVersion
		}
		dimensions = append(dimensions, dto.TalentDimensionView{
			Dimension:      score.Dimension,
			DimensionLabel: models.TalentDimensionLabel(score.Dimension),
			Raw:            score.Raw,
			Percentile:     score.Percentile,
			NormVersion:    score.NormVersion,
		})
	}

	fit := make([]dto.TalentFitView, 0, len(fitScores))
	for _, score := range fitScores {
		line := lineByID[score.BusinessLineID]
		fit = append(fit, dto.TalentFitView{
			BusinessLineID: score.BusinessLineID,
			SectorKey:      line.SectorKey,
			SectorLabel:    bank.SectorLabel(line.SectorKey),
			Name:           line.Name,
			Score:          score.Score,
		})
	}

	completed := 0
	for _, assessment := range assessments {
		if assessment.Status == models.TalentAssessmentCompleted {
			completed++
		}
	}

	return dto.TalentProfileView{
		Dimensions:      dimensions,
		Fit:             fit,
		NormVersion:     normVersion,
		AssessmentCount: completed,
		Notes: []string{
			"Hasil ini bahan pertimbangan, bukan penentu keputusan.",
			"Peringkat kecocokan memakai bobot lini bisnis, bukan rekomendasi penempatan.",
			"Persentil dihitung terhadap norma berversi (normVersion).",
		},
	}, nil
}

/* --------------------------------- admin ---------------------------------- */

// AdminInstruments mengembalikan instrumen (termasuk nonaktif) beserta jumlah butirnya.
func (s *TalentService) AdminInstruments(ctx context.Context) ([]dto.TalentInstrumentView, error) {
	instruments, err := s.talent.ListInstruments(ctx, false)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(instruments))
	for _, item := range instruments {
		ids = append(ids, item.ID)
	}
	items, err := s.talent.ListItemsByInstruments(ctx, ids)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(instruments))
	for _, item := range items {
		counts[item.InstrumentID]++
	}

	out := make([]dto.TalentInstrumentView, 0, len(instruments))
	for _, instrument := range instruments {
		out = append(out, dto.TalentInstrumentView{
			ID:          instrument.ID,
			Code:        instrument.Code,
			Name:        instrument.Name,
			Kind:        instrument.Kind,
			KindLabel:   models.TalentKindLabel(instrument.Kind),
			Version:     instrument.Version,
			NormVersion: instrument.NormVersion,
			Description: instrument.Description,
			Active:      instrument.Active,
			ItemCount:   counts[instrument.ID],
			Status:      "belum",
			StatusLabel: talentAssessmentStatusLabel(""),
		})
	}
	return out, nil
}

// UpsertInstrument menambah/mengubah satu instrumen (admin).
func (s *TalentService) UpsertInstrument(ctx context.Context, in dto.TalentInstrumentInput) error {
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	version := strings.TrimSpace(in.Version)
	if version == "" {
		version = "v1"
	}
	normVersion := strings.TrimSpace(in.NormVersion)
	if normVersion == "" {
		normVersion = "norm-2026"
	}

	item := models.TalentInstrument{
		ID:          strings.TrimSpace(in.ID),
		Code:        strings.ToLower(strings.TrimSpace(in.Code)),
		Name:        strings.TrimSpace(in.Name),
		Kind:        in.Kind,
		Version:     version,
		NormVersion: normVersion,
		Description: strings.TrimSpace(in.Description),
		Active:      active,
	}
	return s.talent.UpsertInstrument(ctx, &item)
}

// UpsertItem menambah/mengubah satu butir instrumen (admin).
func (s *TalentService) UpsertItem(ctx context.Context, instrumentID string, in dto.TalentItemInput) error {
	instrument, err := s.talent.FindInstrument(ctx, instrumentID)
	if err != nil {
		return err
	}
	if instrument == nil {
		return apperror.NotFound("Instrumen tidak ditemukan.")
	}

	weight := in.Weight
	if weight <= 0 {
		weight = 1
	}
	itemID := strings.TrimSpace(in.ID)
	if itemID == "" {
		itemID = uuid.NewString()
	}
	item := models.TalentItem{
		IDBase:       models.IDBase{ID: itemID},
		InstrumentID: instrument.ID,
		Text:         strings.TrimSpace(in.Text),
		Dimension:    strings.TrimSpace(in.Dimension),
		Weight:       weight,
		Reverse:      in.Reverse,
		SortOrder:    in.SortOrder,
	}
	return s.talent.UpsertItem(ctx, &item)
}

// BusinessLines mengembalikan profil lini bisnis (opsional hanya yang aktif).
func (s *TalentService) BusinessLines(ctx context.Context, activeOnly bool) ([]dto.BusinessLineView, error) {
	lines, err := s.talent.ListBusinessLines(ctx, activeOnly)
	if err != nil {
		return nil, err
	}
	out := make([]dto.BusinessLineView, 0, len(lines))
	for _, line := range lines {
		out = append(out, dto.BusinessLineView{
			ID:          line.ID,
			SectorKey:   line.SectorKey,
			SectorLabel: bank.SectorLabel(line.SectorKey),
			Name:        line.Name,
			Description: line.Description,
			Weights:     talentWeightsFromJSON(line.Weights),
			Active:      line.Active,
		})
	}
	return out, nil
}

// UpsertBusinessLine menambah/mengubah satu profil lini bisnis (admin).
func (s *TalentService) UpsertBusinessLine(ctx context.Context, in dto.BusinessLineInput) ([]dto.BusinessLineView, error) {
	sector := strings.TrimSpace(in.SectorKey)
	if sector == "" {
		sector = bank.DefaultSector
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}

	item := models.BusinessLineProfile{
		IDBase:      models.IDBase{ID: uuid.NewString()},
		SectorKey:   sector,
		Name:        strings.TrimSpace(in.Name),
		Weights:     talentWeightsToJSON(in.Weights),
		Description: strings.TrimSpace(in.Description),
		Active:      active,
	}
	if err := s.talent.UpsertBusinessLine(ctx, &item); err != nil {
		return nil, err
	}
	return s.BusinessLines(ctx, false)
}

// UpdateBusinessLine mengubah sebagian profil lini bisnis (mis. bobot dimensi).
func (s *TalentService) UpdateBusinessLine(ctx context.Context, id string, in dto.BusinessLinePatch) ([]dto.BusinessLineView, error) {
	found, err := s.talent.FindBusinessLine(ctx, id)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, apperror.NotFound("Profil lini bisnis tidak ditemukan.")
	}

	updates := map[string]any{}
	if in.Name != nil {
		updates["name"] = strings.TrimSpace(*in.Name)
	}
	if in.SectorKey != nil {
		updates["sector_key"] = strings.TrimSpace(*in.SectorKey)
	}
	if in.Weights != nil {
		updates["weights"] = talentWeightsToJSON(in.Weights)
	}
	if in.Description != nil {
		updates["description"] = strings.TrimSpace(*in.Description)
	}
	if in.Active != nil {
		updates["active"] = *in.Active
	}
	if err := s.talent.UpdateBusinessLineFields(ctx, id, updates); err != nil {
		return nil, err
	}
	// Perubahan bobot langsung dihitung ulang agar peta talenta tidak menampilkan
	// angka lama (§2.3: hasil lama tidak dihapus, hanya diperbarui `computed_at`).
	if err := s.recomputeFitForLine(ctx, id); err != nil {
		return nil, err
	}
	return s.BusinessLines(ctx, false)
}

// recomputeFitForLine menghitung ulang skor kecocokan satu lini bisnis untuk semua
// peserta yang sudah punya skor dimensi.
func (s *TalentService) recomputeFitForLine(ctx context.Context, lineID string) error {
	line, err := s.talent.FindBusinessLine(ctx, lineID)
	if err != nil {
		return err
	}
	if line == nil || !line.Active {
		return nil
	}
	weights := talentWeightsFromJSON(line.Weights)
	if len(weights) == 0 {
		return nil
	}

	scores, err := s.talent.ListAllScores(ctx)
	if err != nil {
		return err
	}
	percentilesByUser := make(map[string]map[string]float64)
	for _, score := range scores {
		if percentilesByUser[score.UserID] == nil {
			percentilesByUser[score.UserID] = map[string]float64{}
		}
		percentilesByUser[score.UserID][score.Dimension] = score.Percentile
	}

	now := time.Now().UTC()
	fitScores := make([]models.TalentFitScore, 0, len(percentilesByUser))
	for userID, percentiles := range percentilesByUser {
		score, ok := talentFitScore(percentiles, weights)
		if !ok {
			continue
		}
		fitScores = append(fitScores, models.TalentFitScore{
			UserID:         userID,
			BusinessLineID: lineID,
			Score:          score,
			ComputedAt:     now,
		})
	}
	return s.talent.UpsertFitScores(ctx, fitScores)
}

// talentDimensionOrder menentukan urutan dimensi pada tampilan (minat lalu bakat).
var talentDimensionOrder = []string{
	models.TalentDimensionRealistic,
	models.TalentDimensionInvestigative,
	models.TalentDimensionArtistic,
	models.TalentDimensionSocial,
	models.TalentDimensionEnterprising,
	models.TalentDimensionConventional,
	models.TalentDimensionNumerik,
	models.TalentDimensionVerbal,
	models.TalentDimensionFigural,
	models.TalentDimensionLearningAgility,
}

// talentDimensionRank mengembalikan urutan tampil sebuah dimensi.
func talentDimensionRank(dimension string) int {
	for index, item := range talentDimensionOrder {
		if item == dimension {
			return index
		}
	}
	return len(talentDimensionOrder)
}

// talentDimensionViews menyusun daftar dimensi dalam urutan tampil.
func talentDimensionViews(seen map[string]struct{}) []dto.TalentDimensionView {
	out := make([]dto.TalentDimensionView, 0, len(seen))
	for dimension := range seen {
		out = append(out, dto.TalentDimensionView{
			Dimension:      dimension,
			DimensionLabel: models.TalentDimensionLabel(dimension),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := talentDimensionRank(out[i].Dimension), talentDimensionRank(out[j].Dimension)
		if left == right {
			return out[i].Dimension < out[j].Dimension
		}
		return left < right
	})
	return out
}

/* ------------------------------ matriks 9-box ----------------------------- */

// Ambang pembagian band pada matriks 9-box (§4.4).
//
// Dipakai untuk **kedua** sumbu supaya aturannya satu tempat dan bisa diuji:
// <50 rendah, 50–69 sedang, ≥70 tinggi.
const (
	nineBoxLowMax  = 50
	nineBoxHighMin = 70
)

// Band matriks 9-box.
const (
	nineBoxBandLow    = "rendah"
	nineBoxBandMedium = "sedang"
	nineBoxBandHigh   = "tinggi"
)

// nineBoxBand mengubah skor 0–100 menjadi band rendah/sedang/tinggi.
func nineBoxBand(score float64) string {
	switch {
	case score >= nineBoxHighMin:
		return nineBoxBandHigh
	case score >= nineBoxLowMax:
		return nineBoxBandMedium
	default:
		return nineBoxBandLow
	}
}

// nineBoxBandLabel mengembalikan label band siap tampil.
func nineBoxBandLabel(band string) string {
	switch band {
	case nineBoxBandHigh:
		return "Tinggi"
	case nineBoxBandMedium:
		return "Sedang"
	default:
		return "Rendah"
	}
}

// nineBoxQuadrant adalah definisi satu kotak matriks 9-box.
type nineBoxQuadrant struct {
	Key             string
	Label           string
	ShortLabel      string
	Action          string
	PerformanceBand string
	PotentialBand   string
	// Row 0 = potensi tinggi (baris atas); Column 0 = kinerja rendah (kolom kiri).
	Row    int
	Column int
}

// nineBoxQuadrants mengembalikan kesembilan kotak matriks beserta saran tindak
// lanjutnya, sudah dalam urutan baris lalu kolom.
//
// Saran ditulis sebagai *langkah berikutnya*, bukan vonis: matriks ini alat
// bantu keputusan, bukan keputusan penempatan (§4.2).
func nineBoxQuadrants() []nineBoxQuadrant {
	return []nineBoxQuadrant{
		{
			Key: "enigma", Label: "Potensi tinggi, kinerja belum terbaca",
			ShortLabel:      "Potensi belum terbaca",
			Action:          "Beri penugasan menantang dengan pendampingan; tinjau ulang kesesuaian penempatan saat ini.",
			PerformanceBand: nineBoxBandLow, PotentialBand: nineBoxBandHigh, Row: 0, Column: 0,
		},
		{
			Key: "growth", Label: "Potensi tinggi, kinerja tumbuh",
			ShortLabel:      "Potensi tumbuh",
			Action:          "Perluas tanggung jawab bertahap sambil memantau capaian tryout berikutnya.",
			PerformanceBand: nineBoxBandMedium, PotentialBand: nineBoxBandHigh, Row: 0, Column: 1,
		},
		{
			Key: "star", Label: "Bintang — kuat & siap ditempatkan",
			ShortLabel:      "Bintang",
			Action:          "Siap ditempatkan pada peran kunci; tawarkan jalur percepatan dan mentor.",
			PerformanceBand: nineBoxBandHigh, PotentialBand: nineBoxBandHigh, Row: 0, Column: 2,
		},
		{
			Key: "needs_support", Label: "Kinerja rendah, potensi sedang",
			ShortLabel:      "Perlu pendampingan",
			Action:          "Pendampingan teknis terarah dengan target belajar mingguan.",
			PerformanceBand: nineBoxBandLow, PotentialBand: nineBoxBandMedium, Row: 1, Column: 0,
		},
		{
			Key: "core", Label: "Kontributor inti",
			ShortLabel:      "Inti",
			Action:          "Pertahankan kinerja; kembangkan satu kompetensi unggulan.",
			PerformanceBand: nineBoxBandMedium, PotentialBand: nineBoxBandMedium, Row: 1, Column: 1,
		},
		{
			Key: "high_performer", Label: "Kinerja kuat, potensi terukur",
			ShortLabel:      "Kinerja kuat",
			Action:          "Beri peran ahli; uji potensi lewat asesmen formal berikutnya.",
			PerformanceBand: nineBoxBandHigh, PotentialBand: nineBoxBandMedium, Row: 1, Column: 2,
		},
		{
			Key: "needs_improvement", Label: "Perlu perbaikan menyeluruh",
			ShortLabel:      "Perlu perbaikan",
			Action:          "Susun rencana perbaikan belajar dan pembinaan intensif.",
			PerformanceBand: nineBoxBandLow, PotentialBand: nineBoxBandLow, Row: 2, Column: 0,
		},
		{
			Key: "stable", Label: "Kontributor stabil",
			ShortLabel:      "Stabil",
			Action:          "Fokus pada konsistensi; tawarkan pelatihan peningkatan keterampilan.",
			PerformanceBand: nineBoxBandMedium, PotentialBand: nineBoxBandLow, Row: 2, Column: 1,
		},
		{
			Key: "specialist", Label: "Ahli di bidangnya, potensi perlu dipetakan ulang",
			ShortLabel:      "Ahli teknis",
			Action:          "Manfaatkan sebagai mentor teknis; petakan minat lewat instrumen minat–bakat.",
			PerformanceBand: nineBoxBandHigh, PotentialBand: nineBoxBandLow, Row: 2, Column: 2,
		},
	}
}

// nineBoxQuadrantFor mencari kotak dari pasangan band kinerja & potensi.
//
// Bila pasangan tidak dikenal (tidak akan terjadi untuk band yang sah), kotak
// "core" dipakai sebagai jaring pengaman agar tidak ada peserta yang hilang.
func nineBoxQuadrantFor(performanceBand, potentialBand string) nineBoxQuadrant {
	quadrants := nineBoxQuadrants()
	for _, quadrant := range quadrants {
		if quadrant.PerformanceBand == performanceBand && quadrant.PotentialBand == potentialBand {
			return quadrant
		}
	}
	return quadrants[4]
}

// nineBoxPotential menghitung rata-rata persentil dimensi potensi seorang peserta.
//
// Mengembalikan `ok=false` bila peserta belum punya satu pun skor dimensi bakat,
// supaya ia masuk kelompok "data belum cukup" (§4.4).
func nineBoxPotential(scores []models.TalentScore) (float64, bool) {
	var total float64
	var count int
	for _, score := range scores {
		if !models.IsTalentPotentialDimension(score.Dimension) {
			continue
		}
		total += score.Percentile
		count++
	}
	if count == 0 {
		return 0, false
	}
	return math.Round(total/float64(count)*10) / 10, true
}

// NineBoxBand mengubah skor 0–100 menjadi band matriks 9-box
// (rendah/sedang/tinggi) memakai ambang yang sama dengan runtime.
func NineBoxBand(score float64) string {
	return nineBoxBand(score)
}

// NineBoxQuadrantFor mengembalikan kunci kotak matriks 9-box dari pasangan band.
func NineBoxQuadrantFor(performanceBand, potentialBand string) string {
	return nineBoxQuadrantFor(performanceBand, potentialBand).Key
}

// NineBoxPotential menghitung rata-rata persentil dimensi potensi dari daftar
// skor dimensi seorang peserta.
func NineBoxPotential(scores []models.TalentScore) (float64, bool) {
	return nineBoxPotential(scores)
}

// Mapping membentuk heatmap peserta × lini bisnis untuk admin.
func (s *TalentService) Mapping(ctx context.Context, filter repository.MatrixFilter) (dto.TalentMappingView, error) {
	lines, err := s.talent.ListBusinessLines(ctx, true)
	if err != nil {
		return dto.TalentMappingView{}, err
	}
	lineViews, err := s.BusinessLines(ctx, true)
	if err != nil {
		return dto.TalentMappingView{}, err
	}
	users, err := s.competency.ListPesertaForMatrix(ctx, filter)
	if err != nil {
		return dto.TalentMappingView{}, err
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	scores, err := s.talent.ListScoresByUsers(ctx, userIDs)
	if err != nil {
		return dto.TalentMappingView{}, err
	}
	fitScores, err := s.talent.ListFitScoresByUsers(ctx, userIDs)
	if err != nil {
		return dto.TalentMappingView{}, err
	}

	lineNameByID := make(map[string]string, len(lines))
	for _, line := range lines {
		lineNameByID[line.ID] = line.Name
	}

	percentilesByUser := make(map[string]map[string]float64, len(users))
	seen := make(map[string]struct{})
	normVersion := ""
	for _, score := range scores {
		if percentilesByUser[score.UserID] == nil {
			percentilesByUser[score.UserID] = map[string]float64{}
		}
		percentilesByUser[score.UserID][score.Dimension] = score.Percentile
		seen[score.Dimension] = struct{}{}
		if normVersion == "" {
			normVersion = score.NormVersion
		}
	}
	fitByUser := make(map[string]map[string]float64, len(users))
	for _, score := range fitScores {
		if fitByUser[score.UserID] == nil {
			fitByUser[score.UserID] = map[string]float64{}
		}
		fitByUser[score.UserID][score.BusinessLineID] = score.Score
	}

	rows := make([]dto.TalentMappingRowView, 0, len(users))
	for _, user := range users {
		percentiles := percentilesByUser[user.ID]
		if percentiles == nil {
			// Peserta tanpa data tetap dikirim sebagai objek kosong (bukan null)
			// agar klien tidak perlu menangani dua bentuk.
			percentiles = map[string]float64{}
		}
		fit := fitByUser[user.ID]
		if fit == nil {
			fit = map[string]float64{}
		}

		bestID, bestName, bestScore := "", "", 0.0
		for lineID, score := range fit {
			if score > bestScore {
				bestID, bestName, bestScore = lineID, lineNameByID[lineID], score
			}
		}

		rows = append(rows, dto.TalentMappingRowView{
			UserID:       user.ID,
			Name:         user.Name,
			Initials:     competencyInitials(user.Name),
			Program:      user.Program,
			Percentiles:  percentiles,
			Fit:          fit,
			BestLineID:   bestID,
			BestLineName: bestName,
			BestScore:    bestScore,
			HasData:      len(percentiles) > 0,
		})
	}

	return dto.TalentMappingView{
		Dimensions:    talentDimensionViews(seen),
		BusinessLines: lineViews,
		Rows:          rows,
		NormVersion:   normVersion,
		Notes: []string{
			"Skor adalah persentil terhadap norma instrumen, bukan prediksi kinerja.",
			"Peringkat kecocokan memakai bobot lini bisnis dan bukan keputusan penempatan.",
			"Peserta tanpa data ditandai \"data belum cukup\", bukan dinilai rendah.",
		},
	}, nil
}

// MappingDetail membentuk profil talenta satu peserta untuk admin.
func (s *TalentService) MappingDetail(ctx context.Context, userID string) (dto.TalentMappingDetailView, error) {
	profile, err := s.Profile(ctx, userID)
	if err != nil {
		return dto.TalentMappingDetailView{}, err
	}
	assessments, err := s.talent.ListAssessmentsByUser(ctx, userID)
	if err != nil {
		return dto.TalentMappingDetailView{}, err
	}
	instruments, err := s.talent.ListInstruments(ctx, false)
	if err != nil {
		return dto.TalentMappingDetailView{}, err
	}
	nameByInstrument := make(map[string]string, len(instruments))
	for _, instrument := range instruments {
		nameByInstrument[instrument.ID] = instrument.Name
	}

	users, err := s.competency.ListPesertaForMatrix(ctx, repository.MatrixFilter{
		UserIDs:    []string{userID},
		UserIDsSet: true,
		Limit:      1,
	})
	if err != nil {
		return dto.TalentMappingDetailView{}, err
	}
	if len(users) == 0 {
		return dto.TalentMappingDetailView{}, apperror.NotFound("Peserta tidak ditemukan.")
	}
	user := users[0]

	summaries := make([]dto.TalentAssessmentSummaryView, 0, len(assessments))
	for _, assessment := range assessments {
		view := dto.TalentAssessmentSummaryView{
			ID:             assessment.ID,
			InstrumentName: nameByInstrument[assessment.InstrumentID],
			Status:         assessment.Status,
			StatusLabel:    models.TalentAssessmentStatusLabel(assessment.Status),
			StartedAt:      formatTime(assessment.StartedAt),
		}
		if assessment.FinishedAt != nil {
			view.FinishedAt = formatTime(*assessment.FinishedAt)
		}
		summaries = append(summaries, view)
	}

	return dto.TalentMappingDetailView{
		UserID:      user.ID,
		Name:        user.Name,
		Initials:    competencyInitials(user.Name),
		Program:     user.Program,
		Dimensions:  profile.Dimensions,
		Fit:         profile.Fit,
		Assessments: summaries,
		NormVersion: profile.NormVersion,
		Notes:       profile.Notes,
	}, nil
}

/* ---------------------------- matriks 9-box admin ------------------------- */

// NineBox membentuk sebaran peserta pada matriks 9-box (performance × potential).
//
// Sumbu kinerja = rata-rata skor tryout (hasil yang dicatat sistem); sumbu
// potensi = rata-rata persentil dimensi bakat + learning agility. Peserta yang
// belum punya salah satu sumbu masuk kelompok "data belum cukup" — bukan
// kuadran rendah — supaya tidak ada kesimpulan dari ketiadaan data (§4.4).
func (s *TalentService) NineBox(ctx context.Context, filter repository.MatrixFilter) (dto.NineBoxView, error) {
	users, err := s.competency.ListPesertaForMatrix(ctx, filter)
	if err != nil {
		return dto.NineBoxView{}, err
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	scores, err := s.talent.ListScoresByUsers(ctx, userIDs)
	if err != nil {
		return dto.NineBoxView{}, err
	}
	performance, err := s.competency.UserPerformanceScores(ctx, userIDs)
	if err != nil {
		return dto.NineBoxView{}, err
	}

	scoresByUser := make(map[string][]models.TalentScore, len(users))
	for _, score := range scores {
		scoresByUser[score.UserID] = append(scoresByUser[score.UserID], score)
	}
	performanceByUser := make(map[string]repository.UserPerformanceRow, len(performance))
	for _, row := range performance {
		performanceByUser[row.UserID] = row
	}

	quadrants := nineBoxQuadrants()
	membersByQuadrant := make(map[string][]dto.NineBoxMemberView, len(quadrants))
	insufficient := make([]dto.NineBoxMemberView, 0)
	withData := 0

	for _, user := range users {
		potential, hasPotential := nineBoxPotential(scoresByUser[user.ID])
		row, hasPerformance := performanceByUser[user.ID]
		if !hasPotential || !hasPerformance || row.Attempts == 0 {
			insufficient = append(insufficient, dto.NineBoxMemberView{
				UserID:   user.ID,
				Name:     user.Name,
				Initials: competencyInitials(user.Name),
				Program:  user.Program,
			})
			continue
		}

		member := dto.NineBoxMemberView{
			UserID:      user.ID,
			Name:        user.Name,
			Initials:    competencyInitials(user.Name),
			Program:     user.Program,
			Performance: math.Round(row.AverageScore*10) / 10,
			Potential:   potential,
			Attempts:    row.Attempts,
		}
		quadrant := nineBoxQuadrantFor(nineBoxBand(member.Performance), nineBoxBand(member.Potential))
		membersByQuadrant[quadrant.Key] = append(membersByQuadrant[quadrant.Key], member)
		withData++
	}

	cells := make([]dto.NineBoxCellView, 0, len(quadrants))
	for _, quadrant := range quadrants {
		// Kotak kosong tetap dikirim sebagai daftar kosong (bukan null) supaya
		// klien tidak perlu menangani dua bentuk.
		members := membersByQuadrant[quadrant.Key]
		if members == nil {
			members = []dto.NineBoxMemberView{}
		}
		sort.Slice(members, func(i, j int) bool {
			if members[i].Performance == members[j].Performance {
				return members[i].Name < members[j].Name
			}
			return members[i].Performance > members[j].Performance
		})
		cells = append(cells, dto.NineBoxCellView{
			Key:                  quadrant.Key,
			Label:                quadrant.Label,
			ShortLabel:           quadrant.ShortLabel,
			Action:               quadrant.Action,
			PerformanceBand:      quadrant.PerformanceBand,
			PerformanceBandLabel: nineBoxBandLabel(quadrant.PerformanceBand),
			PotentialBand:        quadrant.PotentialBand,
			PotentialBandLabel:   nineBoxBandLabel(quadrant.PotentialBand),
			Row:                  quadrant.Row,
			Column:               quadrant.Column,
			Count:                len(members),
			Users:                members,
		})
	}
	sort.Slice(insufficient, func(i, j int) bool { return insufficient[i].Name < insufficient[j].Name })

	return dto.NineBoxView{
		Cells:        cells,
		Insufficient: insufficient,
		Summary: dto.NineBoxSummaryView{
			TotalPeserta: len(users),
			WithData:     withData,
			Insufficient: len(insufficient),
		},
		PerformanceMetric: "Rata-rata skor tryout",
		PotentialMetric:   "Rata-rata persentil bakat & learning agility",
		Thresholds: dto.NineBoxThresholdView{
			LowMax:  nineBoxLowMax,
			HighMin: nineBoxHighMin,
		},
		Notes: []string{
			"Sumbu kinerja memakai rata-rata skor tryout, bukan penilaian kinerja atasan.",
			"Sumbu potensi memakai persentil instrumen bakat; peserta tanpa instrumen belum dapat dipetakan.",
			"Matriks ini alat bantu keputusan, bukan keputusan penempatan.",
			"Peserta tanpa data dikelompokkan sebagai \"data belum cukup\", bukan dinilai rendah.",
		},
	}, nil
}
