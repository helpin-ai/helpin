package model

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
)

// SupportConversationMerge preserves an original conversation and its direct
// handling parent. Ended edges remain as immutable history intervals.
type SupportConversationMerge struct {
	ID                 string               `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID        string               `json:"workspace_id" gorm:"type:uuid;not null;index"`
	OperationID        string               `json:"operation_id" gorm:"type:uuid;not null;index"`
	SourceID           string               `json:"source_id" gorm:"type:uuid;not null;index"`
	TargetID           string               `json:"target_id" gorm:"type:uuid;not null;index"`
	ActorID            string               `json:"actor_id" gorm:"type:uuid;not null"`
	CrossCustomer      bool                 `json:"cross_customer" gorm:"not null;default:false"`
	CreatedAt          time.Time            `json:"created_at"`
	EndedAt            *time.Time           `json:"ended_at,omitempty"`
	EndedBy            *string              `json:"ended_by,omitempty" gorm:"type:uuid"`
	UnmergeOperationID *string              `json:"-" gorm:"type:uuid;index"`
	Source             *SupportConversation `json:"source,omitempty" gorm:"-"`
	Target             *SupportConversation `json:"target,omitempty" gorm:"-"`
	ActorName          string               `json:"actor_name,omitempty" gorm:"-"`
}

// TableName implements the merge ledger's storage contract.
func (SupportConversationMerge) TableName() string { return "support_conversation_merges" }

// SupportMergeMessage is an explicit history grant, written in the same
// transaction as a message. No wall-clock boundary can accidentally widen it.
type SupportMergeMessage struct {
	WorkspaceID    string `json:"-" gorm:"type:uuid;not null"`
	ConversationID string `json:"conversation_id" gorm:"type:uuid;primaryKey"`
	MessageID      string `json:"message_id" gorm:"type:uuid;primaryKey"`
	MergeID        string `json:"merge_id" gorm:"type:uuid;not null;index"`
}

// TableName implements shared history storage.
func (SupportMergeMessage) TableName() string { return "support_merge_messages" }

// SupportMergeRequest identifies a reviewed atomic operation.
type SupportMergeRequest struct {
	RestoreMailboxID *string  `json:"restore_mailbox_id,omitempty"`
	SourceIDs        []string `json:"source_ids"`
	OperationID      string   `json:"operation_id"`
	PreviewToken     string   `json:"preview_token"`
}

// SupportMergePreview contains only conversations the actor may access.
type SupportMergePreview struct {
	RestoreInboxRequired bool                  `json:"restore_inbox_required,omitempty"`
	Target               SupportConversation   `json:"target"`
	Sources              []SupportConversation `json:"sources"`
	PreviewToken         string                `json:"preview_token"`
	CrossCustomer        bool                  `json:"cross_customer"`
	Participants         []string              `json:"participants"`
	StopsAI              bool                  `json:"stops_ai"`
	Reopens              bool                  `json:"reopens"`
	TransfersAccess      bool                  `json:"transfers_access"`
}

// SupportMergeResult identifies all records invalidated by a merge or unmerge.
type SupportMergeResult struct {
	ConversationID string                     `json:"conversation_id"`
	AffectedIDs    []string                   `json:"affected_ids"`
	Merges         []SupportConversationMerge `json:"merges"`
}

// SameSupportCustomer compares durable identity, never a display name. Matching
// email is a staff merge convenience; it does not grant widget authentication.
func SameSupportCustomer(a, b *SupportConversation) bool {
	value := func(v *string) string {
		if v == nil {
			return ""
		}
		return strings.TrimSpace(*v)
	}
	if a == nil || b == nil {
		return false
	}
	ac, bc := value(a.CRMContactID), value(b.CRMContactID)
	if ac != "" && bc != "" {
		return ac == bc
	}
	if aa, ba := value(a.AnonymousID), value(b.AnonymousID); aa != "" && aa == ba {
		return true
	}
	if a.PrimaryRecipientState == SupportPrimaryRecipientStateUnconfirmed || b.PrimaryRecipientState == SupportPrimaryRecipientStateUnconfirmed {
		return false
	}
	ae, be := value(a.CustomerEmail), value(b.CustomerEmail)
	return ae != "" && strings.EqualFold(ae, be)
}

// SupportMergeRoot resolves a direct-edge graph and rejects corrupt cycles.
func SupportMergeRoot(id string, parents map[string]string) (string, error) {
	seen := make(map[string]bool)
	for parents[id] != "" {
		if seen[id] {
			return "", errors.New("conversation merge cycle")
		}
		seen[id] = true
		id = parents[id]
	}
	return id, nil
}

// SupportMergeCandidates is a scoped page; totals never include hidden records.
type SupportMergeCandidates struct {
	Data                 []SupportConversation `json:"data"`
	HasMore              bool                  `json:"has_more"`
	CrossCustomerAllowed bool                  `json:"cross_customer_allowed"`
}

// SupportMergeJob reconciles runtime cancellation and realtime refresh after a
// committed routing change, including recovery after an API process restart.
type SupportMergeJob struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	WorkspaceID    string    `gorm:"type:uuid;not null;index"`
	ConversationID string    `gorm:"type:uuid;not null"`
	ActorID        string    `gorm:"type:uuid;not null"`
	RunID          *string   `gorm:"type:uuid"`
	AvailableAt    time.Time `gorm:"not null;index"`
	LeaseUntil     *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
}

// TableName returns the merge reconciliation outbox.
func (SupportMergeJob) TableName() string { return "support_merge_jobs" }

// SupportMergeEmailAudience includes confirmed customers and explicitly selected
// CC recipients. Observed senders and BCC addresses never become participants.
func SupportMergeEmailAudience(conversations []SupportConversation) []string {
	emails := []string{}
	for _, c := range conversations {
		if c.EmailUnsubscribed || c.AnonymizedAt != nil || c.PrimaryRecipientState == SupportPrimaryRecipientStateUnconfirmed {
			continue
		}
		addresses := append([]string{}, c.EmailCC...)
		if c.CustomerEmail != nil {
			addresses = append(addresses, *c.CustomerEmail)
		}
		for _, raw := range addresses {
			email := strings.ToLower(strings.TrimSpace(raw))
			if email != "" && !slices.Contains(emails, email) {
				emails = append(emails, email)
			}
		}
	}
	slices.Sort(emails)
	return emails
}

// SharedSupportAttachmentVisible excludes inline files found only in quoted
// email history. Ordinary newly attached files remain part of the new reply.
func SharedSupportAttachmentVisible(m *SupportMessage, a SupportAttachmentPayload) bool {
	if a.ContentID == "" {
		return true
	}
	var projection struct {
		Text *string `json:"merge_public_text"`
	}
	if json.Unmarshal([]byte(m.Metadata), &projection) != nil || projection.Text == nil {
		return true
	}
	cid := strings.Trim(strings.TrimSpace(a.ContentID), "<>")
	return cid != "" && strings.Contains(strings.ToLower(*projection.Text), "cid:"+strings.ToLower(cid))
}
