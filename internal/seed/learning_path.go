package seed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// learningPathDef adalah satu jalur belajar demo beserta langkah & kuisnya.
type learningPathDef struct {
	ID          string
	Slug        string
	Title       string
	Description string
	SectorKey   string
	TestType    string
	ProductSlug string
	Certificate bool
	SortOrder   int
	Steps       []learningPathStepDef
	Quizzes     []lessonQuizDef
}

// learningPathStepDef adalah satu langkah (modul) pada jalur demo.
type learningPathStepDef struct {
	ModuleID string
	// Required false menandai modul opsional (tidak menghalangi sertifikat).
	Required bool
}

// lessonQuizDef adalah kuis demo pada satu pelajaran.
//
// Soalnya diambil dari bank soal demo lewat `BankKey` (bukan id yang ditulis
// tangan), supaya penamaan id soal di dataset boleh berubah tanpa membuat seed
// diam-diam kehilangan soal.
type lessonQuizDef struct {
	ID            string
	LessonID      string
	Title         string
	PassScore     int
	MaxAttempts   int
	TimeLimit     int
	BankKey       string
	QuestionCount int
}

// learningPathDefs adalah kurikulum demo yang mengikuti produk `kelas-intensif`
// sehingga jalur belajar bisa dicoba end-to-end oleh akun demo yang sudah
// terdaftar pada produk itu.
var learningPathDefs = []learningPathDef{
	{
		ID:          "path-tkd-dasar",
		Slug:        "jalur-tkd-dasar",
		Title:       "Jalur TKD Dasar",
		Description: "Urutan belajar terarah untuk Tes Kemampuan Dasar: numerik, verbal, lalu logika analitis.",
		SectorKey:   "umum",
		TestType:    "tkd",
		ProductSlug: "kelas-intensif",
		Certificate: true,
		Steps: []learningPathStepDef{
			{ModuleID: "mod-intensif-numerik", Required: true},
			{ModuleID: "mod-intensif-verbal", Required: true},
			{ModuleID: "mod-intensif-analitis", Required: false},
		},
		Quizzes: []lessonQuizDef{
			{
				ID:            "quiz-numerik-deret",
				LessonID:      "les-numerik-deret",
				Title:         "Kuis Deret & Pola",
				PassScore:     70,
				MaxAttempts:   3,
				TimeLimit:     10,
				BankKey:       "numerik",
				QuestionCount: 3,
			},
		},
	},
}

