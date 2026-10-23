package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/auth"
	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/password"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// RequestMeta berisi metadata klien untuk pencatatan sesi login.
type RequestMeta struct {
	UserAgent string
	IPAddress string
}

// AuthService menangani pendaftaran, login, refresh, dan profil.
type AuthService struct {
	users   *repository.UserRepository
	manager *auth.Manager
	cfg     *config.Config
}

// NewAuthService membentuk service autentikasi.
func NewAuthService(users *repository.UserRepository, manager *auth.Manager, cfg *config.Config) *AuthService {
	return &AuthService{users: users, manager: manager, cfg: cfg}
}

// Register membuat akun peserta baru dan langsung memberi token.
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest, meta RequestMeta) (*dto.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, apperror.Conflict("Email sudah terdaftar. Silakan masuk atau gunakan email lain.")
	} else if !isNotFound(err) {
		return nil, err
	}

	hashed, err := password.Hash(req.Password)
	if err != nil {
		return nil, apperror.Internal(err, "Gagal mengamankan kata sandi.")
	}

	program := strings.TrimSpace(req.Program)
	if program == "" {
		program = "Peserta"
	}

	referral, err := s.generateReferralCode(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Name:         strings.TrimSpace(req.Name),
		Email:        email,
		PasswordHash: hashed,
		Role:         models.RolePeserta,
		Program:      program,
		ReferralCode: &referral,
	}

	if err := s.users.Create(ctx, &user); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, apperror.Conflict("Email sudah terdaftar.")
		}
		return nil, err
	}

	return s.issueTokens(ctx, user, meta)
}

// Login memverifikasi kredensial dan mengeluarkan token.
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, meta RequestMeta) (*dto.AuthResponse, error) {
	user, err := s.users.FindByEmail(ctx, req.Email)
	if err != nil {
		if isNotFound(err) {
			// Pesan disamakan agar tidak membocorkan email mana yang terdaftar.
			return nil, apperror.Unauthorized("Email atau kata sandi tidak cocok.")
		}
		return nil, err
	}

	if !password.Verify(user.PasswordHash, req.Password) {
		return nil, apperror.Unauthorized("Email atau kata sandi tidak cocok.")
	}

	return s.issueTokens(ctx, *user, meta)
}

// Refresh menukar refresh token lama dengan pasangan token baru (rotasi).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, meta RequestMeta) (*dto.AuthResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, apperror.Unauthorized("Refresh token wajib disertakan.")
	}

	stored, err := s.users.FindRefreshToken(ctx, auth.HashToken(refreshToken))
	if err != nil {
		return nil, err
	}
	if stored.RevokedAt != nil {
		return nil, apperror.Unauthorized("Sesi login sudah dicabut. Silakan masuk kembali.")
	}
	if time.Now().UTC().After(stored.ExpiresAt) {
		return nil, apperror.Unauthorized("Sesi login sudah kedaluwarsa. Silakan masuk kembali.")
	}

	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}

	// Rotasi: token lama dicabut supaya tidak bisa dipakai ulang.
	if err := s.users.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, *user, meta)
}

// Logout mencabut refresh token yang dipakai.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	stored, err := s.users.FindRefreshToken(ctx, auth.HashToken(refreshToken))
	if err != nil {
		// Token sudah tidak ada → anggap logout berhasil (idempotent).
		return nil
	}
	return s.users.RevokeRefreshToken(ctx, stored.ID)
}

// Profile mengambil data pengguna yang sedang masuk.
func (s *AuthService) Profile(ctx context.Context, userID string) (*dto.UserView, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := toUserView(*user)
	return &view, nil
}

// UpdateProfile memperbarui nama dan program peserta.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req dto.UpdateProfileRequest) (*dto.UserView, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.Name = strings.TrimSpace(req.Name)
	if strings.TrimSpace(req.Program) != "" {
		user.Program = strings.TrimSpace(req.Program)
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	view := toUserView(*user)
	return &view, nil
}

// issueTokens membuat access token + refresh token baru dan menyimpannya.
func (s *AuthService) issueTokens(ctx context.Context, user models.User, meta RequestMeta) (*dto.AuthResponse, error) {
	accessToken, expiresAt, err := s.manager.GenerateAccessToken(user)
	if err != nil {
		return nil, apperror.Internal(err, "Gagal membuat token akses.")
	}

	rawRefresh, hashRefresh, err := auth.NewRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err, "Gagal membuat token sesi.")
	}

	record := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		UserAgent: truncate(meta.UserAgent, 255),
		IPAddress: truncate(meta.IPAddress, 64),
		ExpiresAt: time.Now().UTC().Add(s.manager.RefreshTTL()),
	}
	if err := s.users.CreateRefreshToken(ctx, &record); err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    formatTime(expiresAt),
		User:         toUserView(user),
	}, nil
}

// generateReferralCode membentuk kode referral unik dari nama pengguna.
func (s *AuthService) generateReferralCode(ctx context.Context, name string) (string, error) {
	base := referralBase(name)
	if base == "" {
		base = "BUMN"
	}

	for attempt := 0; attempt < 8; attempt++ {
		code := fmt.Sprintf("%s%d", base, randomDigits(2))
		if _, err := s.users.FindByReferralCode(ctx, code); err != nil {
			if isNotFound(err) {
				return code, nil
			}
			return "", err
		}
	}
	// Fallback: kode acak penuh agar pendaftaran tidak pernah gagal.
	return fmt.Sprintf("BA%d", randomDigits(6)), nil
}

func referralBase(name string) string {
	var builder strings.Builder
	for _, char := range strings.ToUpper(name) {
		if char >= 'A' && char <= 'Z' {
			builder.WriteRune(char)
		}
		if builder.Len() == 5 {
			break
		}
	}
	return builder.String()
}

func randomDigits(n int) int {
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	value, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return int(time.Now().UnixNano() % 100)
	}
	return int(value.Int64())
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

// isNotFound memeriksa apakah error berasal dari apperror.NotFound.
func isNotFound(err error) bool {
	var appErr *apperror.Error
	return errors.As(err, &appErr) && appErr.Code == apperror.CodeNotFound
}
