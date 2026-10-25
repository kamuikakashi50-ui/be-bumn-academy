package handler

import (
	"fmt"
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
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

// Chat menangani POST /admin/hr/ai/chat.
//
// Asisten HR menjawab pertanyaan tim HR dari analitik yang sudah terintegrasi di
// sistem. Riwayat percakapan dikirim klien karena tidak disimpan di server.
func (h *HRHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req dto.HRChatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.HRChat.Chat(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// Usage menangani GET /admin/hr/ai/usage.
func (h *HRHandler) Usage(w http.ResponseWriter, r *http.Request) {
	usage, err := h.deps.HRChat.Usage(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, usage)
}

// ChatStream menangani POST /admin/hr/ai/chat/stream (Server-Sent Events).
//
// Urutan kejadian: banyak `delta` → `done` (balasan lengkap + kuota + saran) atau
// `error`. Tidak ada kejadian `user` karena asisten HR tidak menyimpan pesan.
func (h *HRHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	var req dto.HRChatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Fail(w, fmt.Errorf("service: streaming tidak didukung oleh server ini"))
		return
	}

	stream, err := h.deps.HRChat.StreamChat(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-cache, no-transform")
	headers.Set("Connection", "keep-alive")
	// Mencegah reverse proxy menahan balasan agar efek pengetikan terasa.
	headers.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	for {
		select {
		case <-r.Context().Done():
			// Klien menutup koneksi; pencatatan kuota tetap diselesaikan service.
			return
		case event, ok := <-stream.Events:
			if !ok {
				return
			}

			switch {
			case event.Error != "":
				writeSSE(w, "error", event)
			case event.Done:
				writeSSE(w, "done", event)
			default:
				writeSSE(w, "delta", event)
			}
			flusher.Flush()

			if event.Done {
				return
			}
		}
	}
}
