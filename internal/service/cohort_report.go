package service

// Laporan kelas khusus (Fase 1 langkah 6): agregat capaian peserta satu kelas,
// daftar prestasi & pendampingan, serta PDF siap kirim ke instansi.
//
// Angka laporan berasal dari `repository.CohortRepository` (query yang selalu
// difilter kohort & peran peserta); bagian di berkas ini murni pengolahan —
// klasifikasi capaian, urutan daftar, dan penyusunan dokumen PDF — sehingga bisa
// diuji tanpa database.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/ebook"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

/* ------------------------------ laporan kelas ----------------------------- */

// Jendela & batas daftar pada laporan kelas.
const (
	cohortReportActiveWindowDays = 30
	cohortReportSkillLimit       = 12
	cohortReportTopLimit         = 5
	cohortReportAtRiskLimit      = 10
)

// Report menyusun laporan satu kelas khusus (agregat per kohort).
//
// Semua angka dihitung dari anggota kelas itu saja, sehingga laporan kelas A tidak
// memuat peserta kelas B (isolasi data §4.7). Peserta yang belum pernah berlatih
// tetap muncul dengan catatan "Belum mulai" — bukan dihitung sebagai gagal.
func (s *CohortService) Report(ctx context.Context, id string) (*dto.CohortReportView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.report(ctx, *cohort)
}

// report menyusun laporan dari data kelas yang sudah dimuat.
func (s *CohortService) report(ctx context.Context, cohort models.Cohort) (*dto.CohortReportView, error) {
	threshold := s.cfg.Business.PassingThreshold
	now := time.Now().UTC()

	counts, err := s.cohortCounts(ctx, []string{cohort.ID})
	if err != nil {
		return nil, err
	}
	rows, err := s.cohorts.CohortReportMembers(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}
	attempts, err := s.cohorts.CohortReportAttemptStats(ctx, cohort.ID, threshold)
	if err != nil {
		return nil, err
	}
	learning, err := s.cohorts.CohortReportLearningStats(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}
	modules, err := s.cohorts.CohortReportModuleProgress(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}
	skills, err := s.cohorts.CohortReportSkillProfile(ctx, cohort.ID, cohortReportSkillLimit)
	if err != nil {
		return nil, err
	}
	active, err := s.cohorts.CohortReportActiveMembers(ctx, cohort.ID, now.AddDate(0, 0, -cohortReportActiveWindowDays))
	if err != nil {
		return nil, err
	}
	mentors, err := s.users.ListByIDs(ctx, mentorIDsOf(cohort))
	if err != nil {
		return nil, err
	}

	members := make([]dto.CohortReportMemberView, 0, len(rows))
	certificates := 0
	reporting := 0
	for _, row := range rows {
		view := cohortReportMemberView(row, int(learning.TotalLessons), threshold)
		certificates += view.Certificates
		if view.AttemptCount > 0 {
			reporting++
		}
		members = append(members, view)
	}

	// Laporan menghitung **peserta** saja: mentor & admin instansi tidak dinilai.
	memberCount, err := s.cohorts.CohortReportMemberCounts(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}

	summary := dto.CohortReportSummary{
		TotalMembers:      memberCount.Total,
		ActiveMembers:     memberCount.Active,
		CompletedMembers:  memberCount.Completed,
		LeftMembers:       memberCount.Left,
		ReportingMembers:  reporting,
		ParticipationRate: cohortPercentFloat(int64(reporting), int64(memberCount.Total)),
		ActiveLast30Days:  active,
		ActiveWindowDays:  cohortReportActiveWindowDays,
		TotalAttempts:     attempts.TotalAttempts,
		AverageScore:      roundTo1(attempts.AverageScore),
		TopScore:          attempts.TopScore,
		PassingRate:       cohortPercentFloat(attempts.PassedAttempts, attempts.TotalAttempts),
		PracticeHours:     roundTo1(float64(attempts.TotalSeconds) / 3600),
		InClassAttempts:   attempts.InClassAttempts,
		DistinctTryouts:   attempts.DistinctTryouts,
		TotalLessons:      learning.TotalLessons,
		CompletedLessons:  learning.CompletedLessons,
		CompletionRate:    cohortPercentFloat(learning.CompletedLessons, learning.TotalLessons),
		Certificates:      certificates,
		PassingThreshold:  threshold,
	}

	return &dto.CohortReportView{
		Cohort:      s.cohortSummaryView(cohort, counts[cohort.ID], mentorName(cohort, mentors)),
		Period:      dto.CohortReportPeriod{StartsAt: formatTimePtr(cohort.StartsAt), EndsAt: formatTimePtr(cohort.EndsAt), AccessUntil: formatTimePtr(cohort.AccessUntil)},
		Summary:     summary,
		Modules:     cohortReportModuleViews(modules),
		Skills:      cohortReportSkillViews(skills),
		Members:     members,
		TopLearners: cohortReportTopLearners(members, cohortReportTopLimit),
		AtRisk:      cohortReportAtRisk(members, cohortReportAtRiskLimit),
		GeneratedAt: formatTime(now),
		Notes: []string{
			"Angka pada laporan ini berasal dari data latihan & progres materi anggota kelas, bukan prediksi kelulusan seleksi BUMN.",
			fmt.Sprintf("\"Perlu pendampingan\" berarti skor terakhir di bawah ambang internal (%d); \"Belum mulai\" berarti peserta belum pernah mengerjakan tryout.", threshold),
		},
	}, nil
}

