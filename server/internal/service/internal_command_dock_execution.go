package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	dockExecutionApprovalPhase = "dock_execution_confirm"
	dockExecutionGrantTTL      = 15 * time.Minute
)

type dockExecutionOperation struct {
	ToolName    string                 `json:"tool_name"`
	MaxCalls    int                    `json:"max_calls"`
	Constraints map[string]interface{} `json:"constraints,omitempty"`
	ExactInput  bool                   `json:"exact_input,omitempty"`
}

type dockExecutionSpec struct {
	Operations       []dockExecutionOperation `json:"operations"`
	ExpectedOutcomes []string                 `json:"expected_outcomes,omitempty"`
}

func (s *InternalCommandService) registerDockExecutionCommands() {
	operationSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tool_name":   map[string]any{"type": "string", "minLength": 1, "description": "Mutating tool alias to authorize."},
			"max_calls":   map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "description": "Maximum calls allowed for this tool. Defaults to 1."},
			"constraints": map[string]any{"type": "object", "minProperties": 1, "description": "Required exact top-level input fields every call must match, such as space_id, document_id, or task_id."},
		},
		"required":             []string{"tool_name", "constraints"},
		"additionalProperties": false,
	}
	s.register(InternalCommandDefinition{
		Name:                 "agents.prepare_dock_execution",
		Module:               "agents",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.prepare_dock_execution",
			Alias:       "prepare_dock_execution",
			Category:    "Agents",
			Description: "Prepare a bounded set of direct Dock mutations. After this succeeds, request approval with phase dock_execution_confirm and action {\"proposal_id\":...}; do not copy the mutation payload into the approval.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary":           map[string]any{"type": "string", "minLength": 1, "description": "Short user-facing description of the intended changes."},
					"operations":        map[string]any{"type": "array", "minItems": 1, "maxItems": 20, "items": operationSchema},
					"expected_outcomes": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}, "maxItems": 20},
				},
				"required":             []string{"summary", "operations", "expected_outcomes"},
				"additionalProperties": false,
			},
		},
		Execute: s.executePrepareDockExecution,
	})
	s.register(InternalCommandDefinition{
		Name:                 "agents.activate_dock_execution",
		Module:               "agents",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.activate_dock_execution",
			Alias:       "activate_dock_execution",
			Category:    "Agents",
			Description: "Activate the immutable Dock execution proposal referenced by a resolved dock_execution_confirm approval. Pass only the approval interaction ID.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"approval_interaction_id": map[string]any{"type": "string", "minLength": 1}},
				"required":   []string{"approval_interaction_id"}, "additionalProperties": false,
			},
		},
		Execute: s.executeActivateDockExecution,
	})
	s.register(InternalCommandDefinition{
		Name:                 "agents.finish_dock_execution",
		Module:               "agents",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "agents.finish_dock_execution",
			Alias:       "finish_dock_execution",
			Category:    "Agents",
			Description: "Finish an active Dock execution after its mutations succeed. Returns the approved outcomes and consumed call counts; call this before claiming completion.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"proposal_id": map[string]any{"type": "string", "minLength": 1}},
				"required":   []string{"proposal_id"}, "additionalProperties": false,
			},
		},
		Execute: s.executeFinishDockExecution,
	})
}

