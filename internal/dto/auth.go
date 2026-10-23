package dto

// RegisterRequest adalah payload pendaftaran peserta.
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=3,max=120"`
	Email    string `json:"email" validate:"required,email,max=180"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Program  string `json:"program" validate:"omitempty,max=120"`
}

// LoginRequest adalah payload masuk.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest adalah payload perpanjangan token.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// UpdateProfileRequest adalah payload pembaruan profil.
type UpdateProfileRequest struct {
	Name    string `json:"name" validate:"required,min=3,max=120"`
	Program string `json:"program" validate:"omitempty,max=120"`
}

// AuthResponse adalah hasil autentikasi.
type AuthResponse struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	ExpiresAt    string   `json:"expiresAt"`
	User         UserView `json:"user"`
}

// UserView adalah representasi pengguna untuk klien (tanpa data sensitif).
type UserView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Program      string `json:"program"`
	Initials     string `json:"initials"`
	ReferralCode string `json:"referralCode,omitempty"`
}
