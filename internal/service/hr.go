package service

import (
	"context"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Batas analitik HR — dikumpulkan di satu tempat agar mudah disesuaikan.
const (
	// hrActiveWindowDays adalah rentang hari yang dianggap "aktif" berlatih.
	hrActiveWindowDays = 30
	// hrAtRiskLimit membatasi jumlah peserta pada daftar pendampingan.
	hrAtRiskLimit = 12
	// hrTopLearnerLimit membatasi jumlah peserta pada daftar berprestasi.
	hrTopLearnerLimit = 8
	// hrSkillLimit membatasi jumlah baris profil kompetensi.
	hrSkillLimit = 12
	// hrTrendMonths adalah panjang tren aktivitas bulanan.
	hrTrendMonths = 6
	// hrProgramLimit membatasi jumlah baris sebaran program.
	hrProgramLimit = 12
)

// HRService menyusun analitik SDM (people analytics) untuk panel admin:
// partisipasi peserta, kelengkapan belajar, kesiapan kompetensi, peserta
// berprestasi, dan peserta yang perlu pendampingan.
//
// Seluruh angka dihitung dari data nyata backend (users, attempts, lesson
// progress) — tidak ada data mock.
type HRService struct {
	users     *repository.UserRepository
	attempts  *repository.AttemptRepository
	classroom *repository.ClassroomRepository
	cfg       *config.Config
}

// NewHRService membentuk service analitik HR.
func NewHRService(
	users *repository.UserRepository,
	attempts *repository.AttemptRepository,
	classroom *repository.ClassroomRepository,
	cfg *config.Config,
) *HRService {
	return &HRService{users: users, attempts: attempts, classroom: classroom, cfg: cfg}
}

// Analytics menghitung seluruh metrik analitik HR.
func (s *HRService) Analytics(ctx context.Context) (*dto.HRAnalyticsView, error) {
	threshold := s.cfg.Business.PassingThreshold

	totalPeserta, err := s.users.CountByRole(ctx, models.RolePeserta)
	if err != nil {
		return nil, err
	}

	global, err := s.attempts.GlobalStats(ctx, threshold)
	if err != nil {
		return nil, err
	}

	participation, err := s.attempts.ParticipationStats(ctx, hrActiveWindowDays)
	if err != nil {
		return nil, err
	}

	learning, err := s.classroom.LearningStats(ctx)
	if err != nil {
		return nil, err
	}

	distribution, err := s.attempts.ScoreDistribution(ctx)
	if err != nil {
		return nil, err
	}
	buckets := make([]dto.BucketCount, 0, len(distribution))
	for _, row := range distribution {
		buckets = append(buckets, dto.BucketCount{Label: row.Label, Count: row.Count})
	}

	skills, err := s.attempts.SkillProfile(ctx, hrSkillLimit)
	if err != nil {
		return nil, err
	}
	skillProfile := make([]dto.SkillScoreView, 0, len(skills))
	for _, skill := range skills {
		skillProfile = append(skillProfile, dto.SkillScoreView{
			Name:         skill.Name,
			AverageScore: roundTo1(skill.AverageScore),
			Attempts:     skill.Attempts,
		})
	}

	trend, err := s.attempts.MonthlyTrend(ctx, hrTrendMonths)
	if err != nil {
		return nil, err
	}
	monthly := make([]dto.MonthlyPoint, 0, len(trend))
	for _, point := range trend {
		monthly = append(monthly, dto.MonthlyPoint{
			Label:        monthLabel(point.Label),
			Attempts:     point.Attempts,
			AverageScore: roundTo1(point.AverageScore),
		})
	}

	programs, err := s.users.ProgramBreakdown(ctx)
	if err != nil {
		return nil, err
	}
	programBreakdown := make([]dto.ProgramBreakdown, 0, len(programs))
	for index, row := range programs {
		if index >= hrProgramLimit {
			break
		}
		programBreakdown = append(programBreakdown, dto.ProgramBreakdown{
			Program:      programLabel(row.Program),
			Peserta:      row.Peserta,
			Attempts:     row.Attempts,
			AverageScore: roundTo1(row.AverageScore),
		})
	}

	atRiskRows, err := s.attempts.AtRiskLearners(ctx, threshold, hrAtRiskLimit)
	if err != nil {
		return nil, err
	}
	atRisk := make([]dto.AtRiskLearnerView, 0, len(atRiskRows))
	for _, row := range atRiskRows {
		atRisk = append(atRisk, dto.AtRiskLearnerView{
			Name:         row.Name,
			Initials:     initialsOf(row.Name),
			Email:        row.Email,
			Program:      programLabel(row.Program),
			AttemptCount: row.AttemptCount,
			LastScore:    row.LastScore,
			LastActivity: formatTimePtr(row.LastActivity),
			Reason:       atRiskReason(row.AttemptCount, row.LastScore),
		})
	}

	topLearnerRows, err := s.attempts.TopLearners(ctx, hrTopLearnerLimit)
	if err != nil {
		return nil, err
	}
	topLearners := make([]dto.TopLearnerView, 0, len(topLearnerRows))
	for _, row := range topLearnerRows {
		topLearners = append(topLearners, dto.TopLearnerView{
			Name:         row.Name,
			Initials:     initialsOf(row.Name),
			Email:        row.Email,
			Program:      programLabel(row.Program),
			AttemptCount: row.AttemptCount,
			AverageScore: roundTo1(row.AverageScore),
			BestScore:    row.BestScore,
			BestTitle:    row.BestTitle,
			LastActivity: formatTimePtr(row.LastActivity),
			Note:         topLearnerNote(row.AttemptCount, row.AverageScore, row.BestScore, threshold),
		})
	}

	// Peserta yang belum pernah latihan = total peserta − peserta dengan percobaan.
	inactive := totalPeserta - participation.Learners
	if inactive < 0 {
		inactive = 0
	}
	return &dto.HRAnalyticsView{
		TotalPeserta:      totalPeserta,
		ActivePeserta:     participation.ActiveLearners,
		InactivePeserta:   inactive,
		ParticipationRate: percentageFloat(participation.ActiveLearners, totalPeserta),
		ActiveWindowDays:  hrActiveWindowDays,

		TotalAttempts:    global.TotalAttempts,
		AverageScore:     roundTo1(global.AverageScore),
		TopScore:         global.TopScore,
		PassingThreshold: threshold,
		PassingRate:      percentageFloat(global.PassedAttempts, global.TotalAttempts),
		PracticeHours:    roundTo1(float64(participation.PracticeSeconds) / 3600),
		DistinctTryouts:  global.DistinctTryouts,

		TotalLessons:     learning.TotalLessons,
		CompletedLessons: learning.CompletedLessons,
		ActiveLearners:   learning.ActiveLearners,
		AverageProgress:  roundTo1(learning.AverageProgress),
		CompletionRate:   percentageFloat(learning.CompletedLessons, learning.TotalLessons*learning.ActiveLearners),

		ScoreDistribution: emptyIfNil(buckets),
		SkillProfile:      emptyIfNil(skillProfile),
		MonthlyTrend:      emptyIfNil(monthly),
		ProgramBreakdown:  emptyIfNil(programBreakdown),
		TopLearners:       emptyIfNil(topLearners),
		AtRiskLearners:    emptyIfNil(atRisk),
		GeneratedAt:       formatTime(time.Now().UTC()),
	}, nil
}

// programLabel menormalkan nama program peserta (kosong → "Belum diisi").
func programLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Belum diisi"
	}
	return trimmed
}

// atRiskReason menyusun alasan peserta masuk daftar pendampingan.
func atRiskReason(attemptCount int64, lastScore *int) string {
	if attemptCount <= 0 || lastScore == nil {
		return "Belum pernah mengerjakan tryout"
	}
	return "Skor terakhir di bawah ambang"
}

// topLearnerNote menyusun catatan singkat untuk daftar peserta berprestasi:
// apakah skor terbaiknya sudah melewati ambang, dan apakah capaian itu konsisten
// (rata-rata beberapa percobaan juga di atas ambang).
func topLearnerNote(attemptCount int64, averageScore float64, bestScore, threshold int) string {
	switch {
	case bestScore < threshold:
		return "Skor terbaik belum mencapai ambang"
	case attemptCount >= 3 && averageScore >= float64(threshold):
		return "Konsisten di atas ambang"
	default:
		return "Skor terbaik di atas ambang"
	}
}
