package seed

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Seed asesmen minat–bakat demo (Fase 3).
//
// Tanpa bagian ini halaman talent mapping selalu kosong pada database baru:
// instrumen & norma sudah disemai, tetapi belum ada peserta yang mengerjakannya —
// sehingga peta talenta, matriks 9-box, dan halaman peserta tidak punya isi.
//
// Profil di bawah menghasilkan **persentil & skor kecocokan nyata** karena
// perhitungannya memakai helper yang sama dengan runtime
// (`service.TalentRawScores` / `TalentPercentile` / `TalentFitScore`), bukan
// rumus salinan yang bisa menyimpang.

// riasecDimensionOrder menentukan urutan nilai pada `talentProfileDef.MinatValues`.
var riasecDimensionOrder = []string{
	models.TalentDimensionRealistic,
	models.TalentDimensionInvestigative,
	models.TalentDimensionArtistic,
	models.TalentDimensionSocial,
	models.TalentDimensionEnterprising,
	models.TalentDimensionConventional,
}

// bakatDimensionOrder menentukan urutan nilai pada `talentProfileDef.BakatValues`.
var bakatDimensionOrder = []string{
	models.TalentDimensionNumerik,
	models.TalentDimensionVerbal,
	models.TalentDimensionFigural,
	models.TalentDimensionLearningAgility,
}

// talentProfileDef adalah pola jawaban satu peserta demo.
//
// Nilai 1–5 per dimensi (bukan per butir) supaya profilnya mudah dibaca: seluruh
// butir satu dimensi memakai nilai yang sama, termasuk butir terbalik yang
// pembalikannya ditangani perhitungan skor.
type talentProfileDef struct {
	Email       string
	MinatValues [6]int
	BakatValues [4]int
	// DaysAgo jarak hari sejak asesmen dikumpulkan (menjaga urutan riwayat).
	DaysAgo int
}

// talentProfileDefs adalah peserta demo yang punya hasil instrumen.
//
// Sebarannya sengaja dibuat agar **seluruh sembilan kotak matriks 9-box terisi**:
// kombinasi rata-rata skor tryout (dari `hrAttemptDefs`) dan persentil bakat di
// sini menempatkan peserta di kotak yang berbeda-beda — termasuk baris kinerja
// bawah yang biasanya kosong.
var talentProfileDefs = []talentProfileDef{
	// Peserta inti: punya hasil instrumen, tetapi belum pernah tryout → masuk
	// kelompok "data belum cukup" pada 9-box (contoh aturan §4.4).
	{Email: "peserta@bumnacademy.id", MinatValues: [6]int{4, 5, 3, 4, 4, 4}, BakatValues: [4]int{4, 4, 4, 4}, DaysAgo: 21},

	// Dewi — kinerja tinggi & potensi tinggi → "Bintang".
	{Email: "dewi.lestari@bumnacademy.id", MinatValues: [6]int{5, 5, 3, 4, 5, 4}, BakatValues: [4]int{5, 5, 4, 5}, DaysAgo: 12},
	// Rangga — kinerja sedang, potensi tinggi → "Potensi tumbuh".
	{Email: "rangga.pratama@bumnacademy.id", MinatValues: [6]int{4, 5, 3, 4, 5, 4}, BakatValues: [4]int{5, 4, 5, 4}, DaysAgo: 9},
	// Siti — kinerja tinggi, potensi rendah → "Ahli di bidangnya".
	{Email: "siti.nurhaliza@bumnacademy.id", MinatValues: [6]int{3, 4, 4, 5, 4, 5}, BakatValues: [4]int{3, 4, 3, 3}, DaysAgo: 15},
	// Bayu — keduanya tinggi → "Bintang".
	{Email: "bayu.saputra@bumnacademy.id", MinatValues: [6]int{4, 5, 3, 4, 4, 5}, BakatValues: [4]int{5, 5, 5, 5}, DaysAgo: 18},
	// Intan — keduanya sedang → "Kontributor inti".
	{Email: "intan.permata@bumnacademy.id", MinatValues: [6]int{4, 4, 4, 5, 4, 4}, BakatValues: [4]int{4, 4, 4, 3}, DaysAgo: 11},
	// Fajar — kinerja sedang, potensi rendah → "Kontributor stabil".
	{Email: "fajar.ramadhan@bumnacademy.id", MinatValues: [6]int{4, 3, 4, 4, 3, 4}, BakatValues: [4]int{3, 3, 3, 3}, DaysAgo: 14},
	// Nadia — kinerja tinggi, potensi sedang → "Kinerja kuat, potensi terukur".
	{Email: "nadia.safira@bumnacademy.id", MinatValues: [6]int{3, 5, 4, 4, 4, 4}, BakatValues: [4]int{4, 4, 3, 4}, DaysAgo: 8},
	// Yoga — kinerja sedang, potensi tinggi → "Potensi tumbuh".
	{Email: "yoga.pratama@bumnacademy.id", MinatValues: [6]int{4, 4, 3, 4, 4, 4}, BakatValues: [4]int{4, 5, 4, 4}, DaysAgo: 16},
	// Hana & Rian — punya hasil instrumen tetapi belum pernah tryout → "data belum cukup".
	{Email: "hana.maulida@bumnacademy.id", MinatValues: [6]int{4, 4, 4, 4, 4, 4}, BakatValues: [4]int{4, 4, 4, 4}, DaysAgo: 6},
	{Email: "rian.kusuma@bumnacademy.id", MinatValues: [6]int{4, 3, 4, 5, 4, 4}, BakatValues: [4]int{4, 4, 4, 4}, DaysAgo: 5},
	// Bimo — keduanya rendah → "Perlu perbaikan menyeluruh".
	{Email: "bimo.prasetyo@bumnacademy.id", MinatValues: [6]int{3, 3, 4, 4, 3, 4}, BakatValues: [4]int{2, 2, 2, 2}, DaysAgo: 13},
	// Clara — kinerja rendah, potensi tinggi → "Potensi tinggi, kinerja belum terbaca".
	{Email: "clara.anggraini@bumnacademy.id", MinatValues: [6]int{4, 4, 3, 4, 5, 4}, BakatValues: [4]int{5, 5, 5, 5}, DaysAgo: 10},
	// Dimas — kinerja rendah, potensi sedang → "Kinerja rendah, potensi sedang".
	{Email: "dimas.prakoso@bumnacademy.id", MinatValues: [6]int{4, 3, 4, 4, 3, 4}, BakatValues: [4]int{4, 4, 3, 4}, DaysAgo: 7},
}

