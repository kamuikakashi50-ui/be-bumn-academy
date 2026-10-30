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
	checker := NewAccessChecker(nil)
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
	checker := NewAccessChecker(nil)
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
