package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Uji rumus KPI kelulusan (Fase 2 §4.3) pada bagian yang murni perhitungan:
// pembulatan, penanganan pembagi nol, pemisahan `verified` vs `reported`, urutan
// tahapan, dan funnel yang tidak pernah menaikkan angka.

func TestKPIPercentageMembulatkanSatuDesimal(t *testing.T) {
	cases := []struct {
		passed, counted int64
		want            float64
	}{
		{passed: 1, counted: 3, want: 33.3},
		{passed: 2, counted: 3, want: 66.7},
		{passed: 3, counted: 4, want: 75},
		{passed: 0, counted: 5, want: 0},
		{passed: 5, counted: 5, want: 100},
	}

	for _, tc := range cases {
		if got := kpiPercentage(tc.passed, tc.counted); got != tc.want {
			t.Errorf("kpiPercentage(%d, %d) = %v, harap %v", tc.passed, tc.counted, got, tc.want)
		}
	}
}

func TestKPIPercentagePembagiNolMenghasilkanNol(t *testing.T) {
	// Pembagi nol tidak boleh menjadi NaN/Inf: dasbor & ekspor akan menampilkan
	// angka yang tidak bisa dibaca klien.
	for _, counted := range []int64{0, -3} {
		if got := kpiPercentage(5, counted); got != 0 {
			t.Errorf("kpiPercentage(5, %d) = %v, harap 0", counted, got)
		}
	}
}

func TestKPIRateMembawaDasarPerhitungan(t *testing.T) {
	rate := kpiRate(3, 8)

	if rate.Value != 37.5 {
		t.Errorf("Value = %v, harap 37.5", rate.Value)
	}
	if rate.N != 8 {
		t.Errorf("N = %d, harap 8 (§6.3: setiap angka menyertakan n)", rate.N)
	}
	if rate.Passed != 3 {
		t.Errorf("Passed = %d, harap 3", rate.Passed)
	}
}

func TestKPIFilterWithSourceTidakMengubahFilterAsal(t *testing.T) {
	filter := repository.KPIFilter{Year: 2026, CohortID: "cohort-1"}

	verified := kpiFilterWithSource(filter, "verified")
	if verified.Source != "verified" {
		t.Errorf("Source = %q, harap verified", verified.Source)
	}
	if verified.Year != 2026 || verified.CohortID != "cohort-1" {
		t.Error("penyaring lain harus tetap dipertahankan")
	}
	if filter.Source != "" {
		t.Errorf("filter asal tidak boleh berubah, dapat %q", filter.Source)
	}
}

func TestKPIFinalStageKeyMengikutiPenandaIsFinal(t *testing.T) {
	stages := []models.SelectionStage{
		{Key: "administrasi"},
		{Key: "offering", IsFinal: true},
		{Key: "mcu"},
	}

	if got := kpiFinalStageKey(stages); got != "offering" {
		t.Errorf("kpiFinalStageKey = %q, harap offering", got)
	}
	if got := kpiFinalStageKey([]models.SelectionStage{{Key: "administrasi"}}); got != "" {
		t.Errorf("tanpa tahap final harus kosong, dapat %q", got)
	}
}

func TestKPIGroupLabelMemakaiTaksonomiFase0(t *testing.T) {
	sector := repository.KPIBreakdownRow{GroupKey: "jasa_keuangan", Label: "jasa_keuangan"}
	if got := kpiGroupLabel("sector", sector); got == "" || got == "jasa_keuangan" {
		t.Errorf("label klaster harus label taksonomi, dapat %q", got)
	}

	cohort := repository.KPIBreakdownRow{GroupKey: "cohort-1", Label: "Kelas Khusus Batch 1"}
	if got := kpiGroupLabel("cohort", cohort); got != "Kelas Khusus Batch 1" {
		t.Errorf("label kelas = %q, harap nama kelas", got)
	}

	empty := repository.KPIBreakdownRow{GroupKey: "program-x"}
	if got := kpiGroupLabel("program", empty); got != "Tanpa program" {
		t.Errorf("label program kosong = %q, harap penanda 'Tanpa program'", got)
	}
}

