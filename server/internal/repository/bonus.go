package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// BonusRepository handles database operations for bonus calculations, finance, and audit.
type BonusRepository struct {
	db *gorm.DB
}

// NewBonusRepository creates a new BonusRepository.
func NewBonusRepository(db *gorm.DB) *BonusRepository {
	return &BonusRepository{db: db}
}

// SaveCalculations upserts a batch of bonus calculations.
func (r *BonusRepository) SaveCalculations(ctx context.Context, calcs []model.BonusCalculation) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range calcs {
			calc := model.BonusCalculation{
				WorkspaceID:        c.WorkspaceID,
				QuarterID:          c.QuarterID,
				EmployeeID:         c.EmployeeID,
				FinalAmount:        c.FinalAmount,
				BonusTier:          c.BonusTier,
				TeamTQI:            c.TeamTQI,
				IndividualIQI:      c.IndividualIQI,
				FinalScore:         c.FinalScore,
				BaseSalary:         c.BaseSalary,
				IsOverride:         c.IsOverride,
				OverrideReason:     c.OverrideReason,
				OverrideAppliedBy:  c.OverrideAppliedBy,
				OverrideAppliedAt:  c.OverrideAppliedAt,
				CalculationDetails: c.CalculationDetails,
			}
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "workspace_id"}, {Name: "quarter_id"}, {Name: "employee_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"final_amount", "bonus_tier", "team_tqi", "individual_iqi",
					"final_score", "base_salary", "is_override", "override_reason",
					"override_applied_by", "override_applied_at", "calculation_details",
				}),
			}).Create(&calc).Error
			if err != nil {
				return fmt.Errorf("upsert bonus calculation: %w", err)
			}
		}
		return nil
	})
}

// GetCalculations returns all bonus calculations for a workspace/quarter.
func (r *BonusRepository) GetCalculations(ctx context.Context, workspaceID, quarterID string) ([]model.BonusCalculation, error) {
	var calcs []model.BonusCalculation
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		Order("employee_id").
		Find(&calcs).Error
	if err != nil {
		return nil, fmt.Errorf("get calculations: %w", err)
	}
	return calcs, nil
}

// Lock sets the locked_at timestamp on all calculations for a workspace/quarter.
func (r *BonusRepository) Lock(ctx context.Context, workspaceID, quarterID string) error {
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&model.BonusCalculation{}).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		Update("locked_at", now).Error
	if err != nil {
		return fmt.Errorf("lock calculations: %w", err)
	}
	return nil
}

// Unlock clears the locked_at timestamp on all calculations for a workspace/quarter.
func (r *BonusRepository) Unlock(ctx context.Context, workspaceID, quarterID string) error {
	err := r.db.WithContext(ctx).
		Model(&model.BonusCalculation{}).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		Update("locked_at", nil).Error
	if err != nil {
		return fmt.Errorf("unlock calculations: %w", err)
	}
	return nil
}

// UpsertFinance inserts or updates quarterly finance settings.
func (r *BonusRepository) UpsertFinance(ctx context.Context, req model.UpsertFinanceRequest) (*model.FinanceSettings, error) {
	fs := &model.FinanceSettings{
		WorkspaceID:         req.WorkspaceID,
		QuarterID:           req.QuarterID,
		MRRStart:            req.MRRStart,
		MRREnd:              req.MRREnd,
		BonusPoolPercentage: req.BonusPoolPercentage,
		MaxBonusPool:        req.MaxBonusPool,
		TeamWeight:          req.TeamWeight,
		BonusTiers:          req.BonusTiers,
		TotalPool:           req.TotalPool,
		TotalPaid:           req.TotalPaid,
		PoolUtilization:     req.PoolUtilization,
		BudgetFactor:        req.BudgetFactor,
		TotalBasicSalary:    req.TotalBasicSalary,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}, {Name: "quarter_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"mrr_start", "mrr_end", "bonus_pool_percentage", "max_bonus_pool",
				"team_weight", "bonus_tiers", "total_pool", "total_paid",
				"pool_utilization", "budget_factor", "total_basic_salary",
			}),
		}).
		Create(fs).Error
	if err != nil {
		return nil, fmt.Errorf("upsert finance: %w", err)
	}
	// Re-fetch to get correct ID and timestamps after upsert.
	result := &model.FinanceSettings{}
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND quarter_id = ?", req.WorkspaceID, req.QuarterID).
		First(result).Error; err != nil {
		return nil, fmt.Errorf("upsert finance: %w", err)
	}
	return result, nil
}

// GetFinance returns the quarterly finance settings for a workspace/quarter.
func (r *BonusRepository) GetFinance(ctx context.Context, workspaceID, quarterID string) (*model.FinanceSettings, error) {
	fs := &model.FinanceSettings{}
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		First(fs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get finance: %w", err)
	}
	return fs, nil
}

// CreateAuditEntry inserts a bonus audit log entry.
func (r *BonusRepository) CreateAuditEntry(ctx context.Context, entry model.BonusAuditLog) (*model.BonusAuditLog, error) {
	a := &model.BonusAuditLog{
		WorkspaceID:     entry.WorkspaceID,
		QuarterID:       entry.QuarterID,
		Action:          entry.Action,
		EmployeeID:      entry.EmployeeID,
		PerformedBy:     entry.PerformedBy,
		PerformedByName: entry.PerformedByName,
		PerformedByRole: entry.PerformedByRole,
		OldValue:        entry.OldValue,
		NewValue:        entry.NewValue,
		Justification:   entry.Justification,
		AffectedCount:   entry.AffectedCount,
	}
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("create audit entry: %w", err)
	}
	return a, nil
}

// ListAuditEntries returns audit log entries for a workspace/quarter.
func (r *BonusRepository) ListAuditEntries(ctx context.Context, workspaceID, quarterID string) ([]model.BonusAuditLog, error) {
	var entries []model.BonusAuditLog
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		Order("performed_at DESC").
		Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	return entries, nil
}
