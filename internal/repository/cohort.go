package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CohortRepository mengakses instansi mitra, kelas khusus (kohort), keanggotaan,
// kurikulum kelas, kode undangan, dan jejak auditnya (Fase 1 langkah 4).
//
// Repository ini sengaja tidak memuat aturan bisnis (kuota, kedaluwarsa kode,
// kelayakan akses): aturan itu hidup di service & model murni agar bisa diuji
// tanpa database. Yang disimpan di sini adalah bentuk query-nya.
type CohortRepository struct {
	db *gorm.DB
}

// NewCohortRepository membentuk repository.
func NewCohortRepository(db *gorm.DB) *CohortRepository {
	return &CohortRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *CohortRepository) DB() *gorm.DB { return r.db }

/* ------------------------------- instansi ---------------------------------- */

// OrganizationFilter adalah filter daftar instansi mitra.
type OrganizationFilter struct {
	Search     string
	SectorKey  string
	ActiveOnly bool
}

// ListOrganizations mengambil daftar instansi mitra.
func (r *CohortRepository) ListOrganizations(ctx context.Context, filter OrganizationFilter) ([]models.Organization, error) {
	query := r.db.WithContext(ctx).Model(&models.Organization{})
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(slug) LIKE ?", like, like)
	}
	if filter.SectorKey != "" {
		query = query.Where("sector_key = ?", filter.SectorKey)
	}
	if filter.ActiveOnly {
		query = query.Where("active = ?", true)
	}

	var out []models.Organization
	err := query.Order("name ASC").Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar instansi.")
	}
	return out, nil
}

// FindOrganization mengambil satu instansi berdasarkan id.
func (r *CohortRepository) FindOrganization(ctx context.Context, id string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.WithContext(ctx).First(&org, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Instansi tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat instansi.")
	}
	return &org, nil
}

// FindOrganizationBySlug mengambil satu instansi berdasarkan slug.
func (r *CohortRepository) FindOrganizationBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.WithContext(ctx).First(&org, "slug = ?", slug).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Instansi tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat instansi.")
	}
	return &org, nil
}

// UpsertOrganization menyimpan instansi (idempoten, dipakai panel admin & seed).
func (r *CohortRepository) UpsertOrganization(ctx context.Context, org *models.Organization) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "slug", "sector_key", "logo_url",
			"pic_name", "pic_email", "pic_phone", "active",
		}),
	}).Create(org).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan instansi.")
	}
	return nil
}

/* -------------------------------- kohort ----------------------------------- */

// CohortFilter adalah filter daftar kelas khusus.
type CohortFilter struct {
	OrgID     string
	Status    string
	SectorKey string
	Search    string
	// ActiveOnly menyaring kelas berstatus `aktif` (dipakai endpoint peserta).
	ActiveOnly bool
}

// ListCohorts mengambil daftar kelas khusus beserta instansinya.
func (r *CohortRepository) ListCohorts(ctx context.Context, filter CohortFilter) ([]models.Cohort, error) {
	query := r.db.WithContext(ctx).Model(&models.Cohort{})
	if filter.OrgID != "" {
		query = query.Where("org_id = ?", filter.OrgID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.ActiveOnly {
		query = query.Where("status = ?", models.CohortAktif)
	}
	if filter.SectorKey != "" {
		query = query.Where("sector_key = ?", filter.SectorKey)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(code) LIKE ?", like, like)
	}

	var out []models.Cohort
	err := query.
		Preload("Organization").
		Order("starts_at DESC NULLS LAST, name ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat daftar kelas khusus.")
	}
	return out, nil
}

// FindCohort mengambil satu kelas beserta instansi & kurikulumnya.
func (r *CohortRepository) FindCohort(ctx context.Context, id string) (*models.Cohort, error) {
	var cohort models.Cohort
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Preload("Curriculum", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, module_id ASC")
		}).
		Preload("Curriculum.Module").
		First(&cohort, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Kelas khusus tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat kelas khusus.")
	}
	return &cohort, nil
}

// FindCohortByCode mengambil satu kelas berdasarkan kode internalnya (label laporan).
func (r *CohortRepository) FindCohortByCode(ctx context.Context, code string) (*models.Cohort, error) {
	var cohort models.Cohort
	err := r.db.WithContext(ctx).
		Preload("Organization").
		First(&cohort, "code = ?", code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Kelas khusus tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat kelas khusus.")
	}
	return &cohort, nil
}

