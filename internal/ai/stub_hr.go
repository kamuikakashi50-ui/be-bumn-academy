package ai

import (
	"fmt"
	"strings"
)

// renderHRChat menyusun balasan asisten HR dari fakta agregat kohort.
//
// Tujuannya sama dengan mentor heuristik: fitur tetap berguna tanpa jaringan,
// dan angkanya identik dengan yang dibaca model bahasa sungguhan.
func (p *StubProvider) renderHRChat(ctx HRContext, messages []Message) string {
	question := hrQuestion(messages)

	switch {
	// Maksud yang paling spesifik diperiksa lebih dulu agar tidak tertutup oleh
	// kata kunci umum (mis. "siapa yang perlu pendampingan" tidak boleh jatuh ke
	// cabang kesiapan hanya karena memuat "siap").
	case containsAny(question, "pendampingan", "risiko", "berisiko", "intervensi", "dibantu", "coaching"):
		return hrAtRisk(ctx)
	case containsAny(question, "berprestasi", "prestasi", "terbaik", "top", "unggul", "apresiasi"):
		return hrTopLearners(ctx)
	case containsAny(question, "program", "segmen", "unit", "divisi", "kelompok"):
		return hrProgramBreakdown(ctx)
	case containsAny(question, "kompetensi", "kategori", "subtes", "lemah", "kuat", "profil"):
		return hrSkillProfile(ctx)
	case containsAny(question, "partisipasi", "aktif", "belum", "tidak pernah", "pasif", "hadir"):
		return hrParticipation(ctx)
	case containsAny(question, "kesiapan", "siapkan", "persiapan", "ambang", "lulus", "kelulusan", "di atas"):
		return hrReadiness(ctx)
	case containsAny(question, "materi", "kelas", "progres", "kelengkapan", "belajar", "modul"):
		return hrLearningCompletion(ctx)
	case containsAny(question, "tren", "bulan", "aktivitas", "perkembangan", "waktu"):
		return hrMonthlyTrend(ctx)
	case containsAny(question, "sebaran", "distribusi", "rentang", "histogram"):
		return hrScoreDistribution(ctx)
	default:
		return hrSummary(ctx)
	}
}

// hrQuestion mengambil pertanyaan tim HR tanpa blok data di belakangnya.
//
// Blok data memuat kata seperti "belum" dan "aktif" pada catatan peserta; bila
// ikut terbaca, deteksi maksud pertanyaan akan salah arah.
func hrQuestion(messages []Message) string {
	question := lastUserMessage(messages)
	if start := strings.Index(question, hrContextOpen); start >= 0 {
		question = question[:start]
	}
	return strings.ToLower(strings.TrimSpace(question))
}

// hrSummary merangkum kondisi kohort sebagai pembuka percakapan.
func hrSummary(ctx HRContext) string {
	var builder strings.Builder
	builder.WriteString("### Ringkasan kohort\n\n")
	fmt.Fprintf(&builder,
		"**%d peserta terdaftar**, **%d aktif berlatih** dalam %d hari terakhir (%.1f%%), dan **%d belum berlatih**.\n\n",
		ctx.TotalPeserta, ctx.ActivePeserta, ctx.ActiveWindowDays, ctx.ParticipationRate, ctx.InactivePeserta)

	fmt.Fprintf(&builder,
		"Dari **%d percobaan tryout** pada %d paket, rata-rata skor **%.1f** dengan skor tertinggi **%d**. "+
			"Sebanyak **%.1f%% percobaan** berada di atas ambang latihan %d.\n\n",
		ctx.TotalAttempts, ctx.DistinctTryouts, ctx.AverageScore, ctx.TopScore, ctx.PassingRate, ctx.PassingThreshold)

	if ctx.TotalLessons > 0 {
		fmt.Fprintf(&builder,
			"Kelengkapan belajar: **%d dari %d materi** sudah diselesaikan (%d peserta membuka materi), "+
				"rata-rata progres **%.1f%%**.\n\n",
			ctx.CompletedLessons, ctx.TotalLessons, ctx.ActiveLearners, ctx.AverageProgress)
	}

	if weak := ctx.WeakestSkills(2); len(weak) > 0 {
		builder.WriteString("**Perlu perhatian**\n\n")
		for _, skill := range weak {
			fmt.Fprintf(&builder, "- **%s** — rata-rata %.1f dari %d percobaan\n", skill.Name, skill.AverageScore, skill.Attempts)
		}
		builder.WriteString("\n")
	}

	builder.WriteString("**Langkah berikutnya**\n\n")
	builder.WriteString(hrActionItems(ctx))
	return strings.TrimSpace(builder.String())
}

