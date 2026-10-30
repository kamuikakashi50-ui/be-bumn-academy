// Package bank berisi metadata kategori bank soal dan dimensi taksonomi soal
// (sektor BUMN, jenis tes, keluarga jabatan).
//
// Daftar kategori mencerminkan BANK_LABELS pada front-end
// (src/lib/api/banks.ts), sedangkan sektor/jenis tes/keluarga jabatan
// mencerminkan src/lib/api/taxonomy.ts sehingga label konsisten di kedua sisi.
package bank

import "sort"

// Option adalah satu pilihan taksonomi (kunci + label) yang dikirim ke UI.
type Option struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Labels memetakan kunci bank → label yang ditampilkan.
//
// Sembilan kunci pertama adalah kategori umum yang dipakai bersama seluruh
// tryout; sisanya kategori bidang untuk soal Tes Kemampuan Bidang (TKB) pada
// sektor BUMN tertentu.
var Labels = map[string]string{
	"numerik":          "Penalaran Numerik",
	"verbal":           "Penalaran Verbal",
	"analitis":         "Logika Analitis",
	"silogisme":        "Silogisme",
	"figural":          "Penalaran Figural",
	"akhlaq":           "Nilai AKHLAK",
	"wawasan":          "Wawasan Kebangsaan",
	"reading":          "Reading Comprehension",
	"structure":        "Structure & Vocabulary",
	"akuntansi":        "Akuntansi & Keuangan",
	"risiko_kepatuhan": "Manajemen Risiko & Kepatuhan",
	"teknis_produksi":  "Teknis Produksi & Mutu",
	"k3":               "Keselamatan & Kesehatan Kerja",
	"teknis_operasi":   "Teknis Operasi",
	"hse":              "HSE & Lingkungan",
	"it_dasar":         "Dasar Teknologi Informasi",
	"layanan":          "Layanan & Komunikasi Pelanggan",
	"manajemen_proyek": "Manajemen Proyek",
	"psikotes":         "Psikotes (Deret, Kraepelin, Kepribadian)",
}

