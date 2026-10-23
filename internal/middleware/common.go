package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/go-chi/chi/v5/middleware"
)

// RequestLogger mencatat setiap permintaan dengan format terstruktur (slog).
//
// Level log menyesuaikan status: 5xx → error, 4xx → warn, sisanya → info,
// sehingga log produksi tetap mudah dipindai saat trafik tinggi.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(wrapped, r)

		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}

		attributes := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", wrapped.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
			"ip", r.RemoteAddr,
		}

		switch {
		case status >= 500:
			slog.Error("request", attributes...)
		case status >= 400:
			slog.Warn("request", attributes...)
		default:
			slog.Info("request", attributes...)
		}
	})
}

// RateLimit membatasi jumlah permintaan per klien.
//
// Kunci pembatas memakai id pengguna bila sudah login (agar satu kantor/NAT
// dengan banyak peserta tidak saling memblokir), dan IP sebagai fallback.
func RateLimit(limiter *cache.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientKey(r)
			if !limiter.Allow(r.Context(), key) {
				w.Header().Set("Retry-After", "1")
				response.Fail(w, apperror.New(apperror.CodeRateLimited, http.StatusTooManyRequests,
					"Terlalu banyak permintaan. Coba lagi sebentar."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientKey membentuk kunci pembatas dari identitas pengguna atau alamat IP.
func clientKey(r *http.Request) string {
	if userID := UserID(r.Context()); userID != "" {
		return "user:" + userID
	}
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ip = strings.TrimSpace(realIP)
	}
	return "ip:" + ip
}
