package seed

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji konsistensi data kamus kompetensi (Fase 3 langkah 1): kode unik, deskriptor
// level lengkap, dan seluruh rujukan menunjuk kompetensi yang benar-benar ada.

func TestKompetensiDefKodeUnikDanLengkap(t *testing.T) {
	seen := make(map[string]struct{}, len(competencyDefs))
	for _, def := range competencyDefs {
		if def.Code == "" || def.Name == "" {
			t.Fatalf("kompetensi tanpa kode/nama: %+v", def)
		}
		if strings.ToLower(def.Code) != def.Code {
			t.Errorf("kode kompetensi harus huruf kecil: %q", def.Code)
		}
		if _, ok := seen[def.Code]; ok {
			t.Errorf("kode kompetensi ganda: %q", def.Code)
		}
		seen[def.Code] = struct{}{}

		switch def.Cluster {
		case models.CompetencyClusterTeknis, models.CompetencyClusterManajerial,
			models.CompetencyClusterSosialKultural:
		default:
			t.Errorf("klaster kompetensi %q tidak dikenal (kompetensi %s)", def.Cluster, def.Code)
		}

		descriptors := competencyLevelDescriptors(def.Name)
		for index, descriptor := range descriptors {
			if strings.TrimSpace(descriptor) == "" {
				t.Errorf("deskriptor level %d kompetensi %s kosong", index+1, def.Code)
			}
			if !strings.Contains(descriptor, def.Name) {
				t.Errorf("deskriptor level %d kompetensi %s tidak menyebut nama kompetensi", index+1, def.Code)
			}
		}
	}
}

func TestStandarJabatanMenunjukKompetensiYangAda(t *testing.T) {
	known := make(map[string]struct{}, len(competencyDefs))
	for _, def := range competencyDefs {
		known[def.Code] = struct{}{}
	}

	for _, req := range positionRequirementDefs {
		if _, ok := known[req.Code]; !ok {
			t.Errorf("standar jabatan %s menunjuk kompetensi %q yang tidak ada", req.Family, req.Code)
		}
		if req.MinLevel < 1 || req.MinLevel > 5 {
			t.Errorf("standar jabatan %s/%s punya min level di luar 1–5: %d", req.Family, req.Code, req.MinLevel)
		}
	}
}

func TestPemetaanBankMenunjukKompetensiYangAdaDanUnik(t *testing.T) {
	known := make(map[string]struct{}, len(competencyDefs))
	for _, def := range competencyDefs {
		known[def.Code] = struct{}{}
	}

	seen := make(map[string]struct{}, len(bankCompetencyDefs))
	for _, def := range bankCompetencyDefs {
		if _, ok := known[def.Code]; !ok {
			t.Errorf("pemetaan bank %q menunjuk kompetensi %q yang tidak ada", def.BankKey, def.Code)
		}
		if _, ok := seen[def.BankKey]; ok {
			t.Errorf("kunci bank ganda pada pemetaan: %q", def.BankKey)
		}
		seen[def.BankKey] = struct{}{}
	}

	// Kategori umum demo harus terpetakan agar proksi tryout benar-benar terisi.
	for _, key := range []string{"numerik", "verbal", "analitis", "akhlaq"} {
		if _, ok := seen[key]; !ok {
			t.Errorf("kategori bank %q belum dipetakan ke kompetensi", key)
		}
	}
}
