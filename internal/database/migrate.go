package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/bumn-academy/be-bumn-academy/migrations"
	_ "github.com/jackc/pgx/v5/stdlib" // driver "pgx" untuk database/sql
)

// Direction arah migrasi.
type Direction string

const (
	// Up menjalankan migrasi.
	Up Direction = "up"
	// Down me-rollback migrasi.
	Down Direction = "down"
)

// migrationLockKey adalah advisory lock PostgreSQL supaya hanya satu proses
// yang menjalankan migrasi walau beberapa replika API start bersamaan.
const migrationLockKey int64 = 7669381101

var migrationFileRe = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// Migration adalah satu berkas migrasi (pasangan up/down).
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// MigrationRecord adalah baris pada tabel schema_migrations.
type MigrationRecord struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at,omitempty"`
}

// OpenSQL membuka *sql.DB memakai driver pgx (dipakai runner migrasi).
func OpenSQL(dsnURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsnURL)
	if err != nil {
		return nil, fmt.Errorf("migrate: gagal membuka koneksi: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// LoadMigrations membaca dan mengurutkan seluruh berkas migrasi yang di-embed.
func LoadMigrations() ([]Migration, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("migrate: gagal membaca direktori migrasi: %w", err)
	}

	byVersion := map[int]*Migration{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := migrationFileRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		version, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			return nil, fmt.Errorf("migrate: nomor versi tidak valid pada %s: %w", entry.Name(), convErr)
		}
		content, readErr := migrations.FS.ReadFile(entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("migrate: gagal membaca %s: %w", entry.Name(), readErr)
		}

		current, ok := byVersion[version]
		if !ok {
			current = &Migration{Version: version, Name: m[2]}
			byVersion[version] = current
		}
		if m[3] == "up" {
			current.Up = string(content)
		} else {
			current.Down = string(content)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, mig := range byVersion {
		if mig.Up == "" {
			return nil, fmt.Errorf("migrate: migrasi %d (%s) tidak punya berkas .up.sql", mig.Version, mig.Name)
		}
		out = append(out, *mig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// RunOptions mengatur perilaku runner.
type RunOptions struct {
	Direction Direction
	// Steps membatasi jumlah migrasi yang dijalankan (0 = semua untuk up,
	// 1 untuk down).
	Steps int
}

// Migrate menjalankan migrasi. Operasi dibungkus advisory lock + transaksi
// per berkas sehingga aman dipanggil dari beberapa replika sekaligus.
func Migrate(ctx context.Context, sqlDB *sql.DB, opts RunOptions) ([]MigrationRecord, error) {
	if err := ensureSchemaTable(ctx, sqlDB); err != nil {
		return nil, err
	}

	all, err := LoadMigrations()
	if err != nil {
		return nil, err
	}

	applied, err := appliedVersions(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	// Advisory lock harus berada di satu koneksi yang sama.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate: gagal mengambil koneksi: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return nil, fmt.Errorf("migrate: gagal mengambil advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if opts.Direction == Down {
		return runDown(ctx, conn, all, applied, opts.Steps)
	}
	return runUp(ctx, conn, all, applied, opts.Steps)
}

func runUp(ctx context.Context, conn *sql.Conn, all []Migration, applied map[int]bool, steps int) ([]MigrationRecord, error) {
	var done []MigrationRecord
	for _, mig := range all {
		if applied[mig.Version] {
			continue
		}
		if steps > 0 && len(done) >= steps {
			break
		}
		if err := execMigrationFile(ctx, conn, mig.Up); err != nil {
			return done, fmt.Errorf("migrate up %d_%s: %w", mig.Version, mig.Name, err)
		}
		recordedAt := time.Now().UTC()
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES ($1, $2, $3)`,
			mig.Version, mig.Name, recordedAt,
		); err != nil {
			return done, fmt.Errorf("migrate up %d_%s: gagal mencatat versi: %w", mig.Version, mig.Name, err)
		}
		done = append(done, MigrationRecord{Version: mig.Version, Name: mig.Name, AppliedAt: recordedAt})
	}
	return done, nil
}

func runDown(ctx context.Context, conn *sql.Conn, all []Migration, applied map[int]bool, steps int) ([]MigrationRecord, error) {
	// Jalankan dari versi terbesar ke terkecil.
	candidates := make([]Migration, 0, len(all))
	for _, mig := range all {
		if applied[mig.Version] {
			candidates = append(candidates, mig)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Version > candidates[j].Version })

	if steps <= 0 {
		steps = 1
	}

	var done []MigrationRecord
	for _, mig := range candidates {
		if len(done) >= steps {
			break
		}
		if mig.Down == "" {
			return done, fmt.Errorf("migrate down %d_%s: berkas .down.sql tidak tersedia", mig.Version, mig.Name)
		}
		if err := execMigrationFile(ctx, conn, mig.Down); err != nil {
			return done, fmt.Errorf("migrate down %d_%s: %w", mig.Version, mig.Name, err)
		}
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM schema_migrations WHERE version = $1`, mig.Version,
		); err != nil {
			return done, fmt.Errorf("migrate down %d_%s: gagal menghapus catatan versi: %w", mig.Version, mig.Name, err)
		}
		done = append(done, MigrationRecord{Version: mig.Version, Name: mig.Name})
	}
	return done, nil
}

// execMigrationFile menjalankan seluruh isi berkas dalam satu transaksi.
//
// Isi berkas dikirim sebagai satu perintah tanpa argumen → driver pgx memakai
// simple query protocol yang mendukung banyak statement sekaligus, sehingga
// tidak perlu memecah SQL manual (yang rawan salah pada string/komentar).
func execMigrationFile(ctx context.Context, conn *sql.Conn, content string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Status mengembalikan daftar migrasi yang sudah diterapkan dan yang masih
// tertunda.
func Status(ctx context.Context, sqlDB *sql.DB) (applied []MigrationRecord, pending []MigrationRecord, err error) {
	if err = ensureSchemaTable(ctx, sqlDB); err != nil {
		return nil, nil, err
	}
	all, err := LoadMigrations()
	if err != nil {
		return nil, nil, err
	}
	appliedSet, err := appliedVersions(ctx, sqlDB)
	if err != nil {
		return nil, nil, err
	}

	for _, mig := range all {
		if !appliedSet[mig.Version] {
			pending = append(pending, MigrationRecord{Version: mig.Version, Name: mig.Name})
		}
	}

	applied, err = appliedRecords(ctx, sqlDB)
	if err != nil {
		return nil, nil, err
	}
	return applied, pending, nil
}

func ensureSchemaTable(ctx context.Context, sqlDB *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT        NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`
	if _, err := sqlDB.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("migrate: gagal menyiapkan tabel schema_migrations: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, sqlDB *sql.DB) (map[int]bool, error) {
	records, err := appliedRecords(ctx, sqlDB)
	if err != nil {
		return nil, err
	}
	out := make(map[int]bool, len(records))
	for _, r := range records {
		out[r.Version] = true
	}
	return out, nil
}

func appliedRecords(ctx context.Context, sqlDB *sql.DB) ([]MigrationRecord, error) {
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT version, name, applied_at FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("migrate: gagal membaca schema_migrations: %w", err)
	}
	defer rows.Close()

	var out []MigrationRecord
	for rows.Next() {
		var r MigrationRecord
		if err := rows.Scan(&r.Version, &r.Name, &r.AppliedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
