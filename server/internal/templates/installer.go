package templates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type Installer struct {
	db       *gorm.DB
	registry *Registry
}

type InstallRequest struct {
	WorkspaceID string
	TemplateKey string
	ActorID     string
	Name        string
	Inputs      map[string]any
}

type InstallResult struct {
	Template Template              `json:"template"`
	Agent    *model.Agent          `json:"agent,omitempty"`
	Rule     *model.AutomationRule `json:"rule"`
}

func NewInstaller(db *gorm.DB, registry *Registry) *Installer {
	return &Installer{db: db, registry: registry}
}

func (i *Installer) Install(ctx context.Context, req InstallRequest) (*InstallResult, error) {
	if i == nil || i.db == nil || i.registry == nil {
		return nil, fmt.Errorf("template installer is not configured")
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	templateKey := strings.TrimSpace(req.TemplateKey)
	actorID := strings.TrimSpace(req.ActorID)
	if workspaceID == "" || templateKey == "" {
		return nil, fmt.Errorf("workspace_id and template_key are required")
	}
	tmpl, ok := i.registry.Get(templateKey)
	if !ok {
		return nil, fmt.Errorf("template %q not found", templateKey)
	}
	if err := validateInstallInputs(tmpl, req.Inputs); err != nil {
		return nil, err
	}

	instanceID := newTemplateInstanceID()
	templateVersion := tmpl.Version
	templateName := strings.TrimSpace(req.Name)
	if templateName == "" {
		templateName = tmpl.Name
	}

	var result InstallResult
	err := i.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result.Template = tmpl
		agent, err := i.resolveAgent(ctx, tx, tmpl, workspaceID, instanceID, templateVersion, req.Inputs)
		if err != nil {
			return err
		}
		result.Agent = agent

		rule, err := buildRule(tmpl, workspaceID, actorID, templateName, instanceID, templateVersion, req.Inputs, agent)
		if err != nil {
			return err
		}
		if err := tx.Create(rule).Error; err != nil {
			return fmt.Errorf("create automation rule: %w", err)
		}
		result.Rule = rule
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (i *Installer) resolveAgent(ctx context.Context, tx *gorm.DB, tmpl Template, workspaceID, instanceID string, templateVersion int, inputs map[string]any) (*model.Agent, error) {
	mode, err := tmpl.Agent.mode()
	if err != nil {
		return nil, err
	}
	switch mode {
	case "none":
		return nil, nil
	case "reuse_system":
		var agent model.Agent
		err := tx.WithContext(ctx).
			Where("workspace_id = ? AND is_system = ? AND preset_key = ?", workspaceID, true, tmpl.Agent.ReuseSystem).
			Order("created_at ASC").
			First(&agent).Error
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("system agent %q not found", tmpl.Agent.ReuseSystem)
		}
		if err != nil {
			return nil, fmt.Errorf("load system agent: %w", err)
		}
		return &agent, nil
	case "pick_existing":
		agentID := stringInput(inputs, "agent_id")
		if agentID == "" {
			return nil, fmt.Errorf("agent_id is required")
		}
		var agent model.Agent
		err := tx.WithContext(ctx).
			Where("id = ? AND workspace_id = ?", agentID, workspaceID).
			First(&agent).Error
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("agent %q not found", agentID)
		}
		if err != nil {
			return nil, fmt.Errorf("load agent: %w", err)
		}
		if err := validatePickedAgent(tmpl.Agent.PickExisting.Constraints, &agent); err != nil {
			return nil, err
		}
		return &agent, nil
	case "create":
		agent := &model.Agent{
			ID:                    newTemplateInstanceID(),
			WorkspaceID:           workspaceID,
			IsSystem:              false,
			Name:                  renderNameTemplate(tmpl.Agent.Create.NameTemplate, tmpl.Name),
			PresetKey:             strings.TrimSpace(tmpl.Agent.Create.Preset),
			Role:                  tmpl.Name,
			Status:                "idle",
			RuntimeKind:           firstNonEmpty(tmpl.Agent.Create.RuntimeKind, model.AgentTemplateRuntimeKindNativeSDK),
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			ExecutionConfig:       model.JSONBlob(`{}`),
			AllowedTools:          mustJSON(tmpl.Agent.Create.AllowedTools),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        mustJSON(tmpl.Agent.Create.AllowedTargets),
			ApprovalMode:          firstNonEmpty(tmpl.Agent.Create.ApprovalMode, "always"),
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: "interactive",
			TemplateKey:           strPtr(tmpl.Key),
			TemplateInstanceID:    strPtr(instanceID),
			TemplateVersion:       &templateVersion,
		}
		if err := tx.WithContext(ctx).Create(agent).Error; err != nil {
			return nil, fmt.Errorf("create template agent: %w", err)
		}
		return agent, nil
	default:
		return nil, fmt.Errorf("unsupported agent mode %q", mode)
	}
}

