package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// BillingHandler melayani checkout, pesanan, dan webhook pembayaran.
type BillingHandler struct {
	deps *Deps
}

// NewBillingHandler membentuk handler billing.
func NewBillingHandler(deps *Deps) *BillingHandler {
	return &BillingHandler{deps: deps}
}

// Quote menangani POST /checkout/quote.
func (h *BillingHandler) Quote(w http.ResponseWriter, r *http.Request) {
	var req dto.QuoteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	quote, err := h.deps.Billing.Quote(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, quote)
}

// CreateOrder menangani POST /orders.
func (h *BillingHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateOrderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	order, err := h.deps.Billing.CreateOrder(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, order)
}

// ListOrders menangani GET /orders.
func (h *BillingHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page := paginationFrom(r)
	orders, total, err := h.deps.Billing.ListOrders(r.Context(), middleware.UserID(r.Context()), page.Page, page.PerPage)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Paginated(w, orders, page.Meta(total))
}

// GetOrder menangani GET /orders/{id}.
func (h *BillingHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.deps.Billing.GetOrder(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, order)
}

// CancelOrder menangani POST /orders/{id}/cancel.
func (h *BillingHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.deps.Billing.CancelOrder(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, order)
}

// PayOrder menangani POST /orders/{id}/pay (simulasi pembayaran berhasil).
func (h *BillingHandler) PayOrder(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Billing.PayOrder(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// Enrollments menangani GET /enrollments.
func (h *BillingHandler) Enrollments(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Billing.ListEnrollments(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// MidtransWebhook menangani POST /webhooks/midtrans.
//
// Endpoint ini dipanggil server Midtrans (bukan browser), jadi tidak memakai
// autentikasi JWT. Signature notifikasi diverifikasi dengan Server Key sehingga
// permintaan palsu ditolak.
func (h *BillingHandler) MidtransWebhook(w http.ResponseWriter, r *http.Request) {
	var notif dto.MidtransNotification
	if err := decodeJSONOptional(w, r, &notif); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Billing.HandleWebhook(r.Context(), notif)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// MidtransConfig menangani GET /midtrans/config (publik).
//
// Dipakai front-end untuk memuat snap.js dan memutuskan apakah menampilkan
// tombol bayar Midtrans atau simulator sandbox lokal.
func (h *BillingHandler) MidtransConfig(w http.ResponseWriter, r *http.Request) {
	response.OK(w, h.deps.Billing.MidtransConfig())
}

// PaymentIntent menangani POST /orders/{id}/payment.
//
// Membuat sesi pembayaran Midtrans (Snap) untuk sebuah pesanan; `?refresh=true`
// memaksa pembuatan token baru.
func (h *BillingHandler) PaymentIntent(w http.ResponseWriter, r *http.Request) {
	refresh := queryBool(r, "refresh")
	intent, err := h.deps.Billing.PaymentIntent(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), refresh,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, intent)
}

// PaymentStatus menangani GET /orders/{id}/payment/status.
//
// Menyinkronkan status pesanan dengan Midtrans (rekonsiliasi bila notifikasi
// webhook belum sampai ke server lokal).
func (h *BillingHandler) PaymentStatus(w http.ResponseWriter, r *http.Request) {
	order, err := h.deps.Billing.SyncPaymentStatus(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, order)
}

// SandboxPayment menangani POST /orders/{id}/sandbox/settle.
//
// Simulator pembayaran lokal: mengirim notifikasi tiruan bertanda tangan ke
// jalur webhook yang sama sehingga alur checkout bisa diuji tanpa kredensial
// Midtrans maupun URL publik.
func (h *BillingHandler) SandboxPayment(w http.ResponseWriter, r *http.Request) {
	var req dto.SandboxPaymentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Billing.SandboxSimulate(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req.Outcome,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}
