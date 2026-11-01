package service

import (
	"context"
	"math"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// KPIService menghitung KPI kelulusan BUMN (Fase 2 langkah 4).
//
// Seluruh perhitungan ada di sini (bukan di klien) sehingga dasbor admin, ekspor
// CSV, laporan PDF, dan ringkasan publik memakai angka yang identik (§2.5).
//
// Dua aturan yang dipegang:
//   - **tidak ada angka gabungan**: setiap KPI dikirim sebagai pasangan
//     `verified` (laporan terverifikasi) dan `reported` (semua laporan) (§2.3);
//   - setiap angka menyertakan `n` (dasar perhitungan) dan pembagi nol menjadi `0`,
//     bukan `NaN`/`Inf` — pola yang sama dengan `percentageFloat` di corporate.go.
type KPIService struct {
	outcomes *repository.OutcomeRepository
	cfg      *config.Config
}

// NewKPIService membentuk service KPI.
func NewKPIService(outcomes *repository.OutcomeRepository, cfg *config.Config) *KPIService {
	return &KPIService{outcomes: outcomes, cfg: cfg}
}

/* ---------------------------- helper murni -------------------------------- */

// kpiPercentage menghitung persentase satu desimal; pembagi nol menghasilkan 0.
func kpiPercentage(passed, counted int64) float64 {
	if counted <= 0 {
		return 0
	}
	return math.Round(float64(passed)/float64(counted)*1000) / 10
}

// kpiRate membungkus pembilang/penyebut menjadi satu angka KPI.
func kpiRate(passed, counted int64) dto.KPIRateView {
	return dto.KPIRateView{
		Value:  kpiPercentage(passed, counted),
		N:      int(counted),
		Passed: int(passed),
	}
}

// kpiFilterWithSource menyalin filter dengan tingkat kualitas data tertentu.
func kpiFilterWithSource(filter repository.KPIFilter, source string) repository.KPIFilter {
	scoped := filter
	scoped.Source = source
	return scoped
}

// kpiFinalStageKey mengembalikan kunci tahap penentu kelulusan akhir.
func kpiFinalStageKey(stages []models.SelectionStage) string {
	for _, stage := range stages {
		if stage.IsFinal {
			return stage.Key
		}
	}
	return ""
}

// kpiIndexStageRows mengubah baris capaian tahap menjadi peta per kunci tahap.
func kpiIndexStageRows(rows []repository.KPIStagePassRow) map[string]repository.KPIStagePassRow {
	out := make(map[string]repository.KPIStagePassRow, len(rows))
	for _, row := range rows {
		out[row.StageKey] = row
	}
	return out
}

// kpiNotes adalah catatan pembacaan yang selalu ikut pada dasbor & ekspor.
func kpiNotes() []string {
	return []string{
		"Angka \"terverifikasi\" hanya memakai laporan yang buktinya sudah diperiksa admin; \"dilaporkan\" memuat seluruh laporan.",
		"Peserta yang belum melaporkan hasil seleksi tidak dihitung sebagai gagal — mereka masuk kelompok \"belum ada laporan\".",
		"Perbandingan latihan vs kelulusan menyatakan korelasi, bukan sebab-akibat.",
		"Angka ini bukan prediksi kelulusan seleksi BUMN.",
	}
}

// kpiGroupLabel menentukan label kelompok yang ditampilkan.
//
// Klaster memakai label taksonomi Fase 0 (kunci `company_sector`), sedangkan kelas
// & program memakai label dari database (nama kelas / nama program).
func kpiGroupLabel(dimension string, row repository.KPIBreakdownRow) string {
	if dimension == "sector" {
		return bank.SectorLabel(row.GroupKey)
	}
	if row.Label == "" {
		return "Tanpa " + dimension
	}
	return row.Label
}

/* -------------------------------- dasbor ---------------------------------- */

// Dashboard menghitung seluruh angka dasbor KPI.
func (s *KPIService) Dashboard(ctx context.Context, filter repository.KPIFilter) (dto.KPIDashboardView, error) {
	if filter.Year == 0 {
		filter.Year = time.Now().UTC().Year()
	}
	now := time.Now().UTC()

	participants, err := s.outcomes.KPIParticipants(ctx, filter)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	stages, err := s.outcomes.ListStages(ctx, true)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}

	stageRates, finalRates, err := s.stageRates(ctx, filter, stages)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	finalStageKey := kpiFinalStageKey(stages)

	sectorBreakdown, err := s.breakdown(ctx, filter, "sector", finalStageKey)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	cohortBreakdown, err := s.breakdown(ctx, filter, "cohort", finalStageKey)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	programBreakdown, err := s.breakdown(ctx, filter, "program", finalStageKey)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}

	practice, err := s.practice(ctx, filter)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	curriculum, err := s.curriculum(ctx, filter)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	timeToPass, err := s.timeToPass(ctx, filter)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	funnel, err := s.funnel(ctx, filter, stageRates)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}
	targets, err := s.targetProgress(ctx, filter)
	if err != nil {
		return dto.KPIDashboardView{}, err
	}

	reported := int(participants.Reported)
	notReported := int(participants.TotalParticipants) - reported
	if notReported < 0 {
		notReported = 0
	}

	return dto.KPIDashboardView{
		Period: dto.KPIDashboardPeriod{Year: filter.Year, GeneratedAt: formatTime(now)},
		Summary: dto.KPIDashboardSummary{
			TotalParticipants: int(participants.TotalParticipants),
			Reported:          reported,
			Verified:          int(participants.Verified),
			// Peserta yang belum melapor dipisahkan, bukan dihitung sebagai gagal (§4.3).
			NotReported:    notReported,
			FinalPassRate:  finalRates,
			CurriculumRate: curriculum,
			TimeToPassDays: timeToPass,
		},
		StagePassRate:    stageRates,
		SectorBreakdown:  sectorBreakdown,
		CohortBreakdown:  cohortBreakdown,
		ProgramBreakdown: programBreakdown,
		Practice:         practice,
		Funnel:           funnel,
		Targets:          targets,
		Notes:            kpiNotes(),
	}, nil
}

