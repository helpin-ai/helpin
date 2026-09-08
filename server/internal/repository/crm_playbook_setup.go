package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// PrepareSetup creates a disabled connected Flow and reuses Beacon without reconciling its settings.
// The candidate factory is pure and used only when this workspace has no built-in Beacon.
func (r *CRMPlaybookExecutionRepository) PrepareSetup(ctx context.Context, ws, id, actorUser, actorMember string, req model.PrepareCRMPlaybookSetupRequest, newBeacon func() (*model.Agent, error)) (*model.CRMPlaybookSetup, error) {
	var result *model.CRMPlaybookSetup
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		if err := requirePlaybookExecutionMember(tx, ws, actorMember); err != nil {
			return err
		}
		var book model.CRMPlaybook
		if err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&book).Error; err != nil {
			return err
		}
		if book.Revision != req.ExpectedRevision || book.PublishedVersionID == nil || *book.PublishedVersionID != req.PlaybookVersionID {
			return ErrCRMPlaybookStale
		}
		agent, err := NewAgentRepository(tx).GetSystemByPreset(ctx, ws, model.AgentPresetCRMOperator)
		if err != nil {
			return err
		}
		if agent == nil {
			agent, err = newBeacon()
			if err != nil {
				return err
			}
			if agent == nil || agent.WorkspaceID != ws || !agent.IsSystem || agent.EffectivePresetKey() != model.AgentPresetCRMOperator {
				return ErrCRMPlaybookUnavailable
			}
			if err := NewAgentRepository(tx).Create(ctx, agent); err != nil {
				return err
			}
		}
		flowID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("helpin.crm.playbook.flow:"+ws+":"+id)).String()
		var flow model.AutomationRule
		err = tx.Where("workspace_id = ? AND id = ?", ws, flowID).Take(&flow).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			trigger, err := json.Marshal(map[string]string{"playbook_id": id})
			if err != nil {
				return err
			}
			action, err := json.Marshal(model.ActionConfigRunAgent{AgentID: agent.ID, TargetType: "crm_record"})
			if err != nil {
				return err
			}
			flow = model.AutomationRule{ID: flowID, WorkspaceID: ws, Name: book.Draft.Name, TriggerType: model.CRMPlaybookWorkDue, TriggerConfig: trigger,
				ActionType: model.ActionStartAgentRun, ActionConfig: action, CreatedBy: &actorUser, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			if err := NewAutomationRuleRepository(tx).CreateDisabled(ctx, &flow); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if flow.Enabled || flow.TriggerType != model.CRMPlaybookWorkDue {
			return ErrCRMPlaybookUnavailable
		}
		result = &model.CRMPlaybookSetup{FlowID: flow.ID, AgentID: agent.ID}
		return nil
	})
	return result, err
}
