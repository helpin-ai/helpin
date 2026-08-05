package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// dockApprovalPayloadKind is the request_approval payload kind the dock
// orchestrator must use before any mutating agents.* command. Launch and agent
// draft calls resolve their immutable action by approval interaction ID;
// legacy dedicated actions retain canonical-hash validation.
const dockApprovalPayloadKind = "dock_plan_confirm"

// supportApprovalPayloadKind is the request_approval payload kind a support
// chat run must use for child launches that are not strictly read-only; the
// approval is resolved by a teammate from the inbox. Read-only launches
// (allowed_tools ⊆ supportChildReadOnlyTools) auto-approve.
const supportApprovalPayloadKind = "support_plan_confirm"

// orchestratorRunKind identifies which orchestration surface a calling run
// belongs to.
type orchestratorRunKind int

const (
	orchestratorRunDock orchestratorRunKind = iota + 1
	orchestratorRunSupport
)

type dockGetRunRequest struct {
	RunID        string `json:"run_id"`
	PlanID       string `json:"plan_id"`
	DetailLevel  string `json:"detail_level"`
	ResultOffset int    `json:"result_offset"`
	ResultLimit  int    `json:"result_limit"`
}

type dockGetRunResponse struct {
	RunID           string                     `json:"run_id"`
	Status          string                     `json:"status"`
	PauseReason     string                     `json:"pause_reason"`
	AgentID         string                     `json:"agent_id"`
	TargetType      string                     `json:"target_type"`
	TargetID        string                     `json:"target_id"`
	Error           string                     `json:"error,omitempty"`
	ResultAvailable bool                       `json:"result_available"`
	Result          *dockRunResultExcerpt      `json:"result,omitempty"`
	Artifacts       []dockRunArtifactReference `json:"artifacts,omitempty"`
}

// SetAgentOrchestrationDependencies wires the collaborators used by the
// agents.* command tools: the command-bar dispatcher (plan validation +
// execution) and the interaction repository (approval verification).
func (s *InternalCommandService) SetAgentOrchestrationDependencies(commandBar *CommandBarService, interactionRepo *repository.AgentRunInteractionRepository) {
	s.commandBarService = commandBar
	s.agentRunInteractionRepo = interactionRepo
}

// SetDockActionProposalRepository wires durable, scoped Dock execution grants.
func (s *InternalCommandService) SetDockActionProposalRepository(repo *repository.DockActionProposalRepository) {
	s.dockActionProposalRepo = repo
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
		// Workspace targets always resolve to the run's workspace; models
		// sometimes stuff a display name into target.id, so drop it from the
		// canonical form (dispatch supplies the real workspace id).
		if step.Target.Type == "" || step.Target.Type == "workspace" {
			step.Target.ID = ""
		}
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

// resolveOrchestratorRun resolves the calling run and requires it to be an
// orchestrator — a dock chat's backing run, or a support conversation's chat
// run. Only orchestrators may use the agents.* launch tools.
func (s *InternalCommandService) resolveOrchestratorRun(ctx context.Context, meta model.InternalCommandContext) (*model.AgentRun, orchestratorRunKind, error) {
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, 0, err
	}
	switch {
	case run == nil:
		return nil, 0, fmt.Errorf("agent launch tools are only available to dock chat or support chat runs")
	case run.DockChatID != nil && strings.TrimSpace(*run.DockChatID) != "":
		return run, orchestratorRunDock, nil
	case strings.TrimSpace(run.TargetType) == "support_conversation" && runInputTriggerType(run) == supportChatTriggerType:
		return run, orchestratorRunSupport, nil
	default:
		return nil, 0, fmt.Errorf("agent launch tools are only available to dock chat or support chat runs")
	}
}

// resolveDockChatRun resolves the calling run and requires it to be a dock
// chat's backing run — for tools that stay dock-only (create/promote agents).
func (s *InternalCommandService) resolveDockChatRun(ctx context.Context, meta model.InternalCommandContext) (*model.AgentRun, error) {
	run, kind, err := s.resolveOrchestratorRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	if kind != orchestratorRunDock {
		return nil, fmt.Errorf("this tool is only available to dock chat runs")
	}
	return run, nil
}