// stageRates menghitung capaian tiap tahap untuk kedua tingkat kualitas data.
//
// Daftar tahapan selalu lengkap (termasuk tahap tanpa laporan) supaya bentuk
// dasbor stabil dan tahap penentu kelulusan selalu muncul — walau angkanya 0.
func (s *KPIService) stageRates(
	ctx context.Context,
	filter repository.KPIFilter,
	stages []models.SelectionStage,
) ([]dto.KPIStageRateView, dto.KPIRatePairView, error) {
	verified, err := s.outcomes.KPIStagePass(ctx, kpiFilterWithSource(filter, "verified"))
	if err != nil {
		return nil, dto.KPIRatePairView{}, err
	}
	reported, err := s.outcomes.KPIStagePass(ctx, filter)
	if err != nil {
		return nil, dto.KPIRatePairView{}, err
	}
	rates, final := kpiMergeStageRates(stages, verified, reported)
	return rates, final, nil
}

// kpiMergeStageRates menggabungkan capaian tahap dari dua tingkat kualitas data.
//
// Fungsi murni: urutan tahapan mengikuti daftar referensi (bukan urutan baris
// query), dan tahap tanpa laporan tetap muncul dengan angka 0.
func kpiMergeStageRates(
	stages []models.SelectionStage,
	verified []repository.KPIStagePassRow,
	reported []repository.KPIStagePassRow,
) ([]dto.KPIStageRateView, dto.KPIRatePairView) {
	verifiedByKey := kpiIndexStageRows(verified)
	reportedByKey := kpiIndexStageRows(reported)

	rates := make([]dto.KPIStageRateView, 0, len(stages))
	var final dto.KPIRatePairView
	for _, stage := range stages {
		scoped := verifiedByKey[stage.Key]
		all := reportedByKey[stage.Key]
		pair := dto.KPIRatePairView{
			Verified: kpiRate(scoped.Passed, scoped.Counted),
			Reported: kpiRate(all.Passed, all.Counted),
		}
		rates = append(rates, dto.KPIStageRateView{
			StageKey:   stage.Key,
			StageLabel: stage.Label,
			IsFinal:    stage.IsFinal,
			Rates:      pair,
		})
		if stage.IsFinal {
			final = pair
		}
	}
	return rates, final
}

