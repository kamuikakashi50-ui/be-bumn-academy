// Package router menyusun seluruh route HTTP aplikasi.
package router

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/handler"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"gorm.io/gorm"
)

// Options berisi dependensi yang dibutuhkan router.
type Options struct {
	Config  *config.Config
	DB      *gorm.DB
	Cache   *cache.Client
	Limiter *cache.Limiter
	Manager *auth.Manager
	Version string

	AuthService      *service.AuthService
	CatalogService   *service.CatalogService
	BillingService   *service.BillingService
	TryoutService    *service.TryoutService
	LearningService  *service.LearningService
	ClassroomService *service.ClassroomService
	CorporateService *service.CorporateService
	AdminService     *service.AdminService
	MentorService    *service.MentorService
}

// New menyusun router utama aplikasi.
func New(opts Options) http.Handler {
	deps := &handler.Deps{
		Auth:      opts.AuthService,
		Catalog:   opts.CatalogService,
		Billing:   opts.BillingService,
		Tryout:    opts.TryoutService,
		Learning:  opts.LearningService,
		Classroom: opts.ClassroomService,
		Corporate: opts.CorporateService,
		Admin:     opts.AdminService,
		Mentor:    opts.MentorService,
	}

	authHandler := handler.NewAuthHandler(deps)
	catalogHandler := handler.NewCatalogHandler(deps)
	billingHandler := handler.NewBillingHandler(deps)
	tryoutHandler := handler.NewTryoutHandler(deps)
	learningHandler := handler.NewLearningHandler(deps)
	classroomHandler := handler.NewClassroomHandler(deps)
	corporateHandler := handler.NewCorporateHandler(deps)
	adminHandler := handler.NewAdminHandler(deps)
	aiHandler := handler.NewAIHandler(deps)
	healthHandler := handler.NewHealthHandler(opts.DB, opts.Cache, opts.Version)
	mediaHandler := handler.NewMediaHandler(opts.Config.App.MediaDir)

	authMiddleware := middleware.NewAuth(opts.Manager)

	r := chi.NewRouter()

	// Middleware dasar: berlaku untuk semua route.
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestLogger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   opts.Config.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(chimiddleware.Compress(5))
	// Timeout per request: query yang menggantung tidak boleh menahan koneksi pool.
	r.Use(chimiddleware.Timeout(opts.Config.HTTP.RequestTimeout))

	// Pemeriksaan kesehatan (tanpa rate limit agar probe tidak pernah diblokir).
	r.Get("/healthz", healthHandler.Live)
	r.Get("/readyz", healthHandler.Ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.RateLimit(opts.Limiter))
		registerRoutes(r, routeHandlers{
			auth:      authHandler,
			catalog:   catalogHandler,
			billing:   billingHandler,
			tryout:    tryoutHandler,
			learning:  learningHandler,
			classroom: classroomHandler,
			corporate: corporateHandler,
			media:     mediaHandler,
			admin:     adminHandler,
			ai:        aiHandler,
			authMid:   authMiddleware,
		})
	})

	// Balasan standar untuk route yang tidak dikenal.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, `{"success":false,"error":{"code":"NOT_FOUND","message":"Endpoint tidak ditemukan."}}`)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMethodNotAllowed, `{"success":false,"error":{"code":"METHOD_NOT_ALLOWED","message":"Metode HTTP tidak didukung untuk endpoint ini."}}`)
	})

	return r
}

