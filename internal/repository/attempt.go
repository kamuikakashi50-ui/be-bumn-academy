package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
)

// AttemptRepository mengakses riwayat hasil tryout dan papan peringkat.
type AttemptRepository struct {
	db *gorm.DB
}

// NewAttemptRepository membentuk repository.
func NewAttemptRepository(db *gorm.DB) *AttemptRepository {
	return &AttemptRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *AttemptRepository) DB() *gorm.DB { return r.db }

// FindByID mengambil attempt beserta rincian skor per kategori.
func (r *AttemptRepository) FindByID(ctx context.Context, id string) (*models.Attempt, error) {
	var attempt models.Attempt
	err := r.db.WithContext(ctx).
		Preload("Categories", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		First(&attempt, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Hasil tryout tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat hasil tryout.")
	}
	return &attempt, nil
}

// ListByUser mengambil riwayat tryout seorang peserta.
func (r *AttemptRepository) ListByUser(ctx context.Context, userID string, page, perPage int) ([]models.Attempt, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Attempt{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err, "Gagal menghitung riwayat tryout.")
	}

	var attempts []models.Attempt
	err := query.
		Preload("Categories", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Order("taken_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&attempts).Error
	if err != nil {
		return nil, 0, apperror.Internal(err, "Gagal memuat riwayat tryout.")
	}
	return attempts, total, nil
}

// ListRecentByUser mengambil beberapa attempt terbaru tanpa paginasi
// (dipakai ringkasan dashboard).
func (r *AttemptRepository) ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.Attempt, error) {
	var attempts []models.Attempt
	err := r.db.WithContext(ctx).
		Preload("Categories", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Where("user_id = ?", userID).
		Order("taken_at DESC").
		Limit(limit).
		Find(&attempts).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat riwayat tryout.")
	}
	return attempts, nil
}

// CountByUser menghitung seluruh attempt milik seorang peserta.
func (r *AttemptRepository) CountByUser(ctx context.Context, userID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Where("user_id = ?", userID).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung hasil tryout peserta.")
	}
	return total, nil
}

// CountByUserAndTryout menghitung berapa kali peserta mengerjakan sebuah tryout.
func (r *AttemptRepository) CountByUserAndTryout(ctx context.Context, userID, tryoutID string) (int, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Where("user_id = ? AND tryout_id = ?", userID, tryoutID).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung percobaan tryout.")
	}
	return int(total), nil
}

// CountByUserPerTryout menghitung percobaan per tryout untuk seorang peserta
// dalam satu query (menghindari N+1 pada daftar tryout).
func (r *AttemptRepository) CountByUserPerTryout(ctx context.Context, userID string) (map[string]int, error) {
	type row struct {
		TryoutID string
		Total    int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select("tryout_id, count(*) AS total").
		Where("user_id = ?", userID).
		Group("tryout_id").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung percobaan tryout.")
	}

	out := make(map[string]int, len(rows))
	for _, item := range rows {
		out[item.TryoutID] = item.Total
	}
	return out, nil
}

// Stats adalah ringkasan performa peserta.
type Stats struct {
	TotalAttempts int64   `gorm:"column:total_attempts"`
	AverageScore  float64 `gorm:"column:average_score"`
	BestScore     int     `gorm:"column:best_score"`
	LastScore     int     `gorm:"column:last_score"`
	PassedCount   int64   `gorm:"column:passed_count"`
}

// StatsByUser menghitung ringkasan performa peserta dalam satu query agregat.
//
// `last_score` diambil dengan ARRAY_AGG(...)[1] (bukan subquery berkorelasi)
// karena query ini tidak memakai GROUP BY — subquery berkorelasi ke kolom luar
// tidak diizinkan PostgreSQL pada bentuk tersebut.
func (r *AttemptRepository) StatsByUser(ctx context.Context, userID string, passingThreshold int) (Stats, error) {
	var stats Stats
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select(`COUNT(*) AS total_attempts,
		        COALESCE(AVG(score), 0) AS average_score,
		        COALESCE(MAX(score), 0) AS best_score,
		        COALESCE((ARRAY_AGG(score ORDER BY taken_at DESC))[1], 0) AS last_score,
		        COUNT(*) FILTER (WHERE score >= ?) AS passed_count`, passingThreshold).
		Where("user_id = ?", userID).
		Scan(&stats).Error
	if err != nil {
		return Stats{}, apperror.Internal(err, "Gagal menghitung statistik peserta.")
	}
	return stats, nil
}

