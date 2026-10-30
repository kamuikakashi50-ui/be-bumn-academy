package certificate

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// Uji generator PDF sertifikat: berkas harus PDF yang sah, memuat identitas
// peserta & jalur, dan menandai sertifikat yang dicabut.

func demoDocument() Document {
	return Document{
		Number:        "BA-2026-000001",
		Participant:   "Dewi Lestari",
		PathTitle:     "Jalur TKD Dasar",
		TestTypeLabel: "Tes Kompetensi Dasar",
		SectorLabel:   "Umum / Lintas Klaster",
		IssuedAt:      time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC),
		VerifyURL:     "https://bumnacademy.id/verifikasi/BA-2026-000001",
	}
}

func TestRenderMenghasilkanPDFLanskapSah(t *testing.T) {
	data := Render(demoDocument())

	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) {
		t.Fatalf("berkas bukan PDF 1.4: %q", data[:min(12, len(data))])
	}
	if !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		t.Error("berkas tidak diakhiri penanda EOF PDF")
	}

	raw := string(data)
	for _, want := range []string{
		"/MediaBox [0 0 842 595]", // lanskap A4
		"startxref",
		"SERTIFIKAT PENYELESAIAN",
		"BA-2026-000001",
		"DEWI LESTARI",
		"Jalur TKD Dasar",
		"30 Oktober 2026",
		"verifikasi/BA-2026-000001",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("berkas tidak memuat %q", want)
		}
	}
}

func TestRenderMenandaiSertifikatDicabutDanInstansi(t *testing.T) {
	doc := demoDocument()
	doc.Revoked = true
	doc.Organization = "Bank Nusantara"

	raw := string(Render(doc))
	if !strings.Contains(raw, "DICABUT") {
		t.Error("sertifikat yang dicabut harus memuat penanda DICABUT")
	}
	if !strings.Contains(raw, "Bank Nusantara") {
		t.Error("sertifikat kelas khusus harus memuat nama instansi")
	}
}

func TestFormatDateIDMemakaiNamaBulanIndonesia(t *testing.T) {
	// Tanggal dikonversi ke WIB lebih dulu, sehingga 23:30 UTC 31 Desember 2025
	// tercetak sebagai 1 Januari 2026 — bukan kesalahan, melainkan waktu Indonesia.
	cases := []struct {
		value time.Time
		want  string
	}{
		{value: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), want: "1 Januari 2026"},
		{value: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC), want: "30 Oktober 2026"},
		{value: time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC), want: "31 Desember 2025"},
		// Waktu WIB sudah masuk hari berikutnya.
		{value: time.Date(2025, 12, 31, 23, 30, 0, 0, time.UTC), want: "1 Januari 2026"},
	}

	for _, tc := range cases {
		if got := formatDateID(tc.value); got != tc.want {
			t.Errorf("formatDateID(%s) = %q, harap %q", tc.value.Format(time.RFC3339), got, tc.want)
		}
	}

	if got := formatDateID(time.Time{}); got != "-" {
		t.Errorf("tanggal kosong harus menjadi \"-\", dapat %q", got)
	}
}
