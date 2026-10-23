package ai

import (
	"fmt"
	"strings"
)

func whyWrong(question QuestionContext) string {
	if question.IsUnanswered {
		return fmt.Sprintf(
			"Soal %d tidak kamu jawab, jadi tidak ada opsi yang bisa dianalisis. Yang perlu diperhatikan alokasi waktumu: soal ini %s.",
			question.Number, difficultyNote(question),
		)
	}
	if question.SelectedIndex != nil {
		return fmt.Sprintf(
			"Kamu memilih **%s**, sedangkan kunci jawabannya **%s**. %s",
			optionText(question, *question.SelectedIndex),
			optionText(question, question.CorrectIndex),
			strings.TrimSpace(question.Explanation),
		)
	}
	return strings.TrimSpace(question.Explanation)
}

func conceptFor(question QuestionContext) string {
	return fmt.Sprintf("%s\n\n%s", coachingFor(question), conceptForBank(question))
}

func scoreRecap(ctx AttemptContext) string {
	gap := ctx.Score - ctx.PassingThreshold
	switch {
	case gap > 15:
		return fmt.Sprintf(
			"Skormu %d, yaitu %d poin di atas ambang latihan %d. Fokus berikutnya bukan menaikkan skor, "+
				"tetapi menjaga konsistensi dan mempercepat waktu pengerjaan.",
			ctx.Score, gap, ctx.PassingThreshold,
		)
	case gap > 0:
		return fmt.Sprintf(
			"Skormu %d, hanya %d poin di atas ambang %d. Marginnya tipis: satu atau dua soal yang salah "+
				"karena kurang teliti sudah cukup membuatmu turun di bawah ambang.",
			ctx.Score, gap, ctx.PassingThreshold,
		)
	default:
		return fmt.Sprintf(
			"Skormu %d, yaitu %d poin di bawah ambang %d. Jarak ini realistis dikejar: biasanya cukup "+
				"memperbaiki dua subtes terlemah saja.",
			ctx.Score, -gap, ctx.PassingThreshold,
		)
	}
}

func timeStrategy(ctx AttemptContext) string {
	if ctx.DurationSeconds <= 0 {
		return "Biasakan memakai timer saat berlatih: patok batas waktu per subtes, " +
			"lalu sisakan sekitar 10% waktu di akhir untuk menyisir soal kosong."
	}
	perQuestion := ctx.DurationSeconds / maxInt(ctx.Total, 1)
	return fmt.Sprintf(
		"Pada sesi ini kamu memakai %s untuk %d soal — sekitar %d detik per soal. "+
			"Target latihan berikutnya: tekan ke %d detik per soal, lalu sisakan 1 menit terakhir "+
			"tiap subtes untuk menyisir soal kosong.",
		humanDuration(ctx.DurationSeconds), ctx.Total, perQuestion, maxInt(perQuestion*3/4, 20),
	)
}

func studyPriority(ctx AttemptContext) string {
	weak := ctx.WeakSubtests(3)
	if len(weak) == 0 {
		// Semua subtes sudah lulus ambang: fokus ke margin terketat.
		weak = ctx.WeakestSubtests(2)
	}
	if len(weak) == 0 {
		return "Belum ada data subtes yang bisa dianalisis."
	}

	var builder strings.Builder
	builder.WriteString("Urutan prioritas belajarmu berdasarkan sesi ini:\n\n")
	for index, subtest := range weak {
		fmt.Fprintf(&builder, "%d. **%s** (skor %d) — %s\n", index+1, subtest.Name, subtest.Score, adviceForSubtest(subtest.Name))
	}
	return strings.TrimSpace(builder.String())
}

/* --------------------------------- fokus soal ------------------------------ */

/* ------------------------------ pembacaan pesan ---------------------------- */

