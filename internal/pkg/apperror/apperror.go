// Package apperror menyediakan error domain yang bisa dipetakan langsung ke
// status HTTP oleh handler.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code adalah kode error mesin (stabil, dipakai front-end untuk logika).
type Code string

// Daftar kode error yang dipakai aplikasi.
const (
	CodeBadRequest   Code = "BAD_REQUEST"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeLocked       Code = "LOCKED"
	CodeRateLimited  Code = "RATE_LIMITED"
	CodeInternal     Code = "INTERNAL_ERROR"
)

// Error adalah error aplikasi dengan kode + status HTTP.
type Error struct {
	Code    Code
	Status  int
	Message string
	// Details berisi informasi tambahan (mis. daftar field yang gagal validasi).
	Details any
	// Err adalah error asli untuk logging internal.
	Err error
}

// Error memenuhi interface error.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap memungkinkan errors.Is/As menembus ke error asli.
func (e *Error) Unwrap() error { return e.Err }

// New membentuk error aplikasi baru.
func New(code Code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Wrap membungkus error asli ke dalam error aplikasi.
func Wrap(err error, code Code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message, Err: err}
}

// WithDetails menambahkan detail (mis. field validasi) ke error.
func (e *Error) WithDetails(details any) *Error {
	e.Details = details
	return e
}

/* --------------------------- constructor singkat --------------------------- */

// BadRequest menandai permintaan tidak valid.
func BadRequest(message string) *Error {
	return New(CodeBadRequest, http.StatusBadRequest, message)
}

// Validation menandai kegagalan validasi input.
func Validation(message string, details any) *Error {
	return New(CodeValidation, http.StatusUnprocessableEntity, message).WithDetails(details)
}

// Unauthorized menandai pengguna belum terautentikasi.
func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, http.StatusUnauthorized, message)
}

// Forbidden menandai pengguna tidak punya hak akses.
func Forbidden(message string) *Error {
	return New(CodeForbidden, http.StatusForbidden, message)
}

// NotFound menandai data tidak ditemukan.
func NotFound(message string) *Error {
	return New(CodeNotFound, http.StatusNotFound, message)
}

// Conflict menandai konflik data (mis. email sudah dipakai).
func Conflict(message string) *Error {
	return New(CodeConflict, http.StatusConflict, message)
}

// Locked menandai sumber daya terkunci (mis. subtes sudah habis waktunya).
func Locked(message string) *Error {
	return New(CodeLocked, http.StatusLocked, message)
}

// Internal membungkus kesalahan tak terduga.
func Internal(err error, message string) *Error {
	if message == "" {
		message = "Terjadi kesalahan pada server."
	}
	return Wrap(err, CodeInternal, http.StatusInternalServerError, message)
}

// From mengambil *Error dari rantai error; bila bukan error aplikasi,
// dikembalikan error internal generik.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err, "")
}
