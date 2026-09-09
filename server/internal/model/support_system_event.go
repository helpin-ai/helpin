package model

// SupportSystemEventType enumerates the distinct kinds of system messages we
// persist on a support conversation. Every system message (rows with
// message_type='system') must carry one of these values so renderers can
// branch on intent instead of keyword-matching prose.
//
// Visibility is governed by is_internal — SystemEventType only describes
// what happened, not who sees it.
type SupportSystemEventType = string

const (
	// SystemEventTeammateJoined — first non-internal reply by a given
	// teammate on the conversation. Widget-visible; matches Intercom's
	// "Jarek joined the conversation" pill.
	SystemEventTeammateJoined SupportSystemEventType = "teammate_joined"

	// SystemEventAssigned — one user assigns the conversation to another
	// user. Admin-only (is_internal=true).
	SystemEventAssigned SupportSystemEventType = "assigned"

	// SystemEventUnassigned — conversation moved back to unassigned.
	// Admin-only.
	SystemEventUnassigned SupportSystemEventType = "unassigned"

	// SystemEventTook — user self-assigns the conversation. Admin-only.
	SystemEventTook SupportSystemEventType = "took"

	// SystemEventAgentAssigned — an AI agent is assigned to the
	// conversation. Admin-only.
	SystemEventAgentAssigned SupportSystemEventType = "agent_assigned"

	// SystemEventMailboxMoved — manual move of the conversation to a
	// different mailbox.
	SystemEventMailboxMoved SupportSystemEventType = "mailbox_moved"

	// SystemEventTriageRouted — AI triage auto-routed the conversation to
	// a mailbox.
	SystemEventTriageRouted SupportSystemEventType = "triage_routed"

	// SystemEventTriageDismissed — teammate dismissed a triage routing
	// suggestion.
	SystemEventTriageDismissed SupportSystemEventType = "triage_dismissed"

	// SystemEventAIEscalated — AI decided to escalate to a human teammate
	// (low confidence, stuck, action unavailable, etc.). Internal-only;
	// the customer-facing escalation reply is sent as a separate public
	// AI reply.
	SystemEventAIEscalated      SupportSystemEventType = "ai_escalated"
	SystemEventDelayedTeamReply SupportSystemEventType = "delayed_team_reply"

	// SystemEventCustomerRequestedHuman — customer explicitly asked to
	// talk to a human (e.g. "I want to talk to a person"). Internal-only
	// counterpart to SystemEventAIEscalated.
	SystemEventCustomerRequestedHuman SupportSystemEventType = "customer_requested_human"

	// SystemEventResolved — conversation marked resolved.
	SystemEventResolved SupportSystemEventType = "resolved"

	// SystemEventReopened — conversation reopened from resolved.
	SystemEventReopened SupportSystemEventType = "reopened"

	// SystemEventClosed — conversation closed (e.g., spam).
	SystemEventClosed SupportSystemEventType = "closed"

	// SystemEventEmailRecipientsUpdated — primary email recipient or copied
	// recipients changed. Admin-only.
	SystemEventEmailRecipientsUpdated SupportSystemEventType = "email_recipients_updated"

	// SystemEventTagAdded — user tag added to the conversation. Admin-only.
	SystemEventTagAdded SupportSystemEventType = "tag_added"

	// SystemEventTagRemoved — user tag removed from the conversation.
	// Admin-only.
	SystemEventTagRemoved SupportSystemEventType = "tag_removed"

	// SystemEventTaskCreated — PM task created from the conversation.
	// Admin-only.
	SystemEventTaskCreated SupportSystemEventType = "task_created"
)

// allSupportSystemEventTypes is the authoritative set of valid event types.
// Kept private so callers go through IsValidSupportSystemEventType.
var allSupportSystemEventTypes = map[SupportSystemEventType]struct{}{
	SystemEventTeammateJoined:         {},
	SystemEventAssigned:               {},
	SystemEventUnassigned:             {},
	SystemEventTook:                   {},
	SystemEventAgentAssigned:          {},
	SystemEventMailboxMoved:           {},
	SystemEventTriageRouted:           {},
	SystemEventTriageDismissed:        {},
	SystemEventAIEscalated:            {},
	SystemEventDelayedTeamReply:       {},
	SystemEventCustomerRequestedHuman: {},
	SystemEventResolved:               {},
	SystemEventReopened:               {},
	SystemEventClosed:                 {},
	SystemEventEmailRecipientsUpdated: {},
	SystemEventTagAdded:               {},
	SystemEventTagRemoved:             {},
	SystemEventTaskCreated:            {},
}

// IsValidSupportSystemEventType reports whether s is a recognized event type.
func IsValidSupportSystemEventType(s string) bool {
	_, ok := allSupportSystemEventTypes[s]
	return ok
}

// SupportSystemEventTypeStrPtr returns a pointer to the given constant for
// convenient assignment to the *string field on SupportMessage.
func SupportSystemEventTypeStrPtr(v SupportSystemEventType) *string {
	s := string(v)
	return &s
}