func (s *InternalCommandService) orchestratorPlanOwningRun(ctx context.Context, chatRun *model.AgentRun, kind orchestratorRunKind, runID string) (*model.CommandBarPlanRecord, error) {
	if s.commandBarService == nil || s.commandBarService.planRepo == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	var plan *model.CommandBarPlanRecord
	var err error
	switch {
	case kind == orchestratorRunDock && chatRun != nil && chatRun.DockChatID != nil:
		plan, err = s.commandBarService.planRepo.FindByDockChatAndRunID(ctx, chatRun.WorkspaceID, *chatRun.DockChatID, runID)
	case kind == orchestratorRunSupport && chatRun != nil:
		plan, err = s.commandBarService.planRepo.FindBySupportConversationAndRunID(ctx, chatRun.WorkspaceID, chatRun.TargetID, runID)
	default:
		return nil, fmt.Errorf("get_agent_run requires an orchestrator run")
	}
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("run_id was not launched from this chat")
	}
	return plan, nil
}

// orchestratorOwnsPlan reports whether a plan was launched from the calling
// orchestrator run's chat or conversation.
func orchestratorOwnsPlan(chatRun *model.AgentRun, kind orchestratorRunKind, plan *model.CommandBarPlanRecord) bool {
	if chatRun == nil || plan == nil {
		return false
	}
	switch kind {
	case orchestratorRunDock:
		return plan.DockChatID != nil && chatRun.DockChatID != nil &&
			strings.TrimSpace(*plan.DockChatID) == strings.TrimSpace(*chatRun.DockChatID)
	case orchestratorRunSupport:
		return plan.SupportConversationID != nil &&
			strings.TrimSpace(*plan.SupportConversationID) == strings.TrimSpace(chatRun.TargetID)
	default:
		return false
	}
}

// checkSupportChildCaps enforces the per-conversation child-plan limits for
// support-launched plans.
func (s *InternalCommandService) checkSupportChildCaps(ctx context.Context, workspaceID, conversationID string) error {
	if s.commandBarService == nil || s.commandBarService.planRepo == nil {
		return fmt.Errorf("command bar service is not configured")
	}
	active, err := s.commandBarService.planRepo.CountPlansForSupportConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		return err
	}
	if active >= supportChildMaxConcurrent {
		return fmt.Errorf("this conversation already has %d sub-agent plans running; wait for their results before launching more", active)
	}
	total, err := s.commandBarService.planRepo.CountPlansForSupportConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		return err
	}
	if total >= supportChildMaxPerConversation {
		return fmt.Errorf("this conversation reached its child-plan limit (%d); answer with what you have or escalate to a human", supportChildMaxPerConversation)
	}
	return nil
}

func normalizeDockGetRunRequest(req dockGetRunRequest) (dockGetRunRequest, error) {
	req.RunID = strings.TrimSpace(req.RunID)
	req.PlanID = strings.TrimSpace(req.PlanID)
	req.DetailLevel = strings.TrimSpace(req.DetailLevel)
	if req.DetailLevel == "" {
		req.DetailLevel = "status"
	}
	if req.RunID == "" && req.PlanID == "" {
		return req, fmt.Errorf("run_id or plan_id is required")
	}
	if req.RunID != "" && req.PlanID != "" {
		return req, fmt.Errorf("run_id and plan_id are mutually exclusive")
	}
	if req.DetailLevel != "status" && req.DetailLevel != "result" {
		return req, fmt.Errorf("detail_level must be status or result")
	}
	if req.DetailLevel == "result" && req.RunID == "" {
		return req, fmt.Errorf("detail_level=result requires run_id")
	}
	if req.DetailLevel == "status" && (req.ResultOffset != 0 || req.ResultLimit != 0) {
		return req, fmt.Errorf("result_offset and result_limit require detail_level=result")
	}
	if req.ResultOffset < 0 {
		return req, fmt.Errorf("result_offset must be zero or greater")
	}
	if req.ResultLimit < 0 || req.ResultLimit > dockRunResultMaxChars {
		return req, fmt.Errorf("result_limit must be between 1 and %d", dockRunResultMaxChars)
	}
	if req.DetailLevel == "result" && req.ResultLimit == 0 {
		req.ResultLimit = dockRunResultDefaultChars
	}
	return req, nil
}

