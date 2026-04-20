package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	emailtpl "github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
)

// emailMentionPattern matches @mentions in email body text for styling.
var emailMentionPattern = regexp.MustCompile(`(@[A-Za-z0-9._-]+)`)

// priorityOrder defines priority ranking for escalation.
var priorityOrder = map[string]int{
	"low":    0,
	"normal": 1,
	"high":   2,
	"urgent": 3,
}

// NotificationService orchestrates notification creation and delivery.
type NotificationService struct {
	notifRepo          *repository.NotificationRepository
	prefRepo           *repository.NotificationPreferenceRepository
	userSettingsRepo   *repository.UserNotificationSettingsRepository
	followerRepo       *repository.FollowerRepository
	userRepo           *repository.UserRepository
	workspaceRepo      *repository.WorkspaceRepository
	installationRepo   *repository.SupportInboxInstallationRepository
	mailboxRepo        *repository.SupportMailboxRepository
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository
	wsPublisher        *ws.Publisher
	presence           ws.PresenceProvider
	emailClient        emailSender
	appBaseURL         string
	logger             *slog.Logger
}

type emailSender interface {
	SendEmail(to, subject, htmlBody, textBody string) error
}

// NewNotificationService creates a new notification service.
func NewNotificationService(
	notifRepo *repository.NotificationRepository,
	prefRepo *repository.NotificationPreferenceRepository,
	userSettingsRepo *repository.UserNotificationSettingsRepository,
	followerRepo *repository.FollowerRepository,
	userRepo *repository.UserRepository,
	workspaceRepo *repository.WorkspaceRepository,
	wsPublisher *ws.Publisher,
	emailClient emailSender,
	appBaseURL string,
) *NotificationService {
	return &NotificationService{
		notifRepo:        notifRepo,
		prefRepo:         prefRepo,
		userSettingsRepo: userSettingsRepo,
		followerRepo:     followerRepo,
		userRepo:         userRepo,
		workspaceRepo:    workspaceRepo,
		wsPublisher:      wsPublisher,
		emailClient:      emailClient,
		appBaseURL:       strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
		logger:           slog.Default().With("service", "notification"),
	}
}

func (s *NotificationService) SetSupportRoutingDependencies(
	installationRepo *repository.SupportInboxInstallationRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	presence ws.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
) *NotificationService {
	if s == nil {
		return nil
	}
	s.installationRepo = installationRepo
	s.mailboxRepo = mailboxRepo
	s.presence = presence
	s.statusOverrideRepo = statusOverrideRepo
	return s
}

type notificationDeliveryPlan struct {
	Channel     string
	Status      string
	DeliveredAt *time.Time
	Error       *string
}

type digestNotificationItem struct {
	NotificationID string
	WorkspaceID    string
	Title          string
	EventCount     int
	LatestAt       time.Time
}

const supportReplyEmailDelay = 3 * time.Minute
const taskAgentAttentionRequiredEventType = "task.agent_attention_required"

