// Package validator membungkus go-playground/validator agar pesan kesalahan
// bisa dikembalikan sebagai map field → pesan yang ramah untuk front-end.
package validator

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	instance *validator.Validate
)

// V mengembalikan instance validator (dibuat sekali, aman untuk konkurensi).
func V() *validator.Validate {
	once.Do(func() {
		instance = validator.New(validator.WithRequiredStructEnabled())
		// Pakai nama field JSON agar pesan cocok dengan kontrak API.
		instance.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
			if name == "" || name == "-" {
				return field.Name
			}
			return name
		})
	})
	return instance
}

// FieldErrors adalah daftar kesalahan per field.
type FieldErrors map[string]string

// Error memenuhi interface error.
func (f FieldErrors) Error() string {
	parts := make([]string, 0, len(f))
	for field, message := range f {
		parts = append(parts, fmt.Sprintf("%s: %s", field, message))
	}
	return strings.Join(parts, "; ")
}

// Validate memvalidasi struct dan mengembalikan FieldErrors bila ada yang gagal.
func Validate(payload any) FieldErrors {
	err := V().Struct(payload)
	if err == nil {
		return nil
	}

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return FieldErrors{"_": err.Error()}
	}

	out := make(FieldErrors, len(validationErrs))
	for _, fieldErr := range validationErrs {
		out[fieldErr.Field()] = messageFor(fieldErr)
	}
	return out
}

// messageFor menerjemahkan tag validasi menjadi pesan bahasa Indonesia.
func messageFor(fieldErr validator.FieldError) string {
	field := fieldErr.Field()
	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s wajib diisi.", field)
	case "email":
		return fmt.Sprintf("%s harus berupa alamat email yang valid.", field)
	case "min":
		return fmt.Sprintf("%s minimal %s karakter.", field, fieldErr.Param())
	case "max":
		return fmt.Sprintf("%s maksimal %s karakter.", field, fieldErr.Param())
	case "len":
		return fmt.Sprintf("%s harus %s karakter.", field, fieldErr.Param())
	case "gte":
		return fmt.Sprintf("%s minimal %s.", field, fieldErr.Param())
	case "lte":
		return fmt.Sprintf("%s maksimal %s.", field, fieldErr.Param())
	case "gt":
		return fmt.Sprintf("%s harus lebih besar dari %s.", field, fieldErr.Param())
	case "oneof":
		return fmt.Sprintf("%s harus salah satu dari: %s.", field, strings.ReplaceAll(fieldErr.Param(), " ", ", "))
	case "url":
		return fmt.Sprintf("%s harus berupa URL yang valid.", field)
	case "uuid":
		return fmt.Sprintf("%s harus berupa UUID yang valid.", field)
	default:
		return fmt.Sprintf("%s tidak valid.", field)
	}
}
