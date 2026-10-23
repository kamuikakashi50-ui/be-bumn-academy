package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

const testSecret = "rahasia-pengujian-minimal-32-karakter-ok"

func testManager() *auth.Manager {
	return auth.NewManager(&config.Config{
		Auth: config.AuthConfig{
			JWTSecret:  testSecret,
			JWTIssuer:  "be-bumn-academy-test",
			AccessTTL:  time.Hour,
			RefreshTTL: 24 * time.Hour,
		},
	})
}

func newTestAuth() *Auth { return NewAuth(testManager()) }

func TestRequiredTanpaTokenMengembalikan401(t *testing.T) {
	mw := newTestAuth()
	handler := mw.Required(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler tidak boleh dipanggil tanpa token")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, harap 401", rec.Code)
	}
}

func TestRequiredTokenTidakValidMengembalikan401(t *testing.T) {
	mw := newTestAuth()
	handler := mw.Required(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler tidak boleh dipanggil dengan token tidak valid")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer token-palsu")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, harap 401", rec.Code)
	}
}

func TestRequiredDenganTokenValid(t *testing.T) {
	mw := newTestAuth()
	user := models.User{Base: models.Base{ID: "u-1"}, Name: "Ilham", Email: "a@b.c", Role: models.RolePeserta}
	token, _, err := testManager().GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("gagal membuat token: %v", err)
	}

	var gotUserID string
	handler := mw.Required(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, harap 200", rec.Code)
	}
	if gotUserID != "u-1" {
		t.Fatalf("user id pada context = %q, harap u-1", gotUserID)
	}
}

func TestOptionalTanpaTokenLanjutSebagaiTamu(t *testing.T) {
	mw := newTestAuth()
	called := false
	handler := mw.Optional(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if UserID(r.Context()) != "" {
			t.Error("tamu tidak boleh punya user id")
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tryouts", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("called=%v status=%d, harap true/200", called, rec.Code)
	}
}

func TestOptionalTokenTidakValidDitolak(t *testing.T) {
	mw := newTestAuth()
	handler := mw.Optional(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler tidak boleh dipanggil")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tryouts", nil)
	req.Header.Set("Authorization", "Bearer token-palsu")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, harap 401", rec.Code)
	}
}

func TestAdminMenolakPeserta(t *testing.T) {
	mw := newTestAuth()
	handler := mw.Admin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("peserta tidak boleh mengakses endpoint admin")
	}))

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "u-1", Role: models.RolePeserta})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, harap 403", rec.Code)
	}
}

func TestAdminMengizinkanAdmin(t *testing.T) {
	mw := newTestAuth()
	called := false
	handler := mw.Admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if !IsAdmin(r.Context()) {
			t.Error("IsAdmin harus true untuk admin")
		}
	}))

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "u-2", Role: models.RoleAdmin})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler harus dipanggil untuk admin")
	}
}

func TestAdminTanpaPrincipalDitolak(t *testing.T) {
	mw := newTestAuth()
	handler := mw.Admin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler tidak boleh dipanggil")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, harap 401", rec.Code)
	}
}

func TestAuthErrorMemetakanTokenHilang(t *testing.T) {
	got := authError(errMissingToken{})
	appErr, ok := got.(*apperror.Error)
	if !ok || appErr.Status != http.StatusUnauthorized {
		t.Fatalf("authError = %#v", got)
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"},
		{"Bearer   abc  ", "abc"},
		{"abc", ""},
		{"", ""},
		{"Basic abc", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		if got := bearerToken(req); got != tc.want {
			t.Errorf("bearerToken(%q) = %q, harap %q", tc.header, got, tc.want)
		}
	}
}

func TestClientKeyMemakaiUserIDBilaAda(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	if got := clientKey(req); got != "ip:10.0.0.1:1234" {
		t.Errorf("clientKey tamu = %q", got)
	}

	ctx := auth.WithPrincipal(req.Context(), auth.Principal{UserID: "u-9"})
	if got := clientKey(req.WithContext(ctx)); got != "user:u-9" {
		t.Errorf("clientKey login = %q", got)
	}
}

func TestClientKeyMenghormatiXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")

	if got := clientKey(req); got != "ip:203.0.113.7" {
		t.Errorf("clientKey = %q", got)
	}
}
