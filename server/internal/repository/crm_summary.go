package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMSummaryRepository handles DB operations for CRM summary artifacts.
type CRMSummaryRepository struct {
	db *gorm.DB
}

// NewCRMSummaryRepository creates a new CRMSummaryRepository.
func NewCRMSummaryRepository(db *gorm.DB) *CRMSummaryRepository {
	return &CRMSummaryRepository{db: db}
}

// GetByEntity returns the current summary artifact for one CRM entity.
func (r *CRMSummaryRepository) GetByEntity(ctx context.Context, workspaceID, entityType, entityID string) (*model.CRMEntitySummary, error) {
	var summary model.CRMEntitySummary
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", workspaceID, entityType, entityID).
		First(&summary).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get crm summary: %w", err)
	}
	return &summary, nil
}

// UpsertRefreshRequest records that an entity needs a fresh summary and is safe
// to call on every signal/email event.
func (r *CRMSummaryRepository) UpsertRefreshRequest(ctx context.Context, input model.CRMEntitySummaryRefreshInput, triggeredAt time.Time) error {
	summary := &model.CRMEntitySummary{
		WorkspaceID:     input.WorkspaceID,
		EntityType:      input.EntityType,
		EntityID:        input.EntityID,
		Status:          model.CRMEntitySummaryStatusPendingRefresh,
		LastTriggeredAt: &triggeredAt,
		Highlights:      model.CRMSummaryHighlights{},
		Metadata:        model.JSONB{},
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "workspace_id"},
				{Name: "entity_type"},
				{Name: "entity_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"status":            model.CRMEntitySummaryStatusPendingRefresh,
				"last_triggered_at": triggeredAt,
				"last_error":        nil,
				"updated_at":        triggeredAt,
			}),
		}).
		Create(summary).Error; err != nil {
		return fmt.Errorf("upsert crm summary refresh request: %w", err)
	}
	return nil
}

// UpdateGenerated stores a freshly computed summary and reports whether a newer
// trigger landed while the generation run was executing.
func (r *CRMSummaryRepository) UpdateGenerated(
	ctx context.Context,
	input model.CRMEntitySummaryRefreshInput,
	requestedAt time.Time,
	computedAt time.Time,
	summaryMarkdown string,
	highlights model.CRMSummaryHighlights,
	sourceWindowStart *time.Time,
	sourceWindowEnd *time.Time,
	metadata model.JSONB,
) (string, bool, error) {
	var status string
	var needsContinue bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.CRMEntitySummary
		if err := tx.
			Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", input.WorkspaceID, input.EntityType, input.EntityID).
			First(&current).Error; err != nil {
			return fmt.Errorf("load crm summary before update: %w", err)
		}

		status = model.CRMEntitySummaryStatusReady
		if current.LastTriggeredAt != nil && current.LastTriggeredAt.After(requestedAt) {
			status = model.CRMEntitySummaryStatusPendingRefresh
			needsContinue = true
		}

		updates := map[string]interface{}{
			"summary_markdown":    strings.TrimSpace(summaryMarkdown),
			"highlights":          highlights,
			"status":              status,
			"computed_at":         computedAt,
			"source_window_start": sourceWindowStart,
			"source_window_end":   sourceWindowEnd,
			"last_error":          nil,
			"metadata":            metadata,
			"updated_at":          computedAt,
		}
		if err := tx.Model(&model.CRMEntitySummary{}).
			Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", input.WorkspaceID, input.EntityType, input.EntityID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update generated crm summary: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return status, needsContinue, nil
}

// UpdateFailure records a failed generation attempt while preserving any
// previously usable summary content.
func (r *CRMSummaryRepository) UpdateFailure(ctx context.Context, input model.CRMEntitySummaryRefreshInput, message string) (string, error) {
	var status string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.CRMEntitySummary
		if err := tx.
			Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", input.WorkspaceID, input.EntityType, input.EntityID).
			First(&current).Error; err != nil {
			return fmt.Errorf("load crm summary before failure update: %w", err)
		}

		if strings.TrimSpace(current.SummaryMarkdown) == "" {
			status = model.CRMEntitySummaryStatusError
		} else {
			status = model.CRMEntitySummaryStatusStale
		}

		if err := tx.Model(&model.CRMEntitySummary{}).
			Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", input.WorkspaceID, input.EntityType, input.EntityID).
			Updates(map[string]interface{}{
				"status":     status,
				"last_error": strings.TrimSpace(message),
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return fmt.Errorf("update failed crm summary: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

// ListOpenDealRefreshInputs returns all currently open deals across workspaces.
func (r *CRMSummaryRepository) ListOpenDealRefreshInputs(ctx context.Context) ([]model.CRMEntitySummaryRefreshInput, error) {
	type row struct {
		WorkspaceID string
		DealID      string
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("crm_deals").
		Select("crm_deals.workspace_id, crm_deals.id AS deal_id").
		Joins("JOIN crm_pipeline_stages ON crm_pipeline_stages.id = crm_deals.stage_id").
		Where("crm_pipeline_stages.stage_type = ?", model.CRMStageTypeOpen).
		Order("crm_deals.updated_at DESC, crm_deals.id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list open deal summaries: %w", err)
	}

	results := make([]model.CRMEntitySummaryRefreshInput, 0, len(rows))
	for _, row := range rows {
		results = append(results, model.CRMEntitySummaryRefreshInput{
			WorkspaceID: row.WorkspaceID,
			EntityType:  model.CRMObjectDeal,
			EntityID:    row.DealID,
		})
	}
	return results, nil
}

// ListRecentlyTouchedContactRefreshInputs returns contacts that had email or
// signal activity since the provided timestamp.
func (r *CRMSummaryRepository) ListRecentlyTouchedContactRefreshInputs(ctx context.Context, since time.Time) ([]model.CRMEntitySummaryRefreshInput, error) {
	type row struct {
		WorkspaceID string
		ContactID   string
	}

	var rows []row
	if err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT workspace_id, contact_id
		FROM (
			SELECT crm_email_message_contacts.workspace_id, crm_email_message_contacts.contact_id
			FROM crm_email_message_contacts
			JOIN crm_email_messages ON crm_email_messages.id = crm_email_message_contacts.message_id
			WHERE crm_email_messages.sent_at >= ?
			UNION
			SELECT workspace_id, contact_id
			FROM crm_buyer_signals
			WHERE contact_id IS NOT NULL AND detected_at >= ?
		) recent_contacts
		ORDER BY workspace_id ASC, contact_id ASC
	`, since, since).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list recently touched contact summaries: %w", err)
	}

	results := make([]model.CRMEntitySummaryRefreshInput, 0, len(rows))
	for _, row := range rows {
		if row.WorkspaceID == "" || row.ContactID == "" {
			continue
		}
		results = append(results, model.CRMEntitySummaryRefreshInput{
			WorkspaceID: row.WorkspaceID,
			EntityType:  model.CRMObjectContact,
			EntityID:    row.ContactID,
		})
	}
	return results, nil
}
