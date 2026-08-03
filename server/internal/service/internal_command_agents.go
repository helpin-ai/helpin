package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// dockApprovalPayloadKind is the request_approval payload kind the dock
// orchestrator must use before any mutating agents.* command. The payload's
// `action` object must equal the tool input (minus approval_interaction_id) —
// enforced server-side by canonical-hash comparison, not by prompt trust.
const dockApprovalPayloadKind = "dock_plan_confirm"

// SetAgentOrchestrationDependencies wires the collaborators used by the
// agents.* command tools: the command-bar dispatcher (plan validation +
// execution) and the interaction repository (approval verification).
func (s *InternalCommandService) SetAgentOrchestrationDependencies(commandBar *CommandBarService, interactionRepo *repository.AgentRunInteractionRepository) {
	s.commandBarService = commandBar
	s.agentRunInteractionRepo = interactionRepo
}

// dockLaunchTarget identifies the entity a launched child run acts on.
type dockLaunchTarget struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// dockLaunchStep is one child-run step requested by the dock orchestrator.
type dockLaunchStep struct {
	AgentID              string           `json:"agent_id,omitempty"`
	UseCommandAgent      bool             `json:"use_command_agent,omitempty"`
	Target               dockLaunchTarget `json:"target"`
	Instructions         string           `json:"instructions"`
	AllowedTools         []string         `json:"allowed_tools,omitempty"`
	DependsOnStepIndexes []int            `json:"depends_on_step_indexes,omitempty"`
}

func normalizeDockLaunchSteps(steps []dockLaunchStep) []dockLaunchStep {
	normalized := make([]dockLaunchStep, 0, len(steps))
	for _, step := range steps {
		step.AgentID = strings.TrimSpace(step.AgentID)
		step.Target.Type = strings.TrimSpace(step.Target.Type)
		step.Target.ID = strings.TrimSpace(step.Target.ID)
		step.Instructions = strings.TrimSpace(step.Instructions)
		tools := make([]string, 0, len(step.AllowedTools))
		for _, tool := range step.AllowedTools {
			if tool = strings.TrimSpace(tool); tool != "" {
				tools = append(tools, tool)
			}
		}
		sort.Strings(tools)
		step.AllowedTools = tools
		normalized = append(normalized, step)
	}
	return normalized
}

