package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPortalRequestListRowCarriesOnlyCustomerSafeFields(t *testing.T) {
	due := time.Date(2026, 10, 28, 15, 0, 0, 0, time.UTC)
	request := model.SupportPortalRequest{Reference: "req_1", Subject: "Export", Status: model.PortalRequestStatusActive,
		ReplyExpectedBy: &due, ConversationID: "internal-conversation-id", OwnerIdentityID: "identity-1"}
	body, err := json.Marshal(portalRequestListRow(request, "identity-1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"reply_expected_by":"2026-10-28T15:00:00Z"`) {
		t.Fatalf("reply time missing: %s", body)
	}
	if strings.Contains(string(body), "internal-conversation-id") || strings.Contains(string(body), "identity-1") {
		t.Fatalf("internal ids leaked: %s", body)
	}
	request.ReplyExpectedBy = nil
	if body, _ := json.Marshal(portalRequestListRow(request, "identity-1")); strings.Contains(string(body), "reply_expected_by") {
		t.Fatalf("empty reply time sent: %s", body)
	}
}
