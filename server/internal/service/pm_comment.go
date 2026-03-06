package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var mentionPattern = regexp.MustCompile(`@([A-Za-z0-9._-]+)`)

// PMCommentService contains comment business logic.
type PMCommentService struct {
	commentRepo     *repository.PMCommentRepository
	storyRepo       *repository.PMStoryRepository
	activityService *PMActivityService
	wsPublisher     *websocket.Publisher
}

// NewPMCommentService creates a new PMCommentService.
func NewPMCommentService(commentRepo *repository.PMCommentRepository, storyRepo *repository.PMStoryRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher) *PMCommentService {
	return &PMCommentService{
		commentRepo:     commentRepo,
		storyRepo:       storyRepo,
		activityService: activityService,
		wsPublisher:     wsPublisher,
	}
}

// List returns comments for an entity.
func (s *PMCommentService) List(ctx context.Context, entityType, entityID string) ([]model.CommentWithAuthor, error) {
	if entityType == "" || entityID == "" {
		return nil, fmt.Errorf("entity_type and entity_id are required")
	}
	return s.commentRepo.List(ctx, entityType, entityID)
}

// Create creates a comment.
func (s *PMCommentService) Create(ctx context.Context, req model.CreateCommentRequest, authorID string, workspaceID string) (*model.CommentWithAuthor, error) {
	if req.EntityType == "" || req.EntityID == "" || strings.TrimSpace(req.Body) == "" {
		return nil, fmt.Errorf("entity_type, entity_id, and body are required")
	}
	if authorID == "" {
		return nil, fmt.Errorf("author is required")
	}

	comment := &model.PMComment{
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		AuthorID:   authorID,
		Body:       strings.TrimSpace(req.Body),
		ParentID:   req.ParentID,
	}
	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, err
	}

	// Auto-follow story when someone comments.
	if req.EntityType == "story" {
		_ = s.storyRepo.AddFollower(ctx, req.EntityID, authorID)
	}

	mentions := extractMentions(comment.Body)
	metadata := map[string]interface{}{}
	if len(mentions) > 0 {
		metadata["mentions"] = mentions
	}
	_ = s.activityService.Log(ctx, workspaceID, req.EntityType, req.EntityID, optionalActor(authorID), "comment_added", nil, nil, nil, metadata)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "comment", EntityID: comment.ID, WorkspaceID: workspaceID, ActorID: authorID, ParentType: req.EntityType, ParentID: req.EntityID})

	comments, err := s.commentRepo.List(ctx, req.EntityType, req.EntityID)
	if err != nil {
		return nil, err
	}
	for _, item := range comments {
		if item.Comment.ID == comment.ID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("comment created but could not be loaded")
}

// Update updates a comment if actor is author or admin.
func (s *PMCommentService) Update(ctx context.Context, id string, req model.UpdateCommentRequest, actorID string, isAdmin bool, workspaceID string) (*model.PMComment, error) {
	if strings.TrimSpace(req.Body) == "" {
		return nil, fmt.Errorf("body is required")
	}
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, fmt.Errorf("comment not found")
	}
	if !isAdmin && comment.AuthorID != actorID {
		return nil, fmt.Errorf("only the author can edit this comment")
	}

	oldValue := comment.Body
	comment.Body = strings.TrimSpace(req.Body)
	if err := s.commentRepo.Update(ctx, comment); err != nil {
		return nil, err
	}

	_ = s.activityService.Log(ctx, workspaceID, comment.EntityType, comment.EntityID, optionalActor(actorID), "comment_updated", stringPtr("body"), &oldValue, &comment.Body, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "comment", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: comment.EntityType, ParentID: comment.EntityID})
	return comment, nil
}

// Delete deletes a comment if actor is author or admin.
func (s *PMCommentService) Delete(ctx context.Context, id string, actorID string, isAdmin bool, workspaceID string) error {
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if comment == nil {
		return fmt.Errorf("comment not found")
	}
	if !isAdmin && comment.AuthorID != actorID {
		return fmt.Errorf("only the author can delete this comment")
	}
	if err := s.commentRepo.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.activityService.Log(ctx, workspaceID, comment.EntityType, comment.EntityID, optionalActor(actorID), "comment_deleted", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "comment", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: comment.EntityType, ParentID: comment.EntityID})
	return nil
}

func extractMentions(body string) []string {
	matches := mentionPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	mentions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		username := match[1]
		if _, exists := seen[username]; exists {
			continue
		}
		seen[username] = struct{}{}
		mentions = append(mentions, username)
	}
	return mentions
}
