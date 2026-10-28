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
	webhooks      *WebhookService
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

// SetWebhookService emits sla.breached events.
func (s *SupportSLAService) SetWebhookService(webhooks *WebhookService) *SupportSLAService {
	s.webhooks = webhooks
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
	if err := s.normalizePolicyCompanies(ctx, workspaceID, req, policy); err != nil {
		return err
	}
	if err := s.normalizePolicyAlerts(ctx, workspaceID, req, policy); err != nil {
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
	if err != nil {
		return nil, err
	}
	if policies == nil {
		policies = []model.SupportSLAPolicy{}
	}
	targets := make([]*model.SupportSLAPolicy, len(policies))
	for i := range policies {
		targets[i] = &policies[i]
	}
	return policies, s.attachPolicyCompanies(ctx, workspaceID, targets...)
}

// CreatePolicy adds a policy after the existing ones. It applies to
// conversations created from now on, so nothing needs re-evaluating.
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
	return policy, s.attachPolicyCompanies(ctx, workspaceID, policy)
}

// UpdatePolicy replaces a policy's settings. New targets apply to
// conversations that start the policy afterwards; conversations already on
// it keep the targets they started with. Condition changes and disabling
// re-check open conversations. Re-enabling a policy restarts the point from
// which it applies.
func (s *SupportSLAService) UpdatePolicy(ctx context.Context, workspaceID, policyID string, req model.SupportSLAPolicyRequest) (*model.SupportSLAPolicy, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	policy, err := s.repo.GetPolicy(ctx, workspaceID, policyID)
	if err != nil {
		return nil, err
	}
	wasEnabled := policy.Enabled
	before := [5][]string{policy.Priorities, policy.MailboxIDs, policy.Channels, policy.CompanyTierIDs, policy.CompanyIDs}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	if policy.Enabled && !wasEnabled {
		policy.ActiveSince = s.now()
	}
	if err := s.normalizePolicy(ctx, workspaceID, req, policy); err != nil {
		return nil, err
	}
	after := [5][]string{policy.Priorities, policy.MailboxIDs, policy.Channels, policy.CompanyTierIDs, policy.CompanyIDs}
	for i := range before {
		if !sameSLAConditionValues(before[i], after[i]) {
			policy.ConditionsChangedAt = timePtr(s.now())
			break
		}
	}
	if err := s.repo.SavePolicy(ctx, policy); err != nil {
		return nil, err
	}
	if err := s.attachPolicyCompanies(ctx, workspaceID, policy); err != nil {
		return nil, err
	}
	return policy, s.repo.EnqueueOpenRecords(ctx, workspaceID, s.now())
}

// DeletePolicy removes a policy. Open conversations it governed move to the
// next matching policy or stop SLA tracking; recorded results stay.
func (s *SupportSLAService) DeletePolicy(ctx context.Context, workspaceID, policyID string) error {
	if err := s.repo.DeletePolicy(ctx, workspaceID, policyID); err != nil {
		return err
	}
	return s.repo.EnqueueOpenRecords(ctx, workspaceID, s.now())
}

// ReorderPolicies sets the order in which policies are matched.
func (s *SupportSLAService) ReorderPolicies(ctx context.Context, workspaceID string, policyIDs []string) ([]model.SupportSLAPolicy, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	if err := s.repo.ReorderPolicies(ctx, workspaceID, policyIDs); err != nil {
		return nil, err
	}
	if err := s.repo.EnqueueOpenRecords(ctx, workspaceID, s.now()); err != nil {
		return nil, err
	}
	return s.ListPolicies(ctx, workspaceID)
}

type supportSLABreach struct {
	conversation *model.SupportConversation
	record       model.SupportConversationSLARecord
	metrics      []string
	// policy and company add the policy's extra recipients; policy is nil
	// for conversations that closed with the targets they had.
	policy  *model.SupportSLAPolicy
	company *repository.SupportSLACompany
}

// supportSLABatch caches per-workspace inputs across one queue pass.
type supportSLABatch struct {
	policies map[string][]model.SupportSLAPolicy
	clocks   map[string]supportSLAClock
}

