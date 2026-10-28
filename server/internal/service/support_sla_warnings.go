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
		for _, id := range notify {
			if !teammates[id] {
				return invalidSLAPolicy("a teammate to notify cannot work on support conversations")
			}
		}
		if assignee != nil && !teammates[*assignee] {
			return invalidSLAPolicy("the teammate to assign cannot work on support conversations")
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
// warnBefore is the policy's lead time; nil turns warnings off. It returns
// the metrics to warn about.
func applySupportSLAWarnings(record *model.SupportConversationSLARecord, previous *model.SupportConversationSLARecord, warnBefore *int, now time.Time) []string {
	if previous != nil {
		record.FirstResponseWarnedFor = previous.FirstResponseWarnedFor
		record.NextResponseWarnedFor = previous.NextResponseWarnedFor
		record.ResolutionWarnedFor = previous.ResolutionWarnedFor
	}
	if warnBefore == nil || *warnBefore <= 0 {
		return nil
	}
	lead := time.Duration(*warnBefore) * time.Minute
	var warn []string
	check := func(metric, state string, due *time.Time, warnedFor **time.Time) {
		if state != model.SupportSLAStatePending || due == nil || !now.Before(*due) {
			return
		}
		if *warnedFor != nil && (*warnedFor).Equal(*due) {
			return
		}
		warnAt := due.Add(-lead)
		if now.Before(warnAt) {
			if record.WakeAt == nil || warnAt.Before(*record.WakeAt) {
				record.WakeAt = timePtr(warnAt)
			}
			return
		}
		*warnedFor = timePtr(*due)
		if now.Sub(warnAt) <= lead/supportSLAWarningGrace {
			warn = append(warn, metric)
		}
	}
	check(model.SupportSLAMetricFirstResponse, record.FirstResponseState, record.FirstResponseDueAt, &record.FirstResponseWarnedFor)
	check(model.SupportSLAMetricNextResponse, record.NextResponseState, record.NextResponseDueAt, &record.NextResponseWarnedFor)
	check(model.SupportSLAMetricResolution, record.ResolutionState, record.ResolutionDueAt, &record.ResolutionWarnedFor)
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
	s.webhooks.Emit(ctx, conversation.WorkspaceID, model.WebhookEventSLAWarning, conversation.ID, map[string]any{"sla": payload})

	unassigned := conversation.AssignedUserID == nil || *conversation.AssignedUserID == ""
	if unassigned && policy.AssignUnassignedUserID != nil && s.inbox != nil {
		if err := s.inbox.assignConversationUser(ctx, conversation.WorkspaceID, conversation.ID, policy.AssignUnassignedUserID, nil); err != nil {
			// The teammate may have lost access since the policy was saved.
			slog.WarnContext(ctx, "assign conversation before SLA breach", "conversation_id", conversation.ID, "policy_id", policy.ID, "error", err)
		} else {
			conversation.AssignedUserID = policy.AssignUnassignedUserID
		}
	}
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
