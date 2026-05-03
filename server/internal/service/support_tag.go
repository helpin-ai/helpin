package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type SupportTagService struct {
	tagRepo          *repository.SupportTagRepository
	conversationRepo *repository.SupportConversationRepository
	wsPublisher      *websocket.Publisher
	logger           *slog.Logger
}

func NewSupportTagService(tagRepo *repository.SupportTagRepository, conversationRepo *repository.SupportConversationRepository, wsPublisher *websocket.Publisher) *SupportTagService {
	return &SupportTagService{
		tagRepo:          tagRepo,
		conversationRepo: conversationRepo,
		wsPublisher:      wsPublisher,
		logger:           slog.Default().With("service", "support_tag"),
	}
}

func (s *SupportTagService) List(ctx context.Context, workspaceID string) ([]model.SupportTag, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.tagRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *SupportTagService) Create(ctx context.Context, workspaceID string, req model.CreateSupportTagRequest) (*model.SupportTag, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	name := strings.TrimSpace(req.Name)
	if workspaceID == "" || name == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	existing, err := s.tagRepo.GetByName(ctx, workspaceID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("tag name already exists")
	}
	tag := &model.SupportTag{
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       req.Color,
	}
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		s.logger.ErrorContext(ctx, "failed to create support tag", "error", err, "workspace_id", workspaceID)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "support_tag", tag.ID, workspaceID, "")
	return tag, nil
}

func (s *SupportTagService) Update(ctx context.Context, workspaceID, id string, req model.UpdateSupportTagRequest) (*model.SupportTag, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("workspace_id and tag_id are required")
	}
	tag, err := s.tagRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return nil, fmt.Errorf("tag not found")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		existing, err := s.tagRepo.GetByName(ctx, workspaceID, name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != tag.ID {
			return nil, fmt.Errorf("tag name already exists")
		}
		tag.Name = name
	}
	if req.Color != nil {
		tag.Color = req.Color
	}
	if err := s.tagRepo.Update(ctx, tag); err != nil {
		s.logger.ErrorContext(ctx, "failed to update support tag", "error", err, "workspace_id", workspaceID, "tag_id", id)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_tag", tag.ID, workspaceID, "")
	return tag, nil
}

func (s *SupportTagService) Delete(ctx context.Context, workspaceID, id string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || strings.TrimSpace(id) == "" {
		return fmt.Errorf("workspace_id and tag_id are required")
	}
	tag, err := s.tagRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if tag == nil {
		return fmt.Errorf("tag not found")
	}
	if err := s.tagRepo.Delete(ctx, workspaceID, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete support tag", "error", err, "workspace_id", workspaceID, "tag_id", id)
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "deleted", "support_tag", id, workspaceID, "")
	return nil
}

func (s *SupportTagService) AddConversationTag(ctx context.Context, workspaceID, conversationID, tagID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(tagID) == "" {
		return fmt.Errorf("workspace_id, conversation_id, and tag_id are required")
	}
	if err := s.tagRepo.AddConversationTag(ctx, workspaceID, conversationID, tagID); err != nil {
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_conversation", conversationID, workspaceID, "")
	return nil
}

func (s *SupportTagService) RemoveConversationTag(ctx context.Context, workspaceID, conversationID, tagID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(tagID) == "" {
		return fmt.Errorf("workspace_id, conversation_id, and tag_id are required")
	}
	if err := s.tagRepo.RemoveConversationTag(ctx, workspaceID, conversationID, tagID); err != nil {
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_conversation", conversationID, workspaceID, "")
	return nil
}
