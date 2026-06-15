package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type AgentToolGateway struct {
	runRepo         *repository.AgentRunRepository
	agentRepo       *repository.AgentRepository
	artifactRepo    *repository.AgentRunArtifactRepository
	interactionRepo *repository.AgentRunInteractionRepository
	commandService  *InternalCommandService
	jwtManager      *auth.JWTManager
	wsPublisher     websocket.EventPublisher
}

func NewAgentToolGateway(
	runRepo *repository.AgentRunRepository,
	agentRepo *repository.AgentRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	interactionRepo *repository.AgentRunInteractionRepository,
	commandService *InternalCommandService,
	jwtManager *auth.JWTManager,
	wsPublisher websocket.EventPublisher,
) *AgentToolGateway {
	return &AgentToolGateway{
		runRepo:         runRepo,
		agentRepo:       agentRepo,
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
		commandService:  commandService,
		jwtManager:      jwtManager,
		wsPublisher:     wsPublisher,
	}
}

func (g *AgentToolGateway) ListTools(ctx context.Context, token string) (*model.AgentRunToolListResponse, error) {
	state, err := g.resolveRunToolState(ctx, token)
	if err != nil {
		return nil, err
	}
	allowed := effectiveGatewayTools(state.run, state.agent)
	entries := make([]model.AgentRunToolCatalogEntry, 0)
	entries = append(entries, gatewayRuntimeToolEntries(allowed)...)
	for _, def := range g.commandService.ToolDefinitions() {
		if def.Tool == nil || !allowed[def.Tool.Alias] || !gatewayExposesCommand(def) {
			continue
		}
		entries = append(entries, gatewayToolEntry(def))
	}
	slices.SortFunc(entries, func(a, b model.AgentRunToolCatalogEntry) int {
		if a.Category != b.Category {
			return strings.Compare(a.Category, b.Category)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return &model.AgentRunToolListResponse{RunID: state.run.ID, Tools: entries}, nil
}

func (g *AgentToolGateway) CallTool(ctx context.Context, token string, req model.AgentRunToolCallRequest) (*model.AgentRunToolCallResponse, error) {
	state, err := g.resolveRunToolState(ctx, token)
	if err != nil {
		return nil, err
	}
	toolName := strings.TrimSpace(req.ToolName)
	if toolName == "" {
		return nil, fmt.Errorf("tool_name is required")
	}
	def, ok := g.commandDefinitionForTool(toolName)
	if !ok || !gatewayExposesCommand(def) {
		if gatewayExposesRuntimeTool(toolName) {
			return g.callRuntimeTool(ctx, state, toolName, req.Input)
		}
		return nil, fmt.Errorf("tool %q is not exposed", toolName)
	}
	if !effectiveGatewayTools(state.run, state.agent)[toolName] {
		return nil, fmt.Errorf("tool %q is not allowed for this run", toolName)
	}
	if err := validateGatewayTarget(def, state.run.TargetType); err != nil {
		return nil, err
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage(`{}`)
	}
	if def.Mutating && gatewayRequiresApproval(state.agent) {
		interactionID, err := g.createToolApprovalInteraction(ctx, state, def, req.Input)
		if err != nil {
			return nil, err
		}
		resp := &model.AgentRunToolCallResponse{
			ToolName:         toolName,
			ApprovalRequired: true,
			InteractionID:    interactionID,
			Content: []model.MCPContentItem{{
				Type: "text",
				Text: fmt.Sprintf("approval_required: Helpin approval is required before running %s", toolName),
			}},
		}
		_ = g.recordToolCall(ctx, state.run, toolName, def.Mutating, true, req.Input, resp, nil)
		return resp, nil
	}

	output, err := g.commandService.Execute(ctx, gatewayCommandContext(state.run), def.Name, req.Input)
	resp := &model.AgentRunToolCallResponse{ToolName: toolName}
	if err != nil {
		resp.IsError = true
		resp.Content = []model.MCPContentItem{{Type: "text", Text: err.Error()}}
		_ = g.recordToolCall(ctx, state.run, toolName, def.Mutating, false, req.Input, resp, err)
		return resp, nil
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		text = "{}"
	}
	resp.Content = []model.MCPContentItem{{Type: "text", Text: text}}
	_ = g.recordToolCall(ctx, state.run, toolName, def.Mutating, false, req.Input, resp, nil)
	return resp, nil
}

type runToolState struct {
	run   *model.AgentRun
	agent *model.Agent
}

func (g *AgentToolGateway) resolveRunToolState(ctx context.Context, token string) (*runToolState, error) {
	if g == nil || g.jwtManager == nil || g.runRepo == nil || g.agentRepo == nil || g.commandService == nil {
		return nil, fmt.Errorf("agent tool gateway is not configured")
	}
	claims, err := g.jwtManager.ValidateAgentRunToolToken(strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	run, err := g.runRepo.GetByID(ctx, claims.WorkspaceID, claims.RunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	if runStatusTerminal(run.Status) {
		return nil, fmt.Errorf("agent run is not active")
	}
	agent, err := g.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	return &runToolState{run: run, agent: agent}, nil
}

func (g *AgentToolGateway) callRuntimeTool(ctx context.Context, state *runToolState, toolName string, input json.RawMessage) (*model.AgentRunToolCallResponse, error) {
	if !effectiveGatewayTools(state.run, state.agent)[toolName] {
		return nil, fmt.Errorf("tool %q is not allowed for this run", toolName)
	}
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}

	registry := worker.NewToolRegistry(nil)
	execCtx := &worker.ExecutionContext{Context: ctx}
	output, err := registry.Execute(execCtx, toolName, input)
	resp := &model.AgentRunToolCallResponse{ToolName: toolName}
	if err != nil {
		resp.IsError = true
		resp.Content = []model.MCPContentItem{{Type: "text", Text: err.Error()}}
		_ = g.recordToolCall(ctx, state.run, toolName, false, false, input, resp, err)
		return resp, nil
	}
	if err := g.persistRuntimeToolSideEffects(ctx, state, toolName, input); err != nil {
		resp.IsError = true
		resp.Content = []model.MCPContentItem{{Type: "text", Text: err.Error()}}
		_ = g.recordToolCall(ctx, state.run, toolName, false, false, input, resp, err)
		return resp, nil
	}

	text := strings.TrimSpace(output)
	if text == "" {
		text = "{}"
	}
	resp.Content = []model.MCPContentItem{{Type: "text", Text: text}}
	_ = g.recordToolCall(ctx, state.run, toolName, false, false, input, resp, nil)
	return resp, nil
}

func (g *AgentToolGateway) commandDefinitionForTool(toolName string) (InternalCommandDefinition, bool) {
	for _, def := range g.commandService.ToolDefinitions() {
		if def.Tool != nil && def.Tool.Alias == toolName {
			return def, true
		}
	}
	return InternalCommandDefinition{}, false
}

func gatewayRuntimeToolEntries(allowed map[string]bool) []model.AgentRunToolCatalogEntry {
	if len(allowed) == 0 {
		return nil
	}
	registry := worker.NewToolRegistry(nil)
	entries := make([]model.AgentRunToolCatalogEntry, 0)
	for _, def := range registry.Definitions() {
		if !allowed[def.Name] || !gatewayExposesRuntimeTool(def.Name) {
			continue
		}
		entries = append(entries, model.AgentRunToolCatalogEntry{
			Name:         def.Name,
			Description:  def.Description,
			Category:     "Interaction",
			InputSchema:  def.InputSchema,
			Mutating:     false,
			ApprovalMode: "none",
		})
	}
	return entries
}

func gatewayExposesRuntimeTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case worker.ToolUpdatePlan,
		worker.ToolRequestUserInput,
		worker.ToolRequestApproval,
		worker.ToolRequestReviewCheckpoint,
		worker.ToolPublishPreview,
		worker.ToolPreviewMarkdown,
		worker.ToolPreviewJSON,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
		worker.ToolPublishTaskPlanDoc:
		return true
	default:
		return false
	}
}

func gatewayToolEntry(def InternalCommandDefinition) model.AgentRunToolCatalogEntry {
	meta := def.Tool
	return model.AgentRunToolCatalogEntry{
		Name:                 meta.Alias,
		Description:          meta.Description,
		Category:             meta.Category,
		InputSchema:          meta.InputSchema,
		Mutating:             def.Mutating,
		ApprovalMode:         gatewayApprovalMode(def),
		SupportedTargetTypes: append([]string(nil), def.SupportedTargetTypes...),
	}
}

func gatewayApprovalMode(def InternalCommandDefinition) string {
	if def.Mutating {
		return "helpin"
	}
	return "none"
}

func gatewayExposesCommand(def InternalCommandDefinition) bool {
	if !def.ExposesTool() {
		return false
	}
	switch strings.TrimSpace(def.Module) {
	case "workspace", "pm", "docs", "support", "crm", "git":
		return true
	default:
		return false
	}
}

func validateGatewayTarget(def InternalCommandDefinition, targetType string) error {
	if len(def.SupportedTargetTypes) == 0 || strings.TrimSpace(targetType) == "" {
		return nil
	}
	for _, supported := range def.SupportedTargetTypes {
		if supported == targetType {
			return nil
		}
	}
	return fmt.Errorf("tool %q does not support target type %q", def.Tool.Alias, targetType)
}

func effectiveGatewayTools(run *model.AgentRun, agent *model.Agent) map[string]bool {
	agentTools := parseJSONStringSlice(agent.AllowedTools)
	tools := agentTools
	var input model.AgentRunInputPayload
	if run != nil && len(run.Input) > 0 {
		_ = json.Unmarshal(run.Input, &input)
	}
	if len(input.AllowedTools) > 0 {
		agentSet := make(map[string]bool, len(agentTools))
		for _, tool := range agentTools {
			agentSet[strings.TrimSpace(tool)] = true
		}
		filtered := make([]string, 0, len(input.AllowedTools))
		for _, tool := range input.AllowedTools {
			tool = strings.TrimSpace(tool)
			if agentSet[tool] {
				filtered = append(filtered, tool)
			}
		}
		for _, tool := range agentTools {
			tool = strings.TrimSpace(tool)
			if gatewayExposesRuntimeTool(tool) && !slices.Contains(filtered, tool) {
				filtered = append(filtered, tool)
			}
		}
		tools = filtered
	}
	result := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool = strings.TrimSpace(tool); tool != "" {
			result[tool] = true
		}
	}
	return result
}

func gatewayRequiresApproval(agent *model.Agent) bool {
	if agent == nil {
		return true
	}
	switch strings.TrimSpace(agent.ApprovalMode) {
	case "", "never":
		return false
	default:
		return true
	}
}

func gatewayCommandContext(run *model.AgentRun) model.InternalCommandContext {
	actorID := run.AgentID
	if run.TriggeredByUserID != nil && strings.TrimSpace(*run.TriggeredByUserID) != "" {
		actorID = strings.TrimSpace(*run.TriggeredByUserID)
	}
	return model.InternalCommandContext{
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		AgentID:     run.AgentID,
		RunID:       run.ID,
		TargetType:  run.TargetType,
		TargetID:    run.TargetID,
	}
}

func runStatusTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func (g *AgentToolGateway) persistRuntimeToolSideEffects(ctx context.Context, state *runToolState, toolName string, input json.RawMessage) error {
	if state == nil || state.run == nil {
		return nil
	}
	invocation := model.ToolInvocation{ToolName: toolName, Input: input}
	switch toolName {
	case worker.ToolUpdatePlan:
		plan := worker.ExtractLatestRunPlan([]model.ToolInvocation{invocation})
		if plan == nil {
			return fmt.Errorf("update_plan did not produce a valid run plan")
		}
		if err := g.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeRunPlan, "json", plan); err != nil {
			return err
		}
		g.publishCodingSessionEvent(state.run, "plan.updated", map[string]any{
			"content": plan,
		})
		return nil
	case worker.ToolPublishPreview,
		worker.ToolPreviewMarkdown,
		worker.ToolPreviewJSON,
		worker.ToolPublishPRDDraft,
		worker.ToolPublishTaskPlan,
		worker.ToolPublishTaskPlanDoc:
		previews := worker.ExtractPublishedPreviews([]model.ToolInvocation{invocation})
		if len(previews) == 0 {
			return fmt.Errorf("%s did not produce a valid preview", toolName)
		}
		for _, preview := range previews {
			if err := g.appendRunArtifact(ctx, state.run, worker.RunPreviewArtifactType, "json", preview); err != nil {
				return err
			}
		}
	case worker.ToolRequestApproval:
		approval := worker.ExtractLatestApprovalRequest([]model.ToolInvocation{invocation})
		if approval == nil {
			return fmt.Errorf("request_approval did not produce a valid approval request")
		}
		if err := g.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", approval); err != nil {
			return err
		}
		_, err := g.createRuntimeApprovalInteraction(ctx, state, model.AgentRunInteractionKindApprovalRequest, approval.Title, approval.Summary, approval)
		return err
	case worker.ToolRequestReviewCheckpoint:
		review := worker.ExtractLatestReviewCheckpointRequest([]model.ToolInvocation{invocation})
		if review == nil {
			return fmt.Errorf("request_review_checkpoint did not produce a valid review checkpoint")
		}
		if err := g.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", review); err != nil {
			return err
		}
		_, err := g.createRuntimeApprovalInteraction(ctx, state, model.AgentRunInteractionKindReviewCheckpoint, review.Title, review.Summary, review)
		return err
	case worker.ToolRequestUserInput:
		userInput := worker.ExtractLatestHumanInputRequest([]model.ToolInvocation{invocation})
		if userInput == nil {
			return fmt.Errorf("request_user_input did not produce a valid input request")
		}
		if err := g.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeHumanInputRequest, "json", worker.HumanInputArtifactFromUserInputRequest(userInput)); err != nil {
			return err
		}
		_, err := g.createRuntimeUserInputInteraction(ctx, state, userInput)
		return err
	}
	return nil
}

