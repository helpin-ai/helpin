package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// PreviewTaskFromConversation prepares an editable draft without creating work.
// It uses the existing metered draft generator and public conversation evidence.
func (s *SupportInboxService) PreviewTaskFromConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportTaskDraftPreview, error) {
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.AnonymizedAt != nil {
		return nil, ErrPMTriageSourceUnavailable
	}
	messages, err := s.ListConversationMessages(ctx, workspaceID, conversationID, false)
	if err != nil {
		return nil, err
	}
	draft, err := s.generateTaskDraftFromConversation(ctx, workspaceID, conversation, messages)
	if err != nil {
		return nil, err
	}
	_, hash := supportPMTriageEvidence(conversation, messages)
	return &model.SupportTaskDraftPreview{Name: draft.Title, Description: derefString(supportTaskDescriptionToRichText(draft.Description)), TaskType: draft.TaskType, Priority: draft.Priority, SourceHash: hash}, nil
}

func supportPMTriageEvidence(conversation *model.SupportConversation, messages []model.SupportMessage) (string, string) {
	var text, revision strings.Builder
	text.WriteString(conversation.Subject)
	revision.WriteString(conversation.UpdatedAt.String())
	for _, message := range messages {
		if message.IsInternal || message.DeletedAt.Valid || (message.MessageType != "" && message.MessageType != "reply") {
			continue
		}
		text.WriteString("\n" + message.SenderType + ": " + tiptap.RichTextToMarkdown(message.Content))
		revision.WriteString(message.ID + message.UpdatedAt.String())
	}
	return text.String(), pmTriageHash(text.String() + "\n" + revision.String())
}
