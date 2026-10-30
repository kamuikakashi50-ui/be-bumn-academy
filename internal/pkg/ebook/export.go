package ebook

// MeasureText mengukur lebar teks dengan font standar PDF (Helvetica) pada
// ukuran tertentu.
//
// Diekspor agar generator PDF lain di repo ini (mis. sertifikat) memakai tabel
// lebar font yang sama, bukan menyalinnya — salah tabel berarti teks tidak
// berada di tengah halaman.
func MeasureText(text string, size float64, bold bool) float64 {
	return textWidth(text, size, bold)
}

// EscapeText menyiapkan teks agar aman sebagai string literal PDF
// (encoding Windows-1252 + escape tanda kurung/backslash).
func EscapeText(text string) string { return escape(text) }
