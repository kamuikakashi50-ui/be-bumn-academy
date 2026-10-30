// Package models berisi model GORM yang memetakan tabel PostgreSQL.
//
// Catatan konvensi:
//   - Kolom database memakai snake_case, sedangkan JSON memakai camelCase agar
//     langsung cocok dengan tipe TypeScript di front-end bumn-academy.
//   - Primary key UUID memakai tipe string (di-generate di Go) supaya
//     encoding/decoding dengan driver pgx bebas ambiguitas.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Timestamps adalah kolom waktu standar.
type Timestamps struct {
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// Base adalah primary key UUID + kolom waktu standar.
type Base struct {
	ID string `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	Timestamps
}

// IDBase adalah primary key UUID tanpa kolom waktu standar.
//
// Dipakai tabel yang waktu hidupnya diwakili kolom domain sendiri
// (mis. `quiz_attempts.started_at` / `finished_at`, `certificates.issued_at`),
// sehingga tidak punya `created_at`/`updated_at`. Memakai `Base` pada tabel
// seperti itu membuat INSERT gagal karena GORM ikut mengirim kolom yang tidak ada.
type IDBase struct {
	ID string `gorm:"column:id;type:uuid;primaryKey" json:"id"`
}

// BeforeCreate memastikan ID terisi sebelum INSERT (tanpa bergantung pada
// default database, sehingga nilai balik Create selalu berisi ID).
func (b *IDBase) BeforeCreate(*gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

// BeforeCreate memastikan ID terisi sebelum INSERT (tanpa bergantung pada
// default database, sehingga nilai balik Create selalu berisi ID).
func (b *Base) BeforeCreate(*gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

// NewID membungkus pembuatan UUID agar konsisten di seluruh paket.
func NewID() string { return uuid.NewString() }
