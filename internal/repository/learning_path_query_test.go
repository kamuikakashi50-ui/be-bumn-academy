package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Uji bentuk query Fase 1 (jalur belajar, kuis, sertifikat) memakai GORM DryRun:
// SQL disusun tanpa koneksi database, sehingga aturan penting (penyaringan
// taksonomi, hanya percobaan yang sudah dikumpulkan, penguncian penomoran
// sertifikat, agregasi progres) bisa diuji di unit test.

func TestListPathsQueryMenyaringTaksonomiDanMengurutkan(t *testing.T) {
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.LearningPath{}).
		Where("sector_key = ?", "jasa_keuangan").
		Where("test_type = ?", "tkb").
		Where("active = ?", true).
		Order("sort_order ASC, title ASC"))

	for _, want := range []string{
		"sector_key = ",
		"test_type = ",
		"active = ",
		"ORDER BY sort_order ASC, title ASC",
		`FROM "learning_paths"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query daftar jalur tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestFindPathBySlugQueryMemuatLangkahTerurut(t *testing.T) {
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	// Preload dijalankan sebagai query terpisah saat eksekusi, sehingga yang bisa
	// diperiksa pada mode dry-run adalah: query utama menyaring slug, dan langkah
	// memang di-preload (bukan query N+1 manual di service).
	statement := repo.db.
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, module_id ASC")
		}).
		Session(&gorm.Session{DryRun: true}).
		First(&models.LearningPath{}, "slug = ?", "jalur-tkd-keuangan").Statement

	sql := statement.SQL.String()
	for _, want := range []string{`FROM "learning_paths"`, "slug = "} {
		if !strings.Contains(sql, want) {
			t.Errorf("query detail jalur tidak memuat %q:\n%s", want, sql)
		}
	}
	if _, ok := statement.Preloads["Steps"]; !ok {
		t.Errorf("langkah jalur harus di-preload, preload terdaftar: %v", statement.Preloads)
	}
}

func TestListQuizzesByLessonQueryHanyaKuisAktif(t *testing.T) {
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.LessonQuiz{}).
		Where("lesson_id = ?", "les-tkd-1").
		Where("active = ?", true).
		Order("created_at ASC"))

	for _, want := range []string{
		"lesson_id = ",
		"active = ",
		`FROM "lesson_quizzes"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query daftar kuis tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestCountFinishedAttemptsQueryMengabaikanPercobaanBerjalan(t *testing.T) {
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.QuizAttempt{}).
		Where("quiz_id = ? AND user_id = ? AND finished_at IS NOT NULL", "quiz-1", "user-1"))

	for _, want := range []string{
		"quiz_id = ",
		"finished_at IS NOT NULL",
		`FROM "quiz_attempts"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query hitung percobaan tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestQuizHasBrokenQuestionsQueryMemeriksaSoalHilang(t *testing.T) {
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.LessonQuizQuestion{}).
		Where("quiz_id = ? AND question_id IS NULL", "quiz-1"))

	for _, want := range []string{
		"question_id IS NULL",
		`FROM "lesson_quiz_questions"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query pemeriksaan kuis tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestNextCertificateNumberQueryMengunciBarisCounter(t *testing.T) {
	// Penguncian baris wajib: tanpa itu dua peserta yang lulus bersamaan bisa
	// mendapat nomor sertifikat yang sama.
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&models.CertificateCounter{}, "year = ?", 2026))

	for _, want := range []string{
		`FROM "certificate_counters"`,
		"year = ",
		"FOR UPDATE",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query penomoran sertifikat tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestModuleLessonStatsQueryMengagregasiProgresPeserta(t *testing.T) {
	repo := &LearningPathRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Lesson{}).
		Select(`lessons.module_id AS module_id,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE lesson_progress.completed) AS completed`).
		Joins("LEFT JOIN lesson_progress ON lesson_progress.lesson_id = lessons.id AND lesson_progress.user_id = ?", "user-1").
		Where("lessons.module_id IN ?", []string{"mod-a"}).
		Where("lessons.active = ?", true).
		Group("lessons.module_id"))

	for _, want := range []string{
		"COUNT(*) FILTER (WHERE lesson_progress.completed)",
		"LEFT JOIN lesson_progress",
		`GROUP BY "lessons"."module_id"`,
		`FROM "lessons"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query progres modul tidak memuat %q:\n%s", want, sql)
		}
	}
}

// Uji regresi tag model: GORM menimpa nilai nol dengan nilai tag `default:` yang
// bisa diparse Go (mis. `default:true`, `default:70`) saat INSERT. Karena
// `required`, `certificate`, `pass_score`, dan `max_attempts` punya nilai nol yang
// bermakna (opsional, tanpa sertifikat, 0%, tanpa batas percobaan), kolom-kolom
// itu tidak boleh memakai tag tersebut.
func TestModelJalurDanKuisTidakMemakaiTagDefaultYangMenimpaNilaiNol(t *testing.T) {
	db := newDryRunDB(t)

	cases := []struct {
		name    string
		model   any
		columns []string
	}{
		{
			name:    "langkah jalur",
			model:   &models.LearningPathStep{},
			columns: []string{"required"},
		},
		{
			name:    "jalur belajar",
			model:   &models.LearningPath{},
			columns: []string{"certificate"},
		},
		{
			name:    "kuis pelajaran",
			model:   &models.LessonQuiz{},
			columns: []string{"pass_score", "max_attempts"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statement := &gorm.Statement{DB: db}
			if err := statement.Parse(tc.model); err != nil {
				t.Fatalf("gagal membaca skema model: %v", err)
			}

			for _, column := range tc.columns {
				field := statement.Schema.LookUpField(column)
				if field == nil {
					t.Fatalf("kolom %q tidak ditemukan pada skema", column)
				}
				if field.DefaultValueInterface != nil {
					t.Errorf("kolom %q memakai tag default yang bisa diparse (%v): nilai nol akan ditimpa saat INSERT",
						column, field.DefaultValueInterface)
				}
			}
		})
	}
}
