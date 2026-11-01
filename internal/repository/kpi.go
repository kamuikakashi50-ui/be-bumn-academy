package repository

// Query agregat KPI kelulusan BUMN (Fase 2 langkah 4).
//
// Seluruh angka dihitung di database lalu dirapikan di `internal/service/kpi.go`
// (pembulatan, pembagi nol, penyusunan kelompok) supaya dasbor, ekspor CSV, dan
// laporan PDF memakai angka yang identik (§2.5).
//
// Aturan yang dipegang di semua query di bawah:
//   - `role = 'peserta'` pada tabel `users`: staf/mentor bukan bagian populasi KPI;
//   - `selection_outcomes` difilter tahun/klaster/kelas/program lewat satu helper
//     yang sama, sehingga filter dasbor menentukan pembilang **dan** penyebut (§4.4);
//   - `aggregate_consent` diperiksa saat angka dipakai untuk publik (§4.7).

import (
	"context"
	"fmt"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

// KPIFilter adalah filter kelompok KPI (menentukan pembilang & penyebut).
type KPIFilter struct {
	Year      int
	SectorKey string
	CohortID  string
	// OrgID menyaring peserta yang menjadi anggota kelas instansi tersebut (§4.4).
	OrgID    string
	Program  string
	StageKey string
	// Source: "" (semua laporan), "verified", atau "self_reported".
	Source string
	// ConsentOnly menyaring peserta yang menyetujui pemakaian agregat; dipakai
	// endpoint publik (§4.7).
	ConsentOnly bool
}

// outcomeConditions menyusun kondisi filter pada alias `o` (selection_outcomes).
//
// Dikembalikan sebagai potongan `AND …` supaya bisa ditempelkan pada query apa pun
// tanpa menyusun ulang bentuk WHERE-nya.
func (f KPIFilter) outcomeConditions() (string, []any) {
	var (
		clauses []string
		args    []any
	)
	if f.Year > 0 {
		clauses = append(clauses, "o.selection_year = ?")
		args = append(args, f.Year)
	}
	if f.SectorKey != "" {
		clauses = append(clauses, "o.company_sector = ?")
		args = append(args, f.SectorKey)
	}
	if f.CohortID != "" {
		clauses = append(clauses, "o.cohort_id = ?")
		args = append(args, f.CohortID)
	}
	if f.OrgID != "" {
		// Instansi tidak tercatat langsung pada laporan: relasinya lewat kelas
		// khusus tempat laporan itu dibuat (kolom `cohort_id`).
		clauses = append(clauses, "o.cohort_id IN (SELECT c.id FROM cohorts c WHERE c.org_id = ?)")
		args = append(args, f.OrgID)
	}
	if f.StageKey != "" {
		clauses = append(clauses, "o.stage_key = ?")
		args = append(args, f.StageKey)
	}
	switch f.Source {
	case "verified":
		clauses = append(clauses, "o.source <> 'self_reported'")
	case "self_reported":
		clauses = append(clauses, "o.source = 'self_reported'")
	}
	if f.ConsentOnly {
		clauses = append(clauses, "o.aggregate_consent")
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

// participantConditions menyusun kondisi filter pada alias `u` (users).
func (f KPIFilter) participantConditions() (string, []any) {
	var (
		clauses []string
		args    []any
	)
	if f.Program != "" {
		clauses = append(clauses, "u.program = ?")
		args = append(args, f.Program)
	}
	if f.CohortID != "" {
		clauses = append(clauses,
			"EXISTS (SELECT 1 FROM cohort_members cm WHERE cm.user_id = u.id AND cm.cohort_id = ?)")
		args = append(args, f.CohortID)
	}
	if f.OrgID != "" {
		clauses = append(clauses,
			"EXISTS (SELECT 1 FROM cohort_members cm JOIN cohorts c ON c.id = cm.cohort_id WHERE cm.user_id = u.id AND c.org_id = ?)")
		args = append(args, f.OrgID)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

/* --------------------------------- baris ---------------------------------- */

// KPIParticipantsRow adalah populasi KPI: peserta, pelapor, dan pelapor terverifikasi.
type KPIParticipantsRow struct {
	TotalParticipants int64 `gorm:"column:total_participants"`
	Reported          int64 `gorm:"column:reported"`
	Verified          int64 `gorm:"column:verified"`
}

// KPIStagePassRow adalah capaian satu tahap seleksi.
type KPIStagePassRow struct {
	StageKey   string `gorm:"column:stage_key"`
	StageLabel string `gorm:"column:stage_label"`
	SortOrder  int    `gorm:"column:sort_order"`
	IsFinal    bool   `gorm:"column:is_final"`
	Passed     int64  `gorm:"column:passed"`
	Counted    int64  `gorm:"column:counted"`
}

// KPIBreakdownRow adalah capaian satu kelompok (klaster/kelas/program).
type KPIBreakdownRow struct {
	GroupKey string `gorm:"column:group_key"`
	Label    string `gorm:"column:group_label"`
	Passed   int64  `gorm:"column:passed"`
	Counted  int64  `gorm:"column:counted"`
}

// KPIPracticeRow membandingkan kebiasaan latihan peserta lolos vs tidak lolos.
type KPIPracticeRow struct {
	PassedAverageScore    float64 `gorm:"column:passed_average_score"`
	FailedAverageScore    float64 `gorm:"column:failed_average_score"`
	PassedAverageAttempts float64 `gorm:"column:passed_average_attempts"`
	FailedAverageAttempts float64 `gorm:"column:failed_average_attempts"`
}

// KPICurriculumRow mengukur kelengkapan kurikulum peserta yang lolos tahap akhir.
type KPICurriculumRow struct {
	PassedTotal     int64 `gorm:"column:passed_total"`
	PassedCompleted int64 `gorm:"column:passed_completed"`
}

// KPIFunnelRow adalah angka dasar funnel (tahap seleksi ditambahkan dari query tahap).
type KPIFunnelRow struct {
	Registered          int64 `gorm:"column:registered"`
	Practiced           int64 `gorm:"column:practiced"`
	CurriculumCompleted int64 `gorm:"column:curriculum_completed"`
	Reported            int64 `gorm:"column:reported"`
}

// KPITimeToPassRow adalah median hari dari percobaan pertama ke pengumuman lolos.
type KPITimeToPassRow struct {
	MedianDays float64 `gorm:"column:median_days"`
}

/* --------------------------------- query ---------------------------------- */

// kpiParticipantsQuery menghitung populasi KPI: seluruh peserta pada kelompok yang
// difilter, berapa yang sudah melapor, dan berapa yang laporannya terverifikasi.
//
// `LEFT JOIN` disengaja: peserta yang belum melapor tetap terhitung sebagai
// populasi (kelompok "belum ada laporan"), bukan dianggap gagal (§4.3).
const kpiParticipantsQuery = `
SELECT COUNT(DISTINCT u.id)                       AS total_participants,
       COUNT(DISTINCT o.user_id)                  AS reported,
       COUNT(DISTINCT o.user_id) FILTER (WHERE o.source <> 'self_reported') AS verified
  FROM users u
  LEFT JOIN selection_outcomes o ON o.user_id = u.id%s
 WHERE u.role = 'peserta'%s`

// kpiStagePassQuery menghitung capaian tiap tahap seleksi.
//
// Pembilang & penyebut memakai peserta **unik**: seorang peserta yang melaporkan
// dua instansi pada tahap yang sama tetap dihitung sekali.
const kpiStagePassQuery = `
SELECT s.key AS stage_key, s.label AS stage_label, s.sort_order, s.is_final,
       COUNT(DISTINCT o.user_id) FILTER (WHERE o.status = 'lolos')            AS passed,
       COUNT(DISTINCT o.user_id) FILTER (WHERE o.status IN ('lolos','gagal')) AS counted
  FROM selection_stages s
  JOIN selection_outcomes o ON o.stage_key = s.key
  JOIN users u ON u.id = o.user_id
 WHERE s.active AND u.role = 'peserta'%s
 GROUP BY s.key, s.label, s.sort_order, s.is_final
 ORDER BY s.sort_order ASC, s.key ASC`

// kpiBreakdownTemplate adalah kerangka capaian per kelompok; ekspresi `group_key`
// & `group_label` diisi konstanta di bawah (bukan masukan pengguna).
const kpiBreakdownTemplate = `
SELECT %s AS group_key, %s AS group_label,
       COUNT(DISTINCT o.user_id) FILTER (WHERE o.status = 'lolos')            AS passed,
       COUNT(DISTINCT o.user_id) FILTER (WHERE o.status IN ('lolos','gagal')) AS counted
  FROM selection_outcomes o
  JOIN users u ON u.id = o.user_id
  LEFT JOIN cohorts c ON c.id = o.cohort_id
 WHERE u.role = 'peserta' AND o.stage_key = ?%s
 GROUP BY 1, 2
 ORDER BY counted DESC, group_key ASC`

// kpiBreakdownGroupExprs memetakan dimensi kelompok ke ekspresi SQL-nya.
var kpiBreakdownGroupExprs = map[string][2]string{
	"sector":  {"o.company_sector", "o.company_sector"},
	"cohort":  {"COALESCE(o.cohort_id::text, '')", "COALESCE(c.name, 'Tanpa kelas')"},
	"program": {"u.program", "u.program"},
}

// kpiPracticeQuery membandingkan kebiasaan latihan peserta lolos vs tidak lolos
// pada tahap akhir. Angkanya menyatakan **korelasi**, bukan sebab-akibat (§4.3).
const kpiPracticeQuery = `
WITH final_outcomes AS (
    SELECT o.user_id,
           MAX(CASE WHEN o.status = 'lolos' THEN 1 ELSE 0 END) AS passed
      FROM selection_outcomes o
      JOIN users u ON u.id = o.user_id
      JOIN selection_stages s ON s.key = o.stage_key AND s.is_final
     WHERE u.role = 'peserta'%s
     GROUP BY o.user_id
)
SELECT COALESCE(AVG(a.score) FILTER (WHERE f.passed = 1), 0)::float8 AS passed_average_score,
       COALESCE(AVG(a.score) FILTER (WHERE f.passed = 0), 0)::float8 AS failed_average_score,
       COALESCE(COUNT(a.id) FILTER (WHERE f.passed = 1)::numeric
                / NULLIF(COUNT(DISTINCT a.user_id) FILTER (WHERE f.passed = 1), 0), 0)::float8
           AS passed_average_attempts,
       COALESCE(COUNT(a.id) FILTER (WHERE f.passed = 0)::numeric
                / NULLIF(COUNT(DISTINCT a.user_id) FILTER (WHERE f.passed = 0), 0), 0)::float8
           AS failed_average_attempts
  FROM final_outcomes f
  LEFT JOIN attempts a ON a.user_id = f.user_id`

// kpiCurriculumQuery mengukur berapa peserta yang lolos tahap akhir juga
// menyelesaikan kurikulum (punya sertifikat atau menyelesaikan ≥90% pelajaran aktif).
const kpiCurriculumQuery = `
WITH final_passers AS (
    SELECT DISTINCT o.user_id
      FROM selection_outcomes o
      JOIN users u ON u.id = o.user_id
      JOIN selection_stages s ON s.key = o.stage_key AND s.is_final
     WHERE u.role = 'peserta' AND o.status = 'lolos'%s
),
completed AS (
    SELECT p.user_id
      FROM final_passers p
     WHERE EXISTS (
               SELECT 1 FROM certificates c
                WHERE c.user_id = p.user_id AND c.revoked_at IS NULL
           )
        OR COALESCE((
               SELECT COUNT(*)
                 FROM lesson_progress lp
                 JOIN lessons l ON l.id = lp.lesson_id AND l.active
                WHERE lp.user_id = p.user_id AND lp.completed
           ), 0) >= GREATEST(1, CEIL((SELECT COUNT(*) FROM lessons l2 WHERE l2.active) * 0.9))
)
SELECT (SELECT COUNT(*) FROM final_passers) AS passed_total,
       (SELECT COUNT(*) FROM completed)     AS passed_completed`

// kpiFunnelQuery menghitung angka dasar funnel: terdaftar → berlatih → kurikulum
// selesai → melapor. Tahapan seleksi ditambahkan dari `kpiStagePassQuery` sehingga
// funnel tidak pernah menaikkan angka (§7.1).
const kpiFunnelQuery = `
WITH participants AS (
    SELECT u.id
      FROM users u
     WHERE u.role = 'peserta'%s
),
practiced AS (
    SELECT DISTINCT a.user_id
      FROM attempts a
      JOIN participants p ON p.id = a.user_id
),
completed AS (
    SELECT p.id
      FROM participants p
     WHERE EXISTS (
               SELECT 1 FROM certificates c
                WHERE c.user_id = p.id AND c.revoked_at IS NULL
           )
        OR COALESCE((
               SELECT COUNT(*)
                 FROM lesson_progress lp
                 JOIN lessons l ON l.id = lp.lesson_id AND l.active
                WHERE lp.user_id = p.id AND lp.completed
           ), 0) >= GREATEST(1, CEIL((SELECT COUNT(*) FROM lessons l2 WHERE l2.active) * 0.9))
)
SELECT (SELECT COUNT(*) FROM participants) AS registered,
       (SELECT COUNT(*) FROM practiced)    AS practiced,
       (SELECT COUNT(*) FROM completed)    AS curriculum_completed,
       (SELECT COUNT(DISTINCT o.user_id)
          FROM selection_outcomes o
          JOIN participants p ON p.id = o.user_id
         WHERE TRUE%s)                     AS reported`

// kpiTimeToPassQuery menghitung median hari dari percobaan pertama peserta sampai
// tanggal pengumuman kelulusan tahap akhir.
const kpiTimeToPassQuery = `
WITH first_attempt AS (
    SELECT a.user_id, MIN(a.taken_at) AS first_at
      FROM attempts a
     GROUP BY a.user_id
),
final_pass AS (
    SELECT o.user_id, MIN(o.announced_at) AS announced_at
      FROM selection_outcomes o
      JOIN users u ON u.id = o.user_id
      JOIN selection_stages s ON s.key = o.stage_key AND s.is_final
     WHERE u.role = 'peserta' AND o.status = 'lolos' AND o.announced_at IS NOT NULL%s
     GROUP BY o.user_id
)
SELECT COALESCE(
           PERCENTILE_CONT(0.5) WITHIN GROUP (
               ORDER BY EXTRACT(EPOCH FROM (f.announced_at::timestamp - a.first_at)) / 86400.0
           ), 0
       )::float8 AS median_days
  FROM final_pass f
  JOIN first_attempt a ON a.user_id = f.user_id`

/* --------------------------------- metode ---------------------------------- */

// finalStageFilter mengembalikan salinan filter tanpa penyaring tahap.
//
// Query yang selalu berbicara tentang tahap **akhir** (efektivitas latihan,
// kelengkapan kurikulum, median waktu) tidak boleh ikut disaring tahap pilihan
// pengguna — kalau tidak, angkanya menjadi nol saat dasbor difilter ke tahap awal.
func (f KPIFilter) finalStageFilter() KPIFilter {
	f.StageKey = ""
	return f
}

// KPIParticipants menghitung peserta, pelapor, dan pelapor terverifikasi.
func (r *OutcomeRepository) KPIParticipants(ctx context.Context, filter KPIFilter) (*KPIParticipantsRow, error) {
	outcomeSQL, outcomeArgs := filter.outcomeConditions()
	participantSQL, participantArgs := filter.participantConditions()
	args := append(outcomeArgs, participantArgs...)

	var row KPIParticipantsRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiParticipantsQuery, outcomeSQL, participantSQL), args...).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung populasi KPI.")
	}
	return &row, nil
}

// KPIStagePass menghitung capaian tiap tahap seleksi pada kelompok terfilter.
func (r *OutcomeRepository) KPIStagePass(ctx context.Context, filter KPIFilter) ([]KPIStagePassRow, error) {
	outcomeSQL, outcomeArgs := filter.outcomeConditions()
	participantSQL, participantArgs := filter.participantConditions()
	args := append(outcomeArgs, participantArgs...)

	var rows []KPIStagePassRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiStagePassQuery, outcomeSQL+participantSQL), args...).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung capaian per tahap seleksi.")
	}
	return rows, nil
}

