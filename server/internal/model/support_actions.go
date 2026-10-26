package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// SupportBulkActionLimit bounds how many conversations one bulk request changes.
const SupportBulkActionLimit = 100

// SupportConversationActions is a set of changes a macro or a bulk action
// applies to a conversation. Nil fields are left alone.
type SupportConversationActions struct {
	// Status is open, waiting_on_customer or resolved.
	Status   *string `json:"status,omitempty"`
	Priority *string `json:"priority,omitempty"`
	// AssignUserID assigns a teammate; an empty string unassigns.
	AssignUserID *string `json:"assign_user_id,omitempty"`
	// MailboxID moves the conversation; "shared" is the main inbox.
	MailboxID    *string  `json:"mailbox_id,omitempty"`
	AddTagIDs    []string `json:"add_tag_ids,omitempty"`
	RemoveTagIDs []string `json:"remove_tag_ids,omitempty"`
	// SnoozeMinutes snoozes the conversation for this long.
	SnoozeMinutes *int `json:"snooze_minutes,omitempty"`
}

// Empty reports whether the set changes nothing.
func (a SupportConversationActions) Empty() bool {
	return a.Status == nil && a.Priority == nil && a.AssignUserID == nil && a.MailboxID == nil &&
		len(a.AddTagIDs) == 0 && len(a.RemoveTagIDs) == 0 && a.SnoozeMinutes == nil
}

// Value stores the actions as JSON.
func (a SupportConversationActions) Value() (driver.Value, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// Scan reads the actions from JSON.
func (a *SupportConversationActions) Scan(src any) error {
	*a = SupportConversationActions{}
	var raw []byte
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("scan support conversation actions from %T", src)
	}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, a)
}

// SupportBulkActionRequest applies one set of actions to many conversations.
type SupportBulkActionRequest struct {
	ConversationIDs []string                   `json:"conversation_ids"`
	Actions         SupportConversationActions `json:"actions"`
}

// SupportActionResult reports what happened to one conversation.
type SupportActionResult struct {
	ConversationID string   `json:"conversation_id"`
	Applied        []string `json:"applied"`
	Error          string   `json:"error,omitempty"`
}

// SupportBulkActionResponse lists the result for every requested conversation.
type SupportBulkActionResponse struct {
	Results   []SupportActionResult `json:"results"`
	Succeeded int                   `json:"succeeded"`
	Failed    int                   `json:"failed"`
}

// SupportMacroApplyResponse is the conversation after a macro ran.
type SupportMacroApplyResponse struct {
	Conversation *SupportConversation `json:"conversation"`
	Result       SupportActionResult  `json:"result"`
}
