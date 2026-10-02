package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	flowtemplates "github.com/helpin-ai/helpin/server/internal/templates"
)

type flowBuilderDependencies struct {
	engine    *AutomationRuleEngine
	registry  *flowtemplates.Registry
	installer *flowtemplates.Installer
}

func (s *DockChatService) SetFlowBuilder(engine *AutomationRuleEngine, registry *flowtemplates.Registry, installer *flowtemplates.Installer) *DockChatService {
	s.flowBuilder = &flowBuilderDependencies{engine, registry, installer}
	return s
}

func flowBuilderTools() []string {
	return []string{"get_flow_builder_context", "preview_flow", "save_flow", "request_user_input", "request_approval", "list_spaces", "list_collections", "search_workspace", "list_tasks", "list_deals", "list_contacts"}
}

func isFlowBuilderRun(run *model.AgentRun) bool {
	if run == nil || run.DockChatID == nil || strings.TrimSpace(*run.DockChatID) == "" || run.TargetType != "workspace" || run.TargetID != run.WorkspaceID {
		return false
	}
	var input model.AgentRunInputPayload
	return json.Unmarshal(run.Input, &input) == nil && !input.ExecutionEnabled && sameNormalizedToolSet(input.AllowedTools, flowBuilderTools())
}

const flowBuilderInstructions = `<flow_builder_instructions>
This conversation exclusively builds or edits one automation flow. If current.source_rule exists, first describe its current behavior in one concise prose paragraph, bolding key names, triggers and conditions with Markdown, then ask "What would you like to change?" Do not propose or save changes until the user requests them. Preserve its identity and all unchanged settings. After revisions, ask whether to save the changes; save_flow updates this existing flow. Do not delegate, execute the proposed work, or create unrelated resources. Call get_flow_builder_context first. It returns the actual workspace catalog, supported configuration, current draft, and selected template (if any). Use real IDs from tools. Never invent agents, repositories, teams, states, or supported triggers. Do not select a team unless the user specifies one. Preserve the chosen template and its defaults; ask only for missing required inputs or genuine ambiguity. Use request_user_input for concise questions with named options; never ask for IDs. For a selected template, provide template_inputs and optional agent_name/agent_overrides to preview_flow; the installer derives its trigger/action and new agent. For custom flows provide the supported trigger/action configuration. Omitted draft settings and config fields are preserved on revisions. To remove a trigger/action filter set its config value to null; use an empty string to clear description, team_id, workflow_id or semantic_condition. Always supply a new summary when revising template-specific behavior. Existing installer-owned action fields must remain unchanged when editing; the tool identifies these fields. Schedules use five-field cron in UTC. Use catalog.timezone as the user's default; clarify only an ambiguous or explicitly different timezone. Convert local times using catalog.utc_offset_minutes, including day/week rollover, and set schedule_timezone to the user's IANA timezone for review. UTC schedules are fixed; do not promise daylight-saving adjustment. The preview includes actual upcoming local times; check these against the request. To discover branches call get_flow_builder_context with repository_id; working_branch may intentionally be a new branch. To customize a template-created agent call get_flow_builder_context with include_agent_options=true; use catalog tool names and skill keys/IDs, including required tools. All former template settings are supported: agent_name, system_prompt, approval_mode, allowed_targets, allowed_tools, skills and max_concurrent_runs. Only create-mode templates accept agent overrides. Match pick_existing preset/target constraints to catalog agents. Respect template depends_on, show_if and space_type constraints; skip hidden fields. Use workspace_name/website_url for company defaults when applicable, and confirm ambiguity rather than guessing. A scheduled agent needs an explicit target. Preserve any supplied condition. Exact conditions use supported trigger filters; meaning-based conditions use semantic_condition.text only when available. Conditions are ANDed with trigger filters; express any/all semantic logic clearly in that text. Never silently drop an unsupported condition or promise unsupported event data.
Once details are complete call preview_flow. For templates include summary: concise descriptive prose explaining the template-specific behavior and chosen parameters (destinations, task creation, thresholds, limits and other active inputs), with important names and values bolded using **Markdown**. Do not repeat the trigger, schedule, agent, or approval settings already shown by the UI. Keep description descriptive prose too. Fix validation errors or ask the missing question. The returned draft drives the user's preview. The UI renders a descriptive prose summary of the validated draft with key details bolded. Do not repeat it in a second message or use tables, field lists, or diagrams. Introduce the proposal in one short sentence, then ask whether to create it using request_approval with phase="flow_confirm", summary describing the flow, and action={"revision":"<returned revision>"}. Do not merely tell the user to click a separate button. After explicit approval call save_flow with only approval_interaction_id. A new preview invalidates old approvals. If the user requests changes, revise the preview and ask again. Never call save_flow without the matching approval. Report success only after the tool returns a flow ID. Keep replies short; don't repeat the preview or narrate tool calls. If a request is unsupported, explain the specific limitation and offer a supported alternative without changing the user's intent on your own.
</flow_builder_instructions>`

