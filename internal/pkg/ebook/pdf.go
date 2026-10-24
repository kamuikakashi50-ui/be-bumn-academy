// Package ebook membuat berkas PDF sederhana (tanpa dependency eksternal).
//
// Dipakai ruang kelas digital untuk menghasilkan e-book asli dari konten
// pelajaran: berkasnya PDF 1.4 yang sah (dapat dibuka di peramban, Preview,
// maupun Adobe Reader), berhalaman banyak, dengan judul, paragraf terbungkus
// rapi, subjudul, butir daftar, dan nomor halaman.
//
// Generator ini dibatasi pada font standar PDF (Helvetica) sehingga tidak
// memerlukan font embedding.
package ebook

import (
	"bytes"
	"fmt"
	"strings"
)

// Ukuran halaman A4 (satuan point: 72 point = 1 inci).
const (
	pageWidth  = 595.0
	pageHeight = 842.0
	marginX    = 56.0
	marginTop  = 64.0
	marginY    = 64.0
)

// Ukuran font.
const (
	sizeTitle   = 21.0
	sizeHeading = 14.0
	sizeBody    = 10.5
	lineFactor  = 1.5
)

// Block adalah satu bagian dokumen.
type Block struct {
	// Heading bernilai true untuk subjudul.
	Heading bool
	// Bullet bernilai true untuk butir daftar (memakai tanda "•").
	Bullet bool
	// Text adalah isi bagian (boleh berisi beberapa baris).
	Text string
}

// Document adalah isi e-book yang akan dirender.
type Document struct {
	Title    string
	Subtitle string
	Footer   string
	Blocks   []Block
}

// Render menghasilkan berkas PDF lengkap.
func Render(doc Document) []byte {
	pages := paginate(doc)

	var (
		buf     bytes.Buffer
		offsets []int
	)

	// Nomor objek: 1 katalog, 2 pages, 3 font normal, 4 font bold,
	// lalu 2 objek per halaman (page + content stream).
	const (
		fontRegularID = 3
		fontBoldID    = 4
		firstPageID   = 5
	)

	buf.WriteString("%PDF-1.4\n")
	// Komentar biner agar berkas dikenali sebagai biner oleh alat lain.
	buf.Write([]byte{'%', 0xE2, 0xE3, 0xCF, 0xD3, '\n'})

	writeObject := func(id int, body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	kids := make([]string, 0, len(pages))
	for index := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", firstPageID+index*2))
	}

	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), len(pages)))
	writeObject(fontRegularID, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	writeObject(fontBoldID, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")

	for index, lines := range pages {
		pageID := firstPageID + index*2
		contentID := pageID + 1
		writeObject(pageID, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
			pageWidth, pageHeight, fontRegularID, fontBoldID, contentID,
		))

		stream := pageStream(lines, index+1, len(pages), doc.Footer)
		writeObject(contentID, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}

	// Tabel xref + trailer.
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

// line adalah satu baris siap gambar pada halaman.
type line struct {
	text  string
	size  float64
	bold  bool
	space float64
}

// paginate menyusun baris-baris dokumen dan membaginya menjadi halaman.
func paginate(doc Document) [][]line {
	contentWidth := pageWidth - marginX*2

	out := make([]line, 0, 128)
	out = append(out, line{text: doc.Title, size: sizeTitle, bold: true, space: 12})
	if strings.TrimSpace(doc.Subtitle) != "" {
		out = append(out, line{text: doc.Subtitle, size: sizeBody, space: 22})
	} else {
		out = append(out, line{text: "", size: sizeBody, space: 16})
	}

	for _, block := range doc.Blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			out = append(out, line{size: sizeBody, space: 8})
			continue
		}

		switch {
		case block.Heading:
			out = append(out, line{size: sizeBody, space: 8})
			for _, wrapped := range wrap(text, sizeHeading, true, contentWidth) {
				out = append(out, line{text: wrapped, size: sizeHeading, bold: true, space: 8})
			}
		case block.Bullet:
			wrapped := wrap(text, sizeBody, false, contentWidth-16)
			for index, wrappedLine := range wrapped {
				prefix := "•  "
				if index > 0 {
					prefix = "    "
				}
				out = append(out, line{text: prefix + wrappedLine, size: sizeBody, space: 6})
			}
		default:
			for _, wrapped := range wrap(text, sizeBody, false, contentWidth) {
				out = append(out, line{text: wrapped, size: sizeBody, space: 4})
			}
		}
	}

	// Bagi menjadi halaman berdasarkan tinggi yang tersedia.
	usable := pageHeight - marginTop - marginY
	pages := make([][]line, 0, 4)
	current := make([]line, 0, 48)
	used := 0.0

	for _, item := range out {
		height := item.size*lineFactor + item.space
		if used+height > usable && len(current) > 0 {
			pages = append(pages, current)
			current = make([]line, 0, 48)
			used = 0
		}
		current = append(current, item)
		used += height
	}
	if len(current) > 0 || len(pages) == 0 {
		pages = append(pages, current)
	}
	return pages
}

