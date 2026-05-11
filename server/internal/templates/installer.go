package templates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

var templatePlaceholderPattern = regexp.MustCompile(`\{\{[a-zA-Z0-9_]+\}\}`)

type Installer struct {
	db       *gorm.DB
	registry *Registry
}

type InstallRequest struct {
	WorkspaceID    string
	TemplateKey    string
	ActorID        string
	Name           string
	AgentName      string
	Inputs         map[string]any
	AgentOverrides *model.CreateAgentFromTemplateOverrides
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
		agent, err := i.resolveAgent(ctx, tx, tmpl, workspaceID, instanceID, templateVersion, req.AgentName, req.Inputs, req.AgentOverrides)
		if err != nil {
			return err
		}
		result.Agent = agent

		rule, err := buildRule(ctx, tx, tmpl, workspaceID, actorID, templateName, instanceID, templateVersion, req.Inputs, agent)
		if err != nil {
			return err
		}
		if err := tx.Create(rule).Error; err != nil {
			return fmt.Errorf("create automation rule: %w", err)
		}
		if err := logTemplateActivity(ctx, tx, workspaceID, rule.ID, actorID, "template.installed", map[string]any{
			"template_key":         tmpl.Key,
			"template_instance_id": instanceID,
			"template_version":     templateVersion,
			"agent_id":             agentIDForActivity(agent),
		}); err != nil {
			return err
		}
		result.Rule = rule
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to install flow template",
			"error", err,
			"template_key", templateKey,
			"workspace_id", workspaceID,
			"actor_id", actorID,
		)
		return nil, err
	}
	slog.InfoContext(ctx, "flow template installed",
		"template_key", result.Template.Key,
		"template_instance_id", derefString(result.Rule.TemplateInstanceID),
		"workspace_id", workspaceID,
		"actor_id", actorID,
		"rule_id", result.Rule.ID,
		"agent_id", agentIDForActivity(result.Agent),
	)
	return &result, nil
}

func (i *Installer) resolveAgent(ctx context.Context, tx *gorm.DB, tmpl Template, workspaceID, instanceID string, templateVersion int, agentName string, inputs map[string]any, overrides *model.CreateAgentFromTemplateOverrides) (*model.Agent, error) {
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
		name := renderNameTemplate(tmpl.Agent.Create.NameTemplate, tmpl.Name)
		if override := strings.TrimSpace(agentName); override != "" {
			name = override
		}
		agent := &model.Agent{
			ID:                    newTemplateInstanceID(),
			WorkspaceID:           workspaceID,
			IsSystem:              false,
			Name:                  name,
			PresetKey:             strings.TrimSpace(tmpl.Agent.Create.Preset),
			Role:                  tmpl.Name,
			Status:                "idle",
			RuntimeKind:           firstNonEmpty(tmpl.Agent.Create.RuntimeKind, model.AgentTemplateRuntimeKindNativeSDK),
			Skills:                templateSkillRefs(tmpl.Agent.Create.Skills),
			TriggerMode:           "manual",
			ExecutionConfig:       model.JSONBlob(`{}`),
			SystemPrompt:          strPtr(defaultTemplateSystemPrompt(tmpl, inputs)),
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
		applyTemplateAgentOverrides(agent, overrides)
		if err := tx.WithContext(ctx).Create(agent).Error; err != nil {
			return nil, fmt.Errorf("create template agent: %w", err)
		}
		return agent, nil
	default:
		return nil, fmt.Errorf("unsupported agent mode %q", mode)
	}
}

