package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Uji bagian murni pemetaan kompetensi (Fase 3 §4.3, §4.4): konversi skor → level,
// pencocokan kategori soal, analisis gap, inisial, dan penggabungan multi-sumber.

func TestCompetencyLevelFromScoreAmbang(t *testing.T) {
	cases := []struct {
		score float64
		want  int
	}{
		{score: 0, want: 2},
		{score: 49.9, want: 2},
		{score: 50, want: 3},
		{score: 69.9, want: 3},
		{score: 70, want: 4},
		{score: 84.9, want: 4},
		{score: 85, want: 5},
		{score: 100, want: 5},
	}

	for _, tc := range cases {
		if got := competencyLevelFromScore(tc.score); got != tc.want {
			t.Errorf("competencyLevelFromScore(%v) = %d, harap %d", tc.score, got, tc.want)
		}
	}
}

func TestCompetencyCategoryMatchesKunciDanLabel(t *testing.T) {
	cases := []struct {
		category string
		bankKey  string
		want     bool
	}{
		// Nama subtes demo harus terpetakan ke kunci bank soal.
		{"Penalaran Numerik", "numerik", true},
		{"Penalaran Verbal", "verbal", true},
		{"Logika Analitis", "analitis", true},
		{"Nilai AKHLAK", "akhlaq", true},
		{"Penalaran Kognitif", "kognitif", true},
		{"Reading & Structure", "reading", true},
		// Kunci tak dikenal tetap cocok bila namanya memuat kuncinya.
		{"Simulasi Wawasan", "wawasan", true},
		// Tidak ada hubungan.
		{"Penalaran Numerik", "figural", false},
		{"", "numerik", false},
	}

	for _, tc := range cases {
		if got := competencyCategoryMatches(tc.category, tc.bankKey); got != tc.want {
			t.Errorf("competencyCategoryMatches(%q, %q) = %v, harap %v",
				tc.category, tc.bankKey, got, tc.want)
		}
	}
}

func TestCompetencyGapTidakNegatif(t *testing.T) {
	if gap, ok := competencyGap(4, 3); gap != 1 || ok {
		t.Errorf("competencyGap(4,3) = (%d,%v), harap (1,false)", gap, ok)
	}
	if gap, ok := competencyGap(4, 4); gap != 0 || !ok {
		t.Errorf("competencyGap(4,4) = (%d,%v), harap (0,true)", gap, ok)
	}
	// Melebihi standar bukan gap negatif.
	if gap, ok := competencyGap(3, 5); gap != 0 || !ok {
		t.Errorf("competencyGap(3,5) = (%d,%v), harap (0,true)", gap, ok)
	}
	// Peserta tanpa data (level 0) tetap dihitung belum memenuhi standar.
	if gap, ok := competencyGap(4, 0); gap != 4 || ok {
		t.Errorf("competencyGap(4,0) = (%d,%v), harap (4,false)", gap, ok)
	}
}

func TestCompetencyInitials(t *testing.T) {
	cases := map[string]string{
		"Yoga Pratama":            "YP",
		"Dewi":                    "D",
		"":                        "?",
		"  Rian  Kusuma  Wijaya ": "RK",
	}
	for name, want := range cases {
		if got := competencyInitials(name); got != want {
			t.Errorf("competencyInitials(%q) = %q, harap %q", name, got, want)
		}
	}
}

func TestCompetencyBestLevelMengambilTertinggi(t *testing.T) {
	rows := []models.UserCompetency{
		{CompetencyCode: "penalaran_numerik", Source: models.CompetencySourceTryout, Level: 3},
		{CompetencyCode: "penalaran_numerik", Source: models.CompetencySourceSelf, Level: 4},
		{CompetencyCode: "integritas_akhlaq", Source: models.CompetencySourceSelf, Level: 2},
	}

	best := competencyBestLevel(rows)
	if best["penalaran_numerik"] != 4 {
		t.Errorf("level tertinggi penalaran_numerik = %d, harap 4", best["penalaran_numerik"])
	}
	if best["integritas_akhlaq"] != 2 {
		t.Errorf("level integritas_akhlaq = %d, harap 2", best["integritas_akhlaq"])
	}
}

func TestCompetencyTryoutLevelsMemetakanDanMengambilTertinggi(t *testing.T) {
	scores := []repository.UserSkillScoreRow{
		{UserID: "u1", Name: "Penalaran Numerik", AverageScore: 72},
		{UserID: "u1", Name: "Logika Analitis", AverageScore: 40},
		{UserID: "u1", Name: "Penalaran Kognitif", AverageScore: 88},
	}
	bankMap := []models.BankCompetencyMap{
		{BankKey: "numerik", CompetencyCode: "penalaran_numerik"},
		{BankKey: "analitis", CompetencyCode: "penalaran_analitis"},
		{BankKey: "kognitif", CompetencyCode: "penalaran_analitis"},
	}

	derived := competencyTryoutLevels(scores, bankMap)

	if got := derived["penalaran_numerik"].Level; got != 4 {
		t.Errorf("penalaran_numerik = %d, harap 4 (skor 72)", got)
	}
	// "Logika Analitis" (level 2) dan "Penalaran Kognitif" (level 5) memetakan ke
	// kompetensi yang sama — level tertinggi yang dipakai.
	if got := derived["penalaran_analitis"].Level; got != 5 {
		t.Errorf("penalaran_analitis = %d, harap 5 (kategori dengan skor tertinggi)", got)
	}
}

func TestCompetencyTryoutLevelsTanpaSkorKosong(t *testing.T) {
	derived := competencyTryoutLevels(nil, []models.BankCompetencyMap{
		{BankKey: "numerik", CompetencyCode: "penalaran_numerik"},
	})
	if len(derived) != 0 {
		t.Errorf("tanpa skor, pemetaan harus kosong, dapat %d", len(derived))
	}
}
