package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

const pingTimeout = 2 * time.Second

// Handler serves Kubernetes probe endpoints and a combined health check.
type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/health", h.Health)
	r.Get("/healthz", h.Health)
	r.Get("/livez", h.Liveness)
	r.Get("/readyz", h.Readiness)
	r.Get("/startupz", h.Startup)
	r.Get("/health/live", h.Liveness)
	r.Get("/health/ready", h.Readiness)
	r.Get("/health/startup", h.Startup)
}

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Liveness reports that the process is running. It does not touch Postgres so a
// database outage does not cause Kubernetes to restart the pod.
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{Status: "up"})
}

// Health is the combined check: process plus Postgres connectivity.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeDependencyCheck(w, r)
}

// Readiness reports whether the process can accept traffic. Postgres must be reachable.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	h.writeDependencyCheck(w, r)
}

// Startup reports whether the process has finished starting. Postgres must be reachable.
func (h *Handler) Startup(w http.ResponseWriter, r *http.Request) {
	h.writeDependencyCheck(w, r)
}

func (h *Handler) writeDependencyCheck(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{"postgres": "up"}
	body := response{Status: "up", Checks: checks}

	if err := h.pingDB(r.Context()); err != nil {
		checks["postgres"] = "down"
		body.Status = "down"
		writeJSON(w, http.StatusServiceUnavailable, body)
		return
	}

	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) pingDB(ctx context.Context) error {
	if h.db == nil {
		return gorm.ErrInvalidDB
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

func writeJSON(w http.ResponseWriter, status int, body response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