// ScoreTrendPoint adalah satu titik tren skor.
type ScoreTrendPoint struct {
	Title   string    `gorm:"column:title"`
	Score   int       `gorm:"column:score"`
	TakenAt time.Time `gorm:"column:taken_at"`
}

// ScoreTrend mengambil attempt terbaru untuk grafik tren (dibalik ke urutan naik).
func (r *AttemptRepository) ScoreTrend(ctx context.Context, userID string, limit int) ([]ScoreTrendPoint, error) {
	var rows []ScoreTrendPoint
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select("title, score, taken_at").
		Where("user_id = ?", userID).
		Order("taken_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat tren skor.")
	}
	// Balik agar urut dari lama → baru untuk kebutuhan grafik.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

// CountAll menghitung seluruh attempt.
func (r *AttemptRepository) CountAll(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.Attempt{}).Count(&total).Error; err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung hasil tryout.")
	}
	return total, nil
}

// AverageScore menghitung rata-rata skor seluruh peserta.
func (r *AttemptRepository) AverageScore(ctx context.Context) (float64, error) {
	var avg float64
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select("COALESCE(AVG(score), 0)").
		Scan(&avg).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung rata-rata skor.")
	}
	return avg, nil
}

/* ----------------------------- papan peringkat ---------------------------- */

// LeaderboardRow adalah satu baris papan peringkat.
type LeaderboardRow struct {
	Rank            int       `gorm:"column:rank_no"`
	UserID          string    `gorm:"column:user_id"`
	Name            string    `gorm:"column:name"`
	Score           int       `gorm:"column:score"`
	Correct         int       `gorm:"column:correct"`
	Total           int       `gorm:"column:total"`
	DurationSeconds int       `gorm:"column:duration_seconds"`
	TakenAt         time.Time `gorm:"column:taken_at"`
}

// leaderboardCTE mengambil skor TERBAIK setiap peserta untuk satu tryout.
//
// DISTINCT ON (user_id) memanfaatkan index (tryout_id, score DESC, taken_at ASC)
// sehingga hanya satu baris per peserta yang diproses, lalu ROW_NUMBER memberi
// peringkat. Urutan tie-break: skor tertinggi → waktu tercepat → paling awal.
const leaderboardCTE = `
WITH best AS (
    SELECT DISTINCT ON (a.user_id)
           a.user_id, a.score, a.correct, a.total, a.duration_seconds, a.taken_at
      FROM attempts a
     WHERE a.tryout_id = ?
     ORDER BY a.user_id, a.score DESC, a.duration_seconds ASC, a.taken_at ASC
),
ranked AS (
    SELECT b.*,
           ROW_NUMBER() OVER (ORDER BY b.score DESC, b.duration_seconds ASC, b.taken_at ASC) AS rank_no
      FROM best b
)
`

// Leaderboard mengambil papan peringkat sebuah tryout.
func (r *AttemptRepository) Leaderboard(ctx context.Context, tryoutID string, limit int) ([]LeaderboardRow, error) {
	query := leaderboardCTE + `
SELECT r.rank_no, r.user_id, u.name, r.score, r.correct, r.total, r.duration_seconds, r.taken_at
  FROM ranked r
  JOIN users u ON u.id = r.user_id
 ORDER BY r.rank_no
 LIMIT ?`

	var rows []LeaderboardRow
	if err := r.db.WithContext(ctx).Raw(query, tryoutID, limit).Scan(&rows).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat papan peringkat.")
	}
	return rows, nil
}

// LeaderboardRank mengambil peringkat seorang peserta pada sebuah tryout.
// Mengembalikan (nil, nil) bila peserta belum punya hasil.
func (r *AttemptRepository) LeaderboardRank(ctx context.Context, tryoutID, userID string) (*LeaderboardRow, error) {
	query := leaderboardCTE + `
SELECT r.rank_no, r.user_id, u.name, r.score, r.correct, r.total, r.duration_seconds, r.taken_at
  FROM ranked r
  JOIN users u ON u.id = r.user_id
 WHERE r.user_id = ?`

	var row LeaderboardRow
	err := r.db.WithContext(ctx).Raw(query, tryoutID, userID).Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat peringkat peserta.")
	}
	if row.UserID == "" {
		return nil, nil
	}
	return &row, nil
}

