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

// SupportSLAPolicyLimit bounds how many policies one workspace keeps.
const SupportSLAPolicyLimit = 50

// SupportSLAPolicy sets response and resolution targets for the
// conversations that match its conditions. The first enabled matching
// policy, by position, applies.
type SupportSLAPolicy struct {
	ID          string `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string `json:"name" gorm:"not null"`
	Enabled     bool   `json:"enabled" gorm:"not null;default:true"`
	Position    int    `json:"position" gorm:"type:integer;not null;default:0"`
	// Empty condition lists match every conversation. MailboxIDs may hold
	// "shared" for conversations without a team inbox.
	Priorities           DocsStringArray `json:"priorities" gorm:"type:text[];not null;default:'{}'"`
	MailboxIDs           DocsStringArray `json:"mailbox_ids" gorm:"type:text[];not null;default:'{}'"`
	Channels             DocsStringArray `json:"channels" gorm:"type:text[];not null;default:'{}'"`
	FirstResponseMinutes *int            `json:"first_response_minutes,omitempty" gorm:"type:integer"`
	NextResponseMinutes  *int            `json:"next_response_minutes,omitempty" gorm:"type:integer"`
	ResolutionMinutes    *int            `json:"resolution_minutes,omitempty" gorm:"type:integer"`
	// BusinessHours counts only the workspace's business hours when they are
	// configured; otherwise targets run on calendar time.
	BusinessHours bool `json:"business_hours" gorm:"not null;default:false"`
	// PauseWhileWaiting stops the resolution clock while the conversation is
	// waiting on the customer or snoozed.
	PauseWhileWaiting bool `json:"pause_while_waiting" gorm:"not null;default:true"`
	// ActiveSince is when the policy was last enabled. It applies only to
	// conversations created after this, so enabling it never back-dates
	// breaches onto older conversations.
	ActiveSince time.Time `json:"active_since" gorm:"type:timestamptz;not null;default:now()"`
	CreatedByID *string   `json:"created_by_id,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportSLAPolicy) TableName() string { return "support_sla_policies" }

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
	TargetPauseWhileWaiting    bool `json:"-" gorm:"not null;default:true"`
	// AIInvolved marks conversations the AI handled first. Their clocks
	// start at HumanSince, the handoff to the team; nil means the AI still
	// owns the conversation or resolved it alone.
	AIInvolved bool       `json:"-" gorm:"not null;default:false"`
	HumanSince *time.Time `json:"-" gorm:"type:timestamptz"`

	FirstResponseState      string     `json:"first_response_state" gorm:"not null;default:''"`
	FirstResponseDueAt      *time.Time `json:"first_response_due_at,omitempty" gorm:"type:timestamptz"`
	FirstResponseAchievedAt *time.Time `json:"first_response_achieved_at,omitempty" gorm:"type:timestamptz"`
	FirstResponseBreachedAt *time.Time `json:"first_response_breached_at,omitempty" gorm:"type:timestamptz"`

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
		AIHandling: r.AIInvolved && r.HumanSince == nil && r.DetachedAt == nil, Ended: r.DetachedAt != nil}
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
	Name                 string   `json:"name"`
	Enabled              *bool    `json:"enabled"`
	Priorities           []string `json:"priorities"`
	MailboxIDs           []string `json:"mailbox_ids"`
	Channels             []string `json:"channels"`
	FirstResponseMinutes *int     `json:"first_response_minutes"`
	NextResponseMinutes  *int     `json:"next_response_minutes"`
	ResolutionMinutes    *int     `json:"resolution_minutes"`
	BusinessHours        bool     `json:"business_hours"`
	PauseWhileWaiting    *bool    `json:"pause_while_waiting"`
}

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
