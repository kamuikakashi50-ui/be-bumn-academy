package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStringListRoundTrip(t *testing.T) {
	original := StringList{"numerik", "verbal"}

	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}

	var decoded StringList
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(decoded) != 2 || decoded[1] != "verbal" {
		t.Fatalf("hasil round-trip = %#v", decoded)
	}
}

func TestStringListNilMenjadiArrayKosong(t *testing.T) {
	value, err := StringList(nil).Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if string(value.([]byte)) != "[]" {
		t.Fatalf("nil harus disimpan sebagai [], dapat %s", value)
	}

	var decoded StringList
	if err := decoded.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if decoded == nil || len(decoded) != 0 {
		t.Fatalf("Scan(nil) = %#v, harap slice kosong", decoded)
	}
}

func TestStringListScanString(t *testing.T) {
	var decoded StringList
	if err := decoded.Scan(`["a","b"]`); err != nil {
		t.Fatalf("Scan string error: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestStringListScanTipeSalah(t *testing.T) {
	var decoded StringList
	if err := decoded.Scan(42); err == nil {
		t.Fatal("Scan dengan tipe salah seharusnya error")
	}
}

func TestIDListRoundTrip(t *testing.T) {
	original := IDList{{"num-1", "num-2"}, {"verb-1"}}

	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}

	var decoded IDList
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(decoded) != 2 || len(decoded[0]) != 2 || decoded[1][0] != "verb-1" {
		t.Fatalf("hasil round-trip = %#v", decoded)
	}
}

func TestTimeListRoundTrip(t *testing.T) {
	stamp := time.Date(2026, 3, 14, 9, 20, 0, 0, time.UTC)
	original := TimeList{stamp, {}}

	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}

	var decoded TimeList
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(decoded) != 2 || !decoded[0].Equal(stamp) {
		t.Fatalf("hasil round-trip = %#v", decoded)
	}
	if !decoded[1].IsZero() {
		t.Fatalf("elemen kedua harus waktu nol, dapat %v", decoded[1])
	}
}

func TestBaseBeforeCreate(t *testing.T) {
	base := &Base{}
	if err := base.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if base.ID == "" {
		t.Fatal("BeforeCreate harus mengisi ID")
	}

	// ID yang sudah terisi tidak boleh ditimpa.
	existing := &Base{ID: "sudah-ada"}
	if err := existing.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if existing.ID != "sudah-ada" {
		t.Fatalf("ID tertimpa menjadi %q", existing.ID)
	}
}

func TestUserInitials(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Ilham Pratama", "IP"},
		{"Tim Akademik BUMN", "TA"},
		{"Rina", "R"},
	}
	for _, tc := range cases {
		user := User{Name: tc.name, Email: "x@y.z"}
		if got := user.Initials(); got != tc.want {
			t.Errorf("Initials(%q) = %q, harap %q", tc.name, got, tc.want)
		}
	}

	// Nama kosong → jatuh ke huruf pertama email.
	fallback := User{Email: "peserta@bumnacademy.id"}
	if got := fallback.Initials(); got != "P" {
		t.Errorf("Initials fallback = %q, harap P", got)
	}
}

func TestProductIsFree(t *testing.T) {
	if !(Product{Price: 0}).IsFree() {
		t.Error("harga 0 harus dianggap gratis")
	}
	if (Product{Price: 149000}).IsFree() {
		t.Error("harga berbayar tidak boleh dianggap gratis")
	}
}

func TestRoleValid(t *testing.T) {
	if !RolePeserta.Valid() || !RoleAdmin.Valid() {
		t.Error("role bawaan harus valid")
	}
	if Role("guru").Valid() {
		t.Error("role tidak dikenal harus invalid")
	}
}

func TestDifficultyValid(t *testing.T) {
	for _, level := range []Difficulty{DifficultyMudah, DifficultySedang, DifficultySulit} {
		if !level.Valid() {
			t.Errorf("%q harus valid", level)
		}
	}
	if Difficulty("extreme").Valid() {
		t.Error("tingkat kesulitan tidak dikenal harus invalid")
	}
}

func TestSessionIsActive(t *testing.T) {
	if !(TryoutSession{Status: SessionActive}).IsActive() {
		t.Error("sesi active harus dianggap aktif")
	}
	if (TryoutSession{Status: SessionSubmitted}).IsActive() {
		t.Error("sesi submitted tidak boleh dianggap aktif")
	}
}

func TestJSONBKonstanTanpaPanic(t *testing.T) {
	// Memastikan marshal JSON tetap valid untuk struktur bertingkat.
	payload := map[string]any{"order": IDList{{"a"}, {"b", "c"}}}
	if _, err := json.Marshal(payload); err != nil {
		t.Fatalf("marshal error: %v", err)
	}
}
