package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StubProvider adalah mentor heuristik lokal.
//
// Tujuannya bukan meniru model besar, melainkan memastikan fitur ulasan hasil
// tryout tetap berfungsi penuh tanpa jaringan: dipakai sebagai default
// pengembangan, pada pengujian otomatis, dan sebagai fallback saat penyedia
// utama bermasalah (kuota habis, model belum diaktifkan, gangguan jaringan).
//
// Balasannya dibangun dari fakta nyata hasil tryout (skor per subtes, soal yang
// salah, kunci jawaban, pembahasan) sehingga tetap spesifik dan bisa
// ditindaklanjuti — bukan teks contoh.
type StubProvider struct {
	model string
}

// NewStub membentuk mentor heuristik lokal.
func NewStub() *StubProvider {
	return &StubProvider{model: "minsmart-heuristik-v1"}
}

// Name mengembalikan nama penyedia.
func (p *StubProvider) Name() string { return "stub" }

// Model mengembalikan id model.
func (p *StubProvider) Model() string { return p.model }

// Chat membalas secara lengkap.
func (p *StubProvider) Chat(_ context.Context, req Request) (*Response, error) {
	started := time.Now()
	content := p.render(req)
	return &Response{
		Content:  content,
		Usage:    estimateUsage(req, content),
		Provider: p.Name(),
		Model:    p.model,
		Latency:  time.Since(started),
	}, nil
}

// Stream membalas bertahap agar klien tetap bisa menguji alur streaming.
func (p *StubProvider) Stream(_ context.Context, req Request) (<-chan Chunk, error) {
	content := p.render(req)
	usage := estimateUsage(req, content)

	chunks := make(chan Chunk, 8)
	go func() {
		defer close(chunks)
		// Dipecah per baris supaya efek pengetikan terasa alami di UI.
		for _, line := range splitForStreaming(content) {
			chunks <- Chunk{Delta: line}
		}
		chunks <- Chunk{Done: true, Usage: usage}
	}()
	return chunks, nil
}

// render memilih bentuk balasan: JSON terstruktur untuk analisis, teks markdown
// untuk percakapan mentor, dan ringkasan analitik untuk asisten HR.
func (p *StubProvider) render(req Request) string {
	// Blok data analitik HR memakai penyaji tersendiri karena pertanyaannya
	// seputar kohort, bukan hasil satu peserta.
	if hrCtx, ok := ParseHRContextFromMessages(req.Messages); ok {
		return p.renderHRChat(hrCtx, req.Messages)
	}

	ctx, ok := ParseContextFromMessages(req.Messages)
	if !ok {
		return "Aku belum menerima konteks hasil tryout. Buka halaman hasil tryout lalu mulai ulasan dari sana, ya."
	}

	if req.JSONMode {
		return p.renderAnalysis(ctx)
	}
	return p.renderChat(ctx, req.Messages)
}

/* --------------------------------- analisis -------------------------------- */

// renderAnalysis membalas JSON analisis.
//
// Isinya memakai BaselineAnalysis sehingga mentor heuristik dan model bahasa
// sungguhan berbicara dari fakta yang sama.
func (p *StubProvider) renderAnalysis(ctx AttemptContext) string {
	encoded, err := json.Marshal(BaselineAnalysis(ctx))
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

/* ---------------------------------- chat ---------------------------------- */

func (p *StubProvider) renderChat(ctx AttemptContext, messages []Message) string {
	question, hasQuestion := focusedQuestion(ctx, messages)
	userQuestion := lastUserMessage(messages)

	var builder strings.Builder
	if hasQuestion {
		builder.WriteString(questionCoaching(ctx, question))
	} else {
		builder.WriteString(overallCoaching(ctx))
	}

	// Pesan pembuka otomatis (mis. tombol "jelaskan soal ini") tidak perlu
	// dijawab sebagai pertanyaan tambahan.
	if userQuestion != "" && !lastMessageIsAuto(messages) {
		builder.WriteString("\n\n**Menjawab pertanyaanmu:** ")
		builder.WriteString(answerIntent(ctx, question, hasQuestion, userQuestion))
	}

	return strings.TrimSpace(builder.String())
}

// questionCoaching menyusun penjelasan satu soal secara runtut.
func questionCoaching(ctx AttemptContext, question QuestionContext) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "### Soal %d · %s\n\n", question.Number, question.Subtest)
	fmt.Fprintf(&builder, "> %s\n\n", firstLine(question.Text))
	fmt.Fprintf(&builder, "**Jawaban tepat:** %s\n\n", optionText(question, question.CorrectIndex))

	switch {
	case question.IsUnanswered:
		builder.WriteString("**Jawabanmu:** belum dijawab.\n\n")
	case question.SelectedIndex != nil:
		fmt.Fprintf(&builder, "**Jawabanmu:** %s\n\n", optionText(question, *question.SelectedIndex))
	}

	builder.WriteString("**Kenapa demikian**\n\n")
	builder.WriteString(strings.TrimSpace(question.Explanation))
	builder.WriteString("\n\n**Yang perlu kamu ingat**\n\n")
	builder.WriteString(coachingFor(question))
	builder.WriteString("\n\n")

	if question.IsUnanswered {
		builder.WriteString("**Catatan strategi:** soal ini kosong, bukan salah. ")
		builder.WriteString("Coba sisakan 1 menit di akhir tiap subtes untuk menyisir soal yang belum dijawab, ")
		builder.WriteString("karena satu soal kosong bernilai sama dengan satu soal salah.\n")
	} else {
		builder.WriteString("**Langkah latihan:** kerjakan ulang soal ini tanpa melihat pembahasan, ")
		builder.WriteString("lalu tulis satu kalimat tentang jebakan yang membuatmu memilih opsi lain.\n")
	}

	return builder.String()
}

