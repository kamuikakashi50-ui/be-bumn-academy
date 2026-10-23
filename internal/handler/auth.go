package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
)

// AuthHandler menangani endpoint autentikasi.
type AuthHandler struct {
	deps *Deps
}

// NewAuthHandler membentuk handler autentikasi.
func NewAuthHandler(deps *Deps) *AuthHandler {
	return &AuthHandler{deps: deps}
}

// Register menangani POST /auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Auth.Register(r.Context(), req, requestMeta(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, result)
}

// Login menangani POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Auth.Login(r.Context(), req, requestMeta(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// Refresh menangani POST /auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Auth.Refresh(r.Context(), req.RefreshToken, requestMeta(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// Logout menangani POST /auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	// Body opsional: logout tetap berhasil walau refresh token tidak dikirim.
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	if err := h.deps.Auth.Logout(r.Context(), req.RefreshToken); err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, map[string]any{"loggedOut": true})
}

// Me menangani GET /auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.deps.Auth.Profile(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, user)
}

// UpdateProfile menangani PUT /auth/me.
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProfileRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	user, err := h.deps.Auth.UpdateProfile(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, user)
}

// requestMeta mengambil metadata klien untuk pencatatan sesi login.
func requestMeta(r *http.Request) service.RequestMeta {
	return service.RequestMeta{
		UserAgent: r.UserAgent(),
		IPAddress: clientIP(r),
	}
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}
