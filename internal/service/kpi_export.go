package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/ebook"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Nama metric pada ekspor memakai nilai yang sama dengan `models.KPIMetric`,
// sehingga kolom `metric` pada CSV bisa dipakai langsung oleh klien.
const (
	kpiMetricFinalPass  = models.KPIMetricFinalPassRate
	kpiMetricStagePass  = models.KPIMetricStagePassRate
	kpiMetricCurriculum = models.KPIMetricCurriculumCompletion
)

// Ekspor & ringkasan publik KPI (Fase 2 langkah 4).
//
// CSV untuk diolah klien (Excel/BI), PDF untuk dikirim ke instansi. Keduanya
// memakai angka dari `KPIService.Dashboard`, sehingga isinya identik dengan yang
// tampil di dasbor (§2.5/§4.6). Ekspor **tidak** memuat identitas peserta: baris
// berisi kelompok & angka, bukan nama.

// kpiCSVHeader adalah kolom ekspor agregat.
var kpiCSVHeader = []string{
	"period", "scope", "group_key", "group_label", "stage_key", "metric", "level", "passed", "counted", "value", "n",
}

// ExportCSV membentuk berkas CSV agregat KPI.
func (s *KPIService) ExportCSV(ctx context.Context, filter repository.KPIFilter) (string, []byte, error) {
	report, err := s.Dashboard(ctx, filter)
	if err != nil {
		return "", nil, err
	}

	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	if err := writer.Write(kpiCSVHeader); err != nil {
		return "", nil, err
	}

	period := fmt.Sprintf("%d", report.Period.Year)
	writeRow := func(scope, groupKey, groupLabel, stageKey, metric string, verified, reported dto.KPIRateView) error {
		rows := [][3]any{
			{"verified", verified},
			{"reported", reported},
		}
		for _, entry := range rows {
			rate := entry[1].(dto.KPIRateView)
			record := []string{
				period, scope, groupKey, groupLabel, stageKey, metric, entry[0].(string),
				fmt.Sprintf("%d", rate.Passed), fmt.Sprintf("%d", rate.N),
				fmt.Sprintf("%.1f", rate.Value), fmt.Sprintf("%d", rate.N),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		return nil
	}

	// Ringkasan lintas program.
	if err := writeRow("lintas_program", "", "Semua peserta", "",
		string(kpiMetricFinalPass), report.Summary.FinalPassRate.Verified, report.Summary.FinalPassRate.Reported); err != nil {
		return "", nil, err
	}
	if err := writeRow("lintas_program", "", "Semua peserta", "",
		string(kpiMetricCurriculum), report.Summary.CurriculumRate.Verified, report.Summary.CurriculumRate.Reported); err != nil {
		return "", nil, err
	}

	for _, stage := range report.StagePassRate {
		if err := writeRow("tahap_seleksi", stage.StageKey, stage.StageLabel, stage.StageKey,
			string(kpiMetricStagePass), stage.Rates.Verified, stage.Rates.Reported); err != nil {
			return "", nil, err
		}
	}
	for _, group := range report.SectorBreakdown {
		if err := writeRow("klaster", group.Key, group.Label, "",
			string(kpiMetricFinalPass), group.Rates.Verified, group.Rates.Reported); err != nil {
			return "", nil, err
		}
	}
	for _, group := range report.CohortBreakdown {
		if err := writeRow("kelas_khusus", group.Key, group.Label, "",
			string(kpiMetricFinalPass), group.Rates.Verified, group.Rates.Reported); err != nil {
			return "", nil, err
		}
	}
	for _, group := range report.ProgramBreakdown {
		if err := writeRow("program", group.Key, group.Label, "",
			string(kpiMetricFinalPass), group.Rates.Verified, group.Rates.Reported); err != nil {
			return "", nil, err
		}
	}
	for _, target := range report.Targets {
		record := []string{
			period, "target", target.StageKey, target.OrgName + target.CohortName, target.StageKey,
			target.Metric, "target",
			"", "", fmt.Sprintf("%.2f", target.Target), "",
		}
		if err := writer.Write(record); err != nil {
			return "", nil, err
		}
		record[6] = "actual_verified"
		record[9] = fmt.Sprintf("%.1f", target.ActualVerified)
		if err := writer.Write(record); err != nil {
			return "", nil, err
		}
		record[6] = "actual_reported"
		record[9] = fmt.Sprintf("%.1f", target.ActualReported)
		if err := writer.Write(record); err != nil {
			return "", nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", nil, err
	}

	filename := fmt.Sprintf("kpi-kelulusan-bumn-%d.csv", report.Period.Year)
	return filename, []byte(buf.String()), nil
}

// ExportPDF membentuk laporan KPI siap kirim (PDF) memakai generator yang sudah
// dipakai sertifikat & laporan kelas.
func (s *KPIService) ExportPDF(ctx context.Context, filter repository.KPIFilter) (string, []byte, error) {
	report, err := s.Dashboard(ctx, filter)
	if err != nil {
		return "", nil, err
	}

	summary := report.Summary
	blocks := []ebook.Block{
		{Heading: true, Text: "Ringkasan"},
		{Text: fmt.Sprintf("Peserta: %d · melaporkan hasil seleksi: %d · terverifikasi: %d · belum melapor: %d",
			summary.TotalParticipants, summary.Reported, summary.Verified, summary.NotReported)},
		{Text: fmt.Sprintf("Tingkat kelulusan akhir — terverifikasi %.1f%% (dari %d peserta) · dilaporkan %.1f%% (dari %d peserta)",
			summary.FinalPassRate.Verified.Value, summary.FinalPassRate.Verified.N,
			summary.FinalPassRate.Reported.Value, summary.FinalPassRate.Reported.N)},
		{Text: fmt.Sprintf("Kelengkapan kurikulum peserta lolos — terverifikasi %.1f%% (n %d) · dilaporkan %.1f%% (n %d)",
			summary.CurriculumRate.Verified.Value, summary.CurriculumRate.Verified.N,
			summary.CurriculumRate.Reported.Value, summary.CurriculumRate.Reported.N)},
		{Text: fmt.Sprintf("Median waktu tempuh sampai pengumuman lolos: %.1f hari (terverifikasi) · %.1f hari (dilaporkan)",
			summary.TimeToPassDays.Verified, summary.TimeToPassDays.Reported)},
		{Heading: true, Text: "Capaian per tahap seleksi"},
	}
	for _, stage := range report.StagePassRate {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s: terverifikasi %.1f%% (n %d) · dilaporkan %.1f%% (n %d)",
			stage.StageLabel, stage.Rates.Verified.Value, stage.Rates.Verified.N,
			stage.Rates.Reported.Value, stage.Rates.Reported.N)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Funnel"})
	for _, step := range report.Funnel {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s: %d peserta (terverifikasi %d)",
			step.Label, step.Reported, step.Verified)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Capaian per klaster BUMN"})
	if len(report.SectorBreakdown) == 0 {
		blocks = append(blocks, ebook.Block{Text: "Belum ada laporan hasil seleksi pada periode ini."})
	}
	for _, group := range report.SectorBreakdown {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s: %.1f%% (n %d) · terverifikasi %.1f%% (n %d)",
			group.Label, group.Rates.Reported.Value, group.Rates.Reported.N,
			group.Rates.Verified.Value, group.Rates.Verified.N)})
	}

	if len(report.CohortBreakdown) > 0 {
		blocks = append(blocks, ebook.Block{Heading: true, Text: "Capaian per kelas khusus"})
		for _, group := range report.CohortBreakdown {
			blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s: %.1f%% (n %d) · terverifikasi %.1f%% (n %d)",
				group.Label, group.Rates.Reported.Value, group.Rates.Reported.N,
				group.Rates.Verified.Value, group.Rates.Verified.N)})
		}
	}

	if len(report.Targets) > 0 {
		blocks = append(blocks, ebook.Block{Heading: true, Text: "Target vs realisasi"})
		for _, target := range report.Targets {
			group := strings.TrimSpace(target.OrgName + " " + target.CohortName)
			if group == "" {
				group = "lintas program"
			}
			status := "belum tercapai"
			if target.Achieved {
				status = "tercapai"
			}
			blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s (%s): target %.2f · realisasi terverifikasi %.1f · dilaporkan %.1f — %s",
				target.MetricLabel, group, target.Target, target.ActualVerified, target.ActualReported, status)})
		}
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Catatan pembacaan"})
	for _, note := range report.Notes {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: note})
	}

	doc := ebook.Document{
		Title:    "Laporan KPI Kelulusan BUMN",
		Subtitle: fmt.Sprintf("Periode %d · dibuat %s", report.Period.Year, report.Period.GeneratedAt),
		Footer:   "BUMN Academy — angka dari laporan hasil seleksi peserta (diverifikasi admin)",
		Blocks:   blocks,
	}

	filename := fmt.Sprintf("kpi-kelulusan-bumn-%d.pdf", report.Period.Year)
	return filename, ebook.Render(doc), nil
}

