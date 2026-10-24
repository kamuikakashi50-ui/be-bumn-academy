// Package seed mengisi data demo be-bumn-academy.
//
// Data demo dipakai agar aplikasi bisa langsung dicoba setelah clone: katalog
// program, kategori materi, produk, metode bayar, voucher, bank soal, paket
// tryout, ruang kelas digital, rekaman kelas, jadwal kelas live, serta dua akun
// demo (peserta & admin).
//
// Berkas JSON di paket ini mengikuti modul mock front-end sehingga angka soal
// dan durasi pada kartu tryout sama dengan yang dilihat peserta:
//
//   - data.json      → katalog, bank soal, paket tryout, akun demo
//   - classroom.json → modul & materi ruang kelas (video mengarah ke media/lessons)
//
// Seluruh operasi bersifat idempoten (upsert memakai kunci unik stabil), jadi
// aman dipanggil setiap kali server start (SEED_ON_START=true) maupun lewat
// `go run ./cmd/seed`.
package seed

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed data.json classroom.json
var files embed.FS

/* ------------------------------ data katalog ------------------------------ */

// Dataset adalah isi berkas data.json.
type Dataset struct {
	Users            []UserDef           `json:"users"`
	Programs         []ProgramDef        `json:"programs"`
	MateriCategories []MateriCategoryDef `json:"materiCategories"`
	Products         []ProductDef        `json:"products"`
	PaymentMethods   []PaymentMethodDef  `json:"paymentMethods"`
	Vouchers         []VoucherDef        `json:"vouchers"`
	Questions        []QuestionDef       `json:"questions"`
	Tryouts          []TryoutDef         `json:"tryouts"`
	Enrollments      []EnrollmentDef     `json:"enrollments"`
}

// UserDef adalah satu akun demo.
type UserDef struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	Program      string `json:"program"`
	ReferralCode string `json:"referralCode"`
}

// ProgramDef adalah satu program pada katalog publik.
type ProgramDef struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	Audience    string   `json:"audience"`
	Format      string   `json:"format"`
	Duration    string   `json:"duration"`
	Features    []string `json:"features"`
	Highlighted bool     `json:"highlighted"`
	SortOrder   int      `json:"sortOrder"`
}

// MateriCategoryDef adalah satu kategori materi.
type MateriCategoryDef struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Topics      []string `json:"topics"`
	SortOrder   int      `json:"sortOrder"`
}

// ProductDef adalah satu produk yang dijual.
// PaymentMethodDef adalah satu metode pembayaran.
type PaymentMethodDef struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Channel   string `json:"channel"`
	Fee       int64  `json:"fee"`
	SortOrder int    `json:"sortOrder"`
}

// VoucherDef adalah satu kode promo.
type VoucherDef struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Value       int64  `json:"value"`
	MinPurchase int64  `json:"minPurchase"`
	MaxUses     *int   `json:"maxUses"`
}

// QuestionDef adalah satu soal pada bank soal.
type QuestionDef struct {
	ID          string   `json:"id"`
	BankKey     string   `json:"bankKey"`
	Text        string   `json:"text"`
	Options     []string `json:"options"`
	AnswerIndex int      `json:"answerIndex"`
	Explanation string   `json:"explanation"`
	Difficulty  string   `json:"difficulty"`
	Position    int      `json:"position"`
}

// TryoutDef adalah satu paket tryout beserta subtesnya.
type TryoutDef struct {
	ID              string       `json:"id"`
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Tier            string       `json:"tier"`
	RequiredProduct string       `json:"requiredProduct"`
	Attempts        int          `json:"attempts"`
	AccessUntil     string       `json:"accessUntil"`
	SortOrder       int          `json:"sortOrder"`
	Subtests        []SubtestDef `json:"subtests"`
}

// SubtestDef adalah satu subtes pada paket tryout.
type SubtestDef struct {
	Name    string   `json:"name"`
	Minutes int      `json:"minutes"`
	Banks   []string `json:"banks"`
}

// EnrollmentDef memberi akses produk ke sebuah akun demo.
type EnrollmentDef struct {
	Email       string `json:"email"`
	ProductSlug string `json:"productSlug"`
	Source      string `json:"source"`
}

/* ------------------------------ ruang kelas ------------------------------- */

// ClassroomDataset adalah isi berkas classroom.json.
type ClassroomDataset struct {
	Modules []ModuleDef `json:"modules"`
}

// ModuleDef adalah satu modul ruang kelas beserta materinya.
type ModuleDef struct {
	ID          string      `json:"id"`
	ProductSlug string      `json:"productSlug"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	SortOrder   int         `json:"sortOrder"`
	Lessons     []LessonDef `json:"lessons"`
}

// LessonDef adalah satu materi belajar (video atau e-book).
type LessonDef struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
	Kind            string `json:"kind"`
	VideoURL        string `json:"videoUrl"`
	EbookURL        string `json:"ebookUrl"`
	PosterURL       string `json:"posterUrl"`
	Content         string `json:"content"`
	DurationSeconds int    `json:"durationSeconds"`
	SortOrder       int    `json:"sortOrder"`
}

// loadDataset membaca data.json yang di-embed.
func loadDataset() (*Dataset, error) {
	raw, err := files.ReadFile("data.json")
	if err != nil {
		return nil, fmt.Errorf("seed: gagal membaca data.json: %w", err)
	}

	var out Dataset
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("seed: data.json tidak valid: %w", err)
	}
	return &out, nil
}

// loadClassroom membaca classroom.json yang di-embed.
func loadClassroom() (*ClassroomDataset, error) {
	raw, err := files.ReadFile("classroom.json")
	if err != nil {
		return nil, fmt.Errorf("seed: gagal membaca classroom.json: %w", err)
	}

	var out ClassroomDataset
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("seed: classroom.json tidak valid: %w", err)
	}
	return &out, nil
}

type ProductDef struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	Type        string   `json:"type"`
	Includes    []string `json:"includes"`
	Badge       string   `json:"badge"`
	SortOrder   int      `json:"sortOrder"`
}
