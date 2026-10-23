package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
)

// LearningHandler melayani kelas live, rekaman, dan ringkasan dashboard.
type LearningHandler struct {
	deps *Deps
}

// NewLearningHandler membentuk handler ruang belajar.
func NewLearningHandler(deps *Deps) *LearningHandler {
	return &LearningHandler{deps: deps}
}

// LiveSessions menangani GET /live-sessions.
func (h *LearningHandler) LiveSessions(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Learning.LiveSessions(r.Context(), middleware.UserID(r.Context()), queryBool(r, "upcomingOnly"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// ToggleReminder menangani POST /live-sessions/{id}/reminder.
func (h *LearningHandler) ToggleReminder(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Learning.ToggleReminder(r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// Recordings menangani GET /recordings.
func (h *LearningHandler) Recordings(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Learning.Recordings(
		r.Context(),
		r.URL.Query().Get("category"),
		r.URL.Query().Get("search"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Dashboard menangani GET /dashboard/summary.
func (h *LearningHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	summary, err := h.deps.Learning.Dashboard(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, summary)
}