// UpsertCohort menyimpan kelas khusus.
//
// Asosiasi (`Organization`, `Curriculum`) sengaja di-`Omit`: GORM ikut meng-upsert
// asosiasi saat Create, dan itu pernah membuat `duplicate key` pada langkah jalur
// belajar. Kurikulum disimpan lewat `ReplaceCohortCurriculum` dalam transaksi.
func (r *CohortRepository) UpsertCohort(ctx context.Context, cohort *models.Cohort) error {
	err := r.db.WithContext(ctx).
		Omit("Organization", "Curriculum").
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"org_id", "name", "code", "sector_key", "status",
				"starts_at", "ends_at", "quota", "access_until",
				"mentor_user_id", "notes",
			}),
		}).Create(cohort).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan kelas khusus.")
	}
	return nil
}

// SetCohortStatus mengubah status kelas khusus.
//
// Tabel `cohorts` tidak punya kolom `updated_at` (migrasi 000016), jadi hanya
// kolom status yang ditulis — menulis kolom yang tidak ada membuat UPDATE gagal.
func (r *CohortRepository) SetCohortStatus(ctx context.Context, id string, status models.CohortStatus) error {
	result := r.db.WithContext(ctx).Model(&models.Cohort{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mengubah status kelas khusus.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Kelas khusus tidak ditemukan.")
	}
	return nil
}

// UpdateOrganizationFields mengubah sebagian kolom instansi (PATCH panel admin).
func (r *CohortRepository) UpdateOrganizationFields(ctx context.Context, id string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).Model(&models.Organization{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mengubah instansi.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Instansi tidak ditemukan.")
	}
	return nil
}

// CohortCountsByOrg menghitung jumlah kelas per instansi (satu query, tanpa N+1).
func (r *CohortRepository) CohortCountsByOrg(ctx context.Context) (map[string]int, error) {
	type row struct {
		OrgID string
		Total int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.Cohort{}).
		Select("org_id, COUNT(*) AS total").
		Where("org_id IS NOT NULL").
		Group("org_id").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung kelas per instansi.")
	}

	out := make(map[string]int, len(rows))
	for _, item := range rows {
		out[item.OrgID] = item.Total
	}
	return out, nil
}

// UpdateCohortFields mengubah sebagian kolom kelas khusus (PATCH panel admin).
//
// Tanpa kolom `updated_at`: tabel `cohorts` tidak memilikinya (migrasi 000016),
// dan menulis kolom yang tidak ada membuat seluruh UPDATE gagal.
func (r *CohortRepository) UpdateCohortFields(ctx context.Context, id string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).Model(&models.Cohort{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mengubah kelas khusus.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Kelas khusus tidak ditemukan.")
	}
	return nil
}

// CohortMemberCount adalah jumlah anggota sebuah kelas (total & per status).
type CohortMemberCount struct {
	Total     int
	Active    int
	Completed int
	Left      int
}

// CohortMemberCounts menghitung anggota beberapa kelas sekaligus.
func (r *CohortRepository) CohortMemberCounts(ctx context.Context, cohortIDs []string) (map[string]CohortMemberCount, error) {
	out := map[string]CohortMemberCount{}
	if len(cohortIDs) == 0 {
		return out, nil
	}

	type row struct {
		CohortID  string
		Total     int
		Active    int
		Completed int
		Left      int `gorm:"column:left_members"`
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.CohortMember{}).
		Select(`cohort_id, COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = ?) AS active,
		        COUNT(*) FILTER (WHERE status = ?) AS completed,
		        COUNT(*) FILTER (WHERE status = ?) AS left_members`,
			models.CohortMemberAktif, models.CohortMemberLulus, models.CohortMemberKeluar).
		Where("cohort_id IN ?", cohortIDs).
		Group("cohort_id").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung anggota kelas khusus.")
	}

	for _, item := range rows {
		out[item.CohortID] = CohortMemberCount{
			Total:     item.Total,
			Active:    item.Active,
			Completed: item.Completed,
			Left:      item.Left,
		}
	}
	return out, nil
}

