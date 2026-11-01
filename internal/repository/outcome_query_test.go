package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji bentuk query repository KPI (Fase 2 langkah 1) memakai GORM DryRun: SQL
// disusun tanpa koneksi database, sehingga filter dasbor (tahun, klaster, kelas,
// program, tingkat verifikasi) bisa dipastikan benar-benar masuk ke query.

func TestListOutcomesQueryMenautkanRelasiDanFilter(t *testing.T) {
	repo := &OutcomeRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.outcomeListQuery(context.Background(), OutcomeFilter{
		Year:      2026,
		SectorKey: "energi",
		CohortID:  "cohort-1",
		Program:   "Program Reguler",
		StageKey:  "offering",
		Source:    "verified",
		Search:    "budi",
	}))

	for _, want := range []string{
		`FROM "selection_outcomes"`,
		"JOIN users ON users.id = selection_outcomes.user_id",
		"LEFT JOIN cohorts ON cohorts.id = selection_outcomes.cohort_id",
		"JOIN selection_stages ON selection_stages.key = selection_outcomes.stage_key",
		"selection_outcomes.selection_year = ",
		"selection_outcomes.company_sector = ",
		"selection_outcomes.cohort_id = ",
		"selection_outcomes.stage_key = ",
		"users.program = ",
		// `verified` menyaring dua sumber sekaligus, bukan satu nilai source.
		"selection_outcomes.source <> ",
		"LOWER(users.name) LIKE",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query daftar laporan tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestOutcomeFilterPendingMenyaringLaporanBelumTerverifikasi(t *testing.T) {
	repo := &OutcomeRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.outcomeListQuery(context.Background(), OutcomeFilter{
		OnlyPending: true,
	}))

	if !strings.Contains(sql, "selection_outcomes.source = ") {
		t.Errorf("daftar tinjauan harus menyaring laporan yang belum diverifikasi:\n%s", sql)
	}
	if strings.Contains(sql, "source <> ") {
		t.Errorf("daftar tinjauan tidak boleh memakai penyaring `verified`:\n%s", sql)
	}
}

func TestOutcomeFilterDilaporkanSendiriMemakaiKesamaanSource(t *testing.T) {
	repo := &OutcomeRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.outcomeListQuery(context.Background(), OutcomeFilter{
		Source: string(models.OutcomeSelfReported),
	}))

	if !strings.Contains(sql, "selection_outcomes.source = ") {
		t.Errorf("filter `self_reported` harus memakai kesamaan source:\n%s", sql)
	}
}

func TestFindTargetByKeyMemakaiIsNullUntukKelompokKosong(t *testing.T) {
	repo := &OutcomeRepository{db: newDryRunDB(t)}

	// Kelompok lintas program (tanpa instansi) dan metric non-tahap:
	// perbandingannya harus `IS NULL`, bukan `= ''`.
	query := repo.db.WithContext(context.Background()).Model(&models.KPITarget{})
	query = whereNullableUUID(query, "org_id", "")
	query = whereNullableUUID(query, "cohort_id", "cohort-1")
	query = whereNullableText(query, "stage_key", "")

	sql := sqlDariSession(t, query)
	for _, want := range []string{"org_id IS NULL", "cohort_id = ", "stage_key IS NULL"} {
		if !strings.Contains(sql, want) {
			t.Errorf("query target KPI tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestKPIFilterMenyusunKondisiLaporanDanPeserta(t *testing.T) {
	filter := KPIFilter{
		Year:        2026,
		SectorKey:   "jasa_keuangan",
		CohortID:    "cohort-1",
		Program:     "Kelas Intensif BUMN",
		ConsentOnly: true,
	}

	outcomeSQL, outcomeArgs := filter.outcomeConditions()
	for _, want := range []string{
		"o.selection_year = ?",
		"o.company_sector = ?",
		"o.cohort_id = ?",
		// Endpoint publik hanya memakai laporan yang pemiliknya menyetujui agregat
		// (tanpa placeholder karena kolomnya boolean).
		"o.aggregate_consent",
	} {
		if !strings.Contains(outcomeSQL, want) {
			t.Errorf("kondisi laporan tidak memuat %q: %s", want, outcomeSQL)
		}
	}
	if len(outcomeArgs) != 3 {
		t.Errorf("jumlah argumen kondisi laporan = %d, harap 3 (tahun, sektor, kelas)", len(outcomeArgs))
	}

	participantSQL, participantArgs := filter.participantConditions()
	for _, want := range []string{"u.program = ?", "FROM cohort_members cm"} {
		if !strings.Contains(participantSQL, want) {
			t.Errorf("kondisi peserta tidak memuat %q: %s", want, participantSQL)
		}
	}
	if len(participantArgs) != 2 {
		t.Errorf("jumlah argumen kondisi peserta = %d, harap 2 (program, keanggotaan kelas)", len(participantArgs))
	}
}

func TestKPIFilterFinalStageTidakMenyaringTahap(t *testing.T) {
	filter := KPIFilter{Year: 2026, StageKey: "tahap1_tkd"}

	scoped := filter.finalStageFilter()
	if scoped.StageKey != "" {
		t.Errorf("filter tahap akhir harus melepas penyaring tahap, dapat %q", scoped.StageKey)
	}
	if scoped.Year != filter.Year {
		t.Error("penyaring lain harus tetap dipertahankan")
	}
	if filter.StageKey != "tahap1_tkd" {
		t.Error("salinan tidak boleh mengubah filter aslinya")
	}
}

func TestKPIBreakdownMenolakDimensiTakDikenal(t *testing.T) {
	repo := &OutcomeRepository{db: newDryRunDB(t)}

	if _, err := repo.KPIBreakdown(context.Background(), KPIFilter{}, "instansi", "offering"); err == nil {
		t.Error("dimensi kelompok tak dikenal harus ditolak, bukan menghasilkan query kosong")
	}
}

func TestOutcomeListRowMemetakanKolomGabungan(t *testing.T) {
	db := newDryRunDB(t)

	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&OutcomeListRow{}); err != nil {
		t.Fatalf("gagal membaca skema baris daftar laporan: %v", err)
	}

	// Kolom relasi harus bisa dipindai dari query (dipakai `Scan`), termasuk
	// kolom model yang di-embed.
	for _, column := range []string{"user_name", "user_email", "user_program", "cohort_name", "stage_label", "stage_final", "status", "selection_year"} {
		if statement.Schema.LookUpField(column) == nil {
			t.Errorf("kolom %q tidak dipetakan OutcomeListRow", column)
		}
	}
}