// seedLearningPaths menyiapkan jalur belajar, langkah, dan kuis demo.
//
// Langkah & soal kuis diganti seluruhnya (delete-then-insert) di dalam transaksi,
// mengikuti pola repository admin: susunan yang tersimpan selalu sama dengan yang
// ditulis di seed.
func seedLearningPaths(ctx context.Context, db *gorm.DB, data *Dataset) error {
	for _, def := range learningPathDefs {
		path := models.LearningPath{
			ID:          def.ID,
			Slug:        def.Slug,
			Title:       def.Title,
			Description: def.Description,
			SectorKey:   def.SectorKey,
			TestType:    def.TestType,
			ProductSlug: def.ProductSlug,
			Certificate: def.Certificate,
			SortOrder:   def.SortOrder,
			Active:      true,
		}

		steps := make([]models.LearningPathStep, 0, len(def.Steps))
		for index, stepDef := range def.Steps {
			steps = append(steps, models.LearningPathStep{
				Base:      models.Base{ID: stableID("path-step/" + def.ID + "/" + stepDef.ModuleID)},
				PathID:    def.ID,
				ModuleID:  stepDef.ModuleID,
				Required:  stepDef.Required,
				SortOrder: index,
			})
		}

		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"slug", "title", "description", "sector_key", "test_type",
					"product_slug", "certificate", "sort_order", "active", "updated_at",
				}),
			}).Create(&path).Error; err != nil {
				return err
			}

			if err := tx.Where("path_id = ?", def.ID).Delete(&models.LearningPathStep{}).Error; err != nil {
				return err
			}
			if len(steps) > 0 {
				if err := tx.Create(&steps).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan jalur belajar %s: %w", def.ID, err)
		}

		for _, quizDef := range def.Quizzes {
			if err := seedLessonQuiz(ctx, db, data, quizDef); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedLessonQuiz menyiapkan satu kuis demo beserta susunan soalnya.
//
// Id kuis berupa UUID deterministik dari kunci `def.ID` karena kolom
// `lesson_quizzes.id` bertipe UUID (berbeda dari `learning_paths.id` yang TEXT).
func seedLessonQuiz(ctx context.Context, db *gorm.DB, data *Dataset, def lessonQuizDef) error {
	quizID := stableID("lesson-quiz/" + def.ID)

	questionIDs, err := demoQuestionIDs(ctx, db, data, def.BankKey, def.QuestionCount)
	if err != nil {
		return err
	}
	if len(questionIDs) == 0 {
		// Bank soal demo tidak tersedia (mis. dataset diubah): kuis dilewati dengan
		// peringatan agar seed tetap selesai dan masalahnya tetap terlihat.
		slog.Warn("seed: kuis demo dilewati karena bank soal kosong",
			"kuis", def.ID, "bank", def.BankKey)
		return nil
	}

	quiz := models.LessonQuiz{
		Base:        models.Base{ID: quizID},
		LessonID:    def.LessonID,
		Title:       def.Title,
		PassScore:   def.PassScore,
		MaxAttempts: def.MaxAttempts,
		TimeLimit:   def.TimeLimit,
		Active:      true,
	}

	slots := make([]models.LessonQuizQuestion, 0, len(questionIDs))
	for position, questionID := range questionIDs {
		id := questionID
		slots = append(slots, models.LessonQuizQuestion{
			QuizID:     quizID,
			QuestionID: &id,
			Position:   position,
		})
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"lesson_id", "title", "pass_score", "max_attempts",
				"time_limit", "active", "updated_at",
			}),
		}).Create(&quiz).Error; err != nil {
			return err
		}

		if err := tx.Where("quiz_id = ?", quizID).Delete(&models.LessonQuizQuestion{}).Error; err != nil {
			return err
		}
		return tx.Create(&slots).Error
	})
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan kuis demo %s: %w", def.ID, err)
	}
	return nil
}

// demoQuestionIDs memilih id soal demo untuk sebuah bank soal.
//
// Id kanonik dari dataset (`data.json`) dipakai lebih dulu supaya kuis demo
// menunjuk soal yang sama dengan yang dirujuk dokumentasi. Bila soal itu tidak
// ada (mis. dataset berubah), pencarian jatuh ke bank soal di database dengan
// urutan deterministik.
func demoQuestionIDs(ctx context.Context, db *gorm.DB, data *Dataset, bankKey string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}

	candidates := make([]string, 0, limit)
	for _, def := range data.Questions {
		if def.BankKey != bankKey {
			continue
		}
		candidates = append(candidates, def.ID)
		if len(candidates) == limit {
			break
		}
	}

	if len(candidates) > 0 {
		var existing []string
		err := db.WithContext(ctx).Model(&models.Question{}).
			Where("id IN ? AND active = ?", candidates, true).
			Pluck("id", &existing).Error
		if err != nil {
			return nil, fmt.Errorf("seed: gagal memeriksa soal bank %s: %w", bankKey, err)
		}

		// `IN` tidak menjaga urutan, jadi urutannya dikembalikan sesuai dataset.
		found := make(map[string]struct{}, len(existing))
		for _, id := range existing {
			found[id] = struct{}{}
		}
		ids := make([]string, 0, len(candidates))
		for _, id := range candidates {
			if _, ok := found[id]; ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == len(candidates) {
			return ids, nil
		}
	}

	var ids []string
	err := db.WithContext(ctx).Model(&models.Question{}).
		Where("bank_key = ? AND active = ?", bankKey, true).
		Order("position ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("seed: gagal memuat soal bank %s: %w", bankKey, err)
	}
	return ids, nil
}