func TestKPIMergeStageRatesMemisahkanTerverifikasiDanDilaporkan(t *testing.T) {
	stages := []models.SelectionStage{
		{Key: "administrasi", Label: "Seleksi Administrasi", SortOrder: 10},
		{Key: "offering", Label: "Offering", IsFinal: true, SortOrder: 20},
	}
	verified := []repository.KPIStagePassRow{
		{StageKey: "offering", Passed: 2, Counted: 4},
	}
	reported := []repository.KPIStagePassRow{
		{StageKey: "administrasi", Passed: 6, Counted: 8},
		{StageKey: "offering", Passed: 3, Counted: 8},
	}

	rates, final := kpiMergeStageRates(stages, verified, reported)

	if len(rates) != 2 {
		t.Fatalf("jumlah tahap = %d, harap 2 (semua tahap referensi muncul)", len(rates))
	}
	if rates[0].StageKey != "administrasi" || rates[1].StageKey != "offering" {
		t.Error("urutan tahap harus mengikuti daftar referensi, bukan urutan baris query")
	}

	// Tahap tanpa laporan terverifikasi tetap muncul dengan n = 0.
	if rates[0].Rates.Verified.N != 0 || rates[0].Rates.Verified.Value != 0 {
		t.Errorf("tahap tanpa laporan terverifikasi harus bernilai 0, dapat %+v", rates[0].Rates.Verified)
	}
	if rates[0].Rates.Reported.Value != 75 {
		t.Errorf("capaian administrasi (dilaporkan) = %v, harap 75", rates[0].Rates.Reported.Value)
	}

	// Angka terverifikasi tidak boleh ikut memuat laporan yang belum diperiksa.
	if final.Verified.Value != 50 || final.Verified.N != 4 {
		t.Errorf("finalPassRate terverifikasi = %+v, harap 50%% dari 4 peserta", final.Verified)
	}
	if final.Reported.Value != 37.5 || final.Reported.N != 8 {
		t.Errorf("finalPassRate dilaporkan = %+v, harap 37,5%% dari 8 peserta", final.Reported)
	}
	if !rates[1].IsFinal {
		t.Error("tahap penentu kelulusan harus ditandai isFinal")
	}
}

func TestKPIFunnelStepsTidakPernahMenaikkanAngka(t *testing.T) {
	stageRates := []dto.KPIStageRateView{
		{StageKey: "administrasi", StageLabel: "Administrasi",
			Rates: dto.KPIRatePairView{
				Verified: dto.KPIRateView{Passed: 5},
				Reported: dto.KPIRateView{Passed: 9},
			}},
		{StageKey: "tahap1_tkd", StageLabel: "TKD",
			Rates: dto.KPIRatePairView{
				Verified: dto.KPIRateView{Passed: 4},
				Reported: dto.KPIRateView{Passed: 7},
			}},
		{StageKey: "offering", StageLabel: "Offering", IsFinal: true,
			Rates: dto.KPIRatePairView{
				Verified: dto.KPIRateView{Passed: 2},
				Reported: dto.KPIRateView{Passed: 3},
			}},
	}

	steps := kpiFunnelSteps(
		repository.KPIFunnelRow{Registered: 12, Practiced: 10, CurriculumCompleted: 6, Reported: 8},
		repository.KPIFunnelRow{Registered: 20, Practiced: 15, CurriculumCompleted: 9, Reported: 14},
		stageRates,
	)

	// Langkah dasar muncul lebih dulu, lalu tahapan seleksi berurutan.
	if steps[0].Stage != "registered" || steps[3].Stage != "reported" {
		t.Fatalf("langkah dasar funnel tidak sesuai: %+v", steps[:4])
	}
	if steps[4].Stage != "administrasi" || steps[6].Stage != "offering" {
		t.Fatalf("tahapan funnel tidak berurutan: %+v", steps[4:])
	}

	// Peserta yang lolos tahap akhir dianggap melewati tahap sebelumnya, sehingga
	// angka tiap tahap tidak pernah lebih kecil dari tahap sesudahnya.
	for _, level := range []string{"Verified", "Reported"} {
		prev := -1
		for _, step := range steps[4:] {
			value := step.Reported
			if level == "Verified" {
				value = step.Verified
			}
			if prev >= 0 && value > prev {
				t.Errorf("funnel %s menaik pada tahap %s: %d > %d", level, step.Stage, value, prev)
			}
			prev = value
		}
	}

	if steps[6].Reported != 3 || steps[6].Verified != 2 {
		t.Errorf("tahap akhir funnel = %+v, harap 2 terverifikasi & 3 dilaporkan", steps[6])
	}
}
