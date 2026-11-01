package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// PsychologistHandler melayani mitra psikolog, konsultasi daring, jejak akses
// data asesmen, dan koreksi hasil (Fase 3 langkah 6–7).
type PsychologistHandler struct {
	deps *Deps
}

// NewPsychologistHandler membentuk handler psikolog & konsultasi.
func NewPsychologistHandler(deps *Deps) *PsychologistHandler {
	return &PsychologistHandler{deps: deps}
}

/* --------------------------------- peserta -------------------------------- */

// Psychologists menangani GET /psychologists.
func (h *PsychologistHandler) Psychologists(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Psychologist.PublicPsychologists(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// PsychologistSlots menangani GET /psychologists/{id}/slots.
func (h *PsychologistHandler) PsychologistSlots(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Psychologist.PartnerSlots(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// BookConsultation menangani POST /consultations.
func (h *PsychologistHandler) BookConsultation(w http.ResponseWriter, r *http.Request) {
	var req dto.ConsultationBookingInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	view, err := h.deps.Psychologist.Book(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, view)
}

// MyConsultations menangani GET /me/consultations.
func (h *PsychologistHandler) MyConsultations(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.MyConsultations(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// CancelConsultation menangani POST /me/consultations/{id}/cancel.
func (h *PsychologistHandler) CancelConsultation(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.Cancel(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// DisputeAssessment menangani POST /me/assessments/{id}/dispute.
func (h *PsychologistHandler) DisputeAssessment(w http.ResponseWriter, r *http.Request) {
	var req dto.AssessmentDisputeInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	items, err := h.deps.Psychologist.DisputeAssessment(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, items)
}

// MyDisputes menangani GET /me/assessment-disputes.
func (h *PsychologistHandler) MyDisputes(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Psychologist.MyDisputes(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// MyAccessLogs menangani GET /me/assessment-access-logs.
func (h *PsychologistHandler) MyAccessLogs(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.MyAccessLogs(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

/* ---------------------------------- admin --------------------------------- */

// AdminPsychologists menangani GET /admin/psychologists.
func (h *PsychologistHandler) AdminPsychologists(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Psychologist.AdminPartners(r.Context())
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// UpsertPsychologist menangani POST /admin/psychologists.
func (h *PsychologistHandler) UpsertPsychologist(w http.ResponseWriter, r *http.Request) {
	var req dto.PsychologistPartnerInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	items, err := h.deps.Psychologist.UpsertPartner(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, items)
}

// VerifyPsychologist menangani POST /admin/psychologists/{id}/verify.
func (h *PsychologistHandler) VerifyPsychologist(w http.ResponseWriter, r *http.Request) {
	var req dto.PsychologistVerifyInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	items, err := h.deps.Psychologist.VerifyPartner(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// AdminPsychologistSlots menangani GET /admin/psychologists/{id}/slots.
func (h *PsychologistHandler) AdminPsychologistSlots(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Psychologist.AdminSlots(r.Context(), pathParam(r, "id"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateConsultationSlot menangani POST /admin/consultation-slots.
func (h *PsychologistHandler) CreateConsultationSlot(w http.ResponseWriter, r *http.Request) {
	var req dto.ConsultationSlotInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	items, err := h.deps.Psychologist.CreateSlot(r.Context(), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, items)
}

// AdminConsultations menangani GET /admin/consultations.
func (h *PsychologistHandler) AdminConsultations(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.Consultations(r.Context(), repository.ConsultationRecapFilter{
		PsychologistID: r.URL.Query().Get("psychologistId"),
		Status:         r.URL.Query().Get("status"),
		From:           queryDate(r, "from"),
		Limit:          queryInt(r, "limit", 200),
	})
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// AdminAccessLogs menangani GET /admin/assessment-access-logs/{userId}.
func (h *PsychologistHandler) AdminAccessLogs(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.AdminAccessLogs(r.Context(), pathParam(r, "userId"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

/* --------------------------------- psikolog -------------------------------- */

// Agenda menangani GET /psikolog/agenda.
func (h *PsychologistHandler) Agenda(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.Agenda(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// Participant menangani GET /psikolog/participants/{userId}.
func (h *PsychologistHandler) Participant(w http.ResponseWriter, r *http.Request) {
	view, err := h.deps.Psychologist.Participant(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "userId"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, view)
}

// AddConsultationNote menangani POST /psikolog/consultations/{id}/note.
func (h *PsychologistHandler) AddConsultationNote(w http.ResponseWriter, r *http.Request) {
	var req dto.ConsultationNoteInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	view, err := h.deps.Psychologist.AddNote(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, view)
}

// AssessCompetency menangani POST /psikolog/participants/{userId}/competencies.
func (h *PsychologistHandler) AssessCompetency(w http.ResponseWriter, r *http.Request) {
	var req dto.PsychologistCompetencyInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	if err := h.deps.Psychologist.AssessCompetency(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "userId"), req); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// InterpretInstrument menangani POST /psikolog/instruments/{code}/interpretation.
func (h *PsychologistHandler) InterpretInstrument(w http.ResponseWriter, r *http.Request) {
	var req dto.InstrumentInterpretationInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}
	view, err := h.deps.Psychologist.InterpretInstrument(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "code"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, view)
}

// queryDate membaca query parameter tanggal `YYYY-MM-DD`.
//
// Nilai kosong/tidak valid diabaikan (tanpa penyaring) mengikuti pola
// `queryInt`/`queryBool` yang sudah ada.
func queryDate(r *http.Request, name string) time.Time {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
