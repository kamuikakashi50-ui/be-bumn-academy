package service

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// corporatePerformerLimit adalah jumlah baris skor teratas pada halaman korporasi.
const corporatePerformerLimit = 8

// CorporateService menyusun insight agregat kohort peserta untuk halaman
// "Solusi Korporasi" (B2B).
//
// Semua data yang keluar dari service ini bersifat agregat; nama peserta hanya
// muncul dalam bentuk tersamarkan (mis. "I*** P***").
type CorporateService struct {
	attempts *repository.AttemptRepository
	users    *repository.UserRepository
	cache    *cache.Cache
	cfg      *config.Config
}

// NewCorporateService membentuk service insight korporasi.
func NewCorporateService(
	attempts *repository.AttemptRepository,
	users *repository.UserRepository,
	cacheLayer *cache.Cache,
	cfg *config.Config,
) *CorporateService {
	return &CorporateService{
		attempts: attempts,
		users:    users,
		cache:    cacheLayer,
		cfg:      cfg,
	}
}

// Insights mengembalikan ringkasan agregat (di-cache agar halaman publik tetap
// ringan walau dibuka banyak pengunjung).
func (s *CorporateService) Insights(ctx context.Context) (*dto.CorporateInsightsView, error) {
	threshold := s.cfg.Business.PassingThreshold

	view, err := cache.Remember(ctx, s.cache, cache.KeyCorporateInsights(), s.cfg.Redis.CatalogTTL,
		func(ctx context.Context) (*dto.CorporateInsightsView, error) {
			return s.build(ctx, threshold)
		})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// build menghitung seluruh angka agregat.
func (s *CorporateService) build(ctx context.Context, threshold int) (*dto.CorporateInsightsView, error) {
	totalPeserta, err := s.users.CountByRole(ctx, models.RolePeserta)
	if err != nil {
		return nil, err
	}

	stats, err := s.attempts.GlobalStats(ctx, threshold)
	if err != nil {
		return nil, err
	}

	buckets, err := s.attempts.ScoreDistribution(ctx)
	if err != nil {
		return nil, err
	}
	distribution := make([]dto.BucketCount, 0, len(buckets))
	for _, bucket := range buckets {
		distribution = append(distribution, dto.BucketCount{Label: bucket.Label, Count: bucket.Count})
	}

	skills, err := s.attempts.SkillProfile(ctx, 12)
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

	performers, err := s.attempts.TopPerformers(ctx, corporatePerformerLimit)
	if err != nil {
		return nil, err
	}
	topPerformers := make([]dto.PerformerView, 0, len(performers))
	for index, row := range performers {
		topPerformers = append(topPerformers, dto.PerformerView{
			Rank:            index + 1,
			DisplayName:     MaskName(row.Name),
			Initials:        initialsOf(row.Name),
			TryoutTitle:     row.Title,
			Score:           row.Score,
			Correct:         row.Correct,
			Total:           row.Total,
			DurationSeconds: row.DurationSeconds,
			TakenAt:         formatTime(row.TakenAt),
		})
	}

	trend, err := s.attempts.MonthlyTrend(ctx, 6)
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

	return &dto.CorporateInsightsView{
		TotalPeserta:      totalPeserta,
		ActivePeserta:     stats.ActivePeserta,
		TotalAttempts:     stats.TotalAttempts,
		DistinctTryouts:   stats.DistinctTryouts,
		AverageScore:      roundTo1(stats.AverageScore),
		TopScore:          stats.TopScore,
		PassingRate:       percentageFloat(stats.PassedAttempts, stats.TotalAttempts),
		PassingThreshold:  threshold,
		ScoreDistribution: distribution,
		SkillProfile:      skillProfile,
		TopPerformers:     topPerformers,
		MonthlyTrend:      monthly,
		GeneratedAt:       formatTime(time.Now().UTC()),
	}, nil
}

// MaskName menyamarkan nama peserta agar halaman publik tidak menampilkan
// identitas asli: setiap kata disisakan huruf pertamanya, sisanya menjadi "***".
//
//	"Ilham Pratama" → "I*** P***"
//	"Rina"          → "R***"
func MaskName(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return "Peserta"
	}

	masked := make([]string, 0, len(words))
	for _, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		// Buang tanda baca di awal agar huruf pertama tetap terbaca.
		first := runes[0]
		if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
			masked = append(masked, "***")
			continue
		}
		masked = append(masked, string(unicode.ToUpper(first))+"***")
	}
	if len(masked) == 0 {
		return "Peserta"
	}
	return strings.Join(masked, " ")
}

// monthLabel mengubah label "2026-10" menjadi "Okt 2026".
func monthLabel(label string) string {
	parsed, err := time.Parse("2006-01", label)
	if err != nil {
		return label
	}
	months := []string{
		"Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
		"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
	}
	return months[int(parsed.Month())-1] + " " + parsed.Format("2006")
}

// roundTo1 membulatkan angka ke satu desimal.
func roundTo1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

// percentageFloat menghitung persentase dengan aman (nol bila pembagi nol).
func percentageFloat(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return roundTo1(float64(part) / float64(total) * 100)
}
