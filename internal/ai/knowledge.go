package ai

import "strings"

// knowledgeBase memetakan kategori materi → nasihat belajar yang spesifik.
//
// Isi ini adalah pengetahuan pengajar yang dipakai mentor heuristik lokal
// sekaligus menjadi rujukan bila model bahasa tidak tersedia. Kunci pencarian
// memakai nama subtes maupun label bank soal (keduanya huruf kecil).
var knowledgeBase = map[string]struct {
	Concept string
	Advice  string
}{
	"numerik": {
		Concept: "Soal numerik hampir selalu bisa disederhanakan: ubah soal cerita menjadi satu persamaan, lalu hitung. Kalau muncul deret, periksa selisih antar suku terlebih dahulu, baru rasio.",
		Advice:  "Latih tiga hal berurutan: (1) mengubah soal cerita jadi persamaan, (2) deret angka (selisih, rasio, pola bertingkat), dan (3) persentase/perbandingan. Kerjakan dengan timer karena masalah utamanya biasanya kecepatan, bukan pemahaman.",
	},
	"verbal": {
		Concept: "Soal verbal menguji ketepatan makna, bukan selera bahasa. Untuk sinonim/antonim, uji kata pada satu kalimat netral; untuk analogi, tentukan dulu hubungannya (fungsi, sebab-akibat, bagian-keseluruhan).",
		Advice:  "Perbanyak latihan analogi dan kalimat efektif. Biasakan menuliskan hubungan pada soal analogi sebelum melihat opsi, supaya tidak terjebak opsi yang 'terdengar' benar.",
	},
	"analitis": {
		Concept: "Logika analitis menuntut kesimpulan yang pasti benar dari premis, bukan yang mungkin benar. Periksa apakah kesimpulan memperluas informasi premis; kalau ya, kesimpulan itu tidak sah.",
		Advice:  "Latih modus ponens/tollens dan silogisme kategoris. Untuk soal urutan/posisi, selalu gambar posisinya di kertas sebelum memilih jawaban.",
	},
	"silogisme": {
		Concept: "Pada silogisme, kesimpulan hanya sah bila menghubungkan premis secara langsung. Kata 'sebagian' pada premis tidak boleh berubah menjadi 'semua' di kesimpulan.",
		Advice:  "Latih menandai kuantor setiap premis (semua/sebagian/tidak ada) dan menolak opsi yang menaikkan cakupan kuantor.",
	},
	"figural": {
		Concept: "Pola figural biasanya berulang pada tiga sumbu: bentuk, jumlah elemen, dan orientasi. Tentukan dulu sumbu mana yang berubah, baru cari siklusnya.",
		Advice:  "Latih pengenalan rotasi (90°/180°), pencerminan, dan penambahan/pengurangan elemen. Cukup 10 soal per hari tetapi selalu tuliskan polanya.",
	},
	"akhlaq": {
		Concept: "Soal AKHLAK menilai pemilihan nilai yang paling tepat pada situasi kerja. Kunci jawabannya adalah perilaku yang menjaga kepercayaan dan kepatuhan aturan, bukan yang paling cepat menyelesaikan masalah.",
		Advice:  "Hafalkan enam nilai inti beserta contoh perilakunya, lalu latih memetakan situasi kasus ke nilai yang paling dominan.",
	},
	"wawasan": {
		Concept: "Soal wawasan menguji pemahaman konsep dasar bernegara dan kebijakan publik, bukan hafalan angka. Fokus pada alasan di balik sebuah kebijakan.",
		Advice:  "Baca ringkasan materi wawasan kebangsaan, lalu hubungkan setiap poin dengan contoh kasus nyata agar mudah diingat.",
	},
	"reading": {
		Concept: "Pada reading comprehension, jawaban selalu bisa ditemukan di teks. Skimming mencari gagasan utama, scanning mencari detail spesifik — jangan membaca kata per kata.",
		Advice:  "Latih dua teknik terpisah: baca pertanyaan dulu lalu scanning ke teks, dan skimming satu paragraf dalam 15 detik untuk menangkap gagasan utamanya.",
	},
	"structure": {
		Concept: "Soal structure menguji kesesuaian tenses, kesesuaian subjek-predikat, dan bentuk pasif. Tentukan waktu kejadian lebih dulu, karena itu yang menentukan bentuk kata kerja.",
		Advice:  "Kuasai pola conditional, passive voice, dan subject-verb agreement. Latih dengan membaca ulang kalimat setelah memilih jawaban untuk memastikan terdengar gramatikal.",
	},
}

// coachingFor menyusun nasihat spesifik untuk satu soal.
func coachingFor(question QuestionContext) string {
	if entry, ok := lookupKnowledge(question.BankLabel, question.Subtest); ok {
		return entry.Concept
	}
	return "Baca ulang pertanyaannya sekali lagi sebelum memilih: banyak kesalahan terjadi karena menjawab pertanyaan yang berbeda dari yang diminta."
}

// conceptForBank menjelaskan konsep dasar kategori soal.
func conceptForBank(question QuestionContext) string {
	if entry, ok := lookupKnowledge(question.BankLabel, question.Subtest); ok {
		return entry.Advice
	}
	return "Kerjakan soal sejenis secara berulang sampai polanya terasa familiar."
}

// adviceForSubtest menyusun nasihat belajar untuk sebuah subtes.
func adviceForSubtest(name string) string {
	if entry, ok := lookupKnowledge(name); ok {
		return entry.Advice
	}
	return "Kerjakan soal sejenis secara berulang, catat pola kesalahannya, lalu ulangi setelah 2–3 hari."
}

// lookupKnowledge mencari entri pengetahuan berdasarkan beberapa kunci.
//
// Kunci dicocokkan pada nama lengkap (huruf kecil) lalu pada kata kunci bank
// (mis. "numerik" pada "Penalaran Numerik").
func lookupKnowledge(keys ...string) (struct {
	Concept string
	Advice  string
}, bool) {
	for _, key := range keys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		if entry, ok := knowledgeBase[normalized]; ok {
			return entry, true
		}
		for bankKey, entry := range knowledgeBase {
			if strings.Contains(normalized, bankKey) {
				return entry, true
			}
		}
	}
	return struct {
		Concept string
		Advice  string
	}{}, false
}
