package service

import (
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

func TestPercent(t *testing.T) {
	cases := []struct {
		part, total, want int
	}{
		{0, 0, 0}, // pembagi nol tidak boleh panic
		{5, 0, 0}, // pembagi nol
		{10, 10, 100},
		{0, 10, 0},
		{7, 10, 70},
		{2, 3, 67}, // 66.67 dibulatkan ke atas
		{1, 3, 33}, // 33.33 dibulatkan ke bawah
		{5, 8, 63}, // 62.5 dibulatkan ke atas
		{4, 5, 80},
	}

	for _, tc := range cases {
		if got := percent(tc.part, tc.total); got != tc.want {
			t.Errorf("percent(%d, %d) = %d, harap %d", tc.part, tc.total, got, tc.want)
		}
	}
}

func TestDurationSeconds(t *testing.T) {
	now := time.Now().UTC()

	t.Run("sesi berjalan normal", func(t *testing.T) {
		session := &models.TryoutSession{
			StartedAt: now.Add(-90 * time.Second),
			ExpiresAt: now.Add(30 * time.Minute),
		}
		got := durationSeconds(session)
		if got < 89 || got > 92 {
			t.Errorf("durationSeconds = %d, harap sekitar 90", got)
		}
	})

	t.Run("sesi melewati tenggat dipotong di expires_at", func(t *testing.T) {
		session := &models.TryoutSession{
			StartedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		}
		if got := durationSeconds(session); got != 3600 {
			t.Errorf("durationSeconds = %d, harap 3600", got)
		}
	})
}

func TestRemainingSeconds(t *testing.T) {
	now := time.Now().UTC()
	session := models.TryoutSession{
		SubtestDeadlines: models.TimeList{
			now.Add(60 * time.Second),
			now.Add(-10 * time.Second), // sudah lewat
			{},                         // belum dimulai
		},
	}

	if got := session.RemainingSeconds(0, now); got < 58 || got > 60 {
		t.Errorf("subtes 0 sisa = %d, harap sekitar 60", got)
	}
	if got := session.RemainingSeconds(1, now); got != 0 {
		t.Errorf("subtes lewat tenggat sisa = %d, harap 0", got)
	}
	if got := session.RemainingSeconds(2, now); got != 0 {
		t.Errorf("subtes belum mulai sisa = %d, harap 0", got)
	}
	if got := session.RemainingSeconds(9, now); got != 0 {
		t.Errorf("index di luar rentang sisa = %d, harap 0", got)
	}
}

func TestSubtestQuestionCount(t *testing.T) {
	counts := map[string]map[string]int{
		"to-tkd-01": {"numerik": 5, "verbal": 3},
		"to-tkd-02": {"numerik": 2},
	}
	subtest := models.Subtest{Banks: models.StringList{"numerik", "verbal", "tidak-ada"}}

	if got := subtestQuestionCount("to-tkd-01", subtest, counts); got != 8 {
		t.Errorf("subtestQuestionCount = %d, harap 8", got)
	}
	// Soal milik tryout lain tidak boleh ikut terhitung.
	if got := subtestQuestionCount("to-tkd-02", subtest, counts); got != 2 {
		t.Errorf("subtestQuestionCount tryout lain = %d, harap 2", got)
	}
	if got := subtestQuestionCount("to-tkd-01", models.Subtest{}, counts); got != 0 {
		t.Errorf("subtes tanpa bank = %d, harap 0", got)
	}
	if got := subtestQuestionCount("to-tidak-ada", subtest, counts); got != 0 {
		t.Errorf("tryout tanpa soal = %d, harap 0", got)
	}
}

func TestInitialsOf(t *testing.T) {
	cases := map[string]string{
		"Ilham Pratama":     "IP",
		"rina":              "R",
		"Tim Akademik BUMN": "TA",
		"":                  "",
	}
	for input, want := range cases {
		if got := initialsOf(input); got != want {
			t.Errorf("initialsOf(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(time.Time{}); got != "" {
		t.Errorf("waktu nol harus menjadi string kosong, dapat %q", got)
	}

	stamp := time.Date(2026, 3, 14, 9, 20, 0, 0, time.UTC)
	if got := formatTime(stamp); got != "2026-03-14T09:20:00Z" {
		t.Errorf("formatTime = %q", got)
	}
	if got := formatTimePtr(nil); got != "" {
		t.Errorf("formatTimePtr(nil) = %q, harap kosong", got)
	}
	if got := formatTimePtr(&stamp); got != "2026-03-14T09:20:00Z" {
		t.Errorf("formatTimePtr = %q", got)
	}
}

func TestToAttemptView(t *testing.T) {
	attempt := models.Attempt{
		ID:              "at-1",
		TryoutID:        "to-demo-nasional",
		Title:           "Tryout Demo Nasional",
		Score:           72,
		Correct:         7,
		Total:           10,
		Status:          models.AttemptFailed,
		DurationSeconds: 420,
		TakenAt:         time.Date(2026, 3, 14, 9, 20, 0, 0, time.UTC),
		Categories: []models.AttemptCategory{
			{Name: "Penalaran Numerik", Score: 80, Correct: 4, Total: 5},
		},
	}

	view := toAttemptView(attempt)
	if view.ID != "at-1" || view.Score != 72 || view.Status != "perlu perbaikan" {
		t.Fatalf("toAttemptView menghasilkan %+v", view)
	}
	if len(view.Categories) != 1 || view.Categories[0].Score != 80 {
		t.Fatalf("kategori tidak terpetakan: %+v", view.Categories)
	}
	if view.TakenAt != "2026-03-14T09:20:00Z" {
		t.Fatalf("takenAt = %q", view.TakenAt)
	}
}

func TestEmptyIfNil(t *testing.T) {
	if got := emptyIfNil[string](nil); got == nil || len(got) != 0 {
		t.Errorf("emptyIfNil(nil) harus menjadi slice kosong, dapat %#v", got)
	}
	if got := emptyIfNil([]string{"a"}); len(got) != 1 {
		t.Errorf("emptyIfNil tidak boleh mengubah isi")
	}
}