// Emit creates notifications for all recipients of an event.
func (s *NotificationService) Emit(ctx context.Context, event model.NotificationEventInput) error {
	s.logger.InfoContext(ctx, "emitting notification",
		"event_type", event.EventType,
		"entity_type", event.EntityType,
		"entity_id", event.EntityID,
		"actor_id", event.ActorID,
		"workspace_id", event.WorkspaceID,
		"explicit_recipients", len(event.ExplicitRecipients),
		"category", event.Category,
		"priority", event.Priority,
	)

	// 1. Resolve recipients: followers + explicit recipients
	followers := []string{}
	if !event.SkipFollowers && s.followerRepo != nil {
		var err error
		followers, err = s.followerRepo.GetFollowers(ctx, event.EntityType, event.EntityID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to get followers",
				"error", err,
				"entity_type", event.EntityType,
				"entity_id", event.EntityID,
			)
			followers = []string{}
		}
	}

	s.logger.DebugContext(ctx, "resolved followers",
		"entity_id", event.EntityID,
		"follower_count", len(followers),
		"followers", followers,
	)

	// Merge followers with explicit recipients, deduplicate
	recipientSet := make(map[string]struct{})
	for _, uid := range followers {
		recipientSet[uid] = struct{}{}
	}
	for _, uid := range event.ExplicitRecipients {
		recipientSet[uid] = struct{}{}
	}

	// Auto-populate ActorSnapshot if not provided
	if len(event.ActorSnapshot) == 0 && event.ActorID != "" {
		if actor, err := s.userRepo.GetByID(ctx, event.ActorID); err == nil && actor != nil {
			event.ActorSnapshot = model.JSONB{
				"name":       actor.FullName,
				"avatar_url": actor.AvatarURL,
			}
		} else {
			s.logger.WarnContext(ctx, "failed to resolve actor for snapshot",
				"error", err,
				"actor_id", event.ActorID,
			)
		}
	}

	// Prepend actor name to title if available (e.g. "mentioned you" → "Waqar Azeem mentioned you")
	if event.ActorID != "" && event.ActorSnapshot != nil {
		if rawName, ok := event.ActorSnapshot["name"]; ok {
			if name, ok := rawName.(string); ok && strings.TrimSpace(name) != "" {
				// Only prepend if title doesn't already start with the actor name
				if !strings.HasPrefix(event.Title, name) {
					event.Title = name + " " + event.Title
				}
			}
		}
	}

	// Remove actor (don't self-notify)
	delete(recipientSet, event.ActorID)

	if len(recipientSet) == 0 {
		s.logger.InfoContext(ctx, "no recipients after filtering",
			"event_type", event.EventType,
			"entity_id", event.EntityID,
			"actor_id", event.ActorID,
			"follower_count", len(followers),
			"explicit_count", len(event.ExplicitRecipients),
		)
		return nil
	}

	recipients := make([]string, 0, len(recipientSet))
	for uid := range recipientSet {
		recipients = append(recipients, uid)
	}
	s.logger.InfoContext(ctx, "delivering to recipients",
		"event_type", event.EventType,
		"entity_id", event.EntityID,
		"recipient_count", len(recipients),
		"recipients", recipients,
	)

	priority := event.Priority
	if priority == "" {
		priority = "normal"
	}

	now := time.Now()

	for recipientID := range recipientSet {
		log := s.logger.With("recipient_id", recipientID, "entity_id", event.EntityID)

		// Check user preferences
		shouldNotify, err := s.prefRepo.ShouldNotify(ctx, recipientID, event.WorkspaceID, event.EventType, "in_app", event.TeamID)
		if err != nil {
			log.ErrorContext(ctx, "failed to check preferences", "error", err)
			continue
		}
		if !shouldNotify {
			log.DebugContext(ctx, "skipped by user preferences", "event_type", event.EventType)
			continue
		}

		// Check existing notification for this entity+recipient
		existing, err := s.notifRepo.GetExisting(ctx, recipientID, event.EntityType, event.EntityID, event.WorkspaceID)
		if err != nil {
			log.ErrorContext(ctx, "failed to check existing notification", "error", err)
		}

		actorID := &event.ActorID
		if event.ActorID == "" {
			actorID = nil
		}

		var body *string
		if event.Body != "" {
			body = &event.Body
		}

		if existing != nil {
			// Apply state transition rules
			newStatus, newReadAt, newSnoozedUntil, newArchivedAt := applyStateTransition(existing, priority)
			newPriority := escalatePriority(existing.Priority, priority)

			existing.ActorID = actorID
			existing.EventType = event.EventType
			existing.Title = event.Title
			existing.Body = body
			existing.Metadata = event.Metadata
			existing.LatestEventCategory = event.Category
			existing.ActorSnapshot = event.ActorSnapshot
			existing.EntitySnapshot = event.EntitySnapshot
			existing.ParentEntitySnapshot = event.ParentEntitySnapshot
			existing.EventCount = existing.EventCount + 1
			existing.LastEventAt = now
			existing.Status = newStatus
			existing.Priority = newPriority
			existing.ReadAt = newReadAt
			existing.SnoozedUntil = newSnoozedUntil
			existing.ArchivedAt = newArchivedAt
			existing.UpdatedAt = now

			if err := s.notifRepo.Upsert(ctx, existing); err != nil {
				log.ErrorContext(ctx, "failed to upsert notification", "error", err, "notification_id", existing.ID)
				continue
			}

			log.InfoContext(ctx, "notification updated",
				"notification_id", existing.ID,
				"event_count", existing.EventCount,
				"status", newStatus,
			)

			if err := s.createEventAndDeliveries(ctx, log, existing.ID, recipientID, actorID, event, priority, now); err != nil {
				log.ErrorContext(ctx, "failed to record event deliveries", "error", err)
			}

			// Push via WebSocket
			s.wsPublisher.Publish(ws.Event{
				Action:      "updated",
				Entity:      "notification",
				EntityID:    existing.ID,
				WorkspaceID: event.WorkspaceID,
				ActorID:     event.ActorID,
			})
		} else {
			// Create new notification
			notif := &model.Notification{
				WorkspaceID:          event.WorkspaceID,
				RecipientID:          recipientID,
				ActorID:              actorID,
				EntityType:           event.EntityType,
				EntityID:             event.EntityID,
				EventType:            event.EventType,
				Title:                event.Title,
				Body:                 body,
				Metadata:             event.Metadata,
				LatestEventCategory:  event.Category,
				ActorSnapshot:        event.ActorSnapshot,
				EntitySnapshot:       event.EntitySnapshot,
				ParentEntitySnapshot: event.ParentEntitySnapshot,
				EventCount:           1,
				LastEventAt:          now,
				Status:               "unread",
				Priority:             priority,
			}

			if err := s.notifRepo.Upsert(ctx, notif); err != nil {
				log.ErrorContext(ctx, "failed to create notification", "error", err)
				continue
			}

			log.InfoContext(ctx, "notification created",
				"notification_id", notif.ID,
				"status", "unread",
				"event_type", event.EventType,
			)

			if err := s.createEventAndDeliveries(ctx, log, notif.ID, recipientID, actorID, event, priority, now); err != nil {
				log.ErrorContext(ctx, "failed to record event deliveries", "error", err)
			}

			// Push via WebSocket
			s.wsPublisher.Publish(ws.Event{
				Action:      "created",
				Entity:      "notification",
				EntityID:    notif.ID,
				WorkspaceID: event.WorkspaceID,
				ActorID:     event.ActorID,
			})
		}
	}

	return nil
}

// MarkAgentAttentionResolved clears unread in-app attention notifications for a
// run once the pending interaction has been answered.
func (s *NotificationService) MarkAgentAttentionResolved(ctx context.Context, workspaceID, runID string) error {
	if s == nil || s.notifRepo == nil {
		return nil
	}
	return s.notifRepo.MarkEntityEventTypeAsReadForWorkspace(ctx, workspaceID, "agent_run", runID, taskAgentAttentionRequiredEventType)
}

func (s *NotificationService) createEventAndDeliveries(
	ctx context.Context,
	log *slog.Logger,
	notificationID string,
	recipientID string,
	actorID *string,
	event model.NotificationEventInput,
	priority string,
	now time.Time,
) error {
	notifEvent := &model.NotificationEvent{
		NotificationID: notificationID,
		ActorID:        actorID,
		EventType:      event.EventType,
		Title:          event.Title,
		Metadata:       event.Metadata,
		Category:       event.Category,
		ActorSnapshot:  event.ActorSnapshot,
		Priority:       priority,
	}
	if err := s.notifRepo.CreateEvent(ctx, notifEvent); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	deliveryPlans, err := s.buildDeliveryPlans(ctx, recipientID, event, priority, now)
	if err != nil {
		log.ErrorContext(ctx, "failed to build delivery plans", "error", err)
	}

	for _, plan := range deliveryPlans {
		delivery := &model.NotificationDelivery{
			NotificationEventID: notifEvent.ID,
			Channel:             plan.Channel,
			Status:              plan.Status,
			DeliveredAt:         plan.DeliveredAt,
			Error:               plan.Error,
		}
		if err := s.notifRepo.CreateDelivery(ctx, delivery); err != nil {
			log.ErrorContext(ctx, "failed to create delivery record", "channel", plan.Channel, "error", err)
		}
	}

	return nil
}

