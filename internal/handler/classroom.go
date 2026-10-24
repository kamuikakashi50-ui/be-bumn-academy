package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/middleware"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"github.com/go-chi/chi/v5"
)

// ClassroomHandler melayani ruang kelas digital (materi, progres, e-book) dan
// pengelolaannya oleh admin.
type ClassroomHandler struct {
	deps *Deps
}

// NewClassroomHandler membentuk handler ruang kelas.
func NewClassroomHandler(deps *Deps) *ClassroomHandler {
	return &ClassroomHandler{deps: deps}
}

/* --------------------------------- peserta -------------------------------- */

// Courses menangani GET /classroom.
func (h *ClassroomHandler) Courses(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Classroom.Courses(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// Course menangani GET /classroom/courses/{slug}.
func (h *ClassroomHandler) Course(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.Classroom.Course(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "slug"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// Lesson menangani GET /classroom/lessons/{id}.
func (h *ClassroomHandler) Lesson(w http.ResponseWriter, r *http.Request) {
	detail, err := h.deps.Classroom.Lesson(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, detail)
}

// SaveProgress menangani POST /classroom/lessons/{id}/progress.
func (h *ClassroomHandler) SaveProgress(w http.ResponseWriter, r *http.Request) {
	var req dto.SaveProgressRequest
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	lesson, err := h.deps.Classroom.SaveProgress(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"), req,
	)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, lesson)
}

// Ebook menangani GET /classroom/lessons/{id}/ebook.
//
// Mengembalikan berkas PDF (bisa dibaca langsung di peramban) atau mengalihkan
// ke tautan e-book eksternal bila admin mengisinya. `?download=1` memaksa unduhan.
func (h *ClassroomHandler) Ebook(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Classroom.Ebook(
		r.Context(), middleware.UserID(r.Context()), pathParam(r, "id"),
	)
	if err != nil {
		response.Fail(w, err)
		return
	}

	if result.ExternalURL != "" {
		http.Redirect(w, r, result.ExternalURL, http.StatusFound)
		return
	}

	disposition := "inline"
	if queryBool(r, "download") {
		disposition = "attachment"
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(result.Data)))
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`%s; filename="%s"; filename*=UTF-8''%s`,
		disposition, result.Filename, url.PathEscape(result.Filename),
	))
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Data)
}

// notFoundMedia membentuk galat media yang tidak tersedia.
func notFoundMedia() error {
	return apperror.NotFound("Berkas media tidak ditemukan.")
}

/* ---------------------------------- admin --------------------------------- */

// AdminModules menangani GET /admin/classroom/modules.
func (h *ClassroomHandler) AdminModules(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Classroom.AdminModules(r.Context(), r.URL.Query().Get("productSlug"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateModule menangani POST /admin/classroom/modules.
func (h *ClassroomHandler) CreateModule(w http.ResponseWriter, r *http.Request) {
	var req dto.ModuleInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	module, err := h.deps.Classroom.UpsertModule(r.Context(), req, "")
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, module)
}

// UpdateModule menangani PATCH /admin/classroom/modules/{id}.
func (h *ClassroomHandler) UpdateModule(w http.ResponseWriter, r *http.Request) {
	var req dto.ModulePatch
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	module, err := h.deps.Classroom.UpdateModule(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, module)
}

// DeleteModule menangani DELETE /admin/classroom/modules/{id}.
func (h *ClassroomHandler) DeleteModule(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Classroom.DeleteModule(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

// AdminLessons menangani GET /admin/classroom/lessons.
func (h *ClassroomHandler) AdminLessons(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Classroom.AdminLessons(r.Context(), r.URL.Query().Get("productSlug"))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, items)
}

// CreateLesson menangani POST /admin/classroom/lessons.
func (h *ClassroomHandler) CreateLesson(w http.ResponseWriter, r *http.Request) {
	var req dto.LessonInput
	if err := decodeJSON(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	lesson, err := h.deps.Classroom.UpsertLesson(r.Context(), req, "")
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Created(w, lesson)
}

// UpdateLesson menangani PATCH /admin/classroom/lessons/{id}.
func (h *ClassroomHandler) UpdateLesson(w http.ResponseWriter, r *http.Request) {
	var req dto.LessonPatch
	if err := decodeJSONOptional(w, r, &req); err != nil {
		response.Fail(w, err)
		return
	}

	lesson, err := h.deps.Classroom.UpdateLesson(r.Context(), pathParam(r, "id"), req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.OK(w, lesson)
}

// DeleteLesson menangani DELETE /admin/classroom/lessons/{id}.
func (h *ClassroomHandler) DeleteLesson(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Classroom.DeleteLesson(r.Context(), pathParam(r, "id")); err != nil {
		response.Fail(w, err)
		return
	}
	response.NoContent(w)
}

/* ---------------------------------- media --------------------------------- */

// MediaHandler menyajikan berkas media (video & e-book) dari direktori lokal.
//
// Memakai `http.ServeFile` sehingga permintaan berskala (Range) untuk mencari
// posisi video ditangani sesuai standar HTTP.
type MediaHandler struct {
	root string
}

// NewMediaHandler membentuk handler media.
func NewMediaHandler(root string) *MediaHandler {
	return &MediaHandler{root: root}
}

// allowedExtensions adalah jenis berkas yang boleh disajikan beserta MIME-nya.
var allowedExtensions = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".pdf":  "application/pdf",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".vtt":  "text/vtt; charset=utf-8",
	".srt":  "text/plain; charset=utf-8",
}

// Serve menangani GET /media/*.
//
// Path relatif diambil dari parameter wildcard chi (router memasang endpoint ini
// di bawah /api/v1, sehingga `r.URL.Path` masih memuat prefix tersebut).
func (h *MediaHandler) Serve(w http.ResponseWriter, r *http.Request) {
	if h.root == "" {
		response.Fail(w, notFoundMedia())
		return
	}

	relative := strings.TrimSpace(chi.URLParam(r, "*"))
	if relative == "" {
		relative = strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/media/"))
	}
	if relative == "" || strings.HasPrefix(relative, "/") || strings.Contains(relative, "..") {
		response.Fail(w, notFoundMedia())
		return
	}

	// Hanya berkas dengan ekstensi yang dikenal yang dilayani.
	lower := strings.ToLower(relative)
	contentType := ""
	for ext, mime := range allowedExtensions {
		if strings.HasSuffix(lower, ext) {
			contentType = mime
			break
		}
	}
	if contentType == "" {
		response.Fail(w, notFoundMedia())
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, h.root+"/"+relative)
}
