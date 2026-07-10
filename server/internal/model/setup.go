package model

import "time"

const (
	SetupGoalFoundation        = "foundation"
	SetupGoalProductDelivery   = "product_delivery"
	SetupGoalCustomerSupport   = "customer_support"
	SetupGoalAutomationMastery = "automation_mastery"
	SetupGoalTeamProjects      = "team_project_management"
	SetupGoalHelpCenterDocs    = "help_center_docs"
	SetupGoalInternalDocs      = "internal_docs"
	SetupGoalSalesCRM          = "sales_crm"

	SetupTaskAvailable      = "available"
	SetupTaskCompleted      = "completed"
	SetupTaskBlocked        = "blocked"
	SetupTaskUnavailable    = "unavailable"
	SetupTaskUnableToVerify = "unable_to_verify"
	SetupTaskNeedsAttention = "needs_attention"

	SetupMaturityPreparing   = "preparing"
	SetupMaturityReady       = "ready"
	SetupMaturityActivated   = "activated"
	SetupMaturityEstablished = "established"
	SetupMaturityAdvanced    = "advanced"
)

// SetupGoal records the outcomes a workspace is intentionally adopting.
type SetupGoal struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_setup_goal_workspace_key,priority:1"`
	Key            string    `json:"key" gorm:"not null;uniqueIndex:idx_setup_goal_workspace_key,priority:2"`
	CatalogVersion int       `json:"catalog_version" gorm:"not null;default:1"`
	Source         string    `json:"source" gorm:"not null;default:'onboarding'"`
	Status         string    `json:"status" gorm:"not null;default:'active';index"`
	Position       int       `json:"position" gorm:"not null;default:0"`
	ActivatedAt    time.Time `json:"activated_at" gorm:"not null"`
	CreatedBy      string    `json:"created_by" gorm:"type:uuid;not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SetupGoal) TableName() string { return "setup_goals" }

// SetupIntent atomically preserves onboarding choices until goal rows are initialized.
type SetupIntent struct {
	WorkspaceID string    `json:"-" gorm:"type:uuid;primaryKey"`
	GoalKeys    string    `json:"-" gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt   time.Time `json:"-" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"-" gorm:"autoUpdateTime"`
}

func (SetupIntent) TableName() string { return "setup_intents" }

// SetupAchievement durably records verified value so later data cleanup cannot erase success.
type SetupAchievement struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_setup_achievement,priority:1"`
	GoalKey     string    `json:"goal_key" gorm:"not null;uniqueIndex:idx_setup_achievement,priority:2"`
	TaskKey     string    `json:"task_key" gorm:"not null;uniqueIndex:idx_setup_achievement,priority:3"`
	MemberID    string    `json:"member_id" gorm:"not null;default:'';uniqueIndex:idx_setup_achievement,priority:4"`
	Evidence    string    `json:"evidence" gorm:"type:jsonb;not null;default:'{}'"`
	AchievedAt  time.Time `json:"achieved_at" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SetupAchievement) TableName() string { return "setup_achievements" }

// SetupActionIntent records that a member launched a recommended setup action.
type SetupActionIntent struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MemberID    string    `json:"member_id" gorm:"type:uuid;not null;index"`
	GoalKey     string    `json:"goal_key" gorm:"not null"`
	TaskKey     string    `json:"task_key" gorm:"not null;index"`
	ActionKey   string    `json:"action_key" gorm:"not null"`
	StartedAt   time.Time `json:"started_at" gorm:"not null;index"`
}

func (SetupActionIntent) TableName() string { return "setup_action_intents" }

// MemberSetupPreference keeps the guide personal without hiding shared workspace progress.
type MemberSetupPreference struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_member_setup_pref,priority:1"`
	UserID           string    `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_member_setup_pref,priority:2"`
	SidebarDismissed bool      `json:"sidebar_dismissed" gorm:"not null;default:false"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (MemberSetupPreference) TableName() string { return "member_setup_preferences" }

type SetupAction struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type SetupTask struct {
	Key           string       `json:"key"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Stage         string       `json:"stage"`
	Status        string       `json:"status"`
	Shared        bool         `json:"shared"`
	Core          bool         `json:"core"`
	BlockedReason string       `json:"blocked_reason,omitempty"`
	Action        *SetupAction `json:"action,omitempty"`
}

