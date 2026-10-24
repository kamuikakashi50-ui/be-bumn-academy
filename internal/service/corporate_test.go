package service

import (
	"strings"
	"testing"
)

func TestMaskNameMenyamarkanSetiapKata(t *testing.T) {
	cases := map[string]string{
		"Ilham Pratama":          "I*** P***",
		"Rina":                   "R***",
		"ahmad fauzi ramadhan":   "A*** F*** R***",
		"  Dewi   Lestari  ":     "D*** L***",
		"Ayu Kartika Wulandari":  "A*** K*** W***",
		"Ni Made Ayu":            "N*** M*** A***",
		"":                       "Peserta",
		"   ":                    "Peserta",
		"123 Peserta":            "1*** P***",
		"-tanpa-huruf-awal Nama": "*** N***",
	}

	for input, want := range cases {
		if got := MaskName(input); got != want {
			t.Errorf("MaskName(%q) = %q, harap %q", input, got, want)
		}
	}
}

// Nama asli tidak boleh bocor: selain huruf pertama tiap kata, tidak ada
// karakter lain dari nama yang tersisa pada hasil penyamaran.
func TestMaskNameTidakMembocorkanNama(t *testing.T) {
	nama := "Ilham Pratama"
	masked := MaskName(nama)

	for _, word := range strings.Fields(nama) {
		sisa := strings.ToLower(string([]rune(word)[1:]))
		if sisa != "" && strings.Contains(strings.ToLower(masked), sisa) {
			t.Fatalf("hasil penyamaran %q masih memuat bagian nama %q", masked, sisa)
		}
	}
	if !strings.Contains(masked, "***") {
		t.Fatalf("hasil penyamaran tidak memakai penanda ***: %q", masked)
	}
}

func TestMonthLabelBahasaIndonesia(t *testing.T) {
	cases := map[string]string{
		"2026-10": "Okt 2026",
		"2026-01": "Jan 2026",
		"2025-12": "Des 2025",
		"2026-08": "Agu 2026",
		"ngawur":  "ngawur", // format tak dikenal dikembalikan apa adanya
	}

	for input, want := range cases {
		if got := monthLabel(input); got != want {
			t.Errorf("monthLabel(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestRoundTo1DanPercentageFloat(t *testing.T) {
	rounds := map[float64]float64{
		0:     0,
		66.66: 66.7,
		62.5:  62.5,
		99.94: 99.9,
		100:   100,
	}
	for input, want := range rounds {
		if got := roundTo1(input); got != want {
			t.Errorf("roundTo1(%v) = %v, harap %v", input, got, want)
		}
	}

	percentages := []struct {
		part, total int64
		want        float64
	}{
		{0, 0, 0},    // pembagi nol tidak boleh panic
		{3, 0, 0},    // total kosong
		{1, 4, 25},   // seperempat
		{2, 3, 66.7}, // dibulatkan satu desimal
		{4, 4, 100},  // seluruhnya lulus
	}
	for _, tc := range percentages {
		if got := percentageFloat(tc.part, tc.total); got != tc.want {
			t.Errorf("percentageFloat(%d, %d) = %v, harap %v", tc.part, tc.total, got, tc.want)
		}
	}
}
