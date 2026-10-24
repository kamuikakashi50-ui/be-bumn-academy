package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// CorporateHandler melayani insight agregat untuk halaman "Solusi Korporasi"
// (B2B). Seluruh datanya agregat — tidak ada identitas peserta yang dikirim.
type CorporateHandler struct {
	deps *Deps
}

// NewCorporateHandler membentuk handler insight korporasi.
func NewCorporateHandler(deps *Deps) *CorporateHandler {
	return &CorporateHandler{deps: deps}
}

// Insights menangani GET /insights/corporate.
func (h *CorporateHandler) Insights(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Corporate.Insights(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}
