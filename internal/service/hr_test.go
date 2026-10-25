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

func TestTopLearnerNote(t *testing.T) {
	cases := []struct {
		name         string
		attemptCount int64
		averageScore float64
		bestScore    int
		want         string
	}{
		{"skor terbaik di bawah ambang", 4, 60, 65, "Skor terbaik belum mencapai ambang"},
		{"sekali latihan di atas ambang", 1, 88, 88, "Skor terbaik di atas ambang"},
		{"dua percobaan belum disebut konsisten", 2, 85, 90, "Skor terbaik di atas ambang"},
		{"tiga percobaan & rata-rata di atas ambang", 3, 78, 90, "Konsisten di atas ambang"},
		{"banyak percobaan tetapi rata-rata di bawah ambang", 5, 66, 80, "Skor terbaik di atas ambang"},
		{"tepat di ambang dianggap lulus", 1, 70, 70, "Skor terbaik di atas ambang"},
	}

	for _, tc := range cases {
		got := topLearnerNote(tc.attemptCount, tc.averageScore, tc.bestScore, 70)
		if got != tc.want {
			t.Errorf("%s: topLearnerNote() = %q, harap %q", tc.name, got, tc.want)
		}
	}
}