// overallCoaching menyusun rangkuman bila peserta belum menunjuk satu soal.
func overallCoaching(ctx AttemptContext) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "### Ringkasan hasil · %s\n\n", ctx.Title)
	fmt.Fprintf(&builder, "Skormu **%d** (%d benar dari %d soal, ambang latihan %d) — status *%s*.\n\n",
		ctx.Score, ctx.Correct, ctx.Total, ctx.PassingThreshold, ctx.Status)

	builder.WriteString("**Peta skor per subtes**\n\n")
	for _, subtest := range ctx.Subtests {
		fmt.Fprintf(&builder, "- **%s** — %d (%d/%d benar)\n", subtest.Name, subtest.Score, subtest.Correct, subtest.Total)
	}
	builder.WriteString("\n")

	if weak := ctx.WeakSubtests(1); len(weak) > 0 {
		fmt.Fprintf(&builder, "**Prioritas utama:** %s (skor %d) — %s\n\n",
			weak[0].Name, weak[0].Score, adviceForSubtest(weak[0].Name))
	} else {
		builder.WriteString("Semua subtes sudah di atas ambang latihan — pertahankan ritme belajarmu.\n\n")
	}

	fmt.Fprintf(&builder, "Ada **%d soal** yang belum benar. ", len(ctx.WrongQuestions()))
	builder.WriteString("Sebutkan nomor soalnya (misalnya *\"jelaskan soal 3\"*) dan aku akan menguraikan langkah pengerjaannya satu per satu.\n")

	return builder.String()
}

// answerIntent menjawab pertanyaan bebas peserta dengan konteks yang tersedia.
func answerIntent(ctx AttemptContext, question QuestionContext, hasQuestion bool, userMessage string) string {
	intent := strings.ToLower(userMessage)

	switch {
	case containsAny(intent, "tips", "strategi", "waktu", "manajemen", "cepat"):
		return timeStrategy(ctx)
	case containsAny(intent, "materi", "belajar", "latihan", "fokus", "prioritas", "mulai dari mana"):
		return studyPriority(ctx)
	case containsAny(intent, "berapa", "skor", "nilai", "ambang", "lulus", "peringkat"):
		return scoreRecap(ctx)
	case containsAny(intent, "kenapa", "mengapa", "salah"):
		if hasQuestion {
			return whyWrong(question)
		}
		// Tanpa fokus eksplisit, jelaskan soal yang paling layak dibahas lebih
		// dulu daripada membiarkan pertanyaan umum tak terjawab.
		if wrong := ctx.WrongQuestions(); len(wrong) > 0 {
			return fmt.Sprintf("Ini soal yang paling awal belum kamu benarkan.\n\n%s", whyWrong(wrong[0]))
		}
		return "Tidak ada jawaban yang salah pada sesi ini — semua soal sudah kamu benarkan."
	case containsAny(intent, "konsep", "teori", "rumus", "cara") && hasQuestion:
		return conceptFor(question)
	default:
		if hasQuestion {
			return fmt.Sprintf("Kunci soal %d: %s", question.Number, coachingFor(question))
		}
		return studyPriority(ctx)
	}
}
