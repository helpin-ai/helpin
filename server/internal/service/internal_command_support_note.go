package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) executeSupportAddConversationNote(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Content        string `json:"content"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse support note input: %w", err)
	}
	conversationID, err := resolveSupportConversationCommandID(meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errCommandInput("content is required")
	}
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	if s.agentService == nil || s.agentService.agentRepo == nil {
		return nil, fmt.Errorf("agent repository is not configured")
	}
	agents, err := s.agentService.agentRepo.ListByIDs(ctx, meta.WorkspaceID, []string{meta.AgentID})
	if err != nil {
		return nil, err
	}
	if len(agents) != 1 {
		return nil, errCommandNotFound("agent")
	}
	agent := agents[0]
	note, err := s.supportInboxService.CreateConversationMessage(s.supportOperationalContext(ctx, meta), meta.WorkspaceID, conversationID,
		model.CreateMessageRequest{Content: content, MessageType: "note", IsInternal: true}, "agent", nil, &agent.ID, &agent.Name)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{
		"status": "created", "conversation_id": conversationID, "message_id": note.ID, "is_internal": true,
	}), nil
}
