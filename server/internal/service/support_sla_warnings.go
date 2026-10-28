package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// normalizePolicyAlerts validates who a policy warns and when.
func (s *SupportSLAService) normalizePolicyAlerts(ctx context.Context, workspaceID string, req model.SupportSLAPolicyRequest, policy *model.SupportSLAPolicy) error {
	policy.WarnBeforeMinutes = nil
	if req.WarnBeforeMinutes != nil && *req.WarnBeforeMinutes != 0 {
		warn := *req.WarnBeforeMinutes
		if warn < 1 {
			return invalidSLAPolicy("warn at least 1 minute before a target")
		}
		for _, target := range []*int{policy.FirstResponseMinutes, policy.NextResponseMinutes, policy.ResolutionMinutes} {
			if target != nil && warn >= *target {
				return invalidSLAPolicy("the warning must come before every target; the shortest target is %d minutes", shortestSLATarget(policy))
			}
		}
		policy.WarnBeforeMinutes = &warn
	}
	notify, err := normalizeSLAList(req.NotifyUserIDs, func(string) bool { return true }, "teammate")
	if err != nil {
		return err
	}
	if len(notify) > model.SupportSLANotifyLimit {
		return invalidSLAPolicy("notify up to %d teammates", model.SupportSLANotifyLimit)
	}
	var assignee *string
	if req.AssignUnassignedUserID != nil && strings.TrimSpace(*req.AssignUnassignedUserID) != "" {
		trimmed := strings.TrimSpace(*req.AssignUnassignedUserID)
		assignee = &trimmed
	}
	if len(notify) > 0 || assignee != nil {
		teammates, err := s.supportTeammates(ctx, workspaceID)
		if err != nil {
			return err
		}
		// Teammates saved earlier who lost support access since are dropped,
		// so the policy stays editable; newly chosen ones must have access.
		kept := notify[:0]
		for _, id := range notify {
			switch {
			case teammates[id]:
				kept = append(kept, id)
			case !slices.Contains([]string(policy.NotifyUserIDs), id):
				return invalidSLAPolicy("a teammate to notify cannot work on support conversations")
			}
		}
		notify = kept
		if assignee != nil && !teammates[*assignee] {
			if policy.AssignUnassignedUserID == nil || *policy.AssignUnassignedUserID != *assignee {
				return invalidSLAPolicy("the teammate to assign cannot work on support conversations")
			}
			assignee = nil
		}
	}
	policy.NotifyUserIDs, policy.NotifyCSOwner, policy.AssignUnassignedUserID = notify, req.NotifyCSOwner, assignee
	return nil
}

func shortestSLATarget(policy *model.SupportSLAPolicy) int {
	shortest := 0
	for _, target := range []*int{policy.FirstResponseMinutes, policy.NextResponseMinutes, policy.ResolutionMinutes} {
		if target != nil && (shortest == 0 || *target < shortest) {
			shortest = *target
		}
	}
	return shortest
}

// SLATeammates lists the teammates a policy can notify or assign: members
// who can work on support conversations.
func (s *SupportSLAService) SLATeammates(ctx context.Context, workspaceID string) ([]model.AssignableMember, error) {
	if s.inbox == nil || s.inbox.workspaceRepo == nil {
		return []model.AssignableMember{}, nil
	}
	members, err := s.inbox.workspaceRepo.ListSupportAssignableMembers(ctx, workspaceID, nil)
	if members == nil {
		members = []model.AssignableMember{}
	}
	return members, err
}

// supportTeammates returns the user IDs of the workspace's active support
// teammates.
func (s *SupportSLAService) supportTeammates(ctx context.Context, workspaceID string) (map[string]bool, error) {
	teammates := map[string]bool{}
	if s.inbox == nil || s.inbox.workspaceRepo == nil {
		return teammates, nil
	}
	members, err := s.inbox.workspaceRepo.ListSupportAssignableMembers(ctx, workspaceID, nil)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member.UserID != nil && *member.UserID != "" {
			teammates[*member.UserID] = true
		}
	}
	return teammates, nil
}