// ReportPDF menyiapkan berkas PDF laporan kelas (siap dikirim ke instansi).
func (s *CohortService) ReportPDF(ctx context.Context, id string) ([]byte, string, error) {
	report, err := s.Report(ctx, id)
	if err != nil {
		return nil, "", err
	}

	document := cohortReportDocument(report)
	return ebook.Render(document), "laporan-kelas-" + cohortFileName(report.Cohort.Code) + ".pdf", nil
}

// curriculumViewsFor memuat kurikulum sebuah kelas beserta jumlah materinya.
func (s *CohortService) curriculumViewsFor(ctx context.Context, cohortID string) ([]dto.CohortCurriculumView, error) {
	items, err := s.cohorts.ListCohortCurriculum(ctx, cohortID)
	if err != nil {
		return nil, err
	}

	moduleIDs := make([]string, 0, len(items))
	for _, item := range items {
		moduleIDs = append(moduleIDs, item.ModuleID)
	}
	counts, err := s.classroom.LessonCountsByModules(ctx, moduleIDs)
	if err != nil {
		return nil, err
	}

	out := s.curriculumViews(items)
	for index := range out {
		out[index].LessonCount = counts[out[index].ModuleID]
	}
	return out, nil
}

/* ------------------------- tampilan laporan kelas ------------------------- */

// cohortReportMemberView menyusun capaian satu anggota pada laporan kelas.
func cohortReportMemberView(
	row repository.CohortReportMemberRow,
	lessonsTotal int,
	threshold int,
) dto.CohortReportMemberView {
	user := models.User{Name: row.Name, Email: row.Email}
	return dto.CohortReportMemberView{
		UserID:           row.UserID,
		Name:             row.Name,
		Initials:         user.Initials(),
		Email:            row.Email,
		Program:          row.Program,
		Status:           row.Status,
		JoinedAt:         formatTime(row.JoinedAt),
		AttemptCount:     row.AttemptCount,
		AverageScore:     roundTo1(row.AverageScore),
		BestScore:        row.BestScore,
		LastScore:        row.LastScore,
		LastActivity:     formatTimePtr(row.LastActivity),
		LessonsCompleted: row.LessonsCompleted,
		LessonsTotal:     lessonsTotal,
		ProgressPercent:  percentOf(row.LessonsCompleted, lessonsTotal),
		Certificates:     row.Certificates,
		Note:             cohortMemberNote(row, threshold),
	}
}

// cohortMemberNote mengklasifikasikan capaian anggota untuk kolom catatan.
//
// Aturannya sengaja sederhana dan berbasis angka yang terlihat pada baris yang
// sama, supaya pembaca laporan bisa menelusuri dasarnya:
//
//	"Belum mulai"        → belum ada percobaan tryout
//	"Perlu pendampingan" → skor terakhir di bawah ambang internal
//	"Prestasi"           → skor terakhir & rata-rata sama-sama ≥ ambang
//	"Lancar"             → skor terakhir ≥ ambang, rata-rata masih di bawahnya
func cohortMemberNote(row repository.CohortReportMemberRow, threshold int) string {
	if row.AttemptCount == 0 {
		return "Belum mulai"
	}
	last := row.BestScore
	if row.LastScore != nil {
		last = *row.LastScore
	}
	switch {
	case last < threshold:
		return "Perlu pendampingan"
	case row.AverageScore >= float64(threshold):
		return "Prestasi"
	default:
		return "Lancar"
	}
}

// cohortReportModuleViews menyusun capaian per modul kurikulum kelas.
func cohortReportModuleViews(rows []repository.CohortReportModuleRow) []dto.CohortReportModuleView {
	out := make([]dto.CohortReportModuleView, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.CohortReportModuleView{
			ModuleID:         row.ModuleID,
			ModuleTitle:      row.Title,
			ProductSlug:      row.ProductSlug,
			Required:         row.Required,
			SortOrder:        row.SortOrder,
			LessonCount:      row.LessonCount,
			CompletedLessons: row.CompletedLessons,
			CompletionRate:   cohortPercentFloat(int64(row.CompletedLessons), int64(row.LessonCount)),
		})
	}
	return out
}

