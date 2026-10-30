-- 000013_question_license (rollback)
-- Menghapus tabel izin soal & instrumen psikologi. Backfill lisensi ikut hilang,
-- sehingga penegakan izin (bila menyala) akan menganggap seluruh soal belum
-- berizin.
DROP INDEX IF EXISTS psychological_instruments_status_idx;
DROP TABLE IF EXISTS psychological_instruments;

DROP INDEX IF EXISTS question_licenses_status_idx;
DROP INDEX IF EXISTS question_licenses_question_idx;
DROP TABLE IF EXISTS question_licenses;
