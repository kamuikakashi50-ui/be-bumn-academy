package repository

// Laporan kelas khusus (Fase 1 langkah 6): angka & daftar dihitung dari anggota
// kelas itu saja, sehingga laporan satu kelas tidak pernah memuat peserta kelas
// lain (isolasi data §4.7). Bagian ini dipisah dari `cohort.go` agar berkasnya
// tetap terbaca; aturan bisnisnya ada di `internal/service/cohort_report.go`.

import (
	"context"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

/* ------------------------------ laporan kelas ----------------------------- */

// cohortReportMembersQuery mengambil satu baris per anggota kelas.
//
// Semua angka berasal dari anggota kelas itu saja (`cm.cohort_id = ?`), sehingga
// laporan kelas A tidak mungkin memuat peserta kelas B — inilah isolasi data yang
// diuji otomatis (bagian 7 spesifikasi Fase 1).
//
// Peserta yang belum pernah mengerjakan tryout tetap muncul (LEFT JOIN) supaya
// laporan bisa menandainya "belum mulai", bukan menghilangkannya.
const cohortReportMembersQuery = `
WITH members AS (
    SELECT cm.user_id, cm.status, cm.joined_at, u.name, u.email, u.program
      FROM cohort_members cm
      JOIN users u ON u.id = cm.user_id
     WHERE cm.cohort_id = ? AND cm.role = 'peserta'
),
attempt_stats AS (
    SELECT a.user_id,
           COUNT(*) AS attempt_count,
           COALESCE(AVG(a.score), 0) AS average_score,
           COALESCE(MAX(a.score), 0) AS best_score,
           MAX(a.taken_at) AS last_activity
      FROM attempts a
      JOIN members m ON m.user_id = a.user_id
     GROUP BY a.user_id
),
latest AS (
    SELECT DISTINCT ON (a.user_id) a.user_id, a.score AS last_score
      FROM attempts a
      JOIN members m ON m.user_id = a.user_id
     ORDER BY a.user_id, a.taken_at DESC, a.score DESC
),
progress AS (
    SELECT lp.user_id, COUNT(DISTINCT lp.lesson_id) AS completed
      FROM lesson_progress lp
      JOIN lessons l ON l.id = lp.lesson_id AND l.active
      JOIN cohort_curriculum cc ON cc.module_id = l.module_id AND cc.cohort_id = ?
     WHERE lp.completed
     GROUP BY lp.user_id
),
certificates AS (
    SELECT c.user_id, COUNT(*) AS total
      FROM certificates c
      JOIN members m ON m.user_id = c.user_id
     WHERE c.revoked_at IS NULL
     GROUP BY c.user_id
)
SELECT m.user_id, m.name, m.email, m.program, m.status, m.joined_at,
       COALESCE(a.attempt_count, 0) AS attempt_count,
       COALESCE(a.average_score, 0) AS average_score,
       COALESCE(a.best_score, 0) AS best_score,
       l.last_score AS last_score,
       a.last_activity AS last_activity,
       COALESCE(p.completed, 0) AS lessons_completed,
       COALESCE(c.total, 0) AS certificates
  FROM members m
  LEFT JOIN attempt_stats a ON a.user_id = m.user_id
  LEFT JOIN latest l ON l.user_id = m.user_id
  LEFT JOIN progress p ON p.user_id = m.user_id
  LEFT JOIN certificates c ON c.user_id = m.user_id
 ORDER BY m.name ASC`

// cohortReportAttemptQuery mengagregasi percobaan tryout anggota kelas.
const cohortReportAttemptQuery = `
SELECT COUNT(*) AS total_attempts,
       COALESCE(AVG(a.score), 0) AS average_score,
       COALESCE(MAX(a.score), 0) AS top_score,
       COUNT(*) FILTER (WHERE a.score >= ?) AS passed_attempts,
       COALESCE(SUM(a.duration_seconds), 0) AS total_seconds,
       COUNT(DISTINCT a.tryout_id) AS distinct_tryouts,
       COUNT(*) FILTER (WHERE a.cohort_id = ?) AS in_class_attempts
  FROM attempts a
  JOIN cohort_members cm ON cm.user_id = a.user_id
 WHERE cm.cohort_id = ? AND cm.role = 'peserta'`

// cohortReportLearningQuery menghitung kelengkapan materi kurikulum kelas.
const cohortReportLearningQuery = `
SELECT (SELECT COUNT(DISTINCT l.id)
          FROM cohort_curriculum cc
          JOIN lessons l ON l.module_id = cc.module_id AND l.active
         WHERE cc.cohort_id = ?) AS total_lessons,
       (SELECT COUNT(DISTINCT lp.lesson_id)
          FROM lesson_progress lp
          JOIN lessons l ON l.id = lp.lesson_id AND l.active
          JOIN cohort_curriculum cc ON cc.module_id = l.module_id AND cc.cohort_id = ?
         WHERE lp.completed
           AND lp.user_id IN (SELECT user_id FROM cohort_members WHERE cohort_id = ? AND role = 'peserta')) AS completed_lessons`

// cohortReportModuleQuery mengambil progres materi per modul kurikulum kelas.
const cohortReportModuleQuery = `
SELECT cc.module_id, m.title, m.product_slug, cc.required, cc.sort_order,
       COUNT(DISTINCT l.id) AS lesson_count,
       COUNT(DISTINCT lp.lesson_id) FILTER (WHERE lp.completed) AS completed_lessons
  FROM cohort_curriculum cc
  JOIN course_modules m ON m.id = cc.module_id
  LEFT JOIN lessons l ON l.module_id = cc.module_id AND l.active
  LEFT JOIN lesson_progress lp
         ON lp.lesson_id = l.id
        AND lp.user_id IN (SELECT user_id FROM cohort_members WHERE cohort_id = ? AND role = 'peserta')
 WHERE cc.cohort_id = ?
 GROUP BY cc.module_id, m.title, m.product_slug, cc.required, cc.sort_order
 ORDER BY cc.sort_order ASC, cc.module_id ASC`

// cohortReportSkillQuery mengagregasi skor per subtes (jenis tes) anggota kelas.
const cohortReportSkillQuery = `
SELECT ac.name AS name,
       COALESCE(AVG(ac.score), 0) AS average_score,
       COUNT(*) AS attempts
  FROM attempt_categories ac
  JOIN attempts a ON a.id = ac.attempt_id
  JOIN cohort_members cm ON cm.user_id = a.user_id
 WHERE cm.cohort_id = ? AND cm.role = 'peserta'
 GROUP BY ac.name
 ORDER BY average_score DESC, ac.name ASC
 LIMIT ?`

// cohortReportActiveQuery menghitung anggota yang aktif dalam rentang hari terakhir
// (mengerjakan tryout atau menyentuh materi).
const cohortReportActiveQuery = `
SELECT COUNT(DISTINCT user_id) AS active_members
  FROM (
        SELECT a.user_id
          FROM attempts a
          JOIN cohort_members cm ON cm.user_id = a.user_id
         WHERE cm.cohort_id = ? AND cm.role = 'peserta' AND a.taken_at >= ?
        UNION
        SELECT lp.user_id
          FROM lesson_progress lp
          JOIN cohort_members cm ON cm.user_id = lp.user_id
         WHERE cm.cohort_id = ? AND cm.role = 'peserta' AND lp.last_viewed_at >= ?
       ) aktif`

/* -------------------- hasil query laporan kelas (baris) ------------------- */

// CohortReportMemberRow adalah satu baris laporan kelas (per anggota).
type CohortReportMemberRow struct {
	UserID           string     `gorm:"column:user_id"`
	Name             string     `gorm:"column:name"`
	Email            string     `gorm:"column:email"`
	Program          string     `gorm:"column:program"`
	Status           string     `gorm:"column:status"`
	JoinedAt         time.Time  `gorm:"column:joined_at"`
	AttemptCount     int64      `gorm:"column:attempt_count"`
	AverageScore     float64    `gorm:"column:average_score"`
	BestScore        int        `gorm:"column:best_score"`
	LastScore        *int       `gorm:"column:last_score"`
	LastActivity     *time.Time `gorm:"column:last_activity"`
	LessonsCompleted int        `gorm:"column:lessons_completed"`
	Certificates     int        `gorm:"column:certificates"`
}

// CohortReportAttemptRow adalah agregat percobaan tryout satu kelas.
type CohortReportAttemptRow struct {
	TotalAttempts   int64   `gorm:"column:total_attempts"`
	AverageScore    float64 `gorm:"column:average_score"`
	TopScore        int     `gorm:"column:top_score"`
	PassedAttempts  int64   `gorm:"column:passed_attempts"`
	TotalSeconds    int64   `gorm:"column:total_seconds"`
	DistinctTryouts int64   `gorm:"column:distinct_tryouts"`
	InClassAttempts int64   `gorm:"column:in_class_attempts"`
}

// CohortReportLearningRow adalah agregat kelengkapan materi kelas.
type CohortReportLearningRow struct {
	TotalLessons     int64 `gorm:"column:total_lessons"`
	CompletedLessons int64 `gorm:"column:completed_lessons"`
}

// CohortReportModuleRow adalah progres materi satu modul kurikulum kelas.
type CohortReportModuleRow struct {
	ModuleID         string `gorm:"column:module_id"`
	Title            string `gorm:"column:title"`
	ProductSlug      string `gorm:"column:product_slug"`
	Required         bool   `gorm:"column:required"`
	SortOrder        int    `gorm:"column:sort_order"`
	LessonCount      int    `gorm:"column:lesson_count"`
	CompletedLessons int    `gorm:"column:completed_lessons"`
}

// CohortReportSkillRow adalah rata-rata skor satu subtes pada kelas.
type CohortReportSkillRow struct {
	Name         string  `gorm:"column:name"`
	AverageScore float64 `gorm:"column:average_score"`
	Attempts     int64   `gorm:"column:attempts"`
}

// CohortReportMembers mengambil anggota kelas beserta capaiannya.
func (r *CohortRepository) CohortReportMembers(ctx context.Context, cohortID string) ([]CohortReportMemberRow, error) {
	var rows []CohortReportMemberRow
	err := r.db.WithContext(ctx).Raw(cohortReportMembersQuery, cohortID, cohortID).Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat anggota laporan kelas.")
	}
	return rows, nil
}

