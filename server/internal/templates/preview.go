package templates

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Preview shares installation's input, agent, and rule construction but never
// persists rows, emits activity, or starts schedules.
func (i *Installer) Preview(ctx context.Context, req InstallRequest) (*InstallResult, error) {
	tmpl, ok := i.registry.Get(req.TemplateKey)
	if !ok {
		return nil, notFoundErrorf("template not found")
	}
	if err := validateInstallInputs(tmpl, req.Inputs); err != nil {
		return nil, validationErrorf("%s", err.Error())
	}
	agent, err := i.resolveAgent(ctx, i.db, tmpl, req.WorkspaceID, "preview", tmpl.Version, req.AgentName, req.Inputs, req.AgentOverrides, true)
	if err != nil {
		return nil, err
	}
	rule, err := buildRule(ctx, i.db, tmpl, req.WorkspaceID, req.ActorID, req.Name, req.Description, "preview", tmpl.Version, req.Inputs, agent)
	if err != nil {
		return nil, err
	}
	if err := applyInstallOptions(rule, req); err != nil {
		return nil, err
	}
	return &InstallResult{Template: tmpl, Agent: agent, Rule: rule}, nil
}

func applyInstallOptions(rule *model.AutomationRule, req InstallRequest) error {
	rule.Enabled = !req.Paused
	if strings.TrimSpace(req.SemanticCondition) == "" {
		return nil
	}
	if rule.TriggerType == model.TriggerCron {
		return validationErrorf("conditions require an event trigger")
	}
	var config map[string]any
	if err := json.Unmarshal(rule.TriggerConfig, &config); err != nil {
		return err
	}
	config["semantic_condition"] = map[string]string{"text": strings.TrimSpace(req.SemanticCondition)}
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	rule.TriggerConfig = encoded
	return nil
}