// valuesFor mengembalikan nilai jawaban per dimensi untuk sebuah instrumen.
//
// Mengembalikan nil bila instrumen tidak dikenal (mis. instrumen berlisensi yang
// ditambahkan admin) sehingga profil demo tidak memaksakan jawaban.
func (p talentProfileDef) valuesFor(instrumentID string) map[string]int {
	switch instrumentID {
	case "riasec-v1":
		out := make(map[string]int, len(riasecDimensionOrder))
		for index, dimension := range riasecDimensionOrder {
			out[dimension] = p.MinatValues[index]
		}
		return out
	case "aptitude-v1":
		out := make(map[string]int, len(bakatDimensionOrder))
		for index, dimension := range bakatDimensionOrder {
			out[dimension] = p.BakatValues[index]
		}
		return out
	default:
		return nil
	}
}

/* ------------------------------ fungsi seed ------------------------------- */

// talentNormByDimension memetakan dimensi → norma versi `norm-2026`.
func talentNormByDimension() map[string]talentNormDef {
	out := make(map[string]talentNormDef, len(talentNorms))
	for _, norm := range talentNorms {
		out[norm.Dimension] = norm
	}
	return out
}

// seedTalentAssessments mengisi asesmen, jawaban, skor, dan skor kecocokan demo.
//
// Idempoten: asesmen & jawaban memakai `DO NOTHING` (agar koreksi manual admin
// tidak ditimpa), sedangkan skor & skor kecocokan memakai `DO UPDATE` karena
// keduanya turunan yang harus selalu konsisten dengan norma & bobot terbaru.
func seedTalentAssessments(ctx context.Context, db *gorm.DB, userIDs map[string]string) error {
	now := time.Now().UTC()
	norms := talentNormByDimension()

	for _, profile := range talentProfileDefs {
		userID, ok := userIDs[profile.Email]
		if !ok {
			// Akun demo tidak tersedia → lewati profil ini (seed tetap berhasil).
			continue
		}

		consentID := stableID("consent/assessment/" + profile.Email)
		consent := models.ConsentRecord{
			IDBase:      models.IDBase{ID: consentID},
			UserID:      userID,
			Purpose:     models.ConsentPurposeAssessment,
			TextVersion: "asesmen-v1",
			GrantedAt:   now.AddDate(0, 0, -profile.DaysAgo-1),
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&consent).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan persetujuan asesmen %s: %w", profile.Email, err)
		}

		percentiles := make(map[string]float64, len(riasecDimensionOrder)+len(bakatDimensionOrder))
		for _, instrument := range talentInstruments {
			values := profile.valuesFor(instrument.ID)
			if values == nil {
				continue
			}
			assessmentID := stableID(fmt.Sprintf("talent-assessment/%s/%s", profile.Email, instrument.ID))
			finishedAt := now.AddDate(0, 0, -profile.DaysAgo)
			assessment := models.TalentAssessment{
				IDBase:       models.IDBase{ID: assessmentID},
				UserID:       userID,
				InstrumentID: instrument.ID,
				Status:       models.TalentAssessmentCompleted,
				StartedAt:    finishedAt.Add(-25 * time.Minute),
				FinishedAt:   &finishedAt,
				ConsentID:    &consentID,
			}
			err := db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoNothing: true,
			}).Create(&assessment).Error
			if err != nil {
				return fmt.Errorf("seed: gagal menyimpan asesmen %s/%s: %w", profile.Email, instrument.ID, err)
			}

			items, answers := talentDemoAnswers(instrument, assessmentID, values)
			err = db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "assessment_id"}, {Name: "item_id"}},
				DoNothing: true,
			}).Create(&answers).Error
			if err != nil {
				return fmt.Errorf("seed: gagal menyimpan jawaban %s/%s: %w", profile.Email, instrument.ID, err)
			}

			scores := talentDemoScores(profile, instrument, assessmentID, userID, items, answers, norms, now)
			for _, score := range scores {
				percentiles[score.Dimension] = score.Percentile
			}
			err = db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}, {Name: "dimension"}, {Name: "norm_version"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"raw", "percentile", "assessment_id", "computed_at",
				}),
			}).Create(&scores).Error
			if err != nil {
				return fmt.Errorf("seed: gagal menyimpan skor %s/%s: %w", profile.Email, instrument.ID, err)
			}
		}

		fit := talentDemoFitScores(userID, percentiles, now)
		if len(fit) > 0 {
			err := db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}, {Name: "business_line_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"score", "computed_at"}),
			}).Create(&fit).Error
			if err != nil {
				return fmt.Errorf("seed: gagal menyimpan skor kecocokan %s: %w", profile.Email, err)
			}
		}
	}
	return nil
}

