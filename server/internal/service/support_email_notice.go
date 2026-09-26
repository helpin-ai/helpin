package service

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// A machine-generated header alone does not mean the message is disposable:
// alerts, delivery failures and forwarded support forms can require attention.
// Require both an automation signal and a standalone absence statement. Unknown
// languages and ambiguous requests remain in the human inbox.
func inboundEmailIsAbsenceNotice(payload model.PostmarkInboundPayload, content, reason string) bool {
	if reason != "automatic_reply" && reason != "automated_message" {
		return false
	}
	if isProviderForwardingConfirmation(payload) {
		return false
	}
	automated := false
	for _, h := range payload.Headers {
		value := strings.ToLower(strings.TrimSpace(h.Value))
		switch strings.ToLower(strings.TrimSpace(h.Name)) {
		case "auto-submitted":
			token := strings.TrimSpace(strings.SplitN(value, ";", 2)[0])
			automated = automated || token == "auto-replied" || token == "auto-generated"
		case "x-autoreply", "x-autorespond", "x-auto-response":
			automated = automated || (value != "" && value != "no" && value != "false")
		// These need their own handling even if an autoresponder header is present.
		case "list-id":
			if value != "" {
				return false
			}
		case "precedence":
			if value == "bulk" || value == "list" || value == "junk" {
				return false
			}
		case "return-path":
			if value == "<>" {
				return false
			}
		case "content-type":
			if strings.Contains(value, "report") || strings.Contains(value, "delivery-status") || strings.Contains(value, "disposition-notification") {
				return false
			}
		}
	}
	if !automated {
		return false
	}
	body := strings.ToLower(strings.Join(strings.Fields(content), " "))
	if strings.Contains(body, "?") {
		return false
	}
	// A customer forwarding/quoting an absence notice is still a customer request.
	for _, marker := range []string{"forwarded message", "original message", "please help", "i need help", "please fix", "please cancel", "please refund"} {
		if strings.Contains(body, marker) {
			return false
		}
	}
	for _, phrase := range []string{"i am out of the office", "i'm out of the office", "i am currently out of the office", "i'm currently out of the office", "i am currently away from the office", "i am on vacation", "i'm on vacation", "i am on annual leave", "i'm on annual leave"} {
		if strings.Contains(body, phrase) {
			return true
		}
	}
	return false
}

// Read the persisted decision so ingestion and retries do not reclassify it.
func inboundEmailHasAbsenceNotice(metadata string) bool {
	var value struct {
		Kind string `json:"email_notice_kind"`
	}
	return json.Unmarshal([]byte(metadata), &value) == nil && value.Kind == "out_of_office"
}
