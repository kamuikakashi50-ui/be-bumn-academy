package ai

import (
	"fmt"
	"strings"
)

// headlineFor menyusun judul singkat analisis.
func headlineFor(ctx AttemptContext) string {
	if weak := ctx.WeakSubtests(1); len(weak) > 0 {
		return fmt.Sprintf("Skor %d — %s menahan skormu", ctx.Score, weak[0].Name)
	}
	if ctx.Score >= ctx.ThresholdOrDefault()+15 {
		return fmt.Sprintf("Skor %d — konsisten di atas ambang", ctx.Score)
	}
	return fmt.Sprintf("Skor %d — tinggal merapikan ketelitian", ctx.Score)
}

// summaryFor menyusun rangkuman naratif hasil tryout.
func summaryFor(ctx AttemptContext) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Pada %s kamu mendapat %d dari %d soal (%d%%), dengan ambang latihan %d — status %s. ",
		ctx.Title, ctx.Correct, ctx.Total, ctx.Score, ctx.ThresholdOrDefault(), ctx.Status)

	if strong := ctx.StrongSubtests(1); len(strong) > 0 {
		if strong[0].Score >= ctx.ThresholdOrDefault() {
			fmt.Fprintf(&builder, "Kekuatan terbesarmu ada di %s (skor %d). ", strong[0].Name, strong[0].Score)
		} else {
			fmt.Fprintf(&builder, "Subtes terbaikmu %s baru mencapai skor %d, jadi belum ada subtes yang benar-benar aman. ",
				strong[0].Name, strong[0].Score)
		}
	}

	if weak := ctx.WeakSubtests(2); len(weak) > 0 {
		fmt.Fprintf(&builder, "Yang paling menahan skor total adalah %s (skor %d)", weak[0].Name, weak[0].Score)
		if len(weak) > 1 {
			fmt.Fprintf(&builder, ", disusul %s (skor %d)", weak[1].Name, weak[1].Score)
		}
		builder.WriteString(". ")
	} else {
		builder.WriteString("Semua subtes sudah berada di atas ambang latihan. ")
	}

	if unanswered := countUnanswered(ctx); unanswered > 0 {
		fmt.Fprintf(&builder, "Ada %d soal yang tidak kamu jawab — ini biasanya soal waktu, bukan soal pemahaman. ", unanswered)
	}
	if ctx.DurationSeconds > 0 {
		fmt.Fprintf(&builder, "Kamu memakai %s untuk menyelesaikan sesi ini.", humanDuration(ctx.DurationSeconds))
	}

	return strings.TrimSpace(builder.String())
}

// strengthsFor mendaftar subtes yang sudah kuat.
func strengthsFor(ctx AttemptContext) []Note {
	threshold := ctx.ThresholdOrDefault()
	items := make([]Note, 0, 3)

	for _, subtest := range ctx.StrongSubtests(3) {
		if subtest.Score >= 90 {
			items = append(items, Note{
				Title:   subtest.Name,
				Subtest: subtest.Name,
				Detail: fmt.Sprintf(
					"Skor %d (%d dari %d soal benar) — sangat kuat. Cukup rawat dengan satu sesi latihan ringan sebelum tryout berikutnya.",
					subtest.Score, subtest.Correct, subtest.Total,
				),
			})
			continue
		}
		if subtest.Score >= threshold {
			items = append(items, Note{
				Title:   subtest.Name,
				Subtest: subtest.Name,
				Detail: fmt.Sprintf(
					"Skor %d (%d dari %d soal benar) sudah di atas ambang. Pertahankan dengan latihan singkat berkala agar tidak turun di sesi berikutnya.",
					subtest.Score, subtest.Correct, subtest.Total,
				),
			})
			continue
		}
		// Semua subtes di bawah ambang: sebutkan yang relatif paling baik.
		items = append(items, Note{
			Title:   fmt.Sprintf("%s (relatif paling baik)", subtest.Name),
			Subtest: subtest.Name,
			Detail: fmt.Sprintf(
				"Skor %d (%d dari %d soal benar). Ini modal utamamu saat ini — naikkan sedikit lagi menjadi pondasi sebelum memperbaiki subtes lain.",
				subtest.Score, subtest.Correct, subtest.Total,
			),
		})
	}
	return items
}

// weaknessesFor mendaftar subtes yang masih di bawah ambang batas.
func weaknessesFor(ctx AttemptContext) []Note {
	items := make([]Note, 0, 3)
	for _, subtest := range ctx.WeakSubtests(3) {
		items = append(items, Note{
			Title:   fmt.Sprintf("%s — skor %d", subtest.Name, subtest.Score),
			Subtest: subtest.Name,
			Detail: fmt.Sprintf("%d dari %d soal benar, %d poin di bawah ambang. %s",
				subtest.Correct, subtest.Total, ctx.ThresholdOrDefault()-subtest.Score, adviceForSubtest(subtest.Name)),
		})
	}
	return items
}

