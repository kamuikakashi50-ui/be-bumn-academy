package bank

import (
	"sort"
	"testing"
)

func TestLabel(t *testing.T) {
	if got := Label("numerik"); got != "Penalaran Numerik" {
		t.Errorf("Label(numerik) = %q", got)
	}
	if got := Label("tidak-ada"); got != "tidak-ada" {
		t.Errorf("Label tidak dikenal harus dikembalikan apa adanya, dapat %q", got)
	}
}

func TestIsValid(t *testing.T) {
	for _, key := range []string{"numerik", "verbal", "analitis", "silogisme", "figural", "akhlaq", "wawasan", "reading", "structure"} {
		if !IsValid(key) {
			t.Errorf("%q harus dianggap valid", key)
		}
	}
	if IsValid("matematika") {
		t.Error("kunci tidak dikenal harus invalid")
	}
}

func TestKeysTerurut(t *testing.T) {
	keys := Keys()
	if len(keys) != len(Labels) {
		t.Fatalf("Keys() = %d, harap %d", len(keys), len(Labels))
	}
	if !sort.StringsAreSorted(keys) {
		t.Errorf("Keys() harus terurut: %v", keys)
	}
}

func TestLabelsMencakupKategoriUmum(t *testing.T) {
	// Sembilan kategori umum ini mencerminkan BANK_LABELS pada front-end dan
	// harus selalu ada; kategori bidang (TKB) boleh bertambah seiring konten.
	for _, key := range []string{
		"numerik", "verbal", "analitis", "silogisme", "figural",
		"akhlaq", "wawasan", "reading", "structure",
	} {
		if _, ok := Labels[key]; !ok {
			t.Errorf("kategori umum %q hilang dari Labels", key)
		}
	}
}

func TestBankOptionsMengikutiUrutanTampil(t *testing.T) {
	options := BankOptions()
	if len(options) != len(Labels) {
		t.Fatalf("BankOptions() = %d, harap %d", len(options), len(Labels))
	}
	if options[0].Key != "numerik" {
		t.Errorf("kategori pertama = %q, harap numerik", options[0].Key)
	}
	for _, option := range options {
		if option.Label != Labels[option.Key] {
			t.Errorf("label %q tidak cocok dengan Labels", option.Key)
		}
	}
}

func TestSectorTaxonomy(t *testing.T) {
	if len(Sectors) != 13 { // 12 klaster BUMN + lintas klaster
		t.Errorf("jumlah sektor = %d, harap 13", len(Sectors))
	}
	if !IsValidSector("jasa_keuangan") || !IsValidSector("pertahanan_manufaktur") {
		t.Error("sektor klaster BUMN harus dianggap valid")
	}
	if IsValidSector("sektor_karangan") {
		t.Error("sektor tidak dikenal harus invalid")
	}
	if got := SectorLabel("jasa_keuangan"); got != "Jasa Keuangan" {
		t.Errorf("SectorLabel(jasa_keuangan) = %q", got)
	}
	if got := SectorLabel("tidak-ada"); got != "tidak-ada" {
		t.Errorf("label sektor tak dikenal harus dikembalikan apa adanya, dapat %q", got)
	}
	if got := SectorOrDefault(""); got != DefaultSector {
		t.Errorf("SectorOrDefault(\"\") = %q, harap %q", got, DefaultSector)
	}
	if got := SectorOrDefault("sektor_karangan"); got != DefaultSector {
		t.Errorf("sektor tak dikenal harus jatuh ke %q, dapat %q", DefaultSector, got)
	}
}

func TestTestTypeTaxonomy(t *testing.T) {
	for _, key := range []string{"tkd", "akhlaq", "twk", "bing", "learning_agility", "tkb", "psikotes"} {
		if !IsValidTestType(key) {
			t.Errorf("jenis tes %q harus dianggap valid", key)
		}
	}
	if IsValidTestType("wawancara") {
		t.Error("jenis tes tidak dikenal harus invalid")
	}
	if got := TestTypeOrDefault("tidak-ada"); got != DefaultTestType {
		t.Errorf("TestTypeOrDefault tak dikenal = %q, harap %q", got, DefaultTestType)
	}
}

func TestPositionFamilyTaxonomy(t *testing.T) {
	if !IsValidPositionFamily("keuangan") || !IsValidPositionFamily("ti") {
		t.Error("keluarga jabatan umum harus dianggap valid")
	}
	if got := PositionFamilyOrDefault("karangan"); got != DefaultPositionFamily {
		t.Errorf("PositionFamilyOrDefault tak dikenal = %q, harap %q", got, DefaultPositionFamily)
	}
}

func TestTestTypeForBank(t *testing.T) {
	cases := map[string]string{
		"akhlaq":    "akhlaq",
		"wawasan":   "twk",
		"reading":   "bing",
		"structure": "bing",
		"psikotes":  "psikotes",
		"akuntansi": "tkb",
		"k3":        "tkb",
		"numerik":   "tkd",
		"tidak-ada": "tkd",
	}
	for bankKey, want := range cases {
		if got := TestTypeForBank(bankKey); got != want {
			t.Errorf("TestTypeForBank(%q) = %q, harap %q", bankKey, got, want)
		}
	}
}
