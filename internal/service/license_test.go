package service

import (
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji aturan izin: masa berlaku, validasi periode, dan penolakan soal lisensi
// vendor tanpa nomor perjanjian.

func TestParseOptionalTime(t *testing.T) {
	got, err := parseOptionalTime("2026-12-31")
	if err != nil {
		t.Fatalf("tanggal sederhana harus diterima: %v", err)
	}
	if got == nil || got.Year() != 2026 || got.Month() != time.December || got.Day() != 31 {
		t.Errorf("tanggal terbaca salah: %v", got)
	}

	if parsed, err := parseOptionalTime("   "); err != nil || parsed != nil {
		t.Errorf("string kosong harus menghasilkan nil tanpa error, dapat %v / %v", parsed, err)
	}

	if _, err := parseOptionalTime("31 Desember"); err == nil {
		t.Error("format tanggal tak dikenal harus ditolak")
	}
}

func TestValidateLicensePeriod(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	before := from.AddDate(0, 0, -1)
	after := from.AddDate(1, 0, 0)

	if err := validateLicensePeriod(&from, &after); err != nil {
		t.Errorf("periode wajar tidak boleh error: %v", err)
	}
	if err := validateLicensePeriod(&from, &before); err == nil {
		t.Error("tanggal berakhir sebelum mulai harus ditolak")
	}
	if err := validateLicensePeriod(&from, nil); err != nil {
		t.Errorf("izin tanpa batas waktu harus valid: %v", err)
	}
}

func TestValidateLicenseInputMenolakVendorTanpaNomorPerjanjian(t *testing.T) {
	svc := &LicenseService{}

	if err := svc.ValidateLicenseInput(nil); err != nil {
		t.Errorf("tanpa data izin harus valid (dianggap milik sendiri): %v", err)
	}

	vendor := &dto.QuestionLicenseInput{Scope: string(models.LicenseVendor)}
	if err := svc.ValidateLicenseInput(vendor); err == nil {
		t.Error("lisensi vendor tanpa nomor perjanjian harus ditolak")
	}

	vendor.AgreementNo = "PKS/2026/001"
	if err := svc.ValidateLicenseInput(vendor); err != nil {
		t.Errorf("lisensi vendor lengkap harus diterima: %v", err)
	}

	unknown := &dto.QuestionLicenseInput{Scope: "karangan"}
	if err := svc.ValidateLicenseInput(unknown); err == nil {
		t.Error("cakupan izin tak dikenal harus ditolak")
	}
}

func TestValidateInstrumentMenolakInstrumenBerlisensiTakLengkap(t *testing.T) {
	svc := &LicenseService{}
	psychologist := "11111111-1111-1111-1111-111111111111"

	practice := models.PsychologicalInstrument{Kind: models.InstrumentPractice}
	if err := svc.validateInstrument(practice); err != nil {
		t.Errorf("instrumen latihan tidak butuh lisensi: %v", err)
	}

	incomplete := models.PsychologicalInstrument{Kind: models.InstrumentLicensed}
	if err := svc.validateInstrument(incomplete); err == nil {
		t.Error("instrumen berlisensi tanpa vendor/nomor/psikolog harus ditolak")
	}

	complete := models.PsychologicalInstrument{
		Kind:           models.InstrumentLicensed,
		Vendor:         "Penerbit Alat Tes",
		LicenseNo:      "LIC/2026/001",
		AdministeredBy: &psychologist,
	}
	if err := svc.validateInstrument(complete); err != nil {
		t.Errorf("instrumen berlisensi lengkap harus diterima: %v", err)
	}
}

func TestLicenseStatusOrDanBoolOr(t *testing.T) {
	if got := licenseStatusOr(""); got != models.LicenseActive {
		t.Errorf("status kosong harus aktif, dapat %q", got)
	}
	if got := licenseStatusOr("dicabut"); got != models.LicenseRevoked {
		t.Errorf("status dicabut tidak terbaca, dapat %q", got)
	}

	yes := true
	if !boolOr(&yes, false) {
		t.Error("nilai pointer true harus dipakai")
	}
	if !boolOr(nil, true) {
		t.Error("nilai bawaan harus dipakai saat pointer nil")
	}
	if boolOr(nil, false) {
		t.Error("bawaan false harus dihormati")
	}
}