// KPIBreakdown menghitung capaian satu tahap per kelompok.
//
// `dimension` menerima `sector`, `cohort`, atau `program`; `stageKey` adalah tahap
// yang diukur (biasanya tahap akhir). Filter tahap pada `filter` diabaikan agar
// penyebutnya tidak menjadi nol.
func (r *OutcomeRepository) KPIBreakdown(
	ctx context.Context,
	filter KPIFilter,
	dimension string,
	stageKey string,
) ([]KPIBreakdownRow, error) {
	exprs, ok := kpiBreakdownGroupExprs[dimension]
	if !ok {
		return nil, apperror.Validation("Dimensi kelompok KPI tidak dikenal.", nil)
	}

	scoped := filter.finalStageFilter()
	outcomeSQL, outcomeArgs := scoped.outcomeConditions()
	participantSQL, participantArgs := scoped.participantConditions()
	args := append([]any{stageKey}, outcomeArgs...)
	args = append(args, participantArgs...)

	var rows []KPIBreakdownRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiBreakdownTemplate, exprs[0], exprs[1], outcomeSQL+participantSQL), args...).
		Scan(&rows).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung capaian per kelompok KPI.")
	}
	return rows, nil
}

// KPIPractice menghitung rata-rata skor & jumlah percobaan peserta lolos vs tidak.
func (r *OutcomeRepository) KPIPractice(ctx context.Context, filter KPIFilter) (*KPIPracticeRow, error) {
	scoped := filter.finalStageFilter()
	outcomeSQL, outcomeArgs := scoped.outcomeConditions()
	participantSQL, participantArgs := scoped.participantConditions()
	args := append(outcomeArgs, participantArgs...)

	var row KPIPracticeRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiPracticeQuery, outcomeSQL+participantSQL), args...).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung efektivitas latihan.")
	}
	return &row, nil
}

