package ai

import (
	"log/slog"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
)

// ProviderNameStub adalah nama provider heuristik lokal.
const ProviderNameStub = "stub"

// New membentuk provider sesuai konfigurasi.
//
// Provider `stub` selalu tersedia sehingga fitur ulasan hasil tryout tidak
// pernah mati total. Ketika provider jaringan dikonfigurasi, provider tersebut
// dibungkus fallback ke `stub` (kecuali fallback dimatikan) agar gangguan
// penyedia tidak membuat halaman hasil tryout rusak.
func New(cfg *config.Config, logger *slog.Logger) Provider {
	if logger == nil {
		logger = slog.Default()
	}

	name := strings.ToLower(strings.TrimSpace(cfg.AI.Provider))
	stub := NewStub()

	if name == "" || name == ProviderNameStub {
		return stub
	}

	// Provider jaringan tanpa API key tidak akan pernah berhasil; langsung
	// pakai mentor lokal agar tidak membuang waktu percobaan koneksi.
	if strings.TrimSpace(cfg.AI.APIKey) == "" {
		logger.Warn("AI_API_KEY kosong — mentor AI memakai mesin heuristik lokal",
			"provider_diminta", name)
		return stub
	}

	primary := NewOpenAI(OpenAIOptions{
		Name:        name,
		BaseURL:     cfg.AI.BaseURL,
		APIKey:      cfg.AI.APIKey,
		Model:       cfg.AI.Model,
		Temperature: cfg.AI.Temperature,
		MaxTokens:   cfg.AI.MaxTokens,
		Timeout:     cfg.AI.Timeout,
	})

	logger.Info("mentor AI aktif",
		"provider", name,
		"model", cfg.AI.Model,
		"base_url", cfg.AI.BaseURL,
		"fallback_lokal", cfg.AI.FallbackEnabled,
	)

	if !cfg.AI.FallbackEnabled {
		return primary
	}
	return NewFallback(primary, stub, logger)
}

// Status merangkum keadaan provider untuk endpoint kesehatan.
func Status(provider Provider) map[string]any {
	info := map[string]any{
		"provider": provider.Name(),
		"model":    provider.Model(),
	}

	if fallback, ok := provider.(*FallbackProvider); ok {
		info["degraded"] = fallback.Degraded()
		info["fallbackProvider"] = fallback.Fallback().Name()
		if message := fallback.LastError(); message != "" {
			info["lastError"] = message
		}
	}
	return info
}
