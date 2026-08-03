package service

// support.send_reply / support.escalate_to_human: the only ways a support
// chat run's output reaches the visitor. send_reply re-validates the agent's
// grounding server-side (claims vs the run's evidence snapshot, numeric
// checks, confidence threshold, satisfaction trend) and converts failures
// into the existing escalation machinery — a hallucinated answer cannot reach
// a customer regardless of what the model produced.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	supportReplyKindConversational = "conversational"

	supportReplyModelLabel = "agent-runtime"
)

// SetSupportReplyDependencies wires the collaborators used by the
// send_reply/escalate tools.
func (s *InternalCommandService) SetSupportReplyDependencies(supportAI *SupportAIService, processingRepo *repository.AIMessageProcessingRepository, meter *AIUsageMeter) {
	s.supportAIService = supportAI
	s.supportProcessingRepo = processingRepo
	s.supportUsageMeter = meter
}

// supportReplyGateInput feeds the pure server-side reply gate.
type supportReplyGateInput struct {
	Kind      string
	Contract  *AIResponseContract
	Evidence  []KnowledgeSearchResult
	History   []model.SupportMessage
	Threshold float64
}

// supportReplyGateResult is the gate's verdict.
type supportReplyGateResult struct {
	OK                bool
	Confidence        float64
	EscalationReason  string
	ValidationOutcome string
}

// evaluateSupportReplyGate re-validates an agent-produced reply exactly the
// way the pipeline validated its own answers: evidence-grounded claims and
// numeric matching for answers, a weighted confidence score vs the workspace
// threshold, and the declining-satisfaction trend across recent AI turns.
func evaluateSupportReplyGate(input supportReplyGateInput) supportReplyGateResult {
	kind := normalizeSupportReplyKind(input.Kind)
	result := supportReplyGateResult{ValidationOutcome: supportValidationPass}

	if kind == supportReplyKindAnswer {
		validation := validateSupportAnswer(SupportQueryPlanContract{}, supportEvidenceCoverage{}, input.Evidence, input.Contract)
		result.ValidationOutcome = validation.Outcome
		if validation.Outcome != supportValidationPass {
			result.EscalationReason = "answer_validation_" + validation.Outcome
			return result
		}
	}

	confidence := evaluateConfidence(input.Evidence, input.Contract, kind != supportReplyKindAnswer)
	result.Confidence = confidence
	if kind == supportReplyKindAnswer && confidence < input.Threshold {
		result.EscalationReason = "low_confidence"
		return result
	}

	if signal := evaluatePostAnswerEscalation(input.History, confidence); signal != nil {
		result.EscalationReason = signal.Reason
		return result
	}

	result.OK = true
	return result
}

func normalizeSupportReplyKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case supportReplyKindClarify:
		return supportReplyKindClarify
	case supportReplyKindConversational:
		return supportReplyKindConversational
	case supportReplyKindConfirm:
		return supportReplyKindConfirm
	default:
		return supportReplyKindAnswer
	}
}