func (s *NotificationService) buildDeliveryPlans(
	ctx context.Context,
	recipientID string,
	event model.NotificationEventInput,
	priority string,
	now time.Time,
) ([]notificationDeliveryPlan, error) {
	plans := []notificationDeliveryPlan{
		{
			Channel:     "in_app",
			Status:      "delivered",
			DeliveredAt: &now,
		},
	}

	userSettings, err := s.userSettingsRepo.Get(ctx, recipientID)
	if err != nil {
		return plans, err
	}

	if event.SkipEmailDelivery {
		return plans, nil
	}

	shouldEmail, err := s.prefRepo.ShouldNotify(ctx, recipientID, event.WorkspaceID, event.EventType, "email", event.TeamID)
	if err != nil {
		return plans, err
	}
	if !shouldEmail {
		channel := "email"
		if strings.TrimSpace(event.DelayedEmailChannel) != "" {
			channel = strings.TrimSpace(event.DelayedEmailChannel)
		}
		reason := "email delivery disabled by notification preferences"
		plans = append(plans, notificationDeliveryPlan{
			Channel: channel,
			Status:  "skipped",
			Error:   &reason,
		})
		return plans, nil
	}

	if delayedChannel := strings.TrimSpace(event.DelayedEmailChannel); delayedChannel != "" {
		plans = append(plans, notificationDeliveryPlan{
			Channel: delayedChannel,
			Status:  "pending",
		})
		return plans, nil
	}

	emailChannel := selectEmailDeliveryChannel(priority, userSettings.EmailDigestFrequency)
	if emailChannel == "" {
		return plans, nil
	}

	if emailChannel == "digest" {
		if s.emailClient == nil {
			reason := "email client not configured"
			plans = append(plans, notificationDeliveryPlan{
				Channel: "digest",
				Status:  "skipped",
				Error:   &reason,
			})
			return plans, nil
		}
		plans = append(plans, notificationDeliveryPlan{
			Channel: "digest",
			Status:  "pending",
		})
		return plans, nil
	}

	emailPlan := notificationDeliveryPlan{
		Channel: "email",
		Status:  "pending",
	}

	if s.emailClient == nil {
		msg := "email client not configured"
		emailPlan.Status = "skipped"
		emailPlan.Error = &msg
		return append(plans, emailPlan), nil
	}
	if s.userRepo == nil {
		msg := "user repository not configured"
		emailPlan.Status = "failed"
		emailPlan.Error = &msg
		return append(plans, emailPlan), nil
	}

	recipient, err := s.userRepo.GetByID(ctx, recipientID)
	if err != nil {
		msg := fmt.Sprintf("resolve recipient email: %v", err)
		emailPlan.Status = "failed"
		emailPlan.Error = &msg
		return append(plans, emailPlan), nil
	}
	if recipient == nil || strings.TrimSpace(recipient.Email) == "" {
		msg := "recipient email unavailable"
		emailPlan.Status = "skipped"
		emailPlan.Error = &msg
		return append(plans, emailPlan), nil
	}

	subject, htmlBody, textBody := s.renderImmediateEmail(ctx, event)
	if err := s.emailClient.SendEmail(recipient.Email, subject, htmlBody, textBody); err != nil {
		msg := err.Error()
		emailPlan.Status = "failed"
		emailPlan.Error = &msg
		return append(plans, emailPlan), nil
	}

	emailPlan.Status = "delivered"
	emailPlan.DeliveredAt = &now
	return append(plans, emailPlan), nil
}

func selectEmailDeliveryChannel(priority, digestFrequency string) string {
	switch priority {
	case "urgent", "high":
		return "email"
	}

	switch normalizeEmailDigestFrequency(digestFrequency) {
	case "immediate":
		return "email"
	case "daily", "weekly":
		return "digest"
	default:
		return ""
	}
}

func normalizeEmailDigestFrequency(freq string) string {
	switch strings.ToLower(strings.TrimSpace(freq)) {
	case "", "daily":
		return "daily"
	case "weekly":
		return "weekly"
	case "immediate":
		return "immediate"
	case "never", "none":
		return "none"
	default:
		return "daily"
	}
}

func (s *NotificationService) renderImmediateEmail(ctx context.Context, event model.NotificationEventInput) (string, string, string) {
	workspaceName := "Helpin"
	workspaceSlug := ""
	if s.workspaceRepo != nil {
		if workspace, err := s.workspaceRepo.GetByID(ctx, event.WorkspaceID); err == nil && workspace != nil {
			if strings.TrimSpace(workspace.Name) != "" {
				workspaceName = workspace.Name
			}
			workspaceSlug = workspace.Slug
		}
	}

	actorName := "Someone"
	if event.ActorSnapshot != nil {
		if rawName, ok := event.ActorSnapshot["name"]; ok {
			if name, ok := rawName.(string); ok && strings.TrimSpace(name) != "" {
				actorName = name
			}
		}
	}

	// Extract entity display info from snapshot.
	entityTitle := ""
	entityDisplayID := ""
	if event.EntitySnapshot != nil {
		if raw, ok := event.EntitySnapshot["title"]; ok {
			if v, ok := raw.(string); ok {
				entityTitle = v
			}
		}
		if raw, ok := event.EntitySnapshot["display_id"]; ok {
			if v, ok := raw.(string); ok {
				entityDisplayID = v
			}
		}
	}

	entityURL := buildEntityURL(s.appBaseURL, workspaceSlug, event.EntityType, event.EntityID)

	subject := fmt.Sprintf("[%s] %s", workspaceName, event.Title)
	textBody := event.Title
	if strings.TrimSpace(event.Body) != "" {
		textBody += "\n\n" + event.Body
	}
	textBody += fmt.Sprintf("\n\nWorkspace: %s", workspaceName)
	if entityURL != "" {
		textBody += "\n\nView in Helpin: " + entityURL
	}

	// Build the actor action line based on the event category.
	actorLine := fmt.Sprintf(`<strong style="color: #111111; font-weight: 600;">%s</strong> %s`,
		html.EscapeString(actorName), html.EscapeString(immediateEmailActionText(event)))

	// Build the task/entity block if we have a title.
	taskBlockHTML := ""
	if entityTitle != "" {
		taskBlockHTML = emailtpl.TaskBlockHTML(entityDisplayID, entityTitle)
	}

	// Build the comment/body block (only if non-empty).
	commentBlockHTML := ""
	if trimmedBody := strings.TrimSpace(event.Body); trimmedBody != "" {
		escapedBody := html.EscapeString(trimmedBody)
		escapedBody = emailMentionPattern.ReplaceAllString(escapedBody,
			`<span style="color: #111111; font-weight: 500;">$1</span>`)
		commentBlockHTML = emailtpl.CommentBlockHTML(escapedBody)
	}

	// Build CTA section only if we have a URL.
	ctaHTML := ""
	if entityURL != "" {
		ctaHTML = emailtpl.CTAButtonHTML(entityURL)
	}

	// Build contextual footer line.
	footerText := immediateEmailFooterText(event, workspaceName, entityDisplayID)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>Notification</title>
  %s
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; margin: 0; padding: 0; background-color: #f5f5f5; -webkit-font-smoothing: antialiased;">
  <!-- Preheader text (hidden) -->
  <div style="display: none; max-height: 0; overflow: hidden;">
    %s in %s: %s
  </div>

  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background-color: #f5f5f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="max-width: 520px; width: 100%%; background: #ffffff; border-radius: 8px; border: 1px solid #e8e8e8; overflow: hidden;">

          %s

          <!-- Content -->
          <tr>
            <td style="padding: 28px 32px 32px;">
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">

                <!-- Actor line -->
                <tr>
                  <td style="font-size: 14px; color: #555555; line-height: 1.5; padding-bottom: 20px;">
                    %s
                  </td>
                </tr>

                %s

                %s

                %s

              </table>
            </td>
          </tr>

          %s

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		emailtpl.BrandHeaderCSS(),
		html.EscapeString(actorName),
		html.EscapeString(workspaceName),
		html.EscapeString(event.Title),
		emailtpl.NotificationEmailHeaderHTML(workspaceName),
		actorLine,
		taskBlockHTML,
		commentBlockHTML,
		ctaHTML,
		emailtpl.NotificationFooterHTML(footerText),
	)

	return subject, htmlBody, textBody
}

