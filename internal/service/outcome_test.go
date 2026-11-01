package service

import (
	"strings"
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji aturan tracer study hasil seleksi (Fase 2 §4.1/§4.2) pada bagian keputusan
// yang murni: validasi status & tahun, tingkat verifikasi, reset verifikasi saat
// peserta mengubah laporan, dan alasan penolakan yang wajib diisi.

func TestOutcomeStatusFromInputMenerimaHanyaStatusDikenal(t *testing.T) {
	valid := map[string]models.OutcomeStatus{
		"lolos":             models.OutcomeLolos,
		"gagal":             models.OutcomeGagal,
		"menunggu":          models.OutcomeMenunggu,
		"mengundurkan_diri": models.OutcomeMengundurkanDiri,
		// Spasi di sekitar nilai dirapikan; nilainya sendiri tetap huruf kecil.
		" lolos ": models.OutcomeLolos,
	}

	for input, want := range valid {
		got, err := outcomeStatusFromInput(input)
		if err != nil {
			t.Errorf("outcomeStatusFromInput(%q) error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("outcomeStatusFromInput(%q) = %q, harap %q", input, got, want)
		}
	}

	for _, invalid := range []string{"", "diterima", "LOLOS"} {
		if _, err := outcomeStatusFromInput(invalid); err == nil {
			t.Errorf("status %q harus ditolak (VALIDATION_ERROR)", invalid)
		}
	}
}

func TestSelectionYearValidMengikutiRentangWajar(t *testing.T) {
	for _, year := range []int{2020, 2026, 2100} {
		if !selectionYearValid(year) {
			t.Errorf("tahun %d harus dianggap valid", year)
		}
	}
	for _, year := range []int{0, 1999, 2019, 2101} {
		if selectionYearValid(year) {
			t.Errorf("tahun %d harus ditolak", year)
		}
	}
}

func TestVerifySourceMemilihTingkatVerifikasi(t *testing.T) {
	if got := verifySource(false); got != models.OutcomeAdminVerified {
		t.Errorf("verifikasi biasa = %q, harap %q", got, models.OutcomeAdminVerified)
	}
	if got := verifySource(true); got != models.OutcomeDocumentVerified {
		t.Errorf("verifikasi dengan dokumen = %q, harap %q", got, models.OutcomeDocumentVerified)
	}
	if !verifySource(true).Verified() || !verifySource(false).Verified() {
		t.Error("kedua tingkat verifikasi harus dihitung sebagai laporan terverifikasi")
	}
}

func TestResetVerifikasiSetelahPesertaMengubahLaporan(t *testing.T) {
	actor := "admin-1"
	verifiedAt := time.Now().UTC()

	outcome := models.SelectionOutcome{
		Source:     models.OutcomeAdminVerified,
		VerifiedBy: &actor,
		VerifiedAt: &verifiedAt,
	}

	if !resetVerificationAfterParticipantEdit(&outcome) {
		t.Fatal("laporan terverifikasi yang diubah peserta harus dikembalikan ke belum terverifikasi")
	}
	if outcome.Source != models.OutcomeSelfReported {
		t.Errorf("source = %q, harap %q", outcome.Source, models.OutcomeSelfReported)
	}
	if outcome.VerifiedBy != nil || outcome.VerifiedAt != nil {
		t.Error("jejak verifikasi harus dikosongkan agar laporan kembali ke daftar tinjauan")
	}
	if outcome.Verified() {
		t.Error("laporan yang diubah peserta tidak boleh dihitung sebagai terverifikasi")
	}

	// Laporan yang belum pernah diverifikasi tidak berubah.
	fresh := models.SelectionOutcome{Source: models.OutcomeSelfReported}
	if resetVerificationAfterParticipantEdit(&fresh) {
		t.Error("laporan yang belum terverifikasi tidak perlu direset")
	}
	if fresh.Source != models.OutcomeSelfReported {
		t.Errorf("source laporan baru berubah menjadi %q", fresh.Source)
	}
}

func TestValidateRejectReasonMewajibkanAlasan(t *testing.T) {
	if _, err := validateRejectReason("  "); err == nil {
		t.Error("alasan penolakan kosong harus ditolak")
	}
	if _, err := validateRejectReason("no"); err == nil {
		t.Error("alasan terlalu pendek harus ditolak")
	}

	reason, err := validateRejectReason("  Bukti pengumuman tidak terbaca.  ")
	if err != nil {
		t.Fatalf("alasan valid ditolak: %v", err)
	}
	if strings.TrimSpace(reason) != reason || reason == "" {
		t.Errorf("alasan harus dirapikan, dapat %q", reason)
	}
}

func TestParseDateOnlyMenerimaTanggalDanKosong(t *testing.T) {
	parsed, err := parseDateOnly("2026-05-12")
	if err != nil {
		t.Fatalf("parseDateOnly error: %v", err)
	}
	if parsed == nil || parsed.Format("2006-01-02") != "2026-05-12" {
		t.Errorf("tanggal = %v, harap 2026-05-12", parsed)
	}

	// Tanggal pengumuman opsional: peserta boleh melapor sebelum pengumuman resmi.
	empty, err := parseDateOnly("   ")
	if err != nil {
		t.Fatalf("nilai kosong tidak boleh menjadi error: %v", err)
	}
	if empty != nil {
		t.Errorf("nilai kosong harus menghasilkan nil, dapat %v", empty)
	}

	if _, err := parseDateOnly("12/05/2026"); err == nil {
		t.Error("format tanggal selain YYYY-MM-DD harus ditolak")
	}
}

func TestValidateEvidenceFileMenerimaGambarDanPDF(t *testing.T) {
	for _, name := range []string{"bukti.png", "bukti.JPG", "bukti.jpeg", "bukti.webp", "pengumuman.pdf"} {
		if err := validateEvidenceFile(name, 1024); err != nil {
			t.Errorf("%s harus diterima, dapat error: %v", name, err)
		}
	}

	if err := validateEvidenceFile("bukti.exe", 1024); err == nil {
		t.Error("jenis berkas di luar daftar harus ditolak")
	}
	if err := validateEvidenceFile("bukti.png", 0); err == nil {
		t.Error("berkas kosong harus ditolak")
	}
	if err := validateEvidenceFile("bukti.png", maxEvidenceBytes+1); err == nil {
		t.Error("berkas melebihi batas harus ditolak")
	}
}