// hrParticipation menjawab pertanyaan seputar keikutsertaan peserta.
func hrParticipation(ctx HRContext) string {
	var builder strings.Builder
	builder.WriteString("### Partisipasi latihan\n\n")
	fmt.Fprintf(&builder,
		"- Peserta terdaftar: **%d**\n- Aktif (%d hari terakhir): **%d** (%.1f%%)\n- Belum berlatih: **%d**\n",
		ctx.TotalPeserta, ctx.ActiveWindowDays, ctx.ActivePeserta, ctx.ParticipationRate, ctx.InactivePeserta)
	fmt.Fprintf(&builder, "- Total percobaan tryout: **%d** pada %d paket\n\n", ctx.TotalAttempts, ctx.DistinctTryouts)

	if len(ctx.AtRiskLearners) == 0 {
		builder.WriteString("Tidak ada peserta pada daftar pendampingan saat ini.\n")
		return strings.TrimSpace(builder.String())
	}

	builder.WriteString("**Peserta yang perlu dihubungi**\n\n")
	for _, learner := range ctx.AtRiskLearners {
		fmt.Fprintf(&builder, "- **%s** (%s) — %s\n", learner.Name, learner.Program, hrLearnerActivity(learner))
	}
	builder.WriteString("\nSaran: kirim pengingat berjadwal untuk peserta tanpa aktivitas, lalu tawarkan sesi " +
		"latihan bersama agar hambatan awalnya teridentifikasi.\n")
	return strings.TrimSpace(builder.String())
}

// hrReadiness menjawab pertanyaan seputar kesiapan dan ambang skor.
func hrReadiness(ctx HRContext) string {
	var builder strings.Builder
	builder.WriteString("### Kesiapan peserta\n\n")
	fmt.Fprintf(&builder,
		"Rata-rata skor **%.1f** dengan ambang latihan **%d**; **%.1f%% percobaan** di atas ambang. Skor tertinggi **%d**.\n\n",
		ctx.AverageScore, ctx.PassingThreshold, ctx.PassingRate, ctx.TopScore)

	if len(ctx.ScoreDistribution) > 0 {
		builder.WriteString("**Sebaran skor**\n\n")
		for _, bucket := range ctx.ScoreDistribution {
			fmt.Fprintf(&builder, "- %s: **%d percobaan**\n", bucket.Label, bucket.Count)
		}
		builder.WriteString("\n")
	}

	if below := ctx.SkillsBelowThreshold(); len(below) > 0 {
		builder.WriteString("**Kategori di bawah ambang**\n\n")
		for _, skill := range below {
			fmt.Fprintf(&builder, "- **%s** — %.1f (%d percobaan)\n", skill.Name, skill.AverageScore, skill.Attempts)
		}
		builder.WriteString("\n")
	} else {
		builder.WriteString("Semua kategori bernilai sudah berada di atas ambang latihan.\n\n")
	}

	builder.WriteString("Catatan: ambang ini adalah ambang latihan internal, bukan ambang kelulusan seleksi resmi.\n")
	return strings.TrimSpace(builder.String())
}

// hrLearningCompletion menjawab pertanyaan seputar kelengkapan materi.
func hrLearningCompletion(ctx HRContext) string {
	if ctx.TotalLessons == 0 {
		return "Belum ada materi aktif yang bisa diukur kelengkapan belajarnya."
	}

	var builder strings.Builder
	builder.WriteString("### Kelengkapan belajar\n\n")
	fmt.Fprintf(&builder,
		"- Materi aktif: **%d**\n- Materi selesai: **%d**\n- Tingkat penyelesaian: **%.1f%%**\n",
		ctx.TotalLessons, ctx.CompletedLessons, ctx.CompletionRate)
	fmt.Fprintf(&builder,
		"- Peserta yang membuka materi: **%d**\n- Rata-rata progres per peserta: **%.1f%%**\n\n",
		ctx.ActiveLearners, ctx.AverageProgress)

	if ctx.InactivePeserta > 0 {
		fmt.Fprintf(&builder,
			"Ada **%d peserta** tanpa aktivitas belajar sama sekali — mereka belum tersentuh materi maupun tryout. "+
				"Prioritaskan penjangkauan ke kelompok ini sebelum menambah materi baru.\n",
			ctx.InactivePeserta)
	} else {
		builder.WriteString("Seluruh peserta sudah menunjukkan aktivitas belajar. Fokus berikutnya menjaga ritme " +
			"dan mendorong materi yang masih tertahan di tengah jalan.\n")
	}
	return strings.TrimSpace(builder.String())
}