// breakdown menghitung capaian satu tahap per kelompok (klaster/kelas/program).
func (s *KPIService) breakdown(
	ctx context.Context,
	filter repository.KPIFilter,
	dimension string,
	stageKey string,
) ([]dto.KPIBreakdownView, error) {
	verified, err := s.outcomes.KPIBreakdown(ctx, kpiFilterWithSource(filter, "verified"), dimension, stageKey)
	if err != nil {
		return nil, err
	}
	reported, err := s.outcomes.KPIBreakdown(ctx, filter, dimension, stageKey)
	if err != nil {
		return nil, err
	}

	verifiedByKey := make(map[string]repository.KPIBreakdownRow, len(verified))
	for _, row := range verified {
		verifiedByKey[row.GroupKey] = row
	}

	out := make([]dto.KPIBreakdownView, 0, len(reported))
	for _, row := range reported {
		// Kelompok tanpa peserta yang mengikuti tahap ini (n = 0) dilewati: baris
		// seperti itu hanya menambah kebisingan pada tabel & ekspor.
		if row.Counted == 0 {
			continue
		}
		scoped := verifiedByKey[row.GroupKey]
		out = append(out, dto.KPIBreakdownView{
			Key:   row.GroupKey,
			Label: kpiGroupLabel(dimension, row),
			Rates: dto.KPIRatePairView{
				Verified: kpiRate(scoped.Passed, scoped.Counted),
				Reported: kpiRate(row.Passed, row.Counted),
			},
		})
	}
	return out, nil
}

// practice membandingkan kebiasaan latihan peserta lolos vs tidak lolos.
func (s *KPIService) practice(ctx context.Context, filter repository.KPIFilter) (dto.KPIPracticePairView, error) {
	verified, err := s.outcomes.KPIPractice(ctx, kpiFilterWithSource(filter, "verified"))
	if err != nil {
		return dto.KPIPracticePairView{}, err
	}
	reported, err := s.outcomes.KPIPractice(ctx, filter)
	if err != nil {
		return dto.KPIPracticePairView{}, err
	}

	return dto.KPIPracticePairView{
		Verified: kpiPracticeView(verified),
		Reported: kpiPracticeView(reported),
	}, nil
}

// kpiPracticeView merapikan baris efektivitas latihan menjadi satu desimal.
func kpiPracticeView(row *repository.KPIPracticeRow) dto.KPIPracticeView {
	if row == nil {
		return dto.KPIPracticeView{}
	}
	return dto.KPIPracticeView{
		PassedAverageScore:    math.Round(row.PassedAverageScore*10) / 10,
		FailedAverageScore:    math.Round(row.FailedAverageScore*10) / 10,
		PassedAverageAttempts: math.Round(row.PassedAverageAttempts*10) / 10,
		FailedAverageAttempts: math.Round(row.FailedAverageAttempts*10) / 10,
	}
}