func newSupportSLABatch() *supportSLABatch {
	return &supportSLABatch{policies: map[string][]model.SupportSLAPolicy{}, clocks: map[string]supportSLAClock{}}
}

// workspacePolicies returns the enabled policies, or none when the
// workspace no longer has the SLA feature.
func (s *SupportSLAService) workspacePolicies(ctx context.Context, batch *supportSLABatch, workspaceID string) ([]model.SupportSLAPolicy, error) {
	if policies, ok := batch.policies[workspaceID]; ok {
		return policies, nil
	}
	var policies []model.SupportSLAPolicy
	entitled := true
	if s.entitlements != nil {
		if err := s.entitlements.RequireFeature(ctx, workspaceID, EntitlementFeatureSLAPolicies); err != nil {
			var denied *EntitlementError
			if !errors.As(err, &denied) {
				return nil, err
			}
			entitled = false
		}
	}
	if entitled {
		all, err := s.repo.ListPolicies(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, policy := range all {
			if policy.Enabled {
				policies = append(policies, policy)
			}
		}
	}
	batch.policies[workspaceID] = policies
	return policies, nil
}

func (s *SupportSLAService) clockFor(ctx context.Context, batch *supportSLABatch, workspaceID string, businessHours bool) supportSLAClock {
	if !businessHours || s.inbox == nil || s.inbox.installationRepo == nil {
		return newSupportSLAClock(model.SupportInboxSettings{}, false)
	}
	if clock, ok := batch.clocks[workspaceID]; ok {
		return clock
	}
	clock := newSupportSLAClock(model.SupportInboxSettings{}, false)
	installation, err := s.inbox.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "load business hours for SLA", "workspace_id", workspaceID, "error", err)
	} else if installation != nil {
		clock = newSupportSLAClock(parseSettings(installation.Settings), true)
	}
	batch.clocks[workspaceID] = clock
	return clock
}

// supportSLAHandoff works out whether the AI handled a conversation first
// and when the team took over. A handoff time, once recorded, is kept.
func supportSLAHandoff(conversation model.SupportConversation, replies []repository.SupportSLAReplyRow, previous *model.SupportConversationSLARecord, now time.Time) (involved bool, humanSince *time.Time, aiOwnsNow bool) {
	closed := conversation.Status == model.SupportConversationStatusResolved || conversation.Status == model.SupportConversationStatusSpam
	aiOwnsNow = !closed && conversation.AIOwnsResponse()
	involved = aiOwnsNow || conversation.AIState != nil || (previous != nil && previous.AIInvolved)
	var firstTeammate *time.Time
	for _, reply := range replies {
		switch reply.SenderType {
		case "ai", "agent":
			involved = true
		case "user":
			if firstTeammate == nil {
				firstTeammate = timePtr(reply.CreatedAt)
			}
		}
	}
	if !involved {
		return false, nil, false
	}
	if previous != nil && previous.HumanSince != nil {
		return true, previous.HumanSince, aiOwnsNow
	}
	if handoff := earliestTime(conversation.AIEscalatedAt, conversation.CustomerRequestedHumanAt, conversation.AIPausedAt, firstTeammate); handoff != nil {
		return true, handoff, aiOwnsNow
	}
	if aiOwnsNow || closed {
		// The AI still answers, or resolved the conversation on its own.
		return true, nil, aiOwnsNow
	}
	// The team took over without a recorded time.
	return true, timePtr(now), false
}

// EvaluateConversation recomputes one conversation's SLA state and records
// any newly missed target.
func (s *SupportSLAService) EvaluateConversation(ctx context.Context, workspaceID, conversationID string) error {
	return s.evaluateConversation(ctx, newSupportSLABatch(), workspaceID, conversationID)
}

