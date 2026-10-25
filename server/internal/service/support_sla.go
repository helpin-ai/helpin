package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrSupportSLAPolicyInvalid wraps a validation message for a policy.
var ErrSupportSLAPolicyInvalid = errors.New("invalid SLA policy")

var supportSLAMetricLabels = map[string]string{
	model.SupportSLAMetricFirstResponse: "first response",
	model.SupportSLAMetricNextResponse:  "next response",
	model.SupportSLAMetricResolution:    "resolution",
}

var supportSLAChannels = []string{"widget", "email", "portal", "api", "internal"}

// SupportSLAService manages SLA policies and keeps each conversation's SLA
// state current. Database triggers queue conversations whose status,
// priority, inbox, snooze or public replies change; a worker evaluates the
// queue and wakes at each due time to record breaches.
type SupportSLAService struct {
	repo          *repository.SupportSLARepository
	inbox         *SupportInboxService
	notifications *NotificationService
	entitlements  EntitlementPolicy
	now           func() time.Time
}

func NewSupportSLAService(repo *repository.SupportSLARepository, inbox *SupportInboxService) *SupportSLAService {
	return &SupportSLAService{repo: repo, inbox: inbox, now: func() time.Time { return time.Now().UTC() }}
}

// SetNotificationService enables breach notifications to the assignee.
func (s *SupportSLAService) SetNotificationService(notifications *NotificationService) *SupportSLAService {
	s.notifications = notifications
	return s
}

// SetEntitlementService gates policy management on the sla_policies feature.
func (s *SupportSLAService) SetEntitlementService(entitlements EntitlementPolicy) *SupportSLAService {
	s.entitlements = entitlements
	return s
}

func (s *SupportSLAService) requireFeature(ctx context.Context, workspaceID string) error {
	if s.entitlements == nil {
		return nil
	}
	return s.entitlements.RequireFeature(ctx, workspaceID, EntitlementFeatureSLAPolicies)
}

func invalidSLAPolicy(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSupportSLAPolicyInvalid, fmt.Sprintf(format, args...))
}

func normalizeSLATarget(minutes *int, label string) (*int, error) {
	if minutes == nil || *minutes == 0 {
		return nil, nil
	}
	if *minutes < 1 || *minutes > model.SupportSLAPolicyMaxTargetMinutes {
		return nil, invalidSLAPolicy("%s target must be between 1 minute and 90 days", label)
	}
	value := *minutes
	return &value, nil
}

func normalizeSLAList(values []string, allowed func(string) bool, label string) (model.DocsStringArray, error) {
	result := model.DocsStringArray{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains([]string(result), value) {
			continue
		}
		if !allowed(value) {
			return nil, invalidSLAPolicy("unknown %s %q", label, value)
		}
		result = append(result, value)
	}
	return result, nil
}

func (s *SupportSLAService) normalizePolicy(ctx context.Context, workspaceID string, req model.SupportSLAPolicyRequest, policy *model.SupportSLAPolicy) error {
	name := strings.Join(strings.Fields(req.Name), " ")
	if name == "" || len([]rune(name)) > 120 {
		return invalidSLAPolicy("name must be 1 to 120 characters")
	}
	var err error
	if policy.FirstResponseMinutes, err = normalizeSLATarget(req.FirstResponseMinutes, "first response"); err != nil {
		return err
	}
	if policy.NextResponseMinutes, err = normalizeSLATarget(req.NextResponseMinutes, "next response"); err != nil {
		return err
	}
	if policy.ResolutionMinutes, err = normalizeSLATarget(req.ResolutionMinutes, "resolution"); err != nil {
		return err
	}
	if policy.FirstResponseMinutes == nil && policy.NextResponseMinutes == nil && policy.ResolutionMinutes == nil {
		return invalidSLAPolicy("set at least one target")
	}
	if policy.Priorities, err = normalizeSLAList(req.Priorities, model.IsValidSupportConversationPriority, "priority"); err != nil {
		return err
	}
	if policy.Channels, err = normalizeSLAList(req.Channels, func(v string) bool { return slices.Contains(supportSLAChannels, v) }, "channel"); err != nil {
		return err
	}
	mailboxes := map[string]bool{"shared": true}
	if len(req.MailboxIDs) > 0 && s.inbox != nil && s.inbox.mailboxRepo != nil {
		active, err := s.inbox.mailboxRepo.ListByWorkspace(ctx, workspaceID, false)
		if err != nil {
			return err
		}
		for _, mailbox := range active {
			mailboxes[mailbox.ID] = true
		}
	}
	if policy.MailboxIDs, err = normalizeSLAList(req.MailboxIDs, func(v string) bool { return mailboxes[v] }, "inbox"); err != nil {
		return err
	}
	policy.Name = name
	policy.BusinessHours = req.BusinessHours
	policy.PauseWhileWaiting = req.PauseWhileWaiting == nil || *req.PauseWhileWaiting
	return nil
}

