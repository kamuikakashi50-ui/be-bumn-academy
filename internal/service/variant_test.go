package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji pemilihan varian: sesi tryout hanya boleh memakai satu soal per grup varian,
// dan pilihannya harus stabil agar seluruh peserta satu paket mengerjakan
// susunan soal yang sama.

func TestPickVariantsMemilihSatuSoalPerGrup(t *testing.T) {
	group := "grup-angka"
	questions := []models.Question{
		{ID: "q-1", VariantGroup: &group},
		{ID: "q-2", VariantGroup: &group},
		{ID: "q-3", VariantGroup: &group},
		{ID: "q-4"},
		{ID: "q-5"},
	}

	got := pickVariants("tryout-1", "numerik", questions)
	if len(got) != 3 {
		t.Fatalf("jumlah soal terpilih = %d, harap 3 (satu per grup + dua soal biasa)", len(got))
	}

	ids := map[string]bool{}
	for _, question := range got {
		ids[question.ID] = true
	}
	if !ids["q-4"] || !ids["q-5"] {
		t.Errorf("soal tanpa grup varian harus tetap dipakai: %v", ids)
	}

	again := pickVariants("tryout-1", "numerik", questions)
	if len(again) != len(got) {
		t.Fatalf("jumlah soal berubah antar pemanggilan: %d vs %d", len(got), len(again))
	}
	for i := range got {
		if got[i].ID != again[i].ID {
			t.Errorf("pilihan varian tidak stabil pada indeks %d: %s vs %s", i, got[i].ID, again[i].ID)
		}
	}
}

func TestPickVariantsTanpaGrupMengembalikanSemuaSoal(t *testing.T) {
	questions := []models.Question{{ID: "q-1"}, {ID: "q-2"}}
	if got := pickVariants("tryout-1", "numerik", questions); len(got) != 2 {
		t.Errorf("tanpa grup varian seluruh soal harus dipakai, dapat %d", len(got))
	}
}

func TestPickVariantsMengabaikanGrupKosong(t *testing.T) {
	empty := ""
	questions := []models.Question{{ID: "q-1", VariantGroup: &empty}, {ID: "q-2"}}
	if got := pickVariants("tryout-1", "numerik", questions); len(got) != 2 {
		t.Errorf("grup varian kosong sama dengan tanpa grup, dapat %d soal", len(got))
	}
}

func TestStableVariantIndexSelaluDalamRentang(t *testing.T) {
	for i := 0; i < 50; i++ {
		tryoutID := "tryout-" + string(rune('a'+i%26))
		index := stableVariantIndex(tryoutID, "numerik", "grup", 3)
		if index < 0 || index > 2 {
			t.Fatalf("indeks varian di luar rentang: %d", index)
		}
	}
	if got := stableVariantIndex("tryout-1", "numerik", "grup", 1); got != 0 {
		t.Errorf("grup berisi satu soal harus memilih indeks 0, dapat %d", got)
	}
}
