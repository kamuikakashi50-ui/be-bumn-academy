package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// HRHandler melayani analitik SDM (people analytics) panel admin.
//
// Seluruh route di sini berada di bawah grup /admin sehingga sudah dilindungi
// middleware Required + Admin: hanya akun berperan admin yang bisa membukanya.
type HRHandler struct {
	deps *Deps
}

// NewHRHandler membentuk handler analitik HR.
func NewHRHandler(deps *Deps) *HRHandler {
	return &HRHandler{deps: deps}
}

// Analytics menangani GET /admin/hr/analytics.
func (h *HRHandler) Analytics(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.HR.Analytics(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}
