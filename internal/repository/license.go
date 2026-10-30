package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
)

// LicenseRepository mengakses izin/hak pakai soal dan instrumen psikologi.
type LicenseRepository struct {
	db *gorm.DB
}

// NewLicenseRepository membentuk repository.
func NewLicenseRepository(db *gorm.DB) *LicenseRepository {
	return &LicenseRepository{db: db}
}

// LicenseFilter adalah filter daftar izin soal.
type LicenseFilter struct {
	QuestionID string
	Scope      string
	Status     string
	Search     string
	// OnlyProblem membatasi hasil pada izin yang tidak aktif (kedaluwarsa/dicabut).
	OnlyProblem bool
}

// ListLicenses mengambil daftar izin soal dengan paginasi.
func (r *LicenseRepository) ListLicenses(ctx context.Context, filter LicenseFilter, page, perPage int) ([]models.QuestionLicense, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.QuestionLicense{})
	if filter.QuestionID != "" {
		query = query.Where("question_id = ?", filter.QuestionID)
	}
	if filter.Scope != "" {
		query = query.Where("scope = ?", filter.Scope)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.OnlyProblem {
		query = query.Where("status <> ? OR (valid_until IS NOT NULL AND valid_until < ?)",
			models.LicenseActive, time.Now().UTC())
	}
	if filter.Search != "" {
		query = query.Where("owner ILIKE ? OR agreement_no ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.Internal(err, "Gagal menghitung izin soal.")
	}

	var out []models.QuestionLicense
	err := query.Order("created_at DESC, id ASC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out).Error
	if err != nil {
		return nil, 0, apperror.Internal(err, "Gagal memuat izin soal.")
	}
	return out, total, nil
}

// FindLicense mengambil satu izin soal.
func (r *LicenseRepository) FindLicense(ctx context.Context, id string) (*models.QuestionLicense, error) {
	var license models.QuestionLicense
	err := r.db.WithContext(ctx).First(&license, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Izin soal tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat izin soal.")
	}
	return &license, nil
}

// CreateLicense menyimpan izin soal baru.
func (r *LicenseRepository) CreateLicense(ctx context.Context, license *models.QuestionLicense) error {
	if license.ID == "" {
		license.ID = models.NewID()
	}
	if err := r.db.WithContext(ctx).Create(license).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan izin soal.")
	}
	return nil
}

// UpdateLicense menyimpan perubahan izin soal.
func (r *LicenseRepository) UpdateLicense(ctx context.Context, license *models.QuestionLicense) error {
	if err := r.db.WithContext(ctx).Save(license).Error; err != nil {
		return apperror.Internal(err, "Gagal memperbarui izin soal.")
	}
	return nil
}

// DeleteLicense menghapus izin soal.
func (r *LicenseRepository) DeleteLicense(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.QuestionLicense{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus izin soal.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Izin soal tidak ditemukan.")
	}
	return nil
}

// LicensesByQuestion mengelompokkan izin per soal.
//
// Dipakai panel bank soal untuk menampilkan penanda "izin belum lengkap" tanpa
// query per baris.
func (r *LicenseRepository) LicensesByQuestion(ctx context.Context, questionIDs []string) (map[string][]models.QuestionLicense, error) {
	if len(questionIDs) == 0 {
		return map[string][]models.QuestionLicense{}, nil
	}

	var rows []models.QuestionLicense
	err := r.db.WithContext(ctx).
		Where("question_id IN ?", questionIDs).
		Order("question_id ASC, created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat izin soal.")
	}

	out := make(map[string][]models.QuestionLicense, len(rows))
	for _, row := range rows {
		out[row.QuestionID] = append(out[row.QuestionID], row)
	}
	return out, nil
}

// UsableQuestionIDs menyaring id soal yang punya izin masih berlaku.
//
// Ini penegakan izin pada jalur kritis: sesi tryout hanya boleh memuat soal yang
// lolos fungsi ini. Soal tanpa baris lisensi TIDAK ikut (fail-closed), sehingga
// kelalaian melengkapi izin berhenti sebagai soal yang tidak terpakai, bukan
// sebagai pelanggaran perjanjian.
func (r *LicenseRepository) UsableQuestionIDs(ctx context.Context, questionIDs []string, now time.Time) (map[string]struct{}, error) {
	if len(questionIDs) == 0 {
		return map[string]struct{}{}, nil
	}

	var ids []string
	err := r.db.WithContext(ctx).Model(&models.QuestionLicense{}).
		Where("question_id IN ?", questionIDs).
		Where("status <> ?", models.LicenseRevoked).
		Where("valid_until IS NULL OR valid_until >= ?", now).
		Distinct().
		Pluck("question_id", &ids).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memeriksa izin soal.")
	}

	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

