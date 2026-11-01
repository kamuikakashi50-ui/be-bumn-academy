package repository

// Repository KPI kelulusan (Fase 2 langkah 1): referensi tahapan seleksi, laporan
// hasil seleksi peserta, jejak audit verifikasi, dan target KPI instansi.
//
// Aturan bisnisnya (validasi tahap/tahun, dedupe kombinasi, alur verifikasi)
// hidup di service & model murni; berkas ini hanya menyimpan bentuk query-nya.
// Query agregat KPI dipisah ke `kpi.go` agar berkas ini tetap terbaca.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutcomeRepository mengakses data KPI kelulusan BUMN.
type OutcomeRepository struct {
	db *gorm.DB
}

// NewOutcomeRepository membentuk repository.
func NewOutcomeRepository(db *gorm.DB) *OutcomeRepository {
	return &OutcomeRepository{db: db}
}

// DB mengembalikan handle GORM.
func (r *OutcomeRepository) DB() *gorm.DB { return r.db }

/* --------------------------- referensi tahapan ----------------------------- */

// ListStages mengambil daftar tahapan seleksi berurutan.
func (r *OutcomeRepository) ListStages(ctx context.Context, activeOnly bool) ([]models.SelectionStage, error) {
	query := r.db.WithContext(ctx).Model(&models.SelectionStage{})
	if activeOnly {
		query = query.Where("active = ?", true)
	}

	var out []models.SelectionStage
	if err := query.Order("sort_order ASC, key ASC").Find(&out).Error; err != nil {
		return nil, apperror.Internal(err, "Gagal memuat tahapan seleksi.")
	}
	return out, nil
}

// FindStage mengambil satu tahapan seleksi berdasarkan key.
func (r *OutcomeRepository) FindStage(ctx context.Context, key string) (*models.SelectionStage, error) {
	var stage models.SelectionStage
	err := r.db.WithContext(ctx).First(&stage, "key = ?", key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.Validation("Tahapan seleksi tidak dikenal.", nil)
		}
		return nil, apperror.Internal(err, "Gagal memuat tahapan seleksi.")
	}
	return &stage, nil
}

// UpsertStage menyimpan tahapan seleksi (idempoten, dipakai seed & panel admin).
func (r *OutcomeRepository) UpsertStage(ctx context.Context, stage *models.SelectionStage) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"label", "phase", "is_final", "sort_order", "active"}),
	}).Create(stage).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan tahapan seleksi.")
	}
	return nil
}

// UpdateStage menyimpan perubahan label/urutan/status aktif sebuah tahapan.
func (r *OutcomeRepository) UpdateStage(ctx context.Context, stage *models.SelectionStage) error {
	err := r.db.WithContext(ctx).Model(&models.SelectionStage{}).
		Where("key = ?", stage.Key).
		Updates(map[string]any{
			"label":      stage.Label,
			"phase":      stage.Phase,
			"is_final":   stage.IsFinal,
			"sort_order": stage.SortOrder,
			"active":     stage.Active,
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal mengubah tahapan seleksi.")
	}
	return nil
}

/* ---------------------------- hasil seleksi peserta ------------------------ */

// OutcomeFilter adalah filter daftar laporan hasil seleksi.
//
// `Source` menerima nilai khusus `verified` (admin_verified + document_verified)
// karena dasbor & daftar tinjauan hampir selalu memisahkan dua tingkat kualitas
// data, bukan menyaring satu nilai source saja (§2.3).
type OutcomeFilter struct {
	UserID    string
	Year      int
	StageKey  string
	SectorKey string
	CohortID  string
	Program   string
	Source    string
	Search    string
	// OnlyPending menyaring laporan yang belum diverifikasi (daftar tinjauan §4.2).
	OnlyPending bool
	Limit       int
	Offset      int
}

// OutcomeListRow adalah satu baris laporan hasil seleksi beserta relasinya.
type OutcomeListRow struct {
	models.SelectionOutcome
	UserName    string `gorm:"column:user_name"`
	UserEmail   string `gorm:"column:user_email"`
	UserProgram string `gorm:"column:user_program"`
	CohortName  string `gorm:"column:cohort_name"`
	StageLabel  string `gorm:"column:stage_label"`
	StageFinal  bool   `gorm:"column:stage_final"`
}

// outcomeListColumns adalah kolom daftar laporan beserta relasinya.
const outcomeListColumns = `selection_outcomes.*,
	users.name    AS user_name,
	users.email   AS user_email,
	users.program AS user_program,
	COALESCE(cohorts.name, '') AS cohort_name,
	selection_stages.label    AS stage_label,
	selection_stages.is_final AS stage_final`

// outcomeListJoins menautkan laporan ke pemilik, kelas, dan tahapan.
//
// `users` di-JOIN (bukan LEFT JOIN) karena laporan tanpa pemilik tidak punya arti;
// `cohorts` LEFT JOIN karena peserta boleh bukan anggota kelas khusus mana pun.
const outcomeListJoins = `JOIN users ON users.id = selection_outcomes.user_id
	LEFT JOIN cohorts ON cohorts.id = selection_outcomes.cohort_id
	JOIN selection_stages ON selection_stages.key = selection_outcomes.stage_key`

// outcomeListQuery menyusun query daftar laporan dengan filter yang dipasang.
func (r *OutcomeRepository) outcomeListQuery(ctx context.Context, filter OutcomeFilter) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table("selection_outcomes").
		Select(outcomeListColumns).
		Joins(outcomeListJoins)
	return applyOutcomeFilter(query, filter)
}

