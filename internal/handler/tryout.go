package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// TryoutHandler melayani mesin tryout: paket, sesi, autosave, hasil, dan
// papan peringkat.
type TryoutHandler struct {
	deps *Deps
}

// NewTryoutHandler membentuk handler tryout.
func NewTryoutHandler(deps *Deps) *TryoutHandler {
	return &TryoutHandler{deps: deps}
}

// List menangani GET /tryouts.
func (h *TryoutHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Tryout.List(r.Context(), middleware.UserID(r.Context()), middleware.IsAdmin(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Get menangani GET /tryouts/{id}.
func (h *TryoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.Tryout.Get(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), middleware.IsAdmin(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// StartSession menangani POST /tryouts/{id}/sessions.
func (h *TryoutHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.deps.Tryout.StartSession(
		r.Context(),
		middleware.UserID(r.Context()),
		pathParam(r, "id"),
		middleware.IsAdmin(r.Context()),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, session)
}

// GetSession menangani GET /tryout-sessions/{id}.
func (h *TryoutHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.deps.Tryout.GetSession(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, session)
}

// SaveAnswers menangani PATCH /tryout-sessions/{id}/answers.
func (h *TryoutHandler) SaveAnswers(w http.ResponseWriter, r *http.Request) {
	var req dto.SaveAnswersRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Tryout.SaveAnswers(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// Submit menangani POST /tryout-sessions/{id}/submit.
func (h *TryoutHandler) Submit(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Tryout.Submit(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// ListAttempts menangani GET /attempts.
func (h *TryoutHandler) ListAttempts(w http.ResponseWriter, r *http.Request) {
	page := paginationFrom(r)
	items, total, err := h.deps.Tryout.ListAttempts(r.Context(), middleware.UserID(r.Context()), page.Page, page.PerPage)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Paginated(w, items, page.Meta(total))
}

// GetAttempt menangani GET /attempts/{id}.
func (h *TryoutHandler) GetAttempt(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.Tryout.GetAttempt(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// Leaderboard menangani GET /tryouts/{id}/leaderboard.
func (h *TryoutHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Tryout.Leaderboard(
		r.Context(),
		pathParam(r, "id"),
		middleware.UserID(r.Context()),
		queryInt(r, "limit", 20),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}
