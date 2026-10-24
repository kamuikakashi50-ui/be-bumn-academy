package ebook

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// sampleDoc adalah dokumen uji yang menyerupai modul nyata.
func sampleDoc() Document {
	return Document{
		Title:    "Deret Angka & Pola — Pendekatan Sistematis",
		Subtitle: "Modul 1 · Penalaran Numerik · Kelas Intensif",
		Footer:   "BUMN Academy — Ruang Kelas",
		Blocks: []Block{
			{Heading: true, Text: "Tujuan pembelajaran"},
			{Text: "Setelah membaca modul ini kamu dapat mengenali pola deret aritmetika, geometri, dan campuran."},
			{Bullet: true, Text: "Deret aritmetika: selisih antar suku tetap."},
			{Bullet: true, Text: "Deret geometri: rasio antar suku tetap."},
			{Text: strings.Repeat("Latihan rutin membuat pemilihan strategi menjadi refleks. ", 60)},
			{Heading: true, Text: "Contoh soal"},
			{Text: "Tentukan suku ke-6 dari 3, 7, 15, 31, ..."},
		},
	}
}

func TestRenderProducesValidHeaderAndFooter(t *testing.T) {
	data := Render(sampleDoc())

	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) {
		t.Fatalf("berkas tidak diawali header PDF 1.4")
	}
	if !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		t.Fatalf("berkas tidak diakhiri %%EOF")
	}
	if !bytes.Contains(data, []byte("/Root 1 0 R")) {
		t.Fatalf("trailer tidak menunjuk katalog")
	}
}

// Setiap offset pada tabel xref harus menunjuk ke awal objek yang sesuai.
func TestRenderWritesResolvableXref(t *testing.T) {
	data := Render(sampleDoc())

	start := bytes.LastIndex(data, []byte("startxref"))
	if start < 0 {
		t.Fatal("startxref tidak ditemukan")
	}

	raw := strings.TrimSpace(string(data[start+len("startxref"):]))
	offset, err := strconv.Atoi(strings.SplitN(raw, "\n", 2)[0])
	if err != nil {
		t.Fatalf("nilai startxref tidak valid: %v", err)
	}
	if offset >= len(data) {
		t.Fatalf("startxref %d di luar berkas (%d byte)", offset, len(data))
	}
	if !bytes.HasPrefix(data[offset:], []byte("xref\n0 ")) {
		t.Fatalf("startxref tidak menunjuk tabel xref")
	}

	// Baris pertama xref adalah entri bebas, sisanya offset objek.
	lines := strings.Split(string(data[offset:]), "\n")
	if len(lines) < 3 {
		t.Fatal("tabel xref terlalu pendek")
	}
	total, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(lines[1], " ", 2)[1]))
	if err != nil {
		t.Fatalf("jumlah objek pada xref tidak valid: %v", err)
	}

	for index := 1; index < total; index++ {
		// Baris: 0 "xref", 1 "0 N", 2 entri bebas, lalu entri objek 1..N-1.
		entry := lines[index+2]
		if len(entry) < 18 {
			t.Fatalf("entri xref %d tidak lengkap: %q", index, entry)
		}
		objOffset, err := strconv.Atoi(entry[:10])
		if err != nil {
			t.Fatalf("offset objek %d tidak valid: %v", index, err)
		}
		want := fmt.Sprintf("%d 0 obj", index)
		if !bytes.HasPrefix(data[objOffset:], []byte(want)) {
			t.Fatalf("offset objek %d menunjuk ke %q, harap %q",
				index, string(data[objOffset:objOffset+12]), want)
		}
	}
}

func TestRenderPaginatesAndCountsPages(t *testing.T) {
	data := Render(sampleDoc())
	text := string(data)

	if got := strings.Count(text, "/Type /Page "); got < 2 {
		t.Fatalf("dokumen panjang seharusnya lebih dari satu halaman, dapat %d", got)
	}
	if !strings.Contains(text, "/Count 2") && !strings.Contains(text, "/Count 3") {
		t.Fatalf("objek /Pages tidak memuat jumlah halaman yang sesuai")
	}
	if !strings.Contains(text, "(Halaman 1 dari 2)") && !strings.Contains(text, "(Halaman 1 dari 3)") {
		t.Fatalf("footer nomor halaman tidak ditemukan")
	}
}

func TestRenderKeepsSinglePageForShortDocument(t *testing.T) {
	data := Render(Document{Title: "Ringkas", Blocks: []Block{{Text: "Satu paragraf singkat."}}})
	text := string(data)

	if got := strings.Count(text, "/Type /Page "); got != 1 {
		t.Fatalf("dokumen singkat seharusnya satu halaman, dapat %d", got)
	}
	if !strings.Contains(text, "/Count 1") {
		t.Fatal("jumlah halaman tidak sesuai")
	}
}

func TestEscapeHandlesPdfSpecialCharacters(t *testing.T) {
	cases := map[string]string{
		"kurung (buka) dan (tutup)": `kurung \(buka\) dan \(tutup\)`,
		"backslash \\ di tengah":    `backslash \\ di tengah`,
		"bullet • dan pisah — en":   "bullet \x95 dan pisah \x97 en",
		"minus − dan kali ×":        "minus - dan kali \xd7",
		"emoji 🎯 diganti":           "emoji ? diganti",
	}

	for input, want := range cases {
		if got := escape(input); got != want {
			t.Errorf("escape(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestWrapRespectsWidthAndParagraphs(t *testing.T) {
	long := strings.Repeat("kata ", 40)
	lines := wrap(strings.TrimSpace(long), sizeBody, false, 300)

	if len(lines) < 2 {
		t.Fatalf("teks panjang seharusnya dibungkus menjadi beberapa baris, dapat %d", len(lines))
	}
	for _, line := range lines {
		if width := textWidth(line, sizeBody, false); width > 300 {
			t.Errorf("baris melebihi lebar maksimum: %.1f (%q)", width, line)
		}
	}

	paragraphs := wrap("Paragraf pertama.\n\nParagraf kedua.", sizeBody, false, 400)
	if len(paragraphs) != 3 || paragraphs[1] != "" {
		t.Fatalf("pemisah paragraf tidak dipertahankan: %#v", paragraphs)
	}
}

func TestTextWidthUsesFontMetrics(t *testing.T) {
	narrow := textWidth("iii", 10, false)
	wide := textWidth("WWW", 10, false)
	if wide <= narrow {
		t.Fatalf("karakter lebar seharusnya lebih lebar: %f vs %f", wide, narrow)
	}
	if bold := textWidth("Ringkasan", 12, true); bold <= textWidth("Ringkasan", 12, false) {
		t.Fatalf("teks tebal seharusnya lebih lebar")
	}
}
