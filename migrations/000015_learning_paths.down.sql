-- 000015_learning_paths (rollback)
-- Menghapus tabel Fase 1 beserta kolom tambahan pada tabel lama.
--
-- Urutan drop mengikuti kebalikan dependensi: tabel yang merujuk tabel lain
-- dihapus lebih dulu. Data sertifikat & kuis hilang (bukan soft delete) karena
-- rollback dimaksudkan mengembalikan skema, bukan menyembunyikan data.
DROP INDEX IF EXISTS certificates_user_idx;
DROP TABLE IF EXISTS certificate_counters;
DROP TABLE IF EXISTS certificates;

DROP INDEX IF EXISTS quiz_attempts_finished_idx;
DROP INDEX IF EXISTS quiz_attempts_user_idx;
DROP TABLE IF EXISTS quiz_attempts;
DROP TABLE IF EXISTS lesson_quiz_questions;
DROP INDEX IF EXISTS lesson_quizzes_lesson_idx;
DROP TABLE IF EXISTS lesson_quizzes;

DROP INDEX IF EXISTS learning_path_steps_order_idx;
DROP TABLE IF EXISTS learning_path_steps;
DROP INDEX IF EXISTS learning_paths_taxonomy_idx;
DROP TABLE IF EXISTS learning_paths;

ALTER TABLE lessons DROP COLUMN IF EXISTS is_preview;

ALTER TABLE course_modules
    DROP COLUMN IF EXISTS test_type,
    DROP COLUMN IF EXISTS sector_key;
