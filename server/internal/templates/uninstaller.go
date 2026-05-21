package templates

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

const (
	AgentActionNone            = "none"
	AgentActionKept            = "kept"
	AgentActionDeleted         = "deleted"
	AgentActionStillReferenced = "still_referenced"
)

type Uninstaller struct {
	db              *gorm.DB
	scheduleManager RuleScheduleManager
}

type UninstallRequest struct {
	WorkspaceID        string
	TemplateInstanceID string
	ActorID            string
	DeleteCreatedAgent bool
}

type UninstallResult struct {
	TemplateKey        string `json:"template_key"`
	TemplateInstanceID string `json:"template_instance_id"`
	RuleID             string `json:"rule_id"`
	AgentID            string `json:"agent_id,omitempty"`
	AgentAction        string `json:"agent_action"`
}

func NewUninstaller(db *gorm.DB) *Uninstaller {
	return &Uninstaller{db: db}
}

func (u *Uninstaller) SetScheduleManager(manager RuleScheduleManager) *Uninstaller {
	if u != nil {
		u.scheduleManager = manager
	}
	return u
}

func (u *Uninstaller) Uninstall(ctx context.Context, req UninstallRequest) (*UninstallResult, error) {
	if u == nil || u.db == nil {
		return nil, internalErrorf(fmt.Errorf("template uninstaller is not configured"), "flow templates are unavailable")
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	instanceID := strings.TrimSpace(req.TemplateInstanceID)
	actorID := strings.TrimSpace(req.ActorID)
	if workspaceID == "" || instanceID == "" {
		return nil, validationErrorf("workspace_id and template_instance_id are required")
	}

	var result UninstallResult
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rule model.AutomationRule
		if err := tx.WithContext(ctx).
			Where("workspace_id = ? AND template_instance_id = ?", workspaceID, instanceID).
			First(&rule).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return notFoundErrorf("template instance %q not found", instanceID)
			}
			return internalErrorf(err, "could not load template instance")
		}

		result = UninstallResult{
			TemplateKey:        derefString(rule.TemplateKey),
			TemplateInstanceID: instanceID,
			RuleID:             rule.ID,
			AgentAction:        AgentActionNone,
		}

		var agent model.Agent
		hasCreatedAgent := false
		if err := tx.WithContext(ctx).
			Where("workspace_id = ? AND template_instance_id = ?", workspaceID, instanceID).
			First(&agent).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return internalErrorf(err, "could not load template agent")
			}
		} else {
			hasCreatedAgent = true
			result.AgentID = agent.ID
		}

		if rule.TriggerType == model.TriggerCron && u.scheduleManager != nil {
			if err := u.scheduleManager.StopRuleScheduleForRule(ctx, rule.ID); err != nil {
				return internalErrorf(err, "could not stop scheduled flow")
			}
		}
		if err := tx.WithContext(ctx).
			Where("workspace_id = ? AND id = ?", workspaceID, rule.ID).
			Delete(&model.AutomationRule{}).Error; err != nil {
			return internalErrorf(err, "could not delete flow")
		}
		if err := logTemplateActivity(ctx, tx, workspaceID, rule.ID, actorID, "template.uninstalled", map[string]any{
			"template_key":         result.TemplateKey,
			"template_instance_id": instanceID,
			"agent_id":             result.AgentID,
			"delete_created_agent": req.DeleteCreatedAgent,
		}); err != nil {
			return internalErrorf(err, "could not record flow activity")
		}

		if !hasCreatedAgent {
			return nil
		}
		referenced, err := otherRulesReferenceAgent(ctx, tx, workspaceID, agent.ID, rule.ID)
		if err != nil {
			return internalErrorf(err, "could not check agent references")
		}
		if referenced {
			result.AgentAction = AgentActionStillReferenced
			return nil
		}
		if req.DeleteCreatedAgent {
			var runCount int64
			if err := tx.WithContext(ctx).
				Model(&model.AgentRun{}).
				Where("workspace_id = ? AND agent_id = ?", workspaceID, agent.ID).
				Count(&runCount).Error; err != nil {
				return internalErrorf(err, "could not check agent run history")
			}
			if runCount > 0 {
				if err := detachTemplateAgent(ctx, tx, workspaceID, agent.ID); err != nil {
					return err
				}
				result.AgentAction = AgentActionKept
				return nil
			}
			if err := tx.WithContext(ctx).
				Where("workspace_id = ? AND id = ?", workspaceID, agent.ID).
				Delete(&model.Agent{}).Error; err != nil {
				return internalErrorf(err, "could not delete template agent")
			}
			result.AgentAction = AgentActionDeleted
			return nil
		}

		if err := detachTemplateAgent(ctx, tx, workspaceID, agent.ID); err != nil {
			return err
		}
		result.AgentAction = AgentActionKept
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to uninstall flow template",
			"error", err,
			"template_instance_id", instanceID,
			"workspace_id", workspaceID,
			"actor_id", actorID,
		)
		return nil, err
	}
	slog.InfoContext(ctx, "flow template uninstalled",
		"template_key", result.TemplateKey,
		"template_instance_id", result.TemplateInstanceID,
		"workspace_id", workspaceID,
		"actor_id", actorID,
		"agent_id", result.AgentID,
		"agent_action", result.AgentAction,
	)
	return &result, nil
}

func detachTemplateAgent(ctx context.Context, tx *gorm.DB, workspaceID, agentID string) error {
	if err := tx.WithContext(ctx).
		Model(&model.Agent{}).
		Where("workspace_id = ? AND id = ?", workspaceID, agentID).
		Updates(map[string]any{
			"template_key":         nil,
			"template_instance_id": nil,
			"template_version":     nil,
			"is_system":            false,
		}).Error; err != nil {
		return internalErrorf(err, "could not detach template agent")
	}
	return nil
}

func otherRulesReferenceAgent(ctx context.Context, tx *gorm.DB, workspaceID, agentID, deletedRuleID string) (bool, error) {
	var count int64
	query := tx.WithContext(ctx).
		Model(&model.AutomationRule{}).
		Where("workspace_id = ? AND id <> ?", workspaceID, deletedRuleID)

	switch tx.Dialector.Name() {
	case "sqlite":
		query = query.Where("json_extract(action_config, '$.agent_id') = ?", agentID)
	default:
		query = query.Where("action_config ->> 'agent_id' = ?", agentID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check agent rule references: %w", err)
	}
	return count > 0, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
