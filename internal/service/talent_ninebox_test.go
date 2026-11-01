package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji bagian murni matriks 9-box (Fase 3 §4.4): pembagian band, pemetaan
// kotak (kuadran), dan perhitungan sumbu potensi.

func TestNineBoxBandAmbangBatas(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0, nineBoxBandLow},
		{49.9, nineBoxBandLow},
		{50, nineBoxBandMedium},
		{69.9, nineBoxBandMedium},
		{70, nineBoxBandHigh},
		{100, nineBoxBandHigh},
	}
	for _, tc := range cases {
		if got := nineBoxBand(tc.score); got != tc.want {
			t.Errorf("nineBoxBand(%v) = %q, harap %q", tc.score, got, tc.want)
		}
	}
}

func TestNineBoxBandLabel(t *testing.T) {
	if got := nineBoxBandLabel(nineBoxBandHigh); got != "Tinggi" {
		t.Errorf("label band tinggi = %q", got)
	}
	if got := nineBoxBandLabel(nineBoxBandMedium); got != "Sedang" {
		t.Errorf("label band sedang = %q", got)
	}
	if got := nineBoxBandLabel(nineBoxBandLow); got != "Rendah" {
		t.Errorf("label band rendah = %q", got)
	}
}

func TestNineBoxQuadrantsLengkapDanUnik(t *testing.T) {
	quadrants := nineBoxQuadrants()
	if len(quadrants) != 9 {
		t.Fatalf("jumlah kotak = %d, harap 9", len(quadrants))
	}

	keys := make(map[string]struct{}, len(quadrants))
	positions := make(map[[2]int]struct{}, len(quadrants))
	bands := make(map[string]struct{}, len(quadrants))
	for _, quadrant := range quadrants {
		if quadrant.Label == "" || quadrant.Action == "" {
			t.Errorf("kotak %q harus punya label & saran tindak lanjut", quadrant.Key)
		}
		if quadrant.ShortLabel == "" {
			t.Errorf("kotak %q harus punya label pendek untuk tampilan ringkas", quadrant.Key)
		}
		if quadrant.Key == "" {
			t.Error("kotak tanpa key")
		}
		if _, ok := keys[quadrant.Key]; ok {
			t.Errorf("key kotak ganda: %q", quadrant.Key)
		}
		keys[quadrant.Key] = struct{}{}

		position := [2]int{quadrant.Row, quadrant.Column}
		if _, ok := positions[position]; ok {
			t.Errorf("posisi grid ganda: %v", position)
		}
		positions[position] = struct{}{}

		if quadrant.Row < 0 || quadrant.Row > 2 || quadrant.Column < 0 || quadrant.Column > 2 {
			t.Errorf("kotak %q di luar grid 3×3: baris %d kolom %d", quadrant.Key, quadrant.Row, quadrant.Column)
		}
		bands[quadrant.PerformanceBand+"/"+quadrant.PotentialBand] = struct{}{}
	}

	// Setiap kombinasi band harus punya tepat satu kotak.
	for _, performance := range []string{nineBoxBandLow, nineBoxBandMedium, nineBoxBandHigh} {
		for _, potential := range []string{nineBoxBandLow, nineBoxBandMedium, nineBoxBandHigh} {
			if _, ok := bands[performance+"/"+potential]; !ok {
				t.Errorf("tidak ada kotak untuk band kinerja %q × potensi %q", performance, potential)
			}
		}
	}
}

func TestNineBoxQuadrantForMemetakanBand(t *testing.T) {
	cases := []struct {
		performance string
		potential   string
		wantKey     string
		wantRow     int
		wantColumn  int
	}{
		{nineBoxBandHigh, nineBoxBandHigh, "star", 0, 2},
		{nineBoxBandMedium, nineBoxBandHigh, "growth", 0, 1},
		{nineBoxBandLow, nineBoxBandHigh, "enigma", 0, 0},
		{nineBoxBandHigh, nineBoxBandMedium, "high_performer", 1, 2},
		{nineBoxBandMedium, nineBoxBandMedium, "core", 1, 1},
		{nineBoxBandLow, nineBoxBandMedium, "needs_support", 1, 0},
		{nineBoxBandHigh, nineBoxBandLow, "specialist", 2, 2},
		{nineBoxBandMedium, nineBoxBandLow, "stable", 2, 1},
		{nineBoxBandLow, nineBoxBandLow, "needs_improvement", 2, 0},
	}
	for _, tc := range cases {
		got := nineBoxQuadrantFor(tc.performance, tc.potential)
		if got.Key != tc.wantKey {
			t.Errorf("nineBoxQuadrantFor(%q,%q).Key = %q, harap %q",
				tc.performance, tc.potential, got.Key, tc.wantKey)
		}
		if got.Row != tc.wantRow || got.Column != tc.wantColumn {
			t.Errorf("nineBoxQuadrantFor(%q,%q) di baris %d kolom %d, harap baris %d kolom %d",
				tc.performance, tc.potential, got.Row, got.Column, tc.wantRow, tc.wantColumn)
		}
	}
}

