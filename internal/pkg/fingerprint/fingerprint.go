// Package fingerprint membentuk "sidik jari" teks soal agar soal yang identik
// atau hampir sama bisa ditemukan kembali.
//
// Kebutuhan ini muncul dari perjanjian kerahasiaan (pakta) dengan pihak lain:
// soal tidak boleh dipakai ulang atau muncul identik di dua paket/klien,
// sehingga tim akademik perlu dibantu mencari soal kembar — pekerjaan yang
// praktis tidak mungkin dilakukan dengan pencarian manual.
//
// Tiga tingkat pemeriksaan disediakan:
//
//   - Hash         → soal yang sama persis (setelah dinormalisasi).
//   - TemplateHash → soal yang sama persis kecuali angka (satu "keluarga" soal).
//   - Tokens       → kemiripan Jaccard antar himpunan kata, untuk soal yang
//     diubah sedikit kata atau tanda bacanya.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"sort"
	"strings"
	"unicode"
)

// operatorRunes adalah simbol matematika yang dipertahankan saat normalisasi.
// Tanpa ini, "12 + 7" dan "12 * 7" akan dianggap teks yang sama sehingga soal
// berbeda ikut tertandai sebagai kembar.
const operatorRunes = "+-*/=×÷^%<>"

// DefaultThreshold adalah ambang kemiripan untuk menganggap dua soal kembar.
//
// Nilai 0,75 dipilih dari pengukuran pada soal latihan BUMN: soal yang hanya
// diubah satu kata dari 13 kata menghasilkan kemiripan ~0,83, sedangkan soal
// berbeda topik atau berbeda angka menghasilkan ≤0,5. Ambang ini sengaja
// menghasilkan sedikit positif palsu: meninjau satu soal tambahan jauh lebih
// murah daripada melewatkan soal yang terikat perjanjian.
const DefaultThreshold = 0.75

// Normalize menyeragamkan teks soal: huruf kecil, tanda baca menjadi spasi,
// spasi berulang dirapikan, dan simbol matematika dipertahankan sebagai token
// tersendiri. Angka dipertahankan karena membedakan soal hitungan.
func Normalize(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))

	lastSpace := true
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastSpace = false
		case strings.ContainsRune(operatorRunes, r):
			if !lastSpace {
				builder.WriteRune(' ')
			}
			builder.WriteRune(r)
			builder.WriteRune(' ')
			lastSpace = true
		default:
			if !lastSpace {
				builder.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

// Hash mengembalikan SHA-256 heksadesimal dari teks yang sudah dinormalisasi.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(Normalize(text)))
	return hex.EncodeToString(sum[:])
}

// TemplateHash mengembalikan hash teks yang setiap angka utuhnya diganti "#",
// sehingga dua soal dengan rumus sama tetapi angka berbeda dikenali sebagai satu
// keluarga soal. Dipakai untuk memisahkan "soal kembar" dari "varian sah".
func TemplateHash(text string) string {
	normalized := Normalize(text)

	var builder strings.Builder
	builder.Grow(len(normalized))
	inNumber := false
	for _, r := range normalized {
		switch {
		case unicode.IsDigit(r):
			if !inNumber {
				builder.WriteRune('#')
				inNumber = true
			}
		case r == ' ':
			builder.WriteRune(' ')
			inNumber = false
		default:
			builder.WriteRune(r)
			inNumber = false
		}
	}

	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

// Tokens mengembalikan himpunan kata (unik dan terurut) dari teks soal.
//
// Kata didahulukan daripada n-gram karena soal latihan BUMN umumnya pendek:
// pada soal 10–13 kata, n-gram 5 menghasilkan terlalu sedikit shingle sehingga
// satu kata yang diubah sudah menjatuhkan kemiripan ke bawah ambang.
func Tokens(text string) []string {
	words := strings.Fields(Normalize(text))
	if len(words) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(words))
	out := make([]string, 0, len(words))
	for _, word := range words {
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		out = append(out, word)
	}
	sort.Strings(out)
	return out
}

// Similarity menghitung kemiripan Jaccard dua himpunan kata (0..1).
//
// Kedua himpunan diasumsikan terurut sehingga penggabungan berjalan satu
// lintasan tanpa alokasi peta tambahan.
func Similarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	intersection := 0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			intersection++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}

	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// IsNearDuplicate memeriksa apakah dua himpunan kata melewati ambang.
func IsNearDuplicate(a, b []string, threshold float64) bool {
	return Similarity(a, b) >= threshold
}

// tokenHash mengubah satu kata menjadi token heksadesimal pendek (FNV-1a 64).
//
// Disiapkan untuk pemeriksaan pada kolom terindeks bila kelak jumlah soal
// bertambah besar; saat ini sidik jari disimpan sebagai hash SHA-256.
func tokenHash(value string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
