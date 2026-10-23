package main

import (
	"log/slog"
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/ai"
	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
	"github.com/bumn-academy/be-bumn-academy/internal/router"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
	"gorm.io/gorm"
)

// buildRouter menyusun seluruh dependency dan mengembalikan handler HTTP.
func buildRouter(
	cfg *config.Config,
	db *gorm.DB,
	cacheClient *cache.Client,
	cacheLayer *cache.Cache,
	limiter *cache.Limiter,
	manager *auth.Manager,
	logger *slog.Logger,
) http.Handler {
	// Repository.
	users := repository.NewUserRepository(db)
	catalog := repository.NewCatalogRepository(db)
	orders := repository.NewOrderRepository(db)
	enrollments := repository.NewEnrollmentRepository(db)
	tryouts := repository.NewTryoutRepository(db)
	sessions := repository.NewSessionRepository(db)
	attempts := repository.NewAttemptRepository(db)
	learning := repository.NewLearningRepository(db)
	aiRepo := repository.NewAIRepository(db)

	// Service.
	authService := service.NewAuthService(users, manager, cfg)
	catalogService := service.NewCatalogService(catalog, enrollments, cacheLayer, cfg)
	billingService := service.NewBillingService(catalog, orders, enrollments, users, learning, cfg)
	tryoutService := service.NewTryoutService(tryouts, sessions, attempts, enrollments, cacheLayer, cfg)
	learningService := service.NewLearningService(learning, attempts, enrollments, tryoutService, cacheLayer, cfg)
	adminService := service.NewAdminService(
		users, orders, enrollments, catalog, tryouts, attempts, learning,
		catalogService, tryoutService, cfg,
	)

	// Mentor AI ("MinSmart") — provider dipilih dari konfigurasi, dengan
	// mentor heuristik lokal sebagai jaring pengaman.
	provider := ai.New(cfg, logger)
	mentorService := service.NewMentorService(
		attempts, sessions, aiRepo, tryoutService, provider, cacheLayer, cfg, logger,
	)

	return router.New(router.Options{
		Config:          cfg,
		DB:              db,
		Cache:           cacheClient,
		Limiter:         limiter,
		Manager:         manager,
		Version:         version,
		AuthService:     authService,
		CatalogService:  catalogService,
		BillingService:  billingService,
		TryoutService:   tryoutService,
		LearningService: learningService,
		AdminService:    adminService,
		MentorService:   mentorService,
	})
}
