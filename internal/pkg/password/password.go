// Package password menangani hashing kata sandi dengan bcrypt.
package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MinLength adalah panjang minimum kata sandi yang diterima.
const MinLength = 8

// Hash mengubah kata sandi menjadi hash bcrypt.
func Hash(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("password: gagal hashing: %w", err)
	}
	return string(hashed), nil
}

// Verify mencocokkan kata sandi dengan hash tersimpan.
func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