func (s *DockChatService) authorizeFlowBuilder(ctx context.Context, chat *model.DockChat, userID string) (*authorization.Actor, error) {
	if s.flowBuilder == nil || s.authz == nil || chat == nil || chat.UserID != userID || chat.FlowBuilder == nil || chat.ArchivedAt != nil {
		return nil, fmt.Errorf("flow builder is unavailable")
	}
	actor, err := s.authz.ResolveActor(ctx, chat.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}
	allowed, err := s.authz.CanAccessModule(ctx, actor, model.ModuleAutomation)
	if err != nil {
		return nil, err
	}
	if !allowed || !s.authz.Can(actor, authorization.PermPMAdminAutomations) {
		return nil, fmt.Errorf("automation management permission is required")
	}
	return actor, nil
}

func (s *DockChatService) createFlowBuilderChat(ctx context.Context, workspaceID, userID string, req model.CreateDockChatRequest) (*model.DockChat, error) {
	if req.CoverageGapID != nil || req.SupportConversationID != nil || req.ModuleID != nil || req.ExecutionEnabled || (req.Visibility != nil && *req.Visibility != model.DockChatVisibilityPrivate) {
		return nil, fmt.Errorf("flow builder conversations must be private")
	}
	chat := &model.DockChat{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: userID, Title: "New flow", Visibility: model.DockChatVisibilityPrivate, FlowBuilder: &model.FlowBuilderState{TemplateKey: strings.TrimSpace(req.FlowTemplateKey)}}
	if _, err := s.authorizeFlowBuilder(ctx, chat, userID); err != nil {
		return nil, err
	}
	if req.FlowID != "" {
		if req.FlowTemplateKey != "" {
			return nil, fmt.Errorf("choose a flow or a template")
		}
		rule, err := s.flowBuilder.engine.GetRule(ctx, workspaceID, req.FlowID)
		if err != nil {
			return nil, err
		}
		if rule == nil {
			return nil, fmt.Errorf("flow not found")
		}
		if rule.TriggerType == model.CRMPlaybookWorkDue {
			return nil, fmt.Errorf("manage this flow from its CRM Playbook")
		}
		chat.Title = rule.Name
		chat.FlowBuilder.SourceRule = rule
		chat.FlowBuilder.Draft = &model.FlowBuilderDraft{Name: rule.Name, Description: rule.Description, TeamID: rule.TeamID, WorkflowID: rule.WorkflowID, TriggerType: rule.TriggerType, TriggerConfig: rule.TriggerConfig, ActionType: rule.ActionType, ActionConfig: rule.ActionConfig, Paused: !rule.Enabled}
	}
	if chat.FlowBuilder.TemplateKey != "" {
		tmpl, ok := s.flowBuilder.registry.Get(chat.FlowBuilder.TemplateKey)
		if !ok {
			return nil, fmt.Errorf("template not found")
		}
		chat.Title = tmpl.Name
	}
	if chat.FlowBuilder.Draft != nil {
		actor, err := s.authorizeFlowBuilder(ctx, chat, userID)
		if err != nil {
			return nil, err
		}
		catalog, err := s.flowCatalog(ctx, chat, actor)
		if err != nil {
			return nil, err
		}
		if err := s.labelFlowDraft(ctx, chat, chat.FlowBuilder.Draft, catalog); err != nil {
			return nil, err
		}
	}
	if err := s.chatRepo.Create(ctx, chat); err != nil {
		return nil, err
	}
	return chat, nil
}

func (s *DockChatService) saveFlowBuilderState(ctx context.Context, chat *model.DockChat) error {
	raw, err := json.Marshal(chat.FlowBuilder)
	if err != nil {
		return err
	}
	return s.chatRepo.Update(ctx, chat.WorkspaceID, chat.ID, map[string]interface{}{"flow_builder": string(raw)})
}

