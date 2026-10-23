package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringList dipetakan ke kolom JSONB berisi array string
// (mis. features program, includes produk, banks subtes, options soal).
type StringList []string

// Value mengubah StringList menjadi JSON untuk disimpan.
func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	b, err := json.Marshal([]string(s))
	if err != nil {
		return nil, fmt.Errorf("models: gagal marshal StringList: %w", err)
	}
	return b, nil
}

// Scan membaca nilai JSONB dari database.
func (s *StringList) Scan(src any) error {
	raw, err := toBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*s = StringList{}
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("models: gagal unmarshal StringList: %w", err)
	}
	*s = out
	return nil
}

// IDList dipetakan ke kolom JSONB berisi array-of-array id (snapshot susunan
// soal per subtes pada sesi tryout).
type IDList [][]string

// Value mengubah IDList menjadi JSON untuk disimpan.
func (l IDList) Value() (driver.Value, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	b, err := json.Marshal([][]string(l))
	if err != nil {
		return nil, fmt.Errorf("models: gagal marshal IDList: %w", err)
	}
	return b, nil
}

// Scan membaca nilai JSONB dari database.
func (l *IDList) Scan(src any) error {
	raw, err := toBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*l = IDList{}
		return nil
	}
	var out [][]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("models: gagal unmarshal IDList: %w", err)
	}
	*l = out
	return nil
}

// TimeList dipetakan ke kolom JSONB berisi array waktu (tenggat per subtes).
type TimeList []time.Time

// Value mengubah TimeList menjadi JSON untuk disimpan.
func (t TimeList) Value() (driver.Value, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	b, err := json.Marshal([]time.Time(t))
	if err != nil {
		return nil, fmt.Errorf("models: gagal marshal TimeList: %w", err)
	}
	return b, nil
}

// Scan membaca nilai JSONB dari database.
func (t *TimeList) Scan(src any) error {
	raw, err := toBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*t = TimeList{}
		return nil
	}
	var out []time.Time
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("models: gagal unmarshal TimeList: %w", err)
	}
	*t = out
	return nil
}

func toBytes(src any) ([]byte, error) {
	switch v := src.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("models: tipe %T tidak bisa dipindai ke JSONB", src)
	}
}

// JSONList adalah kolom JSONB generik berisi array objek.
//
// Dipakai untuk struktur bertingkat (mis. daftar catatan analisis AI yang
// berisi judul + penjelasan) tanpa perlu menulis Valuer/Scanner berulang.
type JSONList[T any] []T

// Value mengubah JSONList menjadi JSON untuk disimpan.
func (l JSONList[T]) Value() (driver.Value, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	b, err := json.Marshal([]T(l))
	if err != nil {
		return nil, fmt.Errorf("models: gagal marshal JSONList: %w", err)
	}
	return b, nil
}

// Scan membaca nilai JSONB dari database.
func (l *JSONList[T]) Scan(src any) error {
	raw, err := toBytes(src)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		*l = JSONList[T]{}
		return nil
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("models: gagal unmarshal JSONList: %w", err)
	}
	*l = out
	return nil
}
