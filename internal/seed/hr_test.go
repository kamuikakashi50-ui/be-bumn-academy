package seed

import (
	"strconv"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
)

// Uji kontrak data kohort analitik HR: memastikan seed benar-benar mengisi
// seluruh metrik dasbor /admin/hr (partisipasi, sebaran program, tren enam
// bulan, peserta tanpa percobaan) dan tidak menyentuh akun inti.

func TestKohortHRTidakMenimpaAkunInti(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	core := map[string]bool{}
	for _, user := range data.Users {
		core[user.Email] = true
	}

	if len(hrCohortUsers) == 0 {
		t.Fatal("kohort analitik HR kosong")
	}

	seen := map[string]bool{}
	programs := map[string]bool{}
	for _, user := range hrCohortUsers {
		if core[user.Email] {
			t.Errorf("kohort HR memakai email akun inti %s — skrip verifikasi bisa terganggu", user.Email)
		}
		if seen[user.Email] {
			t.Errorf("email kohort %s muncul lebih dari sekali", user.Email)
		}
		seen[user.Email] = true

		if user.Role != string(models.RolePeserta) {
			t.Errorf("akun kohort %s harus berperan peserta, dapat %q", user.Email, user.Role)
		}
		if user.Password == "" {
			t.Errorf("akun kohort %s tidak punya kata sandi", user.Email)
		}
		programs[user.Program] = true
	}

	// Panel "Sebaran per program" butuh lebih dari satu segmen.
	if len(programs) < 3 {
		t.Errorf("kohort HR hanya punya %d segmen program, harap minimal 3", len(programs))
	}
}

func TestPercobaanHRMenunjukAkunKohort(t *testing.T) {
	cohort := map[string]bool{}
	for _, user := range hrCohortUsers {
		cohort[user.Email] = true
	}

	if len(hrAttemptDefs) == 0 {
		t.Fatal("riwayat latihan kohort HR kosong")
	}

	seen := map[string]bool{}
	for _, def := range hrAttemptDefs {
		if !cohort[def.Email] {
			t.Errorf("percobaan menunjuk %s yang bukan anggota kohort", def.Email)
		}
		if def.Score < 0 || def.Score > 100 {
			t.Errorf("skor %d pada %s di luar rentang 0–100", def.Score, def.Email)
		}
		if def.Minutes <= 0 {
			t.Errorf("durasi percobaan %s harus positif", def.Email)
		}
		if def.DaysAgo < 0 {
			t.Errorf("DaysAgo %d pada %s tidak boleh negatif", def.DaysAgo, def.Email)
		}

		// Kunci stableID = email + tryout + DaysAgo; pasangan email/DaysAgo yang
		// sama akan menghasilkan id kembar.
		key := def.Email + "|" + strconv.Itoa(def.DaysAgo)
		if seen[key] {
			t.Errorf("pasangan email/hari kembar: %s hari ke-%d", def.Email, def.DaysAgo)
		}
		seen[key] = true
	}
}

func TestPercobaanHRMenyebarDalamEnamBulan(t *testing.T) {
	var oldest, newest int
	for index, def := range hrAttemptDefs {
		if index == 0 || def.DaysAgo > oldest {
			oldest = def.DaysAgo
		}
		if index == 0 || def.DaysAgo < newest {
			newest = def.DaysAgo
		}
	}

	// Tren bulanan membaca 6 bulan terakhir (~180 hari).
	if oldest < 150 {
		t.Errorf("percobaan tertua hanya %d hari lalu; tren enam bulan akan kosong", oldest)
	}
	// Tingkat partisipasi butuh aktivitas dalam 30 hari terakhir.
	if newest > 30 {
		t.Errorf("tidak ada percobaan dalam 30 hari terakhir (termuda %d hari)", newest)
	}
}

func TestKohortHRPunyaPesertaTanpaPercobaan(t *testing.T) {
	aktif := map[string]bool{}
	for _, def := range hrAttemptDefs {
		aktif[def.Email] = true
	}

	tanpaPercobaan := 0
	for _, user := range hrCohortUsers {
		if !aktif[user.Email] {
			tanpaPercobaan++
		}
	}
	if tanpaPercobaan == 0 {
		t.Error("tidak ada peserta kohort tanpa percobaan; metrik \"belum pernah latihan\" akan nol")
	}
}