// talentDemoAnswers membentuk daftar butir instrumen beserta jawaban profil.
//
// Butir dibentuk dari definisi seed (bukan dibaca dari database) memakai
// `talentItemID`, sehingga ID-nya identik dengan yang disemai
// `seedTalentInstruments`.
func talentDemoAnswers(
	instrument talentInstrumentDef,
	assessmentID string,
	values map[string]int,
) ([]models.TalentItem, []models.TalentAnswer) {
	items := make([]models.TalentItem, 0, len(instrument.Items))
	answers := make([]models.TalentAnswer, 0, len(instrument.Items))
	for index, def := range instrument.Items {
		itemID := talentItemID(instrument.ID, def.Dimension, index)
		items = append(items, models.TalentItem{
			IDBase:       models.IDBase{ID: itemID},
			InstrumentID: instrument.ID,
			Text:         def.Text,
			Dimension:    def.Dimension,
			Weight:       1,
			Reverse:      def.Reverse,
			SortOrder:    index,
		})
		answers = append(answers, models.TalentAnswer{
			AssessmentID: assessmentID,
			ItemID:       itemID,
			Value:        values[def.Dimension],
		})
	}
	return items, answers
}

// talentDemoScores menghitung skor mentah & persentil per dimensi.
//
// Memakai helper service yang sama dengan runtime; cadangan "persentase terhadap
// skor maksimum" juga disalin dari `TalentService.Submit` agar perilakunya sama
// saat norma sebuah dimensi belum tersedia.
func talentDemoScores(
	profile talentProfileDef,
	instrument talentInstrumentDef,
	assessmentID, userID string,
	items []models.TalentItem,
	answers []models.TalentAnswer,
	norms map[string]talentNormDef,
	now time.Time,
) []models.TalentScore {
	answerMap := make(map[string]int, len(answers))
	for _, answer := range answers {
		answerMap[answer.ItemID] = answer.Value
	}
	raw := service.TalentRawScores(items, answerMap)
	maxRaw := service.TalentMaxRawByDimension(items)

	out := make([]models.TalentScore, 0, len(raw))
	for dimension, value := range raw {
		percentile := 0.0
		if norm, ok := norms[dimension]; ok {
			percentile = service.TalentPercentile(value, norm.Mean, norm.SD)
		} else if maximum := maxRaw[dimension]; maximum > 0 {
			percentile = service.TalentClampPercentile(math.Round(value/maximum*1000) / 10)
		}
		assessment := assessmentID
		out = append(out, models.TalentScore{
			UserID:       userID,
			Dimension:    dimension,
			Raw:          math.Round(value*100) / 100,
			Percentile:   percentile,
			NormVersion:  instrument.NormVersion,
			AssessmentID: &assessment,
			ComputedAt:   now,
		})
	}
	return out
}

// talentDemoFitScores menghitung skor kecocokan seluruh lini bisnis aktif.
//
// Lini bisnis tanpa dimensi yang cocok dilewati — sama seperti runtime, yang
// tidak menulis angka menyesatkan untuk bobot kosong (§4.2).
func talentDemoFitScores(userID string, percentiles map[string]float64, now time.Time) []models.TalentFitScore {
	out := make([]models.TalentFitScore, 0, len(businessLineDefs))
	for _, def := range businessLineDefs {
		score, ok := service.TalentFitScore(percentiles, def.Weights)
		if !ok {
			continue
		}
		out = append(out, models.TalentFitScore{
			UserID:         userID,
			BusinessLineID: stableID("business-line/" + def.Key),
			Score:          score,
			ComputedAt:     now,
		})
	}
	return out
}
