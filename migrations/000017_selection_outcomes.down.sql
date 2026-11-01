-- 000017_selection_outcomes (rollback)
-- Menghapus seluruh tabel KPI kelulusan (tracer study hasil seleksi).
--
-- Urutan drop mengikuti kebalikan dependensi: tabel audit & target dihapus lebih
-- dulu, lalu hasil seleksi, terakhir referensi tahapan (yang dijadikan FK oleh
-- keduanya). Data hasil seleksi hilang (bukan soft delete) karena rollback
-- dimaksudkan mengembalikan skema, bukan menyembunyikan data.

DROP INDEX IF EXISTS kpi_targets_period_idx;
DROP INDEX IF EXISTS kpi_targets_unique_idx;
DROP TABLE IF EXISTS kpi_targets;

DROP INDEX IF EXISTS selection_outcome_audits_outcome_idx;
DROP TABLE IF EXISTS selection_outcome_audits;

DROP INDEX IF EXISTS selection_outcomes_user_year_idx;
DROP INDEX IF EXISTS selection_outcomes_pending_idx;
DROP INDEX IF EXISTS selection_outcomes_cohort_idx;
DROP INDEX IF EXISTS selection_outcomes_sector_idx;
DROP INDEX IF EXISTS selection_outcomes_stage_idx;
DROP INDEX IF EXISTS selection_outcomes_unique_idx;
DROP TABLE IF EXISTS selection_outcomes;

DROP TABLE IF EXISTS selection_stages;
