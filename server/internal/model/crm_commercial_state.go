package model

import "time"

// CRMCompanyCommercialState stores the newest accepted server-authenticated state patch.
type CRMCompanyCommercialState struct {
	CompanyID      string    `json:"company_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Version        int       `json:"version" gorm:"not null;default:1"`
	State          JSONB     `json:"state" gorm:"type:jsonb;not null;default:'{}'"`
	StateUpdatedAt time.Time `json:"state_updated_at" gorm:"not null;index"`
	SourceEventID  string    `json:"source_event_id" gorm:"not null"`
	AcceptedAt     time.Time `json:"accepted_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMCompanyCommercialState) TableName() string { return "crm_company_commercial_states" }

// CRMCompanyCommercialStateHistory is the append-only accepted state history.
type CRMCompanyCommercialStateHistory struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_crm_commercial_state_history_workspace_event,priority:1"`
	CompanyID        string    `json:"company_id" gorm:"type:uuid;not null;index"`
	Version          int       `json:"version" gorm:"not null"`
	Patch            JSONB     `json:"patch" gorm:"type:jsonb;not null;default:'{}'"`
	StateSnapshot    JSONB     `json:"state_snapshot" gorm:"type:jsonb;not null;default:'{}'"`
	StateUpdatedAt   time.Time `json:"state_updated_at" gorm:"not null;index"`
	SourceEventID    string    `json:"source_event_id" gorm:"not null;uniqueIndex:idx_crm_commercial_state_history_workspace_event,priority:2"`
	AppliedToCurrent bool      `json:"applied_to_current" gorm:"not null;default:true"`
	AcceptedAt       time.Time `json:"accepted_at" gorm:"not null"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMCompanyCommercialStateHistory) TableName() string {
	return "crm_company_commercial_state_history"
}

// CRMCompanyCommercialStateHealth surfaces rejected and stale integration updates.
type CRMCompanyCommercialStateHealth struct {
	CompanyID           string     `json:"company_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID         string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	LastAcceptedAt      *time.Time `json:"last_accepted_at,omitempty"`
	LastRejectedAt      *time.Time `json:"last_rejected_at,omitempty"`
	LastRejectionReason *string    `json:"last_rejection_reason,omitempty"`
	RejectedUpdateCount int        `json:"rejected_update_count" gorm:"not null;default:0"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMCompanyCommercialStateHealth) TableName() string {
	return "crm_company_commercial_state_health"
}

// CRMSignalConditionState tracks level-rule crossings and re-arm state.
type CRMSignalConditionState struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CompanyID   string     `json:"company_id" gorm:"type:uuid;not null;index"`
	RuleKey     string     `json:"rule_key" gorm:"not null;index"`
	RuleVersion int        `json:"rule_version" gorm:"not null"`
	Motion      string     `json:"motion" gorm:"not null"`
	ScopeKey    string     `json:"scope_key" gorm:"not null;default:''"`
	Armed       bool       `json:"armed" gorm:"not null;default:true"`
	TriggeredAt *time.Time `json:"triggered_at,omitempty"`
	RearmedAt   *time.Time `json:"rearmed_at,omitempty"`
	LastValue   *float64   `json:"last_value,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSignalConditionState) TableName() string { return "crm_signal_condition_states" }

// CRMUsageWeekdayBaseline stores one company metric's weekday expectation.
type CRMUsageWeekdayBaseline struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CompanyID        string    `json:"company_id" gorm:"type:uuid;not null;index"`
	MetricKey        string    `json:"metric_key" gorm:"not null;index"`
	Weekday          int       `json:"weekday" gorm:"not null"`
	MedianValue      float64   `json:"median_value" gorm:"not null"`
	ObservationCount int       `json:"observation_count" gorm:"not null"`
	CompleteWeeks    int       `json:"complete_weeks" gorm:"not null"`
	IdentityMethod   string    `json:"identity_method" gorm:"not null;default:'unknown'"`
	WindowStartedAt  time.Time `json:"window_started_at" gorm:"not null"`
	WindowEndedAt    time.Time `json:"window_ended_at" gorm:"not null"`
	CalculatedAt     time.Time `json:"calculated_at" gorm:"not null"`
}

func (CRMUsageWeekdayBaseline) TableName() string { return "crm_usage_weekday_baselines" }

// CRMSignalBatchSuppression records workspace-wide anomaly/holiday guards.
type CRMSignalBatchSuppression struct {
	ID                        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID               string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RuleKey                   string    `json:"rule_key" gorm:"not null;index"`
	RuleVersion               int       `json:"rule_version" gorm:"not null"`
	EligibleAccounts          int       `json:"eligible_accounts" gorm:"not null"`
	TrippedAccounts           int       `json:"tripped_accounts" gorm:"not null"`
	TrippedRatio              float64   `json:"tripped_ratio" gorm:"not null"`
	Reason                    string    `json:"reason" gorm:"not null"`
	EvaluationWindowStartedAt time.Time `json:"evaluation_window_started_at" gorm:"not null"`
	EvaluationWindowEndedAt   time.Time `json:"evaluation_window_ended_at" gorm:"not null"`
	CreatedAt                 time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMSignalBatchSuppression) TableName() string { return "crm_signal_batch_suppressions" }