// focusAreasFor menyusun area fokus yang bisa langsung dikerjakan.
func focusAreasFor(ctx AttemptContext) []Note {
	targets := ctx.WeakSubtests(3)
	if len(targets) == 0 {
		// Semua subtes sudah lulus ambang: fokuskan ke dua skor terendah agar
		// marginnya makin lebar.
		targets = ctx.WeakestSubtests(2)
	}

	items := make([]Note, 0, len(targets))
	for index, subtest := range targets {
		items = append(items, Note{
			Title:   fmt.Sprintf("Prioritas %d: %s", index+1, subtest.Name),
			Subtest: subtest.Name,
			Detail:  adviceForSubtest(subtest.Name),
		})
	}
	return items
}

func countUnanswered(ctx AttemptContext) int {
	total := 0
	for _, question := range ctx.Questions {
		if question.IsUnanswered {
			total++
		}
	}
	return total
}

// studyPlanFor menyusun rencana belajar bertahap.
func studyPlanFor(ctx AttemptContext) []Note {
	weak := ctx.WeakestSubtests(2)
	plan := make([]Note, 0, 5)

	step := 1
	for _, subtest := range weak {
		plan = append(plan, Note{
			Title:   fmt.Sprintf("Langkah %d — perkuat %s", step, subtest.Name),
			Detail:  fmt.Sprintf("Kerjakan 15–20 soal %s, lalu tulis alasan setiap jawaban yang salah. Ulangi sampai dua sesi berturut-turut skornya di atas %d.", subtest.Name, ctx.PassingThreshold),
			Subtest: subtest.Name,
		})
		step++
	}

	if unanswered := countUnanswered(ctx); unanswered > 0 {
		plan = append(plan, Note{
			Title:  fmt.Sprintf("Langkah %d — latih manajemen waktu", step),
			Detail: fmt.Sprintf("Ada %d soal kosong pada sesi ini. Berlatih dengan timer dan sisakan 1 menit terakhir tiap subtes untuk menyisir soal yang belum dijawab.", unanswered),
		})
		step++
	}

	plan = append(plan, Note{
		Title:  fmt.Sprintf("Langkah %d — ulangi tryout yang sama", step),
		Detail: fmt.Sprintf("Ulangi %s setelah 2–3 hari belajar. Targetkan kenaikan minimal 5 poin, terutama dari subtes yang sekarang terlemah.", ctx.Title),
	})
	step++

	plan = append(plan, Note{
		Title:  fmt.Sprintf("Langkah %d — jaga ritme", step),
		Detail: "Satu sesi tryout per minggu cukup untuk memantau tren. Yang menentukan biasanya konsistensi harian, bukan durasi belajar sekali waktu.",
	})

	return plan
}

// questionNotesFor menyusun catatan singkat untuk soal-soal yang belum benar.
func questionNotesFor(ctx AttemptContext, limit int) []Note {
	// Attempt lama (mis. data demo) bisa tidak punya rincian per soal. Katakan
	// apa adanya daripada mengklaim semua soal benar.
	if len(ctx.Questions) == 0 {
		detail := "Rincian jawaban per soal tidak tersedia untuk sesi ini, jadi pembahasan per nomor belum bisa disusun."
		if ctx.Correct < ctx.Total {
			detail = fmt.Sprintf(
				"Rincian jawaban per soal tidak tersimpan untuk sesi ini (%d dari %d soal benar), jadi pembahasan per nomor belum bisa disusun. Kerjakan sesi baru agar setiap soal bisa dibedah satu per satu.",
				ctx.Correct, ctx.Total,
			)
		}
		return []Note{{Title: "Rincian soal tidak tersedia", Detail: detail}}
	}

	wrong := ctx.WrongQuestions()
	if len(wrong) == 0 {
		return []Note{{
			Title:  "Semua soal benar",
			Detail: "Tidak ada soal yang perlu dibedah ulang pada sesi ini. Coba paket tryout lain untuk menguji konsistensi di materi yang berbeda.",
		}}
	}
	if limit > 0 && len(wrong) > limit {
		wrong = wrong[:limit]
	}

	items := make([]Note, 0, len(wrong))
	for _, question := range wrong {
		detail := strings.TrimSpace(question.Explanation)
		if question.IsUnanswered {
			detail = "Belum dijawab. " + detail
		} else if question.SelectedIndex != nil {
			detail = fmt.Sprintf("Kamu memilih %q, kunci jawabannya %q. %s",
				optionText(question, *question.SelectedIndex),
				optionText(question, question.CorrectIndex),
				detail,
			)
		}
		items = append(items, Note{
			Title:      fmt.Sprintf("Soal %d · %s", question.Number, question.Subtest),
			Detail:     detail,
			Subtest:    question.Subtest,
			QuestionID: question.ID,
		})
	}
	return items
}
