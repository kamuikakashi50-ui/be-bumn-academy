// Package migrations membungkus berkas migrasi SQL agar bisa di-embed ke
// dalam binary. Penamaan berkas mengikuti konvensi golang-migrate
// (NNNNNN_nama.up.sql / .down.sql) sehingga bisa juga dijalankan dengan
// `migrate` CLI bila diperlukan.
package migrations

import "embed"

// FS berisi seluruh berkas migrasi .sql.
//
//go:embed *.sql
var FS embed.FS