func buildRule(tmpl Template, workspaceID, actorID, name, instanceID string, templateVersion int, inputs map[string]any, agent *model.Agent) (*model.AutomationRule, error) {
	triggerConfig, err := buildTriggerConfig(tmpl, inputs)
	if err != nil {
		return nil, err
	}
	actionConfig, err := buildActionConfig(tmpl, inputs, agent)
	if err != nil {
		return nil, err
	}
	triggerType := strings.TrimSpace(tmpl.Trigger.Event)
	if triggerType == "" && tmpl.Trigger.Type == model.TriggerCron {
		triggerType = model.TriggerCron
	}
	return &model.AutomationRule{
		ID:                 newTemplateInstanceID(),
		WorkspaceID:        workspaceID,
		Name:               name,
		Enabled:            true,
		WorkflowID:         nilIfBlank(stringInput(inputs, "workflow_id")),
		TriggerType:        triggerType,
		TriggerConfig:      triggerConfig,
		ActionType:         tmpl.Flow.Action,
		ActionConfig:       actionConfig,
		CreatedBy:          nilIfBlank(actorID),
		TemplateKey:        strPtr(tmpl.Key),
		TemplateInstanceID: strPtr(instanceID),
		TemplateVersion:    &templateVersion,
	}, nil
}

func buildTriggerConfig(tmpl Template, inputs map[string]any) (json.RawMessage, error) {
	if tmpl.Trigger.Type == model.TriggerCron {
		schedule := stringInput(inputs, "schedule")
		if schedule == "" {
			schedule = "0 * * * *"
		}
		return json.Marshal(map[string]string{"schedule": schedule})
	}
	cfg := map[string]any{}
	switch tmpl.Trigger.Event {
	case model.TriggerTaskStateEntered:
		if stateID := firstNonEmpty(stringInput(inputs, "done_state_id"), stringInput(inputs, "from_state_id")); stateID != "" {
			cfg["state_id"] = stateID
		}
	case model.TriggerAgentRunApproved:
		if stateID := stringInput(inputs, "from_state_id"); stateID != "" {
			cfg["state_id"] = stateID
		}
	case model.TriggerGitHubPRMerged, model.TriggerGitHubCheckSuite:
		if branch := firstNonEmpty(stringInput(inputs, "base_branch"), stringInput(inputs, "branch")); branch != "" {
			if tmpl.Trigger.Event == model.TriggerGitHubCheckSuite {
				cfg["branch"] = branch
			} else {
				cfg["base_branch"] = branch
			}
		}
		if conclusion := stringInput(inputs, "conclusion"); conclusion != "" {
			cfg["conclusion"] = conclusion
		}
	case model.TriggerGitHubReleasePub:
		if includePrerelease, ok := inputs["include_prerelease"].(bool); ok {
			cfg["include_prerelease"] = includePrerelease
		}
	}
	return json.Marshal(cfg)
}