// cohortReportSkillViews menyusun profil subtes kelas.
func cohortReportSkillViews(rows []repository.CohortReportSkillRow) []dto.CohortReportSkillView {
	out := make([]dto.CohortReportSkillView, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.CohortReportSkillView{
			Name:         row.Name,
			AverageScore: roundTo1(row.AverageScore),
			Attempts:     row.Attempts,
		})
	}
	return out
}

// cohortReportTopLearners mengambil peserta berprestasi (skor terbaik tertinggi).
func cohortReportTopLearners(members []dto.CohortReportMemberView, limit int) []dto.CohortReportMemberView {
	candidates := make([]dto.CohortReportMemberView, 0, len(members))
	for _, member := range members {
		if member.AttemptCount > 0 {
			candidates = append(candidates, member)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].BestScore != candidates[j].BestScore {
			return candidates[i].BestScore > candidates[j].BestScore
		}
		if candidates[i].AverageScore != candidates[j].AverageScore {
			return candidates[i].AverageScore > candidates[j].AverageScore
		}
		return candidates[i].Name < candidates[j].Name
	})
	return limitCohortViews(candidates, limit)
}

// cohortReportAtRisk mengambil peserta yang perlu pendampingan (belum mulai dulu).
func cohortReportAtRisk(members []dto.CohortReportMemberView, limit int) []dto.CohortReportMemberView {
	candidates := make([]dto.CohortReportMemberView, 0, len(members))
	for _, member := range members {
		if member.Note == "Perlu pendampingan" || member.Note == "Belum mulai" {
			candidates = append(candidates, member)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		leftEmpty := candidates[i].AttemptCount == 0
		rightEmpty := candidates[j].AttemptCount == 0
		if leftEmpty != rightEmpty {
			return leftEmpty
		}
		leftScore, rightScore := lastScoreOf(candidates[i]), lastScoreOf(candidates[j])
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		return candidates[i].Name < candidates[j].Name
	})
	return limitCohortViews(candidates, limit)
}

// lastScoreOf mengambil skor terakhir anggota (skor terbaik bila belum ada).
func lastScoreOf(member dto.CohortReportMemberView) int {
	if member.LastScore != nil {
		return *member.LastScore
	}
	return member.BestScore
}

// limitCohortViews memotong daftar sesuai batas laporan.
func limitCohortViews(items []dto.CohortReportMemberView, limit int) []dto.CohortReportMemberView {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

// cohortPercentFloat menghitung persentase satu desimal (0 bila pembagi nol).
//
// Dipakai angka laporan kelas yang butuh satu desimal (mis. tingkat kelulusan);
// persentase bulat memakai `percentOf` bersama di learning_path.go.
func cohortPercentFloat(part, total int64) float64 {
	if total <= 0 || part <= 0 {
		return 0
	}
	value := roundTo1(float64(part) / float64(total) * 100)
	if value > 100 {
		return 100
	}
	return value
}

// cohortFileName membersihkan kode kelas agar aman dipakai sebagai nama berkas.
func cohortFileName(code string) string {
	clean := strings.Map(func(char rune) rune {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			return char
		case char == '-' || char == '_':
			return char
		default:
			return '-'
		}
	}, code)
	if clean == "" {
		return "kelas"
	}
	return clean
}

/* ------------------------------- PDF laporan ------------------------------ */

