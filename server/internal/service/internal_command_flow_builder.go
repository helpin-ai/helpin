package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) SetFlowBuilderChat(chat *DockChatService) { s.flowBuilderChat = chat }

func (s *InternalCommandService) registerFlowBuilderCommands() {
	for _, definition := range []struct {
		name, description string
		schema            map[string]any
		execute           func(context.Context, model.InternalCommandContext, json.RawMessage) (json.RawMessage, error)
	}{
		{"get_flow_builder_context", "Read the selected template, supported configuration, workspace timezone, agents/repositories/workflows and current draft. Set include_agent_options to discover allowed tools and skills for template customization. Set repository_id to list its actual branches. Only available in a flow builder conversation.", flowBuilderContextSchema(), s.executeFlowBuilderContext},
		{"preview_flow", "Validate and save a flow draft without creating it. For a selected template use template_inputs (and optional agent_name/agent_overrides); trigger/action are derived by the installer. After success describe the flow and call request_approval with phase flow_confirm and the returned approval action. A revision replaces all earlier drafts.", flowBuilderDraftSchema(), s.executePreviewFlow},
		{"save_flow", "Save exactly the reviewed flow, updating the existing flow when editing, after a resolved flow_confirm approval. Pass only approval_interaction_id. Never creates without explicit approval; retries return the same flow.", map[string]any{"type": "object", "properties": map[string]any{"approval_interaction_id": map[string]any{"type": "string"}}, "required": []string{"approval_interaction_id"}, "additionalProperties": false}, s.executeCreateFlow},
	} {
		s.register(InternalCommandDefinition{Name: "automation." + definition.name, Module: "agents", Mutating: false, RequiredPermissionsAll: []authorization.Permission{authorization.PermPMAdminAutomations}, SupportedTargetTypes: []string{"workspace"}, Tool: &commandtools.RuntimeToolMetadata{CommandName: "automation." + definition.name, Alias: definition.name, Category: "Automation", Description: definition.description, InputSchema: definition.schema}, Execute: definition.execute})
	}
	// save_flow follows the existing approved-proposal tools: the explicit
	// approval is checked inside Execute, rather than asking a second time.
}

func flowBuilderDraftSchema() map[string]any {
	text := map[string]any{"type": "string"}
	object := map[string]any{"type": "object"}
	return map[string]any{"type": "object", "properties": map[string]any{"name": text, "description": text, "summary": text, "team_id": text, "workflow_id": text, "trigger_type": text, "trigger_config": object, "action_type": text, "action_config": object, "template_inputs": object, "agent_name": text, "agent_overrides": flowBuilderAgentOverrideSchema(), "schedule_timezone": text, "semantic_condition": text, "paused": map[string]any{"type": "boolean"}}, "required": []string{}, "additionalProperties": false}
}

func (s *InternalCommandService) flowBuilderCommandChat(ctx context.Context, meta model.InternalCommandContext) (*model.DockChat, *model.AgentRun, *authorization.Actor, error) {
	if s.flowBuilderChat == nil {
		return nil, nil, nil, fmt.Errorf("flow builder is unavailable")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, nil, nil, err
	}
	if run.DockChatID == nil {
		return nil, nil, nil, fmt.Errorf("use a flow builder conversation")
	}
	chat, err := s.dockChatRepo.GetByID(ctx, meta.WorkspaceID, *run.DockChatID)
	if err != nil {
		return nil, nil, nil, err
	}
	actor, err := s.flowBuilderChat.authorizeFlowBuilder(ctx, chat, meta.ActorID)
	if err != nil {
		return nil, nil, nil, err
	}
	if chat.ActiveRunID == nil || *chat.ActiveRunID != run.ID {
		return nil, nil, nil, fmt.Errorf("this builder run is no longer current")
	}
	return chat, run, actor, nil
}