// dockActionHash canonicalizes a value by JSON-marshaling it (deterministic
// struct field order) and returns its sha256 hex digest.
func dockActionHash(value interface{}) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("canonicalize dock action: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// resolveDockChatRun resolves the calling run and requires it to be a dock
// chat's backing run — only the dock orchestrator may use the agents.* launch
// tools.
func (s *InternalCommandService) resolveDockChatRun(ctx context.Context, meta model.InternalCommandContext) (*model.AgentRun, error) {
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	if run == nil || run.DockChatID == nil || strings.TrimSpace(*run.DockChatID) == "" {
		return nil, fmt.Errorf("agent launch tools are only available to dock chat runs")
	}
	return run, nil
}

// verifyDockApproval checks that the given interaction on the chat run is a
// resolved-approved dock_plan_confirm whose action content matches actionHash,
// and that it has not been consumed by an earlier launch.
func (s *InternalCommandService) verifyDockApproval(ctx context.Context, meta model.InternalCommandContext, chatRun *model.AgentRun, interactionID, actionType, actionHash string) (*model.AgentRunInteraction, error) {
	if s.agentRunInteractionRepo == nil {
		return nil, fmt.Errorf("interaction repository is not configured")
	}
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil, fmt.Errorf("approval_interaction_id is required: call request_approval with a %q payload first", dockApprovalPayloadKind)
	}
	interaction, err := s.agentRunInteractionRepo.GetByID(ctx, meta.WorkspaceID, chatRun.ID, interactionID)
	if err != nil {
		return nil, err
	}
	if interaction == nil {
		return nil, fmt.Errorf("approval interaction not found on this chat run")
	}
	if interaction.InteractionKind != model.AgentRunInteractionKindApprovalRequest {
		return nil, fmt.Errorf("interaction %q is not an approval request", interactionID)
	}
	if interaction.Status != model.AgentRunInteractionStatusResolved {
		return nil, fmt.Errorf("approval interaction is not resolved yet")
	}
	var response struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(interaction.ResponsePayload, &response); err != nil || strings.TrimSpace(response.Decision) != "approve" {
		return nil, fmt.Errorf("the user did not approve this action")
	}
	var request struct {
		Kind   string          `json:"kind"`
		Action json.RawMessage `json:"action"`
	}
	if err := json.Unmarshal(interaction.RequestPayload, &request); err != nil || strings.TrimSpace(request.Kind) != dockApprovalPayloadKind {
		return nil, fmt.Errorf("approval payload must have kind %q with the proposed action", dockApprovalPayloadKind)
	}
	approvedHash, err := dockApprovedActionHash(actionType, request.Action)
	if err != nil {
		return nil, err
	}
	if approvedHash != actionHash {
		return nil, fmt.Errorf("approved action does not match this call: request approval for exactly the parameters you pass to the tool")
	}
	var runtimeMetadata map[string]interface{}
	_ = json.Unmarshal(interaction.RuntimeMetadata, &runtimeMetadata)
	if runtimeMetadata != nil {
		if consumed, ok := runtimeMetadata["dock_action_consumed"].(bool); ok && consumed {
			return nil, fmt.Errorf("this approval was already used; request a new approval")
		}
	}
	return interaction, nil
}

// dockApprovedActionHash re-canonicalizes the approval payload's action object
// through the same typed structs used for the tool input, so cosmetic JSON
// differences (field order, whitespace) do not break matching.
func dockApprovedActionHash(actionType string, action json.RawMessage) (string, error) {
	switch actionType {
	case "launch":
		var payload struct {
			Steps []dockLaunchStep `json:"steps"`
		}
		if err := json.Unmarshal(action, &payload); err != nil || len(payload.Steps) == 0 {
			// Single-step approvals may inline the step fields directly.
			var step dockLaunchStep
			if err := json.Unmarshal(action, &step); err != nil {
				return "", fmt.Errorf("approval action does not contain launch steps")
			}
			payload.Steps = []dockLaunchStep{step}
		}
		return dockActionHash(normalizeDockLaunchSteps(payload.Steps))
	case "create_agent":
		var payload dockCreateAgentAction
		if err := json.Unmarshal(action, &payload); err != nil {
			return "", fmt.Errorf("approval action does not contain agent draft fields")
		}
		return dockActionHash(payload.normalized())
	case "promote_run":
		var payload dockPromoteRunAction
		if err := json.Unmarshal(action, &payload); err != nil {
			return "", fmt.Errorf("approval action does not contain promotion fields")
		}
		return dockActionHash(payload.normalized())
	case "epic_pipeline":
		var payload dockEpicPipelineAction
		if err := json.Unmarshal(action, &payload); err != nil {
			return "", fmt.Errorf("approval action does not contain the epic id")
		}
		return dockActionHash(payload.normalized())
	default:
		return "", fmt.Errorf("unsupported dock action type %q", actionType)
	}
}

// dockEpicPipelineAction is the canonical epic-pipeline action content.
type dockEpicPipelineAction struct {
	EpicID string `json:"epic_id"`
}

func (a dockEpicPipelineAction) normalized() dockEpicPipelineAction {
	return dockEpicPipelineAction{EpicID: strings.TrimSpace(a.EpicID)}
}