// applyOutcomeFilter memasang filter daftar laporan pada sebuah query.
func applyOutcomeFilter(query *gorm.DB, filter OutcomeFilter) *gorm.DB {
	if filter.UserID != "" {
		query = query.Where("selection_outcomes.user_id = ?", filter.UserID)
	}
	if filter.Year > 0 {
		query = query.Where("selection_outcomes.selection_year = ?", filter.Year)
	}
	if filter.StageKey != "" {
		query = query.Where("selection_outcomes.stage_key = ?", filter.StageKey)
	}
	if filter.SectorKey != "" {
		query = query.Where("selection_outcomes.company_sector = ?", filter.SectorKey)
	}
	if filter.CohortID != "" {
		query = query.Where("selection_outcomes.cohort_id = ?", filter.CohortID)
	}
	if filter.Program != "" {
		query = query.Where("users.program = ?", filter.Program)
	}
	switch filter.Source {
	case "verified":
		query = query.Where("selection_outcomes.source <> ?", models.OutcomeSelfReported)
	case string(models.OutcomeSelfReported):
		query = query.Where("selection_outcomes.source = ?", models.OutcomeSelfReported)
	}
	if filter.OnlyPending {
		query = query.Where("selection_outcomes.source = ?", models.OutcomeSelfReported)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"(LOWER(users.name) LIKE ? OR LOWER(users.email) LIKE ? OR LOWER(selection_outcomes.company_name) LIKE ?)",
			like, like, like,
		)
	}
	return query
}

// ListOutcomes mengambil daftar laporan hasil seleksi beserta relasinya.
func (r *OutcomeRepository) ListOutcomes(ctx context.Context, filter OutcomeFilter) ([]OutcomeListRow, error) {
	query := r.outcomeListQuery(ctx, filter)

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var rows []OutcomeListRow
	err := query.
		Order("selection_outcomes.created_at DESC, selection_outcomes.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat laporan hasil seleksi.")
	}
	return rows, nil
}

// FindOutcome mengambil satu laporan (beserta relasinya) berdasarkan id.
func (r *OutcomeRepository) FindOutcome(ctx context.Context, id string) (*OutcomeListRow, error) {
	var row OutcomeListRow
	err := r.outcomeListQuery(ctx, OutcomeFilter{}).
		Where("selection_outcomes.id = ?", id).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat laporan hasil seleksi.")
	}
	if row.ID == "" {
		return nil, apperror.NotFound("Laporan hasil seleksi tidak ditemukan.")
	}
	return &row, nil
}

// FindOutcomeByKey mencari laporan pada kombinasi (peserta, tahun, tahap, instansi).
//
// Kombinasi ini unik (indeks `selection_outcomes_unique_idx`), jadi laporan kedua
// pada kombinasi yang sama harus memperbarui baris yang ada, bukan menambah (§4.1).
func (r *OutcomeRepository) FindOutcomeByKey(
	ctx context.Context,
	userID string,
	year int,
	stageKey string,
	companyName string,
) (*models.SelectionOutcome, error) {
	var outcome models.SelectionOutcome
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND selection_year = ? AND stage_key = ? AND company_name = ?",
			userID, year, stageKey, companyName).
		First(&outcome).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memeriksa laporan hasil seleksi.")
	}
	return &outcome, nil
}

// CreateOutcome menyimpan laporan hasil seleksi baru.
func (r *OutcomeRepository) CreateOutcome(ctx context.Context, outcome *models.SelectionOutcome) error {
	if err := r.db.WithContext(ctx).Create(outcome).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan laporan hasil seleksi.")
	}
	return nil
}

