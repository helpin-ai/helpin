package model

import "time"

// Kinds of items the team shared with a customer in a support conversation.
const (
	SupportSharedItemKindDoc  = "doc"
	SupportSharedItemKindFile = "file"
)

// SupportSharedItem is a help-center article or a file the team sent to the
// customer in a non-internal conversation reply.
type SupportSharedItem struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	FileType  string    `json:"file_type,omitempty"`
	FileSize  int64     `json:"file_size,omitempty"`
	MessageID string    `json:"message_id"`
	SharedAt  time.Time `json:"shared_at"`
}
