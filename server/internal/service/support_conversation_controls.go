package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var (
	// ErrSupportInvalidPriority means the priority is not low, medium, high or urgent.
	ErrSupportInvalidPriority = errors.New("priority must be low, medium, high or urgent")
	// ErrSupportInvalidSnooze means the snooze time is not between one minute
	// and one year from now.
	ErrSupportInvalidSnooze = errors.New("choose a snooze time between one minute and one year from now")
	// ErrSupportSnoozeClosed means resolved and spam conversations cannot be snoozed.
	ErrSupportSnoozeClosed = errors.New("resolved and spam conversations cannot be snoozed")
)

const (
	supportSnoozeMinimum = time.Minute
	supportSnoozeMaximum = 366 * 24 * time.Hour
)

var supportPriorityLabels = map[string]string{
	model.SupportConversationPriorityLow:    "Low",
	model.SupportConversationPriorityMedium: "Medium",
	model.SupportConversationPriorityHigh:   "High",
	model.SupportConversationPriorityUrgent: "Urgent",
}

// teammateSystemEvent writes an internal timeline entry attributed to a
// teammate, or to Helpin when actorUserID is empty. Failures are logged:
// the change it describes has already been saved.
func (s *SupportInboxService) teammateSystemEvent(ctx context.Context, conversation *model.SupportConversation, actorUserID, eventType string, content func(actor string) string, metadata map[string]any) {
	if s.messageRepo == nil || conversation == nil {
		return
	}
	actor := "Helpin"
	var senderUserID *string
	if strings.TrimSpace(actorUserID) != "" {
		senderUserID = &actorUserID
		actor = "A teammate"
		if name := s.lookupUserName(ctx, actorUserID); strings.TrimSpace(name) != "" {
			if first := supportSystemFirstName(name); first != "" {
				actor = first
			} else {
				actor = name
			}
		}
	}
	senderType := "user"
	if senderUserID == nil {
		senderType = "system"
	}
	displayName := actor
	message := &model.SupportMessage{
		WorkspaceID: conversation.WorkspaceID, ConversationID: conversation.ID,
		SenderType: senderType, SenderUserID: senderUserID, SenderDisplayName: &displayName,
		Content: content(actor), IsInternal: true, MessageType: "system",
		SystemEventType: model.SupportSystemEventTypeStrPtr(eventType),
	}
	if len(metadata) > 0 {
		if raw, err := json.Marshal(metadata); err == nil {
			message.Metadata = string(raw)
		}
	}
	if err := s.messageRepo.Create(ctx, message); err != nil {
		slog.ErrorContext(ctx, "create support system event", "event_type", eventType, "conversation_id", conversation.ID, "error", err)
		return
	}
	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(conversation.WorkspaceID, message, actorUserID))
	}
}

func (s *SupportInboxService) publishConversationUpdated(conversation *model.SupportConversation, actorID string) {
	if s.wsPublisher == nil || conversation == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action: "updated", Entity: "support_conversation", EntityID: conversation.ID,
		WorkspaceID: conversation.WorkspaceID, ActorID: actorID,
	})
}

// UpdateConversationPriority sets a conversation's priority.
func (s *SupportInboxService) UpdateConversationPriority(ctx context.Context, workspaceID, conversationID, priority, actorID string) (*model.SupportConversation, error) {
	priority = strings.ToLower(strings.TrimSpace(priority))
	if !model.IsValidSupportConversationPriority(priority) {
		return nil, ErrSupportInvalidPriority
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	previous := conversation.Priority
	if previous == priority {
		return conversation, nil
	}
	if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversation.ID, map[string]any{"priority": priority}); err != nil {
		return nil, err
	}
	conversation.Priority = priority
	s.teammateSystemEvent(ctx, conversation, actorID, model.SystemEventPriorityChanged, func(actor string) string {
		return fmt.Sprintf("%s changed the priority from %s to %s.", actor, supportPriorityLabel(previous), supportPriorityLabel(priority))
	}, map[string]any{"previous_priority": previous, "priority": priority})
	s.publishConversationUpdated(conversation, actorID)
	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversation.ID, &actorID, "updated", strPtr("priority"), &previous, &priority, nil)
	}
	return conversation, nil
}