// SaveOutcome menyimpan perubahan pada laporan yang sudah ada.
//
// Kolom yang ditulis dibatasi pada isi laporan & status verifikasinya; `user_id`
// tidak pernah berubah sehingga laporan tidak bisa dipindah pemiliknya.
func (r *OutcomeRepository) SaveOutcome(ctx context.Context, outcome *models.SelectionOutcome) error {
	err := r.db.WithContext(ctx).Model(&models.SelectionOutcome{}).
		Where("id = ?", outcome.ID).
		Updates(map[string]any{
			"cohort_id":         outcome.CohortID,
			"stage_key":         outcome.StageKey,
			"company_name":      outcome.CompanyName,
			"company_sector":    outcome.CompanySector,
			"position":          outcome.Position,
			"position_family":   outcome.PositionFamily,
			"selection_year":    outcome.SelectionYear,
			"status":            outcome.Status,
			"announced_at":      outcome.AnnouncedAt,
			"source":            outcome.Source,
			"evidence_url":      outcome.EvidenceURL,
			"note":              outcome.Note,
			"aggregate_consent": outcome.AggregateConsent,
			"verified_by":       outcome.VerifiedBy,
			"verified_at":       outcome.VerifiedAt,
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan perubahan laporan hasil seleksi.")
	}
	return nil
}

// SetOutcomeEvidence menyimpan tautan bukti pengumuman milik sebuah laporan.
func (r *OutcomeRepository) SetOutcomeEvidence(ctx context.Context, id, url string) error {
	err := r.db.WithContext(ctx).Model(&models.SelectionOutcome{}).
		Where("id = ?", id).
		Update("evidence_url", url).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan bukti laporan.")
	}
	return nil
}

// SetOutcomeVerification menyimpan hasil verifikasi admin atas sebuah laporan.
//
// `actorID`/`verifiedAt` berupa pointer agar penolakan bukti bisa mengosongkan
// keduanya (NULL) — kolom UUID tidak menerima string kosong.
func (r *OutcomeRepository) SetOutcomeVerification(
	ctx context.Context,
	id string,
	source models.OutcomeSource,
	note string,
	actorID *string,
	verifiedAt *time.Time,
) error {
	err := r.db.WithContext(ctx).Model(&models.SelectionOutcome{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"source":      source,
			"note":        note,
			"verified_by": actorID,
			"verified_at": verifiedAt,
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal menyimpan verifikasi laporan.")
	}
	return nil
}

// DeleteOutcome menghapus laporan (dipakai peserta yang salah entri).
func (r *OutcomeRepository) DeleteOutcome(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.SelectionOutcome{}, "id = ?", id).Error; err != nil {
		return apperror.Internal(err, "Gagal menghapus laporan hasil seleksi.")
	}
	return nil
}

// ReportedUserIDsInYear mengambil peserta yang sudah melapor pada tahun tertentu.
//
// Dipakai menentukan penerima pengingat tracer study (§4.5): pengingat hanya
// dikirim ke peserta yang **belum** ada di daftar ini.
func (r *OutcomeRepository) ReportedUserIDsInYear(ctx context.Context, year int) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&models.SelectionOutcome{}).
		Distinct().
		Where("selection_year = ?", year).
		Pluck("user_id", &ids).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat peserta yang sudah melapor.")
	}
	return ids, nil
}

// CountOutcomes menghitung jumlah laporan yang cocok dengan filter (paginasi admin).
func (r *OutcomeRepository) CountOutcomes(ctx context.Context, filter OutcomeFilter) (int, error) {
	query := r.outcomeListQuery(ctx, filter).Select("COUNT(*) AS total")

	var row struct {
		Total int `gorm:"column:total"`
	}
	if err := query.Scan(&row).Error; err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung laporan hasil seleksi.")
	}
	return row.Total, nil
}

/* ------------------------------ jejak audit KPI ---------------------------- */

// OutcomeAuditRow adalah satu baris jejak audit beserta nama pelakunya.
type OutcomeAuditRow struct {
	models.SelectionOutcomeAudit
	ActorName string `gorm:"column:actor_name"`
}

// AppendOutcomeAudit mencatat satu aksi verifikasi/perubahan laporan.
func (r *OutcomeRepository) AppendOutcomeAudit(ctx context.Context, audit *models.SelectionOutcomeAudit) error {
	if err := r.db.WithContext(ctx).Create(audit).Error; err != nil {
		return apperror.Internal(err, "Gagal mencatat jejak audit laporan.")
	}
	return nil
}

// ListOutcomeAudits mengambil jejak audit sebuah laporan (terbaru lebih dulu).
func (r *OutcomeRepository) ListOutcomeAudits(ctx context.Context, outcomeID string, limit int) ([]OutcomeAuditRow, error) {
	query := r.db.WithContext(ctx).
		Table("selection_outcome_audits").
		Select("selection_outcome_audits.*, COALESCE(users.name, 'sistem') AS actor_name").
		Joins("LEFT JOIN users ON users.id = selection_outcome_audits.actor_id").
		Where("selection_outcome_audits.outcome_id = ?", outcomeID)
	if limit > 0 {
		query = query.Limit(limit)
	}

	var rows []OutcomeAuditRow
	err := query.Order("selection_outcome_audits.created_at DESC").Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat jejak audit laporan.")
	}
	return rows, nil
}

