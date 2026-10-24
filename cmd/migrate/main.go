// Command migrate menjalankan migrasi SQL yang di-embed ke dalam binary.
//
// Migrasi memakai advisory lock PostgreSQL sehingga aman dijalankan bersamaan
// dengan replika API yang sedang start (lihat internal/database).
//
// Pemakaian:
//
//	go run ./cmd/migrate up            # terapkan seluruh migrasi tertunda
//	go run ./cmd/migrate down          # rollback 1 migrasi terakhir
//	go run ./cmd/migrate down -steps 3 # rollback 3 migrasi terakhir
//	go run ./cmd/migrate status        # lihat migrasi diterapkan & tertunda
//
// Konfigurasi database dibaca dari environment/.env yang sama dengan API
// (DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE).
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/database"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/logger"
)

const usage = `migrate — runner migrasi be-bumn-academy

Pemakaian:
  migrate up            terapkan seluruh migrasi yang belum dijalankan
  migrate down [-steps] rollback migrasi terakhir (default 1)
  migrate status        tampilkan migrasi diterapkan & tertunda
`

func main() {
	if err := run(); err != nil {
		slog.Error("migrasi gagal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	command, steps, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	level := "info"
	if cfg.IsDevelopment() {
		level = "debug"
	}
	logger.New(logger.Options{Level: level, Pretty: cfg.IsDevelopment()})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sqlDB, err := database.OpenSQL(cfg.DB.URL())
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	switch command {
	case "up":
		return applyUp(ctx, sqlDB, steps)
	case "down":
		return applyDown(ctx, sqlDB, steps)
	case "status":
		return printStatus(ctx, sqlDB)
	default:
		return fmt.Errorf("perintah %q tidak dikenal", command)
	}
}

// parseArgs membaca perintah dan opsinya.
//
// `down` menerima flag -steps; perintah lain tidak menerima argumen tambahan.
func parseArgs(args []string) (command string, steps int, err error) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return "", 0, errors.New("perintah migrasi wajib diisi")
	}

	command = args[0]
	switch command {
	case "up":
		// steps = 0 → seluruh migrasi yang tertunda dijalankan.
		return command, 0, nil
	case "down":
		fs := flag.NewFlagSet("migrate down", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		fs.IntVar(&steps, "steps", 1, "jumlah migrasi yang di-rollback")
		if err := fs.Parse(args[1:]); err != nil {
			return "", 0, err
		}
		if steps < 1 {
			return "", 0, errors.New("migrate: -steps minimal 1")
		}
		return command, steps, nil
	case "status":
		return command, 0, nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return "", 0, fmt.Errorf("perintah %q tidak dikenal", command)
	}
}

// applyUp menerapkan migrasi yang belum dijalankan.
func applyUp(ctx context.Context, sqlDB *sql.DB, steps int) error {
	applied, err := database.Migrate(ctx, sqlDB, database.RunOptions{
		Direction: database.Up,
		Steps:     steps,
	})
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		slog.Info("migrasi: database sudah paling baru")
		return nil
	}
	for _, record := range applied {
		slog.Info("migrasi diterapkan", "version", record.Version, "name", record.Name)
	}
	fmt.Printf("\n%d migrasi diterapkan.\n", len(applied))
	return nil
}

// applyDown me-rollback migrasi terakhir.
func applyDown(ctx context.Context, sqlDB *sql.DB, steps int) error {
	rolledBack, err := database.Migrate(ctx, sqlDB, database.RunOptions{
		Direction: database.Down,
		Steps:     steps,
	})
	if err != nil {
		return err
	}
	if len(rolledBack) == 0 {
		slog.Info("migrasi: tidak ada migrasi yang bisa di-rollback")
		return nil
	}
	for _, record := range rolledBack {
		slog.Info("migrasi di-rollback", "version", record.Version, "name", record.Name)
	}
	fmt.Printf("\n%d migrasi di-rollback.\n", len(rolledBack))
	return nil
}

// printStatus menampilkan migrasi yang sudah diterapkan dan yang masih tertunda.
func printStatus(ctx context.Context, sqlDB *sql.DB) error {
	applied, pending, err := database.Status(ctx, sqlDB)
	if err != nil {
		return err
	}

	fmt.Println("\nDiterapkan:")
	if len(applied) == 0 {
		fmt.Println("  (belum ada)")
	}
	for _, record := range applied {
		fmt.Printf("  ✓ %06d_%s  %s\n", record.Version, record.Name, record.AppliedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Println("\nTertunda:")
	if len(pending) == 0 {
		fmt.Println("  (tidak ada)")
	}
	for _, record := range pending {
		fmt.Printf("  • %06d_%s\n", record.Version, record.Name)
	}
	fmt.Printf("\nRingkasan: %d diterapkan, %d tertunda.\n", len(applied), len(pending))
	return nil
}
