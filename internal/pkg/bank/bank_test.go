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

func TestLabelsMencakupSembilanKategori(t *testing.T) {
	// Sembilan kategori ini mencerminkan BANK_LABELS pada front-end.
	if len(Labels) != 9 {
		t.Errorf("jumlah kategori = %d, harap 9", len(Labels))
	}
}
