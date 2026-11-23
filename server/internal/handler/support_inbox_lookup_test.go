package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestWriteSupportConversationLookupError(t *testing.T) {
	const validID = "7f1c8a52-4b7e-4f6a-9a51-2d1f1b5a9c10"
	cases := []struct {
		name       string
		id         string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "missing or access-filtered", id: validID, err: service.ErrSupportConversationUnavailable, wantStatus: http.StatusNotFound, wantBody: `{"error":"conversation not found"}`},
		{name: "wrapped unavailable", id: validID, err: fmt.Errorf("lookup: %w", service.ErrSupportConversationUnavailable), wantStatus: http.StatusNotFound, wantBody: `{"error":"conversation not found"}`},
		{name: "malformed id", id: "not-a-uuid", err: errors.New("pq: invalid input syntax for type uuid"), wantStatus: http.StatusNotFound, wantBody: `{"error":"conversation not found"}`},
		{name: "unexpected failure stays generic", id: validID, err: errors.New("pq: connection refused"), wantStatus: http.StatusInternalServerError, wantBody: `{"error":"could not load conversation"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeSupportConversationLookupError(context.Background(), rec, "ws-1", tc.id, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tc.wantBody {
				t.Fatalf("body = %s, want %s", got, tc.wantBody)
			}
		})
	}
}
