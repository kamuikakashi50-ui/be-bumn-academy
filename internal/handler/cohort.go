package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// CohortHandler melayani kelas khusus (kohort): sisi admin (instansi, kelas,
// keanggotaan, kurikulum, kode undangan, jejak audit) dan sisi peserta (gabung).
type CohortHandler struct {
	deps *Deps
}

// NewCohortHandler membentuk handler kelas khusus.
func NewCohortHandler(deps *Deps) *CohortHandler {
	return &CohortHandler{deps: deps}
}

/* ------------------------------- instansi --------------------------------- */

// AdminOrganizations menangani GET /admin/organizations.
func (h *CohortHandler) AdminOrganizations(w http.ResponseWriter, r *http.Request) {
	filter := repository.OrganizationFilter{
		Search:     r.URL.Query().Get("search"),
		SectorKey:  r.URL.Query().Get("sectorKey"),
		ActiveOnly: queryBool(r, "activeOnly"),
	}

	items, err := h.deps.Cohort.Organizations(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateOrganization menangani POST /admin/organizations.
func (h *CohortHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req dto.OrganizationInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	org, err := h.deps.Cohort.CreateOrganization(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, org)
}

// UpdateOrganization menangani PATCH /admin/organizations/{id}.
func (h *CohortHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	var req dto.OrganizationPatch
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	org, err := h.deps.Cohort.UpdateOrganization(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, org)
}

/* -------------------------------- kohort ---------------------------------- */

// AdminCohorts menangani GET /admin/cohorts.
func (h *CohortHandler) AdminCohorts(w http.ResponseWriter, r *http.Request) {
	filter := repository.CohortFilter{
		OrgID:     r.URL.Query().Get("orgId"),
		Status:    r.URL.Query().Get("status"),
		SectorKey: r.URL.Query().Get("sectorKey"),
		Search:    r.URL.Query().Get("search"),
	}

	items, err := h.deps.Cohort.AdminCohorts(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// AdminCohort menangani GET /admin/cohorts/{id}.
func (h *CohortHandler) AdminCohort(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.Cohort.AdminCohort(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// CreateCohort menangani POST /admin/cohorts.
func (h *CohortHandler) CreateCohort(w http.ResponseWriter, r *http.Request) {
	var req dto.CohortInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	detail, err := h.deps.Cohort.CreateCohort(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, detail)
}

// UpdateCohort menangani PATCH /admin/cohorts/{id}.
func (h *CohortHandler) UpdateCohort(w http.ResponseWriter, r *http.Request) {
	var req dto.CohortPatch
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	detail, err := h.deps.Cohort.UpdateCohort(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// SetCohortCurriculum menangani PUT /admin/cohorts/{id}/curriculum.
func (h *CohortHandler) SetCohortCurriculum(w http.ResponseWriter, r *http.Request) {
	var req []dto.CohortCurriculumInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	detail, err := h.deps.Cohort.SetCohortCurriculum(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

/* ------------------------------ keanggotaan ------------------------------- */

// AdminCohortMembers menangani GET /admin/cohorts/{id}/members.
func (h *CohortHandler) AdminCohortMembers(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Cohort.CohortMembers(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// AddCohortMembers menangani POST /admin/cohorts/{id}/members.
//
// Menerima dua bentuk: JSON (`userIds`/`emails`) untuk penambahan cepat, atau
// berkas Excel/CSV pada field `file` untuk penempatan massal.
func (h *CohortHandler) AddCohortMembers(w http.ResponseWriter, r *http.Request) {
	if isMultipart(r) {
		h.importCohortMembers(w, r)
		return
	}

	var req dto.CohortMemberInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Cohort.AddCohortMembers(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// importCohortMembers membaca berkas Excel/CSV anggota kelas.
func (h *CohortHandler) importCohortMembers(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.deps.Cohort.ImportCohortMembers(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), header.Filename, data,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// UpdateCohortMember menangani PATCH /admin/cohorts/{id}/members/{userId}.
func (h *CohortHandler) UpdateCohortMember(w http.ResponseWriter, r *http.Request) {
	var req dto.CohortMemberPatch
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	items, err := h.deps.Cohort.UpdateCohortMember(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), pathParam(r, "userId"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// RemoveCohortMember menangani DELETE /admin/cohorts/{id}/members/{userId}.
func (h *CohortHandler) RemoveCohortMember(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Cohort.RemoveCohortMember(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), pathParam(r, "userId"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

/* ------------------------------ kode undangan ----------------------------- */

// AdminCohortInvites menangani GET /admin/cohorts/{id}/invites.
func (h *CohortHandler) AdminCohortInvites(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Cohort.CohortInvites(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateCohortInvite menangani POST /admin/cohorts/{id}/invites.
func (h *CohortHandler) CreateCohortInvite(w http.ResponseWriter, r *http.Request) {
	var req dto.CohortInviteInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	invite, err := h.deps.Cohort.CreateCohortInvite(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, invite)
}

// RevokeCohortInvite menangani DELETE /admin/cohorts/{id}/invites/{code}.
func (h *CohortHandler) RevokeCohortInvite(w http.ResponseWriter, r *http.Request) {
	invite, err := h.deps.Cohort.RevokeCohortInvite(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), pathParam(r, "code"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, invite)
}

// AdminCohortAudit menangani GET /admin/cohorts/{id}/audit.
func (h *CohortHandler) AdminCohortAudit(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := h.deps.Cohort.CohortAudit(r.Context(), pathParam(r, "id"), limit)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

/* --------------------------------- laporan -------------------------------- */

// AdminCohortReport menangani GET /admin/cohorts/{id}/report.
func (h *CohortHandler) AdminCohortReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.deps.Cohort.Report(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// AdminCohortReportPDF menangani GET /admin/cohorts/{id}/report.pdf.
func (h *CohortHandler) AdminCohortReportPDF(w http.ResponseWriter, r *http.Request) {
	data, filename, err := h.deps.Cohort.ReportPDF(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	writePDF(w, filename, data, queryBool(r, "download"))
}

/* --------------------------------- peserta -------------------------------- */

// Join menangani POST /cohorts/join.
func (h *CohortHandler) Join(w http.ResponseWriter, r *http.Request) {
	var req dto.JoinCohortRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Cohort.Join(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// MyCohorts menangani GET /cohorts/me.
func (h *CohortHandler) MyCohorts(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Cohort.MyCohorts(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// isMultipart menandai permintaan unggah berkas (import anggota massal).
func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
}