func (s *InternalCommandService) registerSupportReplyCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "support.send_reply",
		Module:               "support",
		Mutating:             true,
		SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.send_reply",
			Alias:       "send_support_reply",
			Category:    "Support",
			Description: "Send your reply to the visitor. For factual answers you MUST first call search_knowledge and cite the evidence_id values that support each material claim — the server re-validates grounding and confidence, and hands the conversation to a human if validation fails. Call exactly once per visitor message, as your final action of the turn.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content":    map[string]any{"type": "string", "description": "The reply text shown to the visitor."},
					"reply_kind": map[string]any{"type": "string", "enum": []string{"answer", "clarify", "conversational", "confirmation"}, "description": "answer = factual answer needing evidence; clarify = asking the visitor a question; conversational = greeting/small talk; confirmation = confirming the visitor's issue is resolved."},
					"confidence": map[string]any{"type": "number", "description": "Your 0-1 confidence that the reply is correct and grounded."},
					"source_doc_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "evidence_id values (from search_knowledge) backing the reply."},
					"claims": map[string]any{
						"type":        "array",
						"description": "Each material factual claim in the reply mapped to the evidence ids that support it. Required for reply_kind=answer.",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"text":         map[string]any{"type": "string"},
								"evidence_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							},
							"required":             []string{"text", "evidence_ids"},
							"additionalProperties": false,
						},
					},
					"resolves_conversation": map[string]any{"type": "boolean", "description": "True only when the visitor confirmed their issue is resolved."},
				},
				"required":             []string{"content", "reply_kind", "confidence"},
				"additionalProperties": false,
			},
		},
		Execute: s.executeSupportSendReply,
	})

	s.register(InternalCommandDefinition{
		Name:                 "support.escalate_to_human",
		Module:               "support",
		Mutating:             true,
		SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.escalate_to_human",
			Alias:       "escalate_to_human",
			Category:    "Support",
			Description: "Hand the conversation to a human teammate. Use when the visitor asks for a human, the request is risky (billing disputes, account deletion, legal), or you cannot answer from the knowledge base. The server sends the availability-aware handoff message — after calling this, end your turn without sending another reply.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reason":        map[string]any{"type": "string", "description": "Short machine reason, e.g. customer_requested, out_of_scope, risky_request, cannot_answer."},
					"issue_key":     map[string]any{"type": "string", "description": "Optional stable key for the visitor's issue."},
					"issue_summary": map[string]any{"type": "string", "description": "Optional one-line summary for the teammate."},
				},
				"required":             []string{"reason"},
				"additionalProperties": false,
			},
		},
		Execute: s.executeSupportEscalate,
	})
}

