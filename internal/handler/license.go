package handler

import (
	"net/http"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// LicenseHandler melayani izin/hak pakai soal, instrumen psikologi, dan
// kepatuhan bank soal (deteksi soal kembar + jejak audit).
type LicenseHandler struct {
	deps *Deps
}

// NewLicenseHandler membentuk handler izin soal.
func NewLicenseHandler(deps *Deps) *LicenseHandler {
	return &LicenseHandler{deps: deps}
}

/* ------------------------------- izin soal ------------------------------- */

// List menangani GET /admin/licenses.
func (h *LicenseHandler) List(w http.ResponseWriter, r *http.Request) {
	page := paginationFrom(r)
	filter := repository.LicenseFilter{
		QuestionID:  r.URL.Query().Get("questionId"),
		Scope:       r.URL.Query().Get("scope"),
		Status:      r.URL.Query().Get("status"),
		Search:      r.URL.Query().Get("search"),
		OnlyProblem: queryBool(r, "onlyProblem"),
	}

	items, total, err := h.deps.License.List(r.Context(), filter, page.Page, page.PerPage)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Paginated(w, items, page.Meta(total))
}

// Summary menangani GET /admin/licenses/summary.
//
// Ringkasan ini menjawab "soal mana yang izinnya belum lengkap": soal tanpa izin
// aktif tidak dikirim ke peserta saat penegakan izin menyala.
func (h *LicenseHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.deps.License.Summary(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, summary)
}

// Create menangani POST /admin/licenses.
func (h *LicenseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateLicenseRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	license, err := h.deps.License.Create(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, license)
}

// Update menangani PATCH /admin/licenses/{id}.
func (h *LicenseHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateLicenseRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	license, err := h.deps.License.Update(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, license)
}

// Delete menangani DELETE /admin/licenses/{id}.
func (h *LicenseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.License.Delete(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* --------------------------- instrumen psikologi -------------------------- */

// ListInstruments menangani GET /admin/instruments.
func (h *LicenseHandler) ListInstruments(w http.ResponseWriter, r *http.Request) {
	filter := repository.InstrumentFilter{
		Kind:       r.URL.Query().Get("kind"),
		Search:     r.URL.Query().Get("search"),
		ActiveOnly: queryBool(r, "activeOnly"),
	}

	items, err := h.deps.License.ListInstruments(r.Context(), filter)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateInstrument menangani POST /admin/instruments.
func (h *LicenseHandler) CreateInstrument(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateInstrumentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	instrument, err := h.deps.License.CreateInstrument(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, instrument)
}

// UpdateInstrument menangani PATCH /admin/instruments/{id}.
func (h *LicenseHandler) UpdateInstrument(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateInstrumentRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	instrument, err := h.deps.License.UpdateInstrument(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, instrument)
}

// DeleteInstrument menangani DELETE /admin/instruments/{id}.
func (h *LicenseHandler) DeleteInstrument(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.License.DeleteInstrument(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* ------------------------------- kepatuhan ------------------------------- */

// CheckDuplicate menangani POST /admin/questions/check-duplicate.
//
// Dipakai panel bank soal sebelum menyimpan soal: admin melihat soal kembar yang
// sudah ada, lalu memutuskan mengganti teks, menjadikannya varian, atau
// melanjutkan.
func (h *LicenseHandler) CheckDuplicate(w http.ResponseWriter, r *http.Request) {
	var req dto.CheckDuplicateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	result, err := h.deps.Compliance.Check(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, result)
}

// UniquenessReport menangani GET /admin/compliance/question-uniqueness.
func (h *LicenseHandler) UniquenessReport(w http.ResponseWriter, r *http.Request) {
	filter := repository.FingerprintFilter{
		BankKey:    r.URL.Query().Get("bankKey"),
		SectorKey:  r.URL.Query().Get("sectorKey"),
		TestType:   r.URL.Query().Get("testType"),
		TryoutID:   r.URL.Query().Get("tryoutId"),
		ActiveOnly: queryBool(r, "activeOnly"),
	}

	report, err := h.deps.Compliance.Report(r.Context(), filter, queryInt(r, "limit", 0))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, report)
}

// AccessLog menangani GET /admin/compliance/access-log.
func (h *LicenseHandler) AccessLog(w http.ResponseWriter, r *http.Request) {
	filter := repository.AccessLogFilter{
		QuestionID: r.URL.Query().Get("questionId"),
		ActorID:    r.URL.Query().Get("actorId"),
		Action:     r.URL.Query().Get("action"),
	}

	items, err := h.deps.Compliance.AccessLog(r.Context(), filter, queryInt(r, "limit", 0))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// RecordVariant menangani POST /admin/questions/{id}/variant.
//
// Menautkan soal ke grup varian dicatat tersendiri karena inilah tindakan yang
// menyatakan "soal ini memang saudara soal lain" — keputusan yang perlu terlihat
// di jejak audit.
func (h *LicenseHandler) RecordVariant(w http.ResponseWriter, r *http.Request) {
	questionID := pathParam(r, "id")
	h.deps.Compliance.Record(
		r.Context(), middleware.UserID(r.Context()), questionID,
		models.QuestionActionVariant, "Soal ditautkan ke grup varian.")
	response.OK(w, map[string]string{"questionId": questionID})
}
