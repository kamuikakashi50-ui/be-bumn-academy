-- 000011_question_tryout (rollback)
-- Catatan: salinan soal yang dibuat saat backfill tidak bisa digabungkan kembali
-- secara otomatis (isinya identik), jadi rollback hanya melepas kolom & index.
DROP INDEX IF EXISTS questions_tryout_bank_position_idx;
ALTER TABLE questions DROP COLUMN IF EXISTS tryout_id;
