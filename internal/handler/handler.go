// Package handler menerjemahkan HTTP request menjadi pemanggilan service.
//
// Handler tidak memuat aturan bisnis: ia hanya memvalidasi bentuk input,
// memanggil service, lalu membentuk balasan JSON.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/validator"
	"github.com/bumn-academy/be-bumn-academy/internal/service"
	"github.com/go-chi/chi/v5"
)

// maxBodyBytes membatasi ukuran body JSON (mencegah payload raksasa).
const maxBodyBytes = 1 << 20 // 1 MiB

// Deps mengumpulkan seluruh service yang dipakai handler.
type Deps struct {
	Auth      *service.AuthService
	Catalog   *service.CatalogService
	Billing   *service.BillingService
	Tryout    *service.TryoutService
	Learning  *service.LearningService
	Classroom *service.ClassroomService
	Admin     *service.AdminService
	Mentor    *service.MentorService
}

// decodeJSON membaca body JSON dan memvalidasi struct tujuan.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return badJSON(err)
	}

	// Pastikan tidak ada isi tambahan setelah objek JSON.
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return apperror.BadRequest("Body request harus berisi satu objek JSON.")
	}

	if fieldErrors := validator.Validate(dst); fieldErrors != nil {
		return apperror.Validation("Data yang dikirim belum lengkap atau tidak valid.", fieldErrors)
	}
	return nil
}

// decodeJSONOptional membaca body JSON tanpa mewajibkan isi (untuk PUT/PATCH).
func decodeJSONOptional(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return badJSON(err)
	}

	if fieldErrors := validator.Validate(dst); fieldErrors != nil {
		return apperror.Validation("Data yang dikirim belum lengkap atau tidak valid.", fieldErrors)
	}
	return nil
}

func badJSON(err error) error {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return apperror.BadRequest("Format JSON tidak valid pada posisi " + strconv.FormatInt(syntaxErr.Offset, 10) + ".")
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return apperror.BadRequest("Tipe data field " + typeErr.Field + " tidak sesuai.")
	}
	if strings.Contains(err.Error(), "unknown field") {
		return apperror.BadRequest("Terdapat field yang tidak dikenali: " + err.Error())
	}
	return apperror.BadRequest("Body request tidak bisa dibaca.")
}

// pathParam mengambil parameter path chi.
func pathParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

// queryInt membaca query parameter bilangan bulat dengan nilai default.
func queryInt(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// queryBool membaca query parameter boolean.
func queryBool(r *http.Request, name string) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return value
}

// paginationFrom membaca dan menormalkan parameter paginasi.
func paginationFrom(r *http.Request) dto.Pagination {
	page := dto.Pagination{
		Page:    queryInt(r, "page", 1),
		PerPage: queryInt(r, "perPage", 20),
	}
	page.Normalize()
	return page
}
