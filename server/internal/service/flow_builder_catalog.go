package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	flowtemplates "github.com/helpin-ai/helpin/server/internal/templates"
)

type flowBuilderAgent struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Targets         []string `json:"targets"`
	PresetKey       string   `json:"preset_key,omitempty"`
	SourcePresetKey string   `json:"source_preset_key,omitempty"`
}
type flowBuilderRepository struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}
type flowBuilderTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
type flowBuilderCatalog struct {
	SourceRule         *model.AutomationRule      `json:"-"`
	WorkspaceName      string                     `json:"workspace_name"`
	WebsiteURL         *string                    `json:"website_url,omitempty"`
	Timezone           string                     `json:"timezone"`
	UTCOffsetMinutes   int                        `json:"utc_offset_minutes"`
	CurrentTime        string                     `json:"current_time"`
	Tools              []flowBuilderTool          `json:"tools,omitempty"`
	Skills             []model.SkillCatalogEntry  `json:"skills,omitempty"`
	Branches           []model.GitBranch          `json:"branches,omitempty"`
	WorkspaceID        string                     `json:"workspace_id"`
	Agents             []flowBuilderAgent         `json:"agents"`
	Repositories       []flowBuilderRepository    `json:"repositories"`
	Workflows          []model.WorkflowWithStates `json:"workflows"`
	Teams              []model.WorkspaceTeam      `json:"teams"`
	Template           *flowtemplates.Template    `json:"template,omitempty"`
	SemanticConditions bool                       `json:"semantic_conditions_available"`
}

func (s *DockChatService) flowCatalog(ctx context.Context, chat *model.DockChat, actor *authorization.Actor) (flowBuilderCatalog, error) {
	c := flowBuilderCatalog{WorkspaceID: chat.WorkspaceID, Timezone: "UTC", SourceRule: chat.FlowBuilder.SourceRule}
	workspace, err := s.commandService.workspaceRepo.GetByID(ctx, chat.WorkspaceID)
	if err != nil {
		return c, err
	}
	if workspace == nil {
		return c, fmt.Errorf("workspace not found")
	}
	c.WorkspaceName, c.WebsiteURL = workspace.Name, workspace.WebsiteURL
	if workspace.Timezone != "" {
		c.Timezone = workspace.Timezone
	}
	location, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return c, fmt.Errorf("workspace timezone is invalid")
	}
	now := time.Now().In(location)
	_, offset := now.Zone()
	c.UTCOffsetMinutes, c.CurrentTime = offset/60, now.Format(time.RFC3339)
	agents, err := s.agentService.ListAgentsForActor(ctx, chat.WorkspaceID, actor)
	if err != nil {
		return c, err
	}
	for _, a := range agents {
		c.Agents = append(c.Agents, flowBuilderAgent{ID: a.ID, Name: a.Name, Targets: parseJSONStringSlice(a.AllowedTargets), PresetKey: a.PresetKey, SourcePresetKey: a.SourcePresetKey})
	}
	repos, err := s.flowBuilder.engine.gitService.ListRepositories(ctx, chat.WorkspaceID)
	if err != nil {
		return c, err
	}
	for _, r := range repos {
		c.Repositories = append(c.Repositories, flowBuilderRepository{r.ID, r.FullName})
	}
	c.Workflows, err = s.flowBuilder.engine.workflowRepo.ListByWorkspace(ctx, chat.WorkspaceID)
	if err != nil {
		return c, err
	}
	c.Teams, err = s.commandService.workspaceRepo.ListTeams(ctx, chat.WorkspaceID)
	if err != nil {
		return c, err
	}
	c.SemanticConditions = s.flowBuilder.engine.jevDecisions.SemanticConditionAvailability(chat.WorkspaceID).Available
	if chat.FlowBuilder.TemplateKey != "" {
		tmpl, ok := s.flowBuilder.registry.Get(chat.FlowBuilder.TemplateKey)
		if !ok {
			return c, fmt.Errorf("template is no longer available")
		}
		c.Template = &tmpl
	}
	return c, nil
}

