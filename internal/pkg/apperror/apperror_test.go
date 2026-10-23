package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestFrom(t *testing.T) {
	original := NotFound("Data tidak ditemukan.")
	got := From(original)
	if got != original {
		t.Fatal("From harus mengembalikan error aplikasi yang sama")
	}
	if got.Status != http.StatusNotFound || got.Code != CodeNotFound {
		t.Fatalf("status/kode = %d/%s", got.Status, got.Code)
	}
}

func TestFromErrorBiasaMenjadiInternal(t *testing.T) {
	got := From(errors.New("koneksi putus"))
	if got.Code != CodeInternal || got.Status != http.StatusInternalServerError {
		t.Fatalf("error biasa harus menjadi INTERNAL_ERROR, dapat %s/%d", got.Code, got.Status)
	}
	if got.Message == "" {
		t.Fatal("pesan default tidak boleh kosong")
	}
}

func TestFromErrorTerbungkus(t *testing.T) {
	wrapped := fmt.Errorf("lapisan service: %w", Conflict("Email sudah dipakai."))
	got := From(wrapped)
	if got.Code != CodeConflict {
		t.Fatalf("errors.As harus menembus pembungkusan, dapat %s", got.Code)
	}
}

func TestFromNil(t *testing.T) {
	if got := From(nil); got != nil {
		t.Fatalf("From(nil) harus nil, dapat %v", got)
	}
}

func TestWithDetails(t *testing.T) {
	details := map[string]string{"email": "wajib diisi"}
	err := Validation("Input tidak valid.", details)
	if err.Details == nil {
		t.Fatal("details tidak boleh hilang")
	}
	if err.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", err.Status)
	}
}

func TestInternalMembungkusErrorAsli(t *testing.T) {
	cause := errors.New("query gagal")
	err := Internal(cause, "Gagal memuat data.")
	if !errors.Is(err, cause) {
		t.Fatal("error asli harus bisa ditelusuri dengan errors.Is")
	}
	if err.Error() == "" {
		t.Fatal("Error() tidak boleh kosong")
	}
}

func TestStatusSetiapConstructor(t *testing.T) {
	cases := map[*Error]int{
		BadRequest("x"):      http.StatusBadRequest,
		Unauthorized("x"):    http.StatusUnauthorized,
		Forbidden("x"):       http.StatusForbidden,
		NotFound("x"):        http.StatusNotFound,
		Conflict("x"):        http.StatusConflict,
		Locked("x"):          http.StatusLocked,
		Validation("x", nil): http.StatusUnprocessableEntity,
	}
	for err, want := range cases {
		if err.Status != want {
			t.Errorf("%s status = %d, harap %d", err.Code, err.Status, want)
		}
	}
}
