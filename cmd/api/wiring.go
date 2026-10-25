package main

import (
	"log/slog"
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/ai"
	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/midtrans"
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
	classroomRepo := repository.NewClassroomRepository(db)
	aiRepo := repository.NewAIRepository(db)

	// Service.
	authService := service.NewAuthService(users, manager, cfg)
	catalogService := service.NewCatalogService(catalog, enrollments, cacheLayer, cfg)

	// Klien Midtrans (Snap) — dipakai billing untuk pembuatan sesi pembayaran,
	// rekonsiliasi status, dan verifikasi signature notifikasi.
	midtransClient := midtrans.New(midtrans.Config{
		ServerKey:    cfg.Business.MidtransServerKey,
		ClientKey:    cfg.Business.MidtransClientKey,
		IsProduction: cfg.Business.MidtransIsProduction,
		Timeout:      cfg.Business.MidtransTimeout,
		SnapBaseURL:  cfg.Business.MidtransSnapBaseURL,
		CoreBaseURL:  cfg.Business.MidtransCoreBaseURL,
	})
	logger.Info("integrasi pembayaran disiapkan", "provider", midtransClient.Describe())

	billingService := service.NewBillingService(catalog, orders, enrollments, users, learning, midtransClient, cfg)
	tryoutService := service.NewTryoutService(tryouts, sessions, attempts, enrollments, cacheLayer, cfg)
	classroomService := service.NewClassroomService(classroomRepo, enrollments, catalog, cfg)
	corporateService := service.NewCorporateService(attempts, users, cacheLayer, cfg)
	learningService := service.NewLearningService(learning, attempts, enrollments, tryoutService, classroomService, cacheLayer, cfg)
	adminService := service.NewAdminService(
		users, orders, enrollments, catalog, tryouts, attempts, learning,
		catalogService, tryoutService, cfg,
	)
	hrService := service.NewHRService(users, attempts, classroomRepo, cfg)

	// Mentor AI ("MinSmart") — provider dipilih dari konfigurasi, dengan
	// mentor heuristik lokal sebagai jaring pengaman.
	provider := ai.New(cfg, logger)
	mentorService := service.NewMentorService(
		attempts, sessions, aiRepo, tryoutService, provider, cacheLayer, cfg, logger,
	)

	return router.New(router.Options{
		Config:           cfg,
		DB:               db,
		Cache:            cacheClient,
		Limiter:          limiter,
		Manager:          manager,
		Version:          version,
		AuthService:      authService,
		CatalogService:   catalogService,
		BillingService:   billingService,
		TryoutService:    tryoutService,
		LearningService:  learningService,
		ClassroomService: classroomService,
		CorporateService: corporateService,
		AdminService:     adminService,
		HRService:        hrService,
		MentorService:    mentorService,
	})
}