// CohortCurriculumCount adalah jumlah modul sebuah kelas (total & wajib).
type CohortCurriculumCount struct {
	Total    int
	Required int
}

// CohortCurriculumCounts menghitung modul kurikulum beberapa kelas sekaligus.
func (r *CohortRepository) CohortCurriculumCounts(ctx context.Context, cohortIDs []string) (map[string]CohortCurriculumCount, error) {
	out := map[string]CohortCurriculumCount{}
	if len(cohortIDs) == 0 {
		return out, nil
	}

	type row struct {
		CohortID string
		Total    int
		Required int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.CohortCurriculumItem{}).
		Select("cohort_id, COUNT(*) AS total, COUNT(*) FILTER (WHERE required) AS required").
		Where("cohort_id IN ?", cohortIDs).
		Group("cohort_id").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung kurikulum kelas khusus.")
	}

	for _, item := range rows {
		out[item.CohortID] = CohortCurriculumCount{Total: item.Total, Required: item.Required}
	}
	return out, nil
}

// CohortInviteCounts menghitung kode undangan beberapa kelas sekaligus.
func (r *CohortRepository) CohortInviteCounts(ctx context.Context, cohortIDs []string) (map[string]int, error) {
	out := map[string]int{}
	if len(cohortIDs) == 0 {
		return out, nil
	}

	type row struct {
		CohortID string
		Total    int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.CohortInvite{}).
		Select("cohort_id, COUNT(*) AS total").
		Where("cohort_id IN ?", cohortIDs).
		Group("cohort_id").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung kode undangan kelas khusus.")
	}

	for _, item := range rows {
		out[item.CohortID] = item.Total
	}
	return out, nil
}

/* ------------------------- kohort seorang peserta -------------------------- */
// activeCohortsForUserQuery menyusun query kelas aktif seorang peserta.
//
// Dipisah dari pemanggilnya supaya bentuk query (status keanggotaan, status kelas,
// dan periode kelas) bisa diuji tanpa database. Masa akses materi (`access_until`)
// diperiksa pada query modul, bukan di sini: kelas yang masa aksesnya lewat tetap
// terlihat sebagai kelas yang diikuti (riwayat & sertifikatnya masih bisa dibuka).
func (r *CohortRepository) activeCohortsForUserQuery(ctx context.Context, userID string, now time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&models.Cohort{}).
		Joins("JOIN cohort_members ON cohort_members.cohort_id = cohorts.id").
		Where("cohort_members.user_id = ?", userID).
		Where("cohort_members.status = ?", models.CohortMemberAktif).
		Where("cohorts.status = ?", models.CohortAktif).
		Where("cohorts.starts_at IS NULL OR cohorts.starts_at <= ?", now).
		Where("cohorts.ends_at IS NULL OR cohorts.ends_at >= ?", now)
}

// ActiveCohortsForUser mengambil kelas yang sedang diikuti peserta.
//
// Dipakai halaman `/dashboard/kelas-khusus` dan penentuan konteks kohort saat
// peserta memulai tryout (agar `attempts.cohort_id` terisi).
func (r *CohortRepository) ActiveCohortsForUser(ctx context.Context, userID string, now time.Time) ([]models.Cohort, error) {
	if userID == "" {
		return nil, nil
	}

	var out []models.Cohort
	err := r.activeCohortsForUserQuery(ctx, userID, now).
		Preload("Organization").
		Order("cohorts.starts_at DESC NULLS LAST, cohorts.name ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kelas khusus peserta.")
	}
	return out, nil
}

// activeCohortModuleQuery menyusun query modul kelas yang boleh dibuka peserta.
//
// Masa akses (`access_until`) diikutkan di sini: setelah lewat, materi kelas
// ditutup walau keanggotaan masih aktif — riwayat & sertifikat tetap bisa dilihat.
func (r *CohortRepository) activeCohortModuleQuery(ctx context.Context, userID string, now time.Time) *gorm.DB {
	return r.db.WithContext(ctx).Model(&models.CohortCurriculumItem{}).
		Distinct("cohort_curriculum.module_id").
		Joins("JOIN cohorts ON cohorts.id = cohort_curriculum.cohort_id").
		Joins("JOIN cohort_members ON cohort_members.cohort_id = cohorts.id").
		Where("cohort_members.user_id = ?", userID).
		Where("cohort_members.status = ?", models.CohortMemberAktif).
		Where("cohorts.status = ?", models.CohortAktif).
		Where("cohorts.starts_at IS NULL OR cohorts.starts_at <= ?", now).
		Where("cohorts.ends_at IS NULL OR cohorts.ends_at >= ?", now).
		Where("cohorts.access_until IS NULL OR cohorts.access_until > ?", now)
}