func (g *AgentToolGateway) publishCodingSessionEvent(run *model.AgentRun, eventType string, payload map[string]any) {
	if g == nil || g.wsPublisher == nil || run == nil || strings.TrimSpace(eventType) == "" {
		return
	}
	eventID := fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano())
	envelope, _ := json.Marshal(model.CodingSessionEvent{
		ID:          eventID,
		SessionID:   run.ID,
		RunID:       run.ID,
		SequenceNo:  int(time.Now().UTC().UnixMilli()),
		Timestamp:   time.Now().UTC(),
		Type:        eventType,
		RuntimeKind: run.RuntimeKind,
		Payload:     payload,
	})
	g.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "coding_session_event",
		EntityID:    eventID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     gatewayActorID(run),
		ParentType:  "coding_session",
		ParentID:    run.ID,
		Data:        envelope,
	})
}

func gatewayActorID(run *model.AgentRun) string {
	if run == nil {
		return ""
	}
	if run.TriggeredByUserID != nil && strings.TrimSpace(*run.TriggeredByUserID) != "" {
		return strings.TrimSpace(*run.TriggeredByUserID)
	}
	return strings.TrimSpace(run.AgentID)
}

func (g *AgentToolGateway) appendRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any) error {
	if g.artifactRepo == nil || run == nil {
		return nil
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	seq, err := g.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	text := string(content)
	return g.artifactRepo.Create(ctx, &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: &text,
		Metadata:      json.RawMessage(`{"source":"agent_tool_gateway"}`),
		SequenceNo:    seq,
	})
}