// curriculum menghitung kelengkapan kurikulum peserta yang lolos tahap akhir.
func (s *KPIService) curriculum(ctx context.Context, filter repository.KPIFilter) (dto.KPIRatePairView, error) {
	verified, err := s.outcomes.KPICurriculum(ctx, kpiFilterWithSource(filter, "verified"))
	if err != nil {
		return dto.KPIRatePairView{}, err
	}
	reported, err := s.outcomes.KPICurriculum(ctx, filter)
	if err != nil {
		return dto.KPIRatePairView{}, err
	}

	return dto.KPIRatePairView{
		Verified: kpiRate(verified.PassedCompleted, verified.PassedTotal),
		Reported: kpiRate(reported.PassedCompleted, reported.PassedTotal),
	}, nil
}

// timeToPass menghitung median hari dari percobaan pertama sampai pengumuman lolos.
func (s *KPIService) timeToPass(ctx context.Context, filter repository.KPIFilter) (dto.KPITimeToPassView, error) {
	verified, err := s.outcomes.KPITimeToPass(ctx, kpiFilterWithSource(filter, "verified"))
	if err != nil {
		return dto.KPITimeToPassView{}, err
	}
	reported, err := s.outcomes.KPITimeToPass(ctx, filter)
	if err != nil {
		return dto.KPITimeToPassView{}, err
	}

	return dto.KPITimeToPassView{
		Verified: math.Round(verified.MedianDays*10) / 10,
		Reported: math.Round(reported.MedianDays*10) / 10,
	}, nil
}

// funnel menyusun funnel: terdaftar → berlatih → kurikulum selesai → melapor →
// tahap seleksi.
//
// Tahap seleksi dihitung **kumulatif dari tahap akhir**: peserta yang lolos tahap
// akhir dianggap sudah melewati tahap sebelumnya, sehingga angka funnel tidak
// pernah naik antar tahap (aturan uji §7.1).
func (s *KPIService) funnel(
	ctx context.Context,
	filter repository.KPIFilter,
	stageRates []dto.KPIStageRateView,
) ([]dto.KPIFunnelStepView, error) {
	verified, err := s.outcomes.KPIFunnel(ctx, kpiFilterWithSource(filter, "verified"))
	if err != nil {
		return nil, err
	}
	reported, err := s.outcomes.KPIFunnel(ctx, filter)
	if err != nil {
		return nil, err
	}

	return kpiFunnelSteps(*verified, *reported, stageRates), nil
}

// kpiFunnelSteps menyusun langkah funnel dari angka dasar & capaian tahap.
//
// Fungsi murni: tahap seleksi dihitung **kumulatif dari tahap akhir** sehingga
// angka funnel tidak pernah naik antar tahap — peserta yang lolos tahap akhir
// dianggap sudah melewati tahap sebelumnya.
func kpiFunnelSteps(
	verified repository.KPIFunnelRow,
	reported repository.KPIFunnelRow,
	stageRates []dto.KPIStageRateView,
) []dto.KPIFunnelStepView {
	steps := []dto.KPIFunnelStepView{
		{Stage: "registered", Label: "Peserta terdaftar",
			Verified: int(verified.Registered), Reported: int(reported.Registered)},
		{Stage: "practiced", Label: "Aktif berlatih",
			Verified: int(verified.Practiced), Reported: int(reported.Practiced)},
		{Stage: "curriculum_completed", Label: "Kurikulum selesai",
			Verified: int(verified.CurriculumCompleted), Reported: int(reported.CurriculumCompleted)},
		{Stage: "reported", Label: "Melaporkan hasil seleksi",
			Verified: int(verified.Reported), Reported: int(reported.Reported)},
	}

	stageSteps := make([]dto.KPIFunnelStepView, len(stageRates))
	cumVerified, cumReported := 0, 0
	for i := len(stageRates) - 1; i >= 0; i-- {
		if passed := stageRates[i].Rates.Verified.Passed; passed > cumVerified {
			cumVerified = passed
		}
		if passed := stageRates[i].Rates.Reported.Passed; passed > cumReported {
			cumReported = passed
		}
		stageSteps[i] = dto.KPIFunnelStepView{
			Stage:    stageRates[i].StageKey,
			Label:    stageRates[i].StageLabel,
			Verified: cumVerified,
			Reported: cumReported,
		}
	}

	return append(steps, stageSteps...)
}