// ActiveCohortModuleIDs mengembalikan id modul yang boleh dibuka peserta karena
// keanggotaan kohort aktifnya.
//
// Inilah jalur akses kedua di samping kepemilikan produk; `AccessChecker`
// memakainya pada langkah 5 sehingga peserta anggota kelas bisa membuka modul
// kelas tanpa membeli produk.
func (r *CohortRepository) ActiveCohortModuleIDs(ctx context.Context, userID string, now time.Time) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if userID == "" {
		return out, nil
	}

	var ids []string
	err := r.activeCohortModuleQuery(ctx, userID, now).Pluck("cohort_curriculum.module_id", &ids).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat modul kelas khusus peserta.")
	}
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

// ActiveOwnerOrgIDs mengembalikan id instansi dari kohort aktif seorang peserta.
//
// Dipakai menyaring bank soal per instansi (aturan §4.5): peserta hanya boleh
// memakai soal bersama + soal milik instansi kelas yang sedang ia ikuti. Daftar
// kosong berarti peserta tidak berada di kelas instansi mana pun.
func (r *CohortRepository) ActiveOwnerOrgIDs(ctx context.Context, userID string, now time.Time) ([]string, error) {
	if userID == "" {
		return nil, nil
	}

	var ids []string
	err := r.activeCohortsForUserQuery(ctx, userID, now).
		Distinct("cohorts.org_id").
		Where("cohorts.org_id IS NOT NULL").
		Pluck("cohorts.org_id", &ids).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat instansi kelas khusus peserta.")
	}
	return ids, nil
}

// ActiveCohortProductSlugs mengembalikan slug produk yang punya modul di kelas
// aktif seorang peserta.
//
// Dipakai agar ruang kelas & jalur belajar produk itu muncul di daftar peserta
// (aksesnya lewat keanggotaan kohort, bukan pembelian).
func (r *CohortRepository) ActiveCohortProductSlugs(ctx context.Context, userID string, now time.Time) ([]string, error) {
	if userID == "" {
		return nil, nil
	}

	var slugs []string
	err := r.db.WithContext(ctx).Model(&models.CohortCurriculumItem{}).
		Distinct("course_modules.product_slug").
		Joins("JOIN course_modules ON course_modules.id = cohort_curriculum.module_id").
		Joins("JOIN cohorts ON cohorts.id = cohort_curriculum.cohort_id").
		Joins("JOIN cohort_members ON cohort_members.cohort_id = cohorts.id").
		Where("cohort_members.user_id = ?", userID).
		Where("cohort_members.status = ?", models.CohortMemberAktif).
		Where("cohorts.status = ?", models.CohortAktif).
		Where("cohorts.starts_at IS NULL OR cohorts.starts_at <= ?", now).
		Where("cohorts.ends_at IS NULL OR cohorts.ends_at >= ?", now).
		Where("cohorts.access_until IS NULL OR cohorts.access_until > ?", now).
		Pluck("course_modules.product_slug", &slugs).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat produk kelas khusus peserta.")
	}
	return slugs, nil
}

/* ------------------------------ keanggotaan -------------------------------- */

// CohortMemberRow adalah anggota kelas beserta identitas penggunanya.
type CohortMemberRow struct {
	UserID   string
	Name     string
	Email    string
	Role     string
	Status   string
	JoinedAt time.Time
}

// ListCohortMembers mengambil anggota sebuah kelas beserta nama & emailnya.
//
// Memakai JOIN (bukan N+1) karena panel kelas menampilkan identitas peserta.
func (r *CohortRepository) ListCohortMembers(ctx context.Context, cohortID string) ([]CohortMemberRow, error) {
	var rows []CohortMemberRow
	err := r.db.WithContext(ctx).Model(&models.CohortMember{}).
		Select(`cohort_members.user_id, users.name, users.email,
		        cohort_members.role, cohort_members.status, cohort_members.joined_at`).
		Joins("JOIN users ON users.id = cohort_members.user_id").
		Where("cohort_members.cohort_id = ?", cohortID).
		Order("cohort_members.joined_at ASC, users.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat anggota kelas khusus.")
	}
	return rows, nil
}

