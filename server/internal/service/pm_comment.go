package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMCommentService contains comment business logic.
type PMCommentService struct {
	commentRepo *repository.PMCommentRepository
	productAnalyticsEmitter
	taskRepo            *repository.PMTaskRepository
	attachmentRepo      *repository.PMAttachmentRepository
	activityService     *PMActivityService
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	workspaceRepo       *repository.WorkspaceRepository
	s3Client            *storage.S3Client
	commentRouter       taskCommentRouter
	logger              *slog.Logger
}

// taskCommentRouter receives human task comments that may address an agent.
type taskCommentRouter interface {
	RouteTaskComment(ctx context.Context, workspaceID string, comment *model.PMComment)
}

// SetTaskCommentRouter forwards human task comments to external agents.
func (s *PMCommentService) SetTaskCommentRouter(router taskCommentRouter) {
	s.commentRouter = router
}

// NewPMCommentService creates a new PMCommentService.
func NewPMCommentService(commentRepo *repository.PMCommentRepository, taskRepo *repository.PMTaskRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, notificationService *NotificationService, workspaceRepo *repository.WorkspaceRepository, s3Client *storage.S3Client) *PMCommentService {
	return &PMCommentService{
		commentRepo:         commentRepo,
		taskRepo:            taskRepo,
		attachmentRepo:      attachmentRepo,
		activityService:     activityService,
		wsPublisher:         wsPublisher,
		notificationService: notificationService,
		workspaceRepo:       workspaceRepo,
		s3Client:            s3Client,
		logger:              slog.Default().With("service", "pm_comment"),
	}
}

// List returns comments for an entity, with attachment URLs resolved.
func (s *PMCommentService) List(ctx context.Context, entityType, entityID string) ([]model.CommentWithAuthor, error) {
	if entityType == "" || entityID == "" {
		return nil, errCommandInput("entity_type and entity_id are required")
	}
	comments, err := s.commentRepo.List(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	for i := range comments {
		s.resolveAttachmentURLs(comments[i].Attachments)
		for j := range comments[i].Replies {
			s.resolveAttachmentURLs(comments[i].Replies[j].Attachments)
		}
	}
	return comments, nil
}

// ListByEntityIDs returns comments grouped by entity ID with attachment URLs resolved.
func (s *PMCommentService) ListByEntityIDs(ctx context.Context, entityType string, entityIDs []string) (map[string][]model.CommentWithAuthor, error) {
	if entityType == "" {
		return nil, errCommandInput("entity_type is required")
	}
	commentsByEntity, err := s.commentRepo.ListByEntityIDs(ctx, entityType, entityIDs)
	if err != nil {
		return nil, err
	}
	for entityID, comments := range commentsByEntity {
		for i := range comments {
			s.resolveAttachmentURLs(comments[i].Attachments)
			for j := range comments[i].Replies {
				s.resolveAttachmentURLs(comments[i].Replies[j].Attachments)
			}
		}
		commentsByEntity[entityID] = comments
	}
	return commentsByEntity, nil
}

// Get returns a single raw comment for callers that need to enforce
// module-specific route boundaries before mutating it.
func (s *PMCommentService) Get(ctx context.Context, id string) (*model.PMComment, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errCommandInput("comment id is required")
	}
	return s.commentRepo.GetByID(ctx, id)
}

// resolveAttachmentURLs populates URL / PublicURL on attachment responses so
// the frontend can render inline previews.
func (s *PMCommentService) resolveAttachmentURLs(attachments []model.AttachmentResponse) {
	if s.s3Client == nil {
		return
	}
	hasPublic := s.s3Client.HasPublicURL()
	for i := range attachments {
		a := attachments[i].Attachment
		if a.StorageKey == "" {
			continue
		}
		if hasPublic {
			attachments[i].PublicURL = s.s3Client.PublicURL(a.StorageKey)
		}
		downloadURL, err := s.s3Client.GeneratePresignedGetURL(a.StorageKey, a.FileName)
		if err == nil {
			attachments[i].URL = downloadURL
		}
	}
}

