package service

import (
	"strings"
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

func TestPercentForCourseProgress(t *testing.T) {
	cases := []struct {
		part, total, want int
	}{
		{0, 0, 0},   // pembagi nol tidak boleh panic
		{3, 0, 0},   // total kosong (produk tanpa materi)
		{0, 4, 0},   // belum ada yang selesai
		{1, 4, 25},  // seperempat modul
		{2, 3, 67},  // dibulatkan ke atas
		{3, 3, 100}, // seluruh modul selesai
	}

	for _, tc := range cases {
		if got := percent(tc.part, tc.total); got != tc.want {
			t.Errorf("percent(%d, %d) = %d, harap %d", tc.part, tc.total, got, tc.want)
		}
	}
}

func TestClampInt(t *testing.T) {
	if got := clampInt(-5, 0, 100); got != 0 {
		t.Errorf("clampInt bawah = %d, harap 0", got)
	}
	if got := clampInt(150, 0, 100); got != 100 {
		t.Errorf("clampInt atas = %d, harap 100", got)
	}
	if got := clampInt(42, 0, 100); got != 42 {
		t.Errorf("clampInt tengah = %d, harap 42", got)
	}
}

func TestEbookBlocksParseContentConventions(t *testing.T) {
	lesson := models.Lesson{
		Summary: "Ringkasan singkat materi.",
		Content: "## Tujuan pembelajaran\nPeserta dapat menghitung deret.\n\n- Poin pertama\n- Poin kedua\n\nPenutup materi.",
	}

	blocks := ebookBlocks(lesson)

	var headings, bullets, paragraphs int
	for _, block := range blocks {
		switch {
		case block.Heading:
			headings++
		case block.Bullet:
			bullets++
		case strings.TrimSpace(block.Text) != "":
			paragraphs++
		}
	}

	// Ringkasan + Tujuan + Catatan (3 judul), 2 butir, dan paragraf isi.
	if headings != 3 {
		t.Errorf("jumlah subjudul = %d, harap 3", headings)
	}
	if bullets != 2 {
		t.Errorf("jumlah butir = %d, harap 2", bullets)
	}
	if paragraphs < 2 {
		t.Errorf("jumlah paragraf = %d, harap minimal 2", paragraphs)
	}

	// Judul ringkasan harus muncul lebih dulu.
	if blocks[0].Text != "Ringkasan" || !blocks[0].Heading {
		t.Errorf("blok pertama = %+v, harap judul Ringkasan", blocks[0])
	}
	// Catatan penutup selalu ditambahkan.
	last := blocks[len(blocks)-1]
	if !strings.Contains(last.Text, "tidak ada klaim afiliasi") && !strings.Contains(strings.ToLower(last.Text), "tidak ada klaim afiliasi resmi") {
		t.Errorf("blok terakhir tidak memuat catatan: %+v", last)
	}
}

func TestEbookBlocksSkipSummaryWhenEmpty(t *testing.T) {
	blocks := ebookBlocks(models.Lesson{Content: "Hanya konten tanpa ringkasan."})
	if blocks[0].Heading && blocks[0].Text == "Ringkasan" {
		t.Fatal("ringkasan kosong tidak boleh menghasilkan subjudul Ringkasan")
	}
}

func TestEbookFilenameSlugified(t *testing.T) {
	cases := map[string]string{
		"Deret Angka & Pola: Pendekatan Sistematis": "deret-angka-pola-pendekatan-sistematis.pdf",
		"E-book: Rangkuman Rumus Numerik":           "e-book-rangkuman-rumus-numerik.pdf",
		"   ":                                       "materi.pdf",
		"###":                                       "materi.pdf",
	}

	for input, want := range cases {
		if got := ebookFilename(input); got != want {
			t.Errorf("ebookFilename(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestProgressViewHandlesNilProgress(t *testing.T) {
	empty := progressView(nil)
	if empty.ProgressPercent != 0 || empty.Completed || empty.PositionSeconds != 0 {
		t.Fatalf("progres kosong tidak sesuai: %+v", empty)
	}

	viewed := progressView(&models.LessonProgress{
		PositionSeconds: 12,
		ProgressPercent: 50,
		Completed:       false,
		LastViewedAt:    time.Date(2026, 10, 24, 8, 30, 0, 0, time.UTC),
	})
	if viewed.PositionSeconds != 12 || viewed.ProgressPercent != 50 {
		t.Fatalf("progres terisi tidak sesuai: %+v", viewed)
	}
	if !strings.HasPrefix(viewed.LastViewedAt, "2026-10-24") {
		t.Fatalf("waktu tayang tidak diformat: %q", viewed.LastViewedAt)
	}
}

func TestCompletedThresholdValue(t *testing.T) {
	// Ambang penyelesaian otomatis dipakai pada SaveProgress; pastikan nilainya
	// tetap konsisten dengan dokumentasi (90 persen).
	if CompletedThreshold != 90 {
		t.Fatalf("CompletedThreshold = %d, harap 90", CompletedThreshold)
	}
}