// immediateEmailActionText returns a human-readable action phrase for the actor line.
func immediateEmailActionText(event model.NotificationEventInput) string {
	switch event.Category {
	case model.NotifCategoryMentions:
		return "mentioned you"
	case model.NotifCategoryComments:
		return "commented on a task assigned to you"
	case model.NotifCategoryAssignments:
		return "assigned a task to you"
	case model.NotifCategoryStatusChanges:
		return "updated the status"
	case model.NotifCategorySupportReplies:
		return "replied to a conversation"
	case model.NotifCategorySupportMentions:
		return "mentioned you in a conversation"
	case model.NotifCategorySprints:
		return "updated a sprint"
	case model.NotifCategorySubscriptions:
		return "made an update"
	default:
		return "made an update"
	}
}

// immediateEmailFooterText returns a contextual one-liner for the email footer.
func immediateEmailFooterText(event model.NotificationEventInput, workspaceName, displayID string) string {
	switch event.Category {
	case model.NotifCategoryMentions:
		return fmt.Sprintf("You were mentioned in %s", workspaceName)
	case model.NotifCategoryComments:
		if displayID != "" {
			return fmt.Sprintf("You're assigned to %s", displayID)
		}
		return fmt.Sprintf("You received a comment in %s", workspaceName)
	case model.NotifCategoryAssignments:
		return fmt.Sprintf("A task was assigned to you in %s", workspaceName)
	case model.NotifCategorySupportReplies:
		return fmt.Sprintf("New reply in %s", workspaceName)
	case model.NotifCategorySupportMentions:
		return fmt.Sprintf("You were mentioned in %s", workspaceName)
	default:
		return fmt.Sprintf("Notification from %s", workspaceName)
	}
}

// ProcessPendingSupportReplyEmails sends delayed fallback emails for unread customer replies.
func (s *NotificationService) ProcessPendingSupportReplyEmails(ctx context.Context, now time.Time) error {
	pendingDeliveries, err := s.notifRepo.ListPendingSupportReplyEmailDeliveries(ctx)
	if err != nil {
		return err
	}
	if len(pendingDeliveries) == 0 {
		return nil
	}

	grouped := make(map[string][]repository.PendingDigestDelivery)
	for _, delivery := range pendingDeliveries {
		grouped[delivery.NotificationID] = append(grouped[delivery.NotificationID], delivery)
	}

	cutoff := now.Add(-supportReplyEmailDelay)
	var firstErr error
	for notificationID, deliveries := range grouped {
		if err := s.processPendingSupportReplyEmailGroup(ctx, notificationID, deliveries, cutoff, now); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.logger.ErrorContext(ctx, "failed to process support reply email deliveries", "notification_id", notificationID, "error", err)
		}
	}

	return firstErr
}

func (s *NotificationService) processPendingSupportReplyEmailGroup(
	ctx context.Context,
	notificationID string,
	deliveries []repository.PendingDigestDelivery,
	cutoff, now time.Time,
) error {
	if len(deliveries) == 0 {
		return nil
	}

	sort.Slice(deliveries, func(i, j int) bool {
		return deliveries[i].CreatedAt.Before(deliveries[j].CreatedAt)
	})

	latest := deliveries[len(deliveries)-1]
	if latest.CreatedAt.After(cutoff) {
		return nil
	}

	supersededIDs := make([]string, 0, len(deliveries)-1)
	for _, delivery := range deliveries[:len(deliveries)-1] {
		supersededIDs = append(supersededIDs, delivery.DeliveryID)
	}
	if len(supersededIDs) > 0 {
		reason := "superseded by a newer customer reply before delay elapsed"
		if err := s.notifRepo.UpdateDeliveryStatus(ctx, supersededIDs, "skipped", nil, &reason); err != nil {
			return err
		}
	}

	notification, err := s.notifRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}
	if notification == nil {
		reason := "notification missing"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "failed", nil, &reason)
	}

	if notification.Status != "unread" {
		reason := "support reply already handled before delay elapsed"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "skipped", nil, &reason)
	}
	if notification.SnoozedUntil != nil && notification.SnoozedUntil.After(now) {
		reason := "notification snoozed"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "skipped", nil, &reason)
	}

	if s.emailClient == nil {
		reason := "email client not configured"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "skipped", nil, &reason)
	}
	if s.userRepo == nil {
		reason := "user repository not configured"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "failed", nil, &reason)
	}

	shouldEmail, err := s.prefRepo.ShouldNotify(ctx, notification.RecipientID, notification.WorkspaceID, notification.EventType, "email", "")
	if err != nil {
		return err
	}
	if !shouldEmail {
		reason := "email delivery disabled by current notification preferences"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "skipped", nil, &reason)
	}

	recipient, err := s.userRepo.GetByID(ctx, notification.RecipientID)
	if err != nil {
		msg := fmt.Sprintf("resolve recipient email: %v", err)
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "failed", nil, &msg)
	}
	if recipient == nil || strings.TrimSpace(recipient.Email) == "" {
		msg := "recipient email unavailable"
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "skipped", nil, &msg)
	}

	event := model.NotificationEventInput{
		WorkspaceID:          notification.WorkspaceID,
		ActorID:              derefString(notification.ActorID),
		EventType:            notification.EventType,
		EntityType:           notification.EntityType,
		EntityID:             notification.EntityID,
		Title:                notification.Title,
		Body:                 derefString(notification.Body),
		Category:             notification.LatestEventCategory,
		Priority:             notification.Priority,
		ActorSnapshot:        notification.ActorSnapshot,
		EntitySnapshot:       notification.EntitySnapshot,
		ParentEntitySnapshot: notification.ParentEntitySnapshot,
		Metadata:             notification.Metadata,
	}
	subject, htmlBody, textBody := s.renderImmediateEmail(ctx, event)
	if err := s.emailClient.SendEmail(recipient.Email, subject, htmlBody, textBody); err != nil {
		msg := err.Error()
		return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "failed", nil, &msg)
	}

	return s.notifRepo.UpdateDeliveryStatus(ctx, []string{latest.DeliveryID}, "delivered", &now, nil)
}

