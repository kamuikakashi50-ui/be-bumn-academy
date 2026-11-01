package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji keselarasan skema talent mapping (Fase 3 langkah 3).

const talentMigrationUp = "000018_talent_mapping.up.sql"
const talentMigrationDown = "000018_talent_mapping.down.sql"

func TestSkemaModelTalentSelarasDenganMigrasi(t *testing.T) {
	migration := readMigration(t, talentMigrationUp)
	db := newDryRunDB(t)

	cases := []struct {
		table string
		model any
	}{
		{table: "consent_records", model: &models.ConsentRecord{}},
		{table: "talent_instruments", model: &models.TalentInstrument{}},
		{table: "talent_items", model: &models.TalentItem{}},
		{table: "talent_norms", model: &models.TalentNorm{}},
		{table: "talent_assessments", model: &models.TalentAssessment{}},
		{table: "talent_answers", model: &models.TalentAnswer{}},
		{table: "talent_scores", model: &models.TalentScore{}},
		{table: "business_line_profiles", model: &models.BusinessLineProfile{}},
		{table: "talent_fit_scores", model: &models.TalentFitScore{}},
	}

	for _, tc := range cases {
		t.Run(tc.table, func(t *testing.T) {
			columns := tableColumns(t, migration, tc.table)

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
				if statement.Schema.LookUpField(column) == nil {
					t.Errorf("kolom %q pada tabel %s tidak dipetakan model", column, tc.table)
				}
			}
		})
	}
}

func TestRollbackTalentMembalikkanSeluruhPerubahan(t *testing.T) {
	down := readMigration(t, talentMigrationDown)

	for _, table := range []string{
		"talent_fit_scores", "business_line_profiles", "talent_scores",
		"talent_answers", "talent_assessments", "talent_norms", "talent_items",
		"talent_instruments", "consent_records",
	} {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Errorf("rollback tidak menghapus tabel %s", table)
		}
	}
	// `talent_instruments` dihapus setelah tabel yang merujuknya, dan
	// `consent_records` paling akhir (dirujuk `talent_assessments`).
	if strings.Index(down, "DROP TABLE IF EXISTS talent_instruments") <
		strings.Index(down, "DROP TABLE IF EXISTS talent_items") {
		t.Error("rollback menghapus `talent_instruments` sebelum `talent_items`")
	}
	if strings.Index(down, "DROP TABLE IF EXISTS consent_records") <
		strings.Index(down, "DROP TABLE IF EXISTS talent_assessments") {
		t.Error("rollback menghapus `consent_records` sebelum `talent_assessments`")
	}
}

// TestMigrasiTalentMenegakkanNilaiTerbatas menjaga aturan yang mudah rusak:
// jawaban 1–5, persentil 0–100, skor kecocokan 0–100, dan status asesmen terbatas.
func TestMigrasiTalentMenegakkanNilaiTerbatas(t *testing.T) {
	migration := readMigration(t, talentMigrationUp)

	for _, constraint := range []string{
		"talent_answers_value_check",
		"talent_scores_percentile_check",
		"talent_fit_scores_score_check",
		"talent_assessments_status_check",
		"talent_norms_sd_check",
	} {
		if !strings.Contains(migration, constraint) {
			t.Errorf("migrasi tidak menegakkan %s", constraint)
		}
	}
}
