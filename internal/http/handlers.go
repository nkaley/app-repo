package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"myapp/internal/domain"
	"myapp/internal/metrics"
)

type service interface {
	Health(ctx context.Context) error
	Ready(ctx context.Context) error
	ListNotes(ctx context.Context) ([]domain.Note, error)
	CreateNote(ctx context.Context, title, content string) (domain.Note, error)
}

type Handler struct {
	svc     service
	metrics *metrics.Metrics
	version string
}

func NewHandler(svc service, m *metrics.Metrics, version string) *Handler {
	return &Handler{svc: svc, metrics: m, version: version}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	if err := h.svc.Health(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	if err := h.svc.Ready(ctx); err != nil {
		slog.Error("readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not-ready"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := h.svc.ListNotes(r.Context())
	if err != nil {
		slog.Error("list notes failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list notes"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": notes})
}

func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title and content are required"})
		return
	}

	note, err := h.svc.CreateNote(r.Context(), req.Title, req.Content)
	if err != nil {
		slog.Error("create note failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create note"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": note})
}

func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (h *Handler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	h.metrics.Handler(w, r)
}

func (h *Handler) Version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": h.version})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func IsServerClosed(err error) bool {
	return errors.Is(err, http.ErrServerClosed)
}
