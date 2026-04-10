package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// TestWriteDocsError verifies that every docs sentinel error is mapped to
// the correct HTTP status code and that unknown errors fall through to a
// generic 500 without leaking their internal message.
func TestWriteDocsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string // substring assertion; empty means "don't check"
	}{
		{
			name:       "collection not found -> 404",
			err:        service.ErrDocsCollectionNotFound,
			wantStatus: http.StatusNotFound,
			wantBody:   "collection not found",
		},
		{
			name:       "space not found -> 404",
			err:        service.ErrDocsSpaceNotFound,
			wantStatus: http.StatusNotFound,
			wantBody:   "space not found",
		},
		{
			name:       "name required -> 400",
			err:        service.ErrDocsCollectionNameRequired,
			wantStatus: http.StatusBadRequest,
			wantBody:   "name is required",
		},
		{
			name:       "parent not found -> 400",
			err:        service.ErrDocsCollectionParentNotFound,
			wantStatus: http.StatusBadRequest,
			wantBody:   "parent collection not found",
		},
		{
			name:       "parent different space -> 400",
			err:        service.ErrDocsCollectionParentDifferentSpace,
			wantStatus: http.StatusBadRequest,
			wantBody:   "different space",
		},
		{
			name:       "self parent -> 400",
			err:        service.ErrDocsCollectionSelfParent,
			wantStatus: http.StatusBadRequest,
			wantBody:   "its own parent",
		},
		{
			name:       "cycle -> 400",
			err:        service.ErrDocsCollectionCycle,
			wantStatus: http.StatusBadRequest,
			wantBody:   "descendant",
		},
		{
			name:       "depth exceeded -> 400",
			err:        service.ErrDocsCollectionDepthExceeded,
			wantStatus: http.StatusBadRequest,
			wantBody:   "depth",
		},
		{
			name:       "cross workspace -> 403",
			err:        service.ErrDocsCrossWorkspace,
			wantStatus: http.StatusForbidden,
			wantBody:   "does not belong",
		},
		{
			name:       "slug taken -> 409",
			err:        service.ErrDocsCollectionSlugTaken,
			wantStatus: http.StatusConflict,
			wantBody:   "already in use",
		},
		{
			name:       "wrapped sentinel still maps via errors.Is",
			err:        fmt.Errorf("reparent failed: %w", service.ErrDocsCollectionCycle),
			wantStatus: http.StatusBadRequest,
			wantBody:   "descendant",
		},
		{
			name:       "unknown error -> 500 with generic message",
			err:        errors.New("something mysterious"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeDocsError(rec, tt.err)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
