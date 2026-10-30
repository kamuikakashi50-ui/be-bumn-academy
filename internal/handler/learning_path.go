package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// LearningPathHandler melayani jalur belajar & kuis: sisi peserta (mengerjakan)
// dan sisi admin (menyusun).
type LearningPathHandler struct {
	deps *Deps
}

// NewLearningPathHandler membentuk handler jalur belajar.
func NewLearningPathHandler(deps *Deps) *LearningPathHandler {
	return &LearningPathHandler{deps: deps}
}

/* --------------------------------- peserta -------------------------------- */

// Paths menangani GET /learning/paths.
func (h *LearningPathHandler) Paths(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.LearningPath.Paths(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Path menangani GET /learning/paths/{slug}.
func (h *LearningPathHandler) Path(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.LearningPath.Path(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "slug"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// LessonQuiz menangani GET /classroom/lessons/{id}/quiz.
func (h *LearningPathHandler) LessonQuiz(w http.ResponseWriter, r *http.Request) {
	quiz, err := h.deps.LearningPath.LessonQuiz(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, quiz)
}

// SubmitQuiz menangani POST /classroom/lessons/{id}/quiz.
func (h *LearningPathHandler) SubmitQuiz(w http.ResponseWriter, r *http.Request) {
	var req dto.SubmitQuizRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.LearningPath.SubmitQuiz(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

/* ----------------------------- jalur (admin) ------------------------------ */

// AdminPaths menangani GET /admin/learning/paths.
func (h *LearningPathHandler) AdminPaths(w http.ResponseWriter, r *http.Request) {
	filter := repository.PathFilter{
		SectorKey:   r.URL.Query().Get("sectorKey"),
		TestType:    r.URL.Query().Get("testType"),
		ProductSlug: r.URL.Query().Get("productSlug"),
		Search:      r.URL.Query().Get("search"),
		ActiveOnly:  queryBool(r, "activeOnly"),
	}

	items, err := h.deps.LearningPath.AdminPaths(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// AdminPath menangani GET /admin/learning/paths/{id} (detail + langkah).
func (h *LearningPathHandler) AdminPath(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.LearningPath.AdminPath(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// CreatePath menangani POST /admin/learning/paths.
func (h *LearningPathHandler) CreatePath(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePathRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	path, err := h.deps.LearningPath.CreatePath(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, path)
}

// UpdatePath menangani PATCH /admin/learning/paths/{id}.
func (h *LearningPathHandler) UpdatePath(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdatePathRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	path, err := h.deps.LearningPath.UpdatePath(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, path)
}

// DeletePath menangani DELETE /admin/learning/paths/{id}.
//
// Penghapusan bersifat lunak (jalur dinonaktifkan): sertifikat & riwayat kuis
// peserta menunjuk jalur ini, jadi barisnya tidak boleh hilang.
func (h *LearningPathHandler) DeletePath(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.LearningPath.SetPathActive(r.Context(), pathParam(r, "id"), false); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// SetPathSteps menangani POST /admin/learning/paths/{id}/steps.
func (h *LearningPathHandler) SetPathSteps(w http.ResponseWriter, r *http.Request) {
	var req dto.SetPathStepsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	path, err := h.deps.LearningPath.SetPathSteps(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, path)
}

/* ------------------------------ kuis (admin) ------------------------------ */

// AdminQuizzes menangani GET /admin/classroom/quizzes?lessonId=.
func (h *LearningPathHandler) AdminQuizzes(w http.ResponseWriter, r *http.Request) {
	lessonID := r.URL.Query().Get("lessonId")
	if lessonID == "" {
		response.Fail(w, apperror.Validation("Parameter lessonId wajib diisi.", nil))
		return
	}

	items, err := h.deps.LearningPath.AdminQuizzes(r.Context(), lessonID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateQuiz menangani POST /admin/classroom/quizzes.
func (h *LearningPathHandler) CreateQuiz(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateQuizRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	quiz, err := h.deps.LearningPath.CreateQuiz(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, quiz)
}

// UpdateQuiz menangani PATCH /admin/classroom/quizzes/{id}.
func (h *LearningPathHandler) UpdateQuiz(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateQuizRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	quiz, err := h.deps.LearningPath.UpdateQuiz(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, quiz)
}

// DeleteQuiz menangani DELETE /admin/classroom/quizzes/{id}.
//
// Kuis tidak dihapus permanen: percobaan peserta menunjuk kuis ini, sehingga kuis
// hanya dinonaktifkan dan riwayat nilainya tetap utuh.
func (h *LearningPathHandler) DeleteQuiz(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.LearningPath.SetQuizActive(r.Context(), pathParam(r, "id"), false); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// QuizAttempts menangani GET /admin/classroom/quizzes/{id}/attempts.
func (h *LearningPathHandler) QuizAttempts(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.LearningPath.QuizAttempts(
		r.Context(), pathParam(r, "id"), queryInt(r, "limit", 50),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}
