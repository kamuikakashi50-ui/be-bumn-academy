package service

import "testing"

func TestProgramLabelNormalizesEmpty(t *testing.T) {
	cases := map[string]string{
		"":                  "Belum diisi",
		"   ":               "Belum diisi",
		"Tim Akademik":      "Tim Akademik",
		"  Paket Simulasi ": "Paket Simulasi",
	}

	for input, want := range cases {
		if got := programLabel(input); got != want {
			t.Errorf("programLabel(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestAtRiskReason(t *testing.T) {
	below := 45
	above := 88

	cases := []struct {
		name         string
		attemptCount int64
		lastScore    *int
		want         string
	}{
		{"belum pernah latihan", 0, nil, "Belum pernah mengerjakan tryout"},
		{"skor kosong walau ada percobaan", 2, nil, "Belum pernah mengerjakan tryout"},
		{"skor di bawah ambang", 3, &below, "Skor terakhir di bawah ambang"},
		{"skor di atas ambang (tidak masuk daftar)", 1, &above, "Skor terakhir di bawah ambang"},
	}

	for _, tc := range cases {
		if got := atRiskReason(tc.attemptCount, tc.lastScore); got != tc.want {
			t.Errorf("%s: atRiskReason() = %q, harap %q", tc.name, got, tc.want)
		}
	}
}
