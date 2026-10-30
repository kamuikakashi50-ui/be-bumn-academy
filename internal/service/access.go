package service

import (
	"context"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// AccessChecker memusatkan aturan akses materi belajar.
//
// Sebelumnya pemeriksaan akses tersebar (`ClassroomService.requireOwnership`).
// Karena jalur belajar & kuis menambah kebutuhan akses yang sama, aturannya
// dikumpulkan di satu tempat agar tidak ada dua definisi "peserta boleh membuka
// materi ini" yang bisa berbeda.
//
// Saat ini dua jalur akses didukung:
//
//  1. kepemilikan produk (`enrollments`) — jalur pembelian;
//  2. `lessons.is_preview` — materi contoh yang boleh dibuka siapa pun.
//
// Jalur ketiga (keanggotaan kohort/kelas khusus) ditambahkan pada langkah Fase 1
// berikutnya, tepat di `HasProduct` — satu titik, bukan di setiap service.
type AccessChecker struct {
	enrollments *repository.EnrollmentRepository
}

// NewAccessChecker membentuk pemeriksa akses materi.
func NewAccessChecker(enrollments *repository.EnrollmentRepository) *AccessChecker {
	return &AccessChecker{enrollments: enrollments}
}

// HasProduct menentukan peserta boleh membuka materi sebuah produk.
func (a *AccessChecker) HasProduct(ctx context.Context, userID, productSlug string) (bool, error) {
	if userID == "" || productSlug == "" {
		return false, nil
	}

	owned, err := a.enrollments.HasProduct(ctx, userID, productSlug)
	if err != nil {
		return false, err
	}
	return owned, nil
}

// OwnedProducts mengembalikan kumpulan slug produk yang boleh diakses peserta.
//
// Dipakai untuk menyaring daftar jalur belajar: jalur yang produknya tidak
// dimiliki tidak ditampilkan, sama seperti daftar ruang kelas pada `/classroom`.
func (a *AccessChecker) OwnedProducts(ctx context.Context, userID string) (map[string]struct{}, error) {
	owned := map[string]struct{}{}
	if userID == "" {
		return owned, nil
	}

	slugs, err := a.enrollments.OwnedSlugs(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, slug := range slugs {
		owned[slug] = struct{}{}
	}
	return owned, nil
}

// RequireProduct mengembalikan FORBIDDEN bila peserta tidak punya akses.
func (a *AccessChecker) RequireProduct(ctx context.Context, userID, productSlug string) error {
	allowed, err := a.HasProduct(ctx, userID, productSlug)
	if err != nil {
		return err
	}
	if !allowed {
		return apperror.Forbidden("Materi ini bagian dari kelas khusus atau produk berbayar.")
	}
	return nil
}

// CanOpenLesson menentukan peserta boleh membuka satu pelajaran.
//
// Materi contoh (`is_preview`) dibuka tanpa akses; sisanya mengikuti kepemilikan
// produk pelajaran tersebut.
func (a *AccessChecker) CanOpenLesson(ctx context.Context, userID string, lesson models.Lesson) (bool, error) {
	if lesson.IsPreview {
		return true, nil
	}
	return a.HasProduct(ctx, userID, lesson.ProductSlug)
}

// RequireLesson mengembalikan FORBIDDEN bila pelajaran tidak boleh dibuka.
func (a *AccessChecker) RequireLesson(ctx context.Context, userID string, lesson models.Lesson) error {
	allowed, err := a.CanOpenLesson(ctx, userID, lesson)
	if err != nil {
		return err
	}
	if !allowed {
		return apperror.Forbidden("Materi ini bagian dari kelas khusus atau produk berbayar.")
	}
	return nil
}