// verifyDockApproval checks that the given interaction on the chat run is a
// resolved-approved confirmation of the expected payload kind whose action
// content matches actionHash, and that it has not been consumed by an earlier
// launch.
func (s *InternalCommandService) verifyDockApproval(ctx context.Context, meta model.InternalCommandContext, chatRun *model.AgentRun, interactionID, actionType, actionHash, payloadKind string) (*model.AgentRunInteraction, error) {
	interaction, approvedAction, err := s.resolvedDockApprovalAction(ctx, meta, chatRun, interactionID, payloadKind)
	if err != nil {
		return nil, err
	}
	approvedHash, err := dockApprovedActionHash(actionType, approvedAction)
	if err != nil {
		return nil, err
	}
	if approvedHash != actionHash {
		return nil, fmt.Errorf("approved action does not match this call: launch with only approval_interaction_id or pass exactly the approved parameters")
	}
	return interaction, nil
}

// resolvedDockApprovalAction returns the immutable action stored on one
// approved, unconsumed interaction. Callers use this for ID-only execution so
// the model never needs to reconstruct long instructions after approval.
func (s *InternalCommandService) resolvedDockApprovalAction(ctx context.Context, meta model.InternalCommandContext, chatRun *model.AgentRun, interactionID, payloadKind string) (*model.AgentRunInteraction, json.RawMessage, error) {
	if s.agentRunInteractionRepo == nil {
		return nil, nil, fmt.Errorf("interaction repository is not configured")
	}
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil, nil, fmt.Errorf("approval_interaction_id is required: call request_approval with a %q payload first", payloadKind)
	}
	interaction, err := s.agentRunInteractionRepo.GetByID(ctx, meta.WorkspaceID, chatRun.ID, interactionID)
	if err != nil {
		return nil, nil, err
	}
	if interaction == nil {
		// The agent knows the runtime-side interaction id; the projected row
		// has its own id and records the runtime id in runtime_metadata.
		interactions, listErr := s.agentRunInteractionRepo.ListByRun(ctx, meta.WorkspaceID, chatRun.ID)
		if listErr != nil {
			return nil, nil, listErr
		}
		for index := range interactions {
			if agentRunInteractionHasRuntimeInteractionID(interactions[index], interactionID) {
				interaction = &interactions[index]
				break
			}
		}
	}
	if interaction == nil {
		return nil, nil, fmt.Errorf("approval interaction not found on this chat run")
	}
	if interaction.InteractionKind != model.AgentRunInteractionKindApprovalRequest {
		return nil, nil, fmt.Errorf("interaction %q is not an approval request", interactionID)
	}
	if interaction.Status != model.AgentRunInteractionStatusResolved {
		return nil, nil, fmt.Errorf("approval interaction is not resolved yet")
	}
	var response struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(interaction.ResponsePayload, &response); err != nil || strings.TrimSpace(response.Decision) != "approve" {
		return nil, nil, fmt.Errorf("the user did not approve this action")
	}
	// The runtime's request_approval tool carries the dock contract as
	// phase="dock_plan_confirm" plus a structured `action` object (also
	// mirrored under raw_input). Accept a top-level `kind` for parity.
	var request struct {
		Kind     string          `json:"kind"`
		Phase    string          `json:"phase"`
		Action   json.RawMessage `json:"action"`
		RawInput struct {
			Action json.RawMessage `json:"action"`
		} `json:"raw_input"`
	}
	if err := json.Unmarshal(interaction.RequestPayload, &request); err != nil {
		return nil, nil, fmt.Errorf("approval payload is not valid JSON")
	}
	kind := strings.TrimSpace(firstNonEmptyString(request.Kind, request.Phase))
	if kind != payloadKind {
		return nil, nil, fmt.Errorf("approval must use phase %q with the proposed action", payloadKind)
	}
	approvedAction := request.Action
	if len(approvedAction) == 0 {
		approvedAction = request.RawInput.Action
	}
	if len(approvedAction) == 0 {
		return nil, nil, fmt.Errorf("approval is missing the structured action: call request_approval with phase %q and an action object", payloadKind)
	}
	var runtimeMetadata map[string]interface{}
	_ = json.Unmarshal(interaction.RuntimeMetadata, &runtimeMetadata)
	if runtimeMetadata != nil {
		if consumed, ok := runtimeMetadata["dock_action_consumed"].(bool); ok && consumed {
			return nil, nil, fmt.Errorf("this approval was already used; request a new approval")
		}
	}
	return interaction, approvedAction, nil
}

