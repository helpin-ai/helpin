package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
)

// priorityOrder defines priority ranking for escalation.
var priorityOrder = map[string]int{
	"low":    0,
	"normal": 1,
	"high":   2,
	"urgent": 3,
}

// NotificationService orchestrates notification creation and delivery.
type NotificationService struct {
	notifRepo *repository.NotificationRepository
	prefRepo  *repository.NotificationPreferenceRepository
	followerRepo *repository.FollowerRepository
	wsPublisher  *ws.Publisher
	logger       *slog.Logger
}

// NewNotificationService creates a new notification service.
func NewNotificationService(
	notifRepo *repository.NotificationRepository,
	prefRepo *repository.NotificationPreferenceRepository,
	followerRepo *repository.FollowerRepository,
	wsPublisher *ws.Publisher,
) *NotificationService {
	return &NotificationService{
		notifRepo:    notifRepo,
		prefRepo:     prefRepo,
		followerRepo: followerRepo,
		wsPublisher:  wsPublisher,
		logger:       slog.Default().With("service", "notification"),
	}
}

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
	followers, err := s.followerRepo.GetFollowers(ctx, event.EntityType, event.EntityID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get followers",
			"error", err,
			"entity_type", event.EntityType,
			"entity_id", event.EntityID,
		)
		followers = []string{}
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

			// Create event record
			notifEvent := &model.NotificationEvent{
				NotificationID: existing.ID,
				ActorID:        actorID,
				EventType:      event.EventType,
				Title:          event.Title,
				Metadata:       event.Metadata,
				Category:       event.Category,
				ActorSnapshot:  event.ActorSnapshot,
				Priority:       priority,
			}
			if err := s.notifRepo.CreateEvent(ctx, notifEvent); err != nil {
				log.ErrorContext(ctx, "failed to create event record", "error", err)
			}

			// Create delivery record
			delivery := &model.NotificationDelivery{
				NotificationEventID: notifEvent.ID,
				Channel:             "in_app",
				Status:              "delivered",
				DeliveredAt:         &now,
			}
			if err := s.notifRepo.CreateDelivery(ctx, delivery); err != nil {
				log.ErrorContext(ctx, "failed to create delivery record", "error", err)
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

			// Create event record
			notifEvent := &model.NotificationEvent{
				NotificationID: notif.ID,
				ActorID:        actorID,
				EventType:      event.EventType,
				Title:          event.Title,
				Metadata:       event.Metadata,
				Category:       event.Category,
				ActorSnapshot:  event.ActorSnapshot,
				Priority:       priority,
			}
			if err := s.notifRepo.CreateEvent(ctx, notifEvent); err != nil {
				log.ErrorContext(ctx, "failed to create event record", "error", err)
			}

			// Create delivery record
			delivery := &model.NotificationDelivery{
				NotificationEventID: notifEvent.ID,
				Channel:             "in_app",
				Status:              "delivered",
				DeliveredAt:         &now,
			}
			if err := s.notifRepo.CreateDelivery(ctx, delivery); err != nil {
				log.ErrorContext(ctx, "failed to create delivery record", "error", err)
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

// List returns paginated notifications for a user.
func (s *NotificationService) List(ctx context.Context, recipientID, workspaceID, status, filter string, limit int, cursor *time.Time) (*model.NotificationListResponse, error) {
	notifs, err := s.notifRepo.List(ctx, recipientID, workspaceID, status, filter, limit, cursor)
	if err != nil {
		return nil, err
	}

	unreadCount, err := s.notifRepo.UnreadCount(ctx, recipientID, workspaceID)
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
	return s.notifRepo.UnreadCount(ctx, recipientID, workspaceID)
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

// GetPreferences returns a user's notification preferences.
func (s *NotificationService) GetPreferences(ctx context.Context, userID, workspaceID string) (*model.NotificationPreference, error) {
	return s.prefRepo.Get(ctx, userID, workspaceID)
}

// UpdatePreferences updates a user's notification preferences.
func (s *NotificationService) UpdatePreferences(ctx context.Context, userID, workspaceID string, req model.UpdateNotificationPreferenceRequest) error {
	pref, err := s.prefRepo.Get(ctx, userID, workspaceID)
	if err != nil {
		return err
	}

	pref.UserID = userID
	pref.WorkspaceID = workspaceID

	if req.DoNotDisturb != nil {
		pref.DoNotDisturb = *req.DoNotDisturb
	}
	if req.DNDUntil != nil {
		pref.DNDUntil = req.DNDUntil
	}
	if req.EmailEnabled != nil {
		pref.EmailEnabled = *req.EmailEnabled
	}
	if req.EmailDigestFrequency != nil {
		pref.EmailDigestFrequency = *req.EmailDigestFrequency
	}
	if req.EmailDigestTime != nil {
		pref.EmailDigestTime = *req.EmailDigestTime
	}
	if req.EmailDigestDay != nil {
		pref.EmailDigestDay = *req.EmailDigestDay
	}
	if req.Timezone != nil {
		pref.Timezone = *req.Timezone
	}
	if req.ChannelPreferences != nil {
		pref.ChannelPreferences = model.JSONB(req.ChannelPreferences)
	}
	if req.BadgeMode != nil {
		pref.BadgeMode = *req.BadgeMode
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
