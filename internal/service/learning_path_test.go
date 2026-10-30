package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Uji logika kuis & jalur belajar: penilaian, batas percobaan, perhitungan
// progres, dan jaminan bahwa kunci jawaban tidak ikut terkirim ke klien.

// demoQuizQuestions adalah tiga soal kuis untuk pengujian penilaian.
func demoQuizQuestions() []quizQuestion {
	return []quizQuestion{
		{ID: "num-01", Text: "Soal 1", Options: []string{"A", "B", "C", "D"}, AnswerIndex: 1, Explanation: "Pembahasan 1"},
		{ID: "num-02", Text: "Soal 2", Options: []string{"A", "B", "C", "D"}, AnswerIndex: 2, Explanation: "Pembahasan 2"},
		{ID: "num-03", Text: "Soal 3", Options: []string{"A", "B", "C", "D"}, AnswerIndex: 0, Explanation: "Pembahasan 3"},
	}
}

func TestGradeQuizMenilaiJawabanBenarSalahDanKosong(t *testing.T) {
	answers := []models.QuizAnswer{
		{QuestionID: "num-01", SelectedIndex: intPointer(1)}, // benar
		{QuestionID: "num-02", SelectedIndex: intPointer(0)}, // salah
		// num-03 tidak dijawab
	}

	correct, review := gradeQuiz(demoQuizQuestions(), answers)
	if correct != 1 {
		t.Fatalf("jumlah jawaban benar = %d, harap 1", correct)
	}
	if len(review) != 3 {
		t.Fatalf("pembahasan harus mencakup seluruh soal kuis, dapat %d", len(review))
	}

	if !review[0].Correct || review[0].AnswerIndex != 1 {
		t.Errorf("soal 1 harus dinilai benar dengan kunci 1: %+v", review[0])
	}
	if review[1].Correct {
		t.Errorf("soal 2 harus dinilai salah: %+v", review[1])
	}
	if review[2].SelectedIndex != nil || review[2].Correct {
		t.Errorf("soal tanpa jawaban harus tercatat tidak dijawab: %+v", review[2])
	}
	// Pembahasan hanya dibuka setelah kuis dikumpulkan, jadi hasil penilaian
	// wajib membawa kunci & penjelasan.
	if review[2].AnswerIndex != 0 || review[2].Explanation != "Pembahasan 3" {
		t.Errorf("pembahasan soal 3 tidak lengkap: %+v", review[2])
	}
}

func TestGradeQuizTidakBisaDigelembungkanDenganJawabanSebagian(t *testing.T) {
	quiz := models.LessonQuiz{PassScore: 70}
	answers := []models.QuizAnswer{{QuestionID: "num-01", SelectedIndex: intPointer(1)}}

	correct, _ := gradeQuiz(demoQuizQuestions(), answers)
	score := quiz.ScoreFor(correct, len(demoQuizQuestions()))

	if correct != 1 || score != 33 {
		t.Fatalf("benar=%d skor=%d, harap benar=1 skor=33 (pembagi adalah jumlah soal kuis)", correct, score)
	}
	if quiz.PassedWith(score) {
		t.Errorf("skor %d tidak boleh lulus dengan ambang %d", score, quiz.PassScore)
	}
}

func TestNormalizeAnswersMengambilJawabanTerakhir(t *testing.T) {
	inputs := []dto.QuizAnswerInput{
		{QuestionID: "num-01", SelectedIndex: intPointer(0)},
		{QuestionID: " num-02 ", SelectedIndex: intPointer(3)},
		{QuestionID: "num-01", SelectedIndex: intPointer(2)}, // jawaban terbaru menang
		{QuestionID: "   "},    // id kosong diabaikan
		{QuestionID: "num-03"}, // tidak dijawab
	}

	answers := normalizeAnswers(inputs)
	if len(answers) != 3 {
		t.Fatalf("jumlah jawaban = %d, harap 3", len(answers))
	}
	if answers[0].SelectedIndex == nil || *answers[0].SelectedIndex != 2 {
		t.Errorf("jawaban terakhir untuk num-01 harus dipakai: %+v", answers[0])
	}
	if answers[1].QuestionID != "num-02" {
		t.Errorf("id soal harus dirapikan dari spasi: %q", answers[1].QuestionID)
	}
	if answers[2].SelectedIndex != nil {
		t.Errorf("jawaban tanpa pilihan harus tetap kosong: %+v", answers[2])
	}
}

func TestAttemptsLeftMenghitungSisaPercobaan(t *testing.T) {
	cases := []struct {
		name        string
		maxAttempts int
		used        int
		want        int
	}{
		{name: "tanpa batas", maxAttempts: 0, used: 5, want: -1},
		{name: "masih ada sisa", maxAttempts: 3, used: 1, want: 2},
		{name: "habis", maxAttempts: 3, used: 3, want: 0},
		{name: "melebihi batas tetap 0", maxAttempts: 2, used: 4, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := attemptsLeft(tc.maxAttempts, tc.used); got != tc.want {
				t.Errorf("attemptsLeft(%d, %d) = %d, harap %d", tc.maxAttempts, tc.used, got, tc.want)
			}
		})
	}
}

