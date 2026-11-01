-- 000020_psychologist (rollback)
DROP TABLE IF EXISTS instrument_interpretations;
DROP TABLE IF EXISTS assessment_disputes;
DROP TABLE IF EXISTS assessment_access_logs;
DROP TABLE IF EXISTS consultation_notes;
DROP TABLE IF EXISTS consultation_bookings;
DROP TABLE IF EXISTS consultation_slots;
DROP TABLE IF EXISTS psychologist_partners;

-- Kembalikan peran ke daftar semula. Baris dengan peran baru akan menghalangi
-- pemasangan ulang constraint; karena itu peran tersebut dinormalkan lebih dulu
-- agar `down` selalu bisa dijalankan (migrasi ini tidak menyimpan datanya).
UPDATE users SET role = 'peserta' WHERE role IN ('psikolog', 'admin_instansi');

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('peserta', 'admin'));
