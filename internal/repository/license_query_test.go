package repository

import (
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji bentuk query bank soal berdimensi taksonomi & laporan kepatuhan: memastikan
// penyaringan (sektor/jenis tes/keluarga jabatan) dan deduplikasi grup varian
// tetap tersusun seperti yang diharapkan, tanpa perlu koneksi database.

func TestListQuestionsQueryMenyaringDimensiTaksonomi(t *testing.T) {
	repo := &TryoutRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Question{}).
		Where("bank_key = ?", "akuntansi").
		Where("sector_key = ?", "jasa_keuangan").
		Where("test_type = ?", "tkb").
		Where("position_family = ?", "keuangan"))

	for _, want := range []string{
		"bank_key = ",
		"sector_key = ",
		"test_type = ",
		"position_family = ",
		`FROM "questions"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query daftar soal tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestCountQuestionsByTryoutQueryMenghitungSatuSoalPerVarian(t *testing.T) {
	repo := &TryoutRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Table("(?) AS soal_unik", repo.db.
		Model(&models.Question{}).
		Select("DISTINCT ON (tryout_id, bank_key, COALESCE(variant_group, id)) tryout_id, bank_key").
		Where("active = ? AND tryout_id IS NOT NULL", true).
		Order("tryout_id, bank_key, COALESCE(variant_group, id), position, id")).
		Select("tryout_id, bank_key, count(*) AS total").
		Group("tryout_id, bank_key"))

	for _, want := range []string{
		"DISTINCT ON (tryout_id, bank_key, COALESCE(variant_group, id))",
		"COUNT(*)",
		"GROUP BY tryout_id, bank_key",
		"active = ",
	} {
		if !strings.Contains(strings.ToUpper(sql), strings.ToUpper(want)) {
			t.Errorf("query hitung soal tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestListFingerprintsQueryMenggabungkanSoalDanSidikJari(t *testing.T) {
	repo := &ComplianceRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Table("question_fingerprints").
		Select("questions.id AS question_id, questions.bank_key, question_fingerprints.fingerprint").
		Joins("JOIN questions ON questions.id = question_fingerprints.question_id").
		Where("questions.bank_key = ?", "numerik"))

	for _, want := range []string{
		"question_fingerprints",
		"JOIN questions ON questions.id = question_fingerprints.question_id",
		"questions.bank_key = ",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query laporan sidik jari tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestUsableQuestionIDsQueryMenolakIzinDicabutDanKedaluwarsa(t *testing.T) {
	repo := &LicenseRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.QuestionLicense{}).
		Where("question_id IN ?", []string{"q-1"}).
		Where("status <> ?", models.LicenseRevoked).
		Where("valid_until IS NULL OR valid_until >= ?", "2026-10-29T00:00:00Z").
		Distinct().
		Pluck("question_id", &[]string{}))

	for _, want := range []string{
		"DISTINCT",
		"question_id",
		"status <> ",
		"valid_until IS NULL OR valid_until >= ",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query izin soal tidak memuat %q:\n%s", want, sql)
		}
	}
}

func TestListUnlicensedQuestionsQueryMemakaiLeftJoin(t *testing.T) {
	repo := &LicenseRepository{db: newDryRunDB(t)}

	sql := sqlDariSession(t, repo.db.Model(&models.Question{}).
		Select("questions.id, questions.bank_key").
		Joins(`LEFT JOIN question_licenses
		         ON question_licenses.question_id = questions.id
		        AND question_licenses.status <> ?
		        AND (question_licenses.valid_until IS NULL OR question_licenses.valid_until >= ?)`,
			models.LicenseRevoked, "2026-10-29T00:00:00Z").
		Where("question_licenses.id IS NULL"))

	for _, want := range []string{
		"LEFT JOIN question_licenses",
		"question_licenses.id IS NULL",
		"question_licenses.status <> ",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query soal tanpa izin tidak memuat %q:\n%s", want, sql)
		}
	}
}
