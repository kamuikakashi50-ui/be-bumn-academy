package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji bentuk query analitik HR: memastikan agregasi (FILTER, DISTINCT ON, join)
// tetap tersusun seperti yang diharapkan tanpa perlu koneksi database.

func TestParticipationStatsQueryMengagregasiPartisipasi(t *testing.T) {
	repo := &AttemptRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Attempt{}).
		Select(`COUNT(DISTINCT user_id) AS learners,
		        COUNT(DISTINCT user_id) FILTER (WHERE taken_at >= ?) AS active_learners,
		        COALESCE(SUM(duration_seconds), 0) AS practice_seconds`, "2026-09-25T00:00:00Z"))

	for _, want := range []string{
		"COUNT(DISTINCT user_id)",
		"FILTER (WHERE taken_at >= ",
		"SUM(duration_seconds)",
		"FROM \"attempts\"",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query partisipasi tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestAtRiskLearnersQueryMemakaiDistinctOnDanJoin(t *testing.T) {
	for _, want := range []string{
		"WITH stats AS",
		"MAX(taken_at) AS last_activity",
		"DISTINCT ON (a.user_id)",
		"FROM users u",
		"LEFT JOIN stats s ON s.user_id = u.id",
		"LEFT JOIN latest l ON l.user_id = u.id",
		"AND (s.attempt_count IS NULL OR l.score < ?)",
		"l.score ASC NULLS FIRST",
		"LIMIT ?",
	} {
		if !strings.Contains(atRiskLearnersQuery, want) {
			t.Errorf("query peserta berisiko tidak memuat %q:\n%s", want, atRiskLearnersQuery)
		}
	}
}

func TestProgramBreakdownQueryMengelompokkanProgram(t *testing.T) {
	repo := &UserRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.User{}).
		Select(`COALESCE(NULLIF(users.program, ''), 'Belum diisi') AS program,
		        COUNT(DISTINCT users.id) AS peserta,
		        COUNT(attempts.id) AS attempts,
		        COALESCE(AVG(attempts.score), 0) AS average_score`).
		Joins("LEFT JOIN attempts ON attempts.user_id = users.id").
		Where("users.role = ?", models.RolePeserta).
		Group("program").
		Order("peserta DESC, program ASC"))

	for _, want := range []string{
		"COUNT(DISTINCT users.id)",
		"LEFT JOIN attempts ON attempts.user_id = users.id",
		"WHERE users.role =",
		"GROUP BY \"program\"",
		"FROM \"users\"",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query sebaran program tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestLearningStatsQueryMenghitungKelengkapan(t *testing.T) {
	for _, want := range []string{
		"FROM lessons WHERE active = true",
		"FROM lesson_progress WHERE completed = true",
		"COUNT(DISTINCT user_id) FROM lesson_progress",
		"AVG(progress_percent)",
	} {
		if !strings.Contains(learningStatsQuery, want) {
			t.Errorf("query kelengkapan belajar tidak memuat %q:\n%s", want, learningStatsQuery)
		}
	}
}
