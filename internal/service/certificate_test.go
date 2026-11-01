package service

import (
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji bagian murni sertifikat: status, pembacaan metadata, dan pencocokan modul
// pada jalur. Pemeriksaan kelayakan (langkah + kuis) memerlukan data nyata dan
// diuji lewat smoke test end-to-end.

func TestCertificateStatusMenerjemahkanPencabutan(t *testing.T) {
	now := time.Now().UTC()

	if got := certificateStatus(&models.Certificate{}); got != "sah" {
		t.Errorf("sertifikat tanpa revoked_at harus \"sah\", dapat %q", got)
	}
	if got := certificateStatus(&models.Certificate{RevokedAt: &now}); got != "dicabut" {
		t.Errorf("sertifikat dengan revoked_at harus \"dicabut\", dapat %q", got)
	}
}

func TestMetadataStringMengambilNilaiTeks(t *testing.T) {
	metadata := models.JSONMap{
		"participantName": "Dewi Lestari",
		"organization":    "  Bank Nusantara  ",
		"attempts":        float64(3),
	}

	if got := metadataString(metadata, "participantName"); got != "Dewi Lestari" {
		t.Errorf("participantName = %q", got)
	}
	if got := metadataString(metadata, "organization"); got != "Bank Nusantara" {
		t.Errorf("organization harus dirapikan dari spasi: %q", got)
	}
	if got := metadataString(metadata, "attempts"); got != "" {
		t.Errorf("nilai non-teks harus diabaikan, dapat %q", got)
	}
	if got := metadataString(metadata, "tidak-ada"); got != "" {
		t.Errorf("kunci yang tidak ada harus kosong, dapat %q", got)
	}
	if got := metadataString(nil, "organization"); got != "" {
		t.Errorf("metadata kosong harus menghasilkan string kosong, dapat %q", got)
	}
}

func TestPathHasModuleMenandaiJalurYangMemuatModul(t *testing.T) {
	path := models.LearningPath{Steps: []models.LearningPathStep{
		{ModuleID: "mod-intensif-numerik"},
		{ModuleID: "mod-intensif-verbal"},
	}}

	if !pathHasModule(path, "mod-intensif-verbal") {
		t.Error("jalur harus dikenali memuat modul yang ada pada langkahnya")
	}
	if pathHasModule(path, "mod-intensif-inggris") {
		t.Error("modul di luar langkah jalur tidak boleh dianggap ada")
	}
	if pathHasModule(models.LearningPath{}, "mod-intensif-numerik") {
		t.Error("jalur tanpa langkah tidak memuat modul apa pun")
	}
}

// Uji metadata sertifikat: peserta kelas khusus harus membawa nama kelas &
// instansinya, sehingga sertifikat kelas bisa menyebut instansi tanpa menebak dari
// data lain (§4.3).
func TestCertificateMetadataMemuatKelasDanInstansi(t *testing.T) {
	path := &models.LearningPath{ID: "path-1", Title: "Jalur TKD Dasar", SectorKey: "jasa_keuangan", TestType: "tkd"}
	issuedAt := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

	// Tanpa kelas khusus: metadata lama tetap lengkap, tanpa nama instansi.
	plain := certificateMetadata("Peserta Demo", path, issuedAt, nil)
	if plain["participantName"] != "Peserta Demo" || plain["pathTitle"] != "Jalur TKD Dasar" {
		t.Errorf("metadata dasar tidak lengkap: %v", plain)
	}
	if _, ok := plain["organization"]; ok {
		t.Error("peserta tanpa kelas khusus tidak boleh punya instansi di metadata")
	}
	if plain["issuedDate"] != "2026-11-20" {
		t.Errorf("tanggal terbit harus format WIB: %v", plain["issuedDate"])
	}

	// Dengan kelas khusus: nama kelas & instansi ikut tersimpan.
	cohort := &models.Cohort{
		ID: "c-1", Name: "Kelas Khusus TKD Batch 1",
		Organization: &models.Organization{ID: "org-1", Name: "Bank Nusantara"},
	}
	withCohort := certificateMetadata("Peserta Demo", path, issuedAt, cohort)
	if withCohort["cohortName"] != "Kelas Khusus TKD Batch 1" {
		t.Errorf("nama kelas tidak tersimpan: %v", withCohort["cohortName"])
	}
	if withCohort["organization"] != "Bank Nusantara" {
		t.Errorf("nama instansi tidak tersimpan: %v", withCohort["organization"])
	}

	// Kelas tanpa instansi tetap aman (tidak menulis instansi kosong).
	internal := certificateMetadata("Peserta Demo", path, issuedAt, &models.Cohort{ID: "c-2", Name: "Kelas Internal"})
	if _, ok := internal["organization"]; ok {
		t.Error("kelas tanpa instansi tidak boleh mengisi kolom instansi")
	}
}