// targetProgress membandingkan realisasi tiap target KPI dengan targetnya.
//
// Realisasi dihitung ulang per **scope target** (instansi/kelas), bukan memakai
// angka dasbor yang sedang difilter, supaya target kelas A tidak dibandingkan
// dengan realisasi kelas B.
func (s *KPIService) targetProgress(
	ctx context.Context,
	filter repository.KPIFilter,
) ([]dto.KPITargetProgressView, error) {
	rows, err := s.outcomes.ListTargets(ctx, repository.KPITargetFilter{Year: filter.Year})
	if err != nil {
		return nil, err
	}

	out := make([]dto.KPITargetProgressView, 0, len(rows))
	for _, row := range rows {
		target := row.KPITarget

		scoped := filter
		scoped.OrgID = strPtrOrEmpty(target.OrgID)
		scoped.CohortID = strPtrOrEmpty(target.CohortID)

		verified, reported, err := s.targetActual(
			ctx, scoped, target.Metric, strPtrOrEmpty(target.StageKey))
		if err != nil {
			return nil, err
		}

		out = append(out, dto.KPITargetProgressView{
			Metric:         string(target.Metric),
			MetricLabel:    target.Metric.Label(),
			StageKey:       strPtrOrEmpty(target.StageKey),
			OrgName:        row.OrgName,
			CohortName:     row.CohortName,
			Target:         target.TargetValue,
			ActualVerified: verified,
			ActualReported: reported,
			// Realisasi dianggap tercapai bila angka **terverifikasi** sudah melewati
			// target: laporan yang belum diperiksa tidak boleh menutup kekurangan.
			Achieved: verified >= target.TargetValue,
		})
	}
	return out, nil
}

// targetActual menghitung realisasi satu metric KPI pada scope tertentu.
func (s *KPIService) targetActual(
	ctx context.Context,
	filter repository.KPIFilter,
	metric models.KPIMetric,
	stageKey string,
) (float64, float64, error) {
	verifiedFilter := kpiFilterWithSource(filter, "verified")

	switch metric {
	case models.KPIMetricParticipants:
		verified, err := s.outcomes.KPIParticipants(ctx, verifiedFilter)
		if err != nil {
			return 0, 0, err
		}
		reported, err := s.outcomes.KPIParticipants(ctx, filter)
		if err != nil {
			return 0, 0, err
		}
		return float64(verified.Reported), float64(reported.Reported), nil

	case models.KPIMetricCurriculumCompletion:
		verified, err := s.outcomes.KPICurriculum(ctx, verifiedFilter)
		if err != nil {
			return 0, 0, err
		}
		reported, err := s.outcomes.KPICurriculum(ctx, filter)
		if err != nil {
			return 0, 0, err
		}
		return kpiPercentage(verified.PassedCompleted, verified.PassedTotal),
			kpiPercentage(reported.PassedCompleted, reported.PassedTotal), nil

	default:
		// `final_pass_rate` & `stage_pass_rate` memakai capaian satu tahap.
		key := stageKey
		if key == "" {
			stages, err := s.outcomes.ListStages(ctx, true)
			if err != nil {
				return 0, 0, err
			}
			key = kpiFinalStageKey(stages)
		}

		verified, err := s.outcomes.KPIStagePass(ctx, verifiedFilter)
		if err != nil {
			return 0, 0, err
		}
		reported, err := s.outcomes.KPIStagePass(ctx, filter)
		if err != nil {
			return 0, 0, err
		}
		verifiedRow := kpiIndexStageRows(verified)[key]
		reportedRow := kpiIndexStageRows(reported)[key]

		return kpiPercentage(verifiedRow.Passed, verifiedRow.Counted),
			kpiPercentage(reportedRow.Passed, reportedRow.Counted), nil
	}
}
