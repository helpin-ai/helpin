package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const SlackNotificationJob = "slack_personal_notification"

type slackNotification struct {
	UserID     string
	SlackUser  string
	EventType  string
	EntityType string
	EntityID   string
	TeamID     string
	Title      string
}

func (s *SlackService) SetNotificationService(n *NotificationService) { s.notifications = n }

func (s *SlackService) NotificationPreferences(ctx context.Context, workspace, user string) (model.SlackNotificationPreferences, error) {
	return s.repo.NotificationPreferences(ctx, workspace, user)
}

func (s *SlackService) SetNotificationPreferences(ctx context.Context, workspace, user string, patch model.UpdateSlackNotificationPreferences) error {
	if patch.Assignments == nil && patch.Mentions == nil {
		return ErrChannelInvalid
	}
	updated, err := s.repo.UpdateNotificationPreferences(ctx, workspace, user, patch)
	if err == nil && !updated {
		return fmt.Errorf("%w: link your Slack account first", ErrChannelInvalid)
	}
	return err
}

func (s *NotificationService) SetSlackNotifications(repo *repository.SlackRepository, jobs *repository.SupportInboundJobRepository) {
	s.slackRepo, s.slackJobs = repo, jobs
}

func slackNotificationCategory(eventType string) string {
	switch eventType {
	case "support_conversation.assigned", "epic.assigned", "checklist.assigned":
		return model.NotifCategoryAssignments
	}
	switch category := model.EventTypeToCategory[eventType]; category {
	case model.NotifCategoryAssignments:
		return category
	case model.NotifCategoryMentions, model.NotifCategorySupportMentions:
		return model.NotifCategoryMentions
	}
	return ""
}

func (s *NotificationService) slackDestination(ctx context.Context, recipient string, event model.NotificationEventInput) (*model.SlackNotificationPreferences, error) {
	pref, err := s.slackRepo.NotificationPreferences(ctx, event.WorkspaceID, recipient)
	if err != nil || !pref.Linked {
		return nil, err
	}
	switch slackNotificationCategory(event.EventType) {
	case model.NotifCategoryAssignments:
		if !pref.Assignments {
			return nil, nil
		}
	case model.NotifCategoryMentions:
		if !pref.Mentions {
			return nil, nil
		}
	default:
		return nil, nil
	}
	enabled, err := s.prefRepo.ShouldNotify(ctx, recipient, event.WorkspaceID, event.EventType, "slack", event.TeamID)
	if err != nil || !enabled {
		return nil, err
	}
	return &pref, nil
}

func (s *NotificationService) queueSlackNotification(ctx context.Context, recipient string, event model.NotificationEventInput) error {
	if s == nil || s.slackRepo == nil || recipient == "" || recipient == event.ActorID || slackNotificationCategory(event.EventType) == "" {
		return nil
	}
	dest, err := s.slackDestination(ctx, recipient, event)
	if err != nil || dest == nil {
		return err
	}
	p := slackNotification{
		UserID: recipient, SlackUser: dest.SlackUser,
		EventType: event.EventType, EntityType: event.EntityType, EntityID: event.EntityID, TeamID: event.TeamID,
		Title: truncate(strings.Join(strings.Fields(event.Title), " "), 500),
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	job := &model.SupportInboundJob{
		ID: uuid.NewString(), Kind: SlackNotificationJob, WorkspaceID: &event.WorkspaceID,
		ChannelAccountID: &dest.AccountID, Status: "pending", AvailableAt: time.Now(), Payload: string(payload),
	}
	if event.EntityType == "support_conversation" {
		job.ConversationID = &event.EntityID
	}
	return s.slackJobs.Create(ctx, job)
}

// Some assignment paths do not emit an inbox event. Deliver the same personal
// Slack alert when one of those paths adds a human owner.
func (s *NotificationService) notifySlackAssignees(ctx context.Context, event model.NotificationEventInput, recipients ...string) {
	for _, recipient := range recipients {
		if err := s.queueSlackNotification(ctx, recipient, event); err != nil {
			s.logger.ErrorContext(ctx, "queue Slack assignment", "error", err, "workspace_id", event.WorkspaceID, "entity_id", event.EntityID)
		}
	}
}

func (s *NotificationService) notifyChecklistAssignee(ctx context.Context, task *model.PMTask, item model.PMChecklistItem, actor string) {
	if item.AssigneeID == nil {
		return
	}
	s.notifySlackAssignees(ctx, model.NotificationEventInput{
		WorkspaceID: task.WorkspaceID, ActorID: actor, TeamID: derefString(task.TeamID),
		EventType: "checklist.assigned", EntityType: "task", EntityID: task.ID, Title: "You were assigned a checklist item on " + task.Name,
	}, *item.AssigneeID)
}

func (s *SlackService) deliverNotification(ctx context.Context, job *model.SupportInboundJob) error {
	var p slackNotification
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}
	if job.WorkspaceID == nil || job.ChannelAccountID == nil {
		return ErrChannelInvalid
	}
	event := model.NotificationEventInput{WorkspaceID: *job.WorkspaceID, EventType: p.EventType, EntityType: p.EntityType, EntityID: p.EntityID, TeamID: p.TeamID}
	dest, err := s.notifications.slackDestination(ctx, p.UserID, event)
	if err != nil || dest == nil || dest.AccountID != *job.ChannelAccountID || dest.SlackUser != p.SlackUser {
		return err
	}
	allowed, err := s.notifications.canReceive(ctx, p.UserID, event)
	if err != nil || !allowed {
		return err
	}
	ws, err := s.notifications.workspaceRepo.GetByID(ctx, event.WorkspaceID)
	if err != nil || ws == nil {
		return err
	}
	account, err := s.repo.Installation(ctx, event.WorkspaceID)
	if err != nil || account == nil || account.ID != dest.AccountID {
		return err
	}
	token, err := s.channels.decryptToken(account)
	if err != nil {
		return err
	}
	link := buildEntityURL(s.notifications.appBaseURL, ws.Slug, p.EntityType, p.EntityID)
	// Message content is user-authored. Plain text keeps mention-like titles literal.
	text := p.Title
	if link != "" {
		text += "\n" + link
	}
	return s.api.Call(ctx, token, "chat.postMessage", map[string]any{
		"channel": dest.SlackUser, "client_msg_id": job.ID, "text": text, "mrkdwn": false,
		"unfurl_links": false, "unfurl_media": false,
	}, nil)
}