// ListPolicies returns the workspace's policies in evaluation order.
func (s *SupportSLAService) ListPolicies(ctx context.Context, workspaceID string) ([]model.SupportSLAPolicy, error) {
	policies, err := s.repo.ListPolicies(ctx, workspaceID)
	if policies == nil {
		policies = []model.SupportSLAPolicy{}
	}
	return policies, err
}

// CreatePolicy adds a policy after the existing ones. It applies to
// conversations created from now on.
func (s *SupportSLAService) CreatePolicy(ctx context.Context, workspaceID, actorID string, req model.SupportSLAPolicyRequest) (*model.SupportSLAPolicy, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	if count, err := s.repo.CountPolicies(ctx, workspaceID); err != nil {
		return nil, err
	} else if count >= model.SupportSLAPolicyLimit {
		return nil, invalidSLAPolicy("a workspace can have up to %d SLA policies", model.SupportSLAPolicyLimit)
	}
	policy := &model.SupportSLAPolicy{ID: uuid.NewString(), WorkspaceID: workspaceID, Enabled: req.Enabled == nil || *req.Enabled, ActiveSince: s.now()}
	if strings.TrimSpace(actorID) != "" {
		policy.CreatedByID = &actorID
	}
	if err := s.normalizePolicy(ctx, workspaceID, req, policy); err != nil {
		return nil, err
	}
	if err := s.repo.CreatePolicy(ctx, policy); err != nil {
		return nil, err
	}
	return policy, s.repo.EnqueueWorkspace(ctx, workspaceID)
}

// UpdatePolicy replaces a policy's settings. Re-enabling a policy restarts
// the point from which it applies.
func (s *SupportSLAService) UpdatePolicy(ctx context.Context, workspaceID, policyID string, req model.SupportSLAPolicyRequest) (*model.SupportSLAPolicy, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	policy, err := s.repo.GetPolicy(ctx, workspaceID, policyID)
	if err != nil {
		return nil, err
	}
	wasEnabled := policy.Enabled
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	if policy.Enabled && !wasEnabled {
		policy.ActiveSince = s.now()
	}
	if err := s.normalizePolicy(ctx, workspaceID, req, policy); err != nil {
		return nil, err
	}
	if err := s.repo.SavePolicy(ctx, policy); err != nil {
		return nil, err
	}
	return policy, s.repo.EnqueueWorkspace(ctx, workspaceID)
}

// DeletePolicy removes a policy and re-evaluates the conversations it governed.
func (s *SupportSLAService) DeletePolicy(ctx context.Context, workspaceID, policyID string) error {
	if err := s.repo.DeletePolicy(ctx, workspaceID, policyID); err != nil {
		return err
	}
	return s.repo.EnqueueWorkspace(ctx, workspaceID)
}