func (s *SupportSLAService) evaluateConversation(ctx context.Context, batch *supportSLABatch, workspaceID, conversationID string) error {
	var breach *supportSLABreach
	var warning *supportSLAWarning
	changed := false
	now := s.now()
	err := s.repo.WithConversationLock(ctx, conversationID, func(repo *repository.SupportSLARepository) error {
		inputs, err := repo.LoadInputs(ctx, workspaceID, conversationID)
		if err != nil {
			return err
		}
		conversation, previous := inputs.Conversation, inputs.Previous
		if conversation == nil || conversation.MergedIntoID != nil || conversation.AnonymizedAt != nil {
			if previous != nil {
				changed = true
				return repo.DeleteRecord(ctx, conversationID)
			}
			return nil
		}
		closed := conversation.Status == model.SupportConversationStatusResolved || conversation.Status == model.SupportConversationStatusSpam
		// Imported conversations carry targets only from the cutover, the
		// first evaluation that finds them open; their history was answered
		// elsewhere and is never judged.
		imported := conversation.Source == model.SupportImportSourceIntercom
		if imported && previous == nil && closed {
			return nil
		}
		matching := *conversation
		if imported {
			matching.CreatedAt = now
			if previous != nil && previous.TargetsFrom != nil {
				matching.CreatedAt = *previous.TargetsFrom
			}
		}
		companyKey := supportSLACompanyKey(inputs.Company)
		in := supportSLAInput{Conversation: *conversation, Now: now, Previous: previous, CompanyKey: companyKey}
		var policy *model.SupportSLAPolicy
		var snapshot supportSLATargets
		hasSnapshot := false
		if previous != nil && previous.DetachedAt == nil {
			snapshot, hasSnapshot = supportSLATargetsFromRecord(*previous)
		}
		switch {
		case closed && previous != nil:
			// Closed conversations finish with the targets they had. A record
			// without targets only stops waking and showing as due.
			if !hasSnapshot {
				if previous.WakeAt == nil && previous.NextDueAt == nil && !previous.Breached {
					return nil
				}
				record := *previous
				record.WakeAt, record.NextDueAt, record.Breached = nil, nil, false
				changed = true
				return repo.SaveRecord(ctx, &record)
			}
			in.Targets, in.AppliedAt, in.TargetsFrom = snapshot, previous.PolicyAppliedAt, previous.TargetsFrom
		default:
			policies, err := s.workspacePolicies(ctx, batch, workspaceID)
			if err != nil {
				return err
			}
			for i := range policies {
				if supportSLAPolicyMatches(policies[i], matching, inputs.Company) {
					policy = &policies[i]
					break
				}
			}
			switch {
			case policy == nil && (previous == nil || previous.DetachedAt != nil):
				return nil
			case policy == nil:
				record := detachSupportSLARecord(*previous, now)
				record.CompanyKey = companyKey
				changed = supportSLAVisibleChange(*previous, record)
				return repo.SaveRecord(ctx, &record)
			case hasSnapshot && snapshot.PolicyID == policy.ID:
				in.Targets, in.AppliedAt, in.TargetsFrom = snapshot, previous.PolicyAppliedAt, previous.TargetsFrom
			default:
				in.Targets = supportSLATargetsFromPolicy(*policy)
				in.AppliedAt, in.TargetsFrom = supportSLAPolicyStart(*policy, matching, inputs.Company, previous, companyKey, now)
				in.Switched = previous != nil
				if imported && previous == nil {
					in.AppliedAt, in.TargetsFrom = timePtr(now), timePtr(now)
				}
			}
		}
		// Only messages copied from the other help desk are history; a later
		// cutover from a company change never turns live replies into it.
		in.HistoryThrough = inputs.ImportedThrough
		in.Replies = make([]supportSLAReply, 0, len(inputs.Replies))
		for _, row := range inputs.Replies {
			in.Replies = append(in.Replies, supportSLAReply{At: row.CreatedAt, SenderType: row.SenderType})
		}
		in.AIInvolved, in.HumanSince, in.AIOwnsNow = supportSLAHandoff(*conversation, inputs.Replies, previous, now)
		in.Resumed = !in.Switched && previous != nil && previous.AIHoldingSince != nil && !in.AIOwnsNow
		in.Clock = s.clockFor(ctx, batch, workspaceID, in.Targets.BusinessHours)
		evaluation := evaluateSupportSLA(in)
		var warnBefore *int
		if policy != nil {
			warnBefore = policy.WarnBeforeMinutes
		}
		warned := applySupportSLAWarnings(&evaluation.Record, previous, warnBefore, in.Clock, now, in.Switched)
		changed = previous == nil || supportSLAVisibleChange(*previous, evaluation.Record)
		if err := repo.SaveRecord(ctx, &evaluation.Record); err != nil {
			return err
		}
		if len(evaluation.NewBreaches) > 0 {
			breach = &supportSLABreach{conversation: conversation, record: evaluation.Record, metrics: evaluation.NewBreaches, policy: policy, company: inputs.Company}
		}
		if len(warned) > 0 && policy != nil {
			warning = &supportSLAWarning{conversation: conversation, record: evaluation.Record, policy: *policy, company: inputs.Company, metrics: warned}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if breach != nil {
		s.announceBreach(ctx, breach)
	}
	if warning != nil {
		s.announceWarning(ctx, warning)
	}
	if changed && s.inbox != nil {
		s.inbox.publishConversationUpdated(&model.SupportConversation{ID: conversationID, WorkspaceID: workspaceID}, "")
	}
	return nil
}

// supportSLAPolicyStart returns when a newly selected policy applies and the
// earliest time its targets count from.
//   - Applying again after no policy did, or after the conversation's company
//     or its tier changed, counts from now: the promise did not exist before.
//   - Replacing another policy for any other reason (priority, inbox,
//     channel) keeps the original start, so a stricter policy can make a
//     target overdue at once.
//   - A first policy counts from the conversation's start, unless the
//     conversation only matches through a later condition or tier change.
func supportSLAPolicyStart(policy model.SupportSLAPolicy, conversation model.SupportConversation, company *repository.SupportSLACompany, previous *model.SupportConversationSLARecord, companyKey string, now time.Time) (appliedAt, targetsFrom *time.Time) {
	switch {
	case previous != nil && previous.DetachedAt != nil:
		return timePtr(now), timePtr(now)
	case previous != nil && previous.CompanyKey != "" && previous.CompanyKey != companyKey:
		return timePtr(now), timePtr(now)
	case previous != nil:
		return timePtr(now), previous.TargetsFrom
	}
	// The latest change after the conversation started that made it match.
	var from *time.Time
	later := func(at *time.Time) {
		if at != nil && conversation.CreatedAt.Before(*at) && (from == nil || at.After(*from)) {
			from = at
		}
	}
	later(policy.ConditionsChangedAt)
	if company != nil && (len(policy.CompanyTierIDs) > 0 || len(policy.CompanyIDs) > 0) {
		later(company.LinkedAt)
		if len(policy.CompanyTierIDs) > 0 {
			later(company.TierChangedAt)
		}
	}
	return from, from
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
	s.webhooks.Emit(ctx, conversation.WorkspaceID, model.WebhookEventSLABreached, conversation.ID, map[string]any{
		"sla": map[string]any{"metrics": breach.metrics, "policy_id": derefString(breach.record.PolicyID), "policy_name": breach.record.PolicyName},
	})
	if s.notifications == nil || closed {
		return
	}
	recipients := s.alertRecipients(ctx, conversation, breach.policy, breach.company)
	if len(recipients) == 0 {
		return
	}
	if err := s.notifications.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: conversation.WorkspaceID, EventType: "support_conversation.sla_breached",
		EntityType: "support_conversation", EntityID: conversation.ID,
		Title:    "SLA missed in " + supportNotificationConversationLabel(conversation),
		Body:     fmt.Sprintf("The %s target of %s was missed.", summary, breach.record.PolicyName),
		Category: model.NotifCategorySupportReplies, Priority: "urgent",
		EntitySnapshot:     buildSupportConversationEntitySnapshot(conversation),
		ExplicitRecipients: recipients, SkipFollowers: true,
	}); err != nil {
		slog.ErrorContext(ctx, "emit SLA breach notification", "conversation_id", conversation.ID, "error", err)
	}
}

// supportSLABreachRecipientLimit bounds notifications for one breach.
const supportSLABreachRecipientLimit = 50

// breachRecipients is the assignee, or the inbox's members when nobody is
// assigned, or the workspace's support teammates for the shared inbox.
func (s *SupportSLAService) breachRecipients(ctx context.Context, conversation *model.SupportConversation) []string {
	if conversation.AssignedUserID != nil && *conversation.AssignedUserID != "" {
		return []string{*conversation.AssignedUserID}
	}
	if s.inbox == nil {
		return nil
	}
	var recipients []string
	if conversation.MailboxID != nil && *conversation.MailboxID != "" && s.inbox.mailboxRepo != nil {
		userIDs, err := s.inbox.mailboxRepo.ListActiveMemberUserIDs(ctx, conversation.WorkspaceID, *conversation.MailboxID)
		if err != nil {
			slog.ErrorContext(ctx, "list inbox members for SLA breach", "conversation_id", conversation.ID, "error", err)
			return nil
		}
		recipients = userIDs
	} else if s.inbox.workspaceRepo != nil {
		members, err := s.inbox.workspaceRepo.ListSupportAssignableMembers(ctx, conversation.WorkspaceID, nil)
		if err != nil {
			slog.ErrorContext(ctx, "list support teammates for SLA breach", "conversation_id", conversation.ID, "error", err)
			return nil
		}
		for _, member := range members {
			if member.UserID != nil && *member.UserID != "" {
				recipients = append(recipients, *member.UserID)
			}
		}
	}
	if len(recipients) > supportSLABreachRecipientLimit {
		recipients = recipients[:supportSLABreachRecipientLimit]
	}
	return recipients
}

// supportSLAWorkLease is how long a claimed queue row stays hidden from
// other workers; a crashed worker's rows return after it.
const supportSLAWorkLease = 2 * time.Minute

// ProcessQueue evaluates queued conversations and conversations whose next
// target is due. It returns how many it evaluated.
func (s *SupportSLAService) ProcessQueue(ctx context.Context, limit int) (int, error) {
	now := s.now()
	work, err := s.repo.ClaimWork(ctx, now, now.Add(supportSLAWorkLease), limit)
	if err != nil {
		return 0, err
	}
	due, err := s.repo.ListDueWakes(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	claimed := make(map[string]model.SupportSLAWork, len(work))
	for _, item := range work {
		claimed[item.ConversationID] = item
	}
	batch := newSupportSLABatch()
	seen := map[string]bool{}
	processed := 0
	for _, item := range append(work, due...) {
		if seen[item.ConversationID] {
			continue
		}
		if ctx.Err() != nil {
			// Unprocessed claims return when their lease ends.
			break
		}
		seen[item.ConversationID] = true
		lease, wasClaimed := claimed[item.ConversationID]
		if err := s.evaluateConversation(ctx, batch, item.WorkspaceID, item.ConversationID); err != nil {
			slog.ErrorContext(ctx, "evaluate support SLA", "conversation_id", item.ConversationID, "error", err)
			// Retry later rather than losing the evaluation, and keep a failing
			// conversation from holding up other due wake-ups.
			retryCtx := context.WithoutCancel(ctx)
			if delayErr := s.repo.DelayWake(retryCtx, item.ConversationID, s.now().Add(time.Minute)); delayErr != nil {
				slog.ErrorContext(ctx, "delay support SLA wake-up", "conversation_id", item.ConversationID, "error", delayErr)
			}
			if requeueErr := s.repo.Enqueue(retryCtx, item.WorkspaceID, []string{item.ConversationID}, s.now().Add(time.Minute)); requeueErr != nil {
				slog.ErrorContext(ctx, "requeue support SLA evaluation", "conversation_id", item.ConversationID, "error", requeueErr)
			}
			continue
		}
		if wasClaimed {
			if err := s.repo.CompleteWork(context.WithoutCancel(ctx), lease); err != nil {
				slog.ErrorContext(ctx, "complete support SLA evaluation", "conversation_id", item.ConversationID, "error", err)
			}
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

func sameSLAConditionValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, value := range a {
		if !slices.Contains(b, value) {
			return false
		}
	}
	return true
}