// Create creates a comment.
func (s *PMCommentService) Create(ctx context.Context, req model.CreateCommentRequest, authorID string, workspaceID string) (*model.CommentWithAuthor, error) {
	if req.EntityType == "" || req.EntityID == "" || strings.TrimSpace(req.Body) == "" {
		return nil, errCommandInput("entity_type, entity_id, and body are required")
	}
	if authorID == "" {
		return nil, errCommandInput("author is required")
	}

	comment := &model.PMComment{
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		AuthorID:   authorID,
		AgentID:    req.AgentID,
		AgentName:  strings.TrimSpace(req.AgentName),
		AgentRunID: req.AgentRunID,
		Body:       strings.TrimSpace(req.Body),
		ParentID:   req.ParentID,
		BlockID:    req.BlockID,
		Range:      req.Range,
		AnchorText: strings.TrimSpace(req.AnchorText),
	}
	if s.attachmentRepo != nil {
		if err := s.commentRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := s.commentRepo.WithTx(tx).Create(ctx, comment); err != nil {
				return err
			}
			if len(req.AttachmentIDs) == 0 {
				return nil
			}
			if err := s.attachmentRepo.WithTx(tx).ReassignToEntity(ctx, req.AttachmentIDs, "comment", comment.ID); err != nil {
				return err
			}
			return nil
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to create comment with attachments", "error", err, "comment_id", comment.ID, "attachment_ids", req.AttachmentIDs)
			return nil, err
		}
	} else if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, err
	}

	// Auto-follow task when someone comments.
	if req.EntityType == "task" {
		if err := s.taskRepo.AddFollower(ctx, req.EntityID, authorID); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow task on comment", "error", err, "entity_id", req.EntityID, "author_id", authorID)
		}
	}

	mentions := extractMentions(comment.Body)
	metadata := map[string]interface{}{}
	if len(mentions) > 0 {
		metadata["mentions"] = mentions
	}
	if comment.BlockID != nil && strings.TrimSpace(*comment.BlockID) != "" {
		metadata["block_id"] = *comment.BlockID
	}
	if strings.TrimSpace(comment.AnchorText) != "" {
		metadata["anchor_text"] = comment.AnchorText
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
	if comment.BlockID != nil && strings.TrimSpace(*comment.BlockID) != "" {
		blockMetadata := map[string]interface{}{
			"comment_id": comment.ID,
			"block_id":   *comment.BlockID,
		}
		if strings.TrimSpace(comment.AnchorText) != "" {
			blockMetadata["anchor_text"] = comment.AnchorText
		}
		if comment.Range != nil {
			blockMetadata["range"] = comment.Range
		}
		if err := s.activityService.Log(ctx, workspaceID, req.EntityType, req.EntityID, optionalActor(authorID), "block_commented", nil, nil, nil, blockMetadata); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for block comment", "error", err, "comment_id", comment.ID, "entity_id", req.EntityID)
		}
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "comment", EntityID: comment.ID, WorkspaceID: workspaceID, ActorID: authorID, ParentType: req.EntityType, ParentID: req.EntityID})
	if s.commentRouter != nil && comment.AgentID == nil && req.EntityType == "task" {
		s.commentRouter.RouteTaskComment(ctx, workspaceID, comment)
	}

	// Emit notification for comment.
	if s.notificationService != nil {
		entityTitle := req.EntityID
		var entityTeamID string
		readableTeamIDs := []string(nil)
		if req.EntityType == "task" {
			if task, _ := s.taskRepo.GetRawByID(ctx, req.EntityID); task != nil {
				entityTitle = task.Name
				entityTeamID = derefString(task.TeamID)
				s.trackProductEvent(ctx, ProductAnalyticsEvent{
					SemanticKey: "comment_created:" + comment.ID, UserID: authorID,
					WorkspaceID: workspaceID, Name: "comment_created", Source: "api",
					OccurredAt: comment.CreatedAt,
					Attributes: map[string]any{"entity_id": comment.ID, "parent_type": req.EntityType, "parent_id": req.EntityID, "is_reply": comment.ParentID != nil, "module": "pm"},
				})
				readableTeamIDs = mentionScopeForTeamID(task.TeamID)
			}
		}

		mentionedUserIDs, err := resolveMentionRecipients(ctx, s.workspaceRepo, workspaceID, comment.Body, authorID, readableTeamIDs)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to resolve comment mention recipients", "error", err, "comment_id", comment.ID, "workspace_id", workspaceID)
			mentionedUserIDs = nil
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

		entitySnapshot := model.JSONB{
			"title": entityTitle,
		}
		commentBody := truncate(tiptap.StripHTML(comment.Body), 200)

		if len(mentionedUserIDs) == 0 {
			if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
				WorkspaceID:    workspaceID,
				ActorID:        authorID,
				EventType:      eventType,
				EntityType:     req.EntityType,
				EntityID:       req.EntityID,
				Title:          "commented on " + entityTitle,
				Body:           commentBody,
				Category:       category,
				Priority:       notifPriority,
				TeamID:         entityTeamID,
				EntitySnapshot: entitySnapshot,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to emit comment notification",
					"error", err,
					"entity_id", req.EntityID,
					"event_type", eventType,
				)
			}
		} else {
			followerRecipients := []string(nil)
			if s.notificationService.followerRepo != nil {
				followers, err := s.notificationService.followerRepo.GetFollowers(ctx, req.EntityType, req.EntityID)
				if err != nil {
					s.logger.ErrorContext(ctx, "failed to load comment followers", "error", err, "entity_type", req.EntityType, "entity_id", req.EntityID)
				} else {
					excluded := make(map[string]struct{}, len(mentionedUserIDs)+1)
					excluded[authorID] = struct{}{}
					for _, userID := range mentionedUserIDs {
						excluded[userID] = struct{}{}
					}
					for _, userID := range followers {
						if _, skip := excluded[userID]; skip {
							continue
						}
						followerRecipients = append(followerRecipients, userID)
					}
				}
			}

			if len(followerRecipients) > 0 {
				if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
					WorkspaceID:        workspaceID,
					ActorID:            authorID,
					EventType:          "comment.created",
					EntityType:         req.EntityType,
					EntityID:           req.EntityID,
					Title:              "commented on " + entityTitle,
					Body:               commentBody,
					Category:           "comment",
					Priority:           "normal",
					TeamID:             entityTeamID,
					ExplicitRecipients: followerRecipients,
					SkipFollowers:      true,
					EntitySnapshot:     entitySnapshot,
				}); err != nil {
					slog.ErrorContext(ctx, "failed to emit follower comment notification",
						"error", err,
						"entity_id", req.EntityID,
						"event_type", "comment.created",
					)
				}
			}

			if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
				WorkspaceID:        workspaceID,
				ActorID:            authorID,
				EventType:          "comment.mention",
				EntityType:         req.EntityType,
				EntityID:           req.EntityID,
				Title:              "mentioned you in a comment on " + entityTitle,
				Body:               commentBody,
				Category:           "mention",
				Priority:           "high",
				TeamID:             entityTeamID,
				ExplicitRecipients: mentionedUserIDs,
				SkipFollowers:      true,
				EntitySnapshot:     entitySnapshot,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to emit comment mention notification",
					"error", err,
					"entity_id", req.EntityID,
					"event_type", "comment.mention",
				)
			}
		}
	}

	comments, err := s.commentRepo.List(ctx, req.EntityType, req.EntityID)
	if err != nil {
		return nil, err
	}
	// Replies are nested inside their parent in the repository's grouped
	// shape, so we have to look at both top-level threads and their replies.
	for _, item := range comments {
		if item.Comment.ID == comment.ID {
			return &item, nil
		}
		for _, reply := range item.Replies {
			if reply.Comment.ID == comment.ID {
				replyCopy := reply
				return &replyCopy, nil
			}
		}
	}

	created, err := s.commentRepo.GetWithAuthor(ctx, comment.ID)
	if err != nil {
		return nil, err
	}
	if created != nil {
		s.resolveAttachmentURLs(created.Attachments)
		return created, nil
	}
	return nil, fmt.Errorf("comment created but could not be loaded")
}