// ReorderPolicies sets the order in which policies are matched.
func (s *SupportSLAService) ReorderPolicies(ctx context.Context, workspaceID string, policyIDs []string) ([]model.SupportSLAPolicy, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	if err := s.repo.ReorderPolicies(ctx, workspaceID, policyIDs); err != nil {
		return nil, err
	}
	if err := s.repo.EnqueueWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}
	return s.ListPolicies(ctx, workspaceID)
}

func (s *SupportSLAService) clockFor(ctx context.Context, workspaceID string, policy model.SupportSLAPolicy) supportSLAClock {
	if !policy.BusinessHours || s.inbox == nil || s.inbox.installationRepo == nil {
		return newSupportSLAClock(model.SupportInboxSettings{}, false)
	}
	installation, err := s.inbox.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil || installation == nil {
		return newSupportSLAClock(model.SupportInboxSettings{}, false)
	}
	return newSupportSLAClock(parseSettings(installation.Settings), true)
}

type supportSLABreach struct {
	conversation *model.SupportConversation
	record       model.SupportConversationSLARecord
	metrics      []string
}

// EvaluateConversation recomputes one conversation's SLA state and records
// any newly missed target.
func (s *SupportSLAService) EvaluateConversation(ctx context.Context, workspaceID, conversationID string) error {
	var breach *supportSLABreach
	changed := false
	err := s.repo.WithConversationLock(ctx, conversationID, func(repo *repository.SupportSLARepository) error {
		inputs, err := repo.LoadInputs(ctx, workspaceID, conversationID)
		if err != nil {
			return err
		}
		conversation := inputs.Conversation
		if conversation == nil || conversation.MergedIntoID != nil || conversation.AnonymizedAt != nil {
			if inputs.Previous != nil {
				changed = true
				return repo.DeleteRecord(ctx, conversationID)
			}
			return nil
		}
		policies, err := repo.ListPolicies(ctx, workspaceID)
		if err != nil {
			return err
		}
		var policy *model.SupportSLAPolicy
		for i := range policies {
			if supportSLAPolicyMatches(policies[i], *conversation) {
				policy = &policies[i]
				break
			}
		}
		if policy == nil {
			if inputs.Previous != nil {
				changed = true
				return repo.DeleteRecord(ctx, conversationID)
			}
			return nil
		}
		previous := inputs.Previous
		if previous != nil && (previous.PolicyID == nil || *previous.PolicyID != policy.ID) {
			// A different policy starts with fresh breach history.
			previous = nil
		}
		replies := make([]supportSLAReply, 0, len(inputs.Replies))
		for _, row := range inputs.Replies {
			replies = append(replies, supportSLAReply{At: row.CreatedAt, SenderType: row.SenderType})
		}
		evaluation := evaluateSupportSLA(*policy, *conversation, replies, s.clockFor(ctx, workspaceID, *policy), s.now(), previous)
		changed = inputs.Previous == nil || supportSLAVisibleChange(*inputs.Previous, evaluation.Record)
		if err := repo.SaveRecord(ctx, &evaluation.Record); err != nil {
			return err
		}
		if len(evaluation.NewBreaches) > 0 {
			breach = &supportSLABreach{conversation: conversation, record: evaluation.Record, metrics: evaluation.NewBreaches}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if breach != nil {
		s.announceBreach(ctx, breach)
	}
	if changed && s.inbox != nil {
		s.inbox.publishConversationUpdated(&model.SupportConversation{ID: conversationID, WorkspaceID: workspaceID}, "")
	}
	return nil
}

// supportSLAVisibleChange reports whether teammates would see a difference.
func supportSLAVisibleChange(before, after model.SupportConversationSLARecord) bool {
	sameTime := func(a, b *time.Time) bool {
		return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b))
	}
	return derefString(before.PolicyID) != derefString(after.PolicyID) || before.Breached != after.Breached ||
		before.FirstResponseState != after.FirstResponseState || before.NextResponseState != after.NextResponseState ||
		before.ResolutionState != after.ResolutionState || !sameTime(before.NextDueAt, after.NextDueAt) ||
		before.NextResponseBreaches != after.NextResponseBreaches
}

// announceBreach writes the timeline entry and notifies the assignee.
func (s *SupportSLAService) announceBreach(ctx context.Context, breach *supportSLABreach) {
	labels := make([]string, 0, len(breach.metrics))
	for _, metric := range breach.metrics {
		labels = append(labels, supportSLAMetricLabels[metric])
	}
	summary := strings.Join(labels, " and ")
	conversation := breach.conversation
	closed := conversation.Status == model.SupportConversationStatusResolved || conversation.Status == model.SupportConversationStatusSpam
	if s.inbox != nil {
		s.inbox.teammateSystemEvent(ctx, conversation, "", model.SystemEventSLABreached, func(string) string {
			return fmt.Sprintf("SLA missed: %s target (%s).", summary, breach.record.PolicyName)
		}, map[string]any{"metrics": breach.metrics, "policy_id": derefString(breach.record.PolicyID), "policy_name": breach.record.PolicyName})
	}
	if s.notifications == nil || closed || conversation.AssignedUserID == nil || *conversation.AssignedUserID == "" {
		return
	}
	if err := s.notifications.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: conversation.WorkspaceID, EventType: "support_conversation.sla_breached",
		EntityType: "support_conversation", EntityID: conversation.ID,
		Title:    "SLA missed in " + supportNotificationConversationLabel(conversation),
		Body:     fmt.Sprintf("The %s target of %s was missed.", summary, breach.record.PolicyName),
		Category: model.NotifCategorySupportReplies, Priority: "urgent",
		EntitySnapshot:     buildSupportConversationEntitySnapshot(conversation),
		ExplicitRecipients: []string{*conversation.AssignedUserID}, SkipFollowers: true,
	}); err != nil {
		slog.ErrorContext(ctx, "emit SLA breach notification", "conversation_id", conversation.ID, "error", err)
	}
}