// ProcessPendingDigests sends due digest emails and updates delivery state.
func (s *NotificationService) ProcessPendingDigests(ctx context.Context, now time.Time) error {
	pendingDeliveries, err := s.notifRepo.ListPendingDigestDeliveries(ctx)
	if err != nil {
		return err
	}
	if len(pendingDeliveries) == 0 {
		return nil
	}

	byRecipient := make(map[string][]repository.PendingDigestDelivery)
	for _, delivery := range pendingDeliveries {
		byRecipient[delivery.RecipientID] = append(byRecipient[delivery.RecipientID], delivery)
	}

	var firstErr error
	for recipientID, deliveries := range byRecipient {
		if err := s.processRecipientDigests(ctx, recipientID, deliveries, now); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.logger.ErrorContext(ctx, "failed to process digest deliveries", "recipient_id", recipientID, "error", err)
		}
	}

	return firstErr
}

func (s *NotificationService) processRecipientDigests(
	ctx context.Context,
	recipientID string,
	deliveries []repository.PendingDigestDelivery,
	now time.Time,
) error {
	userSettings, err := s.userSettingsRepo.Get(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	if !userSettings.EmailEnabled {
		reason := "email delivery disabled by account settings"
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(deliveries), "skipped", nil, &reason)
	}

	if model.IsDNDActive(userSettings.DoNotDisturb, userSettings.DNDUntil, now) {
		reason := "digest delivery skipped while do not disturb is active"
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(deliveries), "skipped", nil, &reason)
	}

	digestFrequency := normalizeEmailDigestFrequency(userSettings.EmailDigestFrequency)
	if digestFrequency != "daily" && digestFrequency != "weekly" {
		reason := "digest delivery disabled by account settings"
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(deliveries), "skipped", nil, &reason)
	}

	cutoff := latestDigestCutoff(now, userSettings)
	dueDeliveries := make([]repository.PendingDigestDelivery, 0, len(deliveries))
	for _, delivery := range deliveries {
		if !delivery.CreatedAt.After(cutoff) {
			dueDeliveries = append(dueDeliveries, delivery)
		}
	}
	if len(dueDeliveries) == 0 {
		return nil
	}

	dueDeliveries, skippedForPrefs, err := s.filterDigestDeliveriesByCurrentPreferences(ctx, recipientID, dueDeliveries)
	if err != nil {
		return err
	}
	if len(skippedForPrefs) > 0 {
		reason := "email delivery disabled by current notification preferences"
		if err := s.notifRepo.UpdateDeliveryStatus(ctx, skippedForPrefs, "skipped", nil, &reason); err != nil {
			return err
		}
	}
	if len(dueDeliveries) == 0 {
		return nil
	}

	if s.emailClient == nil {
		reason := "email client not configured"
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(dueDeliveries), "skipped", nil, &reason)
	}
	if s.userRepo == nil {
		reason := "user repository not configured"
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(dueDeliveries), "failed", nil, &reason)
	}

	recipient, err := s.userRepo.GetByID(ctx, recipientID)
	if err != nil {
		msg := fmt.Sprintf("resolve recipient email: %v", err)
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(dueDeliveries), "failed", nil, &msg)
	}
	if recipient == nil || strings.TrimSpace(recipient.Email) == "" {
		msg := "recipient email unavailable"
		return s.notifRepo.UpdateDeliveryStatus(ctx, deliveryIDs(dueDeliveries), "skipped", nil, &msg)
	}

	items, includedIDs, skippedIDs := buildDigestItems(dueDeliveries, now)
	if len(skippedIDs) > 0 {
		reason := "notification no longer unread at digest time"
		if err := s.notifRepo.UpdateDeliveryStatus(ctx, skippedIDs, "skipped", nil, &reason); err != nil {
			return err
		}
	}
	if len(items) == 0 {
		return nil
	}

	subject, htmlBody, textBody := s.renderDigestEmail(ctx, items)
	if err := s.emailClient.SendEmail(recipient.Email, subject, htmlBody, textBody); err != nil {
		msg := err.Error()
		return s.notifRepo.UpdateDeliveryStatus(ctx, includedIDs, "failed", nil, &msg)
	}

	deliveredAt := now
	return s.notifRepo.UpdateDeliveryStatus(ctx, includedIDs, "delivered", &deliveredAt, nil)
}

func (s *NotificationService) filterDigestDeliveriesByCurrentPreferences(
	ctx context.Context,
	recipientID string,
	deliveries []repository.PendingDigestDelivery,
) ([]repository.PendingDigestDelivery, []string, error) {
	if s.prefRepo == nil {
		return deliveries, nil, nil
	}

	allowed := make([]repository.PendingDigestDelivery, 0, len(deliveries))
	skipped := make([]string, 0)
	for _, delivery := range deliveries {
		shouldEmail, err := s.prefRepo.ShouldNotify(ctx, recipientID, delivery.WorkspaceID, delivery.EventType, "email", "")
		if err != nil {
			return nil, nil, fmt.Errorf("check digest preferences: %w", err)
		}
		if !shouldEmail {
			skipped = append(skipped, delivery.DeliveryID)
			continue
		}
		allowed = append(allowed, delivery)
	}

	return allowed, skipped, nil
}

