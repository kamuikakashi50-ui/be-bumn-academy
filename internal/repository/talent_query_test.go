package repository

import (
	"context"
	"strings"
	"testing"
)

// Uji bentuk query data pendukung matriks 9-box (Fase 3 langkah 5).

func TestUserPerformanceScoresMengelompokkanPerPeserta(t *testing.T) {
	repo := &CompetencyRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Table("attempts AS a").
		Select("a.user_id AS user_id, COALESCE(AVG(a.score), 0) AS average_score, COUNT(*) AS attempts").
		Where("a.user_id IN ?", []string{"u1", "u2"}).
		Group("a.user_id"))

	for _, want := range []string{
		"AVG(a.score)",
		"COUNT(*)",
		"FROM attempts AS a",
		"WHERE a.user_id IN",
		"GROUP BY \"a\".\"user_id\"",
		// Alias kolom harus selaras dengan tag gorm UserPerformanceRow.
		"AS user_id",
		"AS average_score",
		"AS attempts",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query kinerja tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestUserPerformanceScoresTanpaPesertaMengembalikanNil(t *testing.T) {
	repo := &CompetencyRepository{db: newDryRunDB(t)}

	rows, err := repo.UserPerformanceScores(context.Background(), nil)
	if err != nil {
		t.Fatalf("UserPerformanceScores(nil) error: %v", err)
	}
	if rows != nil {
		t.Errorf("daftar peserta kosong harus menghasilkan nil, dapat %d baris", len(rows))
	}
}
