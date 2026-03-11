package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var mentionPattern = regexp.MustCompile(`@([A-Za-z0-9._-]+)`)

// PMCommentService contains comment business logic.
type PMCommentService struct {
	commentRepo         *repository.PMCommentRepository
	storyRepo           *repository.PMStoryRepository
	attachmentRepo      *repository.PMAttachmentRepository
	activityService     *PMActivityService
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	workspaceRepo       *repository.WorkspaceRepository
	logger              *slog.Logger
}

// NewPMCommentService creates a new PMCommentService.
func NewPMCommentService(commentRepo *repository.PMCommentRepository, storyRepo *repository.PMStoryRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, notificationService *NotificationService, workspaceRepo *repository.WorkspaceRepository) *PMCommentService {
	return &PMCommentService{
		commentRepo:         commentRepo,
		storyRepo:           storyRepo,
		attachmentRepo:      attachmentRepo,
		activityService:     activityService,
		wsPublisher:         wsPublisher,
		notificationService: notificationService,
		workspaceRepo:       workspaceRepo,
		logger:              slog.Default().With("service", "pm_comment"),
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

	// Reassign any pre-uploaded attachments to this comment.
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToComment(ctx, req.AttachmentIDs, comment.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to comment", "error", err, "comment_id", comment.ID, "attachment_ids", req.AttachmentIDs)
		}
	}

	// Auto-follow story when someone comments.
	if req.EntityType == "story" {
		if err := s.storyRepo.AddFollower(ctx, req.EntityID, authorID); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow story on comment", "error", err, "entity_id", req.EntityID, "author_id", authorID)
		}
	}

	mentions := extractMentions(comment.Body)
	metadata := map[string]interface{}{}
	if len(mentions) > 0 {
		metadata["mentions"] = mentions
	}

	slog.InfoContext(ctx, "comment created",
		"comment_id", comment.ID,
		"entity_type", req.EntityType,
		"entity_id", req.EntityID,
		"workspace_id", workspaceID,
		"author_id", authorID,
		"mentions", mentions,
	)

	if err := s.activityService.Log(ctx, workspaceID, req.EntityType, req.EntityID, optionalActor(authorID), "comment_added", nil, nil, nil, metadata); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for comment create", "error", err, "comment_id", comment.ID, "entity_id", req.EntityID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "comment", EntityID: comment.ID, WorkspaceID: workspaceID, ActorID: authorID, ParentType: req.EntityType, ParentID: req.EntityID})

	// Emit notification for comment.
	if s.notificationService != nil {
		entityTitle := req.EntityID
		var entityTeamID string
		if req.EntityType == "story" {
			if story, _ := s.storyRepo.GetRawByID(ctx, req.EntityID); story != nil {
				entityTitle = story.Name
				entityTeamID = derefString(story.TeamID)
			}
		}

		// Resolve @mentions to user IDs for explicit notification recipients.
		var mentionedUserIDs []string
		if len(mentions) > 0 && s.workspaceRepo != nil {
			for _, handle := range mentions {
				uid, err := s.workspaceRepo.GetUserIDByHandle(ctx, workspaceID, handle)
				if err != nil {
					slog.ErrorContext(ctx, "failed to resolve mention handle",
						"handle", handle,
						"workspace_id", workspaceID,
						"error", err,
					)
					continue
				}
				if uid == "" {
					slog.WarnContext(ctx, "mention handle not found",
						"handle", handle,
						"workspace_id", workspaceID,
					)
					continue
				}
				slog.InfoContext(ctx, "mention resolved",
					"handle", handle,
					"user_id", uid,
					"workspace_id", workspaceID,
				)
				mentionedUserIDs = append(mentionedUserIDs, uid)
			}
		}

		// Comment notification goes to followers + mentioned users.
		notifPriority := "normal"
		eventType := "comment.created"
		category := "comment"
		if len(mentionedUserIDs) > 0 {
			eventType = "comment.mention"
			category = "mention"
			notifPriority = "high"
		}

		slog.InfoContext(ctx, "emitting comment notification",
			"event_type", eventType,
			"entity_id", req.EntityID,
			"mentioned_user_ids", mentionedUserIDs,
			"priority", notifPriority,
		)

		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        workspaceID,
			ActorID:            authorID,
			EventType:          eventType,
			EntityType:         req.EntityType,
			EntityID:           req.EntityID,
			Title:              "commented on " + entityTitle,
			Body:               truncate(comment.Body, 200),
			Category:           category,
			Priority:           notifPriority,
			TeamID:             entityTeamID,
			ExplicitRecipients: mentionedUserIDs,
			EntitySnapshot: model.JSONB{
				"title": entityTitle,
			},
		}); err != nil {
			slog.ErrorContext(ctx, "failed to emit comment notification",
				"error", err,
				"entity_id", req.EntityID,
				"event_type", eventType,
			)
		}
	}

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

	if err := s.activityService.Log(ctx, workspaceID, comment.EntityType, comment.EntityID, optionalActor(actorID), "comment_updated", stringPtr("body"), &oldValue, &comment.Body, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for comment update", "error", err, "comment_id", id, "entity_id", comment.EntityID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "comment", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: comment.EntityType, ParentID: comment.EntityID})
	s.logger.InfoContext(ctx, "comment updated", "comment_id", id, "entity_type", comment.EntityType, "entity_id", comment.EntityID, "workspace_id", workspaceID, "actor_id", actorID)
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

	if err := s.activityService.Log(ctx, workspaceID, comment.EntityType, comment.EntityID, optionalActor(actorID), "comment_deleted", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for comment delete", "error", err, "comment_id", id, "entity_id", comment.EntityID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "comment", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: comment.EntityType, ParentID: comment.EntityID})
	s.logger.InfoContext(ctx, "comment deleted", "comment_id", id, "entity_type", comment.EntityType, "entity_id", comment.EntityID, "workspace_id", workspaceID, "actor_id", actorID)
	return nil
}

// ToggleReaction adds or removes a reaction on a comment.
func (s *PMCommentService) ToggleReaction(ctx context.Context, commentID, userID, emoji, workspaceID string) ([]model.ReactionSummary, error) {
	if emoji == "" {
		return nil, fmt.Errorf("emoji is required")
	}

	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, fmt.Errorf("comment not found")
	}

	exists, err := s.commentRepo.HasReaction(ctx, commentID, userID, emoji)
	if err != nil {
		return nil, err
	}

	if exists {
		if err := s.commentRepo.RemoveReaction(ctx, commentID, userID, emoji); err != nil {
			return nil, err
		}
		s.logger.InfoContext(ctx, "reaction removed", "comment_id", commentID, "user_id", userID, "emoji", emoji)
	} else {
		reaction := &model.PMCommentReaction{
			CommentID: commentID,
			UserID:    userID,
			Emoji:     emoji,
		}
		if err := s.commentRepo.AddReaction(ctx, reaction); err != nil {
			return nil, err
		}
		s.logger.InfoContext(ctx, "reaction added", "comment_id", commentID, "user_id", userID, "emoji", emoji)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "comment",
		EntityID:    commentID,
		WorkspaceID: workspaceID,
		ActorID:     userID,
		ParentType:  comment.EntityType,
		ParentID:    comment.EntityID,
	})

	// Return updated reactions for this comment.
	comments, err := s.commentRepo.List(ctx, comment.EntityType, comment.EntityID)
	if err != nil {
		return nil, err
	}
	for _, c := range comments {
		if c.Comment.ID == commentID {
			return c.Reactions, nil
		}
		for _, r := range c.Replies {
			if r.Comment.ID == commentID {
				return r.Reactions, nil
			}
		}
	}
	return []model.ReactionSummary{}, nil
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