// CountCohortMembers menghitung anggota kelas dengan status tertentu.
//
// Dipakai menegakkan kuota kelas: hanya anggota `aktif` yang memakai jatah kuota.
func (r *CohortRepository) CountCohortMembers(ctx context.Context, cohortID string, status models.CohortMemberStatus) (int, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.CohortMember{}).
		Where("cohort_id = ? AND status = ?", cohortID, status).
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung anggota kelas khusus.")
	}
	return int(total), nil
}

// FindCohortMember mengambil satu keanggotaan; (nil, nil) bila belum menjadi anggota.
func (r *CohortRepository) FindCohortMember(ctx context.Context, cohortID, userID string) (*models.CohortMember, error) {
	var member models.CohortMember
	err := r.db.WithContext(ctx).First(&member, "cohort_id = ? AND user_id = ?", cohortID, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memuat keanggotaan kelas khusus.")
	}
	return &member, nil
}

// UpsertCohortMembers menambahkan/memperbarui anggota kelas secara massal.
//
// Idempoten: peserta yang sudah menjadi anggota hanya diperbarui peran/statusnya,
// sehingga import ulang daftar peserta tidak menggandakan keanggotaan dan tanggal
// bergabung yang asli tidak ikut berubah.
func (r *CohortRepository) UpsertCohortMembers(ctx context.Context, members []models.CohortMember) error {
	if len(members) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cohort_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role", "status"}),
	}).Create(&members).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan anggota kelas khusus.")
	}
	return nil
}

// SetMemberStatus mengubah status keanggotaan (keluar, lulus, atau aktif kembali).
func (r *CohortRepository) SetMemberStatus(ctx context.Context, cohortID, userID string, status models.CohortMemberStatus) error {
	result := r.db.WithContext(ctx).Model(&models.CohortMember{}).
		Where("cohort_id = ? AND user_id = ?", cohortID, userID).
		Update("status", status)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mengubah status anggota kelas khusus.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Anggota kelas khusus tidak ditemukan.")
	}
	return nil
}

// RemoveCohortMember mengeluarkan anggota dari kelas (mis. salah menambahkan).
func (r *CohortRepository) RemoveCohortMember(ctx context.Context, cohortID, userID string) error {
	result := r.db.WithContext(ctx).
		Where("cohort_id = ? AND user_id = ?", cohortID, userID).
		Delete(&models.CohortMember{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mengeluarkan anggota kelas khusus.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Anggota kelas khusus tidak ditemukan.")
	}
	return nil
}

/* --------------------------- bergabung ke kelas --------------------------- */

// JoinParams adalah data yang ditulis saat peserta bergabung ke sebuah kelas.
type JoinParams struct {
	Member models.CohortMember
	// InviteCode diisi hanya bila pemakaian kode undangan harus dicatat; kosong
	// berarti peserta hanya diaktifkan kembali (kode tidak dipakai ulang).
	InviteCode string
	// Audit diisi untuk mencatat aksi pada jejak audit kelas.
	Audit *models.CohortAuditLog
}

// JoinCohort menulis keanggotaan, pemakaian kode undangan, dan jejak audit dalam
// satu transaksi.
//
// Tanpa transaksi, laporan pemakaian kode (`used_count`) bisa terpisah dari
// keanggotaan yang sebenarnya terbentuk — angka kuota kode jadi tidak bisa
// dipercaya. Idempoten lewat konflik (cohort_id, user_id).
func (r *CohortRepository) JoinCohort(ctx context.Context, params JoinParams) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "cohort_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"role", "status"}),
		}).Create(&params.Member).Error; err != nil {
			return err
		}

		if params.InviteCode != "" {
			result := tx.Model(&models.CohortInvite{}).
				Where("code = ?", params.InviteCode).
				Update("used_count", gorm.Expr("used_count + 1"))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("kode undangan %s tidak ditemukan", params.InviteCode)
			}
		}

		if params.Audit != nil {
			return tx.Create(params.Audit).Error
		}
		return nil
	})
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan keanggotaan kelas khusus.")
	}
	return nil
}

