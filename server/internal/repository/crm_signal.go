package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// CreateSignalIfAbsent inserts a buyer signal once for a given source and signal type.
func (r *CRMSignalRepository) CreateSignalIfAbsent(ctx context.Context, signal *model.CRMBuyerSignal) (bool, error) {
	if signal == nil {
		return false, nil
	}
	var existing model.CRMBuyerSignal
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", signal.WorkspaceID).
		Where("source_type = ?", signal.SourceType).
		Where("source_id = ?", signal.SourceID).
		Where("signal_type = ?", signal.SignalType).
		First(&existing).Error
	if err == nil {
		return false, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("lookup buyer signal by source: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(signal).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, fmt.Errorf("create buyer signal if absent: %w", err)
	}
	return true, nil
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
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}
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

// HasRecentSignalForThread returns true when a same-type signal already exists
// for the same thread inside the provided window.
func (r *CRMSignalRepository) HasRecentSignalForThread(ctx context.Context, workspaceID, threadID, signalType string, since time.Time) (bool, error) {
	if workspaceID == "" || threadID == "" || signalType == "" {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.CRMBuyerSignal{}).
		Where("workspace_id = ?", workspaceID).
		Where("source_thread_id = ?", threadID).
		Where("signal_type = ?", signalType).
		Where("detected_at >= ?", since).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count recent thread signals: %w", err)
	}
	return count > 0, nil
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint")
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