// CohortReportAttemptStats mengambil agregat percobaan anggota kelas.
func (r *CohortRepository) CohortReportAttemptStats(ctx context.Context, cohortID string, passingThreshold int) (*CohortReportAttemptRow, error) {
	var row CohortReportAttemptRow
	err := r.db.WithContext(ctx).
		Raw(cohortReportAttemptQuery, passingThreshold, cohortID, cohortID).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung capaian latihan kelas.")
	}
	return &row, nil
}

// CohortReportLearningStats mengambil kelengkapan materi kurikulum kelas.
func (r *CohortRepository) CohortReportLearningStats(ctx context.Context, cohortID string) (*CohortReportLearningRow, error) {
	var row CohortReportLearningRow
	err := r.db.WithContext(ctx).
		Raw(cohortReportLearningQuery, cohortID, cohortID, cohortID).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung kelengkapan materi kelas.")
	}
	return &row, nil
}

// CohortReportModuleProgress mengambil progres materi per modul kelas.
func (r *CohortRepository) CohortReportModuleProgress(ctx context.Context, cohortID string) ([]CohortReportModuleRow, error) {
	var rows []CohortReportModuleRow
	err := r.db.WithContext(ctx).Raw(cohortReportModuleQuery, cohortID, cohortID).Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memuat progres modul kelas.")
	}
	return rows, nil
}