func (g *AgentToolGateway) createRuntimeApprovalInteraction(ctx context.Context, state *runToolState, interactionKind, title, summary string, requestPayload any) (string, error) {
	if g.interactionRepo == nil || state == nil || state.run == nil {
		return "", nil
	}
	payload, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("marshal approval request: %w", err)
	}
	interaction := &model.AgentRunInteraction{
		WorkspaceID:          state.run.WorkspaceID,
		RunID:                state.run.ID,
		RuntimeKind:          state.run.RuntimeKind,
		InteractionKind:      strings.TrimSpace(interactionKind),
		Status:               model.AgentRunInteractionStatusPending,
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		Title:                gatewayStringPtr(title),
		Summary:              gatewayStringPtr(summary),
		RequestPayload:       payload,
		RuntimeMetadata:      json.RawMessage(`{"source":"agent_tool_gateway"}`),
	}
	if err := g.interactionRepo.Create(ctx, interaction); err != nil {
		return "", err
	}
	return interaction.ID, nil
}

func (g *AgentToolGateway) createRuntimeUserInputInteraction(ctx context.Context, state *runToolState, req *worker.UserInputRequest) (string, error) {
	if g.interactionRepo == nil || state == nil || state.run == nil || req == nil {
		return "", nil
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal input request: %w", err)
	}
	title := "User input required"
	interaction := &model.AgentRunInteraction{
		WorkspaceID:          state.run.WorkspaceID,
		RunID:                state.run.ID,
		RuntimeKind:          state.run.RuntimeKind,
		InteractionKind:      model.AgentRunInteractionKindRequestUserInput,
		Status:               model.AgentRunInteractionStatusPending,
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionCodexV2,
		Title:                &title,
		Summary:              gatewayStringPtr(worker.UserInputSummary(req)),
		RequestPayload:       payload,
		RuntimeMetadata:      json.RawMessage(`{"source":"agent_tool_gateway"}`),
	}
	if err := g.interactionRepo.Create(ctx, interaction); err != nil {
		return "", err
	}
	return interaction.ID, nil
}

func gatewayStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func (g *AgentToolGateway) createToolApprovalInteraction(ctx context.Context, state *runToolState, def InternalCommandDefinition, input json.RawMessage) (string, error) {
	if g.interactionRepo == nil {
		return "", fmt.Errorf("interaction repository is not configured")
	}
	title := "Approve tool call"
	summary := fmt.Sprintf("Approve %s for this agent run.", def.Tool.Alias)
	payload, _ := json.Marshal(map[string]any{
		"tool_name": def.Tool.Alias,
		"mutating":  def.Mutating,
		"input":     json.RawMessage(input),
	})
	interaction := &model.AgentRunInteraction{
		WorkspaceID:          state.run.WorkspaceID,
		RunID:                state.run.ID,
		RuntimeKind:          state.run.RuntimeKind,
		InteractionKind:      model.AgentRunInteractionKindApprovalRequest,
		Status:               model.AgentRunInteractionStatusPending,
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		Title:                &title,
		Summary:              &summary,
		RequestPayload:       payload,
		RuntimeMetadata:      json.RawMessage(`{"source":"agent_tool_gateway"}`),
	}
	if err := g.interactionRepo.Create(ctx, interaction); err != nil {
		return "", err
	}
	return interaction.ID, nil
}

func (g *AgentToolGateway) recordToolCall(ctx context.Context, run *model.AgentRun, toolName string, mutating, approvalRequired bool, input json.RawMessage, resp *model.AgentRunToolCallResponse, callErr error) error {
	if g.artifactRepo == nil || run == nil {
		return nil
	}
	seq, err := g.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	status := "ok"
	errMsg := ""
	if approvalRequired {
		status = "approval_required"
	}
	if callErr != nil {
		status = "error"
		errMsg = callErr.Error()
	}
	payload, _ := json.Marshal(map[string]any{
		"tool_name":         toolName,
		"mutating":          mutating,
		"approval_required": approvalRequired,
		"status":            status,
		"error":             errMsg,
		"input":             json.RawMessage(input),
		"response":          resp,
		"called_at":         time.Now().UTC(),
	})
	content := string(payload)
	return g.artifactRepo.Create(ctx, &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeToolCall,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage(`{"source":"agent_tool_gateway"}`),
		SequenceNo:    seq,
	})
}