// cohortReportDocument menyusun isi PDF laporan kelas.
//
// Bagian murni (tanpa akses data) supaya teks laporan bisa diuji tanpa database;
// angka & daftar diambil apa adanya dari `dto.CohortReportView`.
func cohortReportDocument(report *dto.CohortReportView) ebook.Document {
	cohort := report.Cohort
	subtitle := cohort.Name
	if cohort.OrgName != "" {
		subtitle += " — " + cohort.OrgName
	}

	blocks := []ebook.Block{
		{Heading: true, Text: "Identitas kelas"},
		{Text: "Kelas: " + cohort.Name + " (" + cohort.Code + ")"},
	}
	if cohort.OrgName != "" {
		blocks = append(blocks, ebook.Block{Text: "Instansi: " + cohort.OrgName})
	}
	if cohort.MentorName != "" {
		blocks = append(blocks, ebook.Block{Text: "Mentor: " + cohort.MentorName})
	}
	blocks = append(blocks, ebook.Block{
		Text: "Periode: " + periodText(report.Period) + " · Status: " + cohort.StatusLabel,
	})

	summary := report.Summary
	blocks = append(blocks,
		ebook.Block{Heading: true, Text: "Ringkasan"},
		ebook.Block{Text: fmt.Sprintf("Anggota: %d (aktif %d, lulus %d, keluar %d)",
			summary.TotalMembers, summary.ActiveMembers, summary.CompletedMembers, summary.LeftMembers)},
		ebook.Block{Text: fmt.Sprintf("Partisipasi latihan: %d dari %d anggota (%.1f%%); aktif dalam %d hari terakhir: %d anggota",
			summary.ReportingMembers, summary.TotalMembers, summary.ParticipationRate,
			summary.ActiveWindowDays, summary.ActiveLast30Days)},
		ebook.Block{Text: fmt.Sprintf("Percobaan tryout: %d (tercatat milik kelas ini: %d) · rata-rata skor %.1f · skor tertinggi %d · lulus ambang %d: %.1f%%",
			summary.TotalAttempts, summary.InClassAttempts, summary.AverageScore,
			summary.TopScore, summary.PassingThreshold, summary.PassingRate)},
		ebook.Block{Text: fmt.Sprintf("Kelengkapan materi: %d dari %d materi selesai (%.1f%%) · jam latihan %.1f · sertifikat terbit %d",
			summary.CompletedLessons, summary.TotalLessons, summary.CompletionRate,
			summary.PracticeHours, summary.Certificates)},
	)

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Capaian per modul"})
	if len(report.Modules) == 0 {
		blocks = append(blocks, ebook.Block{Text: "Kurikulum kelas belum diisi."})
	}
	for _, module := range report.Modules {
		label := "wajib"
		if !module.Required {
			label = "opsional"
		}
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s (%s): %d/%d materi selesai (%.1f%%)",
			module.ModuleTitle, label, module.CompletedLessons, module.LessonCount, module.CompletionRate)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Profil subtes"})
	if len(report.Skills) == 0 {
		blocks = append(blocks, ebook.Block{Text: "Belum ada percobaan tryout pada kelas ini."})
	}
	for _, skill := range report.Skills {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s: rata-rata %.1f dari %d percobaan",
			skill.Name, skill.AverageScore, skill.Attempts)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Peserta berprestasi"})
	if len(report.TopLearners) == 0 {
		blocks = append(blocks, ebook.Block{Text: "Belum ada peserta berprestasi pada kelas ini."})
	}
	for _, member := range report.TopLearners {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s (%s): skor terbaik %d, rata-rata %.1f, %d percobaan",
			member.Name, member.Program, member.BestScore, member.AverageScore, member.AttemptCount)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Perlu pendampingan"})
	if len(report.AtRisk) == 0 {
		blocks = append(blocks, ebook.Block{Text: "Tidak ada peserta yang perlu pendampingan."})
	}
	for _, member := range report.AtRisk {
		score := "belum ada percobaan"
		if member.LastScore != nil {
			score = fmt.Sprintf("skor terakhir %d", *member.LastScore)
		}
		blocks = append(blocks, ebook.Block{Bullet: true, Text: fmt.Sprintf("%s (%s): %s, progres materi %d%%",
			member.Name, member.Program, score, member.ProgressPercent)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Rincian anggota"})
	for _, member := range report.Members {
		blocks = append(blocks, ebook.Block{Text: fmt.Sprintf("%s · %s · %d percobaan · rata-rata %.1f · skor terbaik %d · materi %d/%d · sertifikat %d · %s",
			member.Name, member.Status, member.AttemptCount, member.AverageScore,
			member.BestScore, member.LessonsCompleted, member.LessonsTotal,
			member.Certificates, member.Note)})
	}

	blocks = append(blocks, ebook.Block{Heading: true, Text: "Catatan"})
	for _, note := range report.Notes {
		blocks = append(blocks, ebook.Block{Bullet: true, Text: note})
	}

	return ebook.Document{
		Title:    "Laporan Kelas Khusus",
		Subtitle: subtitle,
		Footer:   "BUMN Academy · laporan kelas dibuat " + report.GeneratedAt,
		Blocks:   blocks,
	}
}

// periodText menyusun teks periode kelas untuk laporan.
func periodText(period dto.CohortReportPeriod) string {
	if period.StartsAt == "" && period.EndsAt == "" {
		return "tanpa periode tetap"
	}
	return fmt.Sprintf("%s s.d. %s", orDash(period.StartsAt), orDash(period.EndsAt))
}

// orDash mengembalikan tanda hubung bila nilai kosong.
func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
