package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
)

// Uji bentuk query Fase 1 langkah 4 (kohort/kelas khusus & isolasi bank soal)
// memakai GORM DryRun: SQL disusun tanpa koneksi database, sehingga aturan penting
// (keanggotaan aktif, status kelas, masa akses, isolasi soal antar instansi) bisa
// diuji di unit test.

func TestListCohortsQueryMenyaringDanMemuatInstansi(t *testing.T) {
	repo := &CohortRepository{db: newDryRunDB(t)}

	statement := repo.db.Model(&models.Cohort{}).
		Where("org_id = ?", "org-1").
		Where("status = ?", models.CohortAktif).
		Where("sector_key = ?", "jasa_keuangan").
		Preload("Organization").
		Session(&gorm.Session{DryRun: true}).
		Order("starts_at DESC NULLS LAST, name ASC").
		Find(&[]models.Cohort{}).Statement

	sql := statement.SQL.String()
	for _, want := range []string{
		"org_id = ",
		"status = ",
		"sector_key = ",
		`FROM "cohorts"`,
		"ORDER BY starts_at DESC NULLS LAST, name ASC",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query daftar kelas tidak memuat %q:\n%s", want, sql)
		}
	}
	if _, ok := statement.Preloads["Organization"]; !ok {
		t.Errorf("instansi harus di-preload, preload terdaftar: %v", statement.Preloads)
	}
}

