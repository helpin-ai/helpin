package repository

import "strings"

// System agents have no personal read state. SQL NULL must be used instead of
// an empty string when comparing their absent human identity to a UUID column.
func nullableSupportUserID(userID string) *string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	return &userID
}
