package model

import (
	"strings"
	"testing"
	"time"
)

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

func TestPayloadFromCalendarEventIncludesAttendeeResponses(t *testing.T) {
	contactID := "contact-1"
	event := &CRMCalendarEvent{
		ID: "event-1", WorkspaceID: "workspace-1", Title: "Pricing review",
		Status: CRMCalendarEventStatusConfirmed, StartTime: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		Description: stringPointer(`<p>Review <strong>enterprise</strong> pricing</p>`),
		ContactIDs:  CRMStringList{contactID},
		Attendees: CRMCalendarAttendees{
			{Email: "owner@example.com", Self: true, Organizer: true, ResponseStatus: "accepted"},
			{Email: "buyer@example.com", Name: "Ava", ResponseStatus: "declined"},
		},
	}

	payload := PayloadFromCalendarEvent(event)
	if payload.ContactID == nil || *payload.ContactID != contactID {
		t.Fatalf("ContactID = %v, want %q", payload.ContactID, contactID)
	}
	if payload.Direction != "bilateral" {
		t.Fatalf("Direction = %q, want bilateral", payload.Direction)
	}
	for _, want := range []string{"Review enterprise pricing", "Meeting status: confirmed", "Attendee Ava <buyer@example.com> response: declined"} {
		if !strings.Contains(payload.Body, want) {
			t.Fatalf("Body = %q, want %q", payload.Body, want)
		}
	}
}

func stringPointer(value string) *string { return &value }
