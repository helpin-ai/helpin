package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestWritePublicImportError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			// Its own status: the import form shows the token fields on it.
			name: "a Zendesk help center that needs a token", err: fmt.Errorf("check: %w", service.ErrPublicImportNeedsToken),
			wantStatus: http.StatusPreconditionRequired, wantBody: "Zendesk",
		},
		{
			name: "an address the person can fix", err: fmt.Errorf("%w: nothing was found at this address", service.ErrPublicImportInvalid),
			wantStatus: http.StatusBadRequest, wantBody: `"nothing was found at this address"`,
		},
		{
			name: "anything else keeps its internals", err: errors.New("dial tcp 10.0.0.5:443: refused"),
			wantStatus: http.StatusBadGateway, wantBody: "could not be read",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writePublicImportError(recorder, httptest.NewRequest(http.MethodPost, "/", nil), tt.err)
			if recorder.Code != tt.wantStatus || !strings.Contains(recorder.Body.String(), tt.wantBody) || strings.Contains(recorder.Body.String(), "10.0.0.5") {
				t.Fatalf("response = %d %s, want %d containing %q", recorder.Code, recorder.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}
