package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// CreateCondition records a Flow filter before an action or agent run exists.
func (r *AgentTriggerExecutionRepository) CreateCondition(ctx context.Context, execution *model.AgentTriggerExecution) error {
	if execution.ConditionOutcome == nil || execution.WorkspaceID == "" || execution.ReferenceID == nil {
		return fmt.Errorf("invalid Flow condition activity")
	}
	execution.ID = uuid.NewString()
	return r.db.WithContext(ctx).Table((model.AgentTriggerExecution{}).TableName()).Create(map[string]any{
		"id": execution.ID, "workspace_id": execution.WorkspaceID, "agent_id": nil,
		"binding_id": execution.BindingID, "binding_kind": execution.BindingKind,
		"trigger_type": execution.TriggerType, "reference_id": execution.ReferenceID, "reference_type": execution.ReferenceType,
		"target_type": execution.TargetType, "target_id": execution.TargetID, "status": execution.Status,
		"condition_outcome": execution.ConditionOutcome, "condition_assessment_id": execution.ConditionAssessmentID,
		"fired_at": execution.FiredAt, "completed_at": execution.CompletedAt, "created_at": execution.FiredAt, "updated_at": execution.FiredAt,
	}).Error
}
