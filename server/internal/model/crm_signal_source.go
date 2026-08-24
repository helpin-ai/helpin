package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmtext"
)

// SignalParticipant captures a normalized source participant.
type SignalParticipant struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role"`
}

// SignalSourcePayload is a unified format for signal detection inputs.
type SignalSourcePayload struct {
	SourceType             string              `json:"source_type"` // email, meeting, support, note, or call
	SourceID               string              `json:"source_id"`
	SourceThreadID         *string             `json:"source_thread_id,omitempty"`
	SourceThreadExternalID *string             `json:"source_thread_external_id,omitempty"`
	WorkspaceID            string              `json:"workspace_id"`
	ContactID              *string             `json:"contact_id"`
	DealID                 *string             `json:"deal_id"`
	CompanyID              *string             `json:"company_id"`
	Subject                string              `json:"subject"`
	Body                   string              `json:"body"`
	Participants           []SignalParticipant `json:"participants"`
	Direction              string              `json:"direction"` // "inbound", "outbound", "bilateral"
	OccurredAt             time.Time           `json:"occurred_at"`
	ThreadContext          string              `json:"thread_context"` // preceding messages, truncated
}

// PayloadFromEmail builds a signal source payload from an email message.
func PayloadFromEmail(msg *CRMEmailMessage, threadSubject string) SignalSourcePayload {
	body := crmtext.PreferredBody(stringValue(msg.BodyText), stringValue(msg.BodyHTML), 3000)

	participants := []SignalParticipant{
		{
			Email: msg.FromAddress,
			Name:  stringValue(msg.FromName),
			Role:  CRMEmailParticipantRoleFrom,
		},
	}
	for _, email := range parseAddressJSONArray(msg.ToAddresses) {
		participants = append(participants, SignalParticipant{Email: email, Role: CRMEmailParticipantRoleTo})
	}
	for _, email := range parseAddressJSONArray(msg.CCAddresses) {
		participants = append(participants, SignalParticipant{Email: email, Role: CRMEmailParticipantRoleCC})
	}

	subject := threadSubject
	if subject == "" {
		subject = msg.Subject
	}

	return SignalSourcePayload{
		SourceType:     CRMSignalSourceEmail,
		SourceID:       msg.ID,
		SourceThreadID: msg.ThreadID,
		WorkspaceID:    msg.WorkspaceID,
		ContactID:      msg.ContactID,
		DealID:         msg.DealID,
		Subject:        subject,
		Body:           body,
		Participants:   participants,
		Direction:      msg.Direction,
		OccurredAt:     msg.SentAt,
	}
}

// PayloadFromCalendarEvent builds a signal source payload from a calendar event.
func PayloadFromCalendarEvent(event *CRMCalendarEvent) SignalSourcePayload {
	if event == nil {
		return SignalSourcePayload{}
	}
	lines := make([]string, 0, len(event.Attendees)+3)
	if description := crmtext.PlainText(stringValue(event.Description)); description != "" {
		lines = append(lines, description)
	}
	lines = append(lines,
		fmt.Sprintf("Meeting status: %s", strings.TrimSpace(event.Status)),
		fmt.Sprintf("Meeting starts: %s", event.StartTime.UTC().Format(time.RFC3339)),
	)
	participants := make([]SignalParticipant, 0, len(event.Attendees))
	direction := "bilateral"
	for _, attendee := range event.Attendees {
		emailAddress := strings.ToLower(strings.TrimSpace(attendee.Email))
		if emailAddress == "" {
			continue
		}
		role := CRMEmailParticipantRoleTo
		if attendee.Organizer {
			role = CRMEmailParticipantRoleFrom
		}
		participants = append(participants, SignalParticipant{Email: emailAddress, Name: strings.TrimSpace(attendee.Name), Role: role})
		response := strings.TrimSpace(attendee.ResponseStatus)
		if response != "" && !attendee.Self {
			label := emailAddress
			if strings.TrimSpace(attendee.Name) != "" {
				label = strings.TrimSpace(attendee.Name) + " <" + emailAddress + ">"
			}
			lines = append(lines, fmt.Sprintf("Attendee %s response: %s", label, response))
		}
	}
	var contactID *string
	if len(event.ContactIDs) > 0 && strings.TrimSpace(event.ContactIDs[0]) != "" {
		value := strings.TrimSpace(event.ContactIDs[0])
		contactID = &value
	}
	body := crmtext.Truncate(strings.Join(lines, "\n"), 3000)

	return SignalSourcePayload{
		SourceType:   CRMSignalSourceMeeting,
		SourceID:     event.ID,
		WorkspaceID:  event.WorkspaceID,
		ContactID:    contactID,
		DealID:       event.DealID,
		Subject:      event.Title,
		Body:         body,
		Participants: participants,
		Direction:    direction,
		OccurredAt:   event.StartTime,
	}
}

// PayloadFromSupportMessage builds a signal source payload from a support message.
func PayloadFromSupportMessage(msg *SupportMessage, ticket *SupportConversation) SignalSourcePayload {
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
		SourceType:     CRMSignalSourceSupport,
		SourceID:       msg.ID,
		WorkspaceID:    msg.WorkspaceID,
		ContactID:      contactID,
		CompanyID:      ticket.CRMCompanyID,
		Subject:        ticket.Subject,
		Body:           body,
		Direction:      direction,
		OccurredAt:     msg.CreatedAt,
		SourceThreadID: &ticket.ID,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func parseAddressJSONArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}
