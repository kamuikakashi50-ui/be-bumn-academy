package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji keselarasan skema psikolog & konsultasi (Fase 3 langkah 6–7).

const psychologistMigrationUp = "000020_psychologist.up.sql"
const psychologistMigrationDown = "000020_psychologist.down.sql"

func TestSkemaModelPsikologSelarasDenganMigrasi(t *testing.T) {
	migration := readMigration(t, psychologistMigrationUp)
	db := newDryRunDB(t)

	cases := []struct {
		table string
		model any
	}{
		{table: "psychologist_partners", model: &models.PsychologistPartner{}},
		{table: "consultation_slots", model: &models.ConsultationSlot{}},
		{table: "consultation_bookings", model: &models.ConsultationBooking{}},
		{table: "consultation_notes", model: &models.ConsultationNote{}},
		{table: "assessment_access_logs", model: &models.AssessmentAccessLog{}},
		{table: "assessment_disputes", model: &models.AssessmentDispute{}},
		{table: "instrument_interpretations", model: &models.InstrumentInterpretation{}},
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

func TestRollbackPsikologMembalikkanSeluruhPerubahan(t *testing.T) {
	down := readMigration(t, psychologistMigrationDown)

	for _, table := range []string{
		"instrument_interpretations", "assessment_disputes", "assessment_access_logs",
		"consultation_notes", "consultation_bookings", "consultation_slots",
		"psychologist_partners",
	} {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Errorf("rollback tidak menghapus tabel %s", table)
		}
	}
	// Urutan harus terbalik dari pembuatan: mitra paling akhir dibuat → paling
	// awal dihapus, agar foreign key tidak menghalangi.
	if strings.Index(down, "DROP TABLE IF EXISTS consultation_slots") >
		strings.Index(down, "DROP TABLE IF EXISTS psychologist_partners") {
		t.Error("rollback menghapus `psychologist_partners` sebelum tabel yang merujuknya")
	}
	// Peran baru harus dinormalkan sebelum constraint lama dipasang ulang,
	// kalau tidak `down` gagal saat ada akun berperan psikolog.
	if !strings.Contains(down, "UPDATE users SET role = 'peserta'") {
		t.Error("rollback tidak menormalkan peran psikolog/admin_instansi sebelum memasang constraint lama")
	}
	if !strings.Contains(down, "CHECK (role IN ('peserta', 'admin'))") {
		t.Error("rollback tidak mengembalikan constraint peran semula")
	}
}

// TestMigrasiPsikologMenegakkanNilaiTerbatas menjaga aturan yang mudah rusak:
// status verifikasi, visibilitas catatan, status pemesanan, jenis resource &
// aksi jejak akses, serta peran pengguna.
func TestMigrasiPsikologMenegakkanNilaiTerbatas(t *testing.T) {
	migration := readMigration(t, psychologistMigrationUp)

	for _, constraint := range []string{
		"psychologist_status_check",
		"consultation_slots_mode_check",
		"consultation_slots_window_check",
		"consultation_slots_quota_check",
		"consultation_slots_booked_check",
		"consultation_bookings_status_check",
		"consultation_notes_visibility_check",
		"assessment_access_logs_resource_check",
		"assessment_access_logs_action_check",
		"assessment_disputes_status_check",
	} {
		if !strings.Contains(migration, constraint) {
			t.Errorf("migrasi tidak memuat constraint %s", constraint)
		}
	}
	if !strings.Contains(migration, "CHECK (role IN ('peserta', 'admin', 'psikolog', 'admin_instansi'))") {
		t.Error("migrasi tidak memperluas peran pengguna")
	}
	// Kuota tidak boleh dilampaui di level database (jaring terakhir §4.5).
	if !strings.Contains(migration, "booked <= quota") {
		t.Error("migrasi tidak menjaga `booked <= quota`")
	}
	// Satu peserta satu slot **aktif** (double booking dicegah database lewat
	// unique index parsial; pemesanan yang sudah dibatalkan tidak menghalangi).
	if !strings.Contains(migration, "consultation_bookings_active_unique") {
		t.Error("migrasi tidak mencegah pemesanan ganda aktif pada satu slot")
	}
	if !strings.Contains(migration, "WHERE status <> 'batal'") {
		t.Error("unique index pemesanan harus parsial (mengabaikan status batal)")
	}
}