// UnlicensedQuestion adalah soal yang belum punya izin aktif.
//
// `HasAnyLicense` membedakan dua sebab: soal yang belum pernah diberi izin
// (false) dan soal yang izinnya sudah dicabut atau kedaluwarsa (true). Tanpa
// pembedaan ini, tim akademik bisa mengira izinnya belum dibuat padahal hanya
// perlu diperpanjang.
type UnlicensedQuestion struct {
	ID            string     `json:"id"`
	BankKey       string     `json:"bankKey"`
	SectorKey     string     `json:"sectorKey"`
	TestType      string     `json:"testType"`
	Text          string     `json:"text"`
	Status        string     `json:"status"`
	ValidUntil    *time.Time `json:"validUntil,omitempty"`
	HasAnyLicense bool       `json:"hasAnyLicense"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ListUnlicensedQuestions mengambil soal tanpa izin aktif.
//
// Hasilnya dipakai halaman kepatuhan: tim akademik bisa melengkapi izin sebelum
// soal benar-benar dibutuhkan, bukan setelah peserta gagal membuka tryout.
func (r *LicenseRepository) ListUnlicensedQuestions(ctx context.Context, limit int) ([]UnlicensedQuestion, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	now := time.Now().UTC()

	var rows []UnlicensedQuestion
	err := r.db.WithContext(ctx).Model(&models.Question{}).
		Select(`questions.id, questions.bank_key, questions.sector_key, questions.test_type,
		        questions.text, questions.updated_at,
		        COALESCE(question_licenses.status, '') AS status,
		        question_licenses.valid_until,
		        EXISTS (
		            SELECT 1 FROM question_licenses lain
		             WHERE lain.question_id = questions.id
		        ) AS has_any_license`).
		Joins(`LEFT JOIN question_licenses
		         ON question_licenses.question_id = questions.id
		        AND question_licenses.status <> ?
		        AND (question_licenses.valid_until IS NULL OR question_licenses.valid_until >= ?)`,
			models.LicenseRevoked, now).
		Where("question_licenses.id IS NULL").
		Order("questions.updated_at DESC, questions.id ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat soal tanpa izin.")
	}
	return rows, nil
}

// CountUnlicensedQuestions menghitung seluruh soal tanpa izin aktif.
func (r *LicenseRepository) CountUnlicensedQuestions(ctx context.Context) (int64, error) {
	now := time.Now().UTC()

	var total int64
	err := r.db.WithContext(ctx).Model(&models.Question{}).
		Joins(`LEFT JOIN question_licenses
		         ON question_licenses.question_id = questions.id
		        AND question_licenses.status <> ?
		        AND (question_licenses.valid_until IS NULL OR question_licenses.valid_until >= ?)`,
			models.LicenseRevoked, now).
		Where("question_licenses.id IS NULL").
		Count(&total).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung soal tanpa izin.")
	}
	return total, nil
}

// ListExpiringLicenses mengambil izin yang akan berakhir dalam rentang waktu.
func (r *LicenseRepository) ListExpiringLicenses(ctx context.Context, now, until time.Time, limit int) ([]models.QuestionLicense, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []models.QuestionLicense
	err := r.db.WithContext(ctx).
		Where("status = ?", models.LicenseActive).
		Where("valid_until IS NOT NULL AND valid_until >= ? AND valid_until <= ?", now, until).
		Order("valid_until ASC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat izin yang akan berakhir.")
	}
	return out, nil
}

// CountLicensesByScope menghitung izin per cakupan (internal/vendor/…).
func (r *LicenseRepository) CountLicensesByScope(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Scope string
		Total int64
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&models.QuestionLicense{}).
		Select("scope, count(*) AS total").
		Group("scope").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung izin per cakupan.")
	}

	out := make(map[string]int64, len(rows))
	for _, item := range rows {
		out[item.Scope] = item.Total
	}
	return out, nil
}

/* --------------------------- instrumen psikologi -------------------------- */

// InstrumentFilter adalah filter daftar instrumen psikologi.
type InstrumentFilter struct {
	Kind       string
	Search     string
	ActiveOnly bool
}

// ListInstruments mengambil daftar instrumen psikologi.
func (r *LicenseRepository) ListInstruments(ctx context.Context, filter InstrumentFilter) ([]models.PsychologicalInstrument, error) {
	query := r.db.WithContext(ctx).Model(&models.PsychologicalInstrument{})
	if filter.Kind != "" {
		query = query.Where("kind = ?", filter.Kind)
	}
	if filter.ActiveOnly {
		query = query.Where("active = ?", true)
	}
	if filter.Search != "" {
		query = query.Where("code ILIKE ? OR name ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	var out []models.PsychologicalInstrument
	if err := query.Order("kind ASC, code ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat instrumen psikologi.")
	}
	return out, nil
}

// FindInstrument mengambil satu instrumen psikologi.
func (r *LicenseRepository) FindInstrument(ctx context.Context, id string) (*models.PsychologicalInstrument, error) {
	var instrument models.PsychologicalInstrument
	err := r.db.WithContext(ctx).First(&instrument, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Instrumen psikologi tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat instrumen psikologi.")
	}
	return &instrument, nil
}

// FindInstrumentByCode mengambil instrumen berdasarkan kode uniknya.
func (r *LicenseRepository) FindInstrumentByCode(ctx context.Context, code string) (*models.PsychologicalInstrument, error) {
	var instrument models.PsychologicalInstrument
	err := r.db.WithContext(ctx).First(&instrument, "code = ?", strings.TrimSpace(code)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Instrumen psikologi tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat instrumen psikologi.")
	}
	return &instrument, nil
}

// CreateInstrument menyimpan instrumen psikologi baru.
func (r *LicenseRepository) CreateInstrument(ctx context.Context, instrument *models.PsychologicalInstrument) error {
	if instrument.ID == "" {
		instrument.ID = models.NewID()
	}
	if err := r.db.WithContext(ctx).Create(instrument).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan instrumen psikologi.")
	}
	return nil
}

// UpdateInstrument menyimpan perubahan instrumen psikologi.
func (r *LicenseRepository) UpdateInstrument(ctx context.Context, instrument *models.PsychologicalInstrument) error {
	if err := r.db.WithContext(ctx).Save(instrument).Error; err != nil {
		return apperror.Internal(err, "Gagal memperbarui instrumen psikologi.")
	}
	return nil
}

// DeleteInstrument menghapus instrumen psikologi.
func (r *LicenseRepository) DeleteInstrument(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.PsychologicalInstrument{})
	if result.Error != nil {
		return apperror.Internal(result.Error, "Gagal menghapus instrumen psikologi.")
	}
	if result.RowsAffected == 0 {
		return apperror.NotFound("Instrumen psikologi tidak ditemukan.")
	}
	return nil
}