// routeHandlers mengumpulkan handler agar pemanggilan registerRoutes ringkas.
type routeHandlers struct {
	auth      *handler.AuthHandler
	catalog   *handler.CatalogHandler
	billing   *handler.BillingHandler
	tryout    *handler.TryoutHandler
	learning  *handler.LearningHandler
	classroom *handler.ClassroomHandler
	corporate *handler.CorporateHandler
	media     *handler.MediaHandler
	admin     *handler.AdminHandler
	ai        *handler.AIHandler
	authMid   *middleware.Auth
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// registerRoutes memasang seluruh endpoint di bawah /api/v1.
func registerRoutes(r chi.Router, h routeHandlers) {
	// ------------------------------ publik ------------------------------
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.auth.Register)
		r.Post("/login", h.auth.Login)
		r.Post("/refresh", h.auth.Refresh)
		r.Post("/logout", h.auth.Logout)

		r.Group(func(r chi.Router) {
			r.Use(h.authMid.Required)
			r.Get("/me", h.auth.Me)
			r.Put("/me", h.auth.UpdateProfile)
		})
	})

	r.Get("/programs", h.catalog.Programs)
	r.Get("/materi/categories", h.catalog.MateriCategories)
	r.Get("/payment-methods", h.catalog.PaymentMethods)

	// Katalog & tryout boleh diakses tamu, tetapi dipersonalisasi bila login.
	r.Group(func(r chi.Router) {
		r.Use(h.authMid.Optional)
		r.Get("/products", h.catalog.Products)
		r.Get("/tryouts", h.tryout.List)
		r.Get("/tryouts/{id}", h.tryout.Get)
		r.Get("/tryouts/{id}/leaderboard", h.tryout.Leaderboard)
	})

	// Webhook pembayaran (dipanggil server Midtrans, tanpa JWT).
	r.Post("/webhooks/midtrans", h.billing.MidtransWebhook)

	// Konfigurasi publik Midtrans (dipakai front-end untuk memuat snap.js).
	r.Get("/midtrans/config", h.billing.MidtransConfig)

	// ------------------------------ peserta -----------------------------
	r.Group(func(r chi.Router) {
		r.Use(h.authMid.Required)

		r.Get("/enrollments", h.billing.Enrollments)
		r.Post("/checkout/quote", h.billing.Quote)
		r.Get("/orders", h.billing.ListOrders)
		r.Post("/orders", h.billing.CreateOrder)
		r.Get("/orders/{id}", h.billing.GetOrder)
		r.Post("/orders/{id}/cancel", h.billing.CancelOrder)
		r.Post("/orders/{id}/pay", h.billing.PayOrder)

		// Pembayaran Midtrans (Snap) + rekonsiliasi status.
		r.Post("/orders/{id}/payment", h.billing.PaymentIntent)
		r.Get("/orders/{id}/payment/status", h.billing.PaymentStatus)
		// Simulator sandbox lokal (hanya aktif bila MIDTRANS_SANDBOX_SIMULATOR).
		r.Post("/orders/{id}/sandbox/settle", h.billing.SandboxPayment)

		r.Get("/dashboard/summary", h.learning.Dashboard)
		r.Get("/live-sessions", h.learning.LiveSessions)
		r.Post("/live-sessions/{id}/reminder", h.learning.ToggleReminder)
		r.Get("/recordings", h.learning.Recordings)

		// Ruang kelas digital: materi, progres menonton/membaca, dan e-book.
		r.Get("/classroom", h.classroom.Courses)
		r.Get("/classroom/courses/{slug}", h.classroom.Course)
		r.Get("/classroom/lessons/{id}", h.classroom.Lesson)
		r.Post("/classroom/lessons/{id}/progress", h.classroom.SaveProgress)
		r.Get("/classroom/lessons/{id}/ebook", h.classroom.Ebook)

		// Mesin tryout.
		r.Post("/tryouts/{id}/sessions", h.tryout.StartSession)
		r.Get("/tryout-sessions/{id}", h.tryout.GetSession)
		r.Patch("/tryout-sessions/{id}/answers", h.tryout.SaveAnswers)
		r.Post("/tryout-sessions/{id}/submit", h.tryout.Submit)

		r.Get("/attempts", h.tryout.ListAttempts)
		r.Get("/attempts/{id}", h.tryout.GetAttempt)

		// Mentor AI ("MinSmart") untuk ulasan hasil tryout.
		r.Get("/ai/usage", h.ai.Usage)
		r.Get("/attempts/{id}/ai/analysis", h.ai.Analysis)
		r.Get("/attempts/{id}/ai/conversation", h.ai.Conversation)
		r.Post("/attempts/{id}/ai/chat", h.ai.Chat)
		r.Post("/attempts/{id}/ai/chat/stream", h.ai.ChatStream)
		r.Post("/attempts/{id}/ai/explain", h.ai.ExplainQuestion)
		r.Delete("/attempts/{id}/ai/conversation", h.ai.ResetConversation)
	})

	// Status mentor AI bisa dilihat tamu agar UI bisa menyembunyikan panel
	// ketika fitur belum siap.
	r.Get("/ai/status", h.ai.Status)

	// Insight agregat untuk halaman korporasi (publik, tanpa data pribadi).
	r.Get("/insights/corporate", h.corporate.Insights)

	// Media pembelajaran (video & e-book) — publik agar dapat dimuat langsung
	// oleh elemen <video>/<iframe> di browser tanpa header khusus.
	r.Get("/media/*", h.media.Serve)

	// ------------------------------- admin ------------------------------
	r.Route("/admin", func(r chi.Router) {
		r.Use(h.authMid.Required)
		r.Use(h.authMid.Admin)

		r.Get("/analytics", h.admin.Analytics)
		r.Get("/users", h.admin.ListUsers)

		r.Get("/orders", h.admin.ListOrders)
		r.Post("/orders/{id}/mark-paid", h.admin.MarkOrderPaid)

		r.Get("/questions", h.admin.ListQuestions)
		r.Post("/questions", h.admin.CreateQuestion)
		r.Post("/questions/import", h.admin.ImportQuestions)
		r.Patch("/questions/{id}", h.admin.UpdateQuestion)
		r.Delete("/questions/{id}", h.admin.DeleteQuestion)

		r.Get("/tryouts", h.admin.ListTryouts)
		r.Post("/tryouts", h.admin.CreateTryout)
		r.Patch("/tryouts/{id}", h.admin.UpdateTryout)
		r.Delete("/tryouts/{id}", h.admin.DeleteTryout)

		r.Get("/products", h.admin.ListProducts)
		r.Post("/products", h.admin.CreateProduct)
		r.Patch("/products/{slug}", h.admin.UpdateProduct)
		r.Delete("/products/{slug}", h.admin.DeleteProduct)

		r.Get("/vouchers", h.catalog.Vouchers)
		r.Post("/vouchers", h.admin.CreateVoucher)
		r.Patch("/vouchers/{code}", h.admin.UpdateVoucher)
		r.Delete("/vouchers/{code}", h.admin.DeleteVoucher)

		r.Post("/broadcast", h.admin.Broadcast)
		r.Get("/broadcasts", h.admin.ListBroadcasts)

		// Ruang kelas digital: CRUD modul & pelajaran.
		r.Get("/classroom/modules", h.classroom.AdminModules)
		r.Post("/classroom/modules", h.classroom.CreateModule)
		r.Patch("/classroom/modules/{id}", h.classroom.UpdateModule)
		r.Delete("/classroom/modules/{id}", h.classroom.DeleteModule)
		r.Get("/classroom/lessons", h.classroom.AdminLessons)
		r.Post("/classroom/lessons", h.classroom.CreateLesson)
		r.Patch("/classroom/lessons/{id}", h.classroom.UpdateLesson)
		r.Delete("/classroom/lessons/{id}", h.classroom.DeleteLesson)
	})
}