// ActiveDays menghitung jumlah hari berbeda dengan aktivitas tryout.
func (r *AttemptRepository) ActiveDays(ctx context.Context, userID string, days int) (int, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	var total int64
	err := r.db.WithContext(ctx).Raw(
		`SELECT COUNT(DISTINCT date(taken_at)) FROM attempts WHERE user_id = ? AND taken_at >= ?`,
		userID, since,
	).Scan(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung hari belajar.")
	}
	return int(total), nil
}

// ScoreBucket adalah hitungan jumlah peserta pada rentang nilai tertentu.
type ScoreBucket struct {
	Label string `gorm:"column:label"`
	Count int64  `gorm:"column:count"`
}

// ScoreDistribution mengelompokkan attempt ke dalam rentang nilai.
func (r *AttemptRepository) ScoreDistribution(ctx context.Context) ([]ScoreBucket, error) {
	var rows []ScoreBucket
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select(`CASE
		          WHEN score >= 90 THEN '90-100'
		          WHEN score >= 80 THEN '80-89'
		          WHEN score >= 70 THEN '70-79'
		          WHEN score >= 60 THEN '60-69'
		          ELSE '<60'
		        END AS label,
		        COUNT(*) AS count`).
		Group("label").
		Order("label DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung distribusi skor.")
	}
	return rows, nil
}

/* ------------------------- insight agregat (B2B/HR) ----------------------- */

// GlobalStatsRow adalah ringkasan agregat seluruh percobaan tryout.
type GlobalStatsRow struct {
	TotalAttempts   int64   `gorm:"column:total_attempts"`
	AverageScore    float64 `gorm:"column:average_score"`
	PassedAttempts  int64   `gorm:"column:passed_attempts"`
	ActivePeserta   int64   `gorm:"column:active_peserta"`
	TopScore        int     `gorm:"column:top_score"`
	DistinctTryouts int64   `gorm:"column:distinct_tryouts"`
}

// GlobalStats menghitung agregat seluruh percobaan tryout (tanpa data pribadi).
//
// `passingThreshold` dipakai untuk menghitung jumlah percobaan yang melewati
// ambang batas internal latihan.
func (r *AttemptRepository) GlobalStats(ctx context.Context, passingThreshold int) (*GlobalStatsRow, error) {
	var row GlobalStatsRow
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select(`COUNT(*) AS total_attempts,
		        COALESCE(AVG(score), 0) AS average_score,
		        COUNT(*) FILTER (WHERE score >= ?) AS passed_attempts,
		        COUNT(DISTINCT user_id) AS active_peserta,
		        COALESCE(MAX(score), 0) AS top_score,
		        COUNT(DISTINCT tryout_id) AS distinct_tryouts`, passingThreshold).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung agregat percobaan tryout.")
	}
	return &row, nil
}

// SkillScoreRow adalah rata-rata skor satu kategori/subtes.
type SkillScoreRow struct {
	Name         string  `gorm:"column:name"`
	AverageScore float64 `gorm:"column:average_score"`
	Attempts     int64   `gorm:"column:attempts"`
}

// SkillProfile mengagregasi rata-rata skor per kategori/subtes lintas peserta.
//
// Dipakai untuk menggambarkan kompetensi kohort (mis. kuat di penalaran numerik,
// lemah di logika analitis) tanpa membuka identitas peserta.
func (r *AttemptRepository) SkillProfile(ctx context.Context, limit int) ([]SkillScoreRow, error) {
	if limit <= 0 {
		limit = 12
	}

	var rows []SkillScoreRow
	err := r.db.WithContext(ctx).Model(&models.AttemptCategory{}).
		Select("name AS name, COALESCE(AVG(score), 0) AS average_score, COUNT(*) AS attempts").
		Group("name").
		Order("average_score DESC, name ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung profil kompetensi.")
	}
	return rows, nil
}

