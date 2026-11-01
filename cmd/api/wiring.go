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

	cohortRepo := repository.NewCohortRepository(db)
	// Repository KPI kelulusan (Fase 2): tahapan seleksi, laporan hasil seleksi,
	// jejak audit verifikasi, target KPI, dan query agregatnya.
	outcomeRepo := repository.NewOutcomeRepository(db)
	// Repository kompetensi (Fase 3): kamus, standar jabatan, penilaian peserta,
	// dan pemetaan kategori bank soal.
	competencyRepo := repository.NewCompetencyRepository(db)
	// Repository talent mapping (Fase 3): instrumen minat–bakat, skor & norma,
	// bobot lini bisnis, dan rekaman persetujuan.
	talentRepo := repository.NewTalentRepository(db)
	// Repository psikolog & konsultasi (Fase 3 langkah 6–7): mitra + izin praktik,
	// slot, pemesanan, catatan, jejak akses, dan koreksi hasil.
	psychologistRepo := repository.NewPsychologistRepository(db)
	licenses := repository.NewLicenseRepository(db)
	fingerprints := repository.NewComplianceRepository(db)
	learningPaths := repository.NewLearningPathRepository(db)

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

	// Pemeriksa akses materi dipakai ruang kelas, jalur belajar, kuis, dan
	// sertifikat: satu titik aturan (kepemilikan produk, kelas khusus, materi contoh).
	accessChecker := service.NewAccessChecker(enrollments, cohortRepo)

	billingService := service.NewBillingService(catalog, orders, enrollments, users, learning, midtransClient, cfg)
	tryoutService := service.NewTryoutService(tryouts, sessions, attempts, enrollments, licenses, cohortRepo, cacheLayer, cfg)
	classroomService := service.NewClassroomService(classroomRepo, catalog, accessChecker, cfg)
	corporateService := service.NewCorporateService(attempts, users, cacheLayer, cfg)
	learningService := service.NewLearningService(learning, attempts, enrollments, tryoutService, classroomService, cohortRepo, cacheLayer, cfg)
	complianceService := service.NewComplianceService(fingerprints, tryouts, cfg, logger)
	licenseService := service.NewLicenseService(licenses, tryouts, tryoutService, cfg, logger)
	learningPathService := service.NewLearningPathService(
		learningPaths, classroomRepo, catalog, tryouts, licenses, accessChecker, cfg,
	)
	cohortService := service.NewCohortService(cohortRepo, users, classroomRepo, cfg)
	certificateService := service.NewCertificateService(
		learningPaths, classroomRepo, users, accessChecker, cfg,
	)
	// KPI kelulusan (Fase 2): tracer study hasil seleksi + verifikasi bukti, lalu
	// perhitungan KPI, ekspor, dan ringkasan publiknya.
	outcomeService := service.NewOutcomeService(outcomeRepo, cohortRepo, cfg)
	kpiService := service.NewKPIService(outcomeRepo, cfg)
	// Kompetensi (Fase 3): kamus + standar jabatan + profil peserta (proksi tryout).
	competencyService := service.NewCompetencyService(competencyRepo)
	// Talent mapping (Fase 3): instrumen minat–bakat, skoring & norma, dan fit score.
	talentService := service.NewTalentService(talentRepo, competencyRepo)
	// Psikolog & konsultasi daring (Fase 3 langkah 6–7): mitra + izin praktik,
	// slot & pemesanan, catatan berlapis, jejak akses, dan koreksi hasil.
	psychologistService := service.NewPsychologistService(
		psychologistRepo, talentRepo, competencyRepo, users,
	)
	// Sertifikat diterbitkan otomatis saat peserta menyelesaikan langkah wajib
	// (progres materi) atau lulus kuis jalur — dua titik, satu aturan.
	learningPathService.SetCertificateIssuer(certificateService)
	classroomService.SetCertificateIssuer(certificateService)
	adminService := service.NewAdminService(
		users, orders, enrollments, catalog, tryouts, attempts, learning,
		catalogService, tryoutService, licenseService, complianceService, cfg,
	)
	hrService := service.NewHRService(users, attempts, classroomRepo, cfg)

	// Mentor AI ("MinSmart") — provider dipilih dari konfigurasi, dengan
	// mentor heuristik lokal sebagai jaring pengaman.
	provider := ai.New(cfg, logger)
	mentorService := service.NewMentorService(
		attempts, sessions, aiRepo, tryoutService, provider, cacheLayer, cfg, logger,
	)
	// Asisten HR memakai provider yang sama, tetapi konteksnya data analitik
	// kohort sehingga jawabannya berbicara soal peserta secara agregat.
	hrChatService := service.NewHRChatService(hrService, aiRepo, provider, cfg, logger)

	return router.New(router.Options{
		Config:              cfg,
		DB:                  db,
		Cache:               cacheClient,
		Limiter:             limiter,
		Manager:             manager,
		Version:             version,
		AuthService:         authService,
		CatalogService:      catalogService,
		BillingService:      billingService,
		TryoutService:       tryoutService,
		LearningService:     learningService,
		ClassroomService:    classroomService,
		CorporateService:    corporateService,
		AdminService:        adminService,
		HRService:           hrService,
		HRChatService:       hrChatService,
		MentorService:       mentorService,
		LicenseService:      licenseService,
		ComplianceService:   complianceService,
		LearningPathService: learningPathService,
		CertificateService:  certificateService,
		CohortService:       cohortService,
		OutcomeService:      outcomeService,
		KPIService:          kpiService,
		CompetencyService:   competencyService,
		TalentService:       talentService,
		PsychologistService: psychologistService,
	})
}
