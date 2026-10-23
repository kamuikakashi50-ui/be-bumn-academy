-- 000007_order_sequence
-- Sequence untuk nomor invoice agar pembuatan nomor aman saat checkout
-- serentak (tidak bergantung pada COUNT(*) yang rawan race condition).

CREATE SEQUENCE IF NOT EXISTS order_invoice_seq START WITH 1 INCREMENT BY 1;
