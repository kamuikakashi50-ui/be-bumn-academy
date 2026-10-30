package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji keselarasan skema: setiap model Fase 1 harus punya kolom yang persis sama
// dengan tabelnya di migrasi. Ketidakselarasan seperti ini hanya terlihat saat
// runtime (mis. `quiz_attempts` tidak punya created_at/updated_at, sehingga INSERT
// yang memakai `models.Base` gagal), jadi lebih murah ditangkap di unit test.
func TestSkemaModelFase1SelarasDenganMigrasi(t *testing.T) {
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000015_learning_paths.up.sql"))
	if err != nil {
		t.Fatalf("gagal membaca berkas migrasi: %v", err)
	}

	db := newDryRunDB(t)
	cases := []struct {
		table string
		model any
	}{
		{table: "learning_paths", model: &models.LearningPath{}},
		{table: "learning_path_steps", model: &models.LearningPathStep{}},
		{table: "lesson_quizzes", model: &models.LessonQuiz{}},
		{table: "lesson_quiz_questions", model: &models.LessonQuizQuestion{}},
		{table: "quiz_attempts", model: &models.QuizAttempt{}},
		{table: "certificates", model: &models.Certificate{}},
		{table: "certificate_counters", model: &models.CertificateCounter{}},
	}

	for _, tc := range cases {
		t.Run(tc.table, func(t *testing.T) {
			columns := tableColumns(t, string(migration), tc.table)

			statement := &gorm.Statement{DB: db}
			if err := statement.Parse(tc.model); err != nil {
				t.Fatalf("gagal membaca skema model: %v", err)
			}

			for _, column := range statement.Schema.DBNames {
				if _, ok := columns[column]; !ok {
					t.Errorf("model memakai kolom %q yang tidak ada pada tabel %s (INSERT akan gagal)", column, tc.table)
				}
			}
			for column := range columns {
				found := false
				for _, modelColumn := range statement.Schema.DBNames {
					if modelColumn == column {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("kolom %q pada tabel %s tidak dipetakan model", column, tc.table)
				}
			}
		})
	}
}

// tableColumns mengambil nama kolom sebuah CREATE TABLE dari berkas migrasi.
//
// Sengaja sederhana: hanya menangani format migrasi proyek ini (satu kolom per
// baris, definisi tabel ditutup tanda kurung). CONSTRAINT dan komentar dilewati.
func tableColumns(t *testing.T, migration, table string) map[string]struct{} {
	t.Helper()

	marker := "CREATE TABLE IF NOT EXISTS " + table + " ("
	start := strings.Index(migration, marker)
	if start < 0 {
		t.Fatalf("tabel %s tidak ditemukan di migrasi", table)
	}

	body := migration[start+len(marker):]
	end := strings.Index(body, "\n);")
	if end < 0 {
		t.Fatalf("definisi tabel %s tidak tertutup rapi di migrasi", table)
	}

	out := map[string]struct{}{}
	for _, line := range strings.Split(body[:end], "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Lewati batasan tingkat tabel (PRIMARY KEY (...), UNIQUE (...), dst.).
		if _, isKeyword := tableConstraintKeywords[fields[0]]; isKeyword {
			continue
		}
		out[fields[0]] = struct{}{}
	}
	return out
}

// tableConstraintKeywords adalah kata pertama definisi batasan tingkat tabel yang
// tidak boleh dianggap nama kolom.
var tableConstraintKeywords = map[string]struct{}{
	"CONSTRAINT": {},
	"PRIMARY":    {},
	"UNIQUE":     {},
	"FOREIGN":    {},
	"CHECK":      {},
	"EXCLUDE":    {},
}
