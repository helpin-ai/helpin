package service

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Reconnects must not resurrect a spinner from historical acknowledgment
// metadata after final delivery, handoff, or a disabled support AI setting.
func (s *SupportInboxService) projectWidgetAIProgress(ctx context.Context, conv *model.SupportConversation, messages []model.SupportMessage) error {
	var progressIndexes []int
	for i := range messages {
		var meta AIMessageMetadata
		if json.Unmarshal([]byte(messages[i].Metadata), &meta) == nil && meta.AIProgressState == supportAIProgressChecking {
			progressIndexes = append(progressIndexes, i)
		}
	}
	if len(progressIndexes) == 0 {
		return nil
	}
	pending, err := repository.NewAIMessageProcessingRepository(s.conversationRepo.DB()).LatestProcessingForConversation(ctx, conv.WorkspaceID, conv.ID)
	if err != nil {
		return err
	}
	settings := model.DefaultSupportInboxSettings()
	installation, err := repository.NewSupportInboxInstallationRepository(s.conversationRepo.DB()).GetByWorkspace(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	if installation != nil {
		if err := json.Unmarshal([]byte(installation.Settings), &settings); err != nil {
			return err
		}
	}
	for _, i := range progressIndexes {
		if pending != nil && pending.Status == "waiting_for_result" && !messages[i].CreatedAt.Before(pending.CreatedAt) && !model.SupportAIConversationBlocked(conv) && installation != nil && installation.Active && shouldCreatePublicSupportAIReply(settings) {
			continue
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(messages[i].Metadata), &meta); err != nil {
			return err
		}
		delete(meta, "ai_progress_state")
		raw, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		messages[i].Metadata = string(raw)
	}
	return nil
}
