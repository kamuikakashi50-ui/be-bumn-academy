package service

import (
	"context"

	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// LearningService melayani kelas live, rekaman, pengingat, dan ringkasan
// dashboard peserta.
type LearningService struct {
	learning    *repository.LearningRepository
	attempts    *repository.AttemptRepository
	enrollments *repository.EnrollmentRepository
	tryouts     *TryoutService
	classroom   *ClassroomService
	cache       *cache.Cache
	cfg         *config.Config
}

// NewLearningService membentuk service ruang belajar.
func NewLearningService(
	learning *repository.LearningRepository,
	attempts *repository.AttemptRepository,
	enrollments *repository.EnrollmentRepository,
	tryouts *TryoutService,
	classroom *ClassroomService,
	cacheLayer *cache.Cache,
	cfg *config.Config,
) *LearningService {
	return &LearningService{
		learning:    learning,
		attempts:    attempts,
		enrollments: enrollments,
		tryouts:     tryouts,
		classroom:   classroom,
		cache:       cacheLayer,
		cfg:         cfg,
	}
}

// LiveSessions mengambil jadwal kelas live beserta status pengingat peserta.
func (s *LearningService) LiveSessions(ctx context.Context, userID string, upcomingOnly bool) ([]dto.LiveSessionView, error) {
	rows, err := cache.Remember(ctx, s.cache, cache.KeyLiveSessions(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) ([]models.LiveSession, error) {
			return s.learning.ListLiveSessions(ctx, false)
		})
	if err != nil {
		return nil, err
	}

	reminded := map[string]bool{}
	if userID != "" {
		ids, err := s.learning.ReminderIDsByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			reminded[id] = true
		}
	}

	out := make([]dto.LiveSessionView, 0, len(rows))
	for _, row := range rows {
		if upcomingOnly && row.Status == models.LiveEnded {
			continue
		}
		out = append(out, toLiveSessionView(row, reminded[row.ID]))
	}
	return out, nil
}

// Recordings mengambil koleksi rekaman dengan filter.
func (s *LearningService) Recordings(ctx context.Context, category, search string) ([]dto.RecordingView, error) {
	rows, err := s.learning.ListRecordings(ctx, category, search)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RecordingView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRecordingView(row))
	}
	return out, nil
}

// ToggleReminder menyalakan/mematikan pengingat sesi live.
func (s *LearningService) ToggleReminder(ctx context.Context, userID, liveSessionID string) (*dto.ReminderResponse, error) {
	reminded, err := s.learning.ToggleReminder(ctx, userID, liveSessionID)
	if err != nil {
		return nil, err
	}
	return &dto.ReminderResponse{LiveSessionID: liveSessionID, Reminded: reminded}, nil
}

// Dashboard menyusun ringkasan halaman utama dashboard peserta.
//
// Seluruh data diambil dari query agregat (bukan loop per item) sehingga
// halaman tetap cepat walau riwayat peserta sudah panjang.
func (s *LearningService) Dashboard(ctx context.Context, userID string) (*dto.DashboardSummary, error) {
	stats, err := s.tryouts.Stats(ctx, userID)
	if err != nil {
		return nil, err
	}

	slugs, err := s.enrollments.OwnedSlugs(ctx, userID)
	if err != nil {
		return nil, err
	}

	trend, err := s.tryouts.ScoreTrend(ctx, userID, 8)
	if err != nil {
		return nil, err
	}
	trendPoints := make([]dto.ScoreTrendPoint, 0, len(trend))
	for _, point := range trend {
		trendPoints = append(trendPoints, dto.ScoreTrendPoint{
			Label:   point.Title,
			Score:   point.Score,
			TakenAt: formatTime(point.TakenAt),
		})
	}

	recent, err := s.tryouts.RecentAttempts(ctx, userID, 5)
	if err != nil {
		return nil, err
	}

	allTryouts, err := s.tryouts.List(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	active := make([]dto.TryoutSummary, 0, 4)
	for _, item := range allTryouts {
		if !item.Unlocked {
			continue
		}
		active = append(active, item)
		if len(active) == 4 {
			break
		}
	}

	live, err := s.LiveSessions(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	if len(live) > 3 {
		live = live[:3]
	}

	activeDays, err := s.attempts.ActiveDays(ctx, userID, 7)
	if err != nil {
		return nil, err
	}

	// Pelajaran terakhir yang belum selesai di ruang kelas (bila ada).
	continueLearning := []dto.LessonRefView{}
	if s.classroom != nil {
		items, err := s.classroom.ContinueLearning(ctx, userID, 3)
		if err != nil {
			return nil, err
		}
		continueLearning = items
	}

	return &dto.DashboardSummary{
		TotalAttempts:    int(stats.TotalAttempts),
		AverageScore:     int(stats.AverageScore + 0.5),
		BestScore:        stats.BestScore,
		LastScore:        stats.LastScore,
		PassedCount:      int(stats.PassedCount),
		PassingThreshold: s.tryouts.PassingThreshold(),
		StudyStreakDays:  activeDays,
		EnrolledSlugs:    emptyIfNil(slugs),
		ActiveTryouts:    active,
		RecentAttempts:   recent,
		UpcomingLive:     live,
		ScoreTrend:       trendPoints,
		ContinueLearning: continueLearning,
	}, nil
}
