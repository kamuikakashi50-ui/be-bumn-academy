package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDataDemoSesuaiKontrak memastikan jumlah data demo tetap sama dengan yang
// dijanjikan README dan skrip smoke test (45 soal, 5 paket tryout, 4 produk,
// 4 program, 3 voucher, 8 modul & 15 materi ruang kelas).
func TestDataDemoSesuaiKontrak(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}
	classroom, err := loadClassroom()
	if err != nil {
		t.Fatalf("loadClassroom error: %v", err)
	}

	cases := []struct {
		nama  string
		dapat int
		harap int
	}{
		{"akun demo", len(data.Users), 2},
		{"program", len(data.Programs), 4},
		{"kategori materi", len(data.MateriCategories), 4},
		{"produk", len(data.Products), 4},
		{"voucher", len(data.Vouchers), 3},
		{"soal", len(data.Questions), 45},
		{"paket tryout", len(data.Tryouts), 5},
		{"modul ruang kelas", len(classroom.Modules), 8},
	}
	for _, item := range cases {
		if item.dapat != item.harap {
			t.Errorf("jumlah %s = %d, harap %d", item.nama, item.dapat, item.harap)
		}
	}

	lessons := 0
	for _, module := range classroom.Modules {
		lessons += len(module.Lessons)
	}
	if lessons != 15 {
		t.Errorf("jumlah materi ruang kelas = %d, harap 15", lessons)
	}
}

// TestIdSoalUnik memastikan tidak ada id soal yang tertulis dua kali.
// TestSetiapBankSoalDipakaiTryout memastikan tidak ada bank soal yang
// "menganggur": soal pada bank yang tidak dicantumkan subtes mana pun tidak akan
// pernah tampil di sesi tryout (hanya muncul sebagai "belum ditautkan" di panel).
func TestSetiapBankSoalDipakaiTryout(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	used := map[string]bool{}
	for _, tryout := range data.Tryouts {
		for _, subtest := range tryout.Subtests {
			for _, bankKey := range subtest.Banks {
				used[bankKey] = true
			}
		}
	}

	for _, question := range data.Questions {
		if !used[question.BankKey] {
			t.Errorf("bank %s (soal %s) tidak dipakai subtes mana pun", question.BankKey, question.ID)
		}
	}
}

// TestEkspansiSoalPerTryout menjaga jumlah baris soal yang dihasilkan seed.
//
// Sejak migrasi 000011 satu soal dimiliki satu tryout, jadi definisi soal yang
// bank-nya dipakai beberapa tryout digandakan: 45 definisi pada data.json menjadi
// 103 baris di database (angka ini juga disebut pada README).
func TestEkspansiSoalPerTryout(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	banksPerTryout := make([]map[string]bool, 0, len(data.Tryouts))
	for _, tryout := range data.Tryouts {
		banks := map[string]bool{}
		for _, subtest := range tryout.Subtests {
			for _, bankKey := range subtest.Banks {
				banks[bankKey] = true
			}
		}
		banksPerTryout = append(banksPerTryout, banks)
	}

	total := 0
	for _, question := range data.Questions {
		owners := 0
		for _, banks := range banksPerTryout {
			if banks[question.BankKey] {
				owners++
			}
		}
		if owners == 0 {
			owners = 1 // disimpan sebagai soal "belum ditautkan"
		}
		total += owners
	}

	if total != 103 {
		t.Errorf("jumlah baris soal setelah ekspansi per tryout = %d, harap 103", total)
	}
}

func TestIdSoalUnik(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	seen := map[string]bool{}
	for _, question := range data.Questions {
		if seen[question.ID] {
			t.Errorf("id soal %s muncul lebih dari sekali", question.ID)
		}
		seen[question.ID] = true
	}
}

// TestSetiapSubtesPunyaSoal menegaskan setiap subtes mengambil soal dari bank
// yang benar-benar terisi — StartSession menolak subtes tanpa soal.
func TestSetiapSubtesPunyaSoal(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	perBank := map[string]int{}
	for _, question := range data.Questions {
		perBank[question.BankKey]++
	}

	for _, tryout := range data.Tryouts {
		if len(tryout.Subtests) == 0 {
			t.Errorf("paket %s tidak memiliki subtes", tryout.ID)
		}
		for _, subtest := range tryout.Subtests {
			if subtest.Minutes <= 0 {
				t.Errorf("subtes %s pada %s harus punya alokasi waktu", subtest.Name, tryout.ID)
			}
			for _, bank := range subtest.Banks {
				if perBank[bank] == 0 {
					t.Errorf("subtes %s pada %s memakai bank %s yang kosong", subtest.Name, tryout.ID, bank)
				}
			}
		}
	}
}

