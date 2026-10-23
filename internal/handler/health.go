package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/cache"
	"github.com/bumn-academy/be-bumn-academy/internal/database"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/response"
	"gorm.io/gorm"
)

// HealthHandler melayani pemeriksaan kesehatan aplikasi.
type HealthHandler struct {
	db      *gorm.DB
	cache   *cache.Client
	version string
	started time.Time
}

// NewHealthHandler membentuk handler kesehatan.
func NewHealthHandler(db *gorm.DB, cacheClient *cache.Client, version string) *HealthHandler {
	return &HealthHandler{db: db, cache: cacheClient, version: version, started: time.Now()}
}

// Live menangani GET /healthz — hanya memastikan proses hidup.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{
		"status":    "ok",
		"version":   h.version,
		"uptimeSec": int(time.Since(h.started).Seconds()),
	})
}

// Ready menangani GET /readyz — memastikan dependency siap menerima trafik.
//
// Bila database tidak siap, status 503 dikembalikan supaya load balancer
// menghentikan pengiriman trafik ke replika ini.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	payload := map[string]any{
		"status":  "ok",
		"version": h.version,
	}

	healthy := true

	if sqlDB, err := h.db.DB(); err != nil {
		healthy = false
		payload["database"] = map[string]any{"status": "error", "message": err.Error()}
	} else if err := sqlDB.PingContext(ctx); err != nil {
		healthy = false
		payload["database"] = map[string]any{"status": "error", "message": err.Error()}
	} else {
		stats, _ := database.Stats(h.db)
		payload["database"] = map[string]any{"status": "ok", "pool": stats}
	}

	if h.cache != nil && h.cache.Enabled() {
		if err := h.cache.Ping(ctx); err != nil {
			// Redis tidak wajib: aplikasi tetap jalan tanpa cache.
			payload["redis"] = map[string]any{"status": "degraded", "message": err.Error()}
		} else {
			payload["redis"] = map[string]any{"status": "ok"}
		}
	} else {
		payload["redis"] = map[string]any{"status": "disabled"}
	}

	if !healthy {
		payload["status"] = "unavailable"
		response.JSON(w, http.StatusServiceUnavailable, map[string]any{
			"success": false,
			"data":    payload,
		})
		return
	}

	response.OK(w, payload)
}
