package handler

import (
	"io"
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// maxUploadBytes membatasi ukuran berkas import soal (10 MiB).
const maxUploadBytes = 10 << 20

// AdminHandler melayani dashboard admin (manajemen konten, pengguna, pesanan,
// broadcast, dan analitik).
type AdminHandler struct {
	deps *Deps
}

// NewAdminHandler membentuk handler admin.
func NewAdminHandler(deps *Deps) *AdminHandler {
	return &AdminHandler{deps: deps}
}

/* -------------------------------- analitik -------------------------------- */

// Analytics menangani GET /admin/analytics.
func (h *AdminHandler) Analytics(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Admin.Analytics(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

/* --------------------------------- pengguna -------------------------------- */

// ListUsers menangani GET /admin/users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := paginationFrom(r)
	filter := repository.UserFilter{
		Search: r.URL.Query().Get("search"),
		Role:   r.URL.Query().Get("role"),
	}

	items, total, err := h.deps.Admin.ListUsers(r.Context(), filter, page.Page, page.PerPage)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Paginated(w, items, page.Meta(total))
}

/* --------------------------------- pesanan -------------------------------- */

// ListOrders menangani GET /admin/orders.
func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page := paginationFrom(r)
	filter := repository.OrderFilter{
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
	}

	items, total, err := h.deps.Admin.ListOrders(r.Context(), filter, page.Page, page.PerPage)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Paginated(w, items, page.Meta(total))
}

// MarkOrderPaid menangani POST /admin/orders/{id}/mark-paid.
func (h *AdminHandler) MarkOrderPaid(w http.ResponseWriter, r *http.Request) {
	order, err := h.deps.Admin.MarkOrderPaid(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, order)
}

/* ---------------------------------- soal ---------------------------------- */

// ListQuestions menangani GET /admin/questions.
func (h *AdminHandler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	page := paginationFrom(r)
	filter := repository.QuestionFilter{
		BankKey:    r.URL.Query().Get("bankKey"),
		Search:     r.URL.Query().Get("search"),
		Difficulty: r.URL.Query().Get("difficulty"),
		ActiveOnly: queryBool(r, "activeOnly"),
	}

	items, total, err := h.deps.Admin.ListQuestions(r.Context(), filter, page.Page, page.PerPage)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Paginated(w, items, page.Meta(total))
}

// CreateQuestion menangani POST /admin/questions.
func (h *AdminHandler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateQuestionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	question, err := h.deps.Admin.CreateQuestion(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, question)
}

// UpdateQuestion menangani PATCH /admin/questions/{id}.
func (h *AdminHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateQuestionRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	question, err := h.deps.Admin.UpdateQuestion(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, question)
}

// DeleteQuestion menangani DELETE /admin/questions/{id}.
func (h *AdminHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Admin.DeleteQuestion(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// ImportQuestions menangani POST /admin/questions/import (multipart/form-data).
func (h *AdminHandler) ImportQuestions(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Fail(w, apperror.BadRequest("Berkas import wajib dikirim pada field 'file'."))
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		response.Fail(w, apperror.BadRequest("Berkas import tidak bisa dibaca."))
		return
	}

	result, err := h.deps.Admin.ImportQuestions(r.Context(), header.Filename, data)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

/* ------------------------------ paket tryout ------------------------------ */

// CreateTryout menangani POST /admin/tryouts.
func (h *AdminHandler) CreateTryout(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTryoutRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	detail, err := h.deps.Admin.UpsertTryout(r.Context(), req, "")
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, detail)
}

// UpdateTryout menangani PATCH /admin/tryouts/{id}.
func (h *AdminHandler) UpdateTryout(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateTryoutRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	detail, err := h.deps.Admin.UpdateTryout(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// DeleteTryout menangani DELETE /admin/tryouts/{id}.
func (h *AdminHandler) DeleteTryout(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Admin.DeleteTryout(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* --------------------------------- produk --------------------------------- */

// CreateProduct menangani POST /admin/products.
func (h *AdminHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	product, err := h.deps.Admin.CreateProduct(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, product)
}

// UpdateProduct menangani PATCH /admin/products/{slug}.
func (h *AdminHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateProductRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	product, err := h.deps.Admin.UpdateProduct(r.Context(), pathParam(r, "slug"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, product)
}

// DeleteProduct menangani DELETE /admin/products/{slug}.
func (h *AdminHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Admin.DeleteProduct(r.Context(), pathParam(r, "slug")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* -------------------------------- voucher -------------------------------- */

// CreateVoucher menangani POST /admin/vouchers.
func (h *AdminHandler) CreateVoucher(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateVoucherRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	voucher, err := h.deps.Admin.CreateVoucher(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, voucher)
}

// UpdateVoucher menangani PATCH /admin/vouchers/{code}.
func (h *AdminHandler) UpdateVoucher(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateVoucherRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	voucher, err := h.deps.Admin.UpdateVoucher(r.Context(), pathParam(r, "code"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, voucher)
}

// DeleteVoucher menangani DELETE /admin/vouchers/{code}.
func (h *AdminHandler) DeleteVoucher(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Admin.DeleteVoucher(r.Context(), pathParam(r, "code")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* ------------------------------- broadcast -------------------------------- */

// Broadcast menangani POST /admin/broadcast.
func (h *AdminHandler) Broadcast(w http.ResponseWriter, r *http.Request) {
	var req dto.BroadcastRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Admin.Broadcast(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// ListBroadcasts menangani GET /admin/broadcasts.
func (h *AdminHandler) ListBroadcasts(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Admin.ListBroadcasts(r.Context(), queryInt(r, "limit", 20))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}
