package ai

import (
	"strings"
	"testing"
)

// fixtureAttempt menyusun konteks hasil tryout untuk pengujian.
func fixtureAttempt() AttemptContext {
	selectedWrong := 0
	selectedRight := 1

	return AttemptContext{
		Title:            "Tryout Demo Nasional",
		TryoutID:         "to-demo-nasional",
		Score:            50,
		Correct:          2,
		Total:            4,
		PassingThreshold: 70,
		Status:           "perlu perbaikan",
		DurationSeconds:  480,
		Subtests: []SubtestContext{
			{Name: "Penalaran Numerik", Score: 50, Correct: 1, Total: 2},
			{Name: "Penalaran Verbal", Score: 50, Correct: 1, Total: 2},
		},
		Questions: []QuestionContext{
			{
				ID: "num-1", Number: 1, Subtest: "Penalaran Numerik", BankLabel: "Penalaran Numerik",
				Text: "3, 6, 11, 18, 27, …", Options: []string{"36", "37", "38", "40"},
				CorrectIndex: 2, SelectedIndex: &selectedWrong, IsCorrect: false,
				Explanation: "Selisih naik dua setiap langkah, jadi 27 + 11 = 38.",
			},
			{
				ID: "num-2", Number: 2, Subtest: "Penalaran Numerik", BankLabel: "Penalaran Numerik",
				Text: "4 pekerja menyelesaikan pekerjaan dalam 12 hari.", Options: []string{"6 hari", "8 hari"},
				CorrectIndex: 1, SelectedIndex: &selectedRight, IsCorrect: true,
				Explanation: "Total beban 48 hari-orang dibagi 6 pekerja = 8 hari.",
			},
			{
				ID: "verb-1", Number: 3, Subtest: "Penalaran Verbal", BankLabel: "Penalaran Verbal",
				Text: "Kata KREDIBEL paling dekat maknanya dengan …", Options: []string{"Terkenal", "Dapat dipercaya"},
				CorrectIndex: 1, SelectedIndex: nil, IsCorrect: false, IsUnanswered: true,
				Explanation: "Kredibel berarti dapat dipercaya.",
			},
			{
				ID: "verb-2", Number: 4, Subtest: "Penalaran Verbal", BankLabel: "Penalaran Verbal",
				Text: "Antonim dari kata EFISIEN adalah …", Options: []string{"Hemat", "Cepat"},
				CorrectIndex: 1, SelectedIndex: &selectedRight, IsCorrect: true,
				Explanation: "Efisien berlawanan dengan boros.",
			},
		},
	}
}

func names(subtests []SubtestContext) []string {
	out := make([]string, 0, len(subtests))
	for _, subtest := range subtests {
		out = append(out, subtest.Name)
	}
	return out
}

/* ------------------------------- urutan subtes ------------------------------ */

func TestSortSubtestsTidakTerbalik(t *testing.T) {
	ctx := AttemptContext{
		PassingThreshold: 70,
		Subtests: []SubtestContext{
			{Name: "A", Score: 90},
			{Name: "B", Score: 40},
			{Name: "C", Score: 65},
		},
	}

	strongest := ctx.StrongestSubtests(3)
	if strongest[0].Name != "A" || strongest[2].Name != "B" {
		t.Fatalf("StrongestSubtests harus menurun (A, C, B), dapat %v", names(strongest))
	}

	weakest := ctx.WeakestSubtests(3)
	if weakest[0].Name != "B" || weakest[2].Name != "A" {
		t.Fatalf("WeakestSubtests harus menaik (B, C, A), dapat %v", names(weakest))
	}

	if got := ctx.WeakestSubtests(1); len(got) != 1 || got[0].Name != "B" {
		t.Fatalf("WeakestSubtests(1) = %v, harap B", names(got))
	}
}

func TestStrongDanWeakSubtestsMengikutiAmbang(t *testing.T) {
	ctx := AttemptContext{
		PassingThreshold: 70,
		Subtests: []SubtestContext{
			{Name: "A", Score: 90},
			{Name: "B", Score: 75},
			{Name: "C", Score: 40},
		},
	}

	if got := strings.Join(names(ctx.StrongSubtests(5)), ","); got != "A,B" {
		t.Fatalf("StrongSubtests = %v, harap [A B]", got)
	}
	if got := strings.Join(names(ctx.WeakSubtests(5)), ","); got != "C" {
		t.Fatalf("WeakSubtests = %v, harap [C]", got)
	}
}

func TestWeakSubtestsKosongBilaSemuaLulus(t *testing.T) {
	ctx := AttemptContext{
		PassingThreshold: 70,
		Subtests: []SubtestContext{
			{Name: "A", Score: 90},
			{Name: "B", Score: 80},
		},
	}
	if got := ctx.WeakSubtests(3); len(got) != 0 {
		t.Fatalf("WeakSubtests harus kosong, dapat %v", names(got))
	}
}

func TestStrongSubtestsSelaluAdaIsi(t *testing.T) {
	// Semua subtes di bawah ambang → tetap ada kekuatan relatif.
	ctx := AttemptContext{
		PassingThreshold: 70,
		Subtests: []SubtestContext{
			{Name: "A", Score: 30},
			{Name: "B", Score: 50},
		},
	}
	got := ctx.StrongSubtests(2)
	if len(got) != 1 || got[0].Name != "B" {
		t.Fatalf("StrongSubtests = %v, harap [B] (relatif terbaik)", names(got))
	}
}
