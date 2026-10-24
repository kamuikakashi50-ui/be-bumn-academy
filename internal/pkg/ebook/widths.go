package ebook

import "strings"

// Lebar karakter font standar PDF (satuan 1/1000 em) untuk kode ASCII 32–126.
// Nilai diambil dari metrik AFM Helvetica & Helvetica-Bold sehingga pembungkusan
// baris mengikuti lebar teks sebenarnya.
var (
	regularWidths = []int16{
		278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
		1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
		667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
		333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
		556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
	}

	boldWidths = []int16{
		278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
		975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
		667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
		333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
		611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
	}
)

// textWidth menghitung lebar teks (point) untuk ukuran & ketebalan tertentu.
func textWidth(text string, size float64, bold bool) float64 {
	table := regularWidths
	if bold {
		table = boldWidths
	}

	total := 0
	for _, r := range text {
		switch {
		case r >= 32 && r <= 126:
			total += int(table[r-32])
		case r == '•':
			total += 350
		default:
			// Perkiraan untuk karakter di luar ASCII (mis. aksen).
			total += 556
		}
	}
	return float64(total) / 1000 * size
}

// wrap memotong teks menjadi baris-baris yang tidak melebihi `maxWidth`.
//
// Baris kosong pada sumber dipertahankan sebagai pemisah paragraf.
func wrap(text string, size float64, bold bool, maxWidth float64) []string {
	out := make([]string, 0, 8)

	for _, paragraph := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}

		current := words[0]
		for _, word := range words[1:] {
			candidate := current + " " + word
			if textWidth(candidate, size, bold) <= maxWidth {
				current = candidate
				continue
			}
			out = append(out, current)
			current = word
		}
		out = append(out, current)
	}

	// Buang baris kosong di akhir agar tidak menyisakan ruang kosong.
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}