func (s *InternalCommandService) executePrepareDockExecution(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.dockActionProposalRepo == nil {
		return nil, fmt.Errorf("dock action proposal repository is not configured")
	}
	var req struct {
		Summary          string                   `json:"summary"`
		Operations       []dockExecutionOperation `json:"operations"`
		ExpectedOutcomes []string                 `json:"expected_outcomes"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse prepare dock execution input: %w", err)
	}
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if len(req.Operations) == 0 {
		return nil, fmt.Errorf("operations is required")
	}
	req.ExpectedOutcomes = normalizeStringSlice(req.ExpectedOutcomes)
	if len(req.ExpectedOutcomes) == 0 {
		return nil, fmt.Errorf("expected_outcomes is required")
	}
	run, err := s.resolveDockChatRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	for index := range req.Operations {
		op := &req.Operations[index]
		op.ToolName = strings.TrimSpace(op.ToolName)
		if op.MaxCalls == 0 {
			op.MaxCalls = 1
		}
		if op.MaxCalls < 1 || op.MaxCalls > 20 {
			return nil, fmt.Errorf("operation %d max_calls must be between 1 and 20", index+1)
		}
		if len(op.Constraints) == 0 {
			return nil, fmt.Errorf("operation %d constraints must contain at least one exact target field", index+1)
		}
		def, ok := s.definitionByToolAlias(op.ToolName)
		if !ok || !def.Mutating {
			return nil, fmt.Errorf("operation %d tool_name must name a mutating Helpin tool", index+1)
		}
		if !dockAllowsDirectMutation(def) {
			return nil, fmt.Errorf("tool %q requires a dedicated approval or child agent", op.ToolName)
		}
		if err := s.authorizeCommandActor(meta, def); err != nil {
			return nil, err
		}
	}
	spec := dockExecutionSpec{Operations: req.Operations, ExpectedOutcomes: req.ExpectedOutcomes}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("encode dock execution proposal: %w", err)
	}
	proposal := &model.DockActionProposal{
		WorkspaceID:   meta.WorkspaceID,
		DockChatRunID: run.ID,
		ActorID:       meta.ActorID,
		Kind:          model.DockActionProposalKindExecution,
		Status:        model.DockActionProposalStatusPrepared,
		Summary:       req.Summary,
		Spec:          specJSON,
		Usage:         json.RawMessage(`{}`),
		ExpiresAt:     time.Now().UTC().Add(72 * time.Hour),
	}
	if err := s.dockActionProposalRepo.Create(ctx, proposal); err != nil {
		return nil, err
	}
	return mustJSON(map[string]interface{}{
		"proposal_id":       proposal.ID,
		"summary":           proposal.Summary,
		"operations":        spec.Operations,
		"expected_outcomes": spec.ExpectedOutcomes,
		"approval":          map[string]interface{}{"phase": dockExecutionApprovalPhase, "action": map[string]string{"proposal_id": proposal.ID}},
	}), nil
}

func (s *InternalCommandService) executeActivateDockExecution(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ApprovalInteractionID string `json:"approval_interaction_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse activate dock execution input: %w", err)
	}
	run, err := s.resolveDockChatRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	interaction, proposalID, err := s.resolvedDockExecutionApproval(ctx, meta, run, req.ApprovalInteractionID)
	if err != nil {
		return nil, err
	}
	proposal, err := s.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.DockChatRunID != run.ID || proposal.ActorID != meta.ActorID {
		return nil, fmt.Errorf("approved Dock execution proposal was not prepared by this chat")
	}
	now := time.Now().UTC()
	activated, err := s.dockActionProposalRepo.Activate(ctx, meta.WorkspaceID, proposal.ID, interaction.ID, now)
	if err != nil {
		return nil, err
	}
	if !activated {
		return nil, fmt.Errorf("Dock execution proposal is expired or already activated")
	}
	proposal.Status = model.DockActionProposalStatusActive
	proposal.ExpiresAt = now.Add(dockExecutionGrantTTL)
	// The activation update deliberately keeps the durable proposal expiry; the
	// command guard also caps active grants to 15 minutes from activated_at.
	var spec dockExecutionSpec
	_ = json.Unmarshal(proposal.Spec, &spec)
	return mustJSON(map[string]interface{}{"proposal_id": proposal.ID, "status": "active", "operations": spec.Operations, "expires_in_seconds": int(dockExecutionGrantTTL.Seconds())}), nil
}