func buildDigestItems(deliveries []repository.PendingDigestDelivery, now time.Time) ([]digestNotificationItem, []string, []string) {
	type groupedDigestItem struct {
		item        digestNotificationItem
		deliveryIDs []string
	}

	grouped := make(map[string]*groupedDigestItem)
	includedIDs := make([]string, 0, len(deliveries))
	skippedIDs := make([]string, 0)

	for _, delivery := range deliveries {
		if delivery.NotificationStatus != "unread" {
			skippedIDs = append(skippedIDs, delivery.DeliveryID)
			continue
		}
		if delivery.SnoozedUntil != nil && delivery.SnoozedUntil.After(now) {
			skippedIDs = append(skippedIDs, delivery.DeliveryID)
			continue
		}

		group, ok := grouped[delivery.NotificationID]
		if !ok {
			group = &groupedDigestItem{
				item: digestNotificationItem{
					NotificationID: delivery.NotificationID,
					WorkspaceID:    delivery.WorkspaceID,
					Title:          strings.TrimSpace(delivery.EventTitle),
					EventCount:     0,
					LatestAt:       delivery.CreatedAt,
				},
			}
			grouped[delivery.NotificationID] = group
		}

		if delivery.CreatedAt.After(group.item.LatestAt) {
			group.item.LatestAt = delivery.CreatedAt
		}
		if title := strings.TrimSpace(delivery.EventTitle); title != "" {
			group.item.Title = title
		}

		group.item.EventCount++
		group.deliveryIDs = append(group.deliveryIDs, delivery.DeliveryID)
		includedIDs = append(includedIDs, delivery.DeliveryID)
	}

	items := make([]digestNotificationItem, 0, len(grouped))
	for _, group := range grouped {
		if group.item.Title == "" {
			group.item.Title = "Notification update"
		}
		items = append(items, group.item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].LatestAt.After(items[j].LatestAt)
	})

	return items, includedIDs, skippedIDs
}

func latestDigestCutoff(now time.Time, settings *model.UserNotificationSettings) time.Time {
	loc := digestLocation(settings.Timezone)
	hour, minute := parseDigestTime(settings.EmailDigestTime)
	localNow := now.In(loc)

	switch normalizeEmailDigestFrequency(settings.EmailDigestFrequency) {
	case "weekly":
		targetDay := normalizeDigestWeekday(settings.EmailDigestDay)
		daysBack := (7 + int(localNow.Weekday()) - int(targetDay)) % 7
		candidateDay := localNow.AddDate(0, 0, -daysBack)
		cutoff := time.Date(candidateDay.Year(), candidateDay.Month(), candidateDay.Day(), hour, minute, 0, 0, loc)
		if cutoff.After(localNow) {
			cutoff = cutoff.AddDate(0, 0, -7)
		}
		return cutoff.UTC()
	default:
		cutoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, loc)
		if cutoff.After(localNow) {
			cutoff = cutoff.AddDate(0, 0, -1)
		}
		return cutoff.UTC()
	}
}

func parseDigestTime(raw string) (int, int) {
	var hour, minute int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d:%d", &hour, &minute); err != nil {
		return 9, 0
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 9, 0
	}
	return hour, minute
}

