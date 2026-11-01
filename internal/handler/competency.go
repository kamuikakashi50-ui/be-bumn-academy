package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// CompetencyHandler melayani kamus kompetensi, standar jabatan, profil peserta,
// dan matriks kompetensi (Fase 3 langkah 1–2).
type CompetencyHandler struct {
	deps *Deps
}

// NewCompetencyHandler membentuk handler kompetensi.
func NewCompetencyHandler(deps *Deps) *CompetencyHandler {
	return &CompetencyHandler{deps: deps}
}

/* -------------------------------- peserta --------------------------------- */

// MyCompetencies menangani GET /me/competencies.
func (h *CompetencyHandler) MyCompetencies(w http.ResponseWriter, r *http.Request) {
	report, err := h.deps.Competency.Profile(
		r.Context(), middleware.UserID(r.Context()), r.URL.Query().Get("positionFamily"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// SelfAssess menangani POST /me/competencies/self-assessment.
func (h *CompetencyHandler) SelfAssess(w http.ResponseWriter, r *http.Request) {
	var req dto.SelfAssessmentInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	report, err := h.deps.Competency.SelfAssess(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

/* --------------------------------- admin ---------------------------------- */

// AdminFramework menangani GET /admin/competencies.
func (h *CompetencyHandler) AdminFramework(w http.ResponseWriter, r *http.Request) {
	report, err := h.deps.Competency.Framework(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// UpsertCompetency menangani POST /admin/competencies.
func (h *CompetencyHandler) UpsertCompetency(w http.ResponseWriter, r *http.Request) {
	var req dto.CompetencyInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	report, err := h.deps.Competency.UpsertCompetency(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// UpsertLevel menangani POST /admin/competencies/{code}/levels.
func (h *CompetencyHandler) UpsertLevel(w http.ResponseWriter, r *http.Request) {
	var req dto.CompetencyLevelInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	report, err := h.deps.Competency.UpsertLevel(r.Context(), pathParam(r, "code"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// AdminRequirements menangani GET /admin/positions/{family}/requirements.
func (h *CompetencyHandler) AdminRequirements(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Competency.Requirements(r.Context(), pathParam(r, "family"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// UpsertRequirement menangani POST /admin/positions/{family}/requirements.
func (h *CompetencyHandler) UpsertRequirement(w http.ResponseWriter, r *http.Request) {
	var req dto.PositionRequirementInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	items, err := h.deps.Competency.UpsertRequirement(r.Context(), pathParam(r, "family"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// DeleteRequirement menangani DELETE /admin/positions/{family}/requirements/{code}.
func (h *CompetencyHandler) DeleteRequirement(w http.ResponseWriter, r *http.Request) {
	err := h.deps.Competency.DeleteRequirement(
		r.Context(), pathParam(r, "family"), r.URL.Query().Get("sector"), pathParam(r, "code"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// AdminMatrix menangani GET /admin/competency-matrix.
func (h *CompetencyHandler) AdminMatrix(w http.ResponseWriter, r *http.Request) {
	filter := repository.MatrixFilter{
		Program: r.URL.Query().Get("program"),
		Search:  r.URL.Query().Get("search"),
		Limit:   queryInt(r, "limit", 100),
	}

	// Bila diminta, batasi ke anggota sebuah kelas khusus (Fase 1).
	if cohortID := r.URL.Query().Get("cohortId"); cohortID != "" {
		members, err := h.deps.Cohort.CohortMembers(r.Context(), cohortID)
		if err != nil {
			response.Fail(w, err)
			return
		}
		userIDs := make([]string, 0, len(members))
		for _, member := range members {
			if member.Role == "peserta" {
				userIDs = append(userIDs, member.UserID)
			}
		}
		filter.UserIDs = userIDs
		filter.UserIDsSet = true
	}

	report, err := h.deps.Competency.Matrix(
		r.Context(), filter, r.URL.Query().Get("positionFamily"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}