func supportPriorityLabel(priority string) string {
	if label, ok := supportPriorityLabels[priority]; ok {
		return label
	}
	return priority
}

// SnoozeConversation hides a conversation from the team's queues until the
// given time. A customer reply or a teammate wakes it earlier.
func (s *SupportInboxService) SnoozeConversation(ctx context.Context, workspaceID, conversationID string, until time.Time, actorID string) (*model.SupportConversation, error) {
	now := time.Now().UTC()
	until = until.UTC().Truncate(time.Second)
	if until.Before(now.Add(supportSnoozeMinimum)) || until.After(now.Add(supportSnoozeMaximum)) {
		return nil, ErrSupportInvalidSnooze
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conversation.Status == model.SupportConversationStatusResolved || conversation.Status == model.SupportConversationStatusSpam {
		return nil, ErrSupportSnoozeClosed
	}
	fields := map[string]any{"snoozed_until": until, "snoozed_by_user_id": nil}
	if strings.TrimSpace(actorID) != "" {
		fields["snoozed_by_user_id"] = actorID
	}
	if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversation.ID, fields); err != nil {
		return nil, err
	}
	conversation.SnoozedUntil = &until
	if strings.TrimSpace(actorID) != "" {
		conversation.SnoozedByUserID = &actorID
	}
	s.teammateSystemEvent(ctx, conversation, actorID, model.SystemEventSnoozed, func(actor string) string {
		return fmt.Sprintf("%s snoozed this conversation until %s.", actor, until.Format("Jan 2, 2006 15:04 UTC"))
	}, map[string]any{"snoozed_until": until.Format(time.RFC3339)})
	s.publishConversationUpdated(conversation, actorID)
	return conversation, nil
}

// UnsnoozeConversation returns a snoozed conversation to the team's queues.
func (s *SupportInboxService) UnsnoozeConversation(ctx context.Context, workspaceID, conversationID, actorID string) (*model.SupportConversation, error) {
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conversation.SnoozedUntil == nil {
		return conversation, nil
	}
	if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversation.ID, map[string]any{"snoozed_until": nil, "snoozed_by_user_id": nil}); err != nil {
		return nil, err
	}
	conversation.SnoozedUntil, conversation.SnoozedByUserID = nil, nil
	s.teammateSystemEvent(ctx, conversation, actorID, model.SystemEventUnsnoozed, func(actor string) string {
		return fmt.Sprintf("%s woke this conversation.", actor)
	}, nil)
	s.publishConversationUpdated(conversation, actorID)
	return conversation, nil
}

// WakeDueSnoozes returns conversations whose snooze has ended to the team's
// queues. It is safe to run on several replicas: each conversation is woken
// by exactly one conditional update.
func (s *SupportInboxService) WakeDueSnoozes(ctx context.Context, now time.Time, limit int) (int, error) {
	due, err := s.conversationRepo.ListDueSnoozes(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	woken := 0
	for i := range due {
		conversation := &due[i]
		claimed, err := s.conversationRepo.ClearSnooze(ctx, conversation.WorkspaceID, conversation.ID, *conversation.SnoozedUntil)
		if err != nil {
			slog.ErrorContext(ctx, "wake snoozed support conversation", "conversation_id", conversation.ID, "error", err)
			continue
		}
		if !claimed {
			continue
		}
		woken++
		conversation.SnoozedUntil, conversation.SnoozedByUserID = nil, nil
		if conversation.Status != model.SupportConversationStatusResolved && conversation.Status != model.SupportConversationStatusSpam {
			s.teammateSystemEvent(ctx, conversation, "", model.SystemEventUnsnoozed, func(string) string {
				return "Snooze ended. This conversation is back in the inbox."
			}, nil)
		}
		s.publishConversationUpdated(conversation, "")
	}
	return woken, nil
}

// RunSnoozeWaker wakes due snoozes until ctx is cancelled.
func (s *SupportInboxService) RunSnoozeWaker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.WakeDueSnoozes(ctx, time.Now().UTC(), 200); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "wake due support snoozes", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
