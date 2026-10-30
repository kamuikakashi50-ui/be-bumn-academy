package models

import (
	"encoding/json"
	"testing"
)

// Uji JSONMap (kolom JSONB berisi objek) — dipakai metadata sertifikat.

func TestJSONMapRoundTrip(t *testing.T) {
	original := JSONMap{"name": "Dewi Lestari", "org": "Bank Nusantara", "period": 2026}

	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}

	var decoded JSONMap
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if decoded["name"] != "Dewi Lestari" || decoded["org"] != "Bank Nusantara" {
		t.Fatalf("hasil round-trip = %#v", decoded)
	}
	// Angka JSON kembali sebagai float64 — perilaku yang sama seperti kolom JSONB lain.
	if period, ok := decoded["period"].(float64); !ok || period != 2026 {
		t.Errorf("angka pada JSONMap terbaca %#v", decoded["period"])
	}
}

func TestJSONMapNilMenjadiObjekKosong(t *testing.T) {
	value, err := JSONMap(nil).Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if string(value.([]byte)) != "{}" {
		t.Errorf("JSONMap(nil) harus menjadi {}, dapat %s", value)
	}
}

func TestJSONMapScanStringDanKosong(t *testing.T) {
	var decoded JSONMap
	if err := decoded.Scan(`{"a":1}`); err != nil {
		t.Fatalf("Scan string error: %v", err)
	}
	if decoded["a"] != float64(1) {
		t.Errorf("nilai string tidak terbaca: %#v", decoded)
	}

	var empty JSONMap
	if err := empty.Scan([]byte(nil)); err != nil {
		t.Fatalf("Scan kosong error: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("nilai kosong harus menjadi map kosong, dapat %#v", empty)
	}
}

func TestJSONMapScanJSONTidakValid(t *testing.T) {
	var decoded JSONMap
	if err := decoded.Scan([]byte("{bukan json")); err == nil {
		t.Error("JSON tidak valid harus menghasilkan error")
	}
	if err := decoded.Scan(12345); err == nil {
		t.Error("tipe tidak didukung harus menghasilkan error")
	}
}

func TestJSONMapKompatibelDenganMarshal(t *testing.T) {
	// Metadata sertifikat harus bisa dibaca sebagai JSON biasa oleh handler.
	raw, err := json.Marshal(JSONMap{"name": "Rina"})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(raw) != `{"name":"Rina"}` {
		t.Errorf("hasil marshal = %s", raw)
	}
}