func buildActionConfig(tmpl Template, inputs map[string]any, agent *model.Agent) (json.RawMessage, error) {
	switch tmpl.Flow.Action {
	case model.ActionStartAgentRun:
		if agent == nil {
			return nil, fmt.Errorf("start_agent_run requires an agent")
		}
		cfg := map[string]any{"agent_id": agent.ID}
		if targetInput := strings.TrimSpace(tmpl.Flow.Target["from_input"]); targetInput != "" {
			targetID := stringInput(inputs, targetInput)
			if targetID != "" {
				cfg["target_id"] = targetID
				cfg["target_type"] = inputType(tmpl, targetInput)
			}
		}
		copyFlowParameters(cfg, tmpl, inputs)
		return json.Marshal(cfg)
	case model.ActionMoveToState:
		return json.Marshal(map[string]string{"target_state_id": resolveParameterInput(tmpl, inputs, "target_state_id")})
	case model.ActionMergeBranch:
		return json.Marshal(map[string]string{"target_branch": resolveParameterInput(tmpl, inputs, "target_branch")})
	case model.ActionRunCommand:
		cfg := map[string]any{}
		copyFlowParameters(cfg, tmpl, inputs)
		return json.Marshal(cfg)
	default:
		return nil, fmt.Errorf("unsupported action %q", tmpl.Flow.Action)
	}
}

func validateInstallInputs(tmpl Template, inputs map[string]any) error {
	for _, input := range tmpl.Inputs {
		if !input.Required {
			continue
		}
		value, ok := inputs[input.Key]
		if !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("input %q is required", input.Key)
		}
	}
	return nil
}

func validatePickedAgent(constraints PickExistingConstraints, agent *model.Agent) error {
	if len(constraints.Presets) > 0 && !stringSetContains(constraints.Presets, agent.PresetKey) && !stringSetContains(constraints.Presets, agent.SourcePresetKey) {
		return fmt.Errorf("agent %q does not match required preset", agent.ID)
	}
	if len(constraints.Targets) > 0 {
		var targets []string
		if err := json.Unmarshal(agent.AllowedTargets, &targets); err != nil {
			return fmt.Errorf("agent %q has invalid allowed_targets", agent.ID)
		}
		if !stringSetsOverlap(constraints.Targets, targets) {
			return fmt.Errorf("agent %q cannot run on required targets", agent.ID)
		}
	}
	return nil
}

func copyFlowParameters(out map[string]any, tmpl Template, inputs map[string]any) {
	for key, raw := range tmpl.Flow.Parameters {
		source, ok := raw.(map[string]any)
		if !ok {
			out[key] = raw
			continue
		}
		fromInput, _ := source["from_input"].(string)
		if fromInput == "" {
			out[key] = raw
			continue
		}
		if value, ok := inputs[fromInput]; ok {
			out[key] = value
		}
	}
}

func resolveParameterInput(tmpl Template, inputs map[string]any, key string) string {
	raw, ok := tmpl.Flow.Parameters[key]
	if !ok {
		return ""
	}
	source, ok := raw.(map[string]any)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(raw))
	}
	fromInput, _ := source["from_input"].(string)
	if fromInput == "" {
		return ""
	}
	return stringInput(inputs, fromInput)
}

func inputType(tmpl Template, inputKey string) string {
	for _, input := range tmpl.Inputs {
		if input.Key != inputKey {
			continue
		}
		switch input.Type {
		case "repository":
			return "repository"
		case "collection":
			return "document"
		default:
			return input.Type
		}
	}
	return ""
}

func stringSetsOverlap(left, right []string) bool {
	for _, value := range left {
		if stringSetContains(right, value) {
			return true
		}
	}
	return false
}

func stringSetContains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func stringInput(inputs map[string]any, key string) string {
	value, ok := inputs[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func renderNameTemplate(template, templateName string) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return templateName
	}
	return strings.ReplaceAll(template, "{{template_name}}", templateName)
}

func mustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return payload
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func strPtr(value string) *string {
	value = strings.TrimSpace(value)
	return &value
}

func nilIfBlank(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func newTemplateInstanceID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return ""
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(bytes[:])
	return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
}
