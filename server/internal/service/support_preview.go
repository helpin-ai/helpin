package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const supportPreviewTarget = "support_preview"

var supportPreviewTools = []string{"search_knowledge", "send_support_reply", "escalate_to_human", "list_conversation_messages", "get_support_conversation"}

// SupportPreviewService launches ordinary, metered Runtime runs with an isolated
// target. No live conversation, processing attempt, or visitor message is created.
type SupportPreviewService struct {
	agents   *AgentService
	ai       *SupportAIService
	evidence *repository.SupportRunEvidenceRepository
}

func NewSupportPreviewService(agents *AgentService, ai *SupportAIService, evidence *repository.SupportRunEvidenceRepository) *SupportPreviewService {
	return &SupportPreviewService{agents: agents, ai: ai, evidence: evidence}
}

func supportPreviewSnapshot(run *model.AgentRun) (*model.SupportPreviewSnapshot, error) {
	if run == nil || run.TargetType != supportPreviewTarget {
		return nil, nil
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return nil, err
	}
	if input.SupportPreview == nil || input.Trigger == nil || input.Trigger.Source != model.AgentRunTriggerSourceSystem || input.Trigger.TriggerType != supportPreviewTarget {
		return nil, fmt.Errorf("invalid support preview run")
	}
	return input.SupportPreview, nil
}

func (s *SupportPreviewService) Start(ctx context.Context, workspaceID, agentID string, req model.SupportAIPreviewRequest) (*model.SupportAIPreviewResponse, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("%w: workspace actor required", ErrSupportPreviewInvalidInput)
	}
	message := strings.TrimSpace(req.Message)
	if message == "" || len(message) > 16000 || len(req.History) > 100 {
		return nil, fmt.Errorf("%w: provide a message up to 16000 bytes and at most 100 history turns", ErrSupportPreviewInvalidInput)
	}
	for _, turn := range req.History {
		if len(turn.Content) > 16000 || !slices.Contains([]string{"customer", "ai", "agent", "user"}, turn.SenderType) {
			return nil, fmt.Errorf("%w: invalid history turn", ErrSupportPreviewInvalidInput)
		}
	}
	agent, err := s.agents.requireRunnableAgent(ctx, workspaceID, agentID, "support_conversation")
	if err != nil {
		return nil, err
	}
	history, source, err := s.ai.resolvePreviewHistory(ctx, workspaceID, req)
	if err != nil {
		return nil, err
	}
	settings, err := s.ai.loadSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	// Copy text only: never copy sessions, customer identifiers, attachment URLs,
	// or internal notes into the preview snapshot.
	turns := make([]model.SupportMessage, 0, len(history))
	for _, m := range history {
		turn := model.SupportMessage{SenderType: m.SenderType, MessageType: m.MessageType, Content: supportMessagePromptText(m)}
		// Preserve only the gate's confidence metadata, never arbitrary customer data.
		if confidence, ok := parseAIConfidence(m.Metadata); ok {
			metadata, _ := json.Marshal(map[string]any{"ai_auto_reply": true, "ai_confidence": confidence})
			turn.Metadata = string(metadata)
		}
		turns = append(turns, turn)
	}
	snapshot := &model.SupportPreviewSnapshot{History: turns, Message: message, ConversationSource: source, ConfidenceThreshold: settings.AIConfidenceThreshold, MaxResults: normalizePreviewMaxResults(req.MaxResults)}
	if req.IncludeAnswer == nil || *req.IncludeAnswer {
		if reason := checkHardEscalation(message); reason != "" {
			return &model.SupportAIPreviewResponse{Status: "completed", ConversationSource: source, ConfidenceThreshold: settings.AIConfidenceThreshold, FinalDecision: "handoff", FinalReason: reason}, nil
		}
	}
	if req.IncludeAnswer != nil && !*req.IncludeAnswer {
		results, err := s.ai.searchSupportKnowledge(ctx, workspaceID, agent.ID, "", previewMessages(snapshot), nil)
		if err != nil {
			return nil, err
		}
		if len(results) > snapshot.MaxResults {
			results = results[:snapshot.MaxResults]
		}
		return &model.SupportAIPreviewResponse{Status: "completed", ConversationSource: source, ConfidenceThreshold: settings.AIConfidenceThreshold, FinalDecision: "retrieval_only", FinalReason: "knowledge_search_only", Retrieval: model.SupportAIPreviewRetrieval{QueryCount: 1, ResultCount: len(results), Results: previewSearchResults(results)}}, nil
	}
	tools, excluded, err := supportPreviewAllowedTools(agent)
	if err != nil {
		return nil, err
	}
	snapshot.ExcludedTools = excluded
	id := uuid.NewString()
	trigger := &model.AgentRunTriggerContext{Source: model.AgentRunTriggerSourceSystem, TriggerType: supportPreviewTarget}
	input := model.AgentRunInputPayload{SupportPreview: snapshot, Trigger: trigger, Target: &model.AgentRunTargetContext{TargetType: supportPreviewTarget, TargetID: id}, AllowedTools: tools, AdditionalContext: message}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	run, err := s.agents.createRun(ctx, createRunParams{runID: id, workspaceID: workspaceID, agent: agent, targetType: supportPreviewTarget, targetID: id, actorID: &actor.UserID, trigger: trigger, input: encoded, invocationMode: model.InvocationModeInteractive})
	if err != nil {
		return nil, err
	}
	response := supportPreviewResponse(run, snapshot)
	response.ExcludedTools = excluded
	return response, nil
}