// extractUserQuestion mengisolasi pertanyaan asli peserta dari sebuah pesan.
//
// Pesan yang dikirim service berbentuk:
//
//	[Peserta sedang membahas soal nomor N (Subtes). [[SOAL:id]]]
//	Pertanyaan peserta: <pertanyaan asli>
//
//	<KONTEKS_HASIL_TRYOUT> … </KONTEKS_HASIL_TRYOUT>
//
// Bagian konteks dan penanda internal dibuang supaya teks yang tersisa benar-benar
// berasal dari peserta.
func extractUserQuestion(content string) string {
	text := content
	if start := strings.Index(text, contextOpen); start >= 0 {
		text = text[:start]
	}

	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Peserta sedang membahas") {
			continue
		}
		kept = append(kept, line)
	}
	text = strings.Join(kept, "\n")

	text = strings.ReplaceAll(text, "Pertanyaan peserta:", " ")
	if id, ok := extractMarker(text, "[[SOAL:", "]]"); ok {
		text = strings.ReplaceAll(text, "[[SOAL:"+id+"]]", " ")
	}
	text = strings.ReplaceAll(text, "[[AUTO]]", " ")

	return strings.TrimSpace(text)
}

// focusedQuestion mencari soal yang sedang dibahas pada giliran percakapan ini.
//
// Hanya pesan peserta TERAKHIR yang diperiksa: giliran sebelumnya bisa membahas
// soal lain (mis. "Jelaskan soal nomor 1") dan tidak boleh mengubah fokus
// pertanyaan baru. Fokus ditentukan oleh penanda `[[SOAL:<id>]]` (dipasang
// server) atau penyebutan eksplisit seperti "soal 3" pada pertanyaan peserta.
func focusedQuestion(ctx AttemptContext, messages []Message) (QuestionContext, bool) {
	raw, ok := lastUserContent(messages)
	if !ok {
		return QuestionContext{}, false
	}

	if id, found := extractMarker(raw, "[[SOAL:", "]]"); found {
		if question, exists := ctx.QuestionByID(id); exists {
			return question, true
		}
	}

	question := extractUserQuestion(raw)
	if number, found := extractQuestionNumber(question); found {
		for _, candidate := range ctx.Questions {
			if candidate.Number == number {
				return candidate, true
			}
		}
	}
	return QuestionContext{}, false
}

// lastUserContent mengambil isi pesan peserta terakhir.
func lastUserContent(messages []Message) (string, bool) {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == RoleUser {
			return messages[index].Content, true
		}
	}
	return "", false
}

// extractMarker mengambil nilai di antara prefix dan suffix.
func extractMarker(content, prefix, suffix string) (string, bool) {
	start := strings.Index(content, prefix)
	if start < 0 {
		return "", false
	}
	rest := content[start+len(prefix):]
	end := strings.Index(rest, suffix)
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(rest[:end]), true
}

// extractQuestionNumber mencari pola "soal 3" / "nomor 3" pada teks.
func extractQuestionNumber(content string) (int, bool) {
	lower := strings.ToLower(content)
	for _, keyword := range []string{"soal", "nomor", "no."} {
		idx := strings.Index(lower, keyword)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(lower[idx+len(keyword):])
		number := 0
		digits := 0
		for _, char := range rest {
			if char < '0' || char > '9' {
				break
			}
			number = number*10 + int(char-'0')
			digits++
			if digits > 3 {
				break
			}
		}
		if digits > 0 && number > 0 {
			return number, true
		}
	}
	return 0, false
}

// lastUserMessage mengambil pertanyaan terakhir dari peserta.
func lastUserMessage(messages []Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role != RoleUser {
			continue
		}
		question := extractUserQuestion(messages[index].Content)
		if question != "" {
			return question
		}
	}
	return ""
}

// isAutoPrompt menandai pesan pembuka otomatis dari sistem (bukan pertanyaan
// asli peserta) sehingga tidak perlu dijawab sebagai pertanyaan.
func isAutoPrompt(content string) bool {
	return strings.Contains(content, "[[AUTO]]")
}

// lastMessageIsAuto memeriksa apakah pesan peserta terakhir adalah pesan
// pembuka otomatis dari sistem.
func lastMessageIsAuto(messages []Message) bool {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role != RoleUser {
			continue
		}
		return isAutoPrompt(messages[index].Content)
	}
	return false
}
