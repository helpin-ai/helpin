package model

import "time"

// SLA metrics a policy can target.
const (
	SupportSLAMetricFirstResponse = "first_response"
	SupportSLAMetricNextResponse  = "next_response"
	SupportSLAMetricResolution    = "resolution"
)

// Per-metric SLA states shown on a conversation.
const (
	SupportSLAStatePending   = "pending"
	SupportSLAStatePaused    = "paused"
	SupportSLAStateAchieved  = "achieved"
	SupportSLAStateBreached  = "breached"
	SupportSLAStateCancelled = "cancelled"
)

// SupportSLAPolicyMaxTargetMinutes bounds a target at 90 days.
const SupportSLAPolicyMaxTargetMinutes = 90 * 24 * 60

// SupportSLADueSoonWindow is how close a due time must be for a target to
// count as due soon in the inbox. The frontend badge uses the same window.
const SupportSLADueSoonWindow = time.Hour

// SupportSLAPolicyLimit bounds how many policies one workspace keeps.
const SupportSLAPolicyLimit = 50

// SupportSLAPolicy sets response and resolution targets for the
// conversations that match its conditions. The first enabled matching
// policy, by position, applies.
type SupportSLAPolicy struct {
	ID          string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string `json:"name" gorm:"not null"`
	// Boolean columns carry no default: a GORM default would turn a false
	// value into true on insert, so the server always writes them.
	Enabled  bool `json:"enabled" gorm:"not null"`
	Position int  `json:"position" gorm:"type:integer;not null;default:0"`
	// Empty condition lists match every conversation. MailboxIDs may hold
	// "shared" for conversations without a team inbox.
	Priorities DocsStringArray `json:"priorities" gorm:"type:text[];not null;default:'{}'"`
	MailboxIDs DocsStringArray `json:"mailbox_ids" gorm:"type:text[];not null;default:'{}'"`
	Channels   DocsStringArray `json:"channels" gorm:"type:text[];not null;default:'{}'"`
	// CompanyTierIDs matches the SLA tier of the conversation's company;
	// SupportSLANoTier matches conversations whose company has no tier, or
	// that have no company. CompanyIDs matches specific companies.
	CompanyTierIDs DocsStringArray `json:"company_tier_ids" gorm:"type:text[];not null;default:'{}'"`
	CompanyIDs     DocsStringArray `json:"company_ids" gorm:"type:text[];not null;default:'{}'"`
	// RequestTypeIDs matches the conversation's request type.
	RequestTypeIDs       DocsStringArray `json:"request_type_ids" gorm:"type:text[];not null;default:'{}'"`
	FirstResponseMinutes *int            `json:"first_response_minutes,omitempty" gorm:"type:integer"`
	NextResponseMinutes  *int            `json:"next_response_minutes,omitempty" gorm:"type:integer"`
	ResolutionMinutes    *int            `json:"resolution_minutes,omitempty" gorm:"type:integer"`
	// BusinessHours counts only the workspace's business hours when they are
	// configured; otherwise targets run on calendar time.
	BusinessHours bool `json:"business_hours" gorm:"not null;default:false"`
	// PauseWhileWaiting stops the resolution clock while the conversation is
	// waiting on the customer or snoozed.
	PauseWhileWaiting bool `json:"pause_while_waiting" gorm:"not null"`
	// WarnBeforeMinutes warns this many minutes before a target is due; nil
	// turns warnings off. It must be shorter than every target.
	WarnBeforeMinutes *int `json:"warn_before_minutes,omitempty" gorm:"type:integer"`
	// NotifyUserIDs and NotifyCSOwner (the company's customer success owner)
	// hear about warnings and breaches as well as the usual recipients.
	NotifyUserIDs DocsStringArray `json:"notify_user_ids" gorm:"type:text[];not null;default:'{}'"`
	NotifyCSOwner bool            `json:"notify_cs_owner" gorm:"not null;default:false"`
	// AssignUnassignedUserID takes an unassigned conversation when a target
	// is about to be missed.
	AssignUnassignedUserID *string `json:"assign_unassigned_user_id,omitempty" gorm:"type:uuid"`
	// ActiveSince is when the policy was last enabled. It applies only to
	// conversations created after this, so enabling it never back-dates
	// breaches onto older conversations.
	ActiveSince time.Time `json:"active_since" gorm:"type:timestamptz;not null;default:now()"`
	// ConditionsChangedAt is when the conditions last changed. Conversations
	// that start matching through that change are measured from then.
	ConditionsChangedAt *time.Time `json:"conditions_changed_at,omitempty" gorm:"type:timestamptz"`
	CreatedByID         *string    `json:"created_by_id,omitempty" gorm:"type:uuid"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	// Companies names the companies in CompanyIDs, for display.
	Companies []SupportSLAPolicyCompany `json:"companies" gorm:"-"`
}

func (SupportSLAPolicy) TableName() string { return "support_sla_policies" }

// SupportSLAPolicyCompany is a company a policy names.
type SupportSLAPolicyCompany struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SupportSLANoTier in a policy's CompanyTierIDs matches conversations whose
// company has no SLA tier, and conversations without a company.
const SupportSLANoTier = "none"

// SupportSLATierLimit bounds how many tiers one workspace keeps.
const SupportSLATierLimit = 20

// SupportSLAPolicyCompanyLimit bounds the companies one policy names.
const SupportSLAPolicyCompanyLimit = 50

// SupportSLATier is a level of support promised to companies, such as Gold.
// Companies carry at most one tier; policies match it.
type SupportSLATier struct {
	ID string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	// The unique (workspace_id, lower(name)) index serves workspace lookups.
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description" gorm:"not null;default:''"`
	Position    int       `json:"position" gorm:"type:integer;not null;default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"type:timestamptz;not null;default:now();autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"type:timestamptz;not null;default:now();autoUpdateTime"`
	// CompanyCount and PolicyCount show where the tier is used.
	CompanyCount int64 `json:"company_count" gorm:"-"`
	PolicyCount  int   `json:"policy_count" gorm:"-"`
}

