package service

import (
	"encoding/json"
	"mime"
	"net/mail"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Save suppression signals so retries and delayed work make the same decision.
func inboundEmailAIMetadata(raw string, payload model.PostmarkInboundPayload, content string) string {
	metadata := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	reason := ""
	if isProviderForwardingConfirmation(payload) || model.SupportEmailSuppressesAI(&model.SupportMessage{Content: content}) {
		reason = "automatic_reply"
	}
	for _, header := range payload.Headers {
		value := strings.ToLower(strings.TrimSpace(header.Value))
		switch strings.ToLower(strings.TrimSpace(header.Name)) {
		case "auto-submitted":
			token := strings.TrimSpace(strings.SplitN(value, ";", 2)[0])
			if token != "" && token != "no" {
				reason = "automatic_reply"
			}
		case "x-autoreply", "x-autorespond", "x-auto-response", "x-loop":
			if value != "" && value != "no" && value != "false" {
				reason = "automatic_reply"
			}
		case "return-path":
			if value == "<>" {
				reason = "delivery_report"
			}
		case "content-type":
			mediaType, _, _ := mime.ParseMediaType(value)
			switch mediaType {
			case "multipart/report", "message/delivery-status", "message/disposition-notification", "message/global-delivery-status", "message/global-disposition-notification":
				reason = "delivery_report"
			}
		case "list-id":
			if value != "" {
				reason = "mailing_list"
			}
		case "precedence":
			if value == "bulk" || value == "list" || value == "junk" {
				reason = "mailing_list"
			}
		}
	}
	sender := payload.FromFull.Email
	if sender == "" {
		if address, err := mail.ParseAddress(payload.From); err == nil {
			sender = address.Address
		}
	}
	local := strings.ToLower(strings.SplitN(sender, "@", 2)[0])
	if local == "mailer-daemon" || local == "postmaster" {
		reason = "delivery_report"
	}
	if postmarkInboundSpamSignalsFromHeaders(payload.Headers).shouldAutoSpamNewConversation() {
		reason = "spam"
	}
	metadata["email_auto_reply"] = reason == "automatic_reply"
	metadata["email_ai_suppression_reason"] = reason
	metadata["email_ai_request"] = reason == ""
	encoded, _ := json.Marshal(metadata)
	return string(encoded)
}