// supportSLAWarningGrace is how late, as a share of the lead time, a warning
// may still be sent. An evaluation that runs later than that, or a policy
// that applies inside the warning window, records the warning silently
// instead of sending a burst of stale ones.
const supportSLAWarningGrace = 2

// applySupportSLAWarnings decides which running targets of record need a
// warning now, records them, and moves the wake-up to the next warning.
// warnBefore is the policy's lead time, counted on the targets' clock; nil
// turns warnings off. switched means new targets replaced earlier ones, so
// they may warn again. It returns the metrics to warn about.
//
// First response and resolution warn once: their due time moves with every
// pause, which must not repeat the warning. Next response warns once per
// due time, so every cycle warns.
func applySupportSLAWarnings(record *model.SupportConversationSLARecord, previous *model.SupportConversationSLARecord, warnBefore *int, clock supportSLAClock, now time.Time, switched bool) []string {
	if previous != nil && !switched {
		record.FirstResponseWarnedFor = previous.FirstResponseWarnedFor
		record.NextResponseWarnedFor = previous.NextResponseWarnedFor
		record.ResolutionWarnedFor = previous.ResolutionWarnedFor
	}
	if warnBefore == nil || *warnBefore <= 0 {
		return nil
	}
	lead := time.Duration(*warnBefore) * time.Minute
	var warn []string
	check := func(metric, state string, due *time.Time, warnedFor **time.Time, target *int, oncePerDue bool) {
		if state != model.SupportSLAStatePending || due == nil || !now.Before(*due) {
			return
		}
		// A conversation keeps the targets it started with; the lead time
		// comes from the policy now and may not fit an older, shorter target.
		if target != nil && time.Duration(*target)*time.Minute <= lead {
			return
		}
		if *warnedFor != nil && (!oncePerDue || (*warnedFor).Equal(*due)) {
			return
		}
		warnAt := clock.Sub(*due, lead)
		if now.Before(warnAt) {
			if record.WakeAt == nil || warnAt.Before(*record.WakeAt) {
				record.WakeAt = timePtr(warnAt)
			}
			return
		}
		*warnedFor = timePtr(*due)
		if clock.Between(warnAt, now) <= lead/supportSLAWarningGrace {
			warn = append(warn, metric)
		}
	}
	check(model.SupportSLAMetricFirstResponse, record.FirstResponseState, record.FirstResponseDueAt, &record.FirstResponseWarnedFor, record.TargetFirstResponseMinutes, false)
	check(model.SupportSLAMetricNextResponse, record.NextResponseState, record.NextResponseDueAt, &record.NextResponseWarnedFor, record.TargetNextResponseMinutes, true)
	check(model.SupportSLAMetricResolution, record.ResolutionState, record.ResolutionDueAt, &record.ResolutionWarnedFor, record.TargetResolutionMinutes, false)
	return warn
}

// supportSLAWarning is a set of targets about to be missed.
type supportSLAWarning struct {
	conversation *model.SupportConversation
	record       model.SupportConversationSLARecord
	policy       model.SupportSLAPolicy
	company      *repository.SupportSLACompany
	metrics      []string
}

// announceWarning notifies the usual recipients and the policy's extra
// ones, emits sla.warning, and assigns an unassigned conversation when the
// policy says so.
func (s *SupportSLAService) announceWarning(ctx context.Context, warning *supportSLAWarning) {
	conversation, policy := warning.conversation, warning.policy
	labels := make([]string, 0, len(warning.metrics))
	var due *time.Time
	for _, metric := range warning.metrics {
		labels = append(labels, supportSLAMetricLabels[metric])
		due = earliestTime(due, supportSLAMetricDue(warning.record, metric))
	}
	summary := strings.Join(labels, " and ")
	payload := map[string]any{"metrics": warning.metrics, "policy_id": policy.ID, "policy_name": warning.record.PolicyName}
	if due != nil {
		payload["due_at"] = due.UTC().Format(time.RFC3339)
	}
	if policy.AssignUnassignedUserID != nil && s.inbox != nil {
		s.assignBeforeBreach(ctx, conversation, &policy)
	}
	// After the assignment, so receivers see the new assignee.
	s.webhooks.Emit(ctx, conversation.WorkspaceID, model.WebhookEventSLAWarning, conversation.ID, map[string]any{"sla": payload})
	if s.notifications == nil {
		return
	}
	recipients := s.alertRecipients(ctx, conversation, &policy, warning.company)
	if len(recipients) == 0 {
		return
	}
	remaining := ""
	if due != nil {
		remaining = " in " + formatSLADuration(due.Sub(s.now()))
	}
	if err := s.notifications.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: conversation.WorkspaceID, EventType: "support_conversation.sla_warning",
		EntityType: "support_conversation", EntityID: conversation.ID,
		Title:    "SLA due soon in " + supportNotificationConversationLabel(conversation),
		Body:     fmt.Sprintf("The %s target of %s is due%s.", summary, warning.record.PolicyName, remaining),
		Category: model.NotifCategorySupportReplies, Priority: "high",
		EntitySnapshot:     buildSupportConversationEntitySnapshot(conversation),
		ExplicitRecipients: recipients, SkipFollowers: true,
	}); err != nil {
		slog.ErrorContext(ctx, "emit SLA warning notification", "conversation_id", conversation.ID, "error", err)
	}
}

