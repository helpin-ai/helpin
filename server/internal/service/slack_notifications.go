package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
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
	Metadata   model.JSONB
	Title      string
}

func (s *SlackService) SetNotificationService(n *NotificationService) { s.notifications = n }

func (s *SlackService) NotificationPreferences(ctx context.Context, workspace, user string) (model.SlackNotificationPreferences, error) {
	pref, err := s.repo.NotificationPreferences(ctx, workspace, user)
	if err == nil && pref.Linked {
		pref.Categories = make(map[string]bool, len(model.SlackCategoryDefaults))
		for key := range model.SlackCategoryDefaults {
			pref.Categories[key] = pref.CategoryEnabled(key)
		}
	}
	return pref, err
}

func (s *SlackService) SetNotificationPreferences(ctx context.Context, workspace, user string, patch model.UpdateSlackNotificationPreferences) error {
	for key := range patch.Categories {
		if _, ok := model.SlackCategoryDefaults[key]; !ok {
			return fmt.Errorf("%w: unknown notification row", ErrChannelInvalid)
		}
	}
	if patch.Assignments == nil && patch.Mentions == nil && len(patch.Categories) == 0 {
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

func slackPreferenceKey(event model.NotificationEventInput) string {
	category := model.NotificationPreferenceCategory(event.EventType, event.Metadata)
	switch category {
	case "crm.assignments", "crm.mentions", "crm.updates", "support.assignments", "docs.updates",
		"agents.activity", "automation.attention", "automation.activity", model.WorkflowFailureCategory:
		return category
	}
	switch event.EventType {
	case "support_conversation.mentioned":
		return "support.mentions"
	case "support_conversation.customer_reply", "support_conversation.linked_task_completed":
		return "support.replies"
	case "crm.signal_ready":
		return "crm.signals"
	case "external_mcp.reauthorization_required":
		return "integrations.connection"
	case taskAgentAttentionRequiredEventType:
		return "agents.attention"
	}
	if event.EntityType == "doc" || event.EntityType == "document" {
		switch category {
		case model.NotifCategoryMentions:
			return "docs.mentions"
		case model.NotifCategoryComments:
			return "docs.updates"
		}
		return ""
	}
	switch category {
	case model.NotifCategoryAssignments, model.NotifCategoryMentions, model.NotifCategoryStatusChanges, model.NotifCategoryComments, model.NotifCategorySubscriptions, model.NotifCategorySprints:
		return "projects." + category
	}
	return ""
}

func slackAudienceIncludes(event model.NotificationEventInput, recipient string) bool {
	if event.SlackMentionRecipients != nil {
		return slices.Contains(event.SlackMentionRecipients, recipient)
	}
	return model.SlackCategoryDefaults[slackPreferenceKey(event)] == "off" || slices.Contains(event.ExplicitRecipients, recipient)
}

func (s *NotificationService) slackDestination(ctx context.Context, recipient string, event model.NotificationEventInput) (*model.SlackNotificationPreferences, error) {
	pref, err := s.slackRepo.NotificationPreferences(ctx, event.WorkspaceID, recipient)
	if err != nil || !pref.CategoryEnabled(slackPreferenceKey(event)) {
		return nil, err
	}
	enabled, err := s.prefRepo.ShouldNotify(ctx, recipient, event.WorkspaceID, event.EventType, "slack", event.TeamID)
	if err != nil || !enabled {
		return nil, err
	}
	return &pref, nil
}

func (s *NotificationService) queueSlackNotification(ctx context.Context, recipient string, event model.NotificationEventInput) error {
	if s == nil || s.slackRepo == nil || recipient == "" || recipient == event.ActorID || slackPreferenceKey(event) == "" {
		return nil
	}
	dest, err := s.slackDestination(ctx, recipient, event)
	if err != nil || dest == nil {
		return err
	}
	p := slackNotification{
		UserID: recipient, SlackUser: dest.SlackUser,
		EventType: event.EventType, EntityType: event.EntityType, EntityID: event.EntityID, TeamID: event.TeamID,
		Metadata: model.JSONB{}, Title: truncate(strings.Join(strings.Fields(event.Title), " "), 500),
	}
	// Keep only routing metadata needed when a queued delivery is rechecked.
	for _, key := range []string{"task_id", "dock_chat_id", "automation_rule_id", "interaction_id", "support_message_id", "workflow_execution_id"} {
		if value, ok := event.Metadata[key].(string); ok && value != "" {
			p.Metadata[key] = value
		}
	}
	if event.Metadata["crm_follow_up"] == true {
		p.Metadata["crm_follow_up"] = true
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	job := &model.SupportInboundJob{
		ID: uuid.NewString(), Kind: SlackNotificationJob, WorkspaceID: &event.WorkspaceID,
		ChannelAccountID: &dest.AccountID, Status: "pending", AvailableAt: time.Now(), Payload: string(payload),
	}
	// Replayed runtime interactions and inbound replies have stable source IDs.
	sourceID, _ := p.Metadata["interaction_id"].(string)
	if id, ok := p.Metadata["workflow_execution_id"].(string); ok {
		sourceID = id
	}
	if id, ok := p.Metadata["support_message_id"].(string); ok {
		sourceID = id
	}
	if event.EventType == "agent_run.finished" || event.EventType == "agent_run.started" {
		sourceID = event.EntityID
	}
	if sourceID != "" {
		job.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(strings.Join([]string{SlackNotificationJob, event.WorkspaceID, recipient, event.EventType, event.EntityID, sourceID}, "/"))).String()
	}
	if event.EntityType == "support_conversation" {
		job.ConversationID = &event.EntityID
	}
	return s.slackJobs.Create(ctx, job)
}

func (s *SlackService) deliverNotification(ctx context.Context, job *model.SupportInboundJob) error {
	var p slackNotification
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return err
	}
	if job.WorkspaceID == nil || job.ChannelAccountID == nil {
		return ErrChannelInvalid
	}
	event := model.NotificationEventInput{WorkspaceID: *job.WorkspaceID, EventType: p.EventType, EntityType: p.EntityType, EntityID: p.EntityID, TeamID: p.TeamID, Metadata: p.Metadata}
	dest, err := s.notifications.slackDestination(ctx, p.UserID, event)
	if err != nil || dest == nil || dest.AccountID != *job.ChannelAccountID || dest.SlackUser != p.SlackUser {
		return err
	}
	allowed, err := s.notifications.canReceive(ctx, p.UserID, event)
	if err != nil || !allowed {
		return err
	}
	pending, err := s.repo.NotificationActionPending(ctx, p.UserID, event)
	if err != nil || !pending {
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
	link := buildEntityURL(s.notifications.appBaseURL, ws.Slug, p.EntityType, p.EntityID, p.Metadata)
	message := map[string]any{
		"channel": dest.SlackUser, "client_msg_id": job.ID, "text": p.Title, "mrkdwn": false,
		"unfurl_links": false, "unfurl_media": false,
	}
	if link != "" {
		settings := strings.TrimRight(s.notifications.appBaseURL, "/") + "/w/" + url.PathEscape(ws.Slug) + "/settings/notifications?slack=" + url.QueryEscape(slackPreferenceKey(event))
		message["text"] = p.Title + "\n" + link + "\nManage Slack notifications: " + settings
		message["blocks"] = []map[string]any{
			{"type": "section", "text": map[string]string{"type": "plain_text", "text": p.Title}},
			{"type": "context", "elements": []map[string]string{{"type": "mrkdwn", "text": "<" + link + "|Open in Helpin> · <" + settings + "|Manage Slack notifications>"}}},
		}
	}
	return s.api.Call(ctx, token, "chat.postMessage", message, nil)
}
