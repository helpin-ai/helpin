package model

import "testing"

func TestPayloadFromSupportMessagePreservesCompanyAndConversationEvidence(t *testing.T) {
	companyID, contactID := "company-1", "contact-1"
	message := &SupportMessage{
		ID: "message-1", WorkspaceID: "ws-1", ConversationID: "conversation-1",
		SenderType: "customer", Content: "We need enterprise pricing.",
	}
	conversation := &SupportConversation{
		ID: "conversation-1", WorkspaceID: "ws-1", Subject: "Pricing",
		CRMCompanyID: &companyID, CRMContactID: &contactID,
	}

	payload := PayloadFromSupportMessage(message, conversation)
	if payload.CompanyID == nil || *payload.CompanyID != companyID {
		t.Fatalf("company_id = %v", payload.CompanyID)
	}
	if payload.ContactID == nil || *payload.ContactID != contactID {
		t.Fatalf("contact_id = %v", payload.ContactID)
	}
	if payload.SourceThreadID == nil || *payload.SourceThreadID != conversation.ID {
		t.Fatalf("source_thread_id = %v", payload.SourceThreadID)
	}
	if payload.SourceID != message.ID || payload.Body != message.Content {
		t.Fatalf("payload = %#v", payload)
	}
}
