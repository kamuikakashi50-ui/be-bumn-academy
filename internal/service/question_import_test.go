package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/validator"
)

func TestNormalizeHeader(t *testing.T) {
	cases := map[string]string{
		"BankKey":     "bankkey",
		"bank_key":    "bankkey",
		"bank-key":    "bankkey",
		"Bank Key":    "bankkey",
		"  Answer  ":  "answer",
		"AnswerIndex": "answerindex",
	}
	for input, want := range cases {
		if got := normalizeHeader(input); got != want {
			t.Errorf("normalizeHeader(%q) = %q, harap %q", input, got, want)
		}
	}
}

func TestParseAnswerIndex(t *testing.T) {
	cases := []struct {
		raw     string
		options int
		want    int
		wantErr bool
	}{
		{"A", 4, 0, false},
		{"B", 4, 1, false},
		{"d", 4, 3, false},
		{"1", 4, 0, false}, // 1-based → indeks 0
		{"4", 4, 3, false}, // 1-based → indeks 3
		{"0", 4, 0, false}, // 0-based → indeks 0
		{"3", 4, 2, false}, // 1-based → indeks 2
		{"E", 4, 0, true},  // di luar jumlah pilihan
		{"5", 4, 0, true},
		{"", 4, 0, true},
		{"xyz", 4, 0, true},
	}
	for _, tc := range cases {
		got, err := parseAnswerIndex(tc.raw, tc.options)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseAnswerIndex(%q, %d) harus error", tc.raw, tc.options)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseAnswerIndex(%q, %d) error tak terduga: %v", tc.raw, tc.options, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseAnswerIndex(%q, %d) = %d, harap %d", tc.raw, tc.options, got, tc.want)
		}
	}
}

func TestCollectOptions(t *testing.T) {
	header := map[string]int{
		"optiona": 0, "optionb": 1, "optionc": 2, "optiond": 3,
		"options": 4, "text": 5,
	}

	t.Run("kolom optionA-D", func(t *testing.T) {
		row := []string{"satu", "dua", "tiga", "empat", "", "soal"}
		got := collectOptions(row, header)
		if len(got) != 4 || got[3] != "empat" {
			t.Fatalf("collectOptions = %#v", got)
		}
	})

	t.Run("kolom options dipisah pipa", func(t *testing.T) {
		row := []string{"", "", "", "", "alpha|beta|gamma", "soal"}
		got := collectOptions(row, header)
		if len(got) != 3 || got[0] != "alpha" || got[2] != "gamma" {
			t.Fatalf("collectOptions = %#v", got)
		}
	})

	t.Run("tidak ada pilihan", func(t *testing.T) {
		row := []string{"", "", "", "", "", "soal"}
		if got := collectOptions(row, header); len(got) != 0 {
			t.Fatalf("collectOptions = %#v, harap kosong", got)
		}
	})
}

func TestReadTabularRowsCSV(t *testing.T) {
	csv := "bankKey,text,answer\nnumerik,\"1 + 1 = ?\",B\n"
	rows, err := readTabularRows("soal.csv", []byte(csv))
	if err != nil {
		t.Fatalf("readTabularRows error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("baris = %d, harap 2", len(rows))
	}
	if rows[1][2] != "B" {
		t.Fatalf("kolom answer = %q", rows[1][2])
	}
}

func TestReadTabularRowsDeteksiKonten(t *testing.T) {
	// Nama berkas tanpa ekstensi: format harus dideteksi dari isi (CSV).
	rows, err := readTabularRows("berkas-tanpa-ekstensi", []byte("a,b\n1,2\n"))
	if err != nil {
		t.Fatalf("deteksi CSV dari isi gagal: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("baris = %d, harap 2", len(rows))
	}
}

func TestReadTabularRowsKosong(t *testing.T) {
	if _, err := readTabularRows("kosong.csv", []byte("")); err == nil {
		t.Fatal("berkas CSV kosong seharusnya mengembalikan error")
	}
}

func TestOptionalString(t *testing.T) {
	if got := optionalString("   "); got != nil {
		t.Errorf("string kosong harus menjadi nil, dapat %#v", got)
	}
	got := optionalString("  kelas-intensif  ")
	if got == nil || *got != "kelas-intensif" {
		t.Errorf("optionalString tidak memangkas spasi: %#v", got)
	}
}

func TestValidateQuestion(t *testing.T) {
	if err := validateQuestion([]string{"a"}, 0); err == nil {
		t.Error("satu pilihan harus ditolak")
	}
	if err := validateQuestion([]string{"a", "b"}, 2); err == nil {
		t.Error("indeks di luar rentang harus ditolak")
	}
	if err := validateQuestion([]string{"a", "b"}, 1); err != nil {
		t.Errorf("soal valid ditolak: %v", err)
	}
}

func TestPreview(t *testing.T) {
	if got := preview("pendek", 20); got != "pendek" {
		t.Errorf("preview = %q", got)
	}
	got := preview("ini pesan yang sangat panjang sekali", 10)
	if len([]rune(got)) != 11 {
		t.Errorf("preview tidak dipotong: %q", got)
	}
}

func TestToQuestionViewBankLabel(t *testing.T) {
	question := models.Question{
		ID:          "num-1",
		BankKey:     "numerik",
		Text:        "3, 6, 11, 18, 27, …",
		Options:     models.StringList{"36", "37", "38", "40"},
		AnswerIndex: 2,
		Explanation: "Selisih naik dua setiap langkah.",
		Difficulty:  models.DifficultySedang,
		Active:      true,
	}

	view := toQuestionView(question)
	if view.BankLabel != "Penalaran Numerik" {
		t.Errorf("BankLabel = %q", view.BankLabel)
	}
	if len(view.Options) != 4 {
		t.Errorf("options = %d, harap 4", len(view.Options))
	}
	if view.AnswerIndex != 2 {
		t.Errorf("answerIndex = %d, harap 2", view.AnswerIndex)
	}
}

func TestSubtestInputValidation(t *testing.T) {
	// Memastikan tag validate pada DTO subtes benar-benar bekerja.
	empty := dto.SubtestInput{}
	if err := validator.Validate(empty); err == nil {
		t.Error("SubtestInput kosong seharusnya gagal validasi")
	}

	valid := dto.SubtestInput{Name: "Penalaran Numerik", Minutes: 6, Banks: []string{"numerik"}}
	if err := validator.Validate(valid); err != nil {
		t.Errorf("SubtestInput valid ditolak: %v", err)
	}

	tooLong := dto.SubtestInput{Name: "X", Minutes: 999, Banks: []string{"numerik"}}
	if err := validator.Validate(tooLong); err == nil {
		t.Error("durasi 999 menit seharusnya ditolak")
	}
}