// SetResolved marks a comment thread resolved or reopens it. Any user with the
// module's edit permission may resolve review threads; editing/deleting the
// actual text remains author-gated.
func (s *PMCommentService) SetResolved(ctx context.Context, id string, resolved bool, actorID string, workspaceID string) (*model.PMComment, error) {
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, errCommandNotFound("comment")
	}

	action := "comment_reopened"
	var resolvedAt interface{}
	var resolvedBy interface{}
	if resolved {
		now := time.Now().UTC()
		comment.ResolvedAt = &now
		comment.ResolvedBy = &actorID
		resolvedAt = now
		resolvedBy = actorID
		action = "comment_resolved"
	} else {
		comment.ResolvedAt = nil
		comment.ResolvedBy = nil
	}
	if err := s.commentRepo.UpdateResolution(ctx, comment.ID, resolvedAt, resolvedBy); err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"comment_id": comment.ID,
	}
	if comment.BlockID != nil && strings.TrimSpace(*comment.BlockID) != "" {
		metadata["block_id"] = *comment.BlockID
	}
	if strings.TrimSpace(comment.AnchorText) != "" {
		metadata["anchor_text"] = comment.AnchorText
	}
	if err := s.activityService.Log(ctx, workspaceID, comment.EntityType, comment.EntityID, optionalActor(actorID), action, nil, nil, nil, metadata); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for comment resolution", "error", err, "comment_id", id, "entity_id", comment.EntityID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "comment", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: comment.EntityType, ParentID: comment.EntityID})
	s.logger.InfoContext(ctx, "comment resolution changed", "comment_id", id, "entity_type", comment.EntityType, "entity_id", comment.EntityID, "workspace_id", workspaceID, "actor_id", actorID, "resolved", resolved)
	return comment, nil
}

