package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func crmNotificationActor(ctx context.Context, actorID string) string {
	if actorID != "" {
		return actorID
	}
	if actor := authorization.GetActor(ctx); actor != nil {
		return actor.UserID
	}
	return ""
}

// CRM ownership uses workspace-member IDs. Resolve each new owner inside the
// record's workspace before dispatching a user-addressed notification.
func (n *NotificationService) notifyCRMOwners(ctx context.Context, event model.NotificationEventInput, oldOwners, newOwners []*string) {
	if n == nil || n.slackRepo == nil {
		return
	}
	seen := map[string]bool{"": true}
	for _, owner := range oldOwners {
		seen[derefString(owner)] = true
	}
	event.ActorID = crmNotificationActor(ctx, event.ActorID)
	for _, owner := range newOwners {
		id := derefString(owner)
		if seen[id] {
			continue
		}
		seen[id] = true
		member, err := n.workspaceRepo.GetMembershipByID(ctx, event.WorkspaceID, id)
		if err != nil {
			slog.ErrorContext(ctx, "resolve CRM notification recipient", "error", err, "workspace_id", event.WorkspaceID, "member_id", id)
			continue
		}
		if member != nil {
			n.notifySlackRecipients(ctx, event, derefString(member.UserID))
		}
	}
}

func (n *NotificationService) notifyCRMNoteMentions(ctx context.Context, activity *model.CRMActivity, previous string) {
	if n == nil || n.slackRepo == nil || activity.ActivityType != model.CRMActivityNote {
		return
	}
	added := diffMentionHandles(extractMentions(previous), extractMentions(derefString(activity.Body)))
	if len(added) == 0 {
		return
	}
	event := model.NotificationEventInput{WorkspaceID: activity.WorkspaceID, ActorID: crmNotificationActor(ctx, ""), EventType: "crm.mention", Title: "You were mentioned in a CRM note"}
	switch {
	case activity.DealID != nil:
		event.EntityType, event.EntityID = "crm_deal", *activity.DealID
	case activity.CompanyID != nil:
		event.EntityType, event.EntityID = "crm_company", *activity.CompanyID
	case activity.ContactID != nil:
		event.EntityType, event.EntityID = "crm_contact", *activity.ContactID
	default:
		return
	}
	users, err := resolveMentionRecipientsForHandles(ctx, n.workspaceRepo, activity.WorkspaceID, added, event.ActorID, nil)
	if err != nil {
		slog.ErrorContext(ctx, "resolve CRM note mentions", "error", err, "activity_id", activity.ID)
		return
	}
	n.notifySlackRecipients(ctx, event, users...)
}
