// Package auth menangani JWT, identitas pengguna pada context, dan token refresh.
//
// Access token bersifat stateless (JWT) sehingga API bisa dijalankan pada
// beberapa replika tanpa perlu session store bersama — penting saat ribuan
// peserta mengerjakan tryout serentak.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken menandai token tidak valid/kedaluwarsa.
var ErrInvalidToken = errors.New("auth: token tidak valid atau sudah kedaluwarsa")

// Claims adalah isi access token.
type Claims struct {
	UserID string      `json:"uid"`
	Email  string      `json:"email"`
	Name   string      `json:"name"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

// Principal adalah identitas pemanggil yang ditempelkan ke context request.
type Principal struct {
	UserID string
	Email  string
	Name   string
	Role   models.Role
}

// IsAdmin menandakan pemanggil adalah admin.
func (p Principal) IsAdmin() bool { return p.Role == models.RoleAdmin }

// Manager membuat dan memverifikasi token.
type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager membentuk Manager dari konfigurasi.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		secret:     []byte(cfg.Auth.JWTSecret),
		issuer:     cfg.Auth.JWTIssuer,
		accessTTL:  cfg.Auth.AccessTTL,
		refreshTTL: cfg.Auth.RefreshTTL,
	}
}

// AccessTTL mengembalikan masa berlaku access token.
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL mengembalikan masa berlaku refresh token.
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// GenerateAccessToken membuat JWT bertanda tangan HS256.
func (m *Manager) GenerateAccessToken(user models.User) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)

	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        models.NewID(),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: gagal menandatangani token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken memverifikasi token dan mengembalikan claims.
func (m *Manager) ParseAccessToken(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: metode tanda tangan tidak didukung: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// PrincipalFromClaims mengubah claims menjadi Principal.
func PrincipalFromClaims(claims *Claims) Principal {
	return Principal{
		UserID: claims.UserID,
		Email:  claims.Email,
		Name:   claims.Name,
		Role:   claims.Role,
	}
}

// NewRefreshToken membuat token refresh acak beserta hash-nya untuk disimpan.
//
// Token mentah hanya dikirim ke klien; database menyimpan hash SHA-256 sehingga
// bocornya isi tabel tidak langsung memberi akses.
func NewRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("auth: gagal membuat token acak: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken menghitung hash SHA-256 heksadesimal dari token.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