func supportPreviewAllowedTools(agent *model.Agent) ([]string, []string, error) {
	var tools, excluded []string
	for _, name := range agentcontract.NormalizeToolNames(parseJSONStringSlice(agent.AllowedTools)) {
		if slices.Contains(supportPreviewTools, name) {
			tools = append(tools, name)
		} else {
			excluded = append(excluded, name)
		}
	}
	for _, required := range []string{"search_knowledge", "send_support_reply", "escalate_to_human"} {
		if !slices.Contains(tools, required) {
			return nil, nil, fmt.Errorf("support preview requires agent tool %s", required)
		}
	}
	return tools, excluded, nil
}

func previewMessages(snapshot *model.SupportPreviewSnapshot) []model.SupportMessage {
	return append(append([]model.SupportMessage(nil), snapshot.History...), model.SupportMessage{SenderType: "customer", MessageType: "reply", Content: snapshot.Message})
}

func supportPreviewResponse(run *model.AgentRun, snapshot *model.SupportPreviewSnapshot) *model.SupportAIPreviewResponse {
	response := &model.SupportAIPreviewResponse{RunID: run.ID, Status: run.Status, ConversationSource: snapshot.ConversationSource, ConfidenceThreshold: snapshot.ConfidenceThreshold, FinalDecision: "pending", Retrieval: model.SupportAIPreviewRetrieval{Results: []model.SupportAIPreviewSearchResult{}}}
	var input model.AgentRunInputPayload
	_ = json.Unmarshal(run.Input, &input)
	response.ExcludedTools = snapshot.ExcludedTools
	response.Provider = input.ModelProvider
	response.Model = input.ModelName
	if input.AISelection != nil {
		response.ProfileID = input.AISelection.ProfileID
	}
	return response
}

