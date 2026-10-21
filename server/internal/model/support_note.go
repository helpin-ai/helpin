package model

import (
	"encoding/json"
	"time"
)

// UpdateSupportNoteRequest edits an existing note without creating a new message.
// Omitted attachment_ids preserve files; an empty array removes them.
type UpdateSupportNoteRequest struct {
	Content           string    `json:"content"`
	AttachmentIDs     *[]string `json:"attachment_ids,omitempty"`
	ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
}

// NoteEditedAt reads the explicit edit timestamp; delivery/metadata updates do
// not imply that the message text was edited.
func (m SupportMessage) NoteEditedAt() *time.Time {
	var metadata struct {
		EditedAt *time.Time `json:"edited_at"`
	}
	if !m.IsInternal || json.Unmarshal([]byte(m.Metadata), &metadata) != nil {
		return nil
	}
	return metadata.EditedAt
}
