// Package certificate merender sertifikat penyelesaian jalur belajar sebagai PDF
// lanskap A4 yang sah (tanpa dependency eksternal).
//
// Berkasnya memuat: nomor sertifikat, nama peserta, jalur yang diselesaikan,
// tanggal terbit (format Indonesia), instansi (bila sertifikat kelas khusus),
// tautan verifikasi publik, dan penanda "DICABUT" untuk sertifikat yang dicabut.
// Seluruh teks memakai font standar PDF (Helvetica) sehingga tidak perlu
// embedding font; pengukuran lebar & escape teks dipinjam dari paket `ebook` agar
// tabel lebar font hanya ada di satu tempat.
package certificate

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/pkg/ebook"
)

// Ukuran halaman A4 lanskap (satuan point: 72 point = 1 inci).
const (
	pageWidth  = 842.0
	pageHeight = 595.0
	// Bingkai luar & dalam.
	borderOuter = 24.0
	borderInner = 33.0
)

// Document adalah isi sertifikat yang akan dirender.
type Document struct {
	Number      string
	Participant string
	PathTitle   string
	// TestTypeLabel & SectorLabel melengkapi keterangan jalur (opsional).
	TestTypeLabel string
	SectorLabel   string
	IssuedAt      time.Time
	// Organization adalah nama instansi untuk sertifikat kelas khusus (opsional).
	Organization string
	// VerifyURL adalah tautan verifikasi publik yang dicetak di kaki sertifikat.
	VerifyURL string
	// Revoked menambahkan penanda "DICABUT" pada sertifikat yang dicabut.
	Revoked bool
}