func (s *InternalCommandService) consumeDockApproval(ctx context.Context, interaction *model.AgentRunInteraction, resultID string) {
	if interaction == nil || s.agentRunInteractionRepo == nil {
		return
	}
	runtimeMetadata := map[string]interface{}{}
	_ = json.Unmarshal(interaction.RuntimeMetadata, &runtimeMetadata)
	runtimeMetadata["dock_action_consumed"] = true
	runtimeMetadata["dock_action_result_id"] = resultID
	if encoded, err := json.Marshal(runtimeMetadata); err == nil {
		interaction.RuntimeMetadata = encoded
		_ = s.agentRunInteractionRepo.Update(ctx, interaction)
	}
}

// executeDockLaunch validates approval and dispatches the steps as a
// command-bar plan linked to the calling dock chat run.
func (s *InternalCommandService) executeDockLaunch(ctx context.Context, meta model.InternalCommandContext, steps []dockLaunchStep, approvalInteractionID, prompt string) (json.RawMessage, error) {
	if s.commandBarService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	if s.agentService == nil {
		return nil, fmt.Errorf("agent service is not configured")
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("at least one step is required")
	}
	chatRun, err := s.resolveDockChatRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	steps = normalizeDockLaunchSteps(steps)
	actionHash, err := dockActionHash(steps)
	if err != nil {
		return nil, err
	}
	interaction, err := s.verifyDockApproval(ctx, meta, chatRun, approvalInteractionID, "launch", actionHash)
	if err != nil {
		return nil, err
	}

	planSteps := make([]model.CommandBarPlanStep, 0, len(steps))
	for i, step := range steps {
		if step.Instructions == "" {
			return nil, fmt.Errorf("step %d requires instructions", i+1)
		}
		agentID := step.AgentID
		agentName := ""
		planKind := ""
		if step.UseCommandAgent {
			if len(step.AllowedTools) == 0 {
				return nil, fmt.Errorf("step %d: one-shot command agent steps require allowed_tools", i+1)
			}
			commandAgent, err := s.agentService.ensureBuiltInAgent(ctx, meta.WorkspaceID, meta.ActorID, model.AgentPresetCommandAgent)
			if err != nil {
				return nil, fmt.Errorf("resolve command agent: %w", err)
			}
			agentID = commandAgent.ID
			agentName = commandAgent.Name
			planKind = model.CommandBarPlanKindOneShotCommand
		} else {
			if agentID == "" {
				return nil, fmt.Errorf("step %d requires agent_id (or use_command_agent)", i+1)
			}
			agent, err := s.agentService.GetAgent(ctx, meta.WorkspaceID, agentID)
			if err != nil {
				return nil, fmt.Errorf("step %d: %w", i+1, err)
			}
			agentName = agent.Name
		}
		targetType := step.Target.Type
		targetID := step.Target.ID
		if targetType == "" {
			targetType = "workspace"
			targetID = meta.WorkspaceID
		}
		planSteps = append(planSteps, model.CommandBarPlanStep{
			AgentID:              agentID,
			AgentName:            agentName,
			PlanKind:             planKind,
			Target:               model.CommandBarPageContext{EntityType: targetType, EntityID: targetID},
			Instructions:         step.Instructions,
			AllowedTools:         step.AllowedTools,
			DependsOnStepIndexes: step.DependsOnStepIndexes,
		})
	}

	text := strings.TrimSpace(prompt)
	if text == "" {
		text = planSteps[0].Instructions
	}
	resp, err := s.commandBarService.dispatchPlanCore(ctx, meta.WorkspaceID, meta.ActorID, model.CommandBarDispatchRequest{
		Text:        text,
		PageContext: model.CommandBarPageContext{EntityType: "workspace", EntityID: meta.WorkspaceID},
		Steps:       planSteps,
	}, dispatchPlanParams{parentChatRunID: &chatRun.ID, dockChatID: chatRun.DockChatID})
	if err != nil {
		return nil, err
	}
	s.consumeDockApproval(ctx, interaction, resp.PlanID)

	runIDs := make([]string, 0, len(resp.Runs))
	for _, run := range resp.Runs {
		runIDs = append(runIDs, run.ID)
	}
	return mustJSON(map[string]interface{}{
		"plan_id":  resp.PlanID,
		"status":   "started",
		"run_ids":  runIDs,
		"message":  "Runs started. The result will be delivered into this chat when they finish — tell the user and end your turn.",
		"step_ids": len(planSteps),
	}), nil
}

