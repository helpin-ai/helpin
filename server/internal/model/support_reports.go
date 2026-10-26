package model

import "time"

// SupportReportMaxRange bounds how much history one report reads.
const SupportReportMaxRange = 366 * 24 * time.Hour

// SupportReportRequest selects the period and inboxes a report covers.
type SupportReportRequest struct {
	WorkspaceID string
	From        time.Time
	To          time.Time
	// Timezone is an IANA name used for the daily series.
	Timezone   string
	MailboxIDs []string
	Channels   []string
}

// SupportReportDuration summarizes a set of elapsed times in seconds.
type SupportReportDuration struct {
	Count         int    `json:"count"`
	MedianSeconds *int64 `json:"median_seconds"`
	P90Seconds    *int64 `json:"p90_seconds"`
}

// SupportReportSummary is the headline metrics for one period.
type SupportReportSummary struct {
	ConversationsCreated  int `json:"conversations_created"`
	ConversationsResolved int `json:"conversations_resolved"`
	ResolvedByAI          int `json:"resolved_by_ai"`
	ResolvedByHuman       int `json:"resolved_by_human"`
	// FirstResponse counts any first reply, including the AI agent's.
	FirstResponse      SupportReportDuration `json:"first_response"`
	FirstHumanResponse SupportReportDuration `json:"first_human_response"`
	Resolution         SupportReportDuration `json:"resolution"`
	AIInvolved         int                   `json:"ai_involved"`
	AIResolved         int                   `json:"ai_resolved"`
	AIResolvedConfirm  int                   `json:"ai_resolved_confirmed"`
	AIResolvedAssumed  int                   `json:"ai_resolved_assumed"`
	AIHandoffs         int                   `json:"ai_handoffs"`
	AIInvolvementRate  *float64              `json:"ai_involvement_rate"`
	AIResolutionRate   *float64              `json:"ai_resolution_rate"`
	AIHandoffRate      *float64              `json:"ai_handoff_rate"`
	CSATResponses      int                   `json:"csat_responses"`
	CSATAverage        *float64              `json:"csat_average"`
	CSATPositiveRate   *float64              `json:"csat_positive_rate"`
}

// SupportReportSLATarget counts met and missed targets of one SLA metric.
type SupportReportSLATarget struct {
	Met     int      `json:"met"`
	Missed  int      `json:"missed"`
	HitRate *float64 `json:"hit_rate"`
}

// SupportReportSLA summarizes SLA outcomes of conversations created in the
// period. Targets still running are not counted yet.
type SupportReportSLA struct {
	Conversations int                    `json:"conversations"`
	FirstResponse SupportReportSLATarget `json:"first_response"`
	NextResponse  SupportReportSLATarget `json:"next_response"`
	Resolution    SupportReportSLATarget `json:"resolution"`
}

// SupportReportBacklog is the number of conversations open right now.
type SupportReportBacklog struct {
	Open       int `json:"open"`
	Waiting    int `json:"waiting_on_customer"`
	Unassigned int `json:"unassigned"`
}

// SupportReportDay is one calendar day in the report timezone.
type SupportReportDay struct {
	Date                      string `json:"date"`
	ConversationsCreated      int    `json:"conversations_created"`
	ConversationsResolved     int    `json:"conversations_resolved"`
	ResolvedByAI              int    `json:"resolved_by_ai"`
	MedianFirstHumanResponseS *int64 `json:"median_first_human_response_seconds"`
}

// SupportReportChannel breaks the period down by channel.
type SupportReportChannel struct {
	Channel               string                `json:"channel"`
	ConversationsCreated  int                   `json:"conversations_created"`
	ConversationsResolved int                   `json:"conversations_resolved"`
	FirstHumanResponse    SupportReportDuration `json:"first_human_response"`
}

// SupportReportTeammate is one teammate's activity in the period.
type SupportReportTeammate struct {
	UserID             string                `json:"user_id"`
	Name               string                `json:"name"`
	AvatarURL          *string               `json:"avatar_url,omitempty"`
	Resolved           int                   `json:"resolved"`
	RepliesSent        int                   `json:"replies_sent"`
	FirstHumanResponse SupportReportDuration `json:"first_human_response"`
	CSATResponses      int                   `json:"csat_responses"`
	CSATAverage        *float64              `json:"csat_average"`
}

// SupportReportCSATComment is a recent rating with a comment.
type SupportReportCSATComment struct {
	ConversationID string    `json:"conversation_id"`
	DisplayID      int       `json:"display_id"`
	Subject        string    `json:"subject"`
	Rating         int       `json:"rating"`
	Comment        string    `json:"comment"`
	Channel        string    `json:"channel"`
	ResolvedBy     string    `json:"resolved_by"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

// SupportReportCSAT details satisfaction ratings for the period.
type SupportReportCSAT struct {
	// Distribution counts ratings 1 through 5 (index 0 is rating 1).
	Distribution   [5]int                     `json:"distribution"`
	AIAverage      *float64                   `json:"ai_average"`
	AIResponses    int                        `json:"ai_responses"`
	HumanAverage   *float64                   `json:"human_average"`
	HumanResponses int                        `json:"human_responses"`
	RecentComments []SupportReportCSATComment `json:"recent_comments"`
}

// SupportReportRange echoes the periods the report compares.
type SupportReportRange struct {
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	PreviousFrom time.Time `json:"previous_from"`
	PreviousTo   time.Time `json:"previous_to"`
	Timezone     string    `json:"timezone"`
}

// SupportReportOverview is the support reports page.
type SupportReportOverview struct {
	Range     SupportReportRange      `json:"range"`
	Summary   SupportReportSummary    `json:"summary"`
	Previous  SupportReportSummary    `json:"previous"`
	Backlog   SupportReportBacklog    `json:"backlog"`
	Daily     []SupportReportDay      `json:"daily"`
	Channels  []SupportReportChannel  `json:"channels"`
	Teammates []SupportReportTeammate `json:"teammates"`
	CSAT      SupportReportCSAT       `json:"csat"`
	SLA       SupportReportSLA        `json:"sla"`
}