// KPICurriculum menghitung kelengkapan kurikulum peserta yang lolos tahap akhir.
func (r *OutcomeRepository) KPICurriculum(ctx context.Context, filter KPIFilter) (*KPICurriculumRow, error) {
	scoped := filter.finalStageFilter()
	outcomeSQL, outcomeArgs := scoped.outcomeConditions()
	participantSQL, participantArgs := scoped.participantConditions()
	args := append(outcomeArgs, participantArgs...)

	var row KPICurriculumRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiCurriculumQuery, outcomeSQL+participantSQL), args...).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung kelengkapan kurikulum.")
	}
	return &row, nil
}

// KPIFunnel menghitung angka dasar funnel pada kelompok terfilter.
func (r *OutcomeRepository) KPIFunnel(ctx context.Context, filter KPIFilter) (*KPIFunnelRow, error) {
	scoped := filter.finalStageFilter()
	outcomeSQL, outcomeArgs := scoped.outcomeConditions()
	participantSQL, participantArgs := scoped.participantConditions()

	// Query funnel memakai urutan argumen: kondisi peserta (CTE) lalu kondisi laporan.
	args := append(participantArgs, outcomeArgs...)

	var row KPIFunnelRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiFunnelQuery, participantSQL, outcomeSQL), args...).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung funnel KPI.")
	}
	return &row, nil
}

// KPITimeToPass menghitung median hari dari percobaan pertama sampai pengumuman lolos.
func (r *OutcomeRepository) KPITimeToPass(ctx context.Context, filter KPIFilter) (*KPITimeToPassRow, error) {
	scoped := filter.finalStageFilter()
	outcomeSQL, outcomeArgs := scoped.outcomeConditions()
	participantSQL, participantArgs := scoped.participantConditions()
	args := append(outcomeArgs, participantArgs...)

	var row KPITimeToPassRow
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf(kpiTimeToPassQuery, outcomeSQL+participantSQL), args...).
		Scan(&row).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menghitung waktu tempuh kelulusan.")
	}
	return &row, nil
}
