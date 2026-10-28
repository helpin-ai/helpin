package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestVisitorHistoryExcludesInternalFields(t *testing.T) {
	msgs := []model.SupportMessage{{Content: "Public answer", EmailBCC: model.DocsStringArray{"private-audit@example.invalid"}, EmailDeliveryError: "INTERNAL_DELIVERY_DIAGNOSTIC", Metadata: `{"ai_model":"INTERNAL_MODEL","ai_validation_reasons":["INTERNAL_VALIDATION"],"link_security":[{"status":"safe"}]}`}}
	b, err := json.Marshal(widgetSafeSupportMessages(msgs))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"private-audit@example.invalid", "INTERNAL_DELIVERY_DIAGNOSTIC", "INTERNAL_MODEL", "INTERNAL_VALIDATION"} {
		if strings.Contains(string(b), marker) {
			t.Errorf("visitor history leaked: %s", marker)
		}
	}
	if strings.Contains(string(b), "link_security") {
		t.Fatal("HTTP link security guard failed")
	}

}

func TestWidgetErrorsDoNotDiscloseProviderDetails(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/widget/support/messages", nil)
	rec := httptest.NewRecorder()
	writeWidgetError(rec, req, http.StatusInternalServerError, errors.New("database query PRIVATE_TABLE failed with PRIVATE_TOKEN"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "PRIVATE_") {
		t.Fatalf("unsafe error: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	writeWidgetError(rec, req, http.StatusBadRequest, errors.New("invalid email address"))
	if !strings.Contains(rec.Body.String(), "invalid email address") {
		t.Fatal("safe validation lost")
	}
}

func TestWidgetErrorsExplainRefusedFiles(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/widget/support/attachments/one/confirm", nil)
	rec := httptest.NewRecorder()
	refused := &service.SupportAttachmentRejectedError{Message: "photo.jpg can't be sent because its contents don't match its file type"}
	writeWidgetError(rec, req, http.StatusBadRequest, fmt.Errorf("confirm upload: %w", refused))
	var body struct{ Error, Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest || body.Error != refused.Message {
		t.Fatalf("visitor did not get the reason: %d %s", rec.Code, rec.Body.String())
	}
	if body.Code != service.SupportAttachmentRefusedCode {
		t.Fatalf("code = %q, want the widget told not to offer a retry", body.Code)
	}

	rec = httptest.NewRecorder()
	unfinished := &service.SupportAttachmentRejectedError{Message: "photo.jpg did not finish uploading. Please try again.", Retryable: true}
	writeWidgetError(rec, req, http.StatusBadRequest, unfinished)
	if strings.Contains(rec.Body.String(), "code") || !strings.Contains(rec.Body.String(), unfinished.Message) {
		t.Fatalf("an unfinished upload must stay retryable: %s", rec.Body.String())
	}
}
