package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Role adalah peran pengguna.
type Role string

const (
	// RolePeserta adalah akun peserta.
	RolePeserta Role = "peserta"
	// RoleAdmin adalah akun tim akademik.
	RoleAdmin Role = "admin"
)

// Valid memeriksa apakah role dikenal.
func (r Role) Valid() bool { return r == RolePeserta || r == RoleAdmin }

// User adalah akun pengguna (peserta maupun admin).
type User struct {
	Base
	Name         string  `gorm:"column:name;not null" json:"name"`
	Email        string  `gorm:"column:email;not null" json:"email"`
	PasswordHash string  `gorm:"column:password_hash;not null" json:"-"`
	Role         Role    `gorm:"column:role;not null;default:peserta" json:"role"`
	Program      string  `gorm:"column:program;not null;default:Peserta" json:"program"`
	ReferralCode *string `gorm:"column:referral_code" json:"referralCode,omitempty"`
}

// TableName menetapkan nama tabel.
func (User) TableName() string { return "users" }

// Initials menghasilkan inisial nama (2 huruf) seperti pada front-end.
func (u User) Initials() string {
	parts := strings.Fields(u.Name)
	out := make([]string, 0, 2)
	for _, part := range parts {
		if len(out) == 2 {
			break
		}
		if part == "" {
			continue
		}
		out = append(out, strings.ToUpper(string([]rune(part)[0])))
	}
	if len(out) == 0 {
		return strings.ToUpper(string([]rune(u.Email)[0:1]))
	}
	return strings.Join(out, "")
}

// IsAdmin menandakan akun memiliki hak akses admin.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// RefreshToken menyimpan hash refresh token (rotasi token saat refresh).
type RefreshToken struct {
	ID        string     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID    string     `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	TokenHash string     `gorm:"column:token_hash;not null" json:"-"`
	UserAgent string     `gorm:"column:user_agent" json:"userAgent,omitempty"`
	IPAddress string     `gorm:"column:ip_address" json:"ipAddress,omitempty"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	RevokedAt *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (RefreshToken) TableName() string { return "refresh_tokens" }

// BeforeCreate memastikan ID terisi.
func (t *RefreshToken) BeforeCreate(*gorm.DB) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	return nil
}
