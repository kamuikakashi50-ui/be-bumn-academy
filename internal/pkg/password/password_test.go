package password

import "testing"

func TestHashDanVerify(t *testing.T) {
	hash, err := Hash("rahasia123")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if hash == "rahasia123" {
		t.Fatal("kata sandi tidak boleh disimpan mentah")
	}
	if !Verify(hash, "rahasia123") {
		t.Fatal("kata sandi benar harus lolos verifikasi")
	}
	if Verify(hash, "rahasia124") {
		t.Fatal("kata sandi salah tidak boleh lolos verifikasi")
	}
}

func TestHashBerbedaSetiapKali(t *testing.T) {
	first, _ := Hash("rahasia123")
	second, _ := Hash("rahasia123")
	if first == second {
		t.Fatal("bcrypt harus memakai salt acak sehingga hash berbeda")
	}
}

func TestVerifyHashTidakValid(t *testing.T) {
	if Verify("bukan-hash", "apa saja") {
		t.Fatal("hash tidak valid harus gagal verifikasi")
	}
}

func TestMinLength(t *testing.T) {
	if MinLength != 8 {
		t.Errorf("MinLength = %d, harap 8", MinLength)
	}
}
