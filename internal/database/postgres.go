// Package database menyediakan koneksi PostgreSQL (GORM) dan runner migrasi.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Connect membuka koneksi PostgreSQL dan menyetel connection pool.
//
// Penyetelan pool adalah bagian penting dari optimasi konkurensi: tanpa
// SetMaxOpenConns, GORM/database-sql akan membuka koneksi tanpa batas saat
// ribuan peserta menekan endpoint tryout bersamaan dan PostgreSQL akan
// kehabisan slot (max_connections).
func Connect(ctx context.Context, cfg *config.Config) (*gorm.DB, error) {
	gormLogLevel := gormlogger.Warn
	if cfg.IsDevelopment() {
		gormLogLevel = gormlogger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{
		// SkipDefaultTransaction mematikan transaksi implisit pada setiap
		// Create/Update tunggal → hemat round-trip dan mengurangi beban WAL
		// saat trafik tinggi. Transaksi eksplisit tetap dipakai di service
		// yang memang memerlukan atomicity (mis. submit tryout).
		SkipDefaultTransaction: true,
		// Prepared statement cache: query berulang (autosave jawaban, ambil
		// soal) tidak perlu di-parse ulang oleh PostgreSQL.
		PrepareStmt: cfg.DB.PrepareStmt,
		NowFunc:     func() time.Time { return time.Now().UTC() },
		Logger: gormlogger.New(sqlWriter{}, gormlogger.Config{
			SlowThreshold:             400 * time.Millisecond,
			LogLevel:                  gormLogLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  cfg.IsDevelopment(),
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("database: gagal membuka koneksi: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: gagal mengambil *sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.DB.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("database: PostgreSQL tidak merespons: %w", err)
	}

	slog.Info("postgres terhubung",
		"host", cfg.DB.Host,
		"db", cfg.DB.Name,
		"max_open_conns", cfg.DB.MaxOpenConns,
		"max_idle_conns", cfg.DB.MaxIdleConns,
		"prepare_stmt", cfg.DB.PrepareStmt,
	)

	return db, nil
}

// Stats mengembalikan statistik pool untuk endpoint /healthz.
func Stats(db *gorm.DB) (map[string]any, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	s := sqlDB.Stats()
	return map[string]any{
		"open_connections": s.OpenConnections,
		"in_use":           s.InUse,
		"idle":             s.Idle,
		"max_open":         s.MaxOpenConnections,
		"wait_count":       s.WaitCount,
		"wait_duration_ms": s.WaitDuration.Milliseconds(),
	}, nil
}

// Close menutup pool koneksi dengan rapi.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// sqlWriter menjembatani logger GORM ke log/slog agar format log konsisten.
type sqlWriter struct{}

func (sqlWriter) Printf(format string, args ...any) {
	slog.Debug("gorm", "msg", fmt.Sprintf(format, args...))
}
