// Package config memuat konfigurasi aplikasi dari environment variable.
//
// Semua nilai memiliki default yang aman untuk pengembangan lokal sehingga
// `go run ./cmd/api` bisa langsung jalan tanpa file .env.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config adalah konfigurasi aplikasi yang sudah ter-parse dan tervalidasi.
type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	DB        DBConfig
	Redis     RedisConfig
	Auth      AuthConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
	Business  BusinessConfig
	Seed      SeedConfig
	AI        AIConfig
}

// AIConfig mengatur mentor AI ("MinSmart") pada ulasan hasil tryout.
type AIConfig struct {
	Enabled bool
	// Provider: "ark" | "openai" | "stub".
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	Timeout     time.Duration
	Temperature float64
	MaxTokens   int
	// FallbackEnabled memakai mentor heuristik lokal bila provider utama gagal
	// (kuota habis, model belum diaktifkan, jaringan bermasalah).
	FallbackEnabled bool
	// DailyQuota membatasi jumlah permintaan AI per peserta per hari (0 = tanpa batas).
	DailyQuota int
	// MaxContextQuestions membatasi jumlah soal yang dikirim sebagai konteks.
	MaxContextQuestions int
	// MaxHistoryMessages membatasi riwayat percakapan yang dikirim ke model.
	MaxHistoryMessages int
	StreamEnabled      bool
	CacheTTL           time.Duration
}

// AppConfig berisi identitas aplikasi.
type AppConfig struct {
	Env  string
	Name string
	// MediaDir adalah direktori berkas media (video & e-book) yang disajikan
	// backend pada /api/v1/media/*.
	MediaDir string
}

// HTTPConfig berisi pengaturan server HTTP.
type HTTPConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
}

// DBConfig berisi pengaturan PostgreSQL & connection pool.
type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	TimeZone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PrepareStmt     bool
	AutoMigrate     bool
}

// DSN membentuk connection string PostgreSQL (format keyword/value).
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode, c.TimeZone,
	)
}

// URL membentuk connection string PostgreSQL (format URL) untuk driver pgx.
func (c DBConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&TimeZone=%s",
		url.QueryEscape(c.User), url.QueryEscape(c.Password),
		c.Host, c.Port, c.Name, c.SSLMode, url.QueryEscape(c.TimeZone),
	)
}

// RedisConfig berisi pengaturan cache & rate limit.
type RedisConfig struct {
	Enabled         bool
	Addr            string
	Password        string
	DB              int
	PoolSize        int
	CatalogTTL      time.Duration
	QuestionBankTTL time.Duration
}

// AuthConfig berisi pengaturan JWT.
type AuthConfig struct {
	JWTSecret  string
	JWTIssuer  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// CORSConfig berisi daftar origin yang diizinkan.
type CORSConfig struct {
	AllowedOrigins []string
}

// RateLimitConfig berisi batas permintaan per klien.
type RateLimitConfig struct {
	RPS   float64
	Burst int
}

// BusinessConfig berisi aturan bisnis yang bisa diubah lewat env.
type BusinessConfig struct {
	ReferralDiscountPercent int
	PassingThreshold        int
	MidtransServerKey       string
	MidtransClientKey       string
	MidtransIsProduction    bool
	// MidtransSandboxSimulator mengaktifkan endpoint simulasi pembayaran lokal
	// (meniru notifikasi Midtrans) agar alur checkout bisa diuji tanpa akun
	// Midtrans maupun URL publik. Otomatis nonaktif di produksi.
	MidtransSandboxSimulator bool
	// MidtransTimeout membatasi durasi permintaan ke API Midtrans.
	MidtransTimeout time.Duration
	// MidtransSnapBaseURL & MidtransCoreBaseURL mengarahkan permintaan ke domain
	// lain (mis. mock server saat pengujian). Kosong = domain resmi Midtrans.
	MidtransSnapBaseURL string
	MidtransCoreBaseURL string
	// FrontendBaseURL dipakai untuk URL pengalihan Snap (finish/unfinish/error).
	FrontendBaseURL string
}

// MidtransEnabled menandakan kredensial Midtrans sudah tersedia.
func (c *Config) MidtransEnabled() bool {
	return strings.TrimSpace(c.Business.MidtransServerKey) != ""
}

// SandboxSimulatorAllowed menandakan endpoint simulator pembayaran boleh dipakai.
//
// Dua pengaman: (1) harus diaktifkan eksplisit lewat MIDTRANS_SANDBOX_SIMULATOR,
// dan (2) otomatis nonaktif bila kredensial Midtrans produksi dipakai
// (MIDTRANS_IS_PRODUCTION=true), karena pada mode itu pembayaran harus nyata.
func (c *Config) SandboxSimulatorAllowed() bool {
	if c.Business.MidtransIsProduction {
		return false
	}
	return c.Business.MidtransSandboxSimulator
}

// SeedConfig mengatur penyisipan data demo.
type SeedConfig struct {
	OnStart bool
}

// IsProduction menandakan aplikasi berjalan di mode produksi.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.App.Env, "production")
}

// IsDevelopment menandakan aplikasi berjalan di mode pengembangan.
func (c *Config) IsDevelopment() bool {
	return !c.IsProduction()
}