// TopPerformerRow adalah satu percobaan dengan skor tertinggi.
type TopPerformerRow struct {
	Name            string    `gorm:"column:name"`
	Title           string    `gorm:"column:title"`
	Score           int       `gorm:"column:score"`
	Correct         int       `gorm:"column:correct"`
	Total           int       `gorm:"column:total"`
	DurationSeconds int       `gorm:"column:duration_seconds"`
	TakenAt         time.Time `gorm:"column:taken_at"`
}

// TopPerformers mengambil percobaan dengan skor tertinggi.
//
// Nama peserta ikut diambil hanya untuk disamarkan di lapisan service — halaman
// publik tidak boleh menampilkan identitas asli.
func (r *AttemptRepository) TopPerformers(ctx context.Context, limit int) ([]TopPerformerRow, error) {
	if limit <= 0 {
		limit = 8
	}

	var rows []TopPerformerRow
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select(`users.name AS name, attempts.title AS title, attempts.score AS score,
		        attempts.correct AS correct, attempts.total AS total,
		        attempts.duration_seconds AS duration_seconds, attempts.taken_at AS taken_at`).
		Joins("JOIN users ON users.id = attempts.user_id").
		Order("attempts.score DESC, attempts.duration_seconds ASC, attempts.taken_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar skor teratas.")
	}
	return rows, nil
}

// MonthlyTrendRow adalah jumlah percobaan dan rata-rata skor pada satu bulan.
type MonthlyTrendRow struct {
	Label        string  `gorm:"column:label"`
	Attempts     int64   `gorm:"column:attempts"`
	AverageScore float64 `gorm:"column:average_score"`
}

// ParticipationRow adalah ringkasan partisipasi peserta pada latihan tryout.
type ParticipationRow struct {
	Learners        int64 `gorm:"column:learners"`
	ActiveLearners  int64 `gorm:"column:active_learners"`
	PracticeSeconds int64 `gorm:"column:practice_seconds"`
}

// ParticipationStats menghitung jumlah peserta yang pernah latihan, yang masih
// aktif dalam `days` hari terakhir, dan total durasi pengerjaan (detik).
//
// Dipakai analitik HR untuk mengukur tingkat partisipasi kohort.
func (r *AttemptRepository) ParticipationStats(ctx context.Context, days int) (*ParticipationRow, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	var row ParticipationRow
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select(`COUNT(DISTINCT user_id) AS learners,
		        COUNT(DISTINCT user_id) FILTER (WHERE taken_at >= ?) AS active_learners,
		        COALESCE(SUM(duration_seconds), 0) AS practice_seconds`, since).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung partisipasi peserta.")
	}
	return &row, nil
}

// AtRiskLearnerRow adalah satu peserta yang perlu pendampingan.
type AtRiskLearnerRow struct {
	ID           string     `gorm:"column:id"`
	Name         string     `gorm:"column:name"`
	Email        string     `gorm:"column:email"`
	Program      string     `gorm:"column:program"`
	AttemptCount int64      `gorm:"column:attempt_count"`
	LastScore    *int       `gorm:"column:last_score"`
	LastActivity *time.Time `gorm:"column:last_activity"`
}

// atRiskLearnersQuery mengambil peserta yang perlu pendampingan.
//
// `stats` menghitung jumlah percobaan & aktivitas terakhir per peserta;
// `latest` mengambil skor percobaan terbaru memakai DISTINCT ON. Peserta tanpa
// percobaan didahulukan, lalu skor terendah.
const atRiskLearnersQuery = `
WITH stats AS (
    SELECT user_id, COUNT(*) AS attempt_count, MAX(taken_at) AS last_activity
      FROM attempts
     GROUP BY user_id
),
latest AS (
    SELECT DISTINCT ON (a.user_id) a.user_id, a.score
      FROM attempts a
     ORDER BY a.user_id, a.taken_at DESC, a.score DESC
)
SELECT u.id, u.name, u.email, u.program,
       COALESCE(s.attempt_count, 0) AS attempt_count,
       l.score AS last_score,
       s.last_activity AS last_activity
  FROM users u
  LEFT JOIN stats s ON s.user_id = u.id
  LEFT JOIN latest l ON l.user_id = u.id
 WHERE u.role = ?
   AND (s.attempt_count IS NULL OR l.score < ?)
 ORDER BY (s.attempt_count IS NULL) DESC, l.score ASC NULLS FIRST, u.name ASC
 LIMIT ?`

