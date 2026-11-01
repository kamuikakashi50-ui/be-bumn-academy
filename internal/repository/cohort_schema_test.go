package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji keselarasan skema Fase 1 langkah 4: setiap model kohort harus punya kolom
// yang persis sama dengan tabelnya di migrasi `000016_cohorts`, dan kolom baru
// pada tabel lama (soal, kelas live, riwayat percobaan) benar-benar ditambahkan.
//
// Ketidakselarasan seperti ini hanya terlihat saat runtime (INSERT gagal karena
// kolom tidak ada), jadi lebih murah ditangkap di unit test.

const cohortMigrationUp = "000016_cohorts.up.sql"
const cohortMigrationDown = "000016_cohorts.down.sql"

func TestSkemaModelKohortSelarasDenganMigrasi(t *testing.T) {
	migration := readMigration(t, cohortMigrationUp)

	db := newDryRunDB(t)
	cases := []struct {
		table string
		model any
	}{
		{table: "organizations", model: &models.Organization{}},
		{table: "cohorts", model: &models.Cohort{}},
		{table: "cohort_members", model: &models.CohortMember{}},
		{table: "cohort_curriculum", model: &models.CohortCurriculumItem{}},
		{table: "cohort_invites", model: &models.CohortInvite{}},
		{table: "cohort_audit_logs", model: &models.CohortAuditLog{}},
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

func TestMigrasiKohortMenambahKolomPadaTabelLama(t *testing.T) {
	migration := readMigration(t, cohortMigrationUp)
	db := newDryRunDB(t)

	// Kolom baru harus ada di migrasi DAN dipetakan model, supaya kode yang
	// menulisnya (mis. penetapan instansi soal, konteks kohort sesi tryout) tidak
	// gagal saat runtime.
	cases := []struct {
		table   string
		column  string
		model   any
		alterOf string
	}{
		{table: "questions", column: "owner_org_id", model: &models.Question{}, alterOf: "ALTER TABLE questions"},
		{table: "live_sessions", column: "cohort_id", model: &models.LiveSession{}, alterOf: "ALTER TABLE live_sessions"},
		{table: "attempts", column: "cohort_id", model: &models.Attempt{}, alterOf: "ALTER TABLE attempts"},
	}

	for _, tc := range cases {
		t.Run(tc.table, func(t *testing.T) {
			if !strings.Contains(migration, tc.alterOf) {
				t.Fatalf("migrasi tidak memuat %q", tc.alterOf)
			}
			if !strings.Contains(migration, "ADD COLUMN IF NOT EXISTS "+tc.column) {
				t.Errorf("migrasi tidak menambahkan kolom %s.%s", tc.table, tc.column)
			}

			statement := &gorm.Statement{DB: db}
			if err := statement.Parse(tc.model); err != nil {
				t.Fatalf("gagal membaca skema model: %v", err)
			}
			if statement.Schema.LookUpField(tc.column) == nil {
				t.Errorf("model %T tidak memetakan kolom %s.%s", tc.model, tc.table, tc.column)
			}
		})
	}
}

// TestRollbackKohortMembalikkanSeluruhPerubahan menjaga agar `migrate down` tidak
// meninggalkan tabel atau kolom Fase 1 di database.
func TestRollbackKohortMembalikkanSeluruhPerubahan(t *testing.T) {
	up := readMigration(t, cohortMigrationUp)
	down := readMigration(t, cohortMigrationDown)

	for _, table := range []string{
		"organizations", "cohorts", "cohort_members",
		"cohort_curriculum", "cohort_invites", "cohort_audit_logs",
	} {
		if !strings.Contains(up, "CREATE TABLE IF NOT EXISTS "+table+" (") {
			t.Fatalf("migrasi up tidak membuat tabel %s", table)
		}
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table+";") {
			t.Errorf("rollback tidak menghapus tabel %s", table)
		}
	}

	for _, column := range []string{"owner_org_id", "cohort_id"} {
		if !strings.Contains(down, "DROP COLUMN IF EXISTS "+column+";") {
			t.Errorf("rollback tidak melepas kolom %s", column)
		}
	}
	if !strings.Contains(down, "DROP CONSTRAINT IF EXISTS certificates_cohort_fk") {
		t.Error("rollback tidak melepas foreign key sertifikat → kohort")
	}
}

// readMigration membaca berkas migrasi dari folder repo.
func readMigration(t *testing.T, name string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatalf("gagal membaca berkas migrasi %s: %v", name, err)
	}
	return string(raw)
}