// bankOrder menentukan urutan kategori yang enak dibaca pada panel admin
// (bukan alfabetis): kategori umum lebih dulu, lalu kategori bidang.
var bankOrder = []string{
	"numerik", "verbal", "analitis", "silogisme", "figural",
	"akhlaq", "wawasan", "reading", "structure", "psikotes",
	"akuntansi", "risiko_kepatuhan", "teknis_produksi", "k3",
	"teknis_operasi", "hse", "it_dasar", "layanan", "manajemen_proyek",
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

// BankOptions mengembalikan kategori bank dalam urutan tampil (bukan alfabetis).
func BankOptions() []Option {
	out := make([]Option, 0, len(bankOrder))
	for _, key := range bankOrder {
		if label, ok := Labels[key]; ok {
			out = append(out, Option{Key: key, Label: label})
		}
	}
	return out
}

// IsValid memeriksa apakah kunci bank dikenal.
func IsValid(key string) bool {
	_, ok := Labels[key]
	return ok
}

/* ------------------------- dimensi taksonomi soal ------------------------- */

// Nilai bawaan ketika dimensi belum ditentukan (mis. soal lama atau soal baru
// yang belum dikelompokkan).
const (
	// DefaultSector adalah sektor lintas klaster.
	DefaultSector = "umum"
	// DefaultTestType adalah jenis tes paling umum (Tahap 1 RBB).
	DefaultTestType = "tkd"
	// DefaultPositionFamily adalah keluarga jabatan umum.
	DefaultPositionFamily = "umum"
)

// Sectors adalah klaster BUMN (12 klaster) ditambah pilihan lintas klaster.
// Daftar ini dipakai admin untuk menandai soal sesuai kategori BUMN-nya.
var Sectors = []Option{
	{Key: "umum", Label: "Umum / Lintas Klaster"},
	{Key: "migas_energi", Label: "Migas & Energi"},
	{Key: "minerba", Label: "Mineral & Batubara"},
	{Key: "perkebunan_kehutanan", Label: "Perkebunan & Kehutanan"},
	{Key: "pupuk_pangan", Label: "Pupuk & Pangan"},
	{Key: "farmasi_kesehatan", Label: "Farmasi & Kesehatan"},
	{Key: "pertahanan_manufaktur", Label: "Pertahanan & Manufaktur"},
	{Key: "jasa_keuangan", Label: "Jasa Keuangan"},
	{Key: "asuransi_dapen", Label: "Asuransi & Dana Pensiun"},
	{Key: "telko_media", Label: "Telekomunikasi & Media"},
	{Key: "infrastruktur", Label: "Infrastruktur"},
	{Key: "pariwisata_logistik", Label: "Pariwisata & Logistik"},
	{Key: "perhubungan", Label: "Sarana & Prasarana Perhubungan"},
}

// TestTypes adalah jenis tes pada seleksi BUMN, mengikuti tahapan Rekrutmen
// Bersama BUMN: Tahap 1 (TKD, AKHLAK, Wawasan Kebangsaan), Tahap 2 (Bahasa
// Inggris, Learning Agility), Tahap 3 (Tes Kemampuan Bidang), ditambah psikotes.
var TestTypes = []Option{
	{Key: "tkd", Label: "Tes Kompetensi Dasar"},
	{Key: "akhlaq", Label: "Tes AKHLAK"},
	{Key: "twk", Label: "Tes Wawasan Kebangsaan"},
	{Key: "bing", Label: "Tes Bahasa Inggris"},
	{Key: "learning_agility", Label: "Tes Learning Agility"},
	{Key: "tkb", Label: "Tes Kemampuan Bidang"},
	{Key: "psikotes", Label: "Tes Psikologi"},
}

// PositionFamilies adalah keluarga jabatan yang dipakai untuk mengelompokkan
// soal Tes Kemampuan Bidang sesuai posisi yang dilamar.
var PositionFamilies = []Option{
	{Key: "umum", Label: "Umum"},
	{Key: "keuangan", Label: "Keuangan & Akuntansi"},
	{Key: "manufaktur", Label: "Produksi & Manufaktur"},
	{Key: "operasi", Label: "Operasi & Teknik"},
	{Key: "ti", Label: "Teknologi Informasi & Data"},
	{Key: "pemasaran", Label: "Pemasaran & Penjualan"},
	{Key: "sdm", Label: "Sumber Daya Manusia"},
	{Key: "hukum", Label: "Hukum & Kepatuhan"},
	{Key: "kesehatan", Label: "Kesehatan & Farmasi"},
	{Key: "layanan", Label: "Layanan Pelanggan"},
	{Key: "proyek", Label: "Manajemen Proyek"},
}

// optionMap membentuk peta kunci → label dari daftar pilihan.
func optionMap(options []Option) map[string]string {
	out := make(map[string]string, len(options))
	for _, option := range options {
		out[option.Key] = option.Label
	}
	return out
}

var (
	sectorLabels         = optionMap(Sectors)
	testTypeLabels       = optionMap(TestTypes)
	positionFamilyLabels = optionMap(PositionFamilies)
)

// SectorLabel mengembalikan label sektor; kunci tak dikenal dikembalikan apa adanya.
func SectorLabel(key string) string { return labelOf(sectorLabels, key) }

// IsValidSector memeriksa apakah kunci sektor dikenal.
func IsValidSector(key string) bool { return isValidIn(sectorLabels, key) }

// TestTypeLabel mengembalikan label jenis tes.
func TestTypeLabel(key string) string { return labelOf(testTypeLabels, key) }

// IsValidTestType memeriksa apakah kunci jenis tes dikenal.
func IsValidTestType(key string) bool { return isValidIn(testTypeLabels, key) }

// PositionFamilyLabel mengembalikan label keluarga jabatan.
func PositionFamilyLabel(key string) string { return labelOf(positionFamilyLabels, key) }

// IsValidPositionFamily memeriksa apakah kunci keluarga jabatan dikenal.
func IsValidPositionFamily(key string) bool { return isValidIn(positionFamilyLabels, key) }

// SectorOrDefault mengembalikan sektor yang sudah divalidasi; kosong atau tak
// dikenal jatuh ke sektor lintas klaster.
func SectorOrDefault(key string) string { return orDefault(sectorLabels, key, DefaultSector) }

// TestTypeOrDefault mengembalikan jenis tes yang sudah divalidasi.
func TestTypeOrDefault(key string) string { return orDefault(testTypeLabels, key, DefaultTestType) }

// PositionFamilyOrDefault mengembalikan keluarga jabatan yang sudah divalidasi.
func PositionFamilyOrDefault(key string) string {
	return orDefault(positionFamilyLabels, key, DefaultPositionFamily)
}

// TestTypeForBank menebak jenis tes dari kategori bank soal.
//
// Dipakai saat backfill data lama dan saat seed demo, sehingga soal yang sudah
// ada langsung punya jenis tes yang masuk akal tanpa diisi manual:
// soal AKHLAK → `akhlaq`, wawasan kebangsaan → `twk`, bahasa Inggris → `bing`,
// kategori bidang → `tkb`, sisanya → `tkd` (Tes Kompetensi Dasar).
func TestTypeForBank(bankKey string) string {
	switch bankKey {
	case "akhlaq":
		return "akhlaq"
	case "wawasan":
		return "twk"
	case "reading", "structure":
		return "bing"
	case "psikotes":
		return "psikotes"
	case "akuntansi", "risiko_kepatuhan", "teknis_produksi", "k3",
		"teknis_operasi", "hse", "it_dasar", "layanan", "manajemen_proyek":
		return "tkb"
	default:
		return DefaultTestType
	}
}

func labelOf(labels map[string]string, key string) string {
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

func isValidIn(labels map[string]string, key string) bool {
	_, ok := labels[key]
	return ok
}

func orDefault(labels map[string]string, key, fallback string) string {
	if _, ok := labels[key]; ok {
		return key
	}
	return fallback
}