// AtRiskLearners mengambil peserta yang belum pernah mengerjakan tryout atau
// skor terakhirnya masih di bawah ambang. Peserta tanpa percobaan didahulukan,
// lalu skor terendah, agar tim akademik bisa memprioritaskan pendampingan.
func (r *AttemptRepository) AtRiskLearners(ctx context.Context, threshold, limit int) ([]AtRiskLearnerRow, error) {
	if limit <= 0 {
		limit = 10
	}

	var rows []AtRiskLearnerRow
	err := r.db.WithContext(ctx).
		Raw(atRiskLearnersQuery, string(models.RolePeserta), threshold, limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat peserta yang perlu pendampingan.")
	}
	return rows, nil
}

// TopLearnerRow adalah satu peserta berprestasi (skor terbaik tertinggi).
type TopLearnerRow struct {
	ID           string     `gorm:"column:id"`
	Name         string     `gorm:"column:name"`
	Email        string     `gorm:"column:email"`
	Program      string     `gorm:"column:program"`
	AttemptCount int64      `gorm:"column:attempt_count"`
	AverageScore float64    `gorm:"column:average_score"`
	BestScore    int        `gorm:"column:best_score"`
	BestTitle    string     `gorm:"column:best_title"`
	LastActivity *time.Time `gorm:"column:last_activity"`
}

// topLearnersQuery mengambil peserta berprestasi.
//
// `stats` menghitung jumlah percobaan, rata-rata skor, dan aktivitas terakhir
// per peserta; `best` mengambil percobaan berskor tertinggi tiap peserta memakai
// DISTINCT ON (waktu tercepat menjadi penentu bila skornya sama). Hanya peserta
// yang pernah berlatih yang muncul, diurutkan dari skor terbaik.
const topLearnersQuery = `
WITH stats AS (
    SELECT user_id, COUNT(*) AS attempt_count,
           AVG(score) AS average_score, MAX(taken_at) AS last_activity
      FROM attempts
     GROUP BY user_id
),
best AS (
    SELECT DISTINCT ON (a.user_id) a.user_id, a.score AS best_score, a.title AS best_title
      FROM attempts a
     ORDER BY a.user_id, a.score DESC, a.duration_seconds ASC, a.taken_at DESC
)
SELECT u.id, u.name, u.email, u.program,
       s.attempt_count,
       s.average_score,
       b.best_score,
       b.best_title,
       s.last_activity
  FROM users u
  JOIN stats s ON s.user_id = u.id
  JOIN best b ON b.user_id = u.id
 WHERE u.role = ?
 ORDER BY b.best_score DESC, s.average_score DESC, u.name ASC
 LIMIT ?`

// TopLearners mengambil peserta dengan skor terbaik tertinggi — pasangan dari
// daftar pendampingan agar tim akademik bisa melihat dua ujung sebaran sekaligus.
func (r *AttemptRepository) TopLearners(ctx context.Context, limit int) ([]TopLearnerRow, error) {
	if limit <= 0 {
		limit = 8
	}

	var rows []TopLearnerRow
	err := r.db.WithContext(ctx).
		Raw(topLearnersQuery, string(models.RolePeserta), limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat peserta berprestasi.")
	}
	return rows, nil
}

// MonthlyTrend mengagregasi percobaan per bulan (urut lama → baru).
func (r *AttemptRepository) MonthlyTrend(ctx context.Context, months int) ([]MonthlyTrendRow, error) {
	if months <= 0 {
		months = 6
	}

	since := time.Now().UTC().AddDate(0, -months+1, 0)
	since = time.Date(since.Year(), since.Month(), 1, 0, 0, 0, 0, time.UTC)

	var rows []MonthlyTrendRow
	err := r.db.WithContext(ctx).Model(&models.Attempt{}).
		Select(`to_char(date_trunc('month', taken_at), 'YYYY-MM') AS label,
		        COUNT(*) AS attempts,
		        COALESCE(AVG(score), 0) AS average_score`).
		Where("taken_at >= ?", since).
		Group("label").
		Order("label ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung tren bulanan.")
	}
	return rows, nil
}
