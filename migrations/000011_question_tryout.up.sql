-- 000011_question_tryout
-- Bank soal dikelompokkan per paket tryout: satu soal dimiliki SATU tryout.
--
-- Sebelumnya soal hanya punya `bank_key`, sehingga satu soal otomatis dipakai
-- semua tryout yang mencantumkan bank itu (relasi many-to-many tidak langsung
-- lewat `subtests.banks`) dan panel bank soal tidak bisa dikelompokkan per
-- tryout. Kolom `tryout_id` membuat kepemilikan itu eksplisit.
--
-- Backfill memakai prinsip "tidak ada tryout yang kehilangan soal": setiap soal
-- dipetakan ke seluruh tryout yang memakai bank-nya lewat `subtests.banks`;
-- baris aslinya dipertahankan untuk tryout pertama, tryout lain mendapat
-- SALINAN dengan id baru. Kolomnya dibiarkan NULL-able agar baris lama yang
-- bank-nya tidak dipakai tryout mana pun tetap bisa disimpan (dan tetap terlihat
-- di panel bank soal sebagai "belum ditautkan").

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS tryout_id TEXT REFERENCES tryout_packages (id) ON DELETE CASCADE;

-- Penyusunan sesi dan panel bank soal membaca soal per tryout → index kunci.
CREATE INDEX IF NOT EXISTS questions_tryout_bank_position_idx
    ON questions (tryout_id, bank_key, position, id);

-- 1) Pemetaan soal → tryout (lewat bank yang dicantumkan subtes).
WITH mapped AS (
    SELECT DISTINCT q.id AS question_id, s.tryout_id
      FROM questions q
      JOIN subtests s ON jsonb_exists(s.banks, q.bank_key)
     WHERE q.tryout_id IS NULL
),
ranked AS (
    SELECT question_id, tryout_id,
           row_number() OVER (PARTITION BY question_id ORDER BY tryout_id) AS rn
      FROM mapped
)
UPDATE questions q
   SET tryout_id = r.tryout_id
  FROM ranked r
 WHERE r.question_id = q.id
   AND r.rn = 1;

-- 2) Salinan untuk tryout lain yang memakai bank yang sama.
WITH mapped AS (
    SELECT DISTINCT q.id AS question_id, s.tryout_id
      FROM questions q
      JOIN subtests s ON jsonb_exists(s.banks, q.bank_key)
),
ranked AS (
    SELECT question_id, tryout_id,
           row_number() OVER (PARTITION BY question_id ORDER BY tryout_id) AS rn
      FROM mapped
)
INSERT INTO questions (
    id, bank_key, tryout_id, text, options, answer_index, explanation,
    difficulty, position, active, created_at, updated_at
)
SELECT gen_random_uuid()::text, q.bank_key, r.tryout_id, q.text, q.options,
       q.answer_index, q.explanation, q.difficulty, q.position, q.active,
       now(), now()
  FROM ranked r
  JOIN questions q ON q.id = r.question_id
 WHERE r.rn > 1;