func TestLessonQuizViewTidakMembocorkanKunciJawaban(t *testing.T) {
	// Bentuk data seperti yang dikirim quizView ke peserta.
	view := dto.LessonQuizView{
		ID:        "quiz-numerik-deret",
		Title:     "Kuis Deret & Pola",
		Questions: []dto.QuizQuestionView{{ID: "num-01", Number: 1, Text: "Soal 1", Options: []string{"A", "B"}}},
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("gagal marshal tampilan kuis: %v", err)
	}

	payload := string(raw)
	for _, forbidden := range []string{"answerIndex", "explanation", "\"correct\""} {
		if strings.Contains(payload, forbidden) {
			t.Errorf("payload kuis peserta memuat %q: %s", forbidden, payload)
		}
	}
}

func TestPathProgressPercentMemakaiLangkahWajibSebagaiBasis(t *testing.T) {
	path := models.LearningPath{Steps: []models.LearningPathStep{
		{ModuleID: "mod-1", Required: true},
		{ModuleID: "mod-2", Required: true},
		{ModuleID: "mod-3", Required: false},
	}}
	stats := map[string]repository.LessonStat{
		"mod-1": {ModuleID: "mod-1", Total: 2, Completed: 2}, // selesai
		"mod-2": {ModuleID: "mod-2", Total: 4, Completed: 1}, // separuh
		"mod-3": {ModuleID: "mod-3", Total: 1, Completed: 1}, // opsional, selesai
	}

	counts := countPathSteps(path, stats)
	if counts.required != 2 || counts.completedRequired != 1 {
		t.Fatalf("hitungan langkah wajib salah: %+v", counts)
	}
	if counts.total != 3 || counts.completed != 2 {
		t.Fatalf("hitungan seluruh langkah salah: %+v", counts)
	}
	if got := pathProgressPercent(counts); got != 50 {
		t.Errorf("progres jalur = %d%%, harap 50%% (1 dari 2 langkah wajib)", got)
	}
}

func TestPathProgressPercentTanpaLangkahWajibMemakaiSeluruhLangkah(t *testing.T) {
	path := models.LearningPath{Steps: []models.LearningPathStep{
		{ModuleID: "mod-1", Required: false},
		{ModuleID: "mod-2", Required: false},
	}}
	stats := map[string]repository.LessonStat{
		"mod-1": {ModuleID: "mod-1", Total: 2, Completed: 2},
		"mod-2": {ModuleID: "mod-2", Total: 2, Completed: 0},
	}

	if got := pathProgressPercent(countPathSteps(path, stats)); got != 50 {
		t.Errorf("progres jalur = %d%%, harap 50%%", got)
	}
}

func TestPathProgressPercentJalurKosongNol(t *testing.T) {
	if got := pathProgressPercent(countPathSteps(models.LearningPath{}, nil)); got != 0 {
		t.Errorf("progres jalur tanpa langkah = %d, harap 0", got)
	}
}

func TestModuleCompletedMenolakModulTanpaPelajaranAktif(t *testing.T) {
	// Modul tanpa pelajaran aktif tidak pernah dianggap selesai, supaya jalur
	// yang belum punya materi tidak menampilkan progres palsu 100%.
	if moduleCompleted(repository.LessonStat{ModuleID: "mod-1", Total: 0, Completed: 0}) {
		t.Error("modul tanpa pelajaran tidak boleh dianggap selesai")
	}
	if !moduleCompleted(repository.LessonStat{ModuleID: "mod-1", Total: 3, Completed: 3}) {
		t.Error("modul dengan seluruh pelajaran selesai harus dianggap selesai")
	}
	if moduleCompleted(repository.LessonStat{ModuleID: "mod-1", Total: 3, Completed: 2}) {
		t.Error("modul yang belum tuntas tidak boleh dianggap selesai")
	}
}

func TestPercentOfMembatasiNolSampaiSeratus(t *testing.T) {
	cases := []struct {
		name      string
		part, all int
		want      int
	}{
		{name: "pembagi nol", part: 3, all: 0, want: 0},
		{name: "bagian nol", part: 0, all: 5, want: 0},
		{name: "pembulatan ke bawah", part: 2, all: 3, want: 66},
		{name: "lebih dari total dibatasi 100", part: 5, all: 4, want: 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentOf(tc.part, tc.all); got != tc.want {
				t.Errorf("percentOf(%d, %d) = %d, harap %d", tc.part, tc.all, got, tc.want)
			}
		})
	}
}

// intPointer adalah pembantu pembentukan pointer int untuk kasus uji.
func intPointer(value int) *int { return &value }
