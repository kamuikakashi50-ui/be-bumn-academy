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