func (s *InternalCommandService) executeFinishDockExecution(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ProposalID string `json:"proposal_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse finish dock execution input: %w", err)
	}
	run, err := s.resolveDockChatRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	proposal, err := s.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, strings.TrimSpace(req.ProposalID))
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.DockChatRunID != run.ID || proposal.ActorID != meta.ActorID || proposal.Status != model.DockActionProposalStatusActive {
		return nil, fmt.Errorf("active Dock execution proposal not found for this chat")
	}
	var spec dockExecutionSpec
	_ = json.Unmarshal(proposal.Spec, &spec)
	usage := map[string]int{}
	_ = json.Unmarshal(proposal.Usage, &usage)
	if len(usage) == 0 {
		return nil, fmt.Errorf("Dock execution has not performed any approved mutation")
	}
	completed, err := s.dockActionProposalRepo.Complete(ctx, meta.WorkspaceID, proposal.ID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !completed {
		return nil, fmt.Errorf("Dock execution proposal is no longer active")
	}
	return mustJSON(map[string]interface{}{"proposal_id": proposal.ID, "status": "completed", "usage": usage, "expected_outcomes": spec.ExpectedOutcomes}), nil
}

func (s *InternalCommandService) resolvedDockExecutionApproval(ctx context.Context, meta model.InternalCommandContext, run *model.AgentRun, interactionID string) (*model.AgentRunInteraction, string, error) {
	if s.agentRunInteractionRepo == nil {
		return nil, "", fmt.Errorf("interaction repository is not configured")
	}
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil, "", fmt.Errorf("approval_interaction_id is required")
	}
	interaction, err := s.agentRunInteractionRepo.GetByID(ctx, meta.WorkspaceID, run.ID, interactionID)
	if err != nil {
		return nil, "", err
	}
	if interaction == nil {
		interactions, listErr := s.agentRunInteractionRepo.ListByRun(ctx, meta.WorkspaceID, run.ID)
		if listErr != nil {
			return nil, "", listErr
		}
		for index := range interactions {
			if agentRunInteractionHasRuntimeInteractionID(interactions[index], interactionID) {
				interaction = &interactions[index]
				break
			}
		}
	}
	if interaction == nil || interaction.InteractionKind != model.AgentRunInteractionKindApprovalRequest || interaction.Status != model.AgentRunInteractionStatusResolved {
		return nil, "", fmt.Errorf("resolved approval interaction not found on this chat run")
	}
	var response struct {
		Decision string `json:"decision"`
	}
	if json.Unmarshal(interaction.ResponsePayload, &response) != nil || strings.TrimSpace(response.Decision) != "approve" {
		return nil, "", fmt.Errorf("the user did not approve this execution")
	}
	var request struct {
		Kind   string `json:"kind"`
		Phase  string `json:"phase"`
		Action struct {
			ProposalID string `json:"proposal_id"`
		} `json:"action"`
		RawInput struct {
			Action struct {
				ProposalID string `json:"proposal_id"`
			} `json:"action"`
		} `json:"raw_input"`
	}
	if json.Unmarshal(interaction.RequestPayload, &request) != nil || strings.TrimSpace(firstNonEmptyString(request.Kind, request.Phase)) != dockExecutionApprovalPhase {
		return nil, "", fmt.Errorf("approval must use phase %q", dockExecutionApprovalPhase)
	}
	proposalID := strings.TrimSpace(firstNonEmptyString(request.Action.ProposalID, request.RawInput.Action.ProposalID))
	if proposalID == "" {
		return nil, "", fmt.Errorf("approval action proposal_id is required")
	}
	return interaction, proposalID, nil
}

func (s *InternalCommandService) definitionByToolAlias(alias string) (InternalCommandDefinition, bool) {
	for _, def := range s.definitions {
		if def.Tool != nil && strings.TrimSpace(def.Tool.Alias) == strings.TrimSpace(alias) {
			return def, true
		}
	}
	return InternalCommandDefinition{}, false
}

func dockAllowsDirectMutation(def InternalCommandDefinition) bool {
	switch strings.TrimSpace(def.Module) {
	case "agents", "git", "delivery", "release":
		return false
	}
	if def.Tool == nil {
		return false
	}
	switch strings.TrimSpace(def.Tool.Alias) {
	case "send_support_reply", "escalate_to_human":
		return false
	default:
		return true
	}
}

func (s *InternalCommandService) authorizeDockMutation(ctx context.Context, meta model.InternalCommandContext, def InternalCommandDefinition, input json.RawMessage) error {
	if s.agentRunRepo == nil || strings.TrimSpace(meta.RunID) == "" || def.Module == "agents" {
		return nil
	}
	if s.runtimeOwnsCommandApproval(ctx, meta) {
		return nil
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil || run == nil || run.DockChatID == nil || strings.TrimSpace(*run.DockChatID) == "" {
		return err
	}
	if s.dockActionProposalRepo == nil {
		return fmt.Errorf("Dock mutation guard is not configured")
	}
	alias := ""
	if def.Tool != nil {
		alias = strings.TrimSpace(def.Tool.Alias)
	}
	var values map[string]interface{}
	if err := json.Unmarshal(input, &values); err != nil {
		return fmt.Errorf("parse mutation input for approval: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		proposals, err := s.dockActionProposalRepo.ListActiveForRun(ctx, meta.WorkspaceID, run.ID, time.Now().UTC())
		if err != nil {
			return err
		}
		for index := range proposals {
			proposal := &proposals[index]
			if proposal.ActivatedAt == nil || time.Since(*proposal.ActivatedAt) > dockExecutionGrantTTL {
				continue
			}
			var spec dockExecutionSpec
			if json.Unmarshal(proposal.Spec, &spec) != nil {
				continue
			}
			usage := map[string]int{}
			_ = json.Unmarshal(proposal.Usage, &usage)
			for operationIndex, op := range spec.Operations {
				usageKey := fmt.Sprintf("%d:%s", operationIndex, op.ToolName)
				if op.ToolName != alias || usage[usageKey] >= op.MaxCalls || !dockInputMatchesOperation(values, op) {
					continue
				}
				previous := append(json.RawMessage(nil), proposal.Usage...)
				usage[usageKey]++
				encoded, _ := json.Marshal(usage)
				proposal.Usage = encoded
				consumed, consumeErr := s.dockActionProposalRepo.ConsumeUsage(ctx, proposal, previous)
				if consumeErr != nil {
					return consumeErr
				}
				if consumed {
					return nil
				}
				break
			}
		}
		if attempt == 0 {
			activated, activateErr := s.activateApprovedDockMutation(ctx, meta, run, alias, values)
			if activateErr != nil {
				return activateErr
			}
			if activated {
				continue
			}
		}
	}
	return s.prepareDockMutationApprovalRequired(ctx, meta, run, alias, values)
}

// runtimeOwnsCommandApproval prevents Helpin from asking for a second Dock
// approval after Agent Runtime has applied the configured per-tool policy.
// Approval mode "never" retains the legacy host proposal guard.
func (s *InternalCommandService) runtimeOwnsCommandApproval(ctx context.Context, meta model.InternalCommandContext) bool {
	if s == nil || s.agentService == nil || strings.TrimSpace(meta.AgentID) == "" || strings.TrimSpace(meta.WorkspaceID) == "" {
		return false
	}
	agent, err := s.agentService.GetAgent(ctx, meta.WorkspaceID, meta.AgentID)
	if err != nil || agent == nil {
		return false
	}
	switch strings.TrimSpace(agent.ApprovalMode) {
	case "risk_based", "mutating_tools", "always":
		return true
	default:
		return false
	}
}

// activateApprovedDockMutation makes the common single-mutation flow robust
// to a model omitting activate_dock_execution after approval. The immutable
// proposal and resolved approval still have to match this exact tool input.
func (s *InternalCommandService) activateApprovedDockMutation(ctx context.Context, meta model.InternalCommandContext, run *model.AgentRun, alias string, values map[string]interface{}) (bool, error) {
	if s.agentRunInteractionRepo == nil {
		return false, nil
	}
	interactions, err := s.agentRunInteractionRepo.ListByRun(ctx, meta.WorkspaceID, run.ID)
	if err != nil {
		return false, err
	}
	for index := len(interactions) - 1; index >= 0; index-- {
		interaction := &interactions[index]
		proposalID, approved := approvedDockExecutionProposalID(interaction)
		if !approved {
			continue
		}
		proposal, err := s.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, proposalID)
		if err != nil {
			return false, err
		}
		if proposal == nil || proposal.DockChatRunID != run.ID || proposal.ActorID != meta.ActorID || proposal.Status != model.DockActionProposalStatusPrepared || !dockProposalAllowsMutation(proposal, alias, values) {
			continue
		}
		activated, err := s.dockActionProposalRepo.Activate(ctx, meta.WorkspaceID, proposal.ID, interaction.ID, time.Now().UTC())
		if err != nil {
			return false, err
		}
		if activated {
			return true, nil
		}
	}
	return false, nil
}

func approvedDockExecutionProposalID(interaction *model.AgentRunInteraction) (string, bool) {
	if interaction == nil || interaction.InteractionKind != model.AgentRunInteractionKindApprovalRequest || interaction.Status != model.AgentRunInteractionStatusResolved {
		return "", false
	}
	var response struct {
		Decision string `json:"decision"`
	}
	if json.Unmarshal(interaction.ResponsePayload, &response) != nil || strings.TrimSpace(response.Decision) != "approve" {
		return "", false
	}
	var request struct {
		Kind   string `json:"kind"`
		Phase  string `json:"phase"`
		Action struct {
			ProposalID string `json:"proposal_id"`
		} `json:"action"`
		RawInput struct {
			Action struct {
				ProposalID string `json:"proposal_id"`
			} `json:"action"`
		} `json:"raw_input"`
	}
	if json.Unmarshal(interaction.RequestPayload, &request) != nil || strings.TrimSpace(firstNonEmptyString(request.Kind, request.Phase)) != dockExecutionApprovalPhase {
		return "", false
	}
	proposalID := strings.TrimSpace(firstNonEmptyString(request.Action.ProposalID, request.RawInput.Action.ProposalID))
	return proposalID, proposalID != ""
}

func dockProposalAllowsMutation(proposal *model.DockActionProposal, alias string, values map[string]interface{}) bool {
	if proposal == nil {
		return false
	}
	var spec dockExecutionSpec
	if json.Unmarshal(proposal.Spec, &spec) != nil {
		return false
	}
	for _, op := range spec.Operations {
		if op.ToolName == alias && op.MaxCalls > 0 && dockInputMatchesOperation(values, op) {
			return true
		}
	}
	return false
}

// prepareDockMutationApprovalRequired is a recovery path for a direct mutation
// attempted before prepare_dock_execution. It stores the exact input as a
// one-call immutable proposal and returns only the compact approval payload;
// large document content is not echoed back through the model transcript.
func (s *InternalCommandService) prepareDockMutationApprovalRequired(ctx context.Context, meta model.InternalCommandContext, run *model.AgentRun, alias string, values map[string]interface{}) error {
	if len(values) == 0 {
		return fmt.Errorf("Dock mutation %q requires prepare_dock_execution before approval", alias)
	}
	now := time.Now().UTC()
	prepared, err := s.dockActionProposalRepo.ListPreparedForRun(ctx, meta.WorkspaceID, run.ID, now)
	if err != nil {
		return err
	}
	var proposal *model.DockActionProposal
	for index := range prepared {
		if prepared[index].ActorID == meta.ActorID && dockProposalAllowsMutation(&prepared[index], alias, values) {
			proposal = &prepared[index]
			break
		}
	}
	if proposal == nil {
		summary := dockMutationSummary(alias, values)
		specJSON, marshalErr := json.Marshal(dockExecutionSpec{
			Operations:       []dockExecutionOperation{{ToolName: alias, MaxCalls: 1, Constraints: values, ExactInput: true}},
			ExpectedOutcomes: []string{fmt.Sprintf("%s completes with the approved input", alias)},
		})
		if marshalErr != nil {
			return marshalErr
		}
		proposal = &model.DockActionProposal{
			ID: uuid.NewString(), WorkspaceID: meta.WorkspaceID, DockChatRunID: run.ID, ActorID: meta.ActorID,
			Kind: model.DockActionProposalKindExecution, Status: model.DockActionProposalStatusPrepared,
			Summary: summary, Spec: specJSON, Usage: json.RawMessage(`{}`), ExpiresAt: now.Add(72 * time.Hour),
		}
		if err := s.dockActionProposalRepo.Create(ctx, proposal); err != nil {
			return err
		}
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"code":        "dock_execution_approval_required",
		"message":     "Request approval with the returned payload, then retry this same mutation unchanged; activation is automatic.",
		"proposal_id": proposal.ID,
		"next_tool":   "request_approval",
		"next_input": map[string]interface{}{
			"phase":   dockExecutionApprovalPhase,
			"title":   "Approve workspace change",
			"summary": proposal.Summary,
			"action":  map[string]string{"proposal_id": proposal.ID},
		},
	})
	return errors.New(string(payload))
}

func dockMutationSummary(alias string, values map[string]interface{}) string {
	action := strings.ReplaceAll(strings.TrimSpace(alias), "_", " ")
	label := ""
	for _, key := range []string{"title", "name", "document_id", "task_id", "deal_id"} {
		if value, ok := values[key]; ok {
			label = strings.TrimSpace(fmt.Sprint(value))
			if label != "" {
				break
			}
		}
	}
	labelRunes := []rune(label)
	if len(labelRunes) > 120 {
		label = string(labelRunes[:120]) + "…"
	}
	if label == "" {
		return "Allow Ask Agent to " + action
	}
	return fmt.Sprintf("Allow Ask Agent to %s: %s", action, label)
}

func dockInputMatchesOperation(input map[string]interface{}, operation dockExecutionOperation) bool {
	if operation.ExactInput && len(input) != len(operation.Constraints) {
		return false
	}
	return dockInputMatchesConstraints(input, operation.Constraints)
}

func dockInputMatchesConstraints(input, constraints map[string]interface{}) bool {
	for key, expected := range constraints {
		actual, ok := input[key]
		if !ok || !reflect.DeepEqual(actual, expected) {
			return false
		}
	}
	return true
}
