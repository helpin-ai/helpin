package model

import "time"

type DockChatVisibility string

const (
	DockChatVisibilityPrivate   DockChatVisibility = "private"
	DockChatVisibilityModule    DockChatVisibility = "module"
	DockChatVisibilityWorkspace DockChatVisibility = "workspace"
)

// DockChat is one dock conversation. The creator owns its settings and turns,
// while visibility controls which workspace teammates can read it.
// Each chat is backed by an
// agent-runtime chat-mode run; ActiveRunID points at the agent_runs row for
// the current backing run (nil until the first message, and updated when an
// idle-expired chat is continued through a successor run). All runs that ever
// backed the chat carry AgentRun.DockChatID, so the full history is a real FK
// chain rather than a heuristic match.
type DockChat struct {
	ID                    string             `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string             `json:"workspace_id" gorm:"type:uuid;not null;index:idx_dock_chats_ws_user,priority:1;uniqueIndex:idx_dock_chats_support_conversation,priority:1,where:archived_at IS NULL"`
	UserID                string             `json:"user_id" gorm:"type:uuid;not null;index:idx_dock_chats_ws_user,priority:2;uniqueIndex:idx_dock_chats_support_conversation,priority:2,where:archived_at IS NULL"`
	Title                 string             `json:"title"`
	Visibility            DockChatVisibility `json:"visibility" gorm:"type:text;not null;default:'private';index"`
	ModuleID              *ModuleID          `json:"module_id,omitempty" gorm:"type:text;index"`
	SupportConversationID *string            `json:"support_conversation_id,omitempty" gorm:"type:uuid;index;uniqueIndex:idx_dock_chats_support_conversation,priority:3,where:archived_at IS NULL"`
	ActiveRunID           *string            `json:"active_run_id,omitempty" gorm:"type:uuid;index"`
	NextMessageSequence   int64              `json:"-" gorm:"->"`
	// ActiveRunStatus is a read-only projection used by chat roster surfaces.
	// It is hydrated from ActiveRunID and is not stored on the chat row.
	ActiveRunStatus string     `json:"active_run_status,omitempty" gorm:"-"`
	LastMessageAt   *time.Time `json:"last_message_at,omitempty"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the dock chats table name.
func (DockChat) TableName() string { return "dock_chats" }

// CreateDockChatRequest is the payload for creating a dock chat.
type CreateDockChatRequest struct {
	Title                 string              `json:"title"`
	SupportConversationID *string             `json:"support_conversation_id,omitempty"`
	Visibility            *DockChatVisibility `json:"visibility,omitempty"`
	ModuleID              *ModuleID           `json:"module_id,omitempty"`
}

// UpdateDockChatRequest is the payload for renaming or archiving a dock chat.
type UpdateDockChatRequest struct {
	Title      *string             `json:"title,omitempty"`
	Archived   *bool               `json:"archived,omitempty"`
	Visibility *DockChatVisibility `json:"visibility,omitempty"`
}

// SendDockChatMessageRequest is the payload for a user chat turn.
type SendDockChatMessageRequest struct {
	ClientMessageID string                 `json:"client_message_id"`
	Content         string                 `json:"content"`
	PageContext     map[string]interface{} `json:"page_context,omitempty"`
	References      []DockEntityReference  `json:"references,omitempty"`
	AttachmentIDs   []string               `json:"attachment_ids,omitempty"`
}

// DockEntityReference identifies supplemental workspace context attached to a dock turn.
type DockEntityReference struct {
	EntityType   string `json:"entity_type"`
	EntityID     string `json:"entity_id"`
	DisplayTitle string `json:"display_title"`
}

// GenerateDockChatTitleRequest is the first user turn used to name a chat.
type GenerateDockChatTitleRequest struct {
	Content     string                 `json:"content"`
	PageContext map[string]interface{} `json:"page_context,omitempty"`
}

// DockChatDetail is the read model returned for a single chat: the chat row
// plus a summary of its current backing run and the plans launched from it.
type DockChatDetail struct {
	Chat            DockChat                `json:"chat"`
	Run             *AgentRun               `json:"run,omitempty"`
	AcceptedMessage *AgentRunMessage        `json:"accepted_message,omitempty"`
	PlanIDs         []string                `json:"plan_ids"`
	Plans           []CommandBarPlanSummary `json:"plans,omitempty"`
}

// DockChatMessageListResponse is one stable cursor page of persisted messages
// spanning every agent run that has backed the chat.
type DockChatMessageListResponse struct {
	Messages   []AgentRunMessage `json:"messages"`
	NextBefore *int64            `json:"next_before,omitempty"`
}

// DockChatWorkDetailResponse contains the historical payload omitted from a
// compact completed turn until its disclosure is expanded.
type DockChatWorkDetailResponse struct {
	Messages []AgentRunMessage `json:"messages"`
}

// DockChatListResponse is one stable cursor page of the user's conversations.
type DockChatListResponse struct {
	Chats      []DockChat `json:"chats"`
	NextCursor *string    `json:"next_cursor,omitempty"`
}
