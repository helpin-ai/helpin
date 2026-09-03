package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIActionExecutionRepository stores governed AI attempt audits.
type AIActionExecutionRepository struct{ db *gorm.DB }

// NewAIActionExecutionRepository creates an AI execution audit repository.
func NewAIActionExecutionRepository(db *gorm.DB) *AIActionExecutionRepository {
	return &AIActionExecutionRepository{db: db}
}

// Start idempotently records one immutable governed AI attempt identity.
func (r *AIActionExecutionRepository) Start(ctx context.Context, execution *model.AIActionExecution) (*model.AIActionExecution, error) {
	if r == nil || r.db == nil || execution == nil {
		return nil, fmt.Errorf("AI action execution repository and input are required")
	}
	if strings.TrimSpace(execution.ActionKey) == "" || strings.TrimSpace(execution.PolicyVersion) == "" ||
		strings.TrimSpace(execution.IdempotencyKey) == "" || execution.Attempt <= 0 {
		return nil, fmt.Errorf("action_key, policy_version, idempotency_key, and positive attempt are required")
	}
	if execution.ID == "" {
		execution.ID = uuid.NewString()
	}
	if execution.Status == "" {
		execution.Status = model.AIActionExecutionRunning
	}
	if execution.StartedAt.IsZero() {
		execution.StartedAt = time.Now().UTC()
	}
	if execution.Metadata == nil {
		execution.Metadata = []byte("{}")
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}, {Name: "attempt"}},
		DoNothing: true,
	}).Create(execution)
	if result.Error != nil {
		return nil, fmt.Errorf("start AI action execution: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return execution, nil
	}
	var existing model.AIActionExecution
	if err := r.db.WithContext(ctx).
		Where("idempotency_key = ? AND attempt = ?", execution.IdempotencyKey, execution.Attempt).
		First(&existing).Error; err != nil {
		return nil, fmt.Errorf("load existing AI action execution: %w", err)
	}
	return &existing, nil
}

// Finish idempotently completes an AI attempt without changing request identity.
func (r *AIActionExecutionRepository) Finish(ctx context.Context, id string, result aipolicy.ExecutionResult) error {
	if r == nil || r.db == nil || strings.TrimSpace(id) == "" {
		return fmt.Errorf("AI action execution repository and id are required")
	}
	if result.Status != model.AIActionExecutionSucceeded && result.Status != model.AIActionExecutionFailed {
		return fmt.Errorf("terminal AI action execution status is required")
	}
	if result.CompletedAt.IsZero() {
		result.CompletedAt = time.Now().UTC()
	}
	updates := map[string]interface{}{
		"status": result.Status, "failure_class": result.FailureClass,
		"failure_message": result.FailureMessage, "input_tokens": result.InputTokens,
		"output_tokens": result.OutputTokens, "reasoning_tokens": result.ReasoningTokens,
		"cached_input_tokens": result.CachedInputTokens, "completed_at": result.CompletedAt,
		"updated_at": time.Now().UTC(),
	}
	dbResult := r.db.WithContext(ctx).Model(&model.AIActionExecution{}).
		Where("id = ? AND status = ?", id, model.AIActionExecutionRunning).Updates(updates)
	if dbResult.Error != nil {
		return fmt.Errorf("finish AI action execution: %w", dbResult.Error)
	}
	if dbResult.RowsAffected > 0 {
		return nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.AIActionExecution{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("check AI action execution: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("AI action execution %q not found", id)
	}
	return nil
}
