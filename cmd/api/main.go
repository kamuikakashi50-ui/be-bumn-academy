// Command api menjalankan HTTP server be-bumn-academy.
//
// Alur start-up:
//  1. muat konfigurasi & logger,
//  2. sambung PostgreSQL (pool sudah di-tuning) dan Redis (opsional),
//  3. jalankan migrasi + seed bila diaktifkan,
//  4. susun repository → service → handler → router,
//  5. jalankan server dengan graceful shutdown + worker latar.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/database"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/logger"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
	"github.com/bumn-academy/be-bumn-academy/internal/seed"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
	"gorm.io/gorm"
)

// version dapat diisi saat build: -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("aplikasi berhenti karena error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logLevel := "info"
	if cfg.IsDevelopment() {
		logLevel = "debug"
	}
	logger.New(logger.Options{Level: logLevel, Pretty: cfg.IsDevelopment()})

	slog.Info("memulai be-bumn-academy", "version", version, "env", cfg.App.Env)

	// Context yang dibatalkan saat menerima sinyal berhenti.
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

	if cfg.DB.AutoMigrate {
		if err := runMigrations(ctx, cfg); err != nil {
			return err
		}
	}

	if cfg.Seed.OnStart {
		if err := seed.Run(ctx, db, cfg); err != nil {
			return err
		}
	}

	cacheClient := cache.New(ctx, cfg)
	defer func() {
		if closeErr := cacheClient.Close(); closeErr != nil {
			slog.Warn("gagal menutup koneksi redis", "error", closeErr)
		}
	}()

	cacheLayer := cache.NewCache(cacheClient)
	limiter := cache.NewLimiter(cacheClient, cfg.RateLimit.RPS, cfg.RateLimit.Burst)
	manager := auth.NewManager(cfg)

	handler := buildRouter(cfg, db, cacheClient, cacheLayer, limiter, manager, slog.Default())

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           handler,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Worker latar: menyapu sesi tryout kedaluwarsa & refresh token lama.
	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	go runMaintenance(workerCtx, cfg, db, service.NewTryoutService(
		repository.NewTryoutRepository(db),
		repository.NewSessionRepository(db),
		repository.NewAttemptRepository(db),
		repository.NewEnrollmentRepository(db),
		cacheLayer,
		cfg,
	), repository.NewUserRepository(db))

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server siap", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("sinyal berhenti diterima, menutup server…")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("gagal menutup server dengan rapi: %w", err)
	}

	slog.Info("server berhenti dengan rapi")
	return nil
}

// runMigrations menjalankan seluruh migrasi yang belum diterapkan.
func runMigrations(ctx context.Context, cfg *config.Config) error {
	sqlDB, err := database.OpenSQL(cfg.DB.URL())
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	applied, err := database.Migrate(ctx, sqlDB, database.RunOptions{Direction: database.Up})
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
	return nil
}

// runMaintenance menjalankan tugas berkala di latar belakang.
func runMaintenance(
	ctx context.Context,
	cfg *config.Config,
	db *gorm.DB,
	tryoutSvc *service.TryoutService,
	users *repository.UserRepository,
) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if total, err := tryoutSvc.ExpireStaleSessions(runCtx, 500); err != nil {
				slog.Warn("gagal menyapu sesi kedaluwarsa", "error", err)
			} else if total > 0 {
				slog.Info("sesi tryout kedaluwarsa ditandai", "total", total)
			}

			if total, err := users.DeleteExpiredTokens(runCtx); err != nil {
				slog.Warn("gagal membersihkan refresh token", "error", err)
			} else if total > 0 {
				slog.Info("refresh token kedaluwarsa dibersihkan", "total", total)
			}

			cancel()
		}
	}
}
