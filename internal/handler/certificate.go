package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// CertificateHandler melayani sertifikat jalur belajar: milik peserta (daftar,
// PDF, penerbitan), verifikasi publik, dan pengelolaan admin.
type CertificateHandler struct {
	deps *Deps
}

// NewCertificateHandler membentuk handler sertifikat.
func NewCertificateHandler(deps *Deps) *CertificateHandler {
	return &CertificateHandler{deps: deps}
}

/* --------------------------------- peserta --------------------------------- */

// List menangani GET /certificates.
func (h *CertificateHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Certificate.ListMine(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Issue menangani POST /learning/paths/{slug}/certificate.
func (h *CertificateHandler) Issue(w http.ResponseWriter, r *http.Request) {
	item, err := h.deps.Certificate.Issue(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "slug"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, item)
}

// PDF menangani GET /certificates/{id}/pdf (peserta: hanya miliknya sendiri).
func (h *CertificateHandler) PDF(w http.ResponseWriter, r *http.Request) {
	file, err := h.deps.Certificate.PDF(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), false,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	writePDF(w, file.Filename, file.Data, queryBool(r, "download"))
}

// Verify menangani GET /verifikasi/{nomor} (publik, tanpa login).
func (h *CertificateHandler) Verify(w http.ResponseWriter, r *http.Request) {
	item, err := h.deps.Certificate.Verify(r.Context(), pathParam(r, "nomor"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

/* ---------------------------------- admin ---------------------------------- */

// AdminList menangani GET /admin/certificates.
func (h *CertificateHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	filter := repository.CertificateFilter{
		Search: r.URL.Query().Get("search"),
		PathID: r.URL.Query().Get("pathId"),
		Status: r.URL.Query().Get("status"),
	}

	items, err := h.deps.Certificate.AdminList(r.Context(), filter, queryInt(r, "limit", 200))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// AdminPDF menangani GET /admin/certificates/{id}/pdf (admin boleh semua).
func (h *CertificateHandler) AdminPDF(w http.ResponseWriter, r *http.Request) {
	file, err := h.deps.Certificate.PDF(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), true,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	writePDF(w, file.Filename, file.Data, queryBool(r, "download"))
}

// Revoke menangani DELETE /admin/certificates/{id}.
func (h *CertificateHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Certificate.Revoke(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// Restore menangani POST /admin/certificates/{id}/restore.
func (h *CertificateHandler) Restore(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Certificate.Restore(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// writePDF mengirim berkas PDF dengan header yang sama seperti e-book ruang kelas.
func writePDF(w http.ResponseWriter, filename string, data []byte, download bool) {
	disposition := "inline"
	if download {
		disposition = "attachment"
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`%s; filename="%s"; filename*=UTF-8''%s`,
		disposition, filename, url.PathEscape(filename),
	))
	// Sertifikat bersifat pribadi: jangan disimpan di cache bersama.
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