/* ------------------------------- kurikulum --------------------------------- */

// ListCohortCurriculum mengambil kurikulum kelas beserta modulnya.
func (r *CohortRepository) ListCohortCurriculum(ctx context.Context, cohortID string) ([]models.CohortCurriculumItem, error) {
	var out []models.CohortCurriculumItem
	err := r.db.WithContext(ctx).
		Preload("Module").
		Where("cohort_id = ?", cohortID).
		Order("sort_order ASC, module_id ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kurikulum kelas khusus.")
	}
	return out, nil
}

// ReplaceCohortCurriculum mengganti seluruh kurikulum kelas dalam satu transaksi.
//
// Susunan yang tersimpan selalu sama dengan yang dikirim admin (delete-then-insert),
// mengikuti pola penyusunan langkah jalur belajar.
func (r *CohortRepository) ReplaceCohortCurriculum(ctx context.Context, cohortID string, items []models.CohortCurriculumItem) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cohort_id = ?", cohortID).Delete(&models.CohortCurriculumItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan kurikulum kelas khusus.")
	}
	return nil
}

/* ------------------------------ kode undangan ------------------------------ */
// ListCohortInvites mengambil kode undangan sebuah kelas (termasuk yang dicabut).
func (r *CohortRepository) ListCohortInvites(ctx context.Context, cohortID string) ([]models.CohortInvite, error) {
	var out []models.CohortInvite
	err := r.db.WithContext(ctx).
		Where("cohort_id = ?", cohortID).
		Order("active DESC, code ASC").
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat kode undangan.")
	}
	return out, nil
}

// FindInvite mengambil satu kode undangan beserta kelasnya.
func (r *CohortRepository) FindInvite(ctx context.Context, code string) (*models.CohortInvite, error) {
	var invite models.CohortInvite
	err := r.db.WithContext(ctx).
		Preload("Cohort").
		First(&invite, "code = ?", code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Kode undangan tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat kode undangan.")
	}
	return &invite, nil
}

// UpsertInvite menyimpan kode undangan (idempoten, dipakai panel admin & seed).
func (r *CohortRepository) UpsertInvite(ctx context.Context, invite *models.CohortInvite) error {
	err := r.db.WithContext(ctx).
		Omit("Cohort").
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"cohort_id", "created_by", "max_uses", "used_count", "expires_at", "active",
			}),
		}).Create(invite).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan kode undangan.")
	}
	return nil
}

// SetInviteActive mencabut (`false`) atau mengaktifkan kembali sebuah kode undangan.
func (r *CohortRepository) SetInviteActive(ctx context.Context, code string, active bool) error {
	result := r.db.WithContext(ctx).Model(&models.CohortInvite{}).
		Where("code = ?", code).
		Update("active", active)
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mengubah status kode undangan.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Kode undangan tidak ditemukan.")
	}
	return nil
}

// IncrementInviteUse menambah pemakaian kode undangan satu langkah.
//
// Dijalankan sebagai UPDATE atomik (`used_count = used_count + 1`) supaya dua
// peserta yang memakai kode bersamaan tidak melewati kuota karena saling menimpa.
func (r *CohortRepository) IncrementInviteUse(ctx context.Context, code string) error {
	result := r.db.WithContext(ctx).Model(&models.CohortInvite{}).
		Where("code = ?", code).
		Update("used_count", gorm.Expr("used_count + 1"))
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal mencatat pemakaian kode undangan.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Kode undangan tidak ditemukan.")
	}
	return nil
}

/* ------------------------------- jejak audit ------------------------------- */

// AppendCohortAudit mencatat satu aksi pada jejak audit kelas.
func (r *CohortRepository) AppendCohortAudit(ctx context.Context, log *models.CohortAuditLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat jejak audit kelas khusus.")
	}
	return nil
}

// ListCohortAudit mengambil jejak audit sebuah kelas (terbaru lebih dulu).
func (r *CohortRepository) ListCohortAudit(ctx context.Context, cohortID string, limit int) ([]models.CohortAuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var out []models.CohortAuditLog
	err := r.db.WithContext(ctx).
		Where("cohort_id = ?", cohortID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jejak audit kelas khusus.")
	}
	return out, nil
}
