package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji keselarasan skema kompetensi (Fase 3 langkah 1).
//
// Kesalahan seperti kolom model yang tidak ada di migrasi hanya terlihat saat
// runtime (INSERT gagal); lebih murah ditangkap unit test yang membaca migrasi.

const competencyMigrationUp = "000019_competency.up.sql"
const competencyMigrationDown = "000019_competency.down.sql"

func TestSkemaModelKompetensiSelarasDenganMigrasi(t *testing.T) {
	migration := readMigration(t, competencyMigrationUp)
	db := newDryRunDB(t)

	cases := []struct {
		table string
		model any
	}{
		{table: "competencies", model: &models.Competency{}},
		{table: "competency_levels", model: &models.CompetencyLevel{}},
		{table: "position_requirements", model: &models.PositionRequirement{}},
		{table: "user_competencies", model: &models.UserCompetency{}},
		{table: "bank_competency_map", model: &models.BankCompetencyMap{}},
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

func TestRollbackKompetensiMembalikkanSeluruhPerubahan(t *testing.T) {
	down := readMigration(t, competencyMigrationDown)

	for _, table := range []string{
		"bank_competency_map", "user_competencies", "position_requirements",
		"competency_levels", "competencies",
	} {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Errorf("rollback tidak menghapus tabel %s", table)
		}
	}
	// Tabel referensi (`competencies`) dihapus terakhir karena tabel lain
	// memakai foreign key ke sana.
	if strings.Index(down, "DROP TABLE IF EXISTS competencies") <
		strings.Index(down, "DROP TABLE IF EXISTS bank_competency_map") {
		t.Error("rollback menghapus `competencies` sebelum tabel yang merujuknya")
	}
}

// TestMigrasiKompetensiMenegakkanSumberPenilaian menjaga aturan §2.4: level dari
// tryout adalah proksi, dan sumber penilaian wajib salah satu nilai yang dikenal.
func TestMigrasiKompetensiMenegakkanSumberPenilaian(t *testing.T) {
	migration := readMigration(t, competencyMigrationUp)

	for _, source := range []string{"tryout", "self", "asesor", "manager", "psikotes"} {
		if !strings.Contains(migration, "'"+source+"'") {
			t.Errorf("migrasi tidak mengenal sumber penilaian %q", source)
		}
	}
	if !strings.Contains(migration, "user_competencies_source_check") {
		t.Error("migrasi tidak menegakkan CHECK pada kolom source")
	}
}
