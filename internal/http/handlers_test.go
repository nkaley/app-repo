package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"myapp/internal/domain"
	"myapp/internal/metrics"
)

type stubService struct {
	readyErr error
	notes    []domain.Note
}

func (s *stubService) Health(ctx context.Context) error {
	_ = ctx
	return nil
}

func (s *stubService) Ready(ctx context.Context) error {
	_ = ctx
	return s.readyErr
}

func (s *stubService) ListNotes(ctx context.Context) ([]domain.Note, error) {
	_ = ctx
	return s.notes, nil
}

func (s *stubService) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	_ = ctx
	return domain.Note{
		ID:        1,
		Title:     title,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func TestHealthz(t *testing.T) {
	h := NewHandler(&stubService{}, metrics.New(), "test")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadyzNotReady(t *testing.T) {
	h := NewHandler(&stubService{readyErr: errors.New("db down")}, metrics.New(), "test")
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestCreateNote(t *testing.T) {
	h := NewHandler(&stubService{}, metrics.New(), "test")
	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"title":"hello","content":"world"}`))
	rec := httptest.NewRecorder()

	h.CreateNote(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestListNotes(t *testing.T) {
	h := NewHandler(&stubService{
		notes: []domain.Note{{ID: 1, Title: "a", Content: "b", CreatedAt: time.Now().UTC()}},
	}, metrics.New(), "test")
	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	rec := httptest.NewRecorder()

	h.ListNotes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
}
