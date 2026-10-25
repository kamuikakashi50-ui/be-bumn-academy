package service

import (
	"errors"
	"log/slog"

	"github.com/bumn-academy/be-bumn-academy/internal/ai"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

// Berkas ini memuat helper yang dipakai bersama mentor AI (ulasan hasil tryout)
// dan asisten HR supaya penanganan kegagalan penyedia model tetap seragam.
// `label` adalah nama fitur yang tampil ke pengguna, mis. "Mentor AI".

// providerError mengubah kegagalan penyedia menjadi error aplikasi yang jelas.
func providerError(logger *slog.Logger, label string, err error) error {
	var upstream *ai.UpstreamError
	if !errors.As(err, &upstream) {
		return apperror.Internal(err, label+" sedang tidak bisa dihubungi. Coba lagi sebentar.")
	}

	if upstream.IsConfiguration() {
		logger.Error("konfigurasi penyedia AI bermasalah",
			"fitur", label, "provider", upstream.Provider, "code", upstream.Code, "status", upstream.StatusCode)
		return apperror.New(apperror.CodeInternal, 503,
			label+" belum siap dipakai (konfigurasi model). Hubungi tim akademik.")
	}
	if upstream.IsRetryable() {
		return apperror.New(apperror.CodeInternal, 503, label+" sedang sibuk. Coba lagi beberapa saat.")
	}
	return apperror.Internal(err, label+" gagal menjawab. Coba lagi sebentar.")
}

// isDegradedProvider menandakan provider sedang memakai fallback heuristik lokal.
func isDegradedProvider(provider ai.Provider) bool {
	if fallback, ok := provider.(*ai.FallbackProvider); ok {
		return fallback.Degraded()
	}
	return false
}

// streamErrorMessage mengambil pesan ramah dari error aplikasi agar klien SSE
// tetap menerima kalimat yang bisa dibaca.
func streamErrorMessage(appErr error, label string) string {
	if typed, ok := appErr.(*apperror.Error); ok {
		return typed.Message
	}
	return label + " gagal menjawab. Coba lagi sebentar."
}
