package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// maxEvidenceUploadBytes membatasi unggahan berkas bukti pengumuman (5 MiB).
// Service memeriksa ulang ukuran & jenis berkas, jadi batas di sini hanya jaring
// pengaman agar body besar tidak dibaca ke memori.
const maxEvidenceUploadBytes = 5 << 20

// OutcomeHandler melayani tracer study hasil seleksi (Fase 2): sisi peserta
// (lapor hasil, riwayat, bukti) dan sisi admin (tinjauan, verifikasi, tahapan,
// target KPI, dasbor KPI, ekspor).
type OutcomeHandler struct {
	deps *Deps
}

// NewOutcomeHandler membentuk handler KPI kelulusan.
func NewOutcomeHandler(deps *Deps) *OutcomeHandler {
	return &OutcomeHandler{deps: deps}
}

/* --------------------------------- peserta --------------------------------- */

// OutcomeStages menangani GET /outcomes/stages (daftar tahapan seleksi aktif).
func (h *OutcomeHandler) OutcomeStages(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Outcome.Stages(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// MyOutcomes menangani GET /outcomes/me.
func (h *OutcomeHandler) MyOutcomes(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Outcome.MyOutcomes(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// ReportOutcome menangani POST /outcomes (lapor hasil seleksi).
func (h *OutcomeHandler) ReportOutcome(w http.ResponseWriter, r *http.Request) {
	var req dto.OutcomeInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.Report(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, item)
}

// UpdateOutcome menangani PATCH /outcomes/{id}.
func (h *OutcomeHandler) UpdateOutcome(w http.ResponseWriter, r *http.Request) {
	var req dto.OutcomePatch
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.UpdateOutcome(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

// DeleteOutcome menangani DELETE /outcomes/{id}.
func (h *OutcomeHandler) DeleteOutcome(w http.ResponseWriter, r *http.Request) {
	err := h.deps.Outcome.DeleteOutcome(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// SetOutcomeEvidence menangani POST /outcomes/{id}/evidence.
//
// Menerima dua bentuk: multipart/form-data dengan field `file` (gambar/PDF), atau
// JSON `{"evidenceUrl": "https://…"}` untuk menautkan pengumuman resmi.
func (h *OutcomeHandler) SetOutcomeEvidence(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		h.uploadOutcomeEvidence(w, r)
		return
	}

	var req dto.OutcomeEvidenceInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.SetEvidenceURL(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req.EvidenceURL)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

// uploadOutcomeEvidence membaca berkas bukti dari formulir multipart.
func (h *OutcomeHandler) uploadOutcomeEvidence(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEvidenceUploadBytes)

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Fail(w, apperror.BadRequest("Berkas bukti wajib dikirim pada field 'file'."))
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		response.Fail(w, apperror.BadRequest("Berkas bukti tidak bisa dibaca."))
		return
	}

	item, err := h.deps.Outcome.UploadEvidence(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), header.Filename, data)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

/* ---------------------------------- admin ---------------------------------- */

// AdminOutcomes menangani GET /admin/outcomes (daftar tinjauan laporan).
//
// Filter yang didukung: `year`, `sector`, `cohortId`, `program`, `stage`, `status`
// (verified/self_reported), `pending=true`, `search`, `limit`, `offset`.
func (h *OutcomeHandler) AdminOutcomes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := repository.OutcomeFilter{
		Year:      queryInt(r, "year", 0),
		StageKey:  query.Get("stage"),
		SectorKey: query.Get("sector"),
		CohortID:  query.Get("cohortId"),
		Program:   query.Get("program"),
		Search:    query.Get("search"),
		Limit:     queryInt(r, "limit", 50),
		Offset:    queryInt(r, "offset", 0),
	}
	if queryBool(r, "pending") {
		filter.OnlyPending = true
	} else {
		filter.Source = query.Get("status")
	}

	result, err := h.deps.Outcome.AdminOutcomes(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// AdminOutcomeAudits menangani GET /admin/outcomes/{id}/audit.
func (h *OutcomeHandler) AdminOutcomeAudits(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Outcome.OutcomeAudits(
		r.Context(), pathParam(r, "id"), queryInt(r, "limit", 50))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// VerifyOutcome menangani POST /admin/outcomes/{id}/verify.
func (h *OutcomeHandler) VerifyOutcome(w http.ResponseWriter, r *http.Request) {
	var req dto.OutcomeVerifyInput
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.VerifyOutcome(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

// RejectOutcome menangani POST /admin/outcomes/{id}/reject.
func (h *OutcomeHandler) RejectOutcome(w http.ResponseWriter, r *http.Request) {
	var req dto.OutcomeRejectInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.RejectOutcome(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

// AdminStages menangani GET /admin/selection-stages.
func (h *OutcomeHandler) AdminStages(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Outcome.AdminStages(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// UpdateStage menangani PATCH /admin/selection-stages/{key}.
func (h *OutcomeHandler) UpdateStage(w http.ResponseWriter, r *http.Request) {
	var req dto.SelectionStagePatch
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.UpdateStage(r.Context(), pathParam(r, "key"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

/* ------------------------------ target KPI --------------------------------- */

// AdminTargets menangani GET /admin/kpi/targets.
func (h *OutcomeHandler) AdminTargets(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Outcome.Targets(r.Context(), repository.KPITargetFilter{
		Year:     queryInt(r, "year", 0),
		OrgID:    r.URL.Query().Get("orgId"),
		CohortID: r.URL.Query().Get("cohortId"),
	})
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateTarget menangani POST /admin/kpi/targets.
func (h *OutcomeHandler) CreateTarget(w http.ResponseWriter, r *http.Request) {
	var req dto.KPITargetInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.CreateTarget(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, item)
}

// UpdateTarget menangani PATCH /admin/kpi/targets/{id}.
func (h *OutcomeHandler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	var req dto.KPITargetInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	item, err := h.deps.Outcome.UpdateTarget(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, item)
}

// DeleteTarget menangani DELETE /admin/kpi/targets/{id}.
func (h *OutcomeHandler) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Outcome.DeleteTarget(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* --------------------------------- KPI ------------------------------------- */

// kpiFilter menyusun filter dasbor KPI dari query string.
//
// Filter menentukan pembilang **dan** penyebut (§4.4), jadi nilainya diteruskan
// apa adanya ke service. `verification` menerima `verified`/`self_reported` serta
// padanan Indonesianya (`terverifikasi`/`dilaporkan`), dan `semua`/kosong berarti
// seluruh laporan.
func (h *OutcomeHandler) kpiFilter(r *http.Request) repository.KPIFilter {
	query := r.URL.Query()

	return repository.KPIFilter{
		Year:      queryInt(r, "year", 0),
		SectorKey: query.Get("sector"),
		CohortID:  query.Get("cohortId"),
		OrgID:     query.Get("orgId"),
		Program:   query.Get("program"),
		StageKey:  query.Get("stage"),
		Source:    kpiSourceFromQuery(query.Get("verification")),
	}
}

// kpiSourceFromQuery memetakan nilai `verification` menjadi tingkat kualitas data.
func kpiSourceFromQuery(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "verified", "terverifikasi":
		return "verified"
	case "self_reported", "dilaporkan", "reported":
		return string(models.OutcomeSelfReported)
	default:
		return ""
	}
}

// KPI menangani GET /admin/kpi/bumn.
func (h *OutcomeHandler) KPI(w http.ResponseWriter, r *http.Request) {
	report, err := h.deps.KPI.Dashboard(r.Context(), h.kpiFilter(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// KPIExportCSV menangani GET /admin/kpi/bumn/export.csv.
func (h *OutcomeHandler) KPIExportCSV(w http.ResponseWriter, r *http.Request) {
	filename, data, err := h.deps.KPI.ExportCSV(r.Context(), h.kpiFilter(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	writeDownload(w, filename, "text/csv; charset=utf-8", data, queryBool(r, "download"))
}

// KPIExportPDF menangani GET /admin/kpi/bumn/export.pdf.
func (h *OutcomeHandler) KPIExportPDF(w http.ResponseWriter, r *http.Request) {
	filename, data, err := h.deps.KPI.ExportPDF(r.Context(), h.kpiFilter(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	writePDF(w, filename, data, queryBool(r, "download"))
}

// PublicOutcomes menangani GET /insights/outcomes (tanpa login).
func (h *OutcomeHandler) PublicOutcomes(w http.ResponseWriter, r *http.Request) {
	report, err := h.deps.KPI.PublicOutcomes(r.Context(), queryInt(r, "year", 0))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// writeDownload mengirim berkas unduhan (CSV) dengan header yang aman.
func writeDownload(w http.ResponseWriter, filename, contentType string, data []byte, download bool) {
	disposition := "inline"
	if download {
		disposition = "attachment"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`%s; filename="%s"; filename*=UTF-8''%s`,
		disposition, filename, url.PathEscape(filename),
	))
	// Angka KPI bersifat internal: jangan disimpan di cache bersama.
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