// TestKolomTulisanRepositoryKohortAdaDiMigrasi menjaga nama kolom yang ditulis
// repository lewat literal (`Update("kolom", …)`) benar-benar ada di migrasi.
//
// Kesalahan seperti ini hanya muncul saat runtime sebagai `INTERNAL_ERROR` dan
// pernah terjadi: `cohorts` tidak punya kolom `updated_at`, sehingga PATCH kelas
// selalu gagal walau unit test lain hijau.
func TestKolomTulisanRepositoryKohortAdaDiMigrasi(t *testing.T) {
	source, err := os.ReadFile("cohort.go")
	if err != nil {
		t.Fatalf("gagal membaca repository kohort: %v", err)
	}

	columns := migrationColumns(t, readMigration(t, cohortMigrationUp))
	if len(columns) == 0 {
		t.Fatal("daftar kolom migrasi kosong — pemeriksaan tidak bermakna")
	}

	pattern := regexp.MustCompile(`Update\("([a-z0-9_]+)"`)
	matches := pattern.FindAllStringSubmatch(string(source), -1)
	if len(matches) == 0 {
		t.Fatal("tidak ada penulisan kolom lewat literal yang bisa diperiksa")
	}
	for _, match := range matches {
		if _, ok := columns[match[1]]; !ok {
			t.Errorf("repository menulis kolom %q yang tidak ada di migrasi %s", match[1], cohortMigrationUp)
		}
	}
}

// migrationColumns mengumpulkan seluruh nama kolom yang dibuat sebuah migrasi:
// dari setiap `CREATE TABLE IF NOT EXISTS …` dan setiap `ADD COLUMN`.
func migrationColumns(t *testing.T, migration string) map[string]struct{} {
	t.Helper()

	out := map[string]struct{}{}
	for _, match := range regexp.MustCompile(`CREATE TABLE IF NOT EXISTS ([a-z_]+) \(`).FindAllStringSubmatch(migration, -1) {
		for column := range tableColumns(t, migration, match[1]) {
			out[column] = struct{}{}
		}
	}
	for _, match := range regexp.MustCompile(`ADD COLUMN IF NOT EXISTS ([a-z_]+)`).FindAllStringSubmatch(migration, -1) {
		out[match[1]] = struct{}{}
	}
	return out
}

// TestQueryLaporanKelasSelaluMenyaringCohortID menjaga aturan isolasi laporan
// (bagian 4.7): setiap query laporan kelas wajib memuat filter `cohort_id` dan
// peran `peserta`, sehingga laporan kelas A tidak mungkin memuat peserta kelas B
// maupun staf pengajar.
//
// Penjaga ini membaca sumber query-nya, jadi query baru yang lupa difilter akan
// langsung tertangkap — kesalahan seperti itu tidak terlihat pada unit test biasa
// (datanya kebetulan terpisah) dan hanya muncul di laporan klien.
func TestQueryLaporanKelasSelaluMenyaringCohortID(t *testing.T) {
	source, err := os.ReadFile("cohort_report.go")
	if err != nil {
		t.Fatalf("gagal membaca repository laporan kohort: %v", err)
	}

	pattern := regexp.MustCompile("(?s)cohortReport[A-Za-z]*Query = `(.*?)`")
	matches := pattern.FindAllStringSubmatch(string(source), -1)
	if len(matches) < 6 {
		t.Fatalf("jumlah query laporan kelas yang diperiksa = %d, harap minimal 6", len(matches))
	}

	for _, match := range matches {
		query := match[1]
		if !strings.Contains(query, "cohort_id") {
			t.Errorf("query laporan kelas tidak memuat filter cohort_id:\n%s", query)
		}
		if !strings.Contains(query, "cohort_members") {
			t.Errorf("query laporan kelas tidak membaca keanggotaan kelas:\n%s", query)
		}
		if !strings.Contains(query, "role = 'peserta'") {
			t.Errorf("query laporan kelas tidak menyaring peran peserta (mentor/staf ikut terhitung):\n%s", query)
		}
	}
}

// TestQueryLaporanAnggotaTetapMemuatPesertaTanpaPercobaan menjaga keputusan §4.7:
// peserta yang belum pernah berlatih tetap muncul di laporan (LEFT JOIN), bukan
// dihilangkan — supaya bisa ditandai "Belum mulai".
func TestQueryLaporanAnggotaTetapMemuatPesertaTanpaPercobaan(t *testing.T) {
	source, err := os.ReadFile("cohort_report.go")
	if err != nil {
		t.Fatalf("gagal membaca repository laporan kohort: %v", err)
	}
	if !strings.Contains(string(source), "LEFT JOIN attempt_stats") {
		t.Error("query laporan anggota harus LEFT JOIN agar peserta tanpa percobaan tetap muncul")
	}
}
