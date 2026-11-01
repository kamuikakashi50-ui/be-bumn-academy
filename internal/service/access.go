package service

import (
	"context"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// AccessChecker memusatkan aturan akses materi belajar.
//
// Sebelumnya pemeriksaan akses tersebar (`ClassroomService.requireOwnership`).
// Karena jalur belajar, kuis, e-book, dan kelas khusus menambah kebutuhan akses
// yang sama, aturannya dikumpulkan di satu tempat agar tidak ada dua definisi
// "peserta boleh membuka materi ini" yang bisa berbeda.
//
// Tiga jalur akses yang didukung:
//
//  1. kepemilikan produk (`enrollments`) — jalur pembelian;
//  2. keanggotaan kohort aktif — modul yang ada di kurikulum kelasnya (§4.1);
//  3. `lessons.is_preview` — materi contoh yang boleh dibuka siapa pun.
//
// Pembedaan yang penting: `HasProduct` (akses tingkat produk, dipakai daftar ruang
// kelas & jalur belajar) menerima jalur 1 & 2, sedangkan `CanOpenLesson` (akses
// tingkat pelajaran) menerima jalur 2 hanya bila **modul** pelajaran itu memang ada
// di kurikulum kelas — sehingga anggota kelas tidak bisa membuka modul lain dari
// produk yang sama.
type AccessChecker struct {
	enrollments *repository.EnrollmentRepository
	// cohorts opsional: nil berarti tidak ada konteks kelas khusus (mis. paket uji).
	cohorts *repository.CohortRepository
}

// NewAccessChecker membentuk pemeriksa akses materi.
func NewAccessChecker(
	enrollments *repository.EnrollmentRepository,
	cohorts *repository.CohortRepository,
) *AccessChecker {
	return &AccessChecker{enrollments: enrollments, cohorts: cohorts}
}

/* ------------------------- kepemilikan produk ------------------------------ */

// OwnsProduct menentukan peserta benar-benar memiliki produk (membelinya).
func (a *AccessChecker) OwnsProduct(ctx context.Context, userID, productSlug string) (bool, error) {
	if userID == "" || productSlug == "" || a.enrollments == nil {
		return false, nil
	}
	return a.enrollments.HasProduct(ctx, userID, productSlug)
}

// HasProduct menentukan peserta boleh membuka ruang kelas sebuah produk — baik
// karena memilikinya maupun karena produk itu punya modul di kelas khusus yang
// sedang ia ikuti.
func (a *AccessChecker) HasProduct(ctx context.Context, userID, productSlug string) (bool, error) {
	if userID == "" || productSlug == "" {
		return false, nil
	}

	owned, err := a.OwnsProduct(ctx, userID, productSlug)
	if err != nil || owned {
		return owned, err
	}

	slugs, err := a.CohortProductSlugs(ctx, userID)
	if err != nil {
		return false, err
	}
	_, ok := slugs[productSlug]
	return ok, nil
}

// OwnedProducts mengembalikan kumpulan slug produk yang **dimiliki** peserta.
//
// Dipakai ketika yang dibutuhkan hanya kepemilikan (mis. cek pembelian produk).
func (a *AccessChecker) OwnedProducts(ctx context.Context, userID string) (map[string]struct{}, error) {
	owned := map[string]struct{}{}
	if userID == "" || a.enrollments == nil {
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

// AccessibleProducts mengembalikan produk yang boleh dibuka peserta: yang dimiliki
// ditambah produk yang muncul di kurikulum kelas khususnya.
//
// Dipakai daftar ruang kelas & daftar jalur belajar agar peserta anggota kelas
// tetap melihat kelasnya walau tidak membeli produk.
func (a *AccessChecker) AccessibleProducts(ctx context.Context, userID string) (map[string]struct{}, error) {
	out, err := a.OwnedProducts(ctx, userID)
	if err != nil {
		return nil, err
	}

	slugs, err := a.CohortProductSlugs(ctx, userID)
	if err != nil {
		return nil, err
	}
	for slug := range slugs {
		out[slug] = struct{}{}
	}
	return out, nil
}

/* --------------------------- keanggotaan kohort ---------------------------- */

// CohortModuleIDs mengembalikan id modul dari kurikulum kelas khusus aktif peserta.
func (a *AccessChecker) CohortModuleIDs(ctx context.Context, userID string) (map[string]struct{}, error) {
	if userID == "" || a.cohorts == nil {
		return map[string]struct{}{}, nil
	}
	return a.cohorts.ActiveCohortModuleIDs(ctx, userID, time.Now().UTC())
}

// CohortProductSlugs mengembalikan produk yang punya modul di kelas khusus aktif
// peserta.
func (a *AccessChecker) CohortProductSlugs(ctx context.Context, userID string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if userID == "" || a.cohorts == nil {
		return out, nil
	}

	slugs, err := a.cohorts.ActiveCohortProductSlugs(ctx, userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	for _, slug := range slugs {
		out[slug] = struct{}{}
	}
	return out, nil
}

// PrimaryCohort mengembalikan kelas khusus utama peserta (kelas aktif dengan
// tanggal mulai terbaru), lengkap dengan instansinya.
//
// Dipakai sertifikat: sertifikat peserta kelas menyebut instansinya dan menautkan
// `certificates.cohort_id`. Mengembalikan (nil, nil) bila peserta tidak mengikuti
// kelas khusus mana pun.
func (a *AccessChecker) PrimaryCohort(ctx context.Context, userID string) (*models.Cohort, error) {
	if userID == "" || a.cohorts == nil {
		return nil, nil
	}

	cohorts, err := a.cohorts.ActiveCohortsForUser(ctx, userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if len(cohorts) == 0 {
		return nil, nil
	}
	return &cohorts[0], nil
}

/* ---------------------------- keputusan akses ------------------------------ */
// RequireProduct mengembalikan FORBIDDEN bila peserta tidak punya akses ke produk.
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
// Materi contoh (`is_preview`) dibuka tanpa akses; materi lain mengikuti
// kepemilikan produk **atau** keanggotaan kohort yang kurikulumnya memuat modul
// pelajaran itu.
func (a *AccessChecker) CanOpenLesson(ctx context.Context, userID string, lesson models.Lesson) (bool, error) {
	if lesson.IsPreview {
		return true, nil
	}

	owned, err := a.OwnsProduct(ctx, userID, lesson.ProductSlug)
	if err != nil || owned {
		return owned, err
	}

	modules, err := a.CohortModuleIDs(ctx, userID)
	if err != nil {
		return false, err
	}
	return lessonAllowedByCohort(lesson, modules), nil
}

// lessonAllowedByCohort menentukan pelajaran boleh dibuka karena modulnya ada di
// kurikulum kelas khusus peserta (bentuk murni, agar bisa diuji tanpa database).
func lessonAllowedByCohort(lesson models.Lesson, cohortModules map[string]struct{}) bool {
	if lesson.ModuleID == "" {
		return false
	}
	_, ok := cohortModules[lesson.ModuleID]
	return ok
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
