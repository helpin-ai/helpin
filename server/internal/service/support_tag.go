package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type SupportTagService struct {
	tagRepo          *repository.SupportTagRepository
	conversationRepo *repository.SupportConversationRepository
	messageRepo      *repository.SupportMessageRepository
	userRepo         *repository.UserRepository
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

func (s *SupportTagService) SetMessageRepo(repo *repository.SupportMessageRepository) *SupportTagService {
	s.messageRepo = repo
	return s
}

func (s *SupportTagService) SetUserRepo(repo *repository.UserRepository) *SupportTagService {
	s.userRepo = repo
	return s
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

func (s *SupportTagService) AddConversationTag(ctx context.Context, workspaceID, conversationID, tagID, actorID string) error {
	return s.changeConversationTag(ctx, workspaceID, conversationID, tagID, actorID, true)
}

func (s *SupportTagService) RemoveConversationTag(ctx context.Context, workspaceID, conversationID, tagID, actorID string) error {
	return s.changeConversationTag(ctx, workspaceID, conversationID, tagID, actorID, false)
}

func (s *SupportTagService) changeConversationTag(ctx context.Context, workspaceID, conversationID, tagID, actorID string, add bool) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(tagID) == "" {
		return fmt.Errorf("workspace_id, conversation_id, and tag_id are required")
	}
	var note *model.SupportMessage
	err := s.tagRepo.WithConversation(ctx, workspaceID, conversationID, func(tx *gorm.DB) error {
		bound := s.withTx(tx)
		linked, err := bound.conversationHasTag(ctx, workspaceID, conversationID, tagID)
		if err != nil || linked == add {
			return err
		}
		tag, err := bound.tagRepo.GetByID(ctx, workspaceID, tagID)
		if err != nil {
			return err
		}
		if tag == nil {
			return gorm.ErrRecordNotFound
		}
		event := model.SystemEventTagAdded
		if add {
			err = bound.tagRepo.AddConversationTag(ctx, workspaceID, conversationID, tagID)
		} else {
			event = model.SystemEventTagRemoved
			err = bound.tagRepo.RemoveConversationTag(ctx, workspaceID, conversationID, tagID)
		}
		if err != nil {
			return err
		}
		note, err = bound.createTagSystemMessage(ctx, workspaceID, conversationID, actorID, tagID, tag.Name, event)
		return err
	})
	if err == nil {
		s.publishTagChange(workspaceID, conversationID, note)
	}
	return err
}

func (s *SupportTagService) withTx(tx *gorm.DB) *SupportTagService {
	bound := *s
	bound.tagRepo = repository.NewSupportTagRepository(tx)
	if s.messageRepo != nil {
		bound.messageRepo = s.messageRepo.WithTx(tx)
	}
	if s.userRepo != nil {
		bound.userRepo = repository.NewUserRepository(tx)
	}
	bound.wsPublisher = nil
	return &bound
}

func (s *SupportTagService) publishTagChange(workspaceID, conversationID string, note *model.SupportMessage) {
	if note != nil && s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, note, derefString(note.SenderUserID)))
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_conversation", conversationID, workspaceID, "")
}

func (s *SupportTagService) conversationHasTag(ctx context.Context, workspaceID, conversationID, tagID string) (bool, error) {
	tagsByConversation, err := s.tagRepo.ListByConversationIDs(ctx, workspaceID, []string{conversationID})
	if err != nil {
		return false, err
	}
	for _, tag := range tagsByConversation[conversationID] {
		if tag.ID == tagID {
			return true, nil
		}
	}
	return false, nil
}

func (s *SupportTagService) createTagSystemMessage(ctx context.Context, workspaceID, conversationID, actorUserID, tagID, tagName string, eventType model.SupportSystemEventType) (*model.SupportMessage, error) {
	if s.messageRepo == nil {
		return nil, nil
	}
	displayName := "A teammate"
	if strings.TrimSpace(actorUserID) != "" && s.userRepo != nil {
		if user, err := s.userRepo.GetByID(ctx, actorUserID); err == nil && user != nil {
			name := supportSystemFirstName(user.FullName)
			if name == "" {
				name = user.FullName
			}
			if strings.TrimSpace(name) != "" {
				displayName = name
			}
		} else if err != nil {
			s.logger.WarnContext(ctx, "failed to load support tag actor", "error", err, "user_id", actorUserID)
		}
	}
	action := "added"
	if eventType == model.SystemEventTagRemoved {
		action = "removed"
	}
	var senderUserID *string
	if strings.TrimSpace(actorUserID) != "" {
		senderUserID = &actorUserID
	}
	metadata, err := json.Marshal(map[string]string{"tag_id": tagID})
	if err != nil {
		return nil, err
	}
	msg := &model.SupportMessage{
		Metadata:          string(metadata),
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderUserID:      senderUserID,
		SenderDisplayName: &displayName,
		Content:           fmt.Sprintf("%s %s tag %s.", displayName, action, strings.Join(strings.Fields(strings.TrimSpace(tagName)), " ")),
		IsInternal:        true,
		MessageType:       "system",
		SystemEventType:   model.SupportSystemEventTypeStrPtr(eventType),
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// addAutomaticTag is called only inside the fenced tagging transaction.
func (s *SupportTagService) addAutomaticTag(ctx context.Context, workspaceID, conversationID, tagID string) (*model.SupportMessage, error) {
	linked, err := s.conversationHasTag(ctx, workspaceID, conversationID, tagID)
	if err != nil || linked {
		return nil, err
	}
	tag, err := s.tagRepo.GetByID(ctx, workspaceID, tagID)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return nil, fmt.Errorf("tag not found")
	}
	if s.messageRepo != nil {
		history, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
		if err != nil {
			return nil, err
		}
		if supportTagManuallyRemoved(history, *tag) {
			return nil, nil
		}
	}
	if err := s.tagRepo.AddConversationTag(ctx, workspaceID, conversationID, tagID); err != nil {
		return nil, err
	}
	if s.messageRepo != nil {
		display := "Helpin AI"
		metadata, err := json.Marshal(map[string]string{"tag_id": tagID, "provider": "typesafe", "model": decision.Model})
		if err != nil {
			return nil, err
		}
		msg := &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversationID, SenderType: "ai", SenderDisplayName: &display, Content: "Helpin AI added tag " + tag.Name + ".", IsInternal: true, MessageType: "system", SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventTagAdded), Metadata: string(metadata)}
		if err := s.messageRepo.Create(ctx, msg); err != nil {
			return nil, err
		}
		return msg, nil
	}
	return nil, nil
}

// supportTagManuallyRemoved supports stable IDs and legacy name-only audit messages.
func supportTagManuallyRemoved(history []model.SupportMessage, tag model.SupportTag) bool {
	for _, msg := range history {
		if msg.SystemEventType == nil || *msg.SystemEventType != model.SystemEventTagRemoved || msg.SenderType == "ai" {
			continue
		}
		var metadata struct {
			TagID string `json:"tag_id"`
		}
		if json.Unmarshal([]byte(msg.Metadata), &metadata) == nil && metadata.TagID != "" {
			if metadata.TagID == tag.ID {
				return true
			}
			continue
		}
		if strings.HasSuffix(msg.Content, " removed tag "+strings.Join(strings.Fields(strings.TrimSpace(tag.Name)), " ")+".") {
			return true
		}
	}
	return false
}