// pageStream membentuk operator gambar untuk satu halaman.
func pageStream(lines []line, page, total int, footer string) string {
	var buf strings.Builder

	y := pageHeight - marginTop
	for _, item := range lines {
		y -= item.size*lineFactor + item.space
		if strings.TrimSpace(item.text) == "" {
			continue
		}
		font := "/F1"
		if item.bold {
			font = "/F2"
		}
		fmt.Fprintf(&buf, "BT %s %.1f Tf 1 0 0 1 %.1f %.1f Tm (%s) Tj ET\n",
			font, item.size, marginX, y, escape(item.text))
	}

	// Garis pemisah + footer.
	lineY := marginY - 18
	fmt.Fprintf(&buf, "0.85 0.88 0.92 RG 0.6 w %.1f %.1f m %.1f %.1f l S\n",
		marginX, lineY, pageWidth-marginX, lineY)

	label := strings.TrimSpace(footer)
	if label == "" {
		label = "BUMN Academy"
	}
	counter := fmt.Sprintf("Halaman %d dari %d", page, total)

	fmt.Fprintf(&buf, "BT /F1 8.5 Tf 0.42 0.46 0.53 rg 1 0 0 1 %.1f %.1f Tm (%s) Tj ET\n",
		marginX, lineY-13, escape(label))
	width := textWidth(counter, 8.5, false)
	fmt.Fprintf(&buf, "BT /F1 8.5 Tf 0.42 0.46 0.53 rg 1 0 0 1 %.1f %.1f Tm (%s) Tj ET\n",
		pageWidth-marginX-width, lineY-13, escape(counter))

	return buf.String()
}

// winAnsi menyetarakan karakter tipografi umum ke kode Windows-1252 yang
// dikenali font standar PDF (mis. bullet, tanda pisah, tanda kutip melengkung).
var winAnsi = map[rune]byte{
	'•': 0x95,
	'–': 0x96,
	'—': 0x97,
	'‘': 0x91,
	'’': 0x92,
	'“': 0x93,
	'”': 0x94,
	'…': 0x85,
	'€': 0x80,
	'™': 0x99,
	'≤': 0xB2,
	'≥': 0xB3,
	// Tanda minus matematis & tanda hubung panjang disamakan ke tanda hubung.
	'−': '-',
	'‑': '-',
	'‐': '-',
}

// escape menyiapkan teks agar aman di dalam string literal PDF.
//
// Font standar PDF memakai encoding Windows-1252 (bukan UTF-8), sehingga setiap
// karakter ditulis sebagai satu byte: karakter Latin-1 apa adanya, karakter
// tipografi umum dipetakan lewat `winAnsi`, dan sisanya diganti "?".
func escape(text string) string {
	var buf strings.Builder
	for _, r := range text {
		switch {
		case r == '(' || r == ')' || r == '\\':
			buf.WriteByte('\\')
			buf.WriteByte(byte(r))
		case r < 32:
			buf.WriteByte(' ')
		case r < 127:
			buf.WriteByte(byte(r))
		case r >= 160 && r <= 255:
			buf.WriteByte(byte(r))
		default:
			if mapped, ok := winAnsi[r]; ok {
				buf.WriteByte(mapped)
				continue
			}
			// Karakter di luar Windows-1252 diganti agar berkas tetap sah.
			buf.WriteByte('?')
		}
	}
	return buf.String()
}
