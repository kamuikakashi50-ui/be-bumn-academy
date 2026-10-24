// Command seed mengisi data demo be-bumn-academy.
//
// Dipakai untuk menyiapkan database secara manual:
//
//	go run ./cmd/seed
//
// Operasi bersifat idempoten (upsert), jadi aman dijalankan berulang. Server API
// juga menjalankan seed otomatis saat SEED_ON_START=true.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/database"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/logger"
	"github.com/bumn-academy/be-bumn-academy/internal/seed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed gagal", "error", err)
		os.Exit(1)
	}
}

func run() error {
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

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			slog.Warn("gagal menutup koneksi database", "error", closeErr)
		}
	}()

	if err := seed.Run(ctx, db, cfg); err != nil {
		return err
	}

	slog.Info("seed selesai",
		"akun_demo", "peserta@bumnacademy.id / admin@bumnacademy.id",
		"media_dir", cfg.App.MediaDir,
	)
	return nil
}