func (s *DockChatService) previewFlow(ctx context.Context, chat *model.DockChat, actor *authorization.Actor, draft model.FlowBuilderDraft) (*model.FlowBuilderState, error) {
	if draft.SemanticCondition != "" && chat.FlowBuilder.TemplateKey == "" {
		var config map[string]any
		if err := json.Unmarshal(draft.TriggerConfig, &config); err != nil {
			return nil, err
		}
		config["semantic_condition"] = map[string]string{"text": draft.SemanticCondition}
		draft.TriggerConfig, _ = json.Marshal(config)
	}
	catalog, err := s.flowCatalog(ctx, chat, actor)
	if err != nil {
		return nil, err
	}
	if chat.FlowBuilder.CreatedFlowID != "" {
		return nil, fmt.Errorf("this flow has already been saved")
	}
	createsAgent := catalog.Template != nil && catalog.Template.Agent.Create != nil
	if err := validateFlowAgentOverrides(draft, createsAgent); err != nil {
		return nil, err
	}
	if err := s.validateFlowAgentTools(ctx, chat.WorkspaceID, draft); err != nil {
		return nil, err
	}
	chat.FlowBuilder.CreatesAgent = createsAgent
	if catalog.Template != nil {
		if strings.TrimSpace(draft.Summary) == "" {
			return nil, fmt.Errorf("describe the selected template behavior and its active settings in summary before review")
		}
		chat.FlowBuilder.TemplateVersion = catalog.Template.Version
		if draft.TemplateInputs == nil {
			draft.TemplateInputs = map[string]any{}
		}
		for _, input := range catalog.Template.Inputs {
			if _, ok := draft.TemplateInputs[input.Key]; !ok && input.Default != nil {
				draft.TemplateInputs[input.Key] = input.Default
			}
		}
		if err := s.validateFlowTemplateReferences(ctx, chat.WorkspaceID, *catalog.Template, draft.TemplateInputs, catalog); err != nil {
			return nil, err
		}
		preview, err := s.flowBuilder.installer.Preview(ctx, flowTemplateInstallRequest(chat, draft))
		if err != nil {
			return nil, err
		}
		draft.Name = preview.Rule.Name
		draft.Description = preview.Rule.Description
		draft.TeamID = preview.Rule.TeamID
		draft.WorkflowID = preview.Rule.WorkflowID
		draft.TriggerType = preview.Rule.TriggerType
		draft.TriggerConfig = preview.Rule.TriggerConfig
		draft.ActionType = preview.Rule.ActionType
		draft.ActionConfig = preview.Rule.ActionConfig
		chat.FlowBuilder.Agent = preview.Agent
	} else {
		if err := validateFlowBuilderCatalog(draft, catalog); err != nil {
			return nil, err
		}
	}
	if err := s.validateFlowDraft(ctx, chat.WorkspaceID, draft, chat.FlowBuilder.SourceRule); err != nil {
		return nil, err
	}
	latest, err := s.chatRepo.LatestFlowBuilderUserMessage(ctx, chat.WorkspaceID, chat.ID)
	if err != nil {
		return nil, err
	}
	if err := s.labelFlowDraft(ctx, chat, &draft, catalog); err != nil {
		return nil, err
	}
	if err := setFlowSchedulePreview(chat.FlowBuilder, &draft, catalog.Timezone, time.Now()); err != nil {
		return nil, err
	}
	chat.FlowBuilder.Draft = &draft
	chat.FlowBuilder.Revision = uuid.NewString()
	chat.FlowBuilder.UserMessageID = latest
	if err := s.saveFlowBuilderState(ctx, chat); err != nil {
		return nil, err
	}
	return chat.FlowBuilder, nil
}

func flowTemplateInstallRequest(chat *model.DockChat, d model.FlowBuilderDraft) flowtemplates.InstallRequest {
	return flowtemplates.InstallRequest{WorkspaceID: chat.WorkspaceID, TemplateKey: chat.FlowBuilder.TemplateKey, ActorID: chat.UserID, Name: d.Name, Description: derefString(d.Description), AgentName: d.AgentName, Inputs: d.TemplateInputs, AgentOverrides: d.AgentOverrides, Paused: d.Paused, SemanticCondition: d.SemanticCondition}
}

func (s *DockChatService) validateFlowDraft(ctx context.Context, workspaceID string, d model.FlowBuilderDraft, previous *model.AutomationRule) error {
	e := s.flowBuilder.engine
	if d.SemanticCondition != "" {
		var fields map[string]any
		if err := json.Unmarshal(d.TriggerConfig, &fields); err != nil {
			return err
		}
		fields["semantic_condition"] = map[string]string{"text": d.SemanticCondition}
		var err error
		d.TriggerConfig, err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}
	if err := e.validateRuleRequest(d.TriggerType, d.TriggerConfig, d.ActionType, d.ActionConfig); err != nil {
		return err
	}
	previousTrigger := ""
	var previousConfig json.RawMessage
	if previous != nil {
		previousTrigger = previous.TriggerType
		previousConfig = previous.TriggerConfig
	}
	if err := e.validateSemanticConditionChange(workspaceID, d.TriggerType, d.TriggerConfig, previousTrigger, previousConfig); err != nil {
		return err
	}
	var cfg model.ActionConfigRunAgent
	if d.ActionType != model.ActionStartAgentRun {
		return nil
	}
	if err := json.Unmarshal(d.ActionConfig, &cfg); err != nil {
		return err
	}
	if cfg.Output != nil {
		if cfg.Output.Type != "docs_document" {
			return fmt.Errorf("unsupported flow output")
		}
		if cfg.Output.SpaceID == "" {
			return fmt.Errorf("choose an output space")
		}
		if err := s.chatRepo.ValidateFlowDocsReference(ctx, workspaceID, cfg.Output.SpaceID, derefString(cfg.Output.CollectionID), "any"); err != nil {
			return err
		}
	}
	if cfg.TargetType != "" && cfg.TargetType != "workspace" {
		exists, err := s.chatRepo.FlowBuilderReferenceExists(ctx, workspaceID, cfg.TargetType, cfg.TargetID)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("target is not in this workspace")
		}
	}
	return nil
}

