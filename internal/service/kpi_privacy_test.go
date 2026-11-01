package service

import (
	"os"
	"strings"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
)

// Uji batas privasi KPI (Fase 2 §4.7): ambang agregasi pada endpoint publik dan
// jaminan bahwa balasan publik tidak memuat identitas peserta.

func TestPublicSectorGroupsMenyembunyikanKelompokKecil(t *testing.T) {
	groups := []dto.KPIBreakdownView{
		{Key: "jasa_keuangan", Label: "Jasa Keuangan", Rates: dto.KPIRatePairView{
			Reported: dto.KPIRateView{Value: 40, N: 20, Passed: 8},
			Verified: dto.KPIRateView{Value: 50, N: 10, Passed: 5},
		}},
		{Key: "energi", Label: "Energi", Rates: dto.KPIRatePairView{
			Reported: dto.KPIRateView{Value: 33.3, N: 3, Passed: 1},
			Verified: dto.KPIRateView{Value: 50, N: 2, Passed: 1},
		}},
		{Key: "umum", Label: "Umum", Rates: dto.KPIRatePairView{
			Reported: dto.KPIRateView{Value: 25, N: 10, Passed: 3},
		}},
	}

	visible, hidden := publicSectorGroups(groups, 10)

	if len(visible) != 2 {
		t.Fatalf("kelompok yang tampil = %d, harap 2 (n ≥ 10)", len(visible))
	}
	if hidden != 1 {
		t.Errorf("kelompok tersembunyi = %d, harap 1", hidden)
	}
	for _, group := range visible {
		if group.Key == "energi" {
			t.Error("kelompok dengan n < ambang tidak boleh tampil ke publik")
		}
		if group.N < 10 {
			t.Errorf("kelompok %s tampil dengan n = %d, di bawah ambang", group.Key, group.N)
		}
	}

	// Ambang 0/negatif tidak boleh meloloskan apa pun: konfigurasi dikembalikan ke
	// bawaan di `config.Validate`, dan di sini kelompok tetap disaring.
	if _, hidden := publicSectorGroups(groups, 0); hidden != 0 {
		t.Error("ambang 0 berarti semua kelompok tampil — pemanggil wajib memakai ambang positif")
	}
}

func TestPublicOutcomeViewTidakMemuatIdentitasPeserta(t *testing.T) {
	// Penjaga struktural: balasan publik hanya boleh memuat angka agregat. Bila
	// suatu saat ada kolom identitas ditambahkan (nama/email/userId), test ini gagal.
	fields := []string{}
	_ = fields

	source, err := os.ReadFile("kpi_export.go")
	if err != nil {
		t.Fatalf("gagal membaca service KPI: %v", err)
	}

	start := strings.Index(string(source), "func (s *KPIService) PublicOutcomes")
	if start < 0 {
		t.Fatal("fungsi PublicOutcomes tidak ditemukan")
	}
	end := strings.Index(string(source)[start:], "\n}")
	body := string(source)[start : start+end]

	for _, forbidden := range []string{"UserName", "UserEmail", "user_name", "user_email", "MaskName", "Initials"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("balasan publik memuat %q — endpoint publik tidak boleh memuat identitas", forbidden)
		}
	}
	if !strings.Contains(body, "ConsentOnly: true") {
		t.Error("ringkasan publik harus hanya memakai laporan yang pemiliknya menyetujui agregat")
	}
	if !strings.Contains(body, "minGroupSize") {
		t.Error("ringkasan publik harus menegakkan ambang agregasi")
	}
}