func flowTriggerFields(trigger string) []string {
	switch trigger {
	case model.TriggerTaskStateEntered:
		return []string{"state_id", "state_type", "semantic_condition"}
	case model.TriggerAgentRunApproved:
		return []string{"state_id", "semantic_condition"}
	case model.TriggerCron:
		return []string{"schedule", "category", "preset"}
	case model.TriggerGitHubPush, model.TriggerGitLabPush:
		return []string{"repo_full_name", "branch", "semantic_condition"}
	case model.TriggerGitHubPROpened, model.TriggerGitHubPRMerged, model.TriggerGitHubPRClosed, model.TriggerGitHubPRReviewReq, model.TriggerGitLabMROpened, model.TriggerGitLabMRMerged, model.TriggerGitLabMRClosed:
		return []string{"repo_full_name", "base_branch", "semantic_condition"}
	case model.TriggerGitHubReleasePub, model.TriggerGitLabReleasePub:
		return []string{"repo_full_name", "tag_name", "tag_pattern", "release_kinds", "include_prerelease", "semantic_condition"}
	case model.TriggerGitHubCheckSuite, model.TriggerGitLabPipeline:
		return []string{"repo_full_name", "branch", "conclusion", "semantic_condition"}
	default:
		return nil
	}
}

func flowConfig(raw json.RawMessage, allowed []string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, fmt.Errorf("configuration must be an object")
	}
	for key := range fields {
		if !slices.Contains(allowed, key) {
			return nil, fmt.Errorf("unsupported flow field %q", key)
		}
	}
	return fields, nil
}

