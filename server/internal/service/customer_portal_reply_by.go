package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// portalReplyExpectedBy is when the team's reply is due, for customers: the
// running first-response target, else the running next-response target.
// It is nil when nothing is running, the clock is paused (the AI is
// answering, or the team waits on the customer), or the target is overdue:
// customers see a promise, never a late notice. Resolution targets and
// policy names stay internal.
func portalReplyExpectedBy(record model.SupportConversationSLARecord, now time.Time) *time.Time {
	if record.DetachedAt != nil {
		return nil
	}
	for _, target := range []struct {
		state string
		due   *time.Time
	}{
		{record.FirstResponseState, record.FirstResponseDueAt},
		{record.NextResponseState, record.NextResponseDueAt},
	} {
		if target.state == model.SupportSLAStatePending && target.due != nil && now.Before(*target.due) {
			due := *target.due
			return &due
		}
	}
	return nil
}

// replyExpectedBy returns portalReplyExpectedBy for one conversation when
// the workspace shows it.
func (s *CustomerPortalService) replyExpectedBy(ctx context.Context, ws *PortalWorkspace, conversationID string) *time.Time {
	if !ws.Settings.PortalShowReplyBy || s.inbox == nil || s.inbox.slaService == nil {
		return nil
	}
	records, err := s.inbox.slaService.repo.RecordsByConversation(ctx, ws.ID, []string{conversationID})
	if err != nil {
		slog.ErrorContext(ctx, "load SLA for portal request", "workspace_id", ws.ID, "error", err)
		return nil
	}
	record, ok := records[conversationID]
	if !ok {
		return nil
	}
	return portalReplyExpectedBy(record, time.Now().UTC())
}

// attachReplyExpectedBy fills ReplyExpectedBy on listed requests when the
// workspace shows it.
func (s *CustomerPortalService) attachReplyExpectedBy(ctx context.Context, ws *PortalWorkspace, requests []model.SupportPortalRequest) {
	if !ws.Settings.PortalShowReplyBy || s.inbox == nil || s.inbox.slaService == nil || len(requests) == 0 {
		return
	}
	ids := make([]string, 0, len(requests))
	for _, request := range requests {
		ids = append(ids, request.ConversationID)
	}
	records, err := s.inbox.slaService.repo.RecordsByConversation(ctx, ws.ID, ids)
	if err != nil {
		slog.ErrorContext(ctx, "load SLA for portal requests", "workspace_id", ws.ID, "error", err)
		return
	}
	now := time.Now().UTC()
	for i := range requests {
		if record, ok := records[requests[i].ConversationID]; ok {
			requests[i].ReplyExpectedBy = portalReplyExpectedBy(record, now)
		}
	}
}