/* --------------------------- ringkasan publik ------------------------------ */

// PublicOutcomes membentuk ringkasan agregat untuk halaman korporasi (§4.7).
//
// Aturan yang ditegakkan di sini — bukan di klien:
//   - hanya laporan yang pemiliknya menyetujui pemakaian agregat (`aggregate_consent`);
//   - hanya kelompok dengan `n ≥ KPI_PUBLIC_MIN_GROUP_SIZE` yang ditampilkan;
//   - tidak ada identitas peserta sama sekali pada balasan.
func (s *KPIService) PublicOutcomes(ctx context.Context, year int) (dto.PublicOutcomeInsightView, error) {
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	minGroupSize := s.cfg.Business.KPIPublicMinGroupSize

	filter := repository.KPIFilter{Year: year, ConsentOnly: true}
	participants, err := s.outcomes.KPIParticipants(ctx, filter)
	if err != nil {
		return dto.PublicOutcomeInsightView{}, err
	}
	stages, err := s.outcomes.ListStages(ctx, true)
	if err != nil {
		return dto.PublicOutcomeInsightView{}, err
	}
	finalStageKey := kpiFinalStageKey(stages)

	verifiedStages, err := s.outcomes.KPIStagePass(ctx, kpiFilterWithSource(filter, "verified"))
	if err != nil {
		return dto.PublicOutcomeInsightView{}, err
	}
	reportedStages, err := s.outcomes.KPIStagePass(ctx, filter)
	if err != nil {
		return dto.PublicOutcomeInsightView{}, err
	}
	verifiedFinal := kpiIndexStageRows(verifiedStages)[finalStageKey]
	reportedFinal := kpiIndexStageRows(reportedStages)[finalStageKey]

	sectorGroups, err := s.breakdown(ctx, filter, "sector", finalStageKey)
	if err != nil {
		return dto.PublicOutcomeInsightView{}, err
	}

	bySector, hidden := publicSectorGroups(sectorGroups, minGroupSize)

	return dto.PublicOutcomeInsightView{
		Period:       dto.KPIDashboardPeriod{Year: year, GeneratedAt: formatTime(time.Now().UTC())},
		MinGroupSize: minGroupSize,
		Summary: dto.PublicOutcomeSummaryView{
			Participants:   int(participants.TotalParticipants),
			Reported:       int(participants.Reported),
			Verified:       int(participants.Verified),
			FinalPassRate:  kpiPercentage(reportedFinal.Passed, reportedFinal.Counted),
			VerifiedRate:   kpiPercentage(verifiedFinal.Passed, verifiedFinal.Counted),
			FinalPassCount: int(reportedFinal.Passed),
		},
		BySector:     bySector,
		HiddenGroups: hidden,
		Notes: []string{
			"Angka berasal dari laporan hasil seleksi peserta yang bersedia dipakai sebagai statistik agregat.",
			fmt.Sprintf("Kelompok dengan kurang dari %d peserta tidak ditampilkan agar individu tidak dapat dikenali.", minGroupSize),
			"Angka ini bukan prediksi kelulusan seleksi BUMN.",
		},
	}, nil
}

// publicSectorGroups menyaring kelompok klaster yang boleh tampil ke publik.
//
// Fungsi murni yang menegakkan ambang agregasi (§4.7): kelompok dengan peserta
// kurang dari `minGroupSize` disembunyikan — **tidak** digabung ke "lainnya",
// karena penggabungan tetap bisa menunjuk satu-dua orang pada klaster tertentu.
// Mengembalikan kelompok yang aman ditampilkan beserta jumlah yang disembunyikan.
func publicSectorGroups(
	groups []dto.KPIBreakdownView,
	minGroupSize int,
) ([]dto.PublicOutcomeGroupView, int) {
	out := make([]dto.PublicOutcomeGroupView, 0, len(groups))
	hidden := 0
	for _, group := range groups {
		if group.Rates.Reported.N < minGroupSize {
			hidden++
			continue
		}
		out = append(out, dto.PublicOutcomeGroupView{
			Key:          group.Key,
			Label:        group.Label,
			N:            group.Rates.Reported.N,
			Passed:       group.Rates.Reported.Passed,
			PassRate:     group.Rates.Reported.Value,
			VerifiedRate: group.Rates.Verified.Value,
		})
	}
	return out, hidden
}
