package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// TalentHandler melayani instrumen minat–bakat, asesmen peserta, bobot lini
// bisnis, dan peta talenta (Fase 3 langkah 3–4).
type TalentHandler struct {
	deps *Deps
}

// NewTalentHandler membentuk handler talent mapping.
func NewTalentHandler(deps *Deps) *TalentHandler {
	return &TalentHandler{deps: deps}
}

/* -------------------------------- peserta --------------------------------- */

// Instruments menangani GET /talent/instruments.
func (h *TalentHandler) Instruments(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Talent.Instruments(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// StartAssessment menangani POST /talent/assessments.
func (h *TalentHandler) StartAssessment(w http.ResponseWriter, r *http.Request) {
	var req dto.TalentStartInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	view, err := h.deps.Talent.StartAssessment(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// SaveAnswers menangani PATCH /talent/assessments/{id}/answers.
func (h *TalentHandler) SaveAnswers(w http.ResponseWriter, r *http.Request) {
	var req dto.TalentAnswersInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	view, err := h.deps.Talent.SaveAnswers(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// SubmitAssessment menangani POST /talent/assessments/{id}/submit.
func (h *TalentHandler) SubmitAssessment(w http.ResponseWriter, r *http.Request) {
	profile, err := h.deps.Talent.Submit(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, profile)
}

// Profile menangani GET /talent/profile.
func (h *TalentHandler) Profile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.deps.Talent.Profile(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, profile)
}

/* --------------------------------- admin ---------------------------------- */

// matrixFilter menyusun penyaring daftar peserta dari query string.
//
// Bila `cohortId` diberikan, daftar dibatasi ke anggota kelas itu — sama seperti
// matriks kompetensi — supaya angka antar halaman tetap konsisten.
func (h *TalentHandler) matrixFilter(r *http.Request) (repository.MatrixFilter, error) {
	filter := repository.MatrixFilter{
		Program: r.URL.Query().Get("program"),
		Search:  r.URL.Query().Get("search"),
		Limit:   queryInt(r, "limit", 100),
	}
	cohortID := r.URL.Query().Get("cohortId")
	if cohortID == "" {
		return filter, nil
	}
	members, err := h.deps.Cohort.CohortMembers(r.Context(), cohortID)
	if err != nil {
		return filter, err
	}
	userIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member.Role == "peserta" {
			userIDs = append(userIDs, member.UserID)
		}
	}
	filter.UserIDs = userIDs
	filter.UserIDsSet = true
	return filter, nil
}

// AdminInstruments menangani GET /admin/talent/instruments.
func (h *TalentHandler) AdminInstruments(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Talent.AdminInstruments(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// UpsertInstrument menangani POST /admin/talent/instruments.
func (h *TalentHandler) UpsertInstrument(w http.ResponseWriter, r *http.Request) {
	var req dto.TalentInstrumentInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	if err := h.deps.Talent.UpsertInstrument(r.Context(), req); err != nil {
		response.Fail(w, err)
		return
	}
	items, err := h.deps.Talent.AdminInstruments(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// UpsertInstrumentItem menangani POST /admin/talent/instruments/{id}/items.
func (h *TalentHandler) UpsertInstrumentItem(w http.ResponseWriter, r *http.Request) {
	var req dto.TalentItemInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	if err := h.deps.Talent.UpsertItem(r.Context(), pathParam(r, "id"), req); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// BusinessLines menangani GET /admin/business-lines.
func (h *TalentHandler) BusinessLines(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Talent.BusinessLines(r.Context(), false)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateBusinessLine menangani POST /admin/business-lines.
func (h *TalentHandler) CreateBusinessLine(w http.ResponseWriter, r *http.Request) {
	var req dto.BusinessLineInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	items, err := h.deps.Talent.UpsertBusinessLine(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, items)
}

// UpdateBusinessLine menangani PATCH /admin/business-lines/{id}.
func (h *TalentHandler) UpdateBusinessLine(w http.ResponseWriter, r *http.Request) {
	var req dto.BusinessLinePatch
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	items, err := h.deps.Talent.UpdateBusinessLine(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Mapping menangani GET /admin/talent/mapping.
func (h *TalentHandler) Mapping(w http.ResponseWriter, r *http.Request) {
	filter, err := h.matrixFilter(r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	view, err := h.deps.Talent.Mapping(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// NineBox menangani GET /admin/talent/nine-box.
func (h *TalentHandler) NineBox(w http.ResponseWriter, r *http.Request) {
	filter, err := h.matrixFilter(r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	view, err := h.deps.Talent.NineBox(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// MappingDetail menangani GET /admin/talent/mapping/{userId}.
func (h *TalentHandler) MappingDetail(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Talent.MappingDetail(r.Context(), pathParam(r, "userId"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}