// CohortReportSkillProfile mengambil rata-rata skor per subtes kelas.
func (r *CohortRepository) CohortReportSkillProfile(ctx context.Context, cohortID string, limit int) ([]CohortReportSkillRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	var rows []CohortReportSkillRow
	err := r.db.WithContext(ctx).Raw(cohortReportSkillQuery, cohortID, limit).Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung profil subtes kelas.")
	}
	return rows, nil
}

// CohortReportActiveMembers menghitung anggota yang aktif sejak `since`.
func (r *CohortRepository) CohortReportActiveMembers(ctx context.Context, cohortID string, since time.Time) (int, error) {
	var row struct {
		ActiveMembers int `gorm:"column:active_members"`
	}
	err := r.db.WithContext(ctx).
		Raw(cohortReportActiveQuery, cohortID, since, cohortID, since).
		Scan(&row).Error
	if err != nil {
		return 0, apperror.Internal(err, "Gagal menghitung anggota aktif kelas.")
	}
	return row.ActiveMembers, nil
}

// CohortReportMemberCounts menghitung anggota berperan **peserta** pada satu kelas.
//
// Mentor & admin instansi sengaja tidak dihitung: laporan kelas berbicara soal
// peserta, sehingga angka partisipasi tidak tercampur staf pengajar.
func (r *CohortRepository) CohortReportMemberCounts(ctx context.Context, cohortID string) (CohortMemberCount, error) {
	var row struct {
		Total     int `gorm:"column:total"`
		Active    int `gorm:"column:active"`
		Completed int `gorm:"column:completed"`
		Left      int `gorm:"column:left_members"`
	}
	err := r.db.WithContext(ctx).Model(&models.CohortMember{}).
		Select(`COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = ?) AS active,
		        COUNT(*) FILTER (WHERE status = ?) AS completed,
		        COUNT(*) FILTER (WHERE status = ?) AS left_members`,
			models.CohortMemberAktif, models.CohortMemberLulus, models.CohortMemberKeluar).
		Where("cohort_id = ? AND role = ?", cohortID, models.CohortRolePeserta).
		Scan(&row).Error
	if err != nil {
		return CohortMemberCount{}, apperror.Internal(err, "Gagal menghitung peserta kelas.")
	}
	return CohortMemberCount{Total: row.Total, Active: row.Active, Completed: row.Completed, Left: row.Left}, nil
}
