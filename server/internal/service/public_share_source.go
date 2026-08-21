package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PublicShareSource loads authorized resources and their anonymous projections.
type PublicShareSource struct {
	dockChatService *DockChatService
	agentService    *AgentService
	chatRepo        *repository.DockChatRepository
	messageRepo     *repository.AgentRunMessageRepository
	workspaceRepo   *repository.WorkspaceRepository
}

// NewPublicShareSource creates the data source for public shares.
func NewPublicShareSource(dockChatService *DockChatService, agentService *AgentService, chatRepo *repository.DockChatRepository, messageRepo *repository.AgentRunMessageRepository, workspaceRepo *repository.WorkspaceRepository) *PublicShareSource {
	return &PublicShareSource{dockChatService: dockChatService, agentService: agentService, chatRepo: chatRepo, messageRepo: messageRepo, workspaceRepo: workspaceRepo}
}

// CanAccessDockChat applies the same visibility rules as the authenticated chat view.
func (s *PublicShareSource) CanAccessDockChat(ctx context.Context, workspaceID, actorID, chatID string) (bool, error) {
	_, err := s.dockChatService.GetChat(ctx, workspaceID, actorID, chatID)
	if errors.Is(err, ErrDockChatNotFound) {
		return false, nil
	}
	return err == nil, err
}

// CanAccessAgentRun applies the same rules as the authenticated dock run view.
func (s *PublicShareSource) CanAccessAgentRun(ctx context.Context, workspaceID, actorID, runID string) (bool, error) {
	if strings.TrimSpace(actorID) == "" {
		return false, nil
	}
	run, err := s.agentService.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return false, nil
	}
	if run == nil {
		return false, nil
	}
	if run.DockChatID == nil || strings.TrimSpace(*run.DockChatID) == "" {
		return true, nil
	}
	if s.dockChatService == nil {
		return false, nil
	}
	_, err = s.dockChatService.GetChat(ctx, workspaceID, actorID, strings.TrimSpace(*run.DockChatID))
	if errors.Is(err, ErrDockChatNotFound) {
		return false, nil
	}
	return err == nil, err
}

// PublicDockChat returns the latest compact Ask transcript.
func (s *PublicShareSource) PublicDockChat(ctx context.Context, workspaceID, chatID string) (*model.PublicSharedDockChat, error) {
	chat, err := s.chatRepo.GetByID(ctx, workspaceID, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, ErrPublicShareNotFound
	}
	messages, _, err := s.messageRepo.ListByDockChat(ctx, workspaceID, chatID, nil, 1000)
	if err != nil {
		return nil, fmt.Errorf("list public chat messages: %w", err)
	}
	openPath := ""
	if workspace, workspaceErr := s.workspaceRepo.GetByID(ctx, workspaceID); workspaceErr == nil && workspace != nil {
		openPath = "/w/" + workspace.Slug + "?ask_chat=" + chat.ID
	}
	return &model.PublicSharedDockChat{Title: chat.Title, OpenPath: openPath, Messages: compactDockChatMessagePage(messages), UpdatedAt: chat.UpdatedAt}, nil
}

// PublicAgentRun returns the latest authenticated run projection with private fields removed.
func (s *PublicShareSource) PublicAgentRun(ctx context.Context, workspaceID, runID string) (*model.PublicSharedAgentRun, error) {
	session, err := s.agentService.GetCodingSession(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	eventPage, err := s.agentService.ListCodingSessionEvents(ctx, workspaceID, runID, 0)
	if err != nil {
		return nil, err
	}
	interactions, err := s.agentService.ListRunInteractions(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.agentService.ListRunArtifacts(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	sanitizePublicCodingSession(session)
	events := sanitizePublicEvents(eventPage.Events)
	sanitizePublicInteractions(interactions)
	sanitizePublicArtifacts(artifacts)
	openPath := ""
	if workspace, workspaceErr := s.workspaceRepo.GetByID(ctx, workspaceID); workspaceErr == nil && workspace != nil {
		openPath = "/w/" + workspace.Slug + "/pm/coding-sessions/" + runID
	}
	return &model.PublicSharedAgentRun{Title: session.Title, OpenPath: openPath, Session: session, Events: events, Interactions: interactions, Artifacts: artifacts}, nil
}

func sanitizePublicCodingSession(session *model.CodingSession) {
	if session == nil {
		return
	}
	session.WorkspaceID = ""
	session.SystemPrompt = nil
	session.AuthState = nil
	session.TriggeredByUser = nil
	session.TargetID = ""
	if session.StreamStateSnapshot != nil {
		session.StreamStateSnapshot.LiveReasoningMessage = nil
	}
}

func sanitizePublicEvents(events []model.CodingSessionEvent) []model.CodingSessionEvent {
	result := make([]model.CodingSessionEvent, 0, len(events))
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Type), "reasoning") {
			continue
		}
		event.SessionID = ""
		event.RunID = ""
		event.Payload = sanitizePublicMap(event.Payload)
		event.RuntimeMetadata = sanitizePublicMap(event.RuntimeMetadata)
		result = append(result, event)
	}
	return result
}

func sanitizePublicInteractions(interactions []model.AgentRunInteraction) {
	for index := range interactions {
		interactions[index].WorkspaceID = ""
		interactions[index].RunID = ""
		interactions[index].RequestPayload = sanitizePublicJSON(interactions[index].RequestPayload)
		interactions[index].ResponsePayload = sanitizePublicJSON(interactions[index].ResponsePayload)
		interactions[index].RuntimeMetadata = sanitizePublicJSON(interactions[index].RuntimeMetadata)
	}
}

func sanitizePublicArtifacts(artifacts []model.AgentRunArtifact) {
	for index := range artifacts {
		artifacts[index].WorkspaceID = ""
		artifacts[index].RunID = ""
		artifacts[index].ObjectKey = nil
		artifacts[index].Metadata = sanitizePublicJSON(artifacts[index].Metadata)
	}
}

func sanitizePublicJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(`{}`)
	}
	cleaned, err := json.Marshal(sanitizePublicValue(value))
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return cleaned
}

func sanitizePublicMap(value map[string]any) map[string]any {
	cleaned, _ := sanitizePublicValue(value).(map[string]any)
	return cleaned
}

func sanitizePublicValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, child := range typed {
			if isPublicSecretKey(key) {
				cleaned[key] = "[redacted]"
				continue
			}
			cleaned[key] = sanitizePublicValue(child)
		}
		return cleaned
	case []any:
		for index := range typed {
			typed[index] = sanitizePublicValue(typed[index])
		}
		return typed
	case string:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(typed)), "bearer ") {
			return "[redacted]"
		}
	}
	return value
}

func isPublicSecretKey(key string) bool {
	normalized := strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "api_key", "authorization", "password", "secret", "access_token", "refresh_token", "private_key", "client_secret", "cookie", "set_cookie", "encrypted_value":
		return true
	default:
		return strings.HasSuffix(normalized, "_api_key") || strings.HasSuffix(normalized, "_secret")
	}
}
