package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// CatalogHandler melayani konten katalog publik.
type CatalogHandler struct {
	deps *Deps
}

// NewCatalogHandler membentuk handler katalog.
func NewCatalogHandler(deps *Deps) *CatalogHandler {
	return &CatalogHandler{deps: deps}
}

// Programs menangani GET /programs.
func (h *CatalogHandler) Programs(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Catalog.Programs(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// MateriCategories menangani GET /materi/categories.
func (h *CatalogHandler) MateriCategories(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Catalog.MateriCategories(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Products menangani GET /products.
func (h *CatalogHandler) Products(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Catalog.Products(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// PaymentMethods menangani GET /payment-methods.
func (h *CatalogHandler) PaymentMethods(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Catalog.PaymentMethods(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Vouchers menangani GET /vouchers (khusus admin).
func (h *CatalogHandler) Vouchers(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Catalog.Vouchers(r.Context(), queryBool(r, "includeInactive"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}
