package service

import (
	"context"
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji aturan akses materi: materi contoh (`is_preview`) terbuka tanpa kepemilikan
// produk, sedangkan permintaan tanpa peserta selalu ditolak. Kedua cabang ini tidak
// menyentuh database, sehingga bisa diuji sebagai unit test murni.
//
// Cabang kepemilikan produk (enrollment) diuji lewat smoke test end-to-end karena
// memerlukan data nyata; cabang kohort menyusul di langkah 5.

func TestAccessCheckerMembukaMateriContohTanpaProduk(t *testing.T) {
	checker := NewAccessChecker(nil, nil)
	lesson := models.Lesson{ID: "les-1", ProductSlug: "kelas-intensif", IsPreview: true}

	allowed, err := checker.CanOpenLesson(context.Background(), "user-tanpa-produk", lesson)
	if err != nil {
		t.Fatalf("pemeriksaan akses gagal: %v", err)
	}
	if !allowed {
		t.Error("materi contoh harus bisa dibuka tanpa kepemilikan produk")
	}
	if err := checker.RequireLesson(context.Background(), "user-tanpa-produk", lesson); err != nil {
		t.Errorf("RequireLesson untuk materi contoh = %v, harap nil", err)
	}
}

func TestAccessCheckerMenolakTanpaPesertaAtauProduk(t *testing.T) {
	checker := NewAccessChecker(nil, nil)
	ctx := context.Background()

	allowed, err := checker.HasProduct(ctx, "", "kelas-intensif")
	if err != nil {
		t.Fatalf("pemeriksaan akses gagal: %v", err)
	}
	if allowed {
		t.Error("permintaan tanpa peserta tidak boleh dianggap punya akses")
	}

	allowed, err = checker.HasProduct(ctx, "user-1", "")
	if err != nil {
		t.Fatalf("pemeriksaan akses gagal: %v", err)
	}
	if allowed {
		t.Error("produk kosong tidak boleh dianggap dimiliki")
	}

	owned, err := checker.OwnedProducts(ctx, "")
	if err != nil {
		t.Fatalf("daftar produk dimiliki gagal: %v", err)
	}
	if len(owned) != 0 {
		t.Errorf("daftar produk dimiliki untuk peserta kosong harus kosong: %v", owned)
	}
}

// Uji cabang keanggotaan kohort: peserta boleh membuka materi kelas khususnya
// tanpa membeli produk, tetapi modul di luar kurikulum kelasnya tetap ditolak.
// Logika keputusannya murni (data hasil query sudah dimuat), sehingga bisa diuji
// tanpa database.
func TestLessonAllowedByCohort(t *testing.T) {
	cohortModules := map[string]struct{}{"mod-kelas-1": {}, "mod-kelas-2": {}}

	cases := []struct {
		name   string
		lesson models.Lesson
		want   bool
	}{
		{"modul ada di kurikulum kelas", models.Lesson{ID: "les-1", ModuleID: "mod-kelas-1", ProductSlug: "kelas-intensif"}, true},
		{"modul lain pada produk yang sama", models.Lesson{ID: "les-2", ModuleID: "mod-intensif-akhlak", ProductSlug: "kelas-intensif"}, false},
		{"modul kosong tidak boleh lolos", models.Lesson{ID: "les-3", ModuleID: "", ProductSlug: "kelas-intensif"}, false},
	}

	for _, tc := range cases {
		if got := lessonAllowedByCohort(tc.lesson, cohortModules); got != tc.want {
			t.Errorf("%s: lessonAllowedByCohort = %v, harap %v", tc.name, got, tc.want)
		}
	}

	if lessonAllowedByCohort(models.Lesson{ModuleID: "mod-kelas-1"}, nil) {
		t.Error("tanpa keanggotaan kohort, modul apa pun tidak boleh dibuka lewat cabang ini")
	}
}
