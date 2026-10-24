package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mediaRouter menirukan pemasangan endpoint media pada router sebenarnya
// (di bawah prefix /api/v1) sehingga pengambilan path wildcard ikut teruji.
func mediaRouter(handler *MediaHandler) http.Handler {
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/media/*", handler.Serve)
	})
	return router
}

// newMediaFixture menyiapkan direktori media sementara dengan satu berkas video
// dan satu berkas teks (untuk menguji penyaringan ekstensi).
func newMediaFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	lessons := filepath.Join(root, "lessons")
	if err := os.MkdirAll(lessons, 0o755); err != nil {
		t.Fatalf("gagal membuat direktori media: %v", err)
	}

	// Berkas 64 byte agar permintaan Range bisa diuji.
	video := make([]byte, 64)
	for index := range video {
		video[index] = byte(index)
	}
	if err := os.WriteFile(filepath.Join(lessons, "contoh.mp4"), video, 0o644); err != nil {
		t.Fatalf("gagal menulis berkas video: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "rahasia.txt"), []byte("jangan disajikan"), 0o644); err != nil {
		t.Fatalf("gagal menulis berkas teks: %v", err)
	}
	return root
}

func TestMediaHandlerServesVideo(t *testing.T) {
	media := mediaRouter(NewMediaHandler(newMediaFixture(t)))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/media/lessons/contoh.mp4", nil)
	recorder := httptest.NewRecorder()
	media.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, harap 200", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "video/mp4" {
		t.Errorf("Content-Type = %q, harap video/mp4", contentType)
	}
	if accept := recorder.Header().Get("Accept-Ranges"); accept != "bytes" {
		t.Errorf("Accept-Ranges = %q, harap bytes", accept)
	}
	if recorder.Body.Len() != 64 {
		t.Errorf("panjang balasan = %d, harap 64", recorder.Body.Len())
	}
}

// Permintaan Range harus dijawab 206 dengan Content-Range — inilah yang dipakai
// pemutar video saat peserta melompat ke tengah video.
func TestMediaHandlerSupportsRangeRequests(t *testing.T) {
	media := mediaRouter(NewMediaHandler(newMediaFixture(t)))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/media/lessons/contoh.mp4", nil)
	request.Header.Set("Range", "bytes=16-31")
	recorder := httptest.NewRecorder()
	media.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, harap 206", recorder.Code)
	}
	if contentRange := recorder.Header().Get("Content-Range"); contentRange != "bytes 16-31/64" {
		t.Errorf("Content-Range = %q, harap bytes 16-31/64", contentRange)
	}
	if recorder.Body.Len() != 16 {
		t.Errorf("panjang potongan = %d, harap 16", recorder.Body.Len())
	}
}

func TestMediaHandlerRejectsUnsafePaths(t *testing.T) {
	media := mediaRouter(NewMediaHandler(newMediaFixture(t)))

	cases := []struct {
		name string
		path string
	}{
		{"keluar direktori", "/api/v1/media/../rahasia.mp4"},
		{"keluar lewat subfolder", "/api/v1/media/lessons/../../rahasia.mp4"},
		{"path absolut", "/api/v1/media//etc/hosts.mp4"},
		{"tanpa berkas", "/api/v1/media/"},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		recorder := httptest.NewRecorder()
		media.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusOK || recorder.Code == http.StatusPartialContent {
			t.Errorf("%s: status = %d, seharusnya ditolak", tc.name, recorder.Code)
		}
	}
}

func TestMediaHandlerRejectsUnknownExtension(t *testing.T) {
	media := mediaRouter(NewMediaHandler(newMediaFixture(t)))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/media/rahasia.txt", nil)
	recorder := httptest.NewRecorder()
	media.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, harap 404", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "jangan disajikan") {
		t.Fatal("isi berkas terlarang tidak boleh ikut terkirim")
	}
}

func TestMediaHandlerWithoutRootIsNotFound(t *testing.T) {
	media := mediaRouter(NewMediaHandler(""))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/media/lessons/contoh.mp4", nil)
	recorder := httptest.NewRecorder()
	media.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, harap 404", recorder.Code)
	}
}