// Render menghasilkan berkas PDF sertifikat.
func Render(doc Document) []byte {
	stream := content(doc)

	var (
		buf     bytes.Buffer
		offsets []int
	)

	const (
		fontRegularID = 3
		fontBoldID    = 4
		contentID     = 5
		pageID        = 6
	)

	buf.WriteString("%PDF-1.4\n")
	// Komentar biner agar berkas dikenali sebagai biner oleh alat lain.
	buf.Write([]byte{'%', 0xE2, 0xE3, 0xCF, 0xD3, '\n'})

	writeObject := func(id int, body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1 >>", pageID))
	writeObject(fontRegularID, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	writeObject(fontBoldID, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	writeObject(contentID, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	writeObject(pageID, fmt.Sprintf(
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
		pageWidth, pageHeight, fontRegularID, fontBoldID, contentID,
	))

	xrefOffset := buf.Len()
	total := len(offsets) + 1
	fmt.Fprintf(&buf, "xref\n0 %d\n", total)
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", total, xrefOffset)

	return buf.Bytes()
}

// content menyusun operator gambar seluruh halaman.
func content(doc Document) string {
	var buf strings.Builder

	// Bingkai luar (tebal) & dalam (tipis) dengan warna emas.
	fmt.Fprintf(&buf, "0.72 0.53 0.04 RG 2.4 w %.1f %.1f %.1f %.1f re S\n",
		borderOuter, borderOuter, pageWidth-borderOuter*2, pageHeight-borderOuter*2)
	fmt.Fprintf(&buf, "0.85 0.78 0.55 RG 0.7 w %.1f %.1f %.1f %.1f re S\n",
		borderInner, borderInner, pageWidth-borderInner*2, pageHeight-borderInner*2)

	center(&buf, "BUMN ACADEMY", 11, true, 0.42, 0.46, 0.53, 512)
	fmt.Fprintf(&buf, "0.72 0.53 0.04 RG 1 w %.1f 494 m %.1f 494 l S\n", pageWidth/2-40, pageWidth/2+40)

	center(&buf, "SERTIFIKAT PENYELESAIAN", 27, true, 0.11, 0.15, 0.24, 452)
	center(&buf, "Diberikan kepada", 11.5, false, 0.35, 0.39, 0.46, 412)
	center(&buf, strings.ToUpper(doc.Participant), 24, true, 0.08, 0.11, 0.19, 374)

	description := fmt.Sprintf(
		"atas penyelesaian seluruh langkah wajib pada jalur belajar \u201c%s\u201d%s",
		doc.PathTitle, labelSuffix(doc.TestTypeLabel, doc.SectorLabel),
	)
	y := 336.0
	for _, wrapped := range wrapCentered(description, 11.5, false, pageWidth-220) {
		center(&buf, wrapped, 11.5, false, 0.28, 0.32, 0.39, y)
		y -= 17
	}

	if strings.TrimSpace(doc.Organization) != "" {
		center(&buf, "Diselenggarakan untuk "+strings.TrimSpace(doc.Organization), 11, true, 0.35, 0.39, 0.46, y-8)
	}

	// Kaki sertifikat: nomor (kiri), tanggal (tengah), tautan verifikasi (kanan).
	footerY := borderInner + 52
	center(&buf, "Diterbitkan "+formatDateID(doc.IssuedAt), 10, false, 0.35, 0.39, 0.46, footerY)
	left(&buf, "Nomor: "+doc.Number, 10, true, 0.28, 0.32, 0.39, borderInner+34)
	if doc.VerifyURL != "" {
		right(&buf, "Verifikasi: "+doc.VerifyURL, 9, false, 0.42, 0.46, 0.53, borderInner+34)
	}

	center(&buf, "BUMN Academy \u00b7 Tim Akademik", 9.5, false, 0.55, 0.58, 0.63, borderInner+16)

	if doc.Revoked {
		// Teks miring sebagai penanda bahwa sertifikat tidak lagi berlaku.
		fmt.Fprintf(&buf, "BT /F2 58 Tf 0.83 0.15 0.15 rg 0.707 0.707 -0.707 0.707 %.1f %.1f Tm (%s) Tj ET\n",
			pageWidth/2-140, pageHeight/2-40, ebook.EscapeText("DICABUT"))
	}

	return buf.String()
}

// labelSuffix menyusun keterangan tambahan jalur (jenis tes & klaster).
func labelSuffix(testType, sector string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(testType) != "" {
		parts = append(parts, "jenis tes "+strings.TrimSpace(testType))
	}
	if strings.TrimSpace(sector) != "" {
		parts = append(parts, "klaster "+strings.TrimSpace(sector))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// center menulis teks terpusat secara horizontal pada koordinat y.
func center(buf *strings.Builder, text string, size float64, bold bool, r, g, b, y float64) {
	width := ebook.MeasureText(text, size, bold)
	textAt(buf, text, size, bold, r, g, b, (pageWidth-width)/2, y)
}

// left menulis teks rata kiri pada bingkai dalam.
func left(buf *strings.Builder, text string, size float64, bold bool, r, g, b, y float64) {
	textAt(buf, text, size, bold, r, g, b, borderInner+10, y)
}

// right menulis teks rata kanan pada bingkai dalam.
func right(buf *strings.Builder, text string, size float64, bold bool, r, g, b, y float64) {
	width := ebook.MeasureText(text, size, bold)
	textAt(buf, text, size, bold, r, g, b, pageWidth-borderInner-10-width, y)
}

// textAt menulis satu baris teks pada koordinat tertentu.
func textAt(buf *strings.Builder, text string, size float64, bold bool, r, g, b, x, y float64) {
	if strings.TrimSpace(text) == "" {
		return
	}
	font := "/F1"
	if bold {
		font = "/F2"
	}
	fmt.Fprintf(buf, "BT %s %.1f Tf %.2f %.2f %.2f rg 1 0 0 1 %.1f %.1f Tm (%s) Tj ET\n",
		font, size, r, g, b, x, y, ebook.EscapeText(text))
}

// wrapCentered membungkus teks menjadi baris-baris selebar maxWidth.
func wrapCentered(text string, size float64, bold bool, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	out := make([]string, 0, 2)
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if ebook.MeasureText(candidate, size, bold) <= maxWidth {
			current = candidate
			continue
		}
		out = append(out, current)
		current = word
	}
	return append(out, current)
}

// bulanID adalah nama bulan Indonesia untuk tanggal pada sertifikat.
var bulanID = [...]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// formatDateID membentuk tanggal Indonesia, mis. "30 Oktober 2026".
func formatDateID(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	local := value.In(time.FixedZone("WIB", 7*60*60))
	return fmt.Sprintf("%d %s %d", local.Day(), bulanID[int(local.Month())-1], local.Year())
}
