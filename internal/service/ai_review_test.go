package service

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/ai"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
)

// configWithAI membentuk konfigurasi minimal untuk pengujian mentor.
func configWithAI(enabled bool, quota int, stream bool) *config.Config {
	cfg := &config.Config{}
	cfg.AI.Enabled = enabled
	cfg.AI.DailyQuota = quota
	cfg.AI.StreamEnabled = stream
	return cfg
}

func analysisFixture() ai.AttemptContext {
	return ai.AttemptContext{
		Title:            "Tryout Demo Nasional",
		Score:            50,
		Correct:          2,
		Total:            4,
		PassingThreshold: 70,
		Subtests: []ai.SubtestContext{
			{Name: "Penalaran Numerik", Score: 40, Correct: 2, Total: 5},
			{Name: "Penalaran Verbal", Score: 80, Correct: 4, Total: 5},
		},
		Questions: []ai.QuestionContext{
			{ID: "num-1", Number: 1, Subtest: "Penalaran Numerik", Text: "deret angka", IsCorrect: false},
			{ID: "verb-1", Number: 2, Subtest: "Penalaran Verbal", Text: "sinonim", IsCorrect: true},
		},
	}
}

func TestParseAnalysisJSONMenerimaJSONPolos(t *testing.T) {
	raw := `{"headline":"Skor 50","summary":"ringkasan","strengths":[],"weaknesses":[],
	         "focusAreas":[],"studyPlan":[],"questionNotes":[]}`

	result, err := parseAnalysisJSON(raw)
	if err != nil {
		t.Fatalf("parseAnalysisJSON error: %v", err)
	}
	if result.Headline != "Skor 50" || result.Summary != "ringkasan" {
		t.Fatalf("hasil = %+v", result)
	}
}

func TestParseAnalysisJSONMenerimaPagarMarkdownDanProsa(t *testing.T) {
	raw := "Berikut analisisnya:\n```json\n" +
		`{"headline":"H","summary":"S"}` +
		"\n```\nSemoga membantu!"

	result, err := parseAnalysisJSON(raw)
	if err != nil {
		t.Fatalf("parseAnalysisJSON error: %v", err)
	}
	if result.Headline != "H" {
		t.Fatalf("headline = %q", result.Headline)
	}
}

func TestParseAnalysisJSONMenolakBalasanBukanJSON(t *testing.T) {
	if _, err := parseAnalysisJSON("maaf saya tidak bisa membantu"); err == nil {
		t.Fatal("balasan tanpa objek JSON harus ditolak")
	}
	if _, err := parseAnalysisJSON(""); err == nil {
		t.Fatal("balasan kosong harus ditolak")
	}
}

func TestSanitizeAnalysisMengisiBagianKosongDariBaseline(t *testing.T) {
	attemptCtx := analysisFixture()
	baseline := ai.BaselineAnalysis(attemptCtx)

	// Model hanya mengisi headline; sisanya kosong.
	out := sanitizeAnalysis(ai.AnalysisResult{Headline: "Skor 50 — fokus Numerik"}, baseline, attemptCtx)

	if out.Headline != "Skor 50 — fokus Numerik" {
		t.Fatalf("headline tidak dipakai: %q", out.Headline)
	}
	if out.Summary == "" || len(out.Strengths) == 0 || len(out.FocusAreas) == 0 || len(out.StudyPlan) == 0 {
		t.Fatalf("bagian kosong harus diisi baseline: %+v", out)
	}
}

func TestSanitizeAnalysisMembuangCatatanSoalDiLuarAttempt(t *testing.T) {
	attemptCtx := analysisFixture()
	baseline := ai.BaselineAnalysis(attemptCtx)

	parsed := ai.AnalysisResult{
		Headline: "H",
		Summary:  "S",
		QuestionNotes: []ai.Note{
			{Title: "Soal 1", Detail: "detail sah", QuestionID: "num-1"},
			{Title: "Soal 99", Detail: "soal tidak ada", QuestionID: "num-99"},
			{Title: "", Detail: "judul kosong", QuestionID: "verb-1"},
			{Title: "Tanpa detail", Detail: ""},
		},
	}

	out := sanitizeAnalysis(parsed, baseline, attemptCtx)
	if len(out.QuestionNotes) != 1 {
		t.Fatalf("jumlah catatan = %d, harap 1 (hanya yang valid)", len(out.QuestionNotes))
	}
	if out.QuestionNotes[0].QuestionID != "num-1" {
		t.Fatalf("catatan = %+v", out.QuestionNotes[0])
	}
}

