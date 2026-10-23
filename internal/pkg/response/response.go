// Package response menyeragamkan bentuk balasan JSON API.
//
// Semua endpoint mengembalikan envelope berikut agar front-end cukup menulis
// satu penangan error:
//
//	sukses  → {"success": true,  "data": <payload>}
//	gagal   → {"success": false, "error": {"code": "...", "message": "...", "details": ...}}
package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

// Envelope adalah bentuk balasan standar.
type Envelope struct {
	Success bool          `json:"success"`
	Data    any           `json:"data,omitempty"`
	Error   *ErrorPayload `json:"error,omitempty"`
	Meta    *Meta         `json:"meta,omitempty"`
}

// ErrorPayload adalah detail error pada balasan.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Meta berisi metadata paginasi.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"perPage"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// JSON mengirim balasan dengan status tertentu.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("gagal menulis balasan JSON", "error", err)
	}
}

// OK mengirim data sukses (HTTP 200).
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Envelope{Success: true, Data: data})
}

// Created mengirim data sukses (HTTP 201).
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, Envelope{Success: true, Data: data})
}

// NoContent mengirim balasan tanpa isi (HTTP 204).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Paginated mengirim data sukses beserta metadata paginasi.
func Paginated(w http.ResponseWriter, data any, meta Meta) {
	if meta.TotalPages == 0 && meta.PerPage > 0 {
		meta.TotalPages = int((meta.Total + int64(meta.PerPage) - 1) / int64(meta.PerPage))
	}
	JSON(w, http.StatusOK, Envelope{Success: true, Data: data, Meta: &meta})
}

// Fail mengirim balasan error berdasarkan error aplikasi.
func Fail(w http.ResponseWriter, err error) {
	appErr := apperror.From(err)
	payload := Envelope{
		Success: false,
		Error: &ErrorPayload{
			Code:    string(appErr.Code),
			Message: appErr.Message,
			Details: appErr.Details,
		},
	}
	JSON(w, appErr.Status, payload)
}
