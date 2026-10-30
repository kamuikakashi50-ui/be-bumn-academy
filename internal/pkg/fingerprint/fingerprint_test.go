package fingerprint

import (
	"sort"
	"testing"
)

func TestNormalizeMenyeragamkanTeks(t *testing.T) {
	cases := map[string]string{
		"  Berapa hasil dari 12 + 7?  ": "berapa hasil dari 12 + 7",
		"AKHLAK: Amanah, Kompeten!":     "akhlak amanah kompeten",
		"12*7":                          "12 * 7",
		"":                              "",
		"   ":                           "",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestHashSamaWalauTandaBacaBerbeda(t *testing.T) {
	a := Hash("Berapa hasil 12 + 7?")
	b := Hash("berapa hasil 12+7")
	if a != b {
		t.Errorf("hash seharusnya sama: %s vs %s", a, b)
	}
	if Hash("Berapa hasil 12 + 8?") == a {
		t.Error("angka berbeda harus menghasilkan hash berbeda")
	}
}

func TestTemplateHashMengabaikanAngkaTetapiBukanOperator(t *testing.T) {
	a := TemplateHash("Jika x = 12 dan y = 7, berapa x + y?")
	b := TemplateHash("Jika x = 45 dan y = 3, berapa x + y?")
	if a != b {
		t.Errorf("satu keluarga soal harus berbagi template hash: %s vs %s", a, b)
	}
	if TemplateHash("Jika x = 12 dan y = 7, berapa x * y?") == a {
		t.Error("operator berbeda harus menghasilkan template hash berbeda")
	}
}

func TestTokensUnikDanTerurut(t *testing.T) {
	tokens := Tokens("Kata kata KATA lain")
	if len(tokens) != 2 {
		t.Fatalf("kata kembar harus dibuang, dapat %v", tokens)
	}
	if !sort.StringsAreSorted(tokens) {
		t.Errorf("token harus terurut: %v", tokens)
	}
	if len(Tokens("   ")) != 0 {
		t.Error("teks kosong tidak menghasilkan token")
	}
}

func TestSimilaritySoalIdentik(t *testing.T) {
	text := "Seorang pegawai menyelesaikan 30 laporan dalam 5 hari kerja, berapa laporan per hari"
	if got := Similarity(Tokens(text), Tokens(text)); got != 1 {
		t.Errorf("soal identik harus mirip 1, dapat %v", got)
	}
}

func TestSimilaritySoalDiubahSedikitTetapTinggi(t *testing.T) {
	base := "Seorang pegawai menyelesaikan 30 laporan dalam 5 hari kerja, berapa laporan per hari"
	changed := "Seorang pegawai menyelesaikan 30 laporan dalam 5 hari kerja, berapa laporan setiap hari"

	got := Similarity(Tokens(base), Tokens(changed))
	if got < DefaultThreshold {
		t.Errorf("perubahan kecil harus tetap di atas ambang %v, dapat %v", DefaultThreshold, got)
	}
	if !IsNearDuplicate(Tokens(base), Tokens(changed), DefaultThreshold) {
		t.Error("IsNearDuplicate harus true untuk soal yang diubah sedikit")
	}
}

func TestSimilaritySoalBerbedaRendah(t *testing.T) {
	a := "Sebutkan enam nilai inti AKHLAK beserta contoh penerapannya di tempat kerja"
	b := "Hitung luas segitiga jika alasnya 10 sentimeter dan tingginya 6 sentimeter"

	if got := Similarity(Tokens(a), Tokens(b)); got >= DefaultThreshold {
		t.Errorf("soal berbeda topik tidak boleh melewati ambang, dapat %v", got)
	}
	if IsNearDuplicate(Tokens(a), Tokens(b), DefaultThreshold) {
		t.Error("IsNearDuplicate harus false untuk soal berbeda topik")
	}
}

func TestSimilaritySoalAngkaBerbedaTertandaiSebagaiSatuKeluarga(t *testing.T) {
	// Soal satu keluarga (rumus sama, angka berbeda) memang muncul sebagai soal
	// yang mirip — itu justru yang dicari tim akademik. Yang membedakannya dari
	// soal kembar persis adalah TemplateHash: service menandainya sebagai
	// "keluarga", bukan "identik".
	a := "Diketahui harga barang Rp50.000 dengan diskon 20 persen berapa harga akhir yang harus dibayar pembeli"
	b := "Diketahui harga barang Rp80.000 dengan diskon 15 persen berapa harga akhir yang harus dibayar pembeli"

	if Hash(a) == Hash(b) {
		t.Error("soal dengan angka berbeda bukan kembar persis")
	}
	if TemplateHash(a) != TemplateHash(b) {
		t.Error("soal satu keluarga harus punya template hash sama")
	}
	if !IsNearDuplicate(Tokens(a), Tokens(b), DefaultThreshold) {
		t.Errorf("soal satu keluarga harus muncul sebagai mirip (>= %v)", DefaultThreshold)
	}
}

func TestSimilarityAmanUntukTeksKosong(t *testing.T) {
	if got := Similarity(nil, nil); got != 0 {
		t.Errorf("Similarity(nil, nil) = %v, harap 0", got)
	}
	if got := Similarity(Tokens(""), Tokens("apa pun")); got != 0 {
		t.Errorf("Similarity dengan himpunan kosong = %v, harap 0", got)
	}
}