func (s *DockChatService) validateFlowTemplateReferences(ctx context.Context, workspaceID string, tmpl flowtemplates.Template, inputs map[string]any, c flowBuilderCatalog) error {
	known := map[string]bool{"additional_instructions": true}
	for _, input := range tmpl.Inputs {
		known[input.Key] = true
		if !flowtemplates.InputVisible(input, inputs) {
			continue
		}
		value, _ := inputs[input.Key].(string)
		if value == "" {
			continue
		}
		switch input.Type {
		case "agent":
			found := false
			for _, a := range c.Agents {
				if a.ID == value {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("choose an available agent")
			}
		case "state", "workflow_state":
			found := false
			dependency, _ := inputs[input.DependsOn].(string)
			if input.DependsOn != "" && dependency == "" {
				return fmt.Errorf("choose %s before %s", input.DependsOn, input.Label)
			}
			teamDependency := strings.Contains(input.DependsOn, "team")
			for _, w := range c.Workflows {
				if dependency != "" && ((teamDependency && derefString(w.Workflow.TeamID) != dependency) || (!teamDependency && w.Workflow.ID != dependency)) {
					continue
				}
				for _, state := range w.States {
					if state.ID == value {
						found = true
					}
				}
			}
			if !found {
				return fmt.Errorf("choose a state in the selected workflow")
			}
		case "space":
			spaceType := input.SpaceType
			if spaceType == "" {
				spaceType = model.SpaceTypeInternal
			}
			if err := s.chatRepo.ValidateFlowDocsReference(ctx, workspaceID, value, "", spaceType); err != nil {
				return err
			}
		case "collection":
			space, _ := inputs[input.DependsOn].(string)
			if input.DependsOn != "" && space == "" {
				return fmt.Errorf("choose a space before %s", input.Label)
			}
			if err := s.chatRepo.ValidateFlowDocsReference(ctx, workspaceID, space, value, "any"); err != nil {
				return err
			}
		case "repository", "workflow", "team", "task", "epic":
			exists, err := s.chatRepo.FlowBuilderReferenceExists(ctx, workspaceID, input.Type, value)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("%s is not in this workspace", input.Label)
			}
		}
	}
	for key := range inputs {
		if !known[key] {
			return fmt.Errorf("unsupported template input %q", key)
		}
	}
	return nil
}

func (s *DockChatService) labelFlowDraft(ctx context.Context, chat *model.DockChat, d *model.FlowBuilderDraft, c flowBuilderCatalog) error {
	labels := map[string]string{}
	for _, agent := range c.Agents {
		labels[agent.ID] = agent.Name
	}
	for _, team := range c.Teams {
		labels[team.ID] = team.Name
	}
	var cfg model.ActionConfigRunAgent
	if d.ActionType == model.ActionStartAgentRun {
		if err := json.Unmarshal(d.ActionConfig, &cfg); err != nil {
			return err
		}
		refs := map[string]string{cfg.TargetType: cfg.TargetID}
		if cfg.Output != nil {
			refs["space"] = cfg.Output.SpaceID
			refs["collection"] = derefString(cfg.Output.CollectionID)
		}
		for kind, id := range refs {
			if id == "" {
				continue
			}
			name, err := s.chatRepo.FlowBuilderReferenceLabel(ctx, chat.WorkspaceID, kind, id)
			if err != nil {
				return err
			}
			labels[id] = name
		}
	}
	if chat.FlowBuilder.CreatesAgent && chat.FlowBuilder.Agent != nil && len(chat.FlowBuilder.Agent.Skills) > 0 {
		catalog, err := s.agentService.ListSkillCatalog(ctx, chat.WorkspaceID)
		if err != nil {
			return err
		}
		for _, skill := range catalog.Skills {
			labels[skill.Key] = skill.Title
			if skill.ID != nil {
				labels[*skill.ID] = skill.Title
			}
		}
	}
	chat.FlowBuilder.Labels = labels
	return nil
}