func digestLocation(name string) *time.Location {
	if strings.TrimSpace(name) == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func normalizeDigestWeekday(day int) time.Weekday {
	switch {
	case day == 0:
		return time.Sunday
	case day >= 1 && day <= 6:
		return time.Weekday(day)
	case day == 7:
		return time.Sunday
	default:
		return time.Monday
	}
}

func deliveryIDs(deliveries []repository.PendingDigestDelivery) []string {
	ids := make([]string, 0, len(deliveries))
	for _, delivery := range deliveries {
		ids = append(ids, delivery.DeliveryID)
	}
	return ids
}

func (s *NotificationService) renderDigestEmail(ctx context.Context, items []digestNotificationItem) (string, string, string) {
	if len(items) == 0 {
		return "[Helpin] Notification digest", "<html><body><p>No unread notifications.</p></body></html>", "No unread notifications."
	}

	workspaceNames := make(map[string]string, len(items))
	workspaceSlugs := make(map[string]string, len(items))
	workspaceOrder := make([]string, 0)
	grouped := make(map[string][]digestNotificationItem)
	for _, item := range items {
		if _, ok := grouped[item.WorkspaceID]; !ok {
			workspaceOrder = append(workspaceOrder, item.WorkspaceID)
			workspaceNames[item.WorkspaceID] = s.workspaceName(ctx, item.WorkspaceID)
			workspaceSlugs[item.WorkspaceID] = s.workspaceSlug(ctx, item.WorkspaceID)
		}
		grouped[item.WorkspaceID] = append(grouped[item.WorkspaceID], item)
	}

	subject := fmt.Sprintf("[Helpin] %d unread notifications", len(items))

	// Determine CTA URL: link to the first workspace root.
	ctaURL := ""
	if len(workspaceOrder) > 0 {
		if slug := workspaceSlugs[workspaceOrder[0]]; slug != "" && s.appBaseURL != "" {
			ctaURL = s.appBaseURL + "/w/" + slug
		}
	}

	// Plain text body.
	var textBody strings.Builder
	fmt.Fprintf(&textBody, "You have %d unread notifications.\n\n", len(items))
	for _, workspaceID := range workspaceOrder {
		textBody.WriteString(workspaceNames[workspaceID] + "\n")
		for _, item := range grouped[workspaceID] {
			textBody.WriteString("- " + digestItemLine(item) + "\n")
		}
		textBody.WriteString("\n")
	}
	if ctaURL != "" {
		textBody.WriteString("View all notifications: " + ctaURL + "\n")
	}

	// Build per-workspace HTML sections.
	var wsSections strings.Builder
	for _, workspaceID := range workspaceOrder {
		wsName := html.EscapeString(workspaceNames[workspaceID])
		fmt.Fprintf(&wsSections, `
                      <tr>
                        <td style="padding-bottom: 4px;">
                          <h2 style="margin: 0; font-size: 16px; font-weight: 700; color: #18181b;">%s</h2>
                        </td>
                      </tr>
                      <tr>
                        <td style="padding-bottom: 20px;">
                          <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">`, wsName)
		for _, item := range grouped[workspaceID] {
			line := html.EscapeString(digestItemLine(item))
			fmt.Fprintf(&wsSections, `
                            <tr>
                              <td style="padding: 8px 0; border-bottom: 1px solid #f4f4f5;">
                                <p style="margin: 0; font-size: 14px; line-height: 1.5; color: #52525b;">%s</p>
                              </td>
                            </tr>`, line)
		}
		wsSections.WriteString(`
                          </table>
                        </td>
                      </tr>`)
	}

	// Build CTA button HTML.
	ctaHTML := ""
	if ctaURL != "" {
		ctaHTML = fmt.Sprintf(`
                        <tr>
                          <td align="center" style="padding-top: 4px; padding-bottom: 8px;">
                            <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                              <tr>
                                <td style="border-radius: 7px; background-color: #111111;">
                                  <a href="%s" target="_blank" style="display: inline-block; padding: 10px 28px; font-size: 13px; font-weight: 600; color: #ffffff; text-decoration: none;">View All Notifications</a>
                                </td>
                              </tr>
                            </table>
                          </td>
                        </tr>`, html.EscapeString(ctaURL))
	}

	// Use the first workspace name for the header; fall back to "Helpin".
	headerWorkspaceName := "Helpin"
	if len(workspaceOrder) > 0 {
		headerWorkspaceName = workspaceNames[workspaceOrder[0]]
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>Notification Digest</title>
  %s
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; margin: 0; padding: 0; background-color: #f5f5f5; -webkit-font-smoothing: antialiased;">
  <!-- Preheader text (hidden) -->
  <div style="display: none; max-height: 0; overflow: hidden;">
    You have %d unread notifications
  </div>

  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background-color: #f5f5f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" cellspacing="0" cellpadding="0" border="0" style="max-width: 520px; width: 100%%; background: #ffffff; border-radius: 8px; border: 1px solid #e8e8e8; overflow: hidden;">

          %s

          <!-- Content -->
          <tr>
            <td style="padding: 28px 32px 32px;">
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">

                <!-- Heading -->
                <tr>
                  <td style="padding-bottom: 4px;">
                    <h1 style="margin: 0; font-size: 18px; font-weight: 700; color: #111111; line-height: 1.3;">Notification digest</h1>
                  </td>
                </tr>

                <!-- Subtext -->
                <tr>
                  <td style="padding-bottom: 24px;">
                    <p style="margin: 0; font-size: 14px; color: #555555;">You have %d unread notifications</p>
                  </td>
                </tr>

                <!-- Workspace sections -->
                %s

                <!-- CTA -->
                %s

              </table>
            </td>
          </tr>

          %s

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		emailtpl.BrandHeaderCSS(),
		len(items),
		emailtpl.NotificationEmailHeaderHTML(headerWorkspaceName),
		len(items),
		wsSections.String(),
		ctaHTML,
		emailtpl.NotificationFooterHTML(fmt.Sprintf("You have notifications enabled on %s", headerWorkspaceName)),
	)

	return subject, htmlBody, strings.TrimSpace(textBody.String())
}

func digestItemLine(item digestNotificationItem) string {
	if item.EventCount <= 1 {
		return item.Title
	}
	return fmt.Sprintf("%s (+%d more update%s)", item.Title, item.EventCount-1, pluralSuffix(item.EventCount-1))
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// buildEntityURL constructs a frontend URL for the given entity.
func buildEntityURL(baseURL, slug, entityType, entityID string) string {
	if baseURL == "" || slug == "" {
		return ""
	}
	base := baseURL + "/w/" + slug
	switch entityType {
	case "task", "story":
		return base + "/pm/tasks/" + entityID
	case "epic":
		return base + "/pm/epics/" + entityID
	case "objective":
		return base + "/pm/objectives/" + entityID
	case "sprint":
		return base + "/pm/sprints/" + entityID
	default:
		return base
	}
}

func (s *NotificationService) workspaceSlug(ctx context.Context, workspaceID string) string {
	if s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return ""
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || workspace == nil {
		return ""
	}
	return workspace.Slug
}

func (s *NotificationService) workspaceName(ctx context.Context, workspaceID string) string {
	if s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return "Workspace"
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || workspace == nil || strings.TrimSpace(workspace.Name) == "" {
		return "Workspace"
	}
	return workspace.Name
}

// CleanupArchivedNotifications deletes archived notifications older than the
// specified retention period. Returns the number of notifications deleted.
func (s *NotificationService) CleanupArchivedNotifications(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	count, err := s.notifRepo.DeleteArchivedOlderThan(ctx, cutoff)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to cleanup archived notifications", "error", err, "retention_days", retentionDays)
		return 0, err
	}
	if count > 0 {
		s.logger.InfoContext(ctx, "cleaned up archived notifications", "deleted_count", count, "retention_days", retentionDays, "cutoff", cutoff)
	}
	return count, nil
}

// List returns paginated notifications for a user.
func (s *NotificationService) List(ctx context.Context, recipientID, workspaceID, status, filter string, limit int, cursor *time.Time) (*model.NotificationListResponse, error) {
	notifs, err := s.notifRepo.List(ctx, recipientID, workspaceID, status, filter, limit, cursor)
	if err != nil {
		return nil, err
	}

	unreadCount, err := s.UnreadCount(ctx, recipientID, workspaceID)
	if err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(notifs) == limit {
		last := notifs[len(notifs)-1].LastEventAt.Format(time.RFC3339Nano)
		nextCursor = &last
	}

	return &model.NotificationListResponse{
		Data:        notifs,
		NextCursor:  nextCursor,
		UnreadCount: unreadCount,
	}, nil
}

// UnreadCount returns the number of unread notifications.
func (s *NotificationService) UnreadCount(ctx context.Context, recipientID, workspaceID string) (int, error) {
	userSettings, err := s.userSettingsRepo.Get(ctx, recipientID)
	if err != nil {
		return 0, err
	}
	return s.notifRepo.UnreadCount(ctx, recipientID, workspaceID, userSettings.BadgeMode)
}

// Update updates a notification's status.
func (s *NotificationService) Update(ctx context.Context, id, recipientID string, req model.UpdateNotificationRequest) error {
	updates := map[string]any{}
	now := time.Now()

	if req.Status != nil {
		updates["status"] = *req.Status
		switch *req.Status {
		case "read":
			updates["read_at"] = now
		case "unread":
			updates["read_at"] = nil
		case "archived":
			updates["archived_at"] = now
		}
	}

	if req.SnoozedUntil != nil {
		updates["snoozed_until"] = *req.SnoozedUntil
		updates["status"] = "snoozed"
	}

	return s.notifRepo.Update(ctx, id, recipientID, updates)
}

// MarkAllAsRead marks all unread notifications as read.
func (s *NotificationService) MarkAllAsRead(ctx context.Context, recipientID, workspaceID string) error {
	return s.notifRepo.MarkAllAsRead(ctx, recipientID, workspaceID)
}

// ArchiveAllRead archives all read notifications.
func (s *NotificationService) ArchiveAllRead(ctx context.Context, recipientID, workspaceID string) error {
	return s.notifRepo.ArchiveAllRead(ctx, recipientID, workspaceID)
}

// Delete deletes a notification.
func (s *NotificationService) Delete(ctx context.Context, id, recipientID string) error {
	return s.notifRepo.Delete(ctx, id, recipientID)
}

// MarkEntityCategoryAsRead marks unread notifications as read for a specific entity/category pair.
func (s *NotificationService) MarkEntityCategoryAsRead(ctx context.Context, recipientID, workspaceID, entityType, entityID, category string) error {
	return s.notifRepo.MarkEntityCategoryAsRead(ctx, recipientID, workspaceID, entityType, entityID, category)
}

// GetPreferences returns a user's notification preferences.
// Overlays account-level settings from user_notification_settings for backward compatibility.
func (s *NotificationService) GetPreferences(ctx context.Context, userID, workspaceID string) (*model.NotificationPreference, error) {
	wsPref, err := s.prefRepo.Get(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}

	// Overlay account-level fields so old frontends still see them
	userSettings, err := s.userSettingsRepo.Get(ctx, userID)
	if err == nil && userSettings != nil {
		wsPref.DoNotDisturb = userSettings.DoNotDisturb
		wsPref.DNDUntil = userSettings.DNDUntil
		wsPref.EmailEnabled = userSettings.EmailEnabled
		wsPref.EmailDigestFrequency = userSettings.EmailDigestFrequency
		wsPref.EmailDigestTime = userSettings.EmailDigestTime
		wsPref.EmailDigestDay = userSettings.EmailDigestDay
		wsPref.BadgeMode = userSettings.BadgeMode
		wsPref.Timezone = userSettings.Timezone
	}

	return wsPref, nil
}

// UpdatePreferences updates a user's notification preferences.
// Account-level fields are forwarded to user_notification_settings for backward compatibility.
func (s *NotificationService) UpdatePreferences(ctx context.Context, userID, workspaceID string, req model.UpdateNotificationPreferenceRequest) error {
	// Forward account-level fields to the new table
	hasAccountFields := req.DoNotDisturb != nil || req.DNDUntil != nil || req.EmailEnabled != nil ||
		req.EmailDigestFrequency != nil || req.EmailDigestTime != nil || req.EmailDigestDay != nil ||
		req.Timezone != nil || req.BadgeMode != nil

	if hasAccountFields {
		userSettings, err := s.userSettingsRepo.Get(ctx, userID)
		if err != nil {
			return err
		}
		userSettings.UserID = userID
		if req.DoNotDisturb != nil {
			userSettings.DoNotDisturb = *req.DoNotDisturb
			if !*req.DoNotDisturb && req.DNDUntil == nil {
				userSettings.DNDUntil = nil
			}
		}
		if req.DNDUntil != nil {
			userSettings.DNDUntil = req.DNDUntil
			if req.DoNotDisturb == nil {
				userSettings.DoNotDisturb = model.IsDNDActive(false, req.DNDUntil, time.Now())
			}
		}
		if req.EmailEnabled != nil {
			userSettings.EmailEnabled = *req.EmailEnabled
		}
		if req.EmailDigestFrequency != nil {
			userSettings.EmailDigestFrequency = *req.EmailDigestFrequency
		}
		if req.EmailDigestTime != nil {
			userSettings.EmailDigestTime = *req.EmailDigestTime
		}
		if req.EmailDigestDay != nil {
			userSettings.EmailDigestDay = *req.EmailDigestDay
		}
		if req.Timezone != nil {
			if err := ValidateTimezone(*req.Timezone); err != nil {
				return err
			}
			userSettings.Timezone = *req.Timezone
		}
		if req.BadgeMode != nil {
			userSettings.BadgeMode = *req.BadgeMode
		}
		if err := s.userSettingsRepo.Upsert(ctx, userSettings); err != nil {
			return err
		}
	}

	// Update workspace-level fields
	pref, err := s.prefRepo.Get(ctx, userID, workspaceID)
	if err != nil {
		return err
	}

	pref.UserID = userID
	pref.WorkspaceID = workspaceID

	if req.MuteWorkspace != nil {
		pref.MuteWorkspace = *req.MuteWorkspace
	}
	if req.ChannelPreferences != nil {
		pref.ChannelPreferences = model.JSONB(req.ChannelPreferences)
	}

	return s.prefRepo.Upsert(ctx, pref)
}

// applyStateTransition determines the new state when a new event arrives.
func applyStateTransition(existing *model.Notification, newPriority string) (status string, readAt *time.Time, snoozedUntil *time.Time, archivedAt *time.Time) {
	isUrgent := newPriority == "urgent"
	isHighOrAbove := newPriority == "urgent" || newPriority == "high"

	switch existing.Status {
	case "unread":
		return "unread", existing.ReadAt, existing.SnoozedUntil, existing.ArchivedAt
	case "read":
		// Any new event resets to unread
		return "unread", nil, existing.SnoozedUntil, existing.ArchivedAt
	case "archived":
		if isUrgent {
			return "unread", nil, nil, nil
		}
		return "archived", existing.ReadAt, existing.SnoozedUntil, existing.ArchivedAt
	case "snoozed":
		if isHighOrAbove {
			return "unread", nil, nil, existing.ArchivedAt
		}
		return "snoozed", existing.ReadAt, existing.SnoozedUntil, existing.ArchivedAt
	default:
		return "unread", nil, nil, nil
	}
}

// escalatePriority returns the higher of two priorities.
func escalatePriority(current, incoming string) string {
	if priorityOrder[incoming] > priorityOrder[current] {
		return incoming
	}
	return current
}
