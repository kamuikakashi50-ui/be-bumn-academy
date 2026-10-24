package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newDryRunDB membentuk handle GORM mode DryRun: SQL disusun tanpa koneksi ke
// database, sehingga bentuk query bisa diuji di unit test.
func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.New(postgres.Config{
			DSN: "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable",
		}),
		&gorm.Config{
			DryRun: true,
			// Jangan ping database: cukup menyusun SQL (mode dry run).
			DisableAutomaticPing: true,
			Logger:               logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		t.Fatalf("gagal menyiapkan GORM DryRun: %v", err)
	}
	return db
}

// sqlDariSession menyusun statement tanpa mengeksekusinya.
func sqlDariSession(t *testing.T, session *gorm.DB) string {
	t.Helper()

	statement := session.Session(&gorm.Session{DryRun: true}).Find(&[]struct{}{}).Statement
	return statement.SQL.String()
}

func TestGlobalStatsQueryMenghitungAgregat(t *testing.T) {
	repo := &AttemptRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Attempt{}).
		Select(`COUNT(*) AS total_attempts,
		        COALESCE(AVG(score), 0) AS average_score,
		        COUNT(*) FILTER (WHERE score >= ?) AS passed_attempts,
		        COUNT(DISTINCT user_id) AS active_peserta,
		        COALESCE(MAX(score), 0) AS top_score,
		        COUNT(DISTINCT tryout_id) AS distinct_tryouts`, 70))

	for _, want := range []string{
		"COUNT(*)",
		"AVG(score)",
		"FILTER (WHERE score >= ",
		"COUNT(DISTINCT user_id)",
		"MAX(score)",
		"COUNT(DISTINCT tryout_id)",
		"FROM \"attempts\"",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query agregat tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestSkillProfileQueryMengelompokkanKategori(t *testing.T) {
	repo := &AttemptRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.AttemptCategory{}).
		Select("name AS name, COALESCE(AVG(score), 0) AS average_score, COUNT(*) AS attempts").
		Group("name"))

	for _, want := range []string{
		"FROM \"attempt_categories\"",
		"AVG(score)",
		"GROUP BY \"name\"",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query profil kompetensi tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestTopPerformersQueryMenggabungkanUsers(t *testing.T) {
	repo := &AttemptRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Attempt{}).
		Select(`users.name AS name, attempts.score AS score, attempts.duration_seconds AS duration_seconds`).
		Joins("JOIN users ON users.id = attempts.user_id").
		Order("attempts.score DESC, attempts.duration_seconds ASC, attempts.taken_at DESC").
		Limit(8))

	for _, want := range []string{
		"users.name",
		"JOIN users ON users.id = attempts.user_id",
		"ORDER BY attempts.score DESC",
		"LIMIT",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query skor teratas tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestMonthlyTrendQueryMemakaiDateTrunc(t *testing.T) {
	repo := &AttemptRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Attempt{}).
		Select(`to_char(date_trunc('month', taken_at), 'YYYY-MM') AS label,
		        COUNT(*) AS attempts,
		        COALESCE(AVG(score), 0) AS average_score`).
		Where("taken_at >= ?", "2026-05-01").
		Group("label").
		Order("label ASC"))

	for _, want := range []string{
		"date_trunc('month', taken_at)",
		"to_char(",
		"WHERE taken_at >=",
		"GROUP BY \"label\"",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query tren bulanan tidak memuat %q:\n%s", want, sql)
		}
	}
}
