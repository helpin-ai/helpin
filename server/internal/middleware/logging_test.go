package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// captureHandler implements slog.Handler and captures log records for testing.
// It stores both the Record and any attributes extracted from the Record's Attrs.
type captureHandler struct {
	mu      sync.Mutex
	records []capturedRecord
}

type capturedRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]slog.Value
}

func newCaptureHandler() *captureHandler {
	return &captureHandler{}
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	cr := capturedRecord{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   make(map[string]slog.Value),
	}
	r.Attrs(func(a slog.Attr) bool {
		cr.Attrs[a.Key] = a.Value
		return true
	})
	h.records = append(h.records, cr)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(_ string) slog.Handler {
	return h
}

func (h *captureHandler) getRecords() []capturedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]capturedRecord, len(h.records))
	copy(result, h.records)
	return result
}

func setupLogger() (*captureHandler, func()) {
	ch := newCaptureHandler()
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(ch))
	return ch, func() {
		slog.SetDefault(oldDefault)
	}
}

func TestRequestLogger_200_LogsAtInfoLevel(t *testing.T) {
	ch, restore := setupLogger()
	defer restore()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLogger(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	records := ch.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}

	r := records[0]
	if r.Level != slog.LevelInfo {
		t.Errorf("level = %v, want %v", r.Level, slog.LevelInfo)
	}

	if v, ok := r.Attrs["method"]; !ok || v.String() != "GET" {
		t.Errorf("method = %v (found=%v), want GET", v, ok)
	}
	if v, ok := r.Attrs["path"]; !ok || v.String() != "/api/workspaces" {
		t.Errorf("path = %v (found=%v), want /api/workspaces", v, ok)
	}
	if v, ok := r.Attrs["status"]; !ok || v.Int64() != 200 {
		t.Errorf("status = %v (found=%v), want 200", v, ok)
	}
}

func TestRequestLogger_404_LogsAtWarnLevel(t *testing.T) {
	ch, restore := setupLogger()
	defer restore()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := RequestLogger(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	records := ch.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}

	r := records[0]
	if r.Level != slog.LevelWarn {
		t.Errorf("level = %v, want %v", r.Level, slog.LevelWarn)
	}

	if v, ok := r.Attrs["status"]; !ok || v.Int64() != 404 {
		t.Errorf("status = %v (found=%v), want 404", v, ok)
	}
}

func TestRequestLogger_500_LogsAtErrorLevel(t *testing.T) {
	ch, restore := setupLogger()
	defer restore()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	handler := RequestLogger(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/crash", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	records := ch.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}

	r := records[0]
	if r.Level != slog.LevelError {
		t.Errorf("level = %v, want %v", r.Level, slog.LevelError)
	}

	if v, ok := r.Attrs["status"]; !ok || v.Int64() != 500 {
		t.Errorf("status = %v (found=%v), want 500", v, ok)
	}
}

func TestRequestLogger_WithUserIDInContext(t *testing.T) {
	ch, restore := setupLogger()
	defer restore()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLogger(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	ctx := WithUserID(req.Context(), "user-logged-in")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	records := ch.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}

	r := records[0]
	v, ok := r.Attrs["user_id"]
	if !ok {
		t.Fatal("expected user_id attribute in log record")
	}
	if v.String() != "user-logged-in" {
		t.Errorf("user_id = %q, want %q", v.String(), "user-logged-in")
	}
}

func TestRequestLogger_DurationIsPresent(t *testing.T) {
	ch, restore := setupLogger()
	defer restore()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLogger(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	records := ch.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}

	r := records[0]
	_, ok := r.Attrs["duration_ms"]
	if !ok {
		t.Fatal("expected duration_ms attribute in log record")
	}
}