// hrTopLearners menyoroti peserta dengan capaian terbaik.
func hrTopLearners(ctx HRContext) string {
	if len(ctx.TopLearners) == 0 {
		return "Belum ada peserta dengan capaian tryout yang bisa dijadikan contoh."
	}

	var builder strings.Builder
	builder.WriteString("### Peserta berprestasi\n\n")
	for index, learner := range ctx.TopLearners {
		fmt.Fprintf(&builder, "%d. **%s** (%s) — skor terbaik **%d**, %d percobaan, %s\n",
			index+1, learner.Name, learner.Program, learner.Score, learner.AttemptCount, hrLearnerActivity(learner))
	}
	builder.WriteString("\nSaran: jadikan mereka mentor sebaya atau narasumber sesi latihan bersama, lalu minta " +
		"pola belajarnya dibagikan ke peserta di daftar pendampingan.\n")
	return strings.TrimSpace(builder.String())
}

// hrAtRisk menyusun prioritas pendampingan.
func hrAtRisk(ctx HRContext) string {
	var builder strings.Builder
	builder.WriteString("### Peserta yang perlu pendampingan\n\n")

	if len(ctx.AtRiskLearners) == 0 {
		builder.WriteString("Tidak ada peserta pada daftar pendampingan saat ini. ")
		builder.WriteString("Pertahankan pemantauan mingguan agar penurunan aktivitas terdeteksi lebih awal.\n")
		return strings.TrimSpace(builder.String())
	}

	for index, learner := range ctx.AtRiskLearners {
		fmt.Fprintf(&builder, "%d. **%s** (%s) — %s\n", index+1, learner.Name, learner.Program, hrLearnerActivity(learner))
	}

	builder.WriteString("\n**Rencana intervensi**\n\n")
	builder.WriteString("1. Hubungi peserta tanpa aktivitas lebih dulu — hambatan terbesar biasanya ada di awal.\n")
	if ctx.InactivePeserta > 0 {
		fmt.Fprintf(&builder, "2. Tawarkan sesi pendampingan kelompok untuk **%d peserta** yang belum berlatih sama sekali.\n", ctx.InactivePeserta)
	}
	if weak := ctx.WeakestSkills(1); len(weak) > 0 {
		fmt.Fprintf(&builder, "3. Fokuskan latihan tambahan pada **%s** (rata-rata %.1f).\n", weak[0].Name, weak[0].AverageScore)
	}
	return strings.TrimSpace(builder.String())
}

// hrProgramBreakdown membandingkan capaian antar segmen program.
func hrProgramBreakdown(ctx HRContext) string {
	if len(ctx.ProgramBreakdown) == 0 {
		return "Belum ada data peserta per program yang bisa dibandingkan."
	}

	var builder strings.Builder
	builder.WriteString("### Capaian per program\n\n")
	builder.WriteString("| Program | Peserta | Percobaan | Rata-rata skor |\n| --- | --- | --- | --- |\n")
	for _, program := range ctx.ProgramBreakdown {
		fmt.Fprintf(&builder, "| %s | %d | %d | %.1f |\n", program.Program, program.Peserta, program.Attempts, program.AverageScore)
	}
	builder.WriteString("\n")

	if best, ok := ctx.LargestProgram(); ok {
		fmt.Fprintf(&builder, "Segmen terbesar adalah **%s** (%d peserta, rata-rata %.1f).\n\n",
			best.Program, best.Peserta, best.AverageScore)
	}

	builder.WriteString("Saran: bandingkan segmen dengan rata-rata kohort ")
	fmt.Fprintf(&builder, "(**%.1f**), lalu prioritaskan program yang rata-ratanya di bawah angka tersebut.\n", ctx.AverageScore)
	return strings.TrimSpace(builder.String())
}

// hrSkillProfile menyusun peta kompetensi per kategori.
func hrSkillProfile(ctx HRContext) string {
	if len(ctx.SkillProfile) == 0 {
		return "Belum ada percobaan tryout yang cukup untuk menyusun profil kompetensi."
	}

	var builder strings.Builder
	builder.WriteString("### Profil kompetensi per kategori\n\n")
	for _, skill := range ctx.StrongestSkills(len(ctx.SkillProfile)) {
		marker := ""
		if skill.AverageScore < float64(ctx.PassingThreshold) {
			marker = " — di bawah ambang"
		}
		fmt.Fprintf(&builder, "- **%s** — rata-rata %.1f dari %d percobaan%s\n", skill.Name, skill.AverageScore, skill.Attempts, marker)
	}

	if weak := ctx.WeakestSkills(2); len(weak) > 0 {
		builder.WriteString("\n**Prioritas perbaikan**\n\n")
		for _, skill := range weak {
			fmt.Fprintf(&builder, "- **%s** (%.1f) — tambah latihan terbimbing dan ulasan pembahasan.\n", skill.Name, skill.AverageScore)
		}
	}
	return strings.TrimSpace(builder.String())
}