type SetupJourney struct {
	Key            string      `json:"key"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Accent         string      `json:"accent"`
	Scope          string      `json:"scope"`
	Maturity       string      `json:"maturity"`
	CompletedCount int         `json:"completed_count"`
	TotalCount     int         `json:"total_count"`
	Tasks          []SetupTask `json:"tasks"`
}

type SetupPlaceholderGoal struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SetupRecommendation struct {
	JourneyKey string      `json:"journey_key"`
	TaskKey    string      `json:"task_key"`
	Title      string      `json:"title"`
	Reason     string      `json:"reason"`
	Action     SetupAction `json:"action"`
}

type SetupView struct {
	Goals            []string               `json:"goals"`
	Journeys         []SetupJourney         `json:"journeys"`
	Recommended      *SetupRecommendation   `json:"recommended,omitempty"`
	Preference       MemberSetupPreference  `json:"preference"`
	CompletedCount   int                    `json:"completed_count"`
	TotalCount       int                    `json:"total_count"`
	PlaceholderGoals []SetupPlaceholderGoal `json:"placeholder_goals"`
}

// SetupEvidence contains only outcomes that can be verified from durable product data.
type SetupEvidence struct {
	HasCompanyContext            bool                 `json:"-"`
	TeamCount                    int64                `json:"-"`
	InvitationCount              int64                `json:"-"`
	ActiveMemberCount            int64                `json:"-"`
	InitialWorkCount             int64                `json:"-"`
	CompletedTaskCount           int64                `json:"-"`
	CompletedTaskDayCount        int64                `json:"-"`
	ConnectedRepositoryCount     int64                `json:"-"`
	CompletedAgentRunCount       int64                `json:"-"`
	CompletedAgentRunDayCount    int64                `json:"-"`
	ProductAgentRunCount         int64                `json:"-"`
	MemberProductAgentRunCount   int64                `json:"-"`
	CompletedAgentTargetCount    int64                `json:"-"`
	CompletedAgentMemberCount    int64                `json:"-"`
	MemberAgentRunCount          int64                `json:"-"`
	MemberAgentRunDayCount       int64                `json:"-"`
	MemberAgentTargetCount       int64                `json:"-"`
	EnabledAutomationCount       int64                `json:"-"`
	TriggeredSuccessRunCount     int64                `json:"-"`
	TriggeredSuccessDayCount     int64                `json:"-"`
	ReliableAutomationCount      int64                `json:"-"`
	SupportChannelCount          int64                `json:"-"`
	SupportEmailInboxCount       int64                `json:"-"`
	LiveChatInstallationCount    int64                `json:"-"`
	PublicHelpDocCount           int64                `json:"-"`
	BrandKnowledgeSourceCount    int64                `json:"-"`
	SupportAIAgentActive         bool                 `json:"-"`
	TeamInboxCount               int64                `json:"-"`
	AutomaticRoutingCount        int64                `json:"-"`
	ResolvedConversationCount    int64                `json:"-"`
	ResolvedConversationDayCount int64                `json:"-"`
	KnowledgeSourceCount         int64                `json:"-"`
	AIResolvedConversationCount  int64                `json:"-"`
	MemberSupportAIReplyCount    int64                `json:"-"`
	LinkedSupportTaskCount       int64                `json:"-"`
	SprintCloseoutCount          int64                `json:"-"`
	ReleaseNotesSuccessCount     int64                `json:"-"`
	PlannedProjectCount          int64                `json:"-"`
	PlannedSprintCount           int64                `json:"-"`
	AssignedProjectTaskCount     int64                `json:"-"`
	HelpCenterSpaceCount         int64                `json:"-"`
	HelpCenterContentCount       int64                `json:"-"`
	HelpCenterSiteCount          int64                `json:"-"`
	HelpCenterWidgetCount        int64                `json:"-"`
	InternalDocsSpaceCount       int64                `json:"-"`
	InternalDocsContentCount     int64                `json:"-"`
	InternalDocsPublishedCount   int64                `json:"-"`
	InternalDocsOwnershipCount   int64                `json:"-"`
	InternalAgentKnowledgeCount  int64                `json:"-"`
	InternalDocAgentSuccessCount int64                `json:"-"`
	CRMContactCount              int64                `json:"-"`
	CRMCompanyCount              int64                `json:"-"`
	CRMPipelineCount             int64                `json:"-"`
	CRMActionableDealCount       int64                `json:"-"`
	CRMConnectedEmailCount       int64                `json:"-"`
	CRMAutonomyEnabledCount      int64                `json:"-"`
	CRMSignalValueCount          int64                `json:"-"`
	ApprovalGuardCount           int64                `json:"-"`
	ValidatedSupportCount        int64                `json:"-"`
	CoverageImprovementCount     int64                `json:"-"`
	ApprovalResolvedCount        int64                `json:"-"`
	MemberApprovalResolvedCount  int64                `json:"-"`
	CustomAgentSuccessCount      int64                `json:"-"`
	TaskAchievementTimes         map[string]time.Time `json:"-"`
}

type SetupMemberAgentEvidence struct {
	RunCount       int64
	RunDayCount    int64
	RunTargetCount int64
	ProductCount   int64
	FirstRunAt     time.Time
	RepeatRunAt    time.Time
	FirstProductAt time.Time
}

type UpdateSetupGoalsRequest struct {
	Goals []string `json:"goals"`
}

type UpdateSetupPreferenceRequest struct {
	SidebarDismissed *bool `json:"sidebar_dismissed"`
}