func TestActiveCohortsForUserQueryMemeriksaKeanggotaanStatusDanPeriode(t *testing.T) {
	repo := &CohortRepository{db: newDryRunDB(t)}
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	sql := sqlDariSession(t, repo.activeCohortsForUserQuery(context.Background(), "user-1", now))

	for _, want := range []string{
		`FROM "cohorts"`,
		"JOIN cohort_members ON cohort_members.cohort_id = cohorts.id",
		"cohort_members.user_id = ",
		"cohort_members.status = ",
		"cohorts.status = ",
		"cohorts.starts_at IS NULL",
		"cohorts.ends_at IS NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query kelas aktif peserta tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestActiveCohortModuleQueryMenutupAksesSetelahMasaAktif(t *testing.T) {
	repo := &CohortRepository{db: newDryRunDB(t)}
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	sql := sqlDariSession(t, repo.activeCohortModuleQuery(context.Background(), "user-1", now))

	for _, want := range []string{
		`FROM "cohort_curriculum"`,
		"DISTINCT",
		"JOIN cohorts ON cohorts.id = cohort_curriculum.cohort_id",
		"JOIN cohort_members ON cohort_members.cohort_id = cohorts.id",
		"cohort_members.status = ",
		"cohorts.status = ",
		// Masa akses wajib diperiksa: setelah lewat, materi kelas ditutup walau
		// keanggotaan masih aktif.
		"cohorts.access_until IS NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query modul kelas aktif tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestListCohortMembersQueryMemakaiJoinUsers(t *testing.T) {
	repo := &CohortRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.CohortMember{}).
		Select("cohort_members.user_id, users.name, users.email").
		Joins("JOIN users ON users.id = cohort_members.user_id").
		Where("cohort_members.cohort_id = ?", "cohort-1").
		Order("cohort_members.joined_at ASC, users.name ASC"))

	for _, want := range []string{
		`FROM "cohort_members"`,
		"JOIN users ON users.id = cohort_members.user_id",
		"users.name",
		"cohort_members.cohort_id = ",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query daftar anggota kelas tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestPrimaryKeyGabunganKohortMembuatUpsertIdempoten(t *testing.T) {
	// Import ulang daftar peserta/kurikulum harus memperbarui baris yang ada, bukan
	// menggandakannya. Itu hanya mungkin bila kuncinya gabungan:
	//   cohort_members    → (cohort_id, user_id)
	//   cohort_curriculum → (cohort_id, module_id)
	db := newDryRunDB(t)

	cases := []struct {
		table   string
		model   any
		wantPKs []string
	}{
		{"cohort_members", &models.CohortMember{}, []string{"cohort_id", "user_id"}},
		{"cohort_curriculum", &models.CohortCurriculumItem{}, []string{"cohort_id", "module_id"}},
	}

	for _, tc := range cases {
		t.Run(tc.table, func(t *testing.T) {
			statement := &gorm.Statement{DB: db}
			if err := statement.Parse(tc.model); err != nil {
				t.Fatalf("gagal membaca skema model: %v", err)
			}

			got := make([]string, 0, len(statement.Schema.PrimaryFields))
			for _, field := range statement.Schema.PrimaryFields {
				got = append(got, field.DBName)
			}
			if len(got) != len(tc.wantPKs) {
				t.Fatalf("jumlah primary key %s = %v, harap %v", tc.table, got, tc.wantPKs)
			}
			for i, want := range tc.wantPKs {
				if got[i] != want {
					t.Errorf("primary key %s[%d] = %q, harap %q", tc.table, i, got[i], want)
				}
			}
		})
	}
}

func TestQuestionsByBanksQueryMenyaringKepemilikanInstansi(t *testing.T) {
	// Isolasi bank soal antar klien (aturan §4.5) ditegakkan di jalur penyusunan
	// sesi: soal ber-`owner_org_id` hanya boleh muncul pada kelas instansi itu.
	repo := &TryoutRepository{db: newDryRunDB(t)}
	ctx := context.Background()

	shared := sqlDariSession(t, repo.
		questionsByBanksQuery(ctx, "tryout-1", []string{"numerik"}, nil).
		Model(&models.Question{}))
	for _, want := range []string{`FROM "questions"`, "tryout_id = ", "owner_org_id IS NULL"} {
		if !strings.Contains(shared, want) {
			t.Errorf("query soal tanpa konteks instansi tidak memuat %q:\n%s", want, shared)
		}
	}
	if strings.Contains(shared, "owner_org_id IN ") {
		t.Errorf("tanpa konteks instansi tidak boleh ada soal milik instansi lain:\n%s", shared)
	}

	scoped := sqlDariSession(t, repo.
		questionsByBanksQuery(ctx, "tryout-1", []string{"numerik"}, []string{"org-1", "org-2", "org-1", ""}).
		Model(&models.Question{}))
	for _, want := range []string{
		"bank_key IN ",
		// Hanya dua instansi unik yang dikirim ke query: duplikat dan nilai kosong
		// dibuang (GORM memberi dua placeholder).
		"owner_org_id IS NULL OR owner_org_id IN ($4,$5)",
	} {
		if !strings.Contains(scoped, want) {
			t.Errorf("query soal ber-konteks instansi tidak memuat %q:\n%s", want, scoped)
		}
	}

	// Tanpa tryout/bank, tidak ada query yang dijalankan.
	if query := repo.questionsByBanksQuery(ctx, "", []string{"numerik"}, []string{"org-1"}); query != nil {
		t.Error("tryout kosong harus menghasilkan query nil")
	}
	if query := repo.questionsByBanksQuery(ctx, "tryout-1", nil, []string{"org-1"}); query != nil {
		t.Error("daftar bank kosong harus menghasilkan query nil")
	}
}

// Uji regresi tag model: kolom dengan nilai nol yang bermakna (modul opsional,
// kode undangan yang dicabut, instansi yang dinonaktifkan) tidak boleh memakai tag
// `default:` yang bisa diparse Go — GORM akan menimpa nilai nol saat INSERT.
func TestModelKohortTidakMemakaiTagDefaultYangMenimpaNilaiNol(t *testing.T) {
	db := newDryRunDB(t)

	cases := []struct {
		name    string
		model   any
		columns []string
	}{
		{name: "instansi", model: &models.Organization{}, columns: []string{"active"}},
		{name: "kurikulum kelas", model: &models.CohortCurriculumItem{}, columns: []string{"required"}},
		{name: "kode undangan", model: &models.CohortInvite{}, columns: []string{"active"}},
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