func TestNineBoxQuadrantForJaringPengamanKeCore(t *testing.T) {
	got := nineBoxQuadrantFor("band-tidak-dikenal", "band-tidak-dikenal")
	if got.Key != "core" {
		t.Errorf("band tak dikenal harus jatuh ke kotak core, dapat %q", got.Key)
	}
}

func TestNineBoxPotentialRataRataDimensiBakat(t *testing.T) {
	// Hanya dimensi bakat yang dihitung; dimensi minat (RIASEC) diabaikan.
	scores := []models.TalentScore{
		{Dimension: models.TalentDimensionRealistic, Percentile: 5},
		{Dimension: models.TalentDimensionNumerik, Percentile: 80},
		{Dimension: models.TalentDimensionVerbal, Percentile: 60},
		{Dimension: models.TalentDimensionFigural, Percentile: 70},
		{Dimension: models.TalentDimensionLearningAgility, Percentile: 90},
	}
	got, ok := nineBoxPotential(scores)
	if !ok {
		t.Fatal("potensi harus tersedia saat ada skor dimensi bakat")
	}
	// (80+60+70+90)/4 = 75
	if got != 75 {
		t.Errorf("nineBoxPotential = %v, harap 75", got)
	}
}

func TestNineBoxPotentialDataBelumCukup(t *testing.T) {
	// Tanpa skor sama sekali.
	if _, ok := nineBoxPotential(nil); ok {
		t.Error("peserta tanpa skor harus ditandai belum cukup data")
	}
	// Hanya punya dimensi minat, belum mengerjakan instrumen bakat.
	onlyInterest := []models.TalentScore{
		{Dimension: models.TalentDimensionSocial, Percentile: 90},
		{Dimension: models.TalentDimensionEnterprising, Percentile: 85},
	}
	if _, ok := nineBoxPotential(onlyInterest); ok {
		t.Error("peserta tanpa skor dimensi bakat harus ditandai belum cukup data")
	}
}

func TestNineBoxPotentialMembulatkanSatuDesimal(t *testing.T) {
	scores := []models.TalentScore{
		{Dimension: models.TalentDimensionNumerik, Percentile: 33.3},
		{Dimension: models.TalentDimensionVerbal, Percentile: 33.3},
		{Dimension: models.TalentDimensionFigural, Percentile: 33.4},
	}
	got, ok := nineBoxPotential(scores)
	if !ok {
		t.Fatal("potensi harus tersedia")
	}
	if got != 33.3 {
		t.Errorf("nineBoxPotential = %v, harap 33.3 (satu desimal)", got)
	}
}

func TestTalentPotentialDimensionsHanyaDimensiBakat(t *testing.T) {
	dimensions := models.TalentPotentialDimensions()
	if len(dimensions) != 4 {
		t.Fatalf("jumlah dimensi potensi = %d, harap 4", len(dimensions))
	}
	for _, dimension := range dimensions {
		if !models.IsTalentPotentialDimension(dimension) {
			t.Errorf("dimensi %q harus dikenali sebagai dimensi potensi", dimension)
		}
	}
	if models.IsTalentPotentialDimension(models.TalentDimensionRealistic) {
		t.Error("dimensi minat tidak boleh dianggap dimensi potensi")
	}
	// Salinan: mengubah hasil tidak boleh mengubah daftar internal.
	dimensions[0] = "diubah"
	if models.TalentPotentialDimensions()[0] == "diubah" {
		t.Error("TalentPotentialDimensions harus mengembalikan salinan")
	}
}
