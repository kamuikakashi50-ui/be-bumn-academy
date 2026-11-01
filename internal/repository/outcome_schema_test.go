package repository

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji keselarasan skema & penjaga aturan KPI kelulusan (Fase 2 langkah 1).
//
// Kesalahan seperti kolom model yang tidak ada di migrasi, atau query agregat yang
// lupa menyaring peran peserta, hanya terlihat saat runtime (INSERT gagal, angka
// KPI membengkak karena staf ikut terhitung). Keduanya lebih murah ditangkap di
// unit test yang membaca migrasi & sumber query.

const kpiMigrationUp = "000017_selection_outcomes.up.sql"
const kpiMigrationDown = "000017_selection_outcomes.down.sql"

func TestSkemaModelKPISelarasDenganMigrasi(t *testing.T) {
	migration := readMigration(t, kpiMigrationUp)
	db := newDryRunDB(t)

	cases := []struct {
		table string
		model any
	}{
		{table: "selection_stages", model: &models.SelectionStage{}},
		{table: "selection_outcomes", model: &models.SelectionOutcome{}},
		{table: "selection_outcome_audits", model: &models.SelectionOutcomeAudit{}},
		{table: "kpi_targets", model: &models.KPITarget{}},
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

func TestMigrasiKPIMenyediakanTahapanSeleksiLengkap(t *testing.T) {
	migration := readMigration(t, kpiMigrationUp)

	stages := []string{
		"administrasi", "tahap1_tkd", "tahap1_akhlaq", "tahap1_twk",
		"tahap2_bing", "tahap2_learning_agility", "tkb",
		"wawancara_user", "wawancara_hc", "mcu", "offering",
	}
	for _, stage := range stages {
		if !strings.Contains(migration, "('"+stage+"'") {
			t.Errorf("migrasi tidak menyemai tahapan %q", stage)
		}
	}

	// Tepat satu tahap penentu kelulusan akhir (dipakai `finalPassRate`):
	// baris tahapan yang punya `TRUE` pada kolom is_final **dan** active.
	finalRows := regexp.MustCompile(`(?m)\('[a-z0-9_]+',[^)]*\bTRUE,\s*\d+,\s*TRUE\)`).
		FindAllString(migration, -1)
	if len(finalRows) != 1 {
		t.Fatalf("tahap final harus tepat satu, ditemukan %d baris:\n%s", len(finalRows), strings.Join(finalRows, "\n"))
	}
	if !strings.Contains(finalRows[0], "'offering'") {
		t.Errorf("tahap final harus `offering` (asumsi §10 butir 1), dapat: %s", finalRows[0])
	}
}

func TestRollbackKPIMembalikkanSeluruhPerubahan(t *testing.T) {
	down := readMigration(t, kpiMigrationDown)

	for _, table := range []string{"kpi_targets", "selection_outcome_audits", "selection_outcomes", "selection_stages"} {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Errorf("rollback tidak menghapus tabel %s", table)
		}
	}
	// Tabel referensi dihapus terakhir: `selection_outcomes` & `kpi_targets`
	// memakai foreign key ke `selection_stages`.
	if strings.Index(down, "DROP TABLE IF EXISTS selection_stages") <
		strings.Index(down, "DROP TABLE IF EXISTS selection_outcomes") {
		t.Error("rollback harus menghapus selection_outcomes sebelum selection_stages")
	}
}

// TestKolomTulisanRepositoryKPIAdaDiMigrasi memeriksa setiap nama kolom yang
// ditulis repository KPI (lewat `Update("…")` maupun kunci pada
// `Updates(map[string]any{…})`) benar-benar ada di migrasi 000017.
func TestKolomTulisanRepositoryKPIAdaDiMigrasi(t *testing.T) {
	columns := migrationColumns(t, readMigration(t, kpiMigrationUp))
	if len(columns) == 0 {
		t.Fatal("daftar kolom migrasi kosong — pemeriksaan tidak bermakna")
	}

	updatePattern := regexp.MustCompile(`Update\("([a-z0-9_]+)"`)
	updatesMapPattern := regexp.MustCompile(`(?s)Updates\(map\[string\]any\{(.*?)\n\t*\}\)`)
	keyPattern := regexp.MustCompile(`"([a-z0-9_]+)":`)

	checked := 0
	for _, file := range []string{"outcome.go", "kpi.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("gagal membaca %s: %v", file, err)
		}

		for _, match := range updatePattern.FindAllStringSubmatch(string(source), -1) {
			checked++
			if _, ok := columns[match[1]]; !ok {
				t.Errorf("%s menulis kolom %q yang tidak ada di migrasi %s", file, match[1], kpiMigrationUp)
			}
		}
		for _, block := range updatesMapPattern.FindAllStringSubmatch(string(source), -1) {
			for _, key := range keyPattern.FindAllStringSubmatch(block[1], -1) {
				checked++
				if _, ok := columns[key[1]]; !ok {
					t.Errorf("%s menulis kolom %q yang tidak ada di migrasi %s", file, key[1], kpiMigrationUp)
				}
			}
		}
	}

	if checked < 10 {
		t.Fatalf("jumlah kolom tulisan yang diperiksa hanya %d — penjaga tidak berjalan", checked)
	}
}

// TestQueryKPIMenyaringPesertaDanLaporan menjaga dua aturan yang paling mudah
// rusak saat query baru ditambahkan:
//
//  1. populasi KPI hanya `role = 'peserta'` (mentor/staf bukan peserta seleksi);
//  2. setiap query capaian membaca `selection_outcomes` (angka selalu berdasar
//     laporan hasil seleksi, bukan taksiran dari data latihan).
func TestQueryKPIMenyaringPesertaDanLaporan(t *testing.T) {
	source, err := os.ReadFile("kpi.go")
	if err != nil {
		t.Fatalf("gagal membaca repository KPI: %v", err)
	}

	pattern := regexp.MustCompile("(?s)(kpi[A-Za-z]+(?:Query|Template)) = `(.*?)`")
	matches := pattern.FindAllStringSubmatch(string(source), -1)
	if len(matches) < 7 {
		t.Fatalf("jumlah query KPI yang diperiksa = %d, harap minimal 7", len(matches))
	}

	for _, match := range matches {
		name, query := match[1], match[2]
		if !strings.Contains(query, "role = 'peserta'") {
			t.Errorf("query %s tidak menyaring peran peserta:\n%s", name, query)
		}
		if !strings.Contains(query, "selection_outcomes") {
			t.Errorf("query %s tidak membaca tabel selection_outcomes:\n%s", name, query)
		}
	}
}

// TestModelKPITidakMemakaiTagDefaultYangMenimpaNilaiNol menjaga nilai nol yang
// bermakna: peserta yang menolak agregat, tahap yang dinonaktifkan, dan tahap
// non-final tidak boleh ditimpa tag `default:` saat INSERT.
func TestModelKPITidakMemakaiTagDefaultYangMenimpaNilaiNol(t *testing.T) {
	db := newDryRunDB(t)

	cases := []struct {
		name    string
		model   any
		columns []string
	}{
		{name: "tahapan seleksi", model: &models.SelectionStage{}, columns: []string{"active", "is_final"}},
		{name: "hasil seleksi", model: &models.SelectionOutcome{}, columns: []string{"aggregate_consent"}},
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