// Load membaca .env (bila ada) lalu membentuk Config dari environment.
func Load() (*Config, error) {
	// .env bersifat opsional: abaikan bila tidak ditemukan.
	_ = godotenv.Load(".env", ".env.local")

	cfg := &Config{
		App: AppConfig{
			Env:      getEnv("APP_ENV", "development"),
			Name:     getEnv("APP_NAME", "be-bumn-academy"),
			MediaDir: getEnv("MEDIA_DIR", "./media"),
		},
		HTTP: HTTPConfig{
			Port:            getEnv("HTTP_PORT", "8080"),
			ReadTimeout:     getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getDuration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
			RequestTimeout:  getDuration("REQUEST_TIMEOUT", 20*time.Second),
		},
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "bumn"),
			Password:        getEnv("DB_PASSWORD", "bumn"),
			Name:            getEnv("DB_NAME", "bumn_academy"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			TimeZone:        getEnv("DB_TIMEZONE", "Asia/Jakarta"),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 60),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 30),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),
			PrepareStmt:     getBool("DB_PREPARE_STMT", true),
			AutoMigrate:     getBool("DB_AUTO_MIGRATE", true),
		},
		Redis: RedisConfig{
			Enabled:         getBool("REDIS_ENABLED", true),
			Addr:            getEnv("REDIS_ADDR", "localhost:6379"),
			Password:        getEnv("REDIS_PASSWORD", ""),
			DB:              getInt("REDIS_DB", 0),
			PoolSize:        getInt("REDIS_POOL_SIZE", 50),
			CatalogTTL:      getDuration("CACHE_CATALOG_TTL", 5*time.Minute),
			QuestionBankTTL: getDuration("CACHE_QUESTION_BANK_TTL", 15*time.Minute),
		},
		Auth: AuthConfig{
			JWTSecret:  getEnv("JWT_SECRET", "dev-secret-jangan-dipakai-di-produksi-32"),
			JWTIssuer:  getEnv("JWT_ISSUER", "be-bumn-academy"),
			AccessTTL:  getDuration("JWT_ACCESS_TTL", 24*time.Hour),
			RefreshTTL: getDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
		},
		CORS: CORSConfig{
			AllowedOrigins: getSlice("CORS_ALLOWED_ORIGINS", []string{
				"http://localhost:3000", "http://127.0.0.1:3000",
			}),
		},
		RateLimit: RateLimitConfig{
			RPS:   getFloat("RATE_LIMIT_RPS", 30),
			Burst: getInt("RATE_LIMIT_BURST", 60),
		},
		Business: BusinessConfig{
			ReferralDiscountPercent:  getInt("REFERRAL_DISCOUNT_PERCENT", 10),
			PassingThreshold:         getInt("PASSING_THRESHOLD", 70),
			MidtransServerKey:        getEnv("MIDTRANS_SERVER_KEY", ""),
			MidtransClientKey:        getEnv("MIDTRANS_CLIENT_KEY", ""),
			MidtransIsProduction:     getBool("MIDTRANS_IS_PRODUCTION", false),
			MidtransSandboxSimulator: getBool("MIDTRANS_SANDBOX_SIMULATOR", true),
			MidtransTimeout:          getDuration("MIDTRANS_TIMEOUT", 20*time.Second),
			MidtransSnapBaseURL:      getEnv("MIDTRANS_SNAP_BASE_URL", ""),
			MidtransCoreBaseURL:      getEnv("MIDTRANS_CORE_BASE_URL", ""),
			FrontendBaseURL:          strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://localhost:3000"), "/"),
		},
		Seed: SeedConfig{
			OnStart: getBool("SEED_ON_START", true),
		},
		AI: AIConfig{
			Enabled:             getBool("AI_ENABLED", true),
			Provider:            strings.ToLower(getEnv("AI_PROVIDER", "stub")),
			BaseURL:             getEnv("AI_BASE_URL", "https://ark.ap-southeast.bytepluses.com/api/v3"),
			APIKey:              getEnv("AI_API_KEY", ""),
			Model:               getEnv("AI_MODEL", "deepseek-v4-flash-260425"),
			Timeout:             getDuration("AI_TIMEOUT", 45*time.Second),
			Temperature:         getFloat("AI_TEMPERATURE", 0.3),
			MaxTokens:           getInt("AI_MAX_TOKENS", 1200),
			FallbackEnabled:     getBool("AI_FALLBACK_ENABLED", true),
			DailyQuota:          getInt("AI_DAILY_QUOTA", 120),
			MaxContextQuestions: getInt("AI_MAX_CONTEXT_QUESTIONS", 40),
			MaxHistoryMessages:  getInt("AI_MAX_HISTORY_MESSAGES", 12),
			StreamEnabled:       getBool("AI_STREAM_ENABLED", true),
			CacheTTL:            getDuration("AI_ANALYSIS_CACHE_TTL", 168*time.Hour),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.HTTP.Port == "" {
		return fmt.Errorf("config: HTTP_PORT wajib diisi")
	}
	if c.DB.Host == "" || c.DB.Name == "" {
		return fmt.Errorf("config: DB_HOST dan DB_NAME wajib diisi")
	}
	if c.IsProduction() && len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("config: JWT_SECRET minimal 32 karakter di produksi")
	}
	if c.DB.MaxIdleConns > c.DB.MaxOpenConns {
		c.DB.MaxIdleConns = c.DB.MaxOpenConns
	}
	if c.Business.PassingThreshold < 0 || c.Business.PassingThreshold > 100 {
		return fmt.Errorf("config: PASSING_THRESHOLD harus di antara 0-100")
	}
	return nil
}

/* ----------------------------- helpers env ----------------------------- */

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getInt(key string, fallback int) int {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getFloat(key string, fallback float64) float64 {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

func getBool(key string, fallback bool) bool {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getSlice(key string, fallback []string) []string {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