func (SupportSLATier) TableName() string { return "support_sla_tiers" }

// SupportSLATierRequest creates or renames a tier.
type SupportSLATierRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ReorderSupportSLATiersRequest sets the display order of tiers.
type ReorderSupportSLATiersRequest struct {
	TierIDs []string `json:"tier_ids"`
}

// SetSupportSLATierRequest sets the tier of companies; a nil TierID clears it.
type SetSupportSLATierRequest struct {
	CompanyIDs []string `json:"company_ids"`
	TierID     *string  `json:"tier_id"`
}

// SupportSLASetTierLimit bounds the companies one tier change covers.
const SupportSLASetTierLimit = 200

// SupportCompanySLA is a company's SLA setup: its tier, and the enabled
// policies that apply to its conversations, in evaluation order. Custom
// marks policies that name only this company.
type SupportCompanySLA struct {
	CompanyID string                  `json:"company_id"`
	TierID    *string                 `json:"tier_id"`
	TierName  *string                 `json:"tier_name"`
	Policies  []SupportCompanySLAItem `json:"policies"`
	HasCustom bool                    `json:"has_custom"`
}

// SupportCompanySLAItem is one policy that may apply to a company.
type SupportCompanySLAItem struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Custom               bool            `json:"custom"`
	Priorities           DocsStringArray `json:"priorities"`
	FirstResponseMinutes *int            `json:"first_response_minutes,omitempty"`
	NextResponseMinutes  *int            `json:"next_response_minutes,omitempty"`
	ResolutionMinutes    *int            `json:"resolution_minutes,omitempty"`
	BusinessHours        bool            `json:"business_hours"`
}

