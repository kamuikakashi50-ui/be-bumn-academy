// Package middleware berisi middleware HTTP: autentikasi, otorisasi, rate limit,
// logging, dan timeout.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// Auth memverifikasi JWT pada header Authorization.
type Auth struct {
	manager *auth.Manager
}

// NewAuth membentuk middleware autentikasi.
func NewAuth(manager *auth.Manager) *Auth {
	return &Auth{manager: manager}
}

// Required menolak permintaan tanpa token yang valid.
func (a *Auth) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := a.principalFrom(r)
		if err != nil {
			response.Fail(w, authError(err))
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
}

// Optional menempelkan identitas bila token valid, tetapi tetap melanjutkan
// permintaan bila token tidak ada (dipakai endpoint publik yang bisa
// dipersonalisasi, mis. katalog & daftar tryout).
func (a *Auth) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := a.principalFrom(r)
		if err != nil {
			// Token tidak disertakan → lanjut sebagai tamu.
			if isMissingToken(err) {
				next.ServeHTTP(w, r)
				return
			}
			// Token ada tetapi tidak valid → tolak agar klien tahu sesinya
			// sudah kedaluwarsa dan bisa melakukan refresh.
			response.Fail(w, authError(err))
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
}

// authError memetakan error autentikasi ke error aplikasi yang tepat.
func authError(err error) error {
	if isMissingToken(err) {
		return apperror.Unauthorized("Masuk terlebih dahulu untuk mengakses fitur ini.")
	}
	return err
}

// Admin hanya mengizinkan pengguna dengan peran admin.
func (a *Auth) Admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			response.Fail(w, apperror.Unauthorized("Masuk terlebih dahulu untuk mengakses fitur ini."))
			return
		}
		if !principal.IsAdmin() {
			response.Fail(w, apperror.Forbidden("Halaman ini hanya untuk tim akademik."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// principalFrom membaca dan memverifikasi token dari header Authorization.
func (a *Auth) principalFrom(r *http.Request) (auth.Principal, error) {
	raw := bearerToken(r)
	if raw == "" {
		return auth.Principal{}, errMissingToken{}
	}

	claims, err := a.manager.ParseAccessToken(raw)
	if err != nil {
		return auth.Principal{}, apperror.Unauthorized("Sesi tidak valid atau sudah kedaluwarsa. Silakan masuk kembali.")
	}
	return auth.PrincipalFromClaims(claims), nil
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

type errMissingToken struct{}

func (errMissingToken) Error() string { return "auth: token tidak disertakan" }

func isMissingToken(err error) bool {
	_, ok := err.(errMissingToken)
	return ok
}

// UserID mengambil id pengguna dari context (string kosong bila tamu).
func UserID(ctx context.Context) string {
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return ""
	}
	return principal.UserID
}

// IsAdmin menandakan pemanggil adalah admin.
func IsAdmin(ctx context.Context) bool {
	principal, ok := auth.PrincipalFromContext(ctx)
	return ok && principal.IsAdmin()
}
