package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji bagian murni talent mapping (Fase 3 §4.1–§4.2): pembalikan item, skor
// berbobot, persentil terhadap norma, dan skor kecocokan berbobot.

func TestTalentItemScoreMembalikkanItemTerbalik(t *testing.T) {
	// Item normal: nilai apa adanya × bobot.
	if got := talentItemScore(4, false, 1); got != 4 {
		t.Errorf("talentItemScore(4,false,1) = %v, harap 4", got)
	}
	// Item terbalik pada skala 1–5: 4 → 2.
	if got := talentItemScore(4, true, 1); got != 2 {
		t.Errorf("talentItemScore(4,true,1) = %v, harap 2", got)
	}
	// Bobot butir dikalikan.
	if got := talentItemScore(5, false, 2); got != 10 {
		t.Errorf("talentItemScore(5,false,2) = %v, harap 10", got)
	}
	// Nilai di luar skala dijepit ke 1–5.
	if got := talentItemScore(9, false, 1); got != 5 {
		t.Errorf("talentItemScore(9,false,1) = %v, harap 5", got)
	}
	if got := talentItemScore(0, false, 1); got != 1 {
		t.Errorf("talentItemScore(0,false,1) = %v, harap 1", got)
	}
}

func TestTalentRawScoresMenjumlahkanPerDimensi(t *testing.T) {
	items := []models.TalentItem{
		{IDBase: models.IDBase{ID: "a"}, Dimension: "realistic", Weight: 1},
		{IDBase: models.IDBase{ID: "b"}, Dimension: "realistic", Weight: 1, Reverse: true},
		{IDBase: models.IDBase{ID: "c"}, Dimension: "social", Weight: 1},
	}
	answers := map[string]int{"a": 5, "b": 5, "c": 3}

	raw := talentRawScores(items, answers)
	// realistic: 5 + (1+5-5)=1 → 6; social: 3.
	if raw["realistic"] != 6 {
		t.Errorf("raw realistic = %v, harap 6", raw["realistic"])
	}
	if raw["social"] != 3 {
		t.Errorf("raw social = %v, harap 3", raw["social"])
	}
	// Butir tanpa jawaban tidak dihitung.
	if _, ok := raw["conventional"]; ok {
		t.Error("dimensi tanpa jawaban tidak boleh muncul")
	}
}

func TestTalentPercentileTerhadapNorma(t *testing.T) {
	// Skor tepat di rata-rata → persentil 50.
	if got := talentPercentile(10, 10, 2); got != 50 {
		t.Errorf("talentPercentile(10,10,2) = %v, harap 50", got)
	}
	// Satu simpangan baku di atas rata-rata ≈ 84,1.
	if got := talentPercentile(12, 10, 2); got < 84 || got > 84.2 {
		t.Errorf("talentPercentile(12,10,2) = %v, harap ≈84,1", got)
	}
	// Skor sangat rendah/t inggi dijepit 0–100.
	if got := talentPercentile(-100, 10, 2); got != 0 {
		t.Errorf("persentil skor sangat rendah = %v, harap 0", got)
	}
	if got := talentPercentile(1000, 10, 2); got != 100 {
		t.Errorf("persentil skor sangat tinggi = %v, harap 100", got)
	}
	// SD tidak valid → netral 50 (tidak boleh NaN).
	if got := talentPercentile(10, 10, 0); got != 50 {
		t.Errorf("talentPercentile dengan sd=0 = %v, harap 50", got)
	}
}

func TestTalentFitScoreBerbobot(t *testing.T) {
	percentiles := map[string]float64{"realistic": 80, "social": 40, "numerik": 60}

	// Rata-rata sederhana bila bobot sama.
	if got, ok := talentFitScore(percentiles, map[string]float64{"realistic": 1, "social": 1}); !ok || got != 60 {
		t.Errorf("fit (bobot sama) = %v (%v), harap 60 (true)", got, ok)
	}
	// Bobot tidak sama → condong ke dimensi berbobot besar.
	if got, ok := talentFitScore(percentiles, map[string]float64{"realistic": 3, "social": 1}); !ok || got != 70 {
		t.Errorf("fit (bobot 3:1) = %v (%v), harap 70 (true)", got, ok)
	}
	// Dimensi di bobot yang belum dinilai diabaikan; bobot kosong → tidak ada skor.
	if _, ok := talentFitScore(percentiles, map[string]float64{"figural": 2}); ok {
		t.Error("bobot tanpa dimensi yang dinilai harus menghasilkan ok=false")
	}
	if _, ok := talentFitScore(percentiles, map[string]float64{}); ok {
		t.Error("bobot kosong harus menghasilkan ok=false")
	}
	// Bobot nol/negatif tidak dihitung.
	if _, ok := talentFitScore(percentiles, map[string]float64{"realistic": 0}); ok {
		t.Error("bobot nol harus diabaikan")
	}
}

func TestTalentWeightsJSONBolakBalik(t *testing.T) {
	weights := map[string]float64{"realistic": 3, "social": 1.5}
	raw := talentWeightsToJSON(weights)

	back := talentWeightsFromJSON(raw)
	if back["realistic"] != 3 || back["social"] != 1.5 {
		t.Errorf("bobot tidak sama setelah bolak-balik: %+v", back)
	}
	// Nilai JSON berupa float64 juga dibaca.
	if got := talentWeightsFromJSON(models.JSONMap{"realistic": float64(2)}); got["realistic"] != 2 {
		t.Errorf("bobot float64 = %v, harap 2", got["realistic"])
	}
}

func TestTalentMaxRawByDimension(t *testing.T) {
	items := []models.TalentItem{
		{Dimension: "realistic", Weight: 1},
		{Dimension: "realistic", Weight: 2},
		{Dimension: "social", Weight: 1},
	}
	max := talentMaxRawByDimension(items)
	// realistic: (5×1) + (5×2) = 15; social: 5.
	if max["realistic"] != 15 {
		t.Errorf("max raw realistic = %v, harap 15", max["realistic"])
	}
	if max["social"] != 5 {
		t.Errorf("max raw social = %v, harap 5", max["social"])
	}
}

func TestTalentDimensionViewsBerurutan(t *testing.T) {
	seen := map[string]struct{}{
		models.TalentDimensionNumerik:      {},
		models.TalentDimensionRealistic:    {},
		models.TalentDimensionConventional: {},
	}
	views := talentDimensionViews(seen)
	if len(views) != 3 {
		t.Fatalf("jumlah dimensi = %d, harap 3", len(views))
	}
	// Minat (realistic, conventional) tampil sebelum bakat (numerik).
	if views[0].Dimension != models.TalentDimensionRealistic {
		t.Errorf("dimensi pertama = %q, harap realistic", views[0].Dimension)
	}
	if views[len(views)-1].Dimension != models.TalentDimensionNumerik {
		t.Errorf("dimensi terakhir = %q, harap numerik", views[len(views)-1].Dimension)
	}
}
