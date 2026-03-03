package model

import (
	"encoding/json"
	"time"
)

// BonusCalculation represents a row in the quarterly_bonus_calculations table.
type BonusCalculation struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_bonus_calc_ws_qtr_emp"`
	QuarterID          string          `json:"quarter_id" gorm:"type:uuid;not null;uniqueIndex:idx_bonus_calc_ws_qtr_emp"`
	EmployeeID         string          `json:"employee_id" gorm:"type:uuid;not null;uniqueIndex:idx_bonus_calc_ws_qtr_emp"`
	FinalAmount        float64         `json:"final_amount" gorm:"not null;default:0"`
	BonusTier          *string         `json:"bonus_tier"`
	TeamTQI            *int            `json:"team_tqi"`
	IndividualIQI      *int            `json:"individual_iqi"`
	FinalScore         *int            `json:"final_score"`
	BaseSalary         float64         `json:"base_salary" gorm:"not null;default:0"`
	IsOverride         bool            `json:"is_override" gorm:"not null;default:false"`
	OverrideReason     *string         `json:"override_reason"`
	OverrideAppliedBy  *string         `json:"override_applied_by" gorm:"type:uuid"`
	OverrideAppliedAt  *time.Time      `json:"override_applied_at"`
	CalculationDetails json.RawMessage `json:"calculation_details" gorm:"type:jsonb"`
	LockedAt           *time.Time      `json:"locked_at"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (BonusCalculation) TableName() string { return "quarterly_bonus_calculations" }

// FinanceSettings represents a row in the quarterly_finance_settings table.
type FinanceSettings struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_finance_ws_qtr"`
	QuarterID           string          `json:"quarter_id" gorm:"type:uuid;not null;uniqueIndex:idx_finance_ws_qtr"`
	MRRStart            float64         `json:"mrr_start" gorm:"not null;default:0"`
	MRREnd              float64         `json:"mrr_end" gorm:"not null;default:0"`
	BonusPoolPercentage int             `json:"bonus_pool_percentage" gorm:"not null;default:0"`
	MaxBonusPool        *float64        `json:"max_bonus_pool"`
	TeamWeight          int             `json:"team_weight" gorm:"not null;default:50"`
	BonusTiers          json.RawMessage `json:"bonus_tiers" gorm:"type:jsonb"`
	TotalPool           float64         `json:"total_pool" gorm:"not null;default:0"`
	TotalPaid           float64         `json:"total_paid" gorm:"not null;default:0"`
	PoolUtilization     float64         `json:"pool_utilization" gorm:"not null;default:0"`
	BudgetFactor        float64         `json:"budget_factor" gorm:"not null;default:0"`
	TotalBasicSalary    float64         `json:"total_basic_salary" gorm:"not null;default:0"`
	LockedAt            *time.Time      `json:"locked_at"`
	LockedBy            *string         `json:"locked_by" gorm:"type:uuid"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (FinanceSettings) TableName() string { return "quarterly_finance_settings" }

// BonusAuditLog represents a row in the bonus_audit_log table.
type BonusAuditLog struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	QuarterID       string          `json:"quarter_id" gorm:"type:uuid;not null;index"`
	Action          string          `json:"action" gorm:"not null"`
	EmployeeID      *string         `json:"employee_id" gorm:"type:uuid"`
	PerformedBy     string          `json:"performed_by" gorm:"type:uuid;not null"`
	PerformedByName *string         `json:"performed_by_name"`
	PerformedByRole *string         `json:"performed_by_role"`
	OldValue        json.RawMessage `json:"old_value" gorm:"type:jsonb"`
	NewValue        json.RawMessage `json:"new_value" gorm:"type:jsonb"`
	Justification   *string         `json:"justification"`
	AffectedCount   int             `json:"affected_count" gorm:"not null;default:0"`
	PerformedAt     time.Time       `json:"performed_at" gorm:"autoCreateTime"`
}

func (BonusAuditLog) TableName() string { return "bonus_audit_log" }

// SaveCalculationsRequest is the payload for saving bonus calculations.
type SaveCalculationsRequest struct {
	WorkspaceID  string             `json:"workspace_id"`
	QuarterID    string             `json:"quarter_id"`
	Calculations []BonusCalculation `json:"calculations"`
}

// UpsertFinanceRequest is the payload for upserting finance settings.
type UpsertFinanceRequest struct {
	WorkspaceID         string          `json:"workspace_id"`
	QuarterID           string          `json:"quarter_id"`
	MRRStart            float64         `json:"mrr_start"`
	MRREnd              float64         `json:"mrr_end"`
	BonusPoolPercentage int             `json:"bonus_pool_percentage"`
	MaxBonusPool        *float64        `json:"max_bonus_pool"`
	TeamWeight          int             `json:"team_weight"`
	BonusTiers          json.RawMessage `json:"bonus_tiers"`
	TotalPool           float64         `json:"total_pool"`
	TotalPaid           float64         `json:"total_paid"`
	PoolUtilization     float64         `json:"pool_utilization"`
	BudgetFactor        float64         `json:"budget_factor"`
	TotalBasicSalary    float64         `json:"total_basic_salary"`
}

// LockBonusRequest is the payload for locking bonus calculations.
type LockBonusRequest struct {
	WorkspaceID string `json:"workspace_id"`
	QuarterID   string `json:"quarter_id"`
}

// CreateAuditEntryRequest is the payload for creating a bonus audit log entry.
type CreateAuditEntryRequest struct {
	WorkspaceID     string          `json:"workspace_id"`
	QuarterID       string          `json:"quarter_id"`
	Action          string          `json:"action"`
	EmployeeID      *string         `json:"employee_id"`
	PerformedByName *string         `json:"performed_by_name"`
	PerformedByRole *string         `json:"performed_by_role"`
	OldValue        json.RawMessage `json:"old_value"`
	NewValue        json.RawMessage `json:"new_value"`
	Justification   *string         `json:"justification"`
	AffectedCount   int             `json:"affected_count"`
}
