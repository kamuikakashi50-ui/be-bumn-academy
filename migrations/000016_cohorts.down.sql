-- 000016_cohorts (rollback)
-- Menghapus tabel kelas khusus beserta kolom tambahan pada tabel lama.
--
-- Urutan drop mengikuti kebalikan dependensi: tabel yang merujuk tabel lain
-- dihapus lebih dulu, dan kolom yang merujuk `cohorts`/`organizations` dilepas
-- sebelum kedua tabel itu hilang. Data keanggotaan, kurikulum, kode undangan, dan
-- jejak audit hilang (bukan soft delete) karena rollback dimaksudkan
-- mengembalikan skema, bukan menyembunyikan data.

ALTER TABLE certificates DROP CONSTRAINT IF EXISTS certificates_cohort_fk;

DROP INDEX IF EXISTS attempts_cohort_idx;
ALTER TABLE attempts DROP COLUMN IF EXISTS cohort_id;

DROP INDEX IF EXISTS live_sessions_cohort_idx;
ALTER TABLE live_sessions DROP COLUMN IF EXISTS cohort_id;

DROP INDEX IF EXISTS questions_owner_org_idx;
ALTER TABLE questions DROP COLUMN IF EXISTS owner_org_id;

DROP INDEX IF EXISTS cohort_audit_logs_cohort_idx;
DROP TABLE IF EXISTS cohort_audit_logs;

DROP INDEX IF EXISTS cohort_invites_cohort_idx;
DROP TABLE IF EXISTS cohort_invites;

DROP INDEX IF EXISTS cohort_curriculum_order_idx;
DROP TABLE IF EXISTS cohort_curriculum;

DROP INDEX IF EXISTS cohort_members_user_idx;
DROP TABLE IF EXISTS cohort_members;

DROP INDEX IF EXISTS cohorts_taxonomy_idx;
DROP INDEX IF EXISTS cohorts_org_idx;
DROP TABLE IF EXISTS cohorts;

DROP INDEX IF EXISTS organizations_sector_idx;
DROP TABLE IF EXISTS organizations;
