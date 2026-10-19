package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) analyzeSupportMessageMedia(ctx context.Context, meta model.InternalCommandContext, messages []model.SupportMessage) (string, error) {
	// Ask already reads its attached conversation's media before launching a turn.
	if commandConversationTargetID(meta) == "" || s.supportMediaProvider == nil {
		return "", nil
	}
	var attachments []mediaAttachment
	var contextText strings.Builder
	for _, message := range messages {
		if message.IsInternal {
			continue
		}
		contextText.WriteString(message.SenderType + ": " + message.Content + "\n")
		for _, attachment := range message.Attachments {
			if !model.SharedSupportAttachmentVisible(&message, attachment) || isDecorativeDockChatAsset(attachment.FileName, attachment.FileSize) {
				continue
			}
			if !strings.HasPrefix(attachment.FileType, "image/") && !strings.HasPrefix(attachment.FileType, "video/") {
				continue
			}
			attachments = append(attachments, mediaAttachment{ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.FileType, FileSize: attachment.FileSize, URL: attachment.URL})
		}
	}
	return analyzeAttachmentMedia(ctx, s.supportMediaProvider, meta.WorkspaceID, BillingFeatureSupportAIReply, aipolicy.ActionSupportMediaEnrichment, contextText.String(), attachments)
}