func validateFlowBuilderCatalog(d model.FlowBuilderDraft, c flowBuilderCatalog) error {
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 200 {
		return fmt.Errorf("flow name must be between 1 and 200 characters")
	}
	fields := flowTriggerFields(d.TriggerType)
	if fields == nil {
		return fmt.Errorf("unsupported trigger %q", d.TriggerType)
	}
	trigger, err := flowConfig(d.TriggerConfig, fields)
	if err != nil {
		return err
	}
	// Validate field types as well as names; malformed model output must never
	// silently turn a specific condition into a workspace-wide trigger.
	for key, raw := range trigger {
		if string(raw) == "null" {
			return fmt.Errorf("invalid %s", key)
		}
		if key == "semantic_condition" {
			if _, err := flowConfig(raw, []string{"text"}); err != nil {
				return err
			}
			continue
		}
		if key == "include_prerelease" {
			var b bool
			if json.Unmarshal(raw, &b) != nil {
				return fmt.Errorf("invalid %s", key)
			}
			continue
		}
		if key == "release_kinds" {
			var a []string
			if json.Unmarshal(raw, &a) != nil {
				return fmt.Errorf("invalid %s", key)
			}
			continue
		}
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return fmt.Errorf("invalid %s", key)
		}
	}
	var t map[string]any
	if err := json.Unmarshal(d.TriggerConfig, &t); err != nil {
		return err
	}
	repoName, _ := t["repo_full_name"].(string)
	if repoName != "" && !slices.ContainsFunc(c.Repositories, func(r flowBuilderRepository) bool { return r.FullName == repoName }) {
		return fmt.Errorf("choose a connected repository")
	}
	if d.TeamID != nil && *d.TeamID != "" && !slices.ContainsFunc(c.Teams, func(t model.WorkspaceTeam) bool { return t.ID == *d.TeamID }) {
		return fmt.Errorf("team is not in this workspace")
	}
	var workflow *model.WorkflowWithStates
	if d.WorkflowID != nil && *d.WorkflowID != "" {
		for i := range c.Workflows {
			if c.Workflows[i].Workflow.ID == *d.WorkflowID {
				workflow = &c.Workflows[i]
				break
			}
		}
		if workflow == nil {
			return fmt.Errorf("workflow is not in this workspace")
		}
	}
	stateID, _ := t["state_id"].(string)
	if d.TriggerType == model.TriggerTaskStateEntered || d.TriggerType == model.TriggerAgentRunApproved {
		if workflow == nil && stateID != "" {
			for i := range c.Workflows {
				if slices.ContainsFunc(c.Workflows[i].States, func(s model.PMWorkflowState) bool { return s.ID == stateID }) {
					workflow = &c.Workflows[i]
					break
				}
			}
			if workflow == nil {
				return fmt.Errorf("choose an available workflow state")
			}
		}
		if workflow == nil && d.TriggerType == model.TriggerTaskStateEntered && t["state_type"] == nil {
			return fmt.Errorf("choose a workflow state or state type")
		}
		if value, ok := t["state_type"].(string); ok && !slices.Contains([]string{model.PMStateTypeBacklog, model.PMStateTypeUnstarted, model.PMStateTypeStarted, model.PMStateTypeDone}, value) {
			return fmt.Errorf("unsupported state type")
		}

		if stateID != "" && workflow != nil && !slices.ContainsFunc(workflow.States, func(s model.PMWorkflowState) bool { return s.ID == stateID }) {
			return fmt.Errorf("state is not in the selected workflow")
		}
	}
	var actionFields []string
	switch d.ActionType {
	case model.ActionStartAgentRun:
		actionFields = []string{"agent_id", "target_type", "target_id", "additional_context", "base_branch", "working_branch", "output", "legacy_schedule_agent_id"}
	case model.ActionMoveToState:
		actionFields = []string{"target_state_id"}
	case model.ActionMergeBranch:
		actionFields = []string{"target_branch"}
	default:
		return fmt.Errorf("unsupported action %q", d.ActionType)
	}
	// Templates may persist additional installer-owned parameters. Keep these
	// unchanged during ordinary flow edits rather than rejecting or dropping them.
	inherited := map[string]bool{}
	if c.SourceRule != nil && c.SourceRule.ActionType == d.ActionType {
		var previous, proposed map[string]any
		if json.Unmarshal(c.SourceRule.ActionConfig, &previous) != nil || json.Unmarshal(d.ActionConfig, &proposed) != nil {
			return fmt.Errorf("invalid action configuration")
		}
		for key, value := range previous {
			if slices.Contains(actionFields, key) {
				continue
			}
			if !reflect.DeepEqual(value, proposed[key]) {
				return fmt.Errorf("preserve template-managed setting %q when editing the flow", key)
			}
			actionFields = append(actionFields, key)
			inherited[key] = true
		}
	}
	a, err := flowConfig(d.ActionConfig, actionFields)
	if err != nil {
		return err
	}
	for key, raw := range a {
		if inherited[key] {
			continue
		}
		if key == "output" {
			if _, err := flowConfig(raw, []string{"type", "space_id", "collection_id", "idempotency_key"}); err != nil {
				return err
			}
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return fmt.Errorf("invalid %s", key)
		}
	}
	if d.ActionType != model.ActionStartAgentRun {
		if workflow == nil {
			return fmt.Errorf("this action requires a task workflow")
		}
		if d.ActionType == model.ActionMoveToState {
			var cfg model.ActionConfigMoveToState
			json.Unmarshal(d.ActionConfig, &cfg)
			if !slices.ContainsFunc(workflow.States, func(s model.PMWorkflowState) bool { return s.ID == cfg.TargetStateID }) {
				return fmt.Errorf("destination state is not in the workflow")
			}
		}
		return nil
	}
	var cfg model.ActionConfigRunAgent
	if err := json.Unmarshal(d.ActionConfig, &cfg); err != nil {
		return err
	}
	target := cfg.TargetType
	if target == "" {
		if d.TriggerType == model.TriggerCron {
			return fmt.Errorf("scheduled flows need an explicit target")
		}
		if strings.HasPrefix(d.TriggerType, "github.") || strings.HasPrefix(d.TriggerType, "gitlab.") {
			target = "repository"
		} else {
			target = "task"
		}
	}
	if !slices.Contains([]string{"task", "epic", "repository", "workspace", "crm_deal", "crm_contact", "crm_company"}, target) {
		return fmt.Errorf("unsupported target %q", target)
	}
	found := false
	for _, agent := range c.Agents {
		if agent.ID == cfg.AgentID {
			found = true
			if !slices.Contains(agent.Targets, target) {
				return fmt.Errorf("agent cannot run on %s", target)
			}
		}
	}
	if !found {
		return fmt.Errorf("choose an available agent")
	}
	if cfg.TargetType == "repository" && !slices.ContainsFunc(c.Repositories, func(r flowBuilderRepository) bool { return r.ID == cfg.TargetID }) {
		return fmt.Errorf("target repository is not available")
	}
	if cfg.TargetType == "workspace" && cfg.TargetID != c.WorkspaceID {
		return fmt.Errorf("target workspace does not match")
	}
	return nil
}