func (s *InternalCommandService) approvedDockLaunchSteps(ctx context.Context, meta model.InternalCommandContext, approvalInteractionID string) ([]dockLaunchStep, string, error) {
	chatRun, kind, err := s.resolveOrchestratorRun(ctx, meta)
	if err != nil {
		return nil, "", err
	}
	payloadKind := dockApprovalPayloadKind
	if kind == orchestratorRunSupport {
		payloadKind = supportApprovalPayloadKind
	}
	_, action, err := s.resolvedDockApprovalAction(ctx, meta, chatRun, approvalInteractionID, payloadKind)
	if err != nil {
		return nil, "", err
	}
	var payload struct {
		Prompt string           `json:"prompt"`
		Steps  []dockLaunchStep `json:"steps"`
	}
	if err := json.Unmarshal(action, &payload); err != nil {
		return nil, "", fmt.Errorf("approval action does not contain launch steps")
	}
	if len(payload.Steps) == 0 {
		var step dockLaunchStep
		if err := json.Unmarshal(action, &step); err != nil || strings.TrimSpace(step.Instructions) == "" {
			return nil, "", fmt.Errorf("approval action does not contain launch steps")
		}
		payload.Steps = []dockLaunchStep{step}
	}
	return normalizeDockLaunchSteps(payload.Steps), strings.TrimSpace(payload.Prompt), nil
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
	chatRun, kind, err := s.resolveOrchestratorRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	steps = normalizeDockLaunchSteps(steps)
	actionHash, err := dockActionHash(steps)
	if err != nil {
		return nil, err
	}
	var interaction *model.AgentRunInteraction
	var supportConversationID *string
	switch kind {
	case orchestratorRunSupport:
		conversationID := strings.TrimSpace(chatRun.TargetID)
		supportConversationID = &conversationID
		if err := s.checkSupportChildCaps(ctx, meta.WorkspaceID, conversationID); err != nil {
			return nil, err
		}
		if !supportStepsAreReadOnly(steps) {
			interaction, err = s.verifyDockApproval(ctx, meta, chatRun, approvalInteractionID, "launch", actionHash, supportApprovalPayloadKind)
			if err != nil {
				return nil, fmt.Errorf("%w (this launch is not strictly read-only: either narrow every step's allowed_tools to read-only tools for auto-approval, or call request_approval with phase %q and the exact action — a teammate resolves it from the inbox)", err, supportApprovalPayloadKind)
			}
		}
	default:
		interaction, err = s.verifyDockApproval(ctx, meta, chatRun, approvalInteractionID, "launch", actionHash, dockApprovalPayloadKind)
		if err != nil {
			return nil, err
		}
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
				return nil, fmt.Errorf("step %d: sub-agent steps require allowed_tools", i+1)
			}
			commandAgent, err := s.agentService.ensureBuiltInAgent(ctx, meta.WorkspaceID, meta.ActorID, model.AgentPresetCommandAgent)
			if err != nil {
				return nil, fmt.Errorf("resolve sub-agent: %w", err)
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
		if targetType == "" || targetType == "workspace" {
			targetType = "workspace"
			targetID = meta.WorkspaceID
		}
		instructions := withDockChildHandoffInstruction(step.Instructions)
		if kind == orchestratorRunSupport {
			instructions = withSupportChildHandoffInstruction(step.Instructions)
		}
		planSteps = append(planSteps, model.CommandBarPlanStep{
			AgentID:              agentID,
			AgentName:            agentName,
			PlanKind:             planKind,
			Target:               model.CommandBarPageContext{EntityType: targetType, EntityID: targetID},
			Instructions:         instructions,
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
	}, dispatchPlanParams{parentChatRunID: &chatRun.ID, dockChatID: chatRun.DockChatID, supportConversationID: supportConversationID})
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

type dockAgentDraftSpec struct {
	Draft    model.CustomAgentDraft         `json:"draft"`
	Reasons  []model.CustomAgentDraftReason `json:"reasons,omitempty"`
	Warnings []string                       `json:"warnings,omitempty"`
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
			"use_command_agent": map[string]any{"type": "boolean", "description": "Run a Sub-agent instead of a saved agent. Requires allowed_tools."},
			"target": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{"type": "string", "description": "Target entity type: workspace, task, epic, sprint, objective, document, crm_deal, crm_contact, repository, support_conversation."},
					"id":   map[string]any{"type": "string", "description": "Target entity ID. Defaults to the workspace when omitted."},
				},
			},
			"instructions":            map[string]any{"type": "string", "description": "What the sub-agent run should do."},
			"allowed_tools":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Limited tool list for the sub-agent run (required for use_command_agent)."},
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
	launchTargets := []string{"workspace", "support_conversation"}

	s.register(InternalCommandDefinition{
		Name:                 "agents.start_run",
		Module:               "agents",
		Mutating:             true,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.start_run",
			Alias:       "start_agent_run",
			Category:    "Agents",
			Description: "Start one approved sub-agent run. For Dock launches, pass only approval_interaction_id and the server loads the immutable step from the approved dock_plan_confirm action. Support chat runs may still pass a complete read-only step without approval. The result is delivered back into this chat when the run finishes.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id":                dockLaunchStepSchema()["properties"].(map[string]any)["agent_id"],
					"use_command_agent":       dockLaunchStepSchema()["properties"].(map[string]any)["use_command_agent"],
					"target":                  dockLaunchStepSchema()["properties"].(map[string]any)["target"],
					"instructions":            dockLaunchStepSchema()["properties"].(map[string]any)["instructions"],
					"allowed_tools":           dockLaunchStepSchema()["properties"].(map[string]any)["allowed_tools"],
					"approval_interaction_id": map[string]any{"type": "string", "description": "ID of the resolved approval interaction (dock_plan_confirm or support_plan_confirm). Omit only for auto-approved read-only support launches."},
				},
				"required":             []string{},
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
			if strings.TrimSpace(req.Instructions) == "" && strings.TrimSpace(req.ApprovalInteractionID) != "" {
				steps, _, err := s.approvedDockLaunchSteps(ctx, meta, req.ApprovalInteractionID)
				if err != nil {
					return nil, err
				}
				if len(steps) != 1 {
					return nil, fmt.Errorf("approved action contains %d steps; use start_agent_plan", len(steps))
				}
				req.dockLaunchStep = steps[0]
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
			Description: "Start an approved multi-step sub-agent plan. For Dock launches, pass only approval_interaction_id and the server loads the immutable steps from the approved dock_plan_confirm action. Support chat runs may still pass complete read-only steps without approval. Results are delivered back into this chat when the plan settles.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt":                  map[string]any{"type": "string", "description": "Short description of the overall plan (shown in run surfaces)."},
					"steps":                   map[string]any{"type": "array", "items": dockLaunchStepSchema(), "description": "Plan steps in order."},
					"approval_interaction_id": map[string]any{"type": "string", "description": "ID of the resolved approval interaction (dock_plan_confirm or support_plan_confirm). Omit only for auto-approved read-only support launches."},
				},
				"required":             []string{},
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
			if len(req.Steps) == 0 && strings.TrimSpace(req.ApprovalInteractionID) != "" {
				steps, approvedPrompt, err := s.approvedDockLaunchSteps(ctx, meta, req.ApprovalInteractionID)
				if err != nil {
					return nil, err
				}
				req.Steps = steps
				if strings.TrimSpace(req.Prompt) == "" {
					req.Prompt = approvedPrompt
				}
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
			Description: "Get a sub-agent plan's status, or retrieve the persisted final response of a sub-agent run started from this chat. Use status only when the user asks about progress. Use detail_level=result when a delivered sub-agent summary says summary_truncated=true; retrieve the same run instead of launching replacement work.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"run_id":        map[string]any{"type": "string", "description": "Sub-agent run ID returned in the sub-agent result. Mutually exclusive with plan_id."},
					"plan_id":       map[string]any{"type": "string", "description": "Sub-agent plan ID whose status should be returned. Mutually exclusive with run_id."},
					"detail_level":  map[string]any{"type": "string", "enum": []string{"status", "result"}, "description": "Return status metadata, or a bounded page of the run's persisted final response. Defaults to status."},
					"result_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Unicode-character offset for result retrieval. Defaults to 0."},
					"result_limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": dockRunResultMaxChars, "description": "Maximum Unicode characters to return. Defaults to 6000, max 12000."},
				},
				"required":             []string{},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req dockGetRunRequest
			if len(input) > 0 {
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, fmt.Errorf("parse get run input: %w", err)
				}
			}
			var err error
			req, err = normalizeDockGetRunRequest(req)
			if err != nil {
				return nil, err
			}
			chatRun, kind, err := s.resolveOrchestratorRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			if req.PlanID != "" {
				if s.commandBarService == nil || s.commandBarService.planRepo == nil {
					return nil, fmt.Errorf("command bar service is not configured")
				}
				plan, planErr := s.commandBarService.planRepo.GetByID(ctx, meta.WorkspaceID, req.PlanID)
				if planErr != nil {
					return nil, planErr
				}
				if !orchestratorOwnsPlan(chatRun, kind, plan) {
					return nil, fmt.Errorf("plan_id was not launched from this chat")
				}
				detail, err := s.commandBarService.GetPlan(ctx, meta.WorkspaceID, meta.ActorID, req.PlanID)
				if err != nil {
					return nil, err
				}
				return mustJSON(detail), nil
			}
			if s.agentRunRepo == nil {
				return nil, fmt.Errorf("agent run repository is not configured")
			}
			if _, err := s.orchestratorPlanOwningRun(ctx, chatRun, kind, req.RunID); err != nil {
				return nil, err
			}
			run, err := s.agentRunRepo.GetByID(ctx, meta.WorkspaceID, req.RunID)
			if err != nil {
				return nil, err
			}
			if run == nil {
				return nil, fmt.Errorf("run not found")
			}
			response := dockGetRunResponse{
				RunID:       run.ID,
				Status:      run.Status,
				PauseReason: run.PauseReason,
				AgentID:     run.AgentID,
				TargetType:  run.TargetType,
				TargetID:    run.TargetID,
				Error:       strings.TrimSpace(derefString(run.ErrorMessage)),
			}
			if req.DetailLevel == "result" {
				if s.agentService == nil || s.agentService.runMessageRepo == nil {
					return nil, fmt.Errorf("agent run message repository is not configured")
				}
				messages, messageErr := s.agentService.runMessageRepo.ListByRun(ctx, meta.WorkspaceID, run.ID)
				if messageErr != nil {
					return nil, messageErr
				}
				content := latestAssistantResponse(messages)
				response.ResultAvailable = content != ""
				if response.ResultAvailable {
					if req.ResultOffset > len([]rune(strings.TrimSpace(content))) {
						return nil, fmt.Errorf("result_offset exceeds result char_count")
					}
					excerpt := dockRunResultWindow(content, req.ResultOffset, req.ResultLimit)
					response.Result = &excerpt
				}
				if s.agentRunArtifactRepo != nil {
					artifacts, artifactErr := s.agentRunArtifactRepo.ListByRun(ctx, meta.WorkspaceID, run.ID)
					if artifactErr != nil {
						return nil, artifactErr
					}
					response.Artifacts = dockRunArtifactReferences(artifacts)
				}
				slog.InfoContext(ctx, "child run result retrieved",
					"workspace_id", meta.WorkspaceID, "chat_run_id", chatRun.ID,
					"run_id", run.ID, "result_offset", req.ResultOffset, "result_limit", req.ResultLimit)
			}
			return mustJSON(response), nil
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
			Description: "Cancel a sub-agent run or plan started from this chat. No approval needed — cancelling stops work.",
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
			cancelChatRun, cancelKind, err := s.resolveOrchestratorRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			if planID := strings.TrimSpace(req.PlanID); planID != "" {
				if s.commandBarService == nil || s.commandBarService.planRepo == nil {
					return nil, fmt.Errorf("command bar service is not configured")
				}
				plan, planErr := s.commandBarService.planRepo.GetByID(ctx, meta.WorkspaceID, planID)
				if planErr != nil {
					return nil, planErr
				}
				if !orchestratorOwnsPlan(cancelChatRun, cancelKind, plan) {
					return nil, fmt.Errorf("plan_id was not launched from this chat")
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
			if _, err := s.orchestratorPlanOwningRun(ctx, cancelChatRun, cancelKind, runID); err != nil {
				return nil, err
			}
			if _, err := s.agentService.CancelRun(ctx, meta.WorkspaceID, runID, meta.ActorID); err != nil {
				return nil, err
			}
			return mustJSON(map[string]string{"run_id": runID, "status": "cancelled"}), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "agents.draft_agent",
		Module:               "agents",
		Mutating:             false,
		SupportedTargetTypes: launchTargets,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.draft_agent",
			Alias:       "draft_custom_agent",
			Category:    "Agents",
			Description: "Draft and persist a complete reusable-agent proposal, including prompt, tools, targets, skills, runtime, and warnings. Review the result, then request dock_plan_confirm approval with action {\"proposal_id\":...} before create_custom_agent.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]any{"type": "string", "description": "Optional name override applied to the generated draft."},
					"description": map[string]any{"type": "string", "minLength": 10, "description": "Detailed responsibilities and expected outcomes for the reusable agent."},
				},
				"required":             []string{"description"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.agentService == nil {
				return nil, fmt.Errorf("agent service is not configured")
			}
			if s.dockActionProposalRepo == nil {
				return nil, fmt.Errorf("dock action proposal repository is not configured")
			}
			var req dockCreateAgentAction
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse draft agent input: %w", err)
			}
			req = req.normalized()
			if len(req.Description) < 10 {
				return nil, fmt.Errorf("description must be at least 10 characters")
			}
			chatRun, err := s.resolveDockChatRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			draftResp, err := s.agentService.DraftCustomAgent(ctx, meta.WorkspaceID, model.CustomAgentDraftRequest{Description: req.Description})
			if err != nil {
				return nil, fmt.Errorf("draft agent: %w", err)
			}
			if req.Name != "" {
				draftResp.Draft.Name = req.Name
			}
			spec := dockAgentDraftSpec{Draft: draftResp.Draft, Reasons: draftResp.Reasons, Warnings: draftResp.Warnings}
			specJSON, err := json.Marshal(spec)
			if err != nil {
				return nil, fmt.Errorf("encode agent draft proposal: %w", err)
			}
			proposal := &model.DockActionProposal{
				WorkspaceID: meta.WorkspaceID, DockChatRunID: chatRun.ID, ActorID: meta.ActorID,
				Kind: model.DockActionProposalKindAgentDraft, Status: model.DockActionProposalStatusPrepared,
				Summary: "Create reusable agent " + draftResp.Draft.Name, Spec: specJSON, Usage: json.RawMessage(`{}`),
				ExpiresAt: time.Now().UTC().Add(72 * time.Hour),
			}
			if err := s.dockActionProposalRepo.Create(ctx, proposal); err != nil {
				return nil, err
			}
			return mustJSON(map[string]interface{}{
				"proposal_id": proposal.ID, "draft": draftResp.Draft, "reasons": draftResp.Reasons, "warnings": draftResp.Warnings,
				"approval": map[string]interface{}{"phase": dockApprovalPayloadKind, "action": map[string]string{"proposal_id": proposal.ID}},
			}), nil
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
			Description: "Create the reusable custom agent stored by draft_custom_agent. Pass only the resolved dock_plan_confirm approval_interaction_id; the server loads the immutable approved draft and its full capabilities.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"approval_interaction_id": map[string]any{"type": "string"},
				},
				"required":             []string{"approval_interaction_id"},
				"additionalProperties": false,
			},
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			var req struct {
				ApprovalInteractionID string `json:"approval_interaction_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse create agent input: %w", err)
			}
			if s.agentService == nil {
				return nil, fmt.Errorf("agent service is not configured")
			}
			if s.dockActionProposalRepo == nil {
				return nil, fmt.Errorf("dock action proposal repository is not configured")
			}
			chatRun, err := s.resolveDockChatRun(ctx, meta)
			if err != nil {
				return nil, err
			}
			interaction, approvedAction, err := s.resolvedDockApprovalAction(ctx, meta, chatRun, req.ApprovalInteractionID, dockApprovalPayloadKind)
			if err != nil {
				return nil, err
			}
			var approved struct {
				ProposalID string `json:"proposal_id"`
			}
			if err := json.Unmarshal(approvedAction, &approved); err != nil || strings.TrimSpace(approved.ProposalID) == "" {
				return nil, fmt.Errorf("approval action must contain the draft proposal_id")
			}
			proposal, err := s.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, strings.TrimSpace(approved.ProposalID))
			if err != nil {
				return nil, err
			}
			if proposal == nil || proposal.DockChatRunID != chatRun.ID || proposal.ActorID != meta.ActorID || proposal.Kind != model.DockActionProposalKindAgentDraft {
				return nil, fmt.Errorf("approved agent draft proposal was not prepared by this chat")
			}
			activated, err := s.dockActionProposalRepo.Activate(ctx, meta.WorkspaceID, proposal.ID, interaction.ID, time.Now().UTC())
			if err != nil {
				return nil, err
			}
			if !activated {
				return nil, fmt.Errorf("agent draft proposal is expired or already used")
			}
			var spec dockAgentDraftSpec
			if err := json.Unmarshal(proposal.Spec, &spec); err != nil {
				_ = s.dockActionProposalRepo.ResetActivation(ctx, meta.WorkspaceID, proposal.ID, interaction.ID)
				return nil, fmt.Errorf("decode agent draft proposal: %w", err)
			}
			createReq := commandBarCreateAgentRequestFromDraft(meta.WorkspaceID, spec.Draft, model.ConfirmCommandBarChatProposalRequest{})
			agent, err := s.agentService.CreateAgent(ctx, createReq, meta.ActorID)
			if err != nil {
				_ = s.dockActionProposalRepo.ResetActivation(ctx, meta.WorkspaceID, proposal.ID, interaction.ID)
				return nil, fmt.Errorf("create agent: %w", err)
			}
			_, _ = s.dockActionProposalRepo.Complete(ctx, meta.WorkspaceID, proposal.ID, time.Now().UTC())
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
				interaction, approvalErr := s.verifyDockApproval(ctx, meta, run, req.ApprovalInteractionID, "epic_pipeline", actionHash, dockApprovalPayloadKind)
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
			Description: "Promote a finished sub-agent run into a reusable saved agent. Requires a resolved dock_plan_confirm approval whose action matches this call exactly.",
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
			interaction, err := s.verifyDockApproval(ctx, meta, chatRun, req.ApprovalInteractionID, "promote_run", actionHash, dockApprovalPayloadKind)
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

// compactAgentListEntry is the selection-focused row list_agents returns.
// The full CommandBarAgent shape (with complete allowed_tools) overflows the
// model-visible tool output budget in workspaces with many agents, which
// hides agents that sort late — so the directory stays compact and filterable.
type compactAgentListEntry struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	PresetKey      string   `json:"preset_key,omitempty"`
	Role           string   `json:"role,omitempty"`
	IsSystem       bool     `json:"is_system,omitempty"`
	Status         string   `json:"status,omitempty"`
	AllowedTargets []string `json:"allowed_targets,omitempty"`
}

type compactAgentDirectoryResult struct {
	Agents  []compactAgentListEntry `json:"agents"`
	Total   int                     `json:"total"`
	Omitted int                     `json:"omitted,omitempty"`
	Note    string                  `json:"note,omitempty"`
}

// compactAgentDirectory filters and compacts the actor-visible agents for the
// list_agents tool.
func compactAgentDirectory(agents []model.Agent, query, targetType string, limit int) compactAgentDirectoryResult {
	query = strings.ToLower(strings.TrimSpace(query))
	targetType = strings.TrimSpace(targetType)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	entries := make([]compactAgentListEntry, 0, len(agents))
	for _, agent := range agents {
		targets := parseJSONStringSlice(agent.AllowedTargets)
		if targetType != "" && !slices.Contains(targets, targetType) {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(agent.Name + " " + agent.PresetKey + " " + agent.Role)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		role := strings.TrimSpace(agent.Role)
		if len(role) > 120 {
			role = role[:120] + "…"
		}
		entries = append(entries, compactAgentListEntry{
			ID:             agent.ID,
			Name:           agent.Name,
			PresetKey:      agent.PresetKey,
			Role:           role,
			IsSystem:       agent.IsSystem,
			Status:         agent.Status,
			AllowedTargets: targets,
		})
	}
	// System agents first, then by name, so the built-in specialists survive
	// any downstream truncation.
	slices.SortStableFunc(entries, func(a, b compactAgentListEntry) int {
		if a.IsSystem != b.IsSystem {
			if a.IsSystem {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	total := len(entries)
	omitted := 0
	if len(entries) > limit {
		omitted = len(entries) - limit
		entries = entries[:limit]
	}
	result := compactAgentDirectoryResult{Agents: entries, Total: total, Omitted: omitted}
	if omitted > 0 {
		result.Note = fmt.Sprintf("%d more agents not shown — narrow with query or target_type, or raise limit.", omitted)
	}
	return result
}
