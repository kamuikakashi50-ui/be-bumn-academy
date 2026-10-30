package service

import (
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/fingerprint"
)

// Uji kepatuhan bank soal: klasifikasi kemiripan, pengabaian grup varian, dan
// pemilihan varian. Semuanya murni perhitungan, tanpa database.

func fingerprintDari(questionID, text, group string) complianceFingerprint {
	item := complianceFingerprint{
		questionID: questionID,
		bankKey:    "numerik",
		text:       text,
		hash:       fingerprint.Hash(text),
		template:   fingerprint.TemplateHash(text),
		tokens:     fingerprint.Tokens(text),
	}
	if group != "" {
		item.group = group
	}
	return item
}

func TestClassifyMenemukanSoalKembarPersis(t *testing.T) {
	left := fingerprintDari("q-1", "Berapa hasil dari 12 + 7?", "")
	right := fingerprintDari("q-2", "berapa hasil dari 12+7", "")

	kind, similarity := classify(left, right, fingerprint.DefaultThreshold)
	if kind != dto.DuplicateIdentical {
		t.Errorf("kind = %q, harap %q", kind, dto.DuplicateIdentical)
	}
	if similarity != 1 {
		t.Errorf("kemiripan soal identik = %v, harap 1", similarity)
	}
}

func TestClassifyMenandaiSatuKeluargaSoal(t *testing.T) {
	left := fingerprintDari("q-1", "Jika x = 12 dan y = 7, berapa x + y?", "")
	right := fingerprintDari("q-2", "Jika x = 45 dan y = 3, berapa x + y?", "")

	kind, _ := classify(left, right, fingerprint.DefaultThreshold)
	if kind != dto.DuplicateFamily {
		t.Errorf("kind = %q, harap %q", kind, dto.DuplicateFamily)
	}
}

func TestClassifyMenandaiSoalMirip(t *testing.T) {
	left := fingerprintDari("q-1",
		"Seorang pegawai menyelesaikan 30 laporan dalam 5 hari kerja, berapa laporan per hari", "")
	right := fingerprintDari("q-2",
		"Seorang pegawai menyelesaikan 30 laporan dalam 5 hari kerja, berapa laporan setiap hari", "")

	kind, similarity := classify(left, right, fingerprint.DefaultThreshold)
	if kind != dto.DuplicateSimilar {
		t.Errorf("kind = %q, harap %q", kind, dto.DuplicateSimilar)
	}
	if similarity < fingerprint.DefaultThreshold {
		t.Errorf("kemiripan %v di bawah ambang %v", similarity, fingerprint.DefaultThreshold)
	}
}

func TestClassifyMengabaikanSoalBerbedaTopik(t *testing.T) {
	left := fingerprintDari("q-1", "Sebutkan enam nilai inti AKHLAK beserta contohnya", "")
	right := fingerprintDari("q-2", "Hitung luas segitiga dengan alas 10 sentimeter", "")

	kind, similarity := classify(left, right, fingerprint.DefaultThreshold)
	if kind != "" || similarity != 0 {
		t.Errorf("soal berbeda topik tidak boleh ditandai, dapat kind=%q similarity=%v", kind, similarity)
	}
}

func TestSameVariantGroupMengabaikanVarianSah(t *testing.T) {
	left := fingerprintDari("q-1", "Jika x = 12, berapa x + 1?", "grup-a")
	right := fingerprintDari("q-2", "Jika x = 12, berapa x + 1?", "grup-a")
	other := fingerprintDari("q-3", "Jika x = 12, berapa x + 1?", "grup-b")

	if !sameVariantGroup(left, right) {
		t.Error("soal satu grup varian harus dikenali")
	}
	if sameVariantGroup(left, other) {
		t.Error("grup varian berbeda bukan varian sah")
	}
	if sameVariantGroup(left, fingerprintDari("q-4", "Jika x = 12, berapa x + 1?", "")) {
		t.Error("soal tanpa grup tidak boleh dianggap satu varian")
	}
}

func TestCouldExceedThresholdMenolakPasanganTerlaluBerbeda(t *testing.T) {
	if !couldExceedThreshold(10, 10, 0.75) {
		t.Error("jumlah kata sama harus lolos prasaring")
	}
	if couldExceedThreshold(3, 20, 0.75) {
		t.Error("perbandingan 3 vs 20 kata tidak mungkin melewati ambang 0,75")
	}
	if couldExceedThreshold(0, 10, 0.5) {
		t.Error("himpunan kosong tidak boleh lolos prasaring")
	}
}

func TestApplyLicenseSummaryMemilihIzinAktif(t *testing.T) {
	past := time.Now().UTC().AddDate(0, 0, -1)

	var view dto.QuestionView
	applyLicenseSummary(&view, nil)
	if view.LicenseStatus != "kosong" || view.LicenseCount != 0 {
		t.Errorf("soal tanpa izin harus bertanda kosong, dapat %q", view.LicenseStatus)
	}

	view = dto.QuestionView{}
	applyLicenseSummary(&view, []models.QuestionLicense{
		{Scope: models.LicenseVendor, Owner: "Vendor A", Status: models.LicenseActive, ValidUntil: &past},
		{Scope: models.LicenseInternal, Owner: "BUMN Academy", Status: models.LicenseActive},
	})
	if view.LicenseStatus != string(models.LicenseActive) {
		t.Errorf("harus ada izin aktif, dapat %q", view.LicenseStatus)
	}
	if view.LicenseScope != string(models.LicenseInternal) {
		t.Errorf("izin yang dipakai harus yang masih berlaku, dapat %q", view.LicenseScope)
	}
	if view.LicenseCount != 2 {
		t.Errorf("jumlah izin = %d, harap 2", view.LicenseCount)
	}
}
