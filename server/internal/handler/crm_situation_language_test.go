package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestCRMSituationHTTPUsesPlainLanguage(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		text   string
	}{
		{"changed", repository.ErrCRMSituationStale, http.StatusConflict, "This item has changed. Reload it before saving."},
		{"unknown result", repository.ErrCRMSituationExecutionPending, http.StatusConflict, "Check its status before closing this item."},
		{"unavailable action", service.ErrCRMSituationTransition, http.StatusConflict, "Reload the item to see its current status."},
		{"permission", service.ErrCRMSituationForbidden, http.StatusForbidden, "You don't have access to this item."},
		{"not found", service.ErrCRMSituationNotFound, http.StatusNotFound, "This item could not be found."},
		{"invalid input", service.ErrCRMSituationInput, http.StatusBadRequest, "Check the details and filters"},
		{"internal error", errors.New("private database details"), http.StatusInternalServerError, "This item could not be loaded or saved. Try again."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeSituationError(response, httptest.NewRequest(http.MethodGet, "/situations", nil), tt.err)
			if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.text) {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			for _, internal := range []string{"customer work", "situation", "reconcile", "private database"} {
				if strings.Contains(strings.ToLower(response.Body.String()), internal) {
					t.Fatalf("internal terminology leaked: %s", response.Body.String())
				}
			}
		})
	}
}
