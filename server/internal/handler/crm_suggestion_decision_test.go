package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCRMSuggestionRejectsMalformedApprovalEdits(t *testing.T) {
	// No service is configured: every invalid body must stop before execution.
	h := NewCRMSuggestionHandler(nil)
	for _, body := range []string{"{", "[]", "{}{}", `{"note":"` + strings.Repeat("x", 70<<10) + `"}`} {
		req := httptest.NewRequest(http.MethodPost, "/suggestions/example/accept", strings.NewReader(body))
		response := httptest.NewRecorder()
		h.Accept(response, req)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("malformed edits: %d %s", response.Code, response.Body)
		}
	}
}
