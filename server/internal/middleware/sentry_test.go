package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestSentryRequestContext_PopulatesScope(t *testing.T) {
	hub := sentry.NewHub(nil, sentry.NewScope())

	var event *sentry.Event
	handler := SentryRequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentHub := sentry.GetHubFromContext(r.Context())
		if currentHub == nil {
			t.Fatal("expected hub on context")
		}
		scope := currentHub.Scope()
		if scope == nil {
			t.Fatal("expected scope on hub")
		}
		event = scope.ApplyToEvent(&sentry.Event{}, nil, nil)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	ctx := req.Context()
	ctx = sentry.SetHubOnContext(ctx, hub)
	ctx = context.WithValue(ctx, chimiddleware.RequestIDKey, "req-123")
	ctx = WithWorkspaceID(ctx, "ws-123")
	ctx = WithUserID(ctx, "user-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if event == nil {
		t.Fatal("expected event to be populated")
	}
	if got := event.Tags["request_id"]; got != "req-123" {
		t.Fatalf("request_id = %q, want %q", got, "req-123")
	}
	if got := event.Tags["workspace_id"]; got != "ws-123" {
		t.Fatalf("workspace_id = %q, want %q", got, "ws-123")
	}
	if got := event.Tags["user_id"]; got != "user-123" {
		t.Fatalf("user_id tag = %q, want %q", got, "user-123")
	}
	if got := event.User.ID; got != "user-123" {
		t.Fatalf("user.id = %q, want %q", got, "user-123")
	}
}