// SupportConversationSLARecord is the evaluated SLA state of one
// conversation. It keeps a snapshot of the targets the conversation started
// with, so later policy edits never re-judge it. Achieved results and
// recorded breaches are final.
type SupportConversationSLARecord struct {
	ConversationID string  `json:"conversation_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID    string  `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_conversation_slas_due,priority:1"`
	PolicyID       *string `json:"policy_id,omitempty" gorm:"type:uuid"`
	PolicyName     string  `json:"policy_name" gorm:"not null;default:''"`
	// PolicyAppliedAt is when the current policy replaced an earlier one;
	// nil when it applied from the start.
	PolicyAppliedAt *time.Time `json:"-" gorm:"type:timestamptz"`
	// DetachedAt is set when no policy applies any more; open targets were
	// cancelled and recorded results kept.
	DetachedAt *time.Time `json:"-" gorm:"type:timestamptz"`
	// Snapshot of the policy's targets when it was applied.
	TargetFirstResponseMinutes *int `json:"-" gorm:"type:integer"`
	TargetNextResponseMinutes  *int `json:"-" gorm:"type:integer"`
	TargetResolutionMinutes    *int `json:"-" gorm:"type:integer"`
	TargetBusinessHours        bool `json:"-" gorm:"not null;default:false"`
	TargetPauseWhileWaiting    bool `json:"-" gorm:"not null"`
	// AIInvolved marks conversations the AI handled first. Their clocks
	// start at HumanSince, the handoff to the team; nil means the AI still
	// owns the conversation or resolved it alone.
	AIInvolved bool       `json:"-" gorm:"not null;default:false"`
	HumanSince *time.Time `json:"-" gorm:"type:timestamptz"`
	// AIHoldingSince is set while the AI answers again after a handoff; the
	// team's targets wait until it hands back.
	AIHoldingSince *time.Time `json:"-" gorm:"type:timestamptz"`
	// TargetsFrom is the earliest time targets count from, set when a policy
	// starts applying to a conversation that already existed.
	TargetsFrom *time.Time `json:"-" gorm:"type:timestamptz"`
	// The due time each target was last warned for: every due time warns
	// once, and a new next-response cycle warns again.
	FirstResponseWarnedFor *time.Time `json:"-" gorm:"type:timestamptz"`
	NextResponseWarnedFor  *time.Time `json:"-" gorm:"type:timestamptz"`
	ResolutionWarnedFor    *time.Time `json:"-" gorm:"type:timestamptz"`
	// CompanyKey is the company and tier the record was matched with. When
	// either changes, a newly selected policy counts from the change. Empty
	// means the record predates SLA tiers.
	CompanyKey string `json:"-" gorm:"not null;default:''"`

	FirstResponseState      string     `json:"first_response_state" gorm:"not null;default:''"`
	FirstResponseDueAt      *time.Time `json:"first_response_due_at,omitempty" gorm:"type:timestamptz"`
	FirstResponseAchievedAt *time.Time `json:"first_response_achieved_at,omitempty" gorm:"type:timestamptz"`
	FirstResponseBreachedAt *time.Time `json:"first_response_breached_at,omitempty" gorm:"type:timestamptz"`
	// FirstResponsePausedSeconds excludes finished AI holds from the first
	// response clock (business seconds in business-hours mode).
	FirstResponsePausedSeconds int64 `json:"-" gorm:"not null;default:0"`

	NextResponseState      string     `json:"next_response_state" gorm:"not null;default:''"`
	NextResponseDueAt      *time.Time `json:"next_response_due_at,omitempty" gorm:"type:timestamptz"`
	NextResponseBreachedAt *time.Time `json:"next_response_breached_at,omitempty" gorm:"type:timestamptz"`
	// NextResponseBreaches counts every missed next-response target so far.
	NextResponseBreaches int `json:"next_response_breaches" gorm:"type:integer;not null;default:0"`
	// NextResponseCarriedBreaches counts breaches recorded under earlier
	// policies, for cycles that ended before the current policy applied.
	NextResponseCarriedBreaches int `json:"-" gorm:"type:integer;not null;default:0"`

	ResolutionState      string     `json:"resolution_state" gorm:"not null;default:''"`
	ResolutionDueAt      *time.Time `json:"resolution_due_at,omitempty" gorm:"type:timestamptz"`
	ResolutionAchievedAt *time.Time `json:"resolution_achieved_at,omitempty" gorm:"type:timestamptz"`
	ResolutionBreachedAt *time.Time `json:"resolution_breached_at,omitempty" gorm:"type:timestamptz"`
	// PausedSince is set while the resolution clock is paused; PausedSeconds
	// accumulates finished pauses (business seconds in business-hours mode).
	PausedSince   *time.Time `json:"paused_since,omitempty" gorm:"type:timestamptz"`
	PausedSeconds int64      `json:"paused_seconds" gorm:"not null;default:0"`

	// NextDueAt is the earliest due time of a target that is still open,
	// including overdue ones; the inbox sorts by it.
	NextDueAt *time.Time `json:"next_due_at,omitempty" gorm:"type:timestamptz;index:idx_support_conversation_slas_due,priority:2"`
	// WakeAt is the earliest due time that has not been breached yet; the
	// sweeper re-evaluates the conversation then to record the breach.
	WakeAt *time.Time `json:"-" gorm:"type:timestamptz;index"`
	// Breached reports an open target that is overdue right now.
	Breached  bool      `json:"breached" gorm:"not null;default:false"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportConversationSLARecord) TableName() string { return "support_conversation_slas" }

// SupportConversationSLA is the SLA status attached to a conversation.
type SupportConversationSLA struct {
	PolicyID   *string                      `json:"policy_id,omitempty"`
	PolicyName string                       `json:"policy_name"`
	Metrics    []SupportConversationSLAItem `json:"metrics"`
	NextDueAt  *time.Time                   `json:"next_due_at,omitempty"`
	Breached   bool                         `json:"breached"`
	// AIHandling means the AI still owns the conversation; the team's
	// targets start at the handoff.
	AIHandling bool `json:"ai_handling,omitempty"`
	// Ended means no policy applies any more; open targets were stopped.
	Ended bool `json:"ended,omitempty"`
}

// SupportConversationSLAItem is one targeted metric on a conversation.
type SupportConversationSLAItem struct {
	Metric     string     `json:"metric"`
	State      string     `json:"state"`
	DueAt      *time.Time `json:"due_at,omitempty"`
	AchievedAt *time.Time `json:"achieved_at,omitempty"`
	BreachedAt *time.Time `json:"breached_at,omitempty"`
}

// Summary converts the stored record to the API shape.
func (r SupportConversationSLARecord) Summary() *SupportConversationSLA {
	summary := &SupportConversationSLA{PolicyID: r.PolicyID, PolicyName: r.PolicyName, NextDueAt: r.NextDueAt, Breached: r.Breached,
		AIHandling: r.DetachedAt == nil && (r.AIHoldingSince != nil ||
			r.AIInvolved && r.HumanSince == nil && (r.FirstResponseState == SupportSLAStatePaused || r.ResolutionState == SupportSLAStatePaused)),
		Ended: r.DetachedAt != nil}
	add := func(metric, state string, due, achieved, breached *time.Time) {
		if state == "" {
			return
		}
		summary.Metrics = append(summary.Metrics, SupportConversationSLAItem{Metric: metric, State: state, DueAt: due, AchievedAt: achieved, BreachedAt: breached})
	}
	add(SupportSLAMetricFirstResponse, r.FirstResponseState, r.FirstResponseDueAt, r.FirstResponseAchievedAt, r.FirstResponseBreachedAt)
	add(SupportSLAMetricNextResponse, r.NextResponseState, r.NextResponseDueAt, nil, r.NextResponseBreachedAt)
	add(SupportSLAMetricResolution, r.ResolutionState, r.ResolutionDueAt, r.ResolutionAchievedAt, r.ResolutionBreachedAt)
	if len(summary.Metrics) == 0 {
		return nil
	}
	return summary
}

// SupportSLAWork queues a conversation for SLA evaluation. Database
// triggers enqueue conversations whenever their status, priority, inbox,
// snooze or public replies change.
type SupportSLAWork struct {
	ConversationID string    `json:"conversation_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null"`
	EnqueuedAt     time.Time `json:"enqueued_at" gorm:"type:timestamptz;not null;index"`
}

func (SupportSLAWork) TableName() string { return "support_sla_work" }

// SupportSLAPolicyRequest creates or replaces a policy.
type SupportSLAPolicyRequest struct {
	Name                   string   `json:"name"`
	Enabled                *bool    `json:"enabled"`
	Priorities             []string `json:"priorities"`
	MailboxIDs             []string `json:"mailbox_ids"`
	Channels               []string `json:"channels"`
	CompanyTierIDs         []string `json:"company_tier_ids"`
	CompanyIDs             []string `json:"company_ids"`
	RequestTypeIDs         []string `json:"request_type_ids"`
	FirstResponseMinutes   *int     `json:"first_response_minutes"`
	NextResponseMinutes    *int     `json:"next_response_minutes"`
	ResolutionMinutes      *int     `json:"resolution_minutes"`
	BusinessHours          bool     `json:"business_hours"`
	PauseWhileWaiting      *bool    `json:"pause_while_waiting"`
	WarnBeforeMinutes      *int     `json:"warn_before_minutes"`
	NotifyUserIDs          []string `json:"notify_user_ids"`
	NotifyCSOwner          bool     `json:"notify_cs_owner"`
	AssignUnassignedUserID *string  `json:"assign_unassigned_user_id"`
}

// SupportSLANotifyLimit bounds the extra teammates a policy notifies.
const SupportSLANotifyLimit = 10

// ReorderSupportSLAPoliciesRequest sets the evaluation order.
type ReorderSupportSLAPoliciesRequest struct {
	PolicyIDs []string `json:"policy_ids"`
}

// UpdateConversationPriorityRequest changes a conversation's priority.
type UpdateConversationPriorityRequest struct {
	Priority string `json:"priority"`
}

// SnoozeConversationRequest snoozes a conversation until a time.
type SnoozeConversationRequest struct {
	Until time.Time `json:"until"`
}

// Conversation priorities.
const (
	SupportConversationPriorityLow    = "low"
	SupportConversationPriorityMedium = "medium"
	SupportConversationPriorityHigh   = "high"
	SupportConversationPriorityUrgent = "urgent"
)

// IsValidSupportConversationPriority reports whether p is a known priority.
func IsValidSupportConversationPriority(p string) bool {
	switch p {
	case SupportConversationPriorityLow, SupportConversationPriorityMedium, SupportConversationPriorityHigh, SupportConversationPriorityUrgent:
		return true
	}
	return false
}