func buildRule(ctx context.Context, tx *gorm.DB, tmpl Template, workspaceID, actorID, name, instanceID string, templateVersion int, inputs map[string]any, agent *model.Agent) (*model.AutomationRule, error) {
	triggerConfig, err := buildTriggerConfig(ctx, tx, tmpl, workspaceID, inputs)
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

func buildTriggerConfig(ctx context.Context, tx *gorm.DB, tmpl Template, workspaceID string, inputs map[string]any) (json.RawMessage, error) {
	if tmpl.Trigger.Type == model.TriggerCron {
		schedule := stringInput(inputs, "schedule")
		if schedule == "" {
			schedule = "0 * * * *"
		}
		return json.Marshal(map[string]string{"schedule": schedule})
	}
	cfg := map[string]any{}
	if repoFullName, err := triggerRepoFullName(ctx, tx, tmpl, workspaceID, inputs); err != nil {
		return nil, err
	} else if repoFullName != "" {
		cfg["repo_full_name"] = repoFullName
	}
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

func triggerRepoFullName(ctx context.Context, tx *gorm.DB, tmpl Template, workspaceID string, inputs map[string]any) (string, error) {
	switch strings.TrimSpace(tmpl.Trigger.Event) {
	case model.TriggerGitHubPush,
		model.TriggerGitHubPROpened,
		model.TriggerGitHubPRMerged,
		model.TriggerGitHubPRClosed,
		model.TriggerGitHubPRReviewReq,
		model.TriggerGitHubReleasePub,
		model.TriggerGitHubCheckSuite:
	default:
		return "", nil
	}
	if repoFullName := stringInput(inputs, "repo_full_name"); repoFullName != "" {
		return repoFullName, nil
	}
	repositoryID := stringInput(inputs, "repository_id")
	if repositoryID == "" {
		return "", nil
	}
	var repo model.GitRepository
	err := tx.WithContext(ctx).
		Select("full_name").
		Where("id = ? AND workspace_id = ?", repositoryID, workspaceID).
		First(&repo).Error
	if err == gorm.ErrRecordNotFound {
		return "", fmt.Errorf("repository %q not found", repositoryID)
	}
	if err != nil {
		return "", fmt.Errorf("load repository: %w", err)
	}
	if strings.TrimSpace(repo.FullName) == "" {
		return "", fmt.Errorf("repository %q has no full name", repositoryID)
	}
	return strings.TrimSpace(repo.FullName), nil
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
		if additionalContext := strings.TrimSpace(tmpl.Flow.AdditionalContext); additionalContext != "" {
			cfg["additional_context"] = renderTemplateText(additionalContext, inputs)
		}
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

func applyTemplateAgentOverrides(agent *model.Agent, overrides *model.CreateAgentFromTemplateOverrides) {
	if agent == nil || overrides == nil {
		return
	}
	if overrides.Role != nil {
		if value := strings.TrimSpace(*overrides.Role); value != "" {
			agent.Role = value
		}
	}
	if overrides.RuntimeKind != nil {
		if value := strings.TrimSpace(*overrides.RuntimeKind); value != "" {
			agent.RuntimeKind = value
		}
	}
	if overrides.Skills != nil {
		agent.Skills = overrides.Skills.Normalize()
	}
	if overrides.Provider != nil {
		agent.Provider = trimPtr(overrides.Provider)
	}
	if overrides.Model != nil {
		agent.Model = trimPtr(overrides.Model)
	}
	if overrides.MonthlyTokenBudget != nil {
		if *overrides.MonthlyTokenBudget > 0 {
			agent.MonthlyTokenBudget = overrides.MonthlyTokenBudget
		} else {
			agent.MonthlyTokenBudget = nil
		}
	}
	if len(overrides.ExecutionConfig) > 0 {
		agent.ExecutionConfig = overrides.ExecutionConfig
	}
	if overrides.SystemPrompt != nil {
		agent.SystemPrompt = trimPtr(overrides.SystemPrompt)
	}
	if overrides.PlanningNotes != nil {
		agent.PlanningNotes = trimPtr(overrides.PlanningNotes)
	}
	if len(overrides.AllowedTools) > 0 {
		agent.AllowedTools = json.RawMessage(overrides.AllowedTools)
	}
	if len(overrides.AllowedCommands) > 0 {
		agent.AllowedCommands = json.RawMessage(overrides.AllowedCommands)
	}
	if len(overrides.AllowedTargets) > 0 {
		agent.AllowedTargets = json.RawMessage(overrides.AllowedTargets)
	}
	if overrides.ApprovalMode != nil {
		if value := strings.TrimSpace(*overrides.ApprovalMode); value != "" {
			agent.ApprovalMode = value
		}
	}
	if overrides.MaxConcurrentRuns != nil && *overrides.MaxConcurrentRuns > 0 {
		agent.MaxConcurrentRuns = *overrides.MaxConcurrentRuns
	}
	if overrides.DefaultInvocationMode != nil {
		if value := strings.TrimSpace(*overrides.DefaultInvocationMode); value != "" {
			agent.DefaultInvocationMode = value
		}
	}
}

func defaultTemplateSystemPrompt(tmpl Template, inputs map[string]any) string {
	if tmpl.Agent.Create != nil {
		if prompt := strings.TrimSpace(tmpl.Agent.Create.SystemPrompt); prompt != "" {
			return renderTemplateText(prompt, inputs)
		}
	}
	lines := []string{
		fmt.Sprintf("You are running the %s flow template.", strings.TrimSpace(tmpl.Name)),
		"Use the configured flow inputs as resolved product context. Do not ask the user to provide these values again.",
	}
	if len(inputs) > 0 {
		if payload, err := json.MarshalIndent(inputs, "", "  "); err == nil {
			lines = append(lines, "", "Configured inputs:", "```json", string(payload), "```")
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderTemplateText(text string, inputs map[string]any) string {
	payload, err := json.MarshalIndent(inputs, "", "  ")
	rawJSON := "{}"
	if err == nil {
		rawJSON = string(payload)
	}
	replacerArgs := []string{"{{raw_configuration_json}}", "```json\n" + rawJSON + "\n```"}
	for key, value := range inputs {
		replacerArgs = append(replacerArgs, "{{"+key+"}}", inputLabelValue(value))
	}
	rendered := strings.NewReplacer(replacerArgs...).Replace(text)
	rendered = templatePlaceholderPattern.ReplaceAllString(rendered, "")
	return strings.TrimSpace(rendered)
}

func inputLabelValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(typed, ", ")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func templateSkillRefs(keys []string) model.AgentSkillRefs {
	refs := make(model.AgentSkillRefs, 0, len(keys))
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			refs = append(refs, model.AgentSkillRef{Key: key})
		}
	}
	return refs.Normalize()
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

func trimPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return nilIfBlank(*value)
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