func (s *InternalCommandService) executeSupportSendReply(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	supportAI := s.supportAIService
	if supportAI == nil {
		return nil, fmt.Errorf("support reply service is not configured")
	}
	conversationID := commandConversationTargetID(meta)
	if conversationID == "" {
		return nil, fmt.Errorf("send_reply requires a support conversation target")
	}
	var req struct {
		Content              string            `json:"content"`
		ReplyKind            string            `json:"reply_kind"`
		Confidence           float64           `json:"confidence"`
		SourceDocIDs         []string          `json:"source_doc_ids"`
		Claims               []AIResponseClaim `json:"claims"`
		ResolvesConversation bool              `json:"resolves_conversation"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse send reply input: %w", err)
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	conv, err := supportAI.conversationRepo.GetByID(ctx, meta.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	// The kill-switch wins even mid-turn: a human took over while the agent
	// was thinking, so the reply is suppressed, not published.
	if (conv.HumanTakeover != nil && *conv.HumanTakeover) || conv.CustomerRequestedHumanAt != nil || derefString(conv.AIState) == "escalated" {
		s.settleSupportTurn(ctx, meta.WorkspaceID, conversationID, nil)
		return mustJSON(map[string]any{
			"status":      "suppressed",
			"next_action": "A human owns this conversation now. End your turn without sending anything.",
		}), nil
	}

	settings, err := supportAI.loadSettings(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	agentID := strings.TrimSpace(derefString(settings.AIAgentID))
	if agentID == "" {
		return nil, fmt.Errorf("no support AI agent is configured")
	}

	// Rebuild the evidence the agent actually saw this run.
	var evidence []KnowledgeSearchResult
	if run, runErr := s.resolveCommandRun(ctx, meta); runErr == nil && run != nil && s.supportRunEvidenceRepo != nil {
		rows, listErr := s.supportRunEvidenceRepo.ListByRun(ctx, meta.WorkspaceID, run.ID)
		if listErr != nil {
			slog.WarnContext(ctx, "send_reply: load run evidence failed", "error", listErr, "run_id", run.ID)
		}
		for _, row := range rows {
			evidence = append(evidence, KnowledgeSearchResult{
				ID:          row.EvidenceID,
				ReferenceID: row.ReferenceID,
				SourceType:  row.SourceType,
				SourceID:    row.SourceID,
				DocumentID:  row.DocumentID,
				Title:       row.Title,
				URL:         row.URL,
				IsInternal:  row.IsInternal,
				Content:     row.Content,
			})
		}
	}

	history, err := supportAI.messageRepo.ListByConversation(ctx, meta.WorkspaceID, conversationID, false)
	if err != nil {
		history = nil
	}

	contract := &AIResponseContract{
		Content:      content,
		CanAnswer:    true,
		SourceDocIDs: req.SourceDocIDs,
		Confidence:   req.Confidence,
		Claims:       req.Claims,
	}
	gate := evaluateSupportReplyGate(supportReplyGateInput{
		Kind:      req.ReplyKind,
		Contract:  contract,
		Evidence:  evidence,
		History:   sanitizeConversationHistory(history, ""),
		Threshold: settings.AIConfidenceThreshold,
	})
	if !gate.OK {
		sourceMessageID := s.supportTurnSourceMessageID(ctx, meta.WorkspaceID, conversationID, history)
		if escErr := supportAI.EscalateToHumanForMessageWithIssue(ctx, meta.WorkspaceID, conversationID, sourceMessageID, gate.EscalationReason, "", ""); escErr != nil {
			slog.ErrorContext(ctx, "send_reply: escalate after gate failure failed",
				"error", escErr, "workspace_id", meta.WorkspaceID, "conversation_id", conversationID, "reason", gate.EscalationReason)
		}
		s.settleSupportTurn(ctx, meta.WorkspaceID, conversationID, nil)
		supportAI.publishTypingIndicator(ctx, meta.WorkspaceID, conversationID, false)
		return mustJSON(map[string]any{
			"status":      "escalated",
			"reason":      gate.EscalationReason,
			"next_action": "The reply did not pass server validation and the conversation was handed to a human. End your turn without sending anything else.",
		}), nil
	}

	kind := normalizeSupportReplyKind(req.ReplyKind)
	// Claims cite chunk ids; AISources are keyed by reference ids — translate.
	referenceByID := make(map[string]string, len(evidence))
	for _, item := range evidence {
		referenceByID[item.ID] = item.ReferenceID
	}
	sourceRefIDs := make([]string, 0, len(req.SourceDocIDs))
	for _, id := range req.SourceDocIDs {
		if ref, ok := referenceByID[id]; ok && ref != "" {
			sourceRefIDs = append(sourceRefIDs, ref)
		}
	}
	sources := buildAISources(sourceRefIDs, evidence)
	var message *model.SupportMessage
	if shouldCreatePublicSupportAIReply(*settings) {
		message, err = supportAI.publishAIReply(ctx, meta.WorkspaceID, conversationID, agentID, content, supportReplyModelLabel, 0, gate.Confidence, sources, kind, "", "", "", conv.CustomerEmail, conv.CustomerPhone)
	} else {
		message, err = supportAI.publishAIInternalNote(ctx, meta.WorkspaceID, conversationID, agentID, "Suggested reply:\n\n"+content, supportReplyModelLabel, 0, gate.Confidence, sources, kind, "", "", "", conv.CustomerEmail, conv.CustomerPhone)
	}
	if err != nil {
		return nil, fmt.Errorf("publish reply: %w", err)
	}

	if req.ResolvesConversation && kind == supportReplyKindConfirm {
		resolved := "resolved"
		confirmed := "confirmed"
		if updateErr := supportAI.conversationRepo.UpdateFields(ctx, meta.WorkspaceID, conversationID, map[string]any{
			"ai_state":           &resolved,
			"ai_resolution_type": &confirmed,
			"flow_state":         model.SupportConversationFlowStateResolvedByAI,
		}); updateErr != nil {
			slog.WarnContext(ctx, "send_reply: mark resolved failed", "error", updateErr, "conversation_id", conversationID)
		}
	}

	triggerMessageID := s.supportTurnSourceMessageID(ctx, meta.WorkspaceID, conversationID, history)
	s.settleSupportTurn(ctx, meta.WorkspaceID, conversationID, &message.ID)
	s.consumeSupportReplyBilling(ctx, meta.WorkspaceID, conversationID, message.ID)
	// Answer trace feeds coverage analytics (the retrieval trace was emitted
	// by search_knowledge; this one records the delivered outcome).
	supportAI.recordSupportAIAnswerTrace(ctx, meta.WorkspaceID, conversationID, message.ID,
		triggerMessageID, agentID, SupportQueryPlanContract{}, evidence, contract,
		supportEvidenceCoverage{}, supportAnswerValidation{Outcome: gate.ValidationOutcome},
		gate.Confidence, kind, "runtime_tool", false, nil)
	supportAI.publishTypingIndicator(ctx, meta.WorkspaceID, conversationID, false)

	return mustJSON(map[string]any{
		"status":      "sent",
		"message_id":  message.ID,
		"confidence":  gate.Confidence,
		"next_action": "Reply delivered. End your turn now; do not call send_support_reply again until the visitor responds.",
	}), nil
}

func (s *InternalCommandService) executeSupportEscalate(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	supportAI := s.supportAIService
	if supportAI == nil {
		return nil, fmt.Errorf("support reply service is not configured")
	}
	conversationID := commandConversationTargetID(meta)
	if conversationID == "" {
		return nil, fmt.Errorf("escalate_to_human requires a support conversation target")
	}
	var req struct {
		Reason       string `json:"reason"`
		IssueKey     string `json:"issue_key"`
		IssueSummary string `json:"issue_summary"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse escalate input: %w", err)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "agent_requested"
	}
	sourceMessageID := s.supportTurnSourceMessageID(ctx, meta.WorkspaceID, conversationID, nil)
	if err := supportAI.EscalateToHumanForMessageWithIssue(ctx, meta.WorkspaceID, conversationID, sourceMessageID, reason, strings.TrimSpace(req.IssueKey), strings.TrimSpace(req.IssueSummary)); err != nil {
		return nil, fmt.Errorf("escalate: %w", err)
	}
	s.settleSupportTurn(ctx, meta.WorkspaceID, conversationID, nil)
	supportAI.publishTypingIndicator(ctx, meta.WorkspaceID, conversationID, false)
	return mustJSON(map[string]any{
		"status":      "escalated",
		"next_action": "The conversation was handed to a human with an availability-aware message. End your turn without sending anything else.",
	}), nil
}

