package model

import "time"

// EmailQueueEntry represents a pending email in the fallback delivery queue.
type EmailQueueEntry struct {
	ConversationID     string              `json:"conversation_id"`
	WorkspaceID        string              `json:"workspace_id"`
	FireAt             time.Time           `json:"fire_at"`
	DelayRemainingSecs int                 `json:"delay_remaining_secs"`
	CustomerEmail      string              `json:"customer_email"`
	CustomerName       string              `json:"customer_name,omitempty"`
	Subject            string              `json:"subject"`
	Status             string              `json:"status"`
	MessageCount       int                 `json:"message_count"`
	Messages           []EmailQueueMessage `json:"messages"`
}

// EmailQueueMessage is a summary of a pending message in the queue.
type EmailQueueMessage struct {
	ID                string    `json:"id"`
	Content           string    `json:"content"`
	SenderDisplayName string    `json:"sender_display_name,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// EmailQueueResponse is the response for the admin email queue endpoint.
type EmailQueueResponse struct {
	Entries []EmailQueueEntry `json:"entries"`
	Total   int               `json:"total"`
}
