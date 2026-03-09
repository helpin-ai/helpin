package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMSignalRepository handles DB operations for CRM signals and health scores.
type CRMSignalRepository struct {
	db *gorm.DB
}

// NewCRMSignalRepository creates a new CRMSignalRepository.
func NewCRMSignalRepository(db *gorm.DB) *CRMSignalRepository {
	return &CRMSignalRepository{db: db}
}

// ── Buyer Signals ──

// CreateSignal inserts a buyer signal.
func (r *CRMSignalRepository) CreateSignal(ctx context.Context, signal *model.CRMBuyerSignal) error {
	if err := r.db.WithContext(ctx).Create(signal).Error; err != nil {
		return fmt.Errorf("create buyer signal: %w", err)
	}
	return nil
}

// ListSignals returns buyer signals with optional filters.
func (r *CRMSignalRepository) ListSignals(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination) ([]model.CRMBuyerSignal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).Where("workspace_id = ?", workspaceID)

	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_id = ?", *filters.ContactID)
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}
	if filters.SignalType != nil && *filters.SignalType != "" {
		query = query.Where("signal_type = ?", *filters.SignalType)
	}
	if filters.SourceType != nil && *filters.SourceType != "" {
		query = query.Where("source_type = ?", *filters.SourceType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count buyer signals: %w", err)
	}

	var signals []model.CRMBuyerSignal
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("detected_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&signals).Error; err != nil {
		return nil, 0, fmt.Errorf("list buyer signals: %w", err)
	}
	return signals, total, nil
}

// DeleteSignal removes a buyer signal.
func (r *CRMSignalRepository) DeleteSignal(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMBuyerSignal{}).Error; err != nil {
		return fmt.Errorf("delete buyer signal: %w", err)
	}
	return nil
}

// ── Deal Health Scores ──

// CreateHealthScore inserts a deal health score.
func (r *CRMSignalRepository) CreateHealthScore(ctx context.Context, score *model.CRMDealHealthScore) error {
	if err := r.db.WithContext(ctx).Create(score).Error; err != nil {
		return fmt.Errorf("create deal health score: %w", err)
	}
	return nil
}

// GetLatestHealthScore returns the most recent health score for a deal.
func (r *CRMSignalRepository) GetLatestHealthScore(ctx context.Context, dealID string) (*model.CRMDealHealthScore, error) {
	var score model.CRMDealHealthScore
	if err := r.db.WithContext(ctx).Where("deal_id = ?", dealID).Order("calculated_at DESC").First(&score).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get deal health score: %w", err)
	}
	return &score, nil
}

// ListHealthScores returns health scores for a workspace.
func (r *CRMSignalRepository) ListHealthScores(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.CRMDealHealthScore, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMDealHealthScore{}).Where("workspace_id = ?", workspaceID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count health scores: %w", err)
	}

	var scores []model.CRMDealHealthScore
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("calculated_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&scores).Error; err != nil {
		return nil, 0, fmt.Errorf("list health scores: %w", err)
	}
	return scores, total, nil
}
