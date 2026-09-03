package model

import (
	"strings"
	"time"
)

const (
	SupportRelevanceAssignee = 1 << iota
	SupportRelevanceOpener
	SupportRelevanceMention
)

const supportListPreviewRunes = 500

// SupportConversationUserState stores the read cursor and attention state for
// one agent without changing any teammate's view of the conversation.
type SupportConversationUserState struct {
	WorkspaceID                string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID             string     `json:"conversation_id" gorm:"type:uuid;not null;primaryKey"`
	UserID                     string     `json:"user_id" gorm:"type:uuid;not null;primaryKey"`
	LastReadCustomerMessageID  *string    `json:"last_read_customer_message_id,omitempty" gorm:"type:uuid"`
	LastReadCustomerMessageAt  *time.Time `json:"last_read_customer_message_at,omitempty" gorm:"type:timestamptz"`
	UnreadCustomerMessageCount int        `json:"unread_customer_message_count" gorm:"not null;default:0"`
	ManuallyUnread             bool       `json:"manually_unread" gorm:"not null;default:false"`
	MentionedAt                *time.Time `json:"mentioned_at,omitempty" gorm:"type:timestamptz"`
	RelevanceMask              int        `json:"relevance_mask" gorm:"not null;default:0"`
	Version                    int64      `json:"version" gorm:"not null;default:0"`
	CreatedAt                  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportConversationUserState) TableName() string {
	return "support_conversation_user_states"
}

func (s SupportConversationUserState) EffectiveUnreadCount() int {
	if s.UnreadCustomerMessageCount > 0 {
		return s.UnreadCustomerMessageCount
	}
	if s.ManuallyUnread {
		return 1
	}
	return 0
}

func (s *SupportConversationUserState) ApplyCustomerReply(messageID string, createdAt time.Time) {
	if s == nil || s.RelevanceMask == 0 {
		return
	}
	if s.LastReadCustomerMessageAt != nil && compareMessageTuple(createdAt, messageID, *s.LastReadCustomerMessageAt, derefModelString(s.LastReadCustomerMessageID)) <= 0 {
		return
	}
	s.UnreadCustomerMessageCount++
	s.Version++
}

// MarkReadThrough advances a cursor monotonically. remaining is the exact
// number of customer replies committed after the rendered message.
func (s *SupportConversationUserState) MarkReadThrough(messageID string, createdAt time.Time, remaining int) bool {
	if s == nil {
		return false
	}
	s.ManuallyUnread = false
	if s.LastReadCustomerMessageAt != nil && compareMessageTuple(createdAt, messageID, *s.LastReadCustomerMessageAt, derefModelString(s.LastReadCustomerMessageID)) <= 0 {
		return false
	}
	if remaining < 0 {
		remaining = 0
	}
	s.LastReadCustomerMessageID = &messageID
	s.LastReadCustomerMessageAt = &createdAt
	s.UnreadCustomerMessageCount = remaining
	s.Version++
	return true
}

func (c *SupportConversation) ApplyMessageProjection(message SupportMessage) {
	if c == nil || message.DeletedAt.Valid || message.SystemEventType != nil {
		return
	}
	isListMessage := message.MessageType == "reply" && (!message.IsInternal || strings.TrimSpace(message.Content) != "")
	if isListMessage && tupleAfter(message.CreatedAt, message.ID, c.ListLastMessageAt, c.ListLastMessageID) {
		preview := supportListPreview(message.Content)
		c.ListLastMessageID = supportStateStringPtr(message.ID)
		c.ListLastMessageAt = supportStateTimePtr(message.CreatedAt)
		c.ListLastMessagePreview = &preview
		c.ListLastMessageIsInternal = message.IsInternal
	}
	if message.IsInternal || message.MessageType != "reply" {
		c.SupportStateVersion++
		return
	}
	if tupleAfter(message.CreatedAt, message.ID, c.LastPublicMessageAt, c.LastPublicMessageID) {
		c.LastPublicMessageID = supportStateStringPtr(message.ID)
		c.LastPublicMessageAt = supportStateTimePtr(message.CreatedAt)
		c.LastPublicSenderType = supportStateStringPtr(message.SenderType)
		c.LastPublicSenderDisplayName = cloneStringPtr(message.SenderDisplayName)
	}
	if message.SenderType == "customer" {
		if tupleAfter(message.CreatedAt, message.ID, c.LastCustomerMessageAt, c.LastCustomerMessageID) {
			c.LastCustomerMessageID = supportStateStringPtr(message.ID)
			c.LastCustomerMessageAt = supportStateTimePtr(message.CreatedAt)
		}
		c.UnansweredCustomerMessageCount++
		c.CustomerAwaitingResponse = true
	} else {
		c.UnansweredCustomerMessageCount = 0
		c.CustomerAwaitingResponse = false
	}
	c.RecomputeHumanAttention()
	c.SupportStateVersion++
}

func (c *SupportConversation) RecomputeHumanAttention() {
	if c == nil {
		return
	}
	if c.Status == SupportConversationStatusResolved || c.Status == SupportConversationStatusSpam {
		c.CustomerAwaitingResponse = false
		c.NeedsHumanReply = false
		c.UnansweredCustomerMessageCount = 0
		return
	}
	c.NeedsHumanReply = c.CustomerAwaitingResponse && !c.aiOwnsResponse()
}

func (c SupportConversation) aiOwnsResponse() bool {
	if c.HumanTakeover != nil && *c.HumanTakeover {
		return false
	}
	if c.FlowState != nil {
		return *c.FlowState == SupportConversationFlowStateAIHandling
	}
	return c.AIState != nil && *c.AIState == "pending"
}

func tupleAfter(at time.Time, id string, currentAt *time.Time, currentID *string) bool {
	if currentAt == nil {
		return true
	}
	return compareMessageTuple(at, id, *currentAt, derefModelString(currentID)) > 0
}

func compareMessageTuple(leftAt time.Time, leftID string, rightAt time.Time, rightID string) int {
	if leftAt.Before(rightAt) {
		return -1
	}
	if leftAt.After(rightAt) {
		return 1
	}
	return strings.Compare(leftID, rightID)
}

func supportListPreview(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) > supportListPreviewRunes {
		runes = runes[:supportListPreviewRunes]
	}
	return string(runes)
}

func supportStateStringPtr(value string) *string     { return &value }
func supportStateTimePtr(value time.Time) *time.Time { return &value }

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return supportStateStringPtr(*value)
}

func derefModelString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