func TestSanitizeAnalysisMembatasiJumlahButir(t *testing.T) {
	attemptCtx := analysisFixture()
	baseline := ai.BaselineAnalysis(attemptCtx)

	notes := make([]ai.Note, 0, 20)
	for i := 0; i < 20; i++ {
		notes = append(notes, ai.Note{Title: "butir", Detail: "detail"})
	}

	out := sanitizeAnalysis(ai.AnalysisResult{Headline: "H", Summary: "S", StudyPlan: notes}, baseline, attemptCtx)
	if len(out.StudyPlan) > 6 {
		t.Fatalf("studyPlan = %d butir, harap maksimal 6", len(out.StudyPlan))
	}
}

func TestSanitizeAnalysisMemotongTeksTerlaluPanjang(t *testing.T) {
	attemptCtx := analysisFixture()
	baseline := ai.BaselineAnalysis(attemptCtx)

	long := strings.Repeat("a", 5000)
	out := sanitizeAnalysis(ai.AnalysisResult{Headline: long, Summary: long}, baseline, attemptCtx)

	if len(out.Headline) > 200 {
		t.Fatalf("headline = %d karakter, harap dipotong", len(out.Headline))
	}
	if len(out.Summary) > 2000 {
		t.Fatalf("summary = %d karakter, harap dipotong", len(out.Summary))
	}
}

func TestClampText(t *testing.T) {
	if got := clampText("  pendek  ", 50); got != "pendek" {
		t.Fatalf("clampText = %q", got)
	}
	if got := clampText("abcdef", 3); got != "abc" {
		t.Fatalf("clampText = %q", got)
	}
	if got := clampText("abc", 0); got != "abc" {
		t.Fatalf("batas 0 berarti tanpa pemotongan, dapat %q", got)
	}
}

func TestConversationTitle(t *testing.T) {
	if got := conversationTitle(""); got != "Ulasan Mentor AI" {
		t.Fatalf("judul default = %q", got)
	}
	if got := conversationTitle("Tryout TKD Batch 1"); got != "Ulasan Tryout TKD Batch 1" {
		t.Fatalf("judul = %q", got)
	}
}

func TestResolveFocus(t *testing.T) {
	attemptCtx := analysisFixture()

	if focus, err := resolveFocus(attemptCtx, ""); err != nil || focus != nil {
		t.Fatalf("tanpa questionId harus mengembalikan fokus kosong, dapat %v/%v", focus, err)
	}

	focus, err := resolveFocus(attemptCtx, "num-1")
	if err != nil || focus == nil || focus.Number != 1 {
		t.Fatalf("fokus = %v, err = %v", focus, err)
	}

	if _, err := resolveFocus(attemptCtx, "soal-tidak-ada"); err == nil {
		t.Fatal("soal di luar attempt harus ditolak")
	}
}

func TestNewMessageScopeDanMetadataSoal(t *testing.T) {
	plain := newMessage("conv-1", "user-1", "user", "halo", nil)
	if plain.Scope != "attempt" || plain.QuestionID != nil {
		t.Fatalf("pesan tanpa fokus = %+v", plain)
	}

	question := ai.QuestionContext{ID: "num-1", Number: 3, Subtest: "Penalaran Numerik"}
	focused := newMessage("conv-1", "user-1", "assistant", "penjelasan", &question)
	if focused.Scope != "question" {
		t.Fatalf("scope = %q, harap question", focused.Scope)
	}
	if focused.QuestionID == nil || *focused.QuestionID != "num-1" {
		t.Fatal("questionId harus terisi")
	}
	if focused.QuestionNumber == nil || *focused.QuestionNumber != 3 {
		t.Fatal("nomor soal harus terisi")
	}
	if focused.SubtestName == nil || *focused.SubtestName != "Penalaran Numerik" {
		t.Fatal("nama subtes harus terisi")
	}
}

func TestToNotesMemberiPrioritas(t *testing.T) {
	notes := toNotes([]ai.Note{
		{Title: "satu", Detail: "d1"},
		{Title: "dua", Detail: "d2"},
	})
	if len(notes) != 2 || notes[0].Priority != 1 || notes[1].Priority != 2 {
		t.Fatalf("prioritas = %+v", notes)
	}
	if got := fromNotes(notes); len(got) != 2 || got[1].Title != "dua" {
		t.Fatalf("fromNotes = %+v", got)
	}
}

func TestMentorStatusTanpaFallback(t *testing.T) {
	service := &MentorService{
		provider: ai.NewStub(),
		cfg:      configWithAI(true, 50, true),
	}

	status := service.Status()
	if !status.Enabled || status.Provider != "stub" || status.DailyQuota != 50 {
		t.Fatalf("status = %+v", status)
	}
	if status.Degraded {
		t.Fatal("tanpa fallback, status tidak boleh degraded")
	}
}