// ProcessQueue evaluates queued conversations and conversations whose next
// target is due. It returns how many it evaluated.
func (s *SupportSLAService) ProcessQueue(ctx context.Context, limit int) (int, error) {
	work, err := s.repo.ClaimWork(ctx, limit)
	if err != nil {
		return 0, err
	}
	due, err := s.repo.ListDueWakes(ctx, s.now(), limit)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	processed := 0
	for _, item := range append(work, due...) {
		if seen[item.ConversationID] {
			continue
		}
		seen[item.ConversationID] = true
		if err := s.EvaluateConversation(ctx, item.WorkspaceID, item.ConversationID); err != nil {
			slog.ErrorContext(ctx, "evaluate support SLA", "conversation_id", item.ConversationID, "error", err)
			// Retry later rather than losing the evaluation.
			if requeueErr := s.repo.Enqueue(ctx, item.WorkspaceID, []string{item.ConversationID}, s.now().Add(time.Minute)); requeueErr != nil {
				slog.ErrorContext(ctx, "requeue support SLA evaluation", "conversation_id", item.ConversationID, "error", requeueErr)
			}
			continue
		}
		processed++
	}
	return processed, nil
}

// Run processes the SLA queue until ctx is cancelled.
func (s *SupportSLAService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for {
			processed, err := s.ProcessQueue(ctx, 200)
			if err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "process support SLA queue", "error", err)
			}
			if err != nil || processed < 200 || ctx.Err() != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// HydrateConversations attaches SLA status to conversations.
func (s *SupportSLAService) HydrateConversations(ctx context.Context, workspaceID string, conversations []model.SupportConversation) error {
	if s == nil || len(conversations) == 0 {
		return nil
	}
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	records, err := s.repo.RecordsByConversation(ctx, workspaceID, ids)
	if err != nil {
		return err
	}
	for i := range conversations {
		if record, ok := records[conversations[i].ID]; ok {
			conversations[i].SLA = record.Summary()
		}
	}
	return nil
}