// assignBeforeBreach gives a still unassigned conversation to the policy's
// teammate. It reads the conversation again first, so a teammate who took it
// meanwhile keeps it.
func (s *SupportSLAService) assignBeforeBreach(ctx context.Context, conversation *model.SupportConversation, policy *model.SupportSLAPolicy) {
	current, err := s.inbox.loadConversationUnscoped(ctx, conversation.WorkspaceID, conversation.ID)
	if err != nil || current == nil {
		slog.ErrorContext(ctx, "reload conversation before SLA assignment", "conversation_id", conversation.ID, "error", err)
		return
	}
	if current.AssignedUserID != nil && *current.AssignedUserID != "" {
		conversation.AssignedUserID = current.AssignedUserID
		return
	}
	if err := s.inbox.assignConversationUser(ctx, conversation.WorkspaceID, conversation.ID, policy.AssignUnassignedUserID, nil); err != nil {
		// The teammate may have lost access since the policy was saved.
		slog.WarnContext(ctx, "assign conversation before SLA breach", "conversation_id", conversation.ID, "policy_id", policy.ID, "error", err)
		return
	}
	conversation.AssignedUserID = policy.AssignUnassignedUserID
}

func supportSLAMetricDue(record model.SupportConversationSLARecord, metric string) *time.Time {
	switch metric {
	case model.SupportSLAMetricFirstResponse:
		return record.FirstResponseDueAt
	case model.SupportSLAMetricNextResponse:
		return record.NextResponseDueAt
	default:
		return record.ResolutionDueAt
	}
}

// formatSLADuration renders a time left as "12 minutes" or "2 hours".
func formatSLADuration(d time.Duration) string {
	minutes := int(d.Round(time.Minute) / time.Minute)
	switch {
	case minutes < 1:
		return "less than a minute"
	case minutes == 1:
		return "1 minute"
	case minutes < 120:
		return fmt.Sprintf("%d minutes", minutes)
	default:
		return fmt.Sprintf("%d hours", minutes/60)
	}
}

// alertRecipients is breachRecipients plus the policy's extra teammates and,
// when the policy asks, the company's customer success owner.
func (s *SupportSLAService) alertRecipients(ctx context.Context, conversation *model.SupportConversation, policy *model.SupportSLAPolicy, company *repository.SupportSLACompany) []string {
	recipients := s.breachRecipients(ctx, conversation)
	if policy == nil {
		return recipients
	}
	extra := append([]string(nil), policy.NotifyUserIDs...)
	if policy.NotifyCSOwner && company != nil && company.CSOwnerMemberID != nil {
		userID, err := s.repo.MemberUserID(ctx, conversation.WorkspaceID, *company.CSOwnerMemberID)
		if err != nil {
			slog.ErrorContext(ctx, "find customer success owner for SLA alert", "conversation_id", conversation.ID, "error", err)
		} else if userID != "" {
			extra = append(extra, userID)
		}
	}
	for _, id := range extra {
		if id != "" && !slices.Contains(recipients, id) && len(recipients) < supportSLABreachRecipientLimit {
			recipients = append(recipients, id)
		}
	}
	return recipients
}