// hrMonthlyTrend menampilkan aktivitas beberapa bulan terakhir.
func hrMonthlyTrend(ctx HRContext) string {
	if len(ctx.MonthlyTrend) == 0 {
		return "Belum ada aktivitas tryout yang tercatat pada periode ini."
	}

	var builder strings.Builder
	builder.WriteString("### Tren aktivitas bulanan\n\n")
	builder.WriteString("| Bulan | Percobaan | Rata-rata skor |\n| --- | --- | --- |\n")
	for _, month := range ctx.MonthlyTrend {
		fmt.Fprintf(&builder, "| %s | %d | %.1f |\n", month.Label, month.Attempts, month.AverageScore)
	}

	latest := ctx.MonthlyTrend[len(ctx.MonthlyTrend)-1]
	previous := HRMonth{}
	hasPrevious := len(ctx.MonthlyTrend) > 1
	if hasPrevious {
		previous = ctx.MonthlyTrend[len(ctx.MonthlyTrend)-2]
	}

	builder.WriteString("\n")
	fmt.Fprintf(&builder, "Bulan terakhir (**%s**): %d percobaan, rata-rata %.1f.\n", latest.Label, latest.Attempts, latest.AverageScore)
	if hasPrevious {
		switch {
		case latest.Attempts > previous.Attempts:
			fmt.Fprintf(&builder, "Aktivitas naik **%d percobaan** dibanding %s.\n", latest.Attempts-previous.Attempts, previous.Label)
		case latest.Attempts < previous.Attempts:
			fmt.Fprintf(&builder, "Aktivitas turun **%d percobaan** dibanding %s — periksa apakah bertepatan dengan jadwal kerja padat.\n",
				previous.Attempts-latest.Attempts, previous.Label)
		default:
			fmt.Fprintf(&builder, "Aktivitas stabil dibanding %s.\n", previous.Label)
		}
	}
	return strings.TrimSpace(builder.String())
}

// hrScoreDistribution menampilkan sebaran skor percobaan.
func hrScoreDistribution(ctx HRContext) string {
	if len(ctx.ScoreDistribution) == 0 {
		return "Belum ada nilai percobaan yang bisa disebar ke dalam rentang skor."
	}

	var builder strings.Builder
	builder.WriteString("### Sebaran skor percobaan\n\n")
	for _, bucket := range ctx.ScoreDistribution {
		fmt.Fprintf(&builder, "- %s: **%d percobaan**\n", bucket.Label, bucket.Count)
	}
	fmt.Fprintf(&builder, "\nRata-rata kohort **%.1f**, ambang latihan **%d**, dan **%.1f%% percobaan** sudah di atas ambang.\n",
		ctx.AverageScore, ctx.PassingThreshold, ctx.PassingRate)
	return strings.TrimSpace(builder.String())
}

// hrActionItems menyusun daftar tindakan singkat dari kondisi kohort.
func hrActionItems(ctx HRContext) string {
	var builder strings.Builder
	step := 0

	if ctx.InactivePeserta > 0 {
		step++
		fmt.Fprintf(&builder, "%d. Hubungi **%d peserta** yang belum berlatih — risiko terbesar ada di kelompok ini.\n",
			step, ctx.InactivePeserta)
	}
	if weak := ctx.WeakestSkills(1); len(weak) > 0 {
		step++
		fmt.Fprintf(&builder, "%d. Perkuat **%s** (rata-rata %.1f) lewat latihan tambahan terarah.\n",
			step, weak[0].Name, weak[0].AverageScore)
	}
	if ctx.TotalLessons > 0 && ctx.CompletionRate < 60 {
		step++
		fmt.Fprintf(&builder, "%d. Dorong penyelesaian materi: baru **%.1f%%** materi tuntas.\n", step, ctx.CompletionRate)
	}
	if step == 0 {
		builder.WriteString("1. Pertahankan ritme: seluruh peserta aktif dan tidak ada kategori di bawah ambang.\n")
	}
	return builder.String()
}

// hrLearnerActivity menjelaskan kondisi aktivitas seorang peserta.
func hrLearnerActivity(learner HRLearner) string {
	activity := strings.TrimSpace(learner.Activity)
	if learner.AttemptCount == 0 || !learner.HasScore {
		if activity != "" {
			return "belum pernah mengerjakan tryout (aktivitas terakhir " + activity + ")"
		}
		return "belum pernah mengerjakan tryout"
	}
	if activity != "" {
		return fmt.Sprintf("skor terakhir %d, aktivitas terakhir %s", learner.Score, activity)
	}
	return fmt.Sprintf("skor terakhir %d", learner.Score)
}
