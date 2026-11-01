package seed

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji konsistensi data talent mapping (Fase 3 langkah 3–4): id unik, dimensi
// dikenal, norma mencakup seluruh dimensi, dan bobot lini bisnis menunjuk dimensi
// yang benar-benar ada.

func knownTalentDimensions() map[string]struct{} {
	return map[string]struct{}{
		models.TalentDimensionRealistic:       {},
		models.TalentDimensionInvestigative:   {},
		models.TalentDimensionArtistic:        {},
		models.TalentDimensionSocial:          {},
		models.TalentDimensionEnterprising:    {},
		models.TalentDimensionConventional:    {},
		models.TalentDimensionNumerik:         {},
		models.TalentDimensionVerbal:          {},
		models.TalentDimensionFigural:         {},
		models.TalentDimensionLearningAgility: {},
	}
}

func TestTalentInstrumenDefKonsisten(t *testing.T) {
	known := knownTalentDimensions()
	seenInstruments := map[string]struct{}{}
	dimensions := map[string]struct{}{}

	for _, def := range talentInstruments {
		if def.ID == "" || def.Code == "" || def.Name == "" {
			t.Fatalf("instrumen tanpa id/kode/nama: %+v", def)
		}
		if _, ok := seenInstruments[def.ID]; ok {
			t.Errorf("id instrumen ganda: %q", def.ID)
		}
		seenInstruments[def.ID] = struct{}{}

		switch def.Kind {
		case models.TalentKindMinat, models.TalentKindBakat:
		default:
			t.Errorf("jenis instrumen %q tidak dikenal (instrumen %s)", def.Kind, def.ID)
		}
		if len(def.Items) == 0 {
			t.Errorf("instrumen %s tidak punya butir", def.ID)
		}

		perDimension := map[string]int{}
		for _, item := range def.Items {
			if _, ok := known[item.Dimension]; !ok {
				t.Errorf("butir instrumen %s memakai dimensi tak dikenal %q", def.ID, item.Dimension)
			}
			if strings.TrimSpace(item.Text) == "" {
				t.Errorf("butir instrumen %s punya teks kosong", def.ID)
			}
			perDimension[item.Dimension]++
			dimensions[item.Dimension] = struct{}{}
		}
		for dimension, count := range perDimension {
			if count < 3 {
				t.Errorf("dimensi %s pada instrumen %s hanya punya %d butir (minimal 3)", dimension, def.ID, count)
			}
		}
	}

	// Norma harus mencakup seluruh dimensi yang dipakai butir.
	for _, norm := range talentNorms {
		if _, ok := dimensions[norm.Dimension]; !ok {
			t.Errorf("norma untuk dimensi %q yang tidak dipakai butir mana pun", norm.Dimension)
		}
		if norm.SD <= 0 {
			t.Errorf("norma %s punya SD tidak valid: %v", norm.Dimension, norm.SD)
		}
	}
	normed := map[string]struct{}{}
	for _, norm := range talentNorms {
		normed[norm.Dimension] = struct{}{}
	}
	for dimension := range dimensions {
		if _, ok := normed[dimension]; !ok {
			t.Errorf("dimensi %q belum punya norma", dimension)
		}
	}
}

func TestBusinessLineDefMenunjukDimensiYangAda(t *testing.T) {
	known := knownTalentDimensions()
	seen := map[string]struct{}{}

	for _, def := range businessLineDefs {
		if def.Key == "" || def.Name == "" {
			t.Fatalf("lini bisnis tanpa kunci/nama: %+v", def)
		}
		if _, ok := seen[def.Key]; ok {
			t.Errorf("kunci lini bisnis ganda: %q", def.Key)
		}
		seen[def.Key] = struct{}{}

		if len(def.Weights) == 0 {
			t.Errorf("lini bisnis %s tidak punya bobot", def.Key)
		}
		for dimension, weight := range def.Weights {
			if _, ok := known[dimension]; !ok {
				t.Errorf("lini bisnis %s memakai dimensi tak dikenal %q", def.Key, dimension)
			}
			if weight <= 0 {
				t.Errorf("bobot dimensi %s pada lini bisnis %s harus positif", dimension, def.Key)
			}
		}
	}

	// Klaster prioritas (§10 butir 5) harus tersedia.
	for _, key := range []string{"jasa_keuangan", "pertahanan_manufaktur"} {
		if _, ok := seen[key]; !ok {
			t.Errorf("lini bisnis prioritas %q belum disemai", key)
		}
	}
}
