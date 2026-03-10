package model

import "time"

// SignalSourcePayload is a unified format for signal detection inputs.
type SignalSourcePayload struct {
	SourceType    string    `json:"source_type"`    // "email", "meeting", "support"
	SourceID      string    `json:"source_id"`
	WorkspaceID   string    `json:"workspace_id"`
	ContactID     *string   `json:"contact_id"`
	DealID        *string   `json:"deal_id"`
	Subject       string    `json:"subject"`
	Body          string    `json:"body"`
	Participants  []string  `json:"participants"`
	Direction     string    `json:"direction"` // "inbound", "outbound", "bilateral"
	OccurredAt    time.Time `json:"occurred_at"`
	ThreadContext string    `json:"thread_context"` // preceding messages, truncated
}

// PayloadFromEmail builds a signal source payload from an email message.
func PayloadFromEmail(msg *CRMEmailMessage, threadSubject string) SignalSourcePayload {
	body := ""
	if msg.BodyText != nil {
		body = *msg.BodyText
	}
	if body == "" && msg.BodyHTML != nil {
		body = *msg.BodyHTML
	}
	// Truncate body to 3000 chars
	if len(body) > 3000 {
		body = body[:3000]
	}

	participants := []string{msg.FromAddress}

	return SignalSourcePayload{
		SourceType:   CRMSignalSourceEmail,
		SourceID:     msg.ID,
		WorkspaceID:  msg.WorkspaceID,
		ContactID:    msg.ContactID,
		DealID:       msg.DealID,
		Subject:      msg.Subject,
		Body:         body,
		Participants: participants,
		Direction:    msg.Direction,
		OccurredAt:   msg.SentAt,
	}
}

// PayloadFromCalendarEvent builds a signal source payload from a calendar event.
func PayloadFromCalendarEvent(event *CRMCalendarEvent) SignalSourcePayload {
	body := ""
	if event.Description != nil {
		body = *event.Description
	}
	if len(body) > 3000 {
		body = body[:3000]
	}

	return SignalSourcePayload{
		SourceType:  CRMSignalSourceMeeting,
		SourceID:    event.ID,
		WorkspaceID: event.WorkspaceID,
		DealID:      event.DealID,
		Subject:     event.Title,
		Body:        body,
		Direction:   "bilateral",
		OccurredAt:  event.StartTime,
	}
}

// PayloadFromSupportMessage builds a signal source payload from a support message.
func PayloadFromSupportMessage(msg *SupportMessage, ticket *SupportTicket) SignalSourcePayload {
	body := msg.Content
	if len(body) > 3000 {
		body = body[:3000]
	}

	direction := "inbound"
	if msg.SenderType != "customer" {
		direction = "outbound"
	}

	var contactID *string
	if ticket.CRMContactID != nil {
		contactID = ticket.CRMContactID
	}

	return SignalSourcePayload{
		SourceType:  CRMSignalSourceSupport,
		SourceID:    msg.ID,
		WorkspaceID: msg.WorkspaceID,
		ContactID:   contactID,
		Subject:     ticket.Subject,
		Body:        body,
		Direction:   direction,
		OccurredAt:  msg.CreatedAt,
	}
}