// TestKontrakSmokeTest memastikan data yang diuji skrip smoke test tersedia:
// produk kelas-intensif & tryout-demo, voucher MULAI20, referral ILHAM10, dan
// paket tryout yang dipakai skrip.
func TestKontrakSmokeTest(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	products := map[string]int64{}
	for _, product := range data.Products {
		products[product.Slug] = product.Price
	}
	if products["kelas-intensif"] != 399000 {
		t.Errorf("harga kelas-intensif = %d, harap 399000", products["kelas-intensif"])
	}
	if price, ok := products["tryout-demo"]; !ok || price != 0 {
		t.Errorf("produk gratis tryout-demo tidak sesuai: ada=%v harga=%d", ok, price)
	}

	tryouts := map[string]TryoutDef{}
	for _, tryout := range data.Tryouts {
		tryouts[tryout.ID] = tryout
		if tryout.RequiredProduct != "" {
			if _, ok := products[tryout.RequiredProduct]; !ok {
				t.Errorf("paket %s memerlukan produk %s yang tidak ada", tryout.ID, tryout.RequiredProduct)
			}
		}
	}
	demo, ok := tryouts["to-demo-nasional"]
	if !ok {
		t.Fatalf("paket to-demo-nasional tidak ditemukan")
	}
	if len(demo.Subtests) != 2 {
		t.Errorf("to-demo-nasional harus punya 2 subtes, dapat %d", len(demo.Subtests))
	}
	if premium, ok := tryouts["to-tkd-01"]; !ok || premium.RequiredProduct == "" {
		t.Errorf("to-tkd-01 harus premium dengan produk wajib")
	}

	referral := map[string]bool{}
	for _, user := range data.Users {
		referral[user.ReferralCode] = true
	}
	if !referral["ILHAM10"] {
		t.Errorf("kode referral ILHAM10 (dipakai smoke test) tidak tersedia")
	}

	voucher := map[string]bool{}
	for _, item := range data.Vouchers {
		voucher[item.Code] = true
	}
	if !voucher["MULAI20"] {
		t.Errorf("voucher MULAI20 (dipakai smoke test) tidak tersedia")
	}
}

// TestMateriVideoMengarahKeBerkasNyata memastikan berkas video/poster ruang
// kelas benar-benar ada di folder media sehingga pemutar video tidak 404.
func TestMateriVideoMengarahKeBerkasNyata(t *testing.T) {
	classroom, err := loadClassroom()
	if err != nil {
		t.Fatalf("loadClassroom error: %v", err)
	}

	root := filepath.Join("..", "..")
	for _, module := range classroom.Modules {
		for _, lesson := range module.Lessons {
			if lesson.Kind != "video" {
				if strings.TrimSpace(lesson.Content) == "" {
					t.Errorf("materi e-book %s tidak punya konten", lesson.ID)
				}
				continue
			}
			for _, relative := range []string{lesson.VideoURL, lesson.PosterURL} {
				if strings.TrimSpace(relative) == "" {
					t.Errorf("materi video %s tidak lengkap", lesson.ID)
					continue
				}
				path := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(relative, "/")))
				if _, statErr := os.Stat(path); statErr != nil {
					t.Errorf("berkas media %s tidak ditemukan: %v", relative, statErr)
				}
			}
		}
	}
}

// TestStableIDDeterministik memastikan id turunan seed tetap sama antar
// pemanggilan sehingga seed berulang tidak membuat baris baru.
func TestStableIDDeterministik(t *testing.T) {
	if stableID("program/kelas-intensif") != stableID("program/kelas-intensif") {
		t.Errorf("stableID tidak deterministik")
	}
	if stableID("program/kelas-intensif") == stableID("program/kelas-live") {
		t.Errorf("stableID menghasilkan id sama untuk kunci berbeda")
	}
}
