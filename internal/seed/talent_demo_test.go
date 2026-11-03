package seed

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
)

// Uji data asesmen talenta demo (Fase 3).
//
// Yang paling penting: profil demo harus benar-benar **mengisi seluruh sembilan
// kotak** matriks 9-box. Tanpa uji ini, seed bisa "terlihat lengkap" tetapi
// separuh kotak tetap kosong di UI — kegagalan yang hanya terlihat saat demo.

// seededUserEmails mengumpulkan seluruh email akun yang dibuat seed.
func seededUserEmails(t *testing.T) map[string]bool {
	t.Helper()
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}
	out := map[string]bool{}
	for _, user := range data.Users {
		out[user.Email] = true
	}
	for _, user := range hrCohortUsers {
		out[user.Email] = true
	}
	return out
}

// talentInstrumentByID mengambil definisi instrumen demo berdasarkan id.
func talentInstrumentByID(t *testing.T, id string) talentInstrumentDef {
	t.Helper()
	for _, def := range talentInstruments {
		if def.ID == id {
			return def
		}
	}
	t.Fatalf("instrumen demo %q tidak ditemukan", id)
	return talentInstrumentDef{}
}

// potentialFor menghitung rata-rata persentil dimensi bakat sebuah profil.
//
// Memakai helper service yang sama dengan runtime, jadi angkanya identik dengan
// yang nanti tersimpan di database.
func potentialFor(t *testing.T, profile talentProfileDef) (float64, bool) {
	t.Helper()
	instrument := talentInstrumentByID(t, "aptitude-v1")
	items, answers := talentDemoAnswers(instrument, "uji", profile.valuesFor(instrument.ID))
	answerMap := make(map[string]int, len(answers))
	for _, answer := range answers {
		answerMap[answer.ItemID] = answer.Value
	}
	raw := service.TalentRawScores(items, answerMap)
	norms := talentNormByDimension()

	scores := make([]models.TalentScore, 0, len(raw))
	for dimension, value := range raw {
		norm, ok := norms[dimension]
		if !ok {
			continue
		}
		scores = append(scores, models.TalentScore{
			Dimension:  dimension,
			Raw:        value,
			Percentile: service.TalentPercentile(value, norm.Mean, norm.SD),
		})
	}
	return service.NineBoxPotential(scores)
}

// performanceFor menghitung rata-rata skor tryout demo seorang peserta.
func performanceFor(email string) (float64, bool) {
	var total float64
	var count int
	for _, def := range hrAttemptDefs {
		if def.Email == email {
			total += float64(def.Score)
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func TestProfilTalentDemoKonsisten(t *testing.T) {
	accounts := seededUserEmails(t)
	seen := map[string]bool{}

	for _, profile := range talentProfileDefs {
		if profile.Email == "" {
			t.Fatal("profil talenta tanpa email")
		}
		if seen[profile.Email] {
			t.Errorf("profil talenta %s muncul lebih dari sekali", profile.Email)
		}
		seen[profile.Email] = true

		if !accounts[profile.Email] {
			t.Errorf("profil talenta memakai akun yang tidak dibuat seed: %s", profile.Email)
		}
		if profile.DaysAgo < 0 {
			t.Errorf("profil %s punya DaysAgo negatif (%d)", profile.Email, profile.DaysAgo)
		}

		// Nilai jawaban wajib ada di skala instrumen (1–5).
		values := append([]int{}, profile.MinatValues[:]...)
		values = append(values, profile.BakatValues[:]...)
		for _, value := range values {
			if value < 1 || value > 5 {
				t.Errorf("profil %s memakai nilai jawaban %d di luar skala 1–5", profile.Email, value)
			}
		}

		// Setiap instrumen demo harus punya pemetaan nilai untuk profil ini.
		for _, instrument := range talentInstruments {
			if profile.valuesFor(instrument.ID) == nil {
				t.Errorf("profil %s belum punya pemetaan nilai untuk instrumen %s", profile.Email, instrument.ID)
			}
		}
	}

	// Seluruh dimensi kedua instrumen harus terwakili profil.
	for _, dimension := range riasecDimensionOrder {
		found := false
		for _, profile := range talentProfileDefs {
			if profile.valuesFor("riasec-v1")[dimension] > 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dimensi minat %q tidak diberi nilai oleh profil mana pun", dimension)
		}
	}
	for _, dimension := range bakatDimensionOrder {
		found := false
		for _, profile := range talentProfileDefs {
			if profile.valuesFor("aptitude-v1")[dimension] > 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dimensi bakat %q tidak diberi nilai oleh profil mana pun", dimension)
		}
	}
}

func TestProfilTalentDemoMengisiSeluruhKotakSembilanBox(t *testing.T) {
	quadrants := map[string]bool{}
	mapped := 0
	withoutPerformance := 0

	for _, profile := range talentProfileDefs {
		performance, ok := performanceFor(profile.Email)
		if !ok {
			// Tanpa riwayat tryout → "data belum cukup" (bukan kuadran rendah).
			withoutPerformance++
			continue
		}
		potential, ok := potentialFor(t, profile)
		if !ok {
			t.Errorf("profil %s tidak menghasilkan potensi", profile.Email)
			continue
		}
		mapped++
		quadrants[service.NineBoxQuadrantFor(
			service.NineBoxBand(performance), service.NineBoxBand(potential))] = true
	}

	if mapped == 0 {
		t.Fatal("tidak ada peserta demo yang bisa dipetakan ke 9-box")
	}
	if len(quadrants) != 9 {
		t.Errorf("profil demo hanya mengisi %d dari 9 kotak 9-box: %v", len(quadrants), quadrants)
	}
	if withoutPerformance == 0 {
		t.Error("tidak ada peserta demo berdata instrumen tanpa riwayat tryout; kelompok \"data belum cukup\" akan kosong")
	}
}

func TestSkorKecocokanDemoTerbentukUntukSetiapLiniBisnis(t *testing.T) {
	percentiles := map[string]float64{}
	for _, dimension := range riasecDimensionOrder {
		percentiles[dimension] = 50
	}
	for _, dimension := range bakatDimensionOrder {
		percentiles[dimension] = 50
	}

	for _, line := range businessLineDefs {
		if _, ok := service.TalentFitScore(percentiles, line.Weights); !ok {
			t.Errorf("lini bisnis %q tidak menghasilkan skor kecocokan dari profil lengkap", line.Key)
		}
	}

	// Bobot tanpa dimensi yang cocok harus dilewati, bukan diberi angka 0.
	if _, ok := service.TalentFitScore(percentiles, map[string]float64{"dimensi_tidak_ada": 3}); ok {
		t.Error("bobot tanpa dimensi yang cocok seharusnya tidak menghasilkan skor")
	}
}
