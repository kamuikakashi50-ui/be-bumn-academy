// Package bank berisi metadata kategori bank soal.
//
// Daftar ini mencerminkan BANK_LABELS pada front-end
// (src/lib/mock/questions.ts) sehingga label kategori konsisten di kedua sisi.
package bank

import "sort"

// Labels memetakan kunci bank → label yang ditampilkan.
var Labels = map[string]string{
	"numerik":   "Penalaran Numerik",
	"verbal":    "Penalaran Verbal",
	"analitis":  "Logika Analitis",
	"silogisme": "Silogisme",
	"figural":   "Penalaran Figural",
	"akhlaq":    "Nilai AKHLAK",
	"wawasan":   "Wawasan Kebangsaan",
	"reading":   "Reading Comprehension",
	"structure": "Structure & Vocabulary",
}

// Label mengembalikan label bank; bila tidak dikenal, kunci dikembalikan apa adanya.
func Label(key string) string {
	if label, ok := Labels[key]; ok {
		return label
	}
	return key
}

// Keys mengembalikan seluruh kunci bank dalam urutan alfabetis.
func Keys() []string {
	out := make([]string, 0, len(Labels))
	for key := range Labels {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// IsValid memeriksa apakah kunci bank dikenal.
func IsValid(key string) bool {
	_, ok := Labels[key]
	return ok
}
