package validator

import "testing"

type samplePayload struct {
	Name  string `json:"name" validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"omitempty,oneof=peserta admin"`
}

func TestValidateSukses(t *testing.T) {
	payload := samplePayload{Name: "Ilham", Email: "peserta@bumnacademy.id", Role: "peserta"}
	if err := Validate(payload); err != nil {
		t.Fatalf("payload valid ditolak: %v", err)
	}
}

func TestValidateMenghasilkanPesanPerField(t *testing.T) {
	payload := samplePayload{Name: "Il", Email: "bukan-email"}
	fieldErrors := Validate(payload)
	if fieldErrors == nil {
		t.Fatal("payload tidak valid harus menghasilkan error")
	}
	if _, ok := fieldErrors["name"]; !ok {
		t.Errorf("pesan untuk field name tidak ada: %#v", fieldErrors)
	}
	if _, ok := fieldErrors["email"]; !ok {
		t.Errorf("pesan untuk field email tidak ada: %#v", fieldErrors)
	}
	if len(fieldErrors) != 2 {
		t.Errorf("jumlah field error = %d, harap 2", len(fieldErrors))
	}
}

func TestValidatePesanBahasaIndonesia(t *testing.T) {
	fieldErrors := Validate(samplePayload{Name: "Il", Email: "x"})
	if fieldErrors["name"] == "" || fieldErrors["email"] == "" {
		t.Fatalf("pesan tidak boleh kosong: %#v", fieldErrors)
	}
	if fieldErrors.Error() == "" {
		t.Fatal("Error() harus merangkum seluruh field")
	}
}

func TestValidateNilaiEnum(t *testing.T) {
	fieldErrors := Validate(samplePayload{Name: "Ilham", Email: "a@b.c", Role: "guru"})
	if _, ok := fieldErrors["role"]; !ok {
		t.Fatalf("role di luar daftar harus ditolak: %#v", fieldErrors)
	}
}

func TestValidateNested(t *testing.T) {
	type nested struct {
		Items []struct {
			Name string `json:"name" validate:"required"`
		} `json:"items" validate:"required,min=1,dive"`
	}

	if err := Validate(nested{}); err == nil {
		t.Fatal("slice kosong harus ditolak")
	}

	valid := nested{Items: []struct {
		Name string `json:"name" validate:"required"`
	}{{Name: "Penalaran Numerik"}}}
	if err := Validate(valid); err != nil {
		t.Fatalf("payload bertingkat valid ditolak: %v", err)
	}
}