// dockCreateAgentAction is the canonical create_agent action content.
type dockCreateAgentAction struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description"`
}

func (a dockCreateAgentAction) normalized() dockCreateAgentAction {
	return dockCreateAgentAction{
		Name:        strings.TrimSpace(a.Name),
		Description: strings.TrimSpace(a.Description),
	}
}

// dockPromoteRunAction is the canonical promote_run action content.
type dockPromoteRunAction struct {
	RunID          string   `json:"run_id"`
	Name           string   `json:"name"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	AllowedTargets []string `json:"allowed_targets,omitempty"`
}

func (a dockPromoteRunAction) normalized() dockPromoteRunAction {
	tools := make([]string, 0, len(a.AllowedTools))
	for _, tool := range a.AllowedTools {
		if tool = strings.TrimSpace(tool); tool != "" {
			tools = append(tools, tool)
		}
	}
	sort.Strings(tools)
	targets := make([]string, 0, len(a.AllowedTargets))
	for _, target := range a.AllowedTargets {
		if target = strings.TrimSpace(target); target != "" {
			targets = append(targets, target)
		}
	}
	sort.Strings(targets)
	return dockPromoteRunAction{
		RunID:          strings.TrimSpace(a.RunID),
		Name:           strings.TrimSpace(a.Name),
		AllowedTools:   tools,
		AllowedTargets: targets,
	}
}

func dockLaunchStepSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id":          map[string]any{"type": "string", "description": "ID of the saved agent to run (from list_agents). Omit when use_command_agent is true."},
			"use_command_agent": map[string]any{"type": "boolean", "description": "Run the one-shot Command Agent instead of a saved agent. Requires allowed_tools."},
			"target": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{"type": "string", "description": "Target entity type: workspace, task, epic, document, crm_deal, crm_contact, repository, support_conversation."},
					"id":   map[string]any{"type": "string", "description": "Target entity ID. Defaults to the workspace when omitted."},
				},
			},
			"instructions":  map[string]any{"type": "string", "description": "What the child run should do."},
			"allowed_tools": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Narrowed tool list for the child run (required for use_command_agent)."},
			"depends_on_step_indexes": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Zero-based indexes of steps that must finish first (DAG plans)."},
		},
		"required":             []string{"instructions"},
		"additionalProperties": false,
	}
}

// registerAgentOrchestrationCommands registers the agents.* tools the dock
// orchestrator uses to run other agents, create ad-hoc agents, and get
// results back into the chat. All mutating tools enforce the
// dock_plan_confirm approval contract server-side.
func (s *InternalCommandService) registerAgentOrchestrationCommands() {
	launchTargets := []string{"workspace"}

	s.register(InternalCommandDefinition{
		Name:                 "agents.start_run",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.start_run",
			Alias:       "start_agent_run",
			Category:    "Agents",
			Description: "Start one child agent run (a saved agent by id, or the one-shot Command Agent with narrowed tools). Requires a resolved dock_plan_confirm approval whose action matches this call exactly. The result is delivered back into this chat when the run finishes.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id":                dockLaunchStepSchema()["properties"].(map[string]any)["agent_id"],
					"use_command_agent":       dockLaunchStepSchema()["properties"].(map[string]any)["use_command_agent"],
					"target":                  dockLaunchStepSchema()["properties"].(map[string]any)["target"],
					"instructions":            dockLaunchStepSchema()["properties"].(map[string]any)["instructions"],
					"allowed_tools":           dockLaunchStepSchema()["properties"].(map[string]any)["allowed_tools"],
					"approval_interaction_id": map[string]any{"type": "string", "description": "ID of the resolved dock_plan_confirm approval interaction."},
				},
				"required":             []string{"instructions", "approval_interaction_id"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				dockLaunchStep
				ApprovalInteractionID string `json:"approval_interaction_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse start run input: %w", err)
			}
			return s.executeDockLaunch(ctx, meta, []dockLaunchStep{req.dockLaunchStep}, req.ApprovalInteractionID, req.Instructions)
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "agents.start_plan",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.start_plan",
			Alias:       "start_agent_plan",
			Category:    "Agents",
			Description: "Start a multi-step plan of child agent runs (fan-out or dependency-ordered DAG via depends_on_step_indexes). Requires a resolved dock_plan_confirm approval whose action matches this call exactly. Results are delivered back into this chat when the plan settles.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":                  map[string]any{"type": "string", "description": "Short description of the overall plan (shown in run surfaces)."},
					"steps":                  map[string]any{"type": "array", "items": dockLaunchStepSchema(), "description": "Plan steps in order."},
					"approval_interaction_id": map[string]any{"type": "string", "description": "ID of the resolved dock_plan_confirm approval interaction."},
				},
				"required":             []string{"steps", "approval_interaction_id"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				Prompt                string           `json:"prompt"`
				Steps                 []dockLaunchStep `json:"steps"`
				ApprovalInteractionID string           `json:"approval_interaction_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse start plan input: %w", err)
			}
			return s.executeDockLaunch(ctx, meta, req.Steps, req.ApprovalInteractionID, req.Prompt)
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "agents.get_run",
		Module:               "agents",
		Mutating:             false,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.get_run",
			Alias:       "get_agent_run",
			Category:    "Agents",
			Description: "Get the status of a child agent run or plan started from this chat. Use only when the user explicitly asks about progress — results arrive in this chat automatically.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"run_id":  map[string]any{"type": "string"},
					"plan_id": map[string]any{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				RunID  string `json:"run_id"`
				PlanID string `json:"plan_id"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse get run input: %w", err)
				}
			}
			if planID := strings.TrimSpace(req.PlanID); planID != "" {
				if s.commandBarService == nil {
					return nil, fmt.Errorf("command bar service is not configured")
				}
				detail, err := s.commandBarService.GetPlan(ctx, meta.WorkspaceID, meta.ActorID, planID)
				if err != nil {
					return nil, err
				}
				return mustJSON(detail), nil
			}
			runID := strings.TrimSpace(req.RunID)
			if runID == "" {
				return nil, fmt.Errorf("run_id or plan_id is required")
			}
			if s.agentRunRepo == nil {
				return nil, fmt.Errorf("agent run repository is not configured")
			}
			run, err := s.agentRunRepo.GetByID(ctx, meta.WorkspaceID, runID)
			if err != nil {
				return nil, err
			}
			if run == nil {
				return nil, fmt.Errorf("run not found")
			}
			return mustJSON(map[string]interface{}{
				"run_id":       run.ID,
				"status":       run.Status,
				"pause_reason": run.PauseReason,
				"agent_id":     run.AgentID,
				"target_type":  run.TargetType,
				"target_id":    run.TargetID,
				"error":        derefString(run.ErrorMessage),
			}), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "agents.cancel_run",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.cancel_run",
			Alias:       "cancel_agent_run",
			Category:    "Agents",
			Description: "Cancel a child agent run or plan started from this chat. No approval needed — cancelling stops work.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"run_id":  map[string]any{"type": "string"},
					"plan_id": map[string]any{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				RunID  string `json:"run_id"`
				PlanID string `json:"plan_id"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse cancel input: %w", err)
				}
			}
			if _, err := s.resolveDockChatRun(ctx, meta); err != nil {
				return nil, err
			}
			if planID := strings.TrimSpace(req.PlanID); planID != "" {
				if s.commandBarService == nil {
					return nil, fmt.Errorf("command bar service is not configured")
				}
				if _, err := s.commandBarService.CancelPlan(ctx, meta.WorkspaceID, meta.ActorID, planID); err != nil {
					return nil, err
				}
				return mustJSON(map[string]string{"plan_id": planID, "status": "cancelled"}), nil
			}
			runID := strings.TrimSpace(req.RunID)
			if runID == "" {
				return nil, fmt.Errorf("run_id or plan_id is required")
			}
			if s.agentService == nil {
				return nil, fmt.Errorf("agent service is not configured")
			}
			if _, err := s.agentService.CancelRun(ctx, meta.WorkspaceID, runID, meta.ActorID); err != nil {
				return nil, err
			}
			return mustJSON(map[string]string{"run_id": runID, "status": "cancelled"}), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "agents.create_agent",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.create_agent",
			Alias:       "create_custom_agent",
			Category:    "Agents",
			Description: "Create a reusable custom agent from a description (drafted server-side). Requires a resolved dock_plan_confirm approval whose action matches this call exactly.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":                    map[string]any{"type": "string", "description": "Optional name override for the new agent."},
					"description":             map[string]any{"type": "string", "description": "What the agent should do; used to draft its prompt, tools, and targets."},
					"approval_interaction_id": map[string]any{"type": "string"},
				},
				"required":             []string{"description", "approval_interaction_id"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				dockCreateAgentAction
				ApprovalInteractionID string `json:"approval_interaction_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse create agent input: %w", err)
			}
			if s.agentService == nil {
				return nil, fmt.Errorf("agent service is not configured")
			}
			chatRun, err := s.resolveDockChatRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			action := req.dockCreateAgentAction.normalized()
			if action.Description == "" {
				return nil, fmt.Errorf("description is required")
			}
			actionHash, err := dockActionHash(action)
			if err != nil {
				return nil, err
			}
			interaction, err := s.verifyDockApproval(ctx, meta, chatRun, req.ApprovalInteractionID, "create_agent", actionHash)
			if err != nil {
				return nil, err
			}
			draftResp, err := s.agentService.DraftCustomAgent(ctx, meta.WorkspaceID, model.CustomAgentDraftRequest{Description: action.Description})
			if err != nil {
				return nil, fmt.Errorf("draft agent: %w", err)
			}
			createReq := commandBarCreateAgentRequestFromDraft(meta.WorkspaceID, draftResp.Draft, model.ConfirmCommandBarChatProposalRequest{
				Name: stringPtrIfNotEmpty(action.Name),
			})
			agent, err := s.agentService.CreateAgent(ctx, createReq, meta.ActorID)
			if err != nil {
				return nil, fmt.Errorf("create agent: %w", err)
			}
			s.consumeDockApproval(ctx, interaction, agent.ID)
			return mustJSON(map[string]interface{}{
				"agent_id":        agent.ID,
				"name":            agent.Name,
				"allowed_tools":   parseJSONStringSlice(agent.AllowedTools),
				"allowed_targets": parseJSONStringSlice(agent.AllowedTargets),
				"message":         "Agent created. You can now start it with start_agent_run (that launch needs its own approval).",
			}), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "epic.run_delivery_pipeline",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "epic.run_delivery_pipeline",
			Alias:       "run_epic_delivery_pipeline",
			Category:    "Agents",
			Description: "Run the epic delivery pipeline: implement (Forge), review (Lens), and merge every open task of an epic on its integration branch, ordered by blocking links, then open the epic PR. From a dock chat this requires a dock_plan_confirm approval whose action is {\"epic_id\": ...}; epic-target agent runs may call it directly for their own epic.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"epic_id":                 map[string]any{"type": "string", "description": "Epic to deliver. Defaults to the run's target when the run targets an epic."},
					"approval_interaction_id": map[string]any{"type": "string", "description": "Required when called from a dock chat."},
				},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				EpicID                string `json:"epic_id"`
				ApprovalInteractionID string `json:"approval_interaction_id"`
			}
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse epic pipeline input: %w", err)
				}
			}
			if s.commandBarService == nil {
				return nil, fmt.Errorf("command bar service is not configured")
			}
			run, err := s.resolveCommandRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			if run == nil {
				return nil, fmt.Errorf("epic pipeline requires a run context")
			}
			epicID := strings.TrimSpace(req.EpicID)
			params := dispatchPlanParams{}
			switch {
			case run.DockChatID != nil && strings.TrimSpace(*run.DockChatID) != "":
				if epicID == "" {
					return nil, fmt.Errorf("epic_id is required")
				}
				action := dockEpicPipelineAction{EpicID: epicID}.normalized()
				actionHash, hashErr := dockActionHash(action)
				if hashErr != nil {
					return nil, hashErr
				}
				interaction, approvalErr := s.verifyDockApproval(ctx, meta, run, req.ApprovalInteractionID, "epic_pipeline", actionHash)
				if approvalErr != nil {
					return nil, approvalErr
				}
				params = dispatchPlanParams{parentChatRunID: &run.ID, dockChatID: run.DockChatID}
				resp, startErr := s.commandBarService.StartEpicDeliveryPipeline(ctx, meta.WorkspaceID, meta.ActorID, epicID, params)
				if startErr != nil {
					return nil, startErr
				}
				s.consumeDockApproval(ctx, interaction, resp.PlanID)
				return mustJSON(resp), nil
			case strings.TrimSpace(run.TargetType) == "epic":
				if epicID == "" {
					epicID = strings.TrimSpace(run.TargetID)
				}
				if epicID != strings.TrimSpace(run.TargetID) {
					return nil, fmt.Errorf("epic-target runs may only deliver their own epic")
				}
			default:
				return nil, fmt.Errorf("epic pipeline can only be started from a dock chat (with approval) or an epic-target run")
			}
			resp, err := s.commandBarService.StartEpicDeliveryPipeline(ctx, meta.WorkspaceID, meta.ActorID, epicID, params)
			if err != nil {
				return nil, err
			}
			return mustJSON(resp), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "agents.promote_run",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.promote_run",
			Alias:       "promote_run_to_agent",
			Category:    "Agents",
			Description: "Promote a finished one-shot child run into a reusable saved agent. Requires a resolved dock_plan_confirm approval whose action matches this call exactly.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"run_id":                  map[string]any{"type": "string"},
					"name":                    map[string]any{"type": "string"},
					"allowed_tools":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"allowed_targets":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"approval_interaction_id": map[string]any{"type": "string"},
				},
				"required":             []string{"run_id", "name", "approval_interaction_id"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				dockPromoteRunAction
				ApprovalInteractionID string `json:"approval_interaction_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse promote run input: %w", err)
			}
			if s.commandBarService == nil {
				return nil, fmt.Errorf("command bar service is not configured")
			}
			chatRun, err := s.resolveDockChatRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			action := req.dockPromoteRunAction.normalized()
			actionHash, err := dockActionHash(action)
			if err != nil {
				return nil, err
			}
			interaction, err := s.verifyDockApproval(ctx, meta, chatRun, req.ApprovalInteractionID, "promote_run", actionHash)
			if err != nil {
				return nil, err
			}
			resp, err := s.commandBarService.PromoteRunToAgent(ctx, meta.WorkspaceID, meta.ActorID, action.RunID, model.PromoteCommandBarRunRequest{
				Name:           action.Name,
				AllowedTools:   action.AllowedTools,
				AllowedTargets: action.AllowedTargets,
			})
			if err != nil {
				return nil, err
			}
			s.consumeDockApproval(ctx, interaction, resp.Agent.ID)
			return mustJSON(map[string]interface{}{
				"agent_id": resp.Agent.ID,
				"name":     resp.Agent.Name,
			}), nil
		},
	})
}