func (s *InternalCommandService) executeFlowBuilderContext(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	chat, _, actor, err := s.flowBuilderCommandChat(ctx, meta)
	if err != nil {
		return nil, err
	}
	catalog, err := s.flowBuilderChat.flowCatalog(ctx, chat, actor)
	if err != nil {
		return nil, err
	}
	var options struct {
		IncludeAgentOptions bool   `json:"include_agent_options"`
		RepositoryID        string `json:"repository_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &options); err != nil {
			return nil, err
		}
	}
	if options.IncludeAgentOptions {
		tools, err := s.flowBuilderChat.agentService.ListToolCatalogForWorkspace(ctx, chat.WorkspaceID)
		if err != nil {
			return nil, err
		}
		for _, tool := range tools.Tools {
			catalog.Tools = append(catalog.Tools, flowBuilderTool{Name: tool.Name, Description: tool.Description, Category: tool.Category})
		}
		skills, err := s.flowBuilderChat.agentService.ListSkillCatalog(ctx, chat.WorkspaceID)
		if err != nil {
			return nil, err
		}
		for _, skill := range skills.Skills {
			skill.Instructions = ""
			catalog.Skills = append(catalog.Skills, skill)
		}
	}
	if options.RepositoryID != "" {
		found := false
		for _, repo := range catalog.Repositories {
			if repo.ID == options.RepositoryID {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("choose a connected repository")
		}
		catalog.Branches, err = s.flowBuilderChat.flowBuilder.engine.gitService.ListRepositoryBranches(ctx, chat.WorkspaceID, options.RepositoryID)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(map[string]any{"catalog": catalog, "current": chat.FlowBuilder, "custom_contract": map[string]any{"trigger_fields": map[string]any{"task.state_entered": flowTriggerFields("task.state_entered"), "agent_run.approved": flowTriggerFields("agent_run.approved"), "github.push": flowTriggerFields("github.push"), "github.pull_request_opened": flowTriggerFields("github.pull_request_opened"), "github.pull_request_merged": flowTriggerFields("github.pull_request_merged"), "github.pull_request_closed": flowTriggerFields("github.pull_request_closed"), "github.pull_request_review_requested": flowTriggerFields("github.pull_request_review_requested"), "github.release_published": flowTriggerFields("github.release_published"), "github.check_suite_completed": flowTriggerFields("github.check_suite_completed"), "cron": flowTriggerFields("cron")}, "gitlab": "gitlab.push, gitlab.merge_request_opened, gitlab.merge_request_merged, gitlab.merge_request_closed, gitlab.release_published, gitlab.pipeline_completed use matching GitHub fields", "actions": map[string]any{"start_agent_run": []string{"agent_id", "target_type", "target_id", "additional_context", "base_branch", "working_branch", "output"}, "move_to_state": []string{"target_state_id"}, "merge_branch": []string{"target_branch"}}}})
}

func (s *InternalCommandService) executePreviewFlow(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	chat, _, actor, err := s.flowBuilderCommandChat(ctx, meta)
	if err != nil {
		return nil, err
	}
	var result *model.FlowBuilderState
	err = s.dockChatRepo.WithTurnLock(ctx, chat.WorkspaceID, chat.ID, func() error {
		fresh, _, _, e := s.flowBuilderCommandChat(ctx, meta)
		if e != nil {
			return e
		}
		draft, e := decodeFlowDraft(input, fresh.FlowBuilder.Draft)
		if e != nil {
			return e
		}
		result, e = s.flowBuilderChat.previewFlow(ctx, fresh, actor, draft)
		return e
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"preview": result, "approval": map[string]any{"phase": "flow_confirm", "action": map[string]string{"revision": result.Revision}}})
}

func (s *InternalCommandService) executeCreateFlow(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ApprovalInteractionID string `json:"approval_interaction_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	chat, run, actor, err := s.flowBuilderCommandChat(ctx, meta)
	if err != nil {
		return nil, err
	}
	var result *model.AutomationRule
	err = s.dockChatRepo.WithTurnLock(ctx, chat.WorkspaceID, chat.ID, func() error {
		fresh, _, _, err := s.flowBuilderCommandChat(ctx, meta)
		if err != nil {
			return err
		}
		state := fresh.FlowBuilder
		if state.CreatedFlowID != "" {
			result, err = s.flowBuilderChat.flowBuilder.engine.ruleRepo.GetByID(ctx, chat.WorkspaceID, state.CreatedFlowID)
			return err
		}
		// Deterministic rule ID also closes the crash window between creation and
		// recording success on the conversation.
		existing, err := s.flowBuilderChat.flowBuilder.engine.ruleRepo.GetByID(ctx, chat.WorkspaceID, chat.ID)
		if err != nil {
			return err
		}
		if existing != nil {
			result = existing
			state.CreatedFlowID = existing.ID
			return s.flowBuilderChat.saveFlowBuilderState(ctx, fresh)
		}
		interaction, action, err := s.resolvedDockApprovalAction(ctx, meta, run, req.ApprovalInteractionID, "flow_confirm")
		if err != nil {
			return err
		}
		latest, err := s.dockChatRepo.LatestFlowBuilderUserMessage(ctx, chat.WorkspaceID, chat.ID)
		if err != nil {
			return err
		}
		if err := validateFlowApproval(state, chat.UserID, interaction, action, latest); err != nil {
			return err
		}
		catalog, err := s.flowBuilderChat.flowCatalog(ctx, fresh, actor)
		if err != nil {
			return err
		}
		d := *state.Draft
		if err := s.flowBuilderChat.validateFlowAgentTools(ctx, chat.WorkspaceID, d); err != nil {
			return err
		}
		engine := s.flowBuilderChat.flowBuilder.engine
		if engine.entitlementSvc != nil {
			if err := engine.entitlementSvc.RequireFeature(ctx, chat.WorkspaceID, EntitlementFeatureAutomationFlows); err != nil {
				return err
			}
			if d.TriggerType == model.TriggerCron {
				if err := engine.entitlementSvc.RequireFeature(ctx, chat.WorkspaceID, EntitlementFeatureAgentScheduling); err != nil {
					return err
				}
			}
		}
		if catalog.Template != nil {
			if state.TemplateVersion != catalog.Template.Version {
				return fmt.Errorf("this template changed; prepare and approve a new preview")
			}
			if err := s.flowBuilderChat.validateFlowTemplateReferences(ctx, chat.WorkspaceID, *catalog.Template, d.TemplateInputs, catalog); err != nil {
				return err
			}
		} else {
			if err := validateFlowBuilderCatalog(d, catalog); err != nil {
				return err
			}
		}
		if err := s.flowBuilderChat.validateFlowDraft(ctx, chat.WorkspaceID, d, state.SourceRule); err != nil {
			return err
		}
		if state.SourceRule != nil {
			current, err := engine.GetRule(ctx, chat.WorkspaceID, state.SourceRule.ID)
			if err != nil {
				return err
			}
			if current == nil {
				return fmt.Errorf("flow no longer exists")
			}
			if !current.UpdatedAt.Equal(state.SourceRule.UpdatedAt) {
				// A retry after the rule was saved but before the chat was marked
				// complete is safe only when the persisted rule matches the review.
				if !flowDraftMatchesRule(d, current) {
					return fmt.Errorf("this flow changed elsewhere; reopen it before making changes")
				}
				if err := engine.syncRuleSchedule(ctx, current, state.SourceRule.TriggerType == model.TriggerCron && state.SourceRule.Enabled); err != nil {
					return err
				}
				result = current
			} else {
				enabled := !d.Paused
				teamID, workflowID := derefString(d.TeamID), derefString(d.WorkflowID)
				result, err = engine.updateRule(ctx, chat.WorkspaceID, current.ID, model.UpdateAutomationRuleRequest{Name: &d.Name, Description: d.Description, TeamID: &teamID, WorkflowID: &workflowID, Enabled: &enabled, TriggerType: &d.TriggerType, TriggerConfig: &d.TriggerConfig, ActionType: &d.ActionType, ActionConfig: &d.ActionConfig}, &state.SourceRule.UpdatedAt)
				if err != nil {
					return err
				}
			}
		} else if catalog.Template != nil {
			install := flowTemplateInstallRequest(fresh, d)
			install.RuleID = chat.ID
			installed, err := s.flowBuilderChat.flowBuilder.installer.Install(ctx, install)
			if err != nil {
				return err
			}
			result = installed.Rule
		} else {
			result, err = engine.createRuleForActor(ctx, chat.WorkspaceID, meta.ActorID, model.CreateAutomationRuleRequest{WorkspaceID: chat.WorkspaceID, Name: d.Name, Description: d.Description, TeamID: d.TeamID, WorkflowID: d.WorkflowID, TriggerType: d.TriggerType, TriggerConfig: d.TriggerConfig, ActionType: d.ActionType, ActionConfig: d.ActionConfig}, chat.ID, !d.Paused)
			if err != nil {
				return err
			}
		}
		state.CreatedFlowID = result.ID
		return s.flowBuilderChat.saveFlowBuilderState(ctx, fresh)
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"flow": result, "saved": true})
}

func validateFlowApproval(state *model.FlowBuilderState, owner string, interaction *model.AgentRunInteraction, action json.RawMessage, latest string) error {
	if interaction.ResolvedBy == nil || *interaction.ResolvedBy != owner {
		return fmt.Errorf("the conversation owner must approve this flow")
	}
	var approved struct {
		Revision string `json:"revision"`
	}
	if json.Unmarshal(action, &approved) != nil || approved.Revision == "" || approved.Revision != state.Revision || state.Draft == nil {
		return fmt.Errorf("the flow changed; review and approve the latest preview")
	}
	if latest != state.UserMessageID {
		return fmt.Errorf("the request changed; prepare and approve a new preview")
	}
	return nil
}

func flowDraftMatchesRule(d model.FlowBuilderDraft, r *model.AutomationRule) bool {
	jsonEqual := func(a, b json.RawMessage) bool {
		var left, right any
		if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
			return false
		}
		return reflect.DeepEqual(left, right)
	}
	return d.Name == r.Name && (d.Description == nil || derefString(d.Description) == derefString(r.Description)) && d.Paused == !r.Enabled && derefString(d.TeamID) == derefString(r.TeamID) && derefString(d.WorkflowID) == derefString(r.WorkflowID) && d.TriggerType == r.TriggerType && d.ActionType == r.ActionType && jsonEqual(d.TriggerConfig, r.TriggerConfig) && jsonEqual(d.ActionConfig, r.ActionConfig)
}