func (s *SupportPreviewService) Get(ctx context.Context, workspaceID, agentID, runID string) (*model.SupportAIPreviewResponse, error) {
	run, err := s.agents.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	actor := authorization.GetActor(ctx)
	if run == nil || run.TargetType != supportPreviewTarget || run.AgentID != agentID || actor == nil || actor.WorkspaceID != workspaceID || derefString(run.TriggeredByUserID) != actor.UserID {
		return nil, ErrSupportPreviewConversationNotFound
	}
	snapshot, err := supportPreviewSnapshot(run)
	if err != nil {
		return nil, err
	}
	response := supportPreviewResponse(run, snapshot)
	artifact, err := s.agents.artifactRepo.GetByIDAndWorkspace(ctx, workspaceID, supportPreviewResultID(run.ID))
	if err != nil {
		return nil, err
	}
	if artifact != nil && artifact.InlineContent != nil {
		if err = json.Unmarshal([]byte(*artifact.InlineContent), response); err != nil {
			return nil, err
		}
		response.Status = "completed"
	}
	rows, err := s.aiPreviewEvidence(ctx, workspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	if usage, ok := usageFromRuntimeOutputSummary(run.OutputSummary); ok {
		response.TotalTokensUsed = usage.TotalTokens
	}
	if artifact == nil {
		response.Retrieval.Results = previewSearchResults(rows)
		response.Retrieval.ResultCount = len(rows)
	}
	if response.FinalDecision == "pending" && !model.IsAgentRunActiveStatus(run.Status) {
		response.FinalDecision = "failed"
		response.FinalReason = "runtime_finished_without_support_outcome"
	}
	if response.FinalDecision == "pending" && model.IsAgentRunPausedStatus(run.Status) {
		response.FinalDecision = "blocked"
		response.FinalReason = "runtime_requires_interaction"
	}
	if time.Since(run.CreatedAt) > 3*time.Minute && response.FinalDecision == "pending" {
		response.FinalDecision = "failed"
		response.FinalReason = "preview_timed_out"
	}
	if response.FinalDecision != "pending" && model.IsAgentRunActiveStatus(run.Status) {
		// Stop as soon as an outcome is captured. Cancellation uses the normal
		// runtime lifecycle, including credential cleanup and usage settlement.
		if _, err = s.agents.CancelRun(ctx, workspaceID, run.ID, actor.UserID); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *SupportPreviewService) aiPreviewEvidence(ctx context.Context, workspaceID, runID string) ([]KnowledgeSearchResult, error) {
	// Use the same durable evidence repository as send_support_reply.
	repo := s.evidence
	if repo == nil {
		return nil, fmt.Errorf("preview evidence repository unavailable")
	}
	rows, err := repo.ListByRun(ctx, workspaceID, runID)
	return supportEvidenceFromRows(rows), err
}

func supportPreviewResultID(runID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("support-preview:"+runID)).String()
}

func (s *SupportPreviewService) Cancel(ctx context.Context, workspaceID, agentID, runID string) error {
	run, err := s.agents.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return err
	}
	actor := authorization.GetActor(ctx)
	if run == nil || run.TargetType != supportPreviewTarget || run.AgentID != agentID || actor == nil || actor.WorkspaceID != workspaceID || derefString(run.TriggeredByUserID) != actor.UserID {
		return ErrSupportPreviewConversationNotFound
	}
	if !model.IsAgentRunActiveStatus(run.Status) {
		return nil
	}
	_, err = s.agents.CancelRun(ctx, workspaceID, runID, actor.UserID)
	return err
}

// Preview text may come from a restricted mailbox. Generic run surfaces must
// enforce the same owner boundary as the dedicated preview endpoint.
func requireSupportPreviewReader(ctx context.Context, run *model.AgentRun) error {
	if run == nil || run.TargetType != supportPreviewTarget {
		return nil
	}
	actor := authorization.GetActor(ctx)
	if actor == nil || actor.WorkspaceID != run.WorkspaceID || actor.UserID != derefString(run.TriggeredByUserID) {
		return ErrSupportPreviewConversationNotFound
	}
	return nil
}
func (s *AgentService) requireSupportPreviewReader(ctx context.Context, workspaceID, runID string) error {
	if s.runRepo == nil {
		return nil
	}
	run, err := s.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return err
	}
	return requireSupportPreviewReader(ctx, run)
}
