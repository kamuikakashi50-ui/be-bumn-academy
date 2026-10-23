package ai

// Note adalah satu butir analisis hasil tryout.
type Note struct {
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Subtest    string `json:"subtest,omitempty"`
	QuestionID string `json:"questionId,omitempty"`
	Priority   int    `json:"priority,omitempty"`
}

// AnalysisResult adalah analisis hasil tryout dalam bentuk terstruktur.
type AnalysisResult struct {
	Headline      string `json:"headline"`
	Summary       string `json:"summary"`
	Strengths     []Note `json:"strengths"`
	Weaknesses    []Note `json:"weaknesses"`
	FocusAreas    []Note `json:"focusAreas"`
	StudyPlan     []Note `json:"studyPlan"`
	QuestionNotes []Note `json:"questionNotes"`
}

// ReadinessScore menghitung skor kesiapan (0–100) secara deterministik.
//
// Skor ini SENGAJA tidak dihasilkan model bahasa: nilainya harus bisa
// dipertanggungjawabkan dan konsisten setiap kali halaman dibuka. Rumusnya
// menggabungkan tiga hal yang relevan untuk kesiapan ujian:
//
//   - 70% skor total (pencapaian),
//   - 30% pemerataan antar subtes (100 − selisih skor tertinggi & terendah),
//     karena seleksi biasanya menetapkan ambang per subtes,
//   - pengurangan karena soal yang tidak dijawab (indikasi masalah waktu).
func ReadinessScore(ctx AttemptContext) int {
	base := float64(ctx.Score)

	spread := 0
	if len(ctx.Subtests) > 1 {
		highest, lowest := ctx.Subtests[0].Score, ctx.Subtests[0].Score
		for _, subtest := range ctx.Subtests {
			if subtest.Score > highest {
				highest = subtest.Score
			}
			if subtest.Score < lowest {
				lowest = subtest.Score
			}
		}
		spread = highest - lowest
	}

	evenness := float64(100 - spread)
	score := 0.7*base + 0.3*evenness

	if ctx.Total > 0 {
		unansweredRatio := float64(countUnanswered(ctx)) / float64(ctx.Total)
		score -= 12 * unansweredRatio
	}

	return clampScore(int(score + 0.5))
}

// Readiness menentukan label kesiapan berdasarkan skor dan ambang batas.
func Readiness(ctx AttemptContext) string {
	threshold := ctx.ThresholdOrDefault()

	switch {
	case ctx.Score >= threshold+15 && ReadinessScore(ctx) >= threshold+10:
		return "siap"
	case ctx.Score >= threshold:
		return "hampir siap"
	default:
		return "perlu perbaikan"
	}
}

// BaselineAnalysis menghasilkan analisis lengkap tanpa memanggil model bahasa.
//
// Dipakai sebagai isi default sekaligus jaring pengaman: bila model bahasa tidak
// tersedia atau balasannya tidak bisa diurai, peserta tetap mendapatkan ulasan
// yang spesifik dan bisa ditindaklanjuti.
func BaselineAnalysis(ctx AttemptContext) AnalysisResult {
	return AnalysisResult{
		Headline:      headlineFor(ctx),
		Summary:       summaryFor(ctx),
		Strengths:     strengthsFor(ctx),
		Weaknesses:    weaknessesFor(ctx),
		FocusAreas:    focusAreasFor(ctx),
		StudyPlan:     studyPlanFor(ctx),
		QuestionNotes: questionNotesFor(ctx, 6),
	}
}

// Suggestions menyusun pertanyaan lanjutan yang relevan dengan hasil peserta.
//
// Dipakai UI sebagai tombol cepat agar peserta tidak bingung harus bertanya apa.
func Suggestions(ctx AttemptContext, focus *QuestionContext) []string {
	out := make([]string, 0, 4)

	if focus != nil {
		out = append(out,
			"Jelaskan langkah pengerjaan soal ini dari awal",
			"Kenapa opsi yang aku pilih itu menggoda?",
		)
	}

	if wrong := ctx.WrongQuestions(); len(wrong) > 0 {
		out = append(out, "Soal mana yang paling mendesak aku perbaiki?")
	}

	if weak := ctx.WeakSubtests(1); len(weak) > 0 {
		out = append(out, "Susun rencana belajar untuk "+weak[0].Name)
	} else if weak := ctx.WeakestSubtests(1); len(weak) > 0 {
		out = append(out, "Bagaimana memperlebar margin di "+weak[0].Name+"?")
	}

	out = append(out, "Bagaimana strategi membagi waktu antar subtes?")

	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func clampScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