func TestRotasiTryoutHRValid(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}

	tryouts := map[string]TryoutDef{}
	for _, tryout := range data.Tryouts {
		tryouts[tryout.ID] = tryout
	}
	products := map[string]bool{}
	for _, product := range data.Products {
		products[product.Slug] = true
	}
	akses := map[string]bool{}
	for _, slug := range hrProductAccess {
		if !products[slug] {
			t.Errorf("produk %s pada hrProductAccess tidak ada di data.json", slug)
		}
		akses[slug] = true
	}

	if len(hrTryoutRotation) == 0 {
		t.Fatal("rotasi tryout analitik HR kosong")
	}
	for _, id := range hrTryoutRotation {
		tryout, ok := tryouts[id]
		if !ok {
			t.Errorf("paket tryout %s pada rotasi tidak ada di data.json", id)
			continue
		}
		if len(tryout.Subtests) == 0 {
			t.Errorf("paket %s tidak punya subtes; profil kompetensi akan kosong", id)
		}
		// Paket premium hanya bisa dikerjakan bila produk wajibnya dimiliki kohort.
		if tryout.RequiredProduct != "" && !akses[tryout.RequiredProduct] {
			t.Errorf("paket %s memerlukan produk %s yang tidak diberikan ke kohort", id, tryout.RequiredProduct)
		}
	}
}

func TestPolaProgresHRValid(t *testing.T) {
	if hrCompletedThreshold != service.CompletedThreshold {
		t.Fatalf("hrCompletedThreshold = %d, harus sama dengan service.CompletedThreshold (%d)",
			hrCompletedThreshold, service.CompletedThreshold)
	}

	selesai := 0
	for _, percent := range hrProgressPattern {
		if percent < 0 || percent > 100 {
			t.Errorf("persentase progres %d di luar rentang 0–100", percent)
		}
		if percent >= hrCompletedThreshold {
			selesai++
		}
	}
	if selesai == 0 {
		t.Error("tidak ada materi yang selesai pada pola progres HR")
	}
	if selesai == len(hrProgressPattern) {
		t.Error("semua materi selesai pada pola progres HR; kelengkapan belajar jadi 100%")
	}
}

func TestBuildAttemptCategoriesMembatasiSkor(t *testing.T) {
	subtests := []SubtestDef{
		{Name: "Penalaran Numerik"},
		{Name: "Logika Analitis"},
		{Name: "Penalaran Kognitif"},
	}

	categories, correct, total := buildAttemptCategories("attempt-1", 98, 0, subtests)

	if len(categories) != len(subtests) {
		t.Fatalf("jumlah kategori = %d, harap %d", len(categories), len(subtests))
	}
	if total != 30 {
		t.Errorf("total soal = %d, harap 30", total)
	}
	if correct < 0 || correct > total {
		t.Errorf("jumlah benar %d di luar rentang 0–%d", correct, total)
	}

	for index, category := range categories {
		if category.Score < 0 || category.Score > 100 {
			t.Errorf("skor subtes %s = %d di luar rentang 0–100", category.Name, category.Score)
		}
		if category.SortOrder != index {
			t.Errorf("sortOrder %s = %d, harap %d", category.Name, category.SortOrder, index)
		}
		if category.AttemptID != "attempt-1" {
			t.Errorf("attemptID tidak diteruskan: %s", category.AttemptID)
		}
	}

	// Profil peserta berbeda harus menghasilkan pergeseran berbeda.
	lain, _, _ := buildAttemptCategories("attempt-2", 70, 1, subtests)
	if lain[0].Score == categories[0].Score && lain[1].Score == categories[1].Score {
		t.Error("profil kompetensi antar peserta tidak berbeda")
	}
}

func TestAttemptStatusDanClampScore(t *testing.T) {
	if attemptStatus(70, 70) != models.AttemptPassed {
		t.Error("skor tepat di ambang harus dianggap lulus")
	}
	if attemptStatus(69, 70) != models.AttemptFailed {
		t.Error("skor di bawah ambang harus perlu perbaikan")
	}

	cases := map[int]int{-10: 0, 0: 0, 55: 55, 100: 100, 140: 100}
	for input, want := range cases {
		if got := clampScore(input); got != want {
			t.Errorf("clampScore(%d) = %d, harap %d", input, got, want)
		}
	}
}
