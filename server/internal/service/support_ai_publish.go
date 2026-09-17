package service

// Publishing AI replies/notes and answer-prompt construction.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func supportMessagePromptText(msg model.SupportMessage) string {
	base := strings.TrimSpace(msg.Content)
	if len(msg.Attachments) == 0 {
		return base
	}

	imageNames := make([]string, 0, len(msg.Attachments))
	fileNames := make([]string, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		name := strings.TrimSpace(attachment.FileName)
		if name == "" {
			name = "unnamed file"
		}
		if isImageAttachmentPayload(attachment) {
			imageNames = append(imageNames, name)
		} else {
			fileNames = append(fileNames, name)
		}
	}

	var extras []string
	if len(imageNames) > 0 {
		extras = append(extras, fmt.Sprintf("Customer attached image%s: %s.", supportPluralSuffix(len(imageNames)), strings.Join(imageNames, ", ")))
	}
	if len(fileNames) > 0 {
		extras = append(extras, fmt.Sprintf("Customer attached file%s: %s.", supportPluralSuffix(len(fileNames)), strings.Join(fileNames, ", ")))
	}
	if base == "" {
		return strings.Join(extras, "\n")
	}
	return base + "\n\n" + strings.Join(extras, "\n")
}

func isImageAttachmentPayload(attachment model.SupportAttachmentPayload) bool {
	fileType := strings.ToLower(strings.TrimSpace(attachment.FileType))
	return strings.HasPrefix(fileType, "image/")
}

func supportPluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (s *SupportAIService) publishAIReply(
	ctx context.Context,
	workspaceID, conversationID, agentID, content, modelName string,
	tokensUsed int,
	confidence float64,
	sources []AISource,
	replyKind string,
	issueKey string,
	issueSummary string,
	progressState string,
	customerEmail *string,
	customerPhone *string,
	processingID ...string,
) (*model.SupportMessage, error) {
	metadata := AIMessageMetadata{
		AIAutoReply:     true,
		AISources:       sources,
		AIConfidence:    confidence,
		AIModel:         modelName,
		AITokensUsed:    tokensUsed,
		AIAgentID:       agentID,
		AIReplyKind:     replyKind,
		AIIssueKey:      strings.TrimSpace(issueKey),
		AIIssueSummary:  strings.TrimSpace(issueSummary),
		AIProgressState: strings.TrimSpace(progressState),
	}
	metadataJSON, _ := json.Marshal(metadata)

	aiMsg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "ai",
		SenderAgentID:     &agentID,
		SenderDisplayName: strPtr(helpinAIDisplayName),
		Content:           stripConversationPII(strings.TrimSpace(content), customerEmail, customerPhone),
		MessageType:       "reply",
		Metadata:          string(metadataJSON),
	}
	if s.linkPreviewService != nil {
		s.linkPreviewService.EnrichMessage(ctx, aiMsg)
	}
	if err := s.createSupportAIReply(ctx, aiMsg, processingID); err != nil {
		return nil, fmt.Errorf("create AI message: %w", err)
	}

	publishSupportAIMessageStream(s.wsPublisher, workspaceID, aiMsg, "ai:"+agentID)

	if len(processingID) == 0 {
		pending := "pending"
		_ = s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, map[string]any{
			"ai_state":          &pending,
			"assigned_agent_id": &agentID,
			"ai_turn_count":     gorm.Expr("ai_turn_count + 1"),
			"flow_state":        model.SupportConversationFlowStateAIHandling,
		})
	}

	return aiMsg, nil
}

func (s *SupportAIService) publishAIInternalNote(
	ctx context.Context,
	workspaceID, conversationID, agentID, content, modelName string,
	tokensUsed int,
	confidence float64,
	sources []AISource,
	replyKind string,
	issueKey string,
	issueSummary string,
	progressState string,
	customerEmail *string,
	customerPhone *string,
	processingID ...string,
) (*model.SupportMessage, error) {
	metadata := AIMessageMetadata{
		AIAutoReply:     false,
		AISources:       sources,
		AIConfidence:    confidence,
		AIModel:         modelName,
		AITokensUsed:    tokensUsed,
		AIAgentID:       agentID,
		AIReplyKind:     strings.TrimSpace(replyKind),
		AIIssueKey:      strings.TrimSpace(issueKey),
		AIIssueSummary:  strings.TrimSpace(issueSummary),
		AIProgressState: strings.TrimSpace(progressState),
	}
	metadataJSON, _ := json.Marshal(metadata)

	note := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "ai",
		SenderAgentID:     &agentID,
		SenderDisplayName: strPtr(helpinAIDisplayName),
		Content:           stripConversationPII(strings.TrimSpace(content), customerEmail, customerPhone),
		IsInternal:        true,
		MessageType:       "reply",
		Metadata:          string(metadataJSON),
	}
	if err := s.createSupportAIReply(ctx, note, processingID); err != nil {
		return nil, fmt.Errorf("create AI internal note: %w", err)
	}
	s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, note, "ai:"+agentID))
	return note, nil
}

var errSupportTurnSettled = errors.New("support customer turn already settled")

// An optional processing ID makes runtime publication and turn settlement atomic.
// Other publishers (such as human handoff notices) have their own lifecycle.
func (s *SupportAIService) createSupportAIReply(ctx context.Context, message *model.SupportMessage, processingID []string) error {
	if len(processingID) == 0 {
		return s.messageRepo.Create(ctx, message)
	}
	if s.processingRepo == nil {
		return fmt.Errorf("support processing repository is not configured")
	}
	created, err := s.processingRepo.CreateReply(ctx, processingID[0], message, processingID[1:]...)
	if err != nil {
		return err
	}
	if !created {
		return errSupportTurnSettled
	}
	return nil
}