// Update updates a comment if actor is author or admin.
func (s *PMCommentService) Update(ctx context.Context, id string, req model.UpdateCommentRequest, actorID string, isAdmin bool, workspaceID string) (*model.PMComment, error) {
	if strings.TrimSpace(req.Body) == "" {
		return nil, errCommandInput("body is required")
	}
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, errCommandNotFound("comment")
	}
	if !isAdmin && comment.AuthorID != actorID {
		return nil, fmt.Errorf("only the author can edit this comment")
	}

	oldValue := comment.Body
	comment.Body = strings.TrimSpace(req.Body)
	if s.attachmentRepo != nil {
		if err := s.commentRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := s.commentRepo.WithTx(tx).Update(ctx, comment); err != nil {
				return err
			}
			if len(req.AttachmentIDs) == 0 {
				return nil
			}
			if err := s.attachmentRepo.WithTx(tx).ReassignToEntity(ctx, req.AttachmentIDs, "comment", comment.ID); err != nil {
				return err
			}
			return nil
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to update comment with attachments", "error", err, "comment_id", comment.ID, "attachment_ids", req.AttachmentIDs)
			return nil, err
		}
	} else if err := s.commentRepo.Update(ctx, comment); err != nil {
		return nil, err
	}

	if err := s.activityService.Log(ctx, workspaceID, comment.EntityType, comment.EntityID, optionalActor(actorID), "comment_updated", stringPtr("body"), &oldValue, &comment.Body, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for comment update", "error", err, "comment_id", id, "entity_id", comment.EntityID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "comment", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: comment.EntityType, ParentID: comment.EntityID})

	if s.notificationService != nil {
		entityTitle := comment.EntityID
		var entityTeamID string
		readableTeamIDs := []string(nil)
		if comment.EntityType == "task" {
			if task, _ := s.taskRepo.GetRawByID(ctx, comment.EntityID); task != nil {
				entityTitle = task.Name
				entityTeamID = derefString(task.TeamID)
				readableTeamIDs = mentionScopeForTeamID(task.TeamID)
			}
		}
		addedMentions := diffMentionHandles(extractMentions(oldValue), extractMentions(comment.Body))
		if len(addedMentions) > 0 {
			if _, err := emitMentionNotificationForHandles(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:      workspaceID,
				ActorID:          actorID,
				Body:             comment.Body,
				EventType:        "comment.mention",
				EntityType:       comment.EntityType,
				EntityID:         comment.EntityID,
				Title:            "mentioned you in a comment on " + entityTitle,
				TeamID:           entityTeamID,
				ReadableTeamIDs:  readableTeamIDs,
				EntitySnapshot:   model.JSONB{"title": entityTitle},
				NotificationBody: truncate(tiptap.StripHTML(comment.Body), 200),
			}, addedMentions); err != nil {
				s.logger.ErrorContext(ctx, "failed to emit comment mention notification", "error", err, "comment_id", id, "entity_id", comment.EntityID)
			}
		}
	}
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
		return errCommandNotFound("comment")
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
		return nil, errCommandInput("emoji is required")
	}

	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if comment == nil {
		return nil, errCommandNotFound("comment")
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
