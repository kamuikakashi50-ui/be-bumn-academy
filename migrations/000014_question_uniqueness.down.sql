-- 000014_question_uniqueness (rollback)
DROP INDEX IF EXISTS question_access_logs_actor_idx;
DROP INDEX IF EXISTS question_access_logs_question_idx;
DROP TABLE IF EXISTS question_access_logs;

DROP INDEX IF EXISTS question_fingerprints_template_idx;
DROP INDEX IF EXISTS question_fingerprints_hash_idx;
DROP TABLE IF EXISTS question_fingerprints;