/* -------------------------------- target KPI ------------------------------- */

// KPITargetFilter adalah filter daftar target KPI.
type KPITargetFilter struct {
	Year     int
	OrgID    string
	CohortID string
}

// KPITargetRow adalah satu target KPI beserta nama kelompoknya.
type KPITargetRow struct {
	models.KPITarget
	OrgName    string `gorm:"column:org_name"`
	CohortName string `gorm:"column:cohort_name"`
}

// ListTargets mengambil daftar target KPI beserta nama instansi/kelasnya.
func (r *OutcomeRepository) ListTargets(ctx context.Context, filter KPITargetFilter) ([]KPITargetRow, error) {
	query := r.db.WithContext(ctx).
		Table("kpi_targets").
		Select("kpi_targets.*, COALESCE(organizations.name, '') AS org_name, COALESCE(cohorts.name, '') AS cohort_name").
		Joins("LEFT JOIN organizations ON organizations.id = kpi_targets.org_id").
		Joins("LEFT JOIN cohorts ON cohorts.id = kpi_targets.cohort_id")
	if filter.Year > 0 {
		query = query.Where("kpi_targets.period_year = ?", filter.Year)
	}
	if filter.OrgID != "" {
		query = query.Where("kpi_targets.org_id = ?", filter.OrgID)
	}
	if filter.CohortID != "" {
		query = query.Where("kpi_targets.cohort_id = ?", filter.CohortID)
	}

	var rows []KPITargetRow
	err := query.Order("kpi_targets.period_year DESC, kpi_targets.metric ASC").Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat target KPI.")
	}
	return rows, nil
}

// FindTarget mengambil satu target KPI berdasarkan id.
func (r *OutcomeRepository) FindTarget(ctx context.Context, id string) (*models.KPITarget, error) {
	var target models.KPITarget
	err := r.db.WithContext(ctx).First(&target, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("Target KPI tidak ditemukan.")
		}
		return nil, apperror.Internal(err, "Gagal memuat target KPI.")
	}
	return &target, nil
}

// FindTargetByKey mencari target pada kombinasi kelompok + periode + metric.
//
// `orgID`/`cohortID`/`stageKey` kosong berarti NULL (target lintas program atau
// metric non-tahap), sehingga perbandingannya memakai `IS NULL`.
func (r *OutcomeRepository) FindTargetByKey(
	ctx context.Context,
	orgID string,
	cohortID string,
	year int,
	metric models.KPIMetric,
	stageKey string,
) (*models.KPITarget, error) {
	query := r.db.WithContext(ctx).Model(&models.KPITarget{}).
		Where("period_year = ?", year).
		Where("metric = ?", metric)
	query = whereNullableUUID(query, "org_id", orgID)
	query = whereNullableUUID(query, "cohort_id", cohortID)
	query = whereNullableText(query, "stage_key", stageKey)

	var target models.KPITarget
	err := query.First(&target).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, apperror.Internal(err, "Gagal memeriksa target KPI.")
	}
	return &target, nil
}

// CreateTarget menyimpan target KPI baru.
func (r *OutcomeRepository) CreateTarget(ctx context.Context, target *models.KPITarget) error {
	if err := r.db.WithContext(ctx).Create(target).Error; err != nil {
		return apperror.Internal(err, "Gagal menyimpan target KPI.")
	}
	return nil
}

// UpdateTarget menyimpan perubahan nilai/catatan sebuah target KPI.
func (r *OutcomeRepository) UpdateTarget(ctx context.Context, target *models.KPITarget) error {
	err := r.db.WithContext(ctx).Model(&models.KPITarget{}).
		Where("id = ?", target.ID).
		Updates(map[string]any{
			"target_value": target.TargetValue,
			"note":         target.Note,
			"stage_key":    target.StageKey,
		}).Error
	if err != nil {
		return apperror.Internal(err, "Gagal mengubah target KPI.")
	}
	return nil
}

// DeleteTarget menghapus target KPI.
func (r *OutcomeRepository) DeleteTarget(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.KPITarget{}, "id = ?", id).Error; err != nil {
		return apperror.Internal(err, "Gagal menghapus target KPI.")
	}
	return nil
}

// whereNullableUUID memasang kondisi `IS NULL` atau `= nilai` pada kolom UUID.
func whereNullableUUID(query *gorm.DB, column, value string) *gorm.DB {
	if value == "" {
		return query.Where(column + " IS NULL")
	}
	return query.Where(column+" = ?", value)
}

// whereNullableText memasang kondisi `IS NULL` atau `= nilai` pada kolom teks.
func whereNullableText(query *gorm.DB, column, value string) *gorm.DB {
	if value == "" {
		return query.Where(column + " IS NULL")
	}
	return query.Where(column+" = ?", value)
}
