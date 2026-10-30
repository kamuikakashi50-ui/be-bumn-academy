-- 000012_question_taxonomy (rollback)
-- Menghapus dimensi taksonomi. Nilai `test_type` yang sudah di-backfill ikut
-- hilang karena kolomnya dilepas.
DROP INDEX IF EXISTS questions_retention_idx;
DROP INDEX IF EXISTS questions_variant_group_idx;
DROP INDEX IF EXISTS questions_taxonomy_idx;

ALTER TABLE questions
    DROP COLUMN IF EXISTS retention_until,
    DROP COLUMN IF EXISTS variant_group,
    DROP COLUMN IF EXISTS position_family,
    DROP COLUMN IF EXISTS test_type,
    DROP COLUMN IF EXISTS company_key,
    DROP COLUMN IF EXISTS sector_key;
