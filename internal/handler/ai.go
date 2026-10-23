package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// AIHandler melayani mentor AI ("MinSmart") pada ulasan hasil tryout.
type AIHandler struct {
	deps *Deps
}

// NewAIHandler membentuk handler mentor AI.
func NewAIHandler(deps *Deps) *AIHandler {
	return &AIHandler{deps: deps}
}

// Analysis menangani GET /attempts/{id}/ai/analysis.
//
// Analisis dibuat sekali lalu dipakai ulang. Tambahkan `?force=true` untuk
// memaksa perhitungan ulang.
func (h *AIHandler) Analysis(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Mentor.Analysis(
		r.Context(),
		middleware.UserID(r.Context()),
		pathParam(r, "id"),
		queryBool(r, "force"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// Conversation menangani GET /attempts/{id}/ai/conversation.
func (h *AIHandler) Conversation(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Mentor.Conversation(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// Chat menangani POST /attempts/{id}/ai/chat.
func (h *AIHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req dto.AIChatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Mentor.Chat(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// ExplainQuestion menangani POST /attempts/{id}/ai/explain.
func (h *AIHandler) ExplainQuestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		QuestionID string `json:"questionId" validate:"required,max=64"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Mentor.ExplainQuestion(
		r.Context(),
		middleware.UserID(r.Context()),
		pathParam(r, "id"),
		req.QuestionID,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// ResetConversation menangani DELETE /attempts/{id}/ai/conversation.
func (h *AIHandler) ResetConversation(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Mentor.ResetConversation(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, map[string]any{"reset": true})
}

// Usage menangani GET /ai/usage.
func (h *AIHandler) Usage(w http.ResponseWriter, r *http.Request) {
	usage, err := h.deps.Mentor.Usage(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, usage)
}

// Status menangani GET /ai/status — berguna untuk memantau kesiapan provider.
func (h *AIHandler) Status(w http.ResponseWriter, r *http.Request) {
	response.OK(w, h.deps.Mentor.Status())
}

/* --------------------------------- streaming ------------------------------- */

// ChatStream menangani POST /attempts/{id}/ai/chat/stream (Server-Sent Events).
//
// Urutan kejadian: `user` (pesan peserta tersimpan) → banyak `delta` →
// `done` (pesan asisten lengkap + kuota) atau `error`.
func (h *AIHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	var req dto.AIChatRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Fail(w, fmt.Errorf("service: streaming tidak didukung oleh server ini"))
		return
	}

	stream, err := h.deps.Mentor.StreamChat(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
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

	writeSSE(w, "user", stream.UserMessage)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			// Klien menutup koneksi; balasan tetap disimpan oleh service.
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

// writeSSE menulis satu kejadian Server-Sent Events.
func writeSSE(w http.ResponseWriter, event string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		encoded = []byte(`{"error":"gagal menyusun balasan"}`)
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, encoded)
}