// settleSupportTurn marks the triggering visitor message's processing row
// completed — the signal that this turn produced its one outcome.
func (s *InternalCommandService) settleSupportTurn(ctx context.Context, workspaceID, conversationID string, replyMessageID *string) {
	if s.supportProcessingRepo == nil {
		return
	}
	row, err := s.supportProcessingRepo.LatestProcessingForConversation(ctx, workspaceID, conversationID)
	if err != nil || row == nil {
		return
	}
	if err := s.supportProcessingRepo.MarkCompleted(ctx, row.ID, replyMessageID, 0); err != nil {
		slog.WarnContext(ctx, "settle support turn failed", "error", err, "conversation_id", conversationID)
	}
}

// supportTurnSourceMessageID finds the message the current turn responds to:
// the in-flight processing row's source message, else the latest customer
// message.
func (s *InternalCommandService) supportTurnSourceMessageID(ctx context.Context, workspaceID, conversationID string, history []model.SupportMessage) string {
	if s.supportProcessingRepo != nil {
		if row, err := s.supportProcessingRepo.LatestProcessingForConversation(ctx, workspaceID, conversationID); err == nil && row != nil {
			return row.SourceMessageID
		}
	}
	if history == nil && s.supportAIService != nil {
		history, _ = s.supportAIService.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].SenderType == "customer" {
			return history[i].ID
		}
	}
	return ""
}

// consumeSupportReplyBilling charges the support_ai_reply floor for a
// delivered reply (token usage itself is billed by the run's usage
// checkpoints).
func (s *InternalCommandService) consumeSupportReplyBilling(ctx context.Context, workspaceID, conversationID, messageID string) {
	if s.supportUsageMeter == nil {
		return
	}
	if _, err := s.supportUsageMeter.Consume(ctx, AIUsageMeterInput{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureSupportAIReply,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, BillingFeatureSupportAIReply, conversationID, messageID),
		Metadata: map[string]interface{}{
			"conversation_id": conversationID,
			"message_id":      messageID,
			"origin":          "runtime_tool",
		},
	}); err != nil {
		slog.WarnContext(ctx, "support reply billing failed", "error", err, "conversation_id", conversationID)
	}
}
