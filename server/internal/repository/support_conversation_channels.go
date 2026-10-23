package repository

import (
	"context"
	"sort"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Channels includes the original source and channels used by public messages,
// independent of timeline pagination. EmailNotifiedAt confirms outbound delivery.
func (r *SupportConversationRepository) Channels(ctx context.Context, workspaceID, conversationID, source string) ([]string, error) {
	var rows []struct {
		ViaChannel string
		SenderType string
		Emailed    bool
	}
	err := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Select("DISTINCT COALESCE(via_channel, '') AS via_channel, sender_type, email_notified_at IS NOT NULL AS emailed").
		Where("workspace_id = ? AND conversation_id = ? AND is_internal = ? AND deleted_at IS NULL", workspaceID, conversationID, false).
		Where("sender_type <> ?", "system").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	if source != "" {
		seen[source] = true
	}
	for _, row := range rows {
		if row.Emailed {
			seen["email"] = true
		}
		if row.ViaChannel != "" && (row.ViaChannel != "email" || row.SenderType == "customer" || row.Emailed) {
			seen[row.ViaChannel] = true
		}
	}
	channels := make([]string, 0, len(seen))
	// Keep the original channel first and all additional channels stable.
	delete(seen, source)
	for channel := range seen {
		channels = append(channels, channel)
	}
	sort.Strings(channels)
	if source != "" {
		channels = append([]string{source}, channels...)
	}
	return channels, nil
}
