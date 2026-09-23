package service

// support.send_reply / support.escalate_to_human: the only ways a support
// chat run's output reaches the visitor. send_reply re-validates the agent's
// grounding server-side (claims vs the run's evidence snapshot, numeric
// checks and confidence threshold) and converts failures
// into the existing escalation machinery — a hallucinated answer cannot reach
// a customer regardless of what the model produced.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
// threshold. Scores from different turns are not customer satisfaction signals;
// conversation-level handoff decisions belong to the support lifecycle.
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
			Description: "Send your reply to the visitor. For factual answers you MUST first call search_knowledge and cite the evidence_id values that support each material claim — the server re-validates grounding and confidence. When child work is pending, a conversational acknowledgment keeps the customer turn open; wait for the child result and then send the final answer. Otherwise this must be the final successful action of the turn. If the tool returns rewrite_required, rewrite once in customer-facing language and call it again.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content":        map[string]any{"type": "string", "description": "The reply text shown to the visitor."},
					"reply_kind":     map[string]any{"type": "string", "enum": []string{"answer", "clarify", "conversational", "confirmation"}, "description": "answer = factual answer needing evidence; clarify = asking the visitor a question; conversational = greeting/small talk or a brief acknowledgment while child work is pending; confirmation = confirming the visitor's issue is resolved."},
					"confidence":     map[string]any{"type": "number", "description": "Your 0-1 confidence that the reply is correct and grounded."},
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
			Description: "Hand the conversation to a human teammate. Use when the visitor asks for a human, the request is risky (billing disputes, account deletion, legal), or the permitted research fallback cannot produce a grounded answer. The server sends the availability-aware handoff message — after calling this, end your turn without sending another reply.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reason":               map[string]any{"type": "string", "description": "Short machine reason, e.g. customer_requested, out_of_scope, risky_request, cannot_answer."},
					"issue_key":            map[string]any{"type": "string", "description": "Optional stable key for the visitor's issue."},
					"issue_summary":        map[string]any{"type": "string", "description": "Optional one-line summary for the teammate."},
					"attempted_steps":      map[string]any{"type": "array", "maxItems": 5, "items": map[string]any{"type": "string", "maxLength": 700}, "description": "Brief factual steps and results. Distinguish AI suggestions from customer-confirmed actions. Do not invent completed actions."},
					"unresolved_questions": map[string]any{"type": "array", "maxItems": 5, "items": map[string]any{"type": "string", "maxLength": 700}, "description": "What remains unanswered or requires a human. Internal only; do not repeat sensitive credentials."},
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
	if conversationID == "" && meta.TargetType != supportPreviewTarget {
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
		return nil, errCommandInput("content is required")
	}
	if disclosures := supportReplyInternalProcessDisclosures(content); len(disclosures) > 0 {
		return mustJSON(map[string]any{
			"status":      "rewrite_required",
			"reason":      "internal_process_disclosure",
			"violations":  disclosures,
			"next_action": "Rewrite once as a direct customer-facing answer without mentioning searches, evidence, tools, agents, repositories, confidence machinery, or other internal process; then call send_support_reply again.",
		}), nil
	}

	if meta.TargetType == supportPreviewTarget {
		return s.captureSupportPreviewReply(ctx, meta, req.ReplyKind, &AIResponseContract{Content: content, CanAnswer: true, SourceDocIDs: req.SourceDocIDs, Confidence: req.Confidence, Claims: req.Claims})
	}

	conv, err := supportAI.conversationRepo.GetByID(ctx, meta.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if conv == nil {
		return nil, errCommandNotFound("conversation")
	}
	if conv.AnonymizedAt != nil {
		return mustJSON(map[string]any{"status": "suppressed", "next_action": "This customer was deleted. The conversation is read-only. End your turn."}), nil
	}

	var replyRunID string
	if meta.RunID != "" {
		run, err := s.resolveCommandRun(ctx, meta)
		if err != nil {
			return nil, err
		}
		if run != nil {
			replyRunID = run.ID
		}
		if run == nil || (run.ID != derefString(conv.AIActiveRunID) && (conv.AIControlVersion > 0 || derefString(conv.AIActiveRunID) != "")) {
			return mustJSON(map[string]any{"status": "suppressed", "next_action": "A newer run owns this conversation. End your turn."}), nil
		}
	}
	settings, err := supportAI.loadSettings(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if !supportAIConversationSupported(conv) {
		s.closeEscalatedSupportCommandRun(ctx, meta)
		return mustJSON(map[string]any{"status": "suppressed", "next_action": "AI replies are not enabled for this message channel. End your turn."}), nil
	}
	// The kill-switch wins even mid-turn: a human took over while the agent
	// was thinking, so the reply is suppressed, not published.
	if model.SupportAIConversationBlocked(conv) {
		s.closeEscalatedSupportCommandRun(ctx, meta)
		return mustJSON(map[string]any{
			"status":      "suppressed",
			"next_action": "A human owns this conversation now. End your turn without sending anything.",
		}), nil
	}

	if s.supportProcessingRepo == nil {
		return nil, fmt.Errorf("support processing repository is not configured")
	}
	turn, err := s.supportProcessingRepo.LatestProcessingForConversation(ctx, meta.WorkspaceID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("load support turn: %w", err)
	}
	if turn == nil {
		s.closeEscalatedSupportCommandRun(ctx, meta)
		return mustJSON(map[string]any{"status": "suppressed", "next_action": "This customer turn already has an outcome. Stop; wait for a new customer message."}), nil
	}

	source, err := supportAI.messageRepo.GetByID(ctx, turn.SourceMessageID)
	if err != nil {
		return nil, fmt.Errorf("load support source message: %w", err)
	}
	if source == nil || !model.SupportAIReplyAllowed(*settings, conv, source) || !shouldAutomaticallyProcessSupportAI(*settings) {
		if err := s.supportProcessingRepo.MarkCompleted(ctx, turn.ID, nil, 0); err != nil {
			return nil, err
		}
		s.closeEscalatedSupportCommandRun(ctx, meta)
		return mustJSON(map[string]any{"status": "suppressed", "next_action": "AI replies are not enabled for this message channel. End your turn."}), nil
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
		evidence = supportEvidenceFromRows(rows)
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
	supportAI.publishProgress(meta.WorkspaceID, conversationID, supportAIProgressFinalizing)
	gate := evaluateSupportReplyGate(supportReplyGateInput{
		Kind:      req.ReplyKind,
		Contract:  contract,
		Evidence:  evidence,
		Threshold: settings.AIConfidenceThreshold,
	})
	if !gate.OK {
		slog.WarnContext(ctx, "send_reply: reply gate rejected runtime answer",
			"workspace_id", meta.WorkspaceID,
			"conversation_id", conversationID,
			"reason", gate.EscalationReason,
			"validation_outcome", gate.ValidationOutcome,
			"computed_confidence", gate.Confidence,
			"required_confidence", settings.AIConfidenceThreshold,
			"proposed_confidence", req.Confidence,
			"source_doc_ids", req.SourceDocIDs)
		sourceMessageID := s.supportTurnSourceMessageID(ctx, meta.WorkspaceID, conversationID, history)
		if escErr := supportAI.EscalateToHumanForMessageWithIssue(ctx, meta.WorkspaceID, conversationID, sourceMessageID, gate.EscalationReason, "", "", SupportHandoffBrief{ExpectedRunID: replyRunID}); escErr != nil {
			return nil, fmt.Errorf("handoff after reply validation: %w", escErr)
		}
		s.settleSupportSource(ctx, meta.WorkspaceID, conversationID, sourceMessageID)
		supportAI.publishTypingIndicator(ctx, meta.WorkspaceID, conversationID, false)
		s.closeEscalatedSupportCommandRun(ctx, meta)
		return mustJSON(map[string]any{
			"status":      "escalated",
			"reason":      gate.EscalationReason,
			"next_action": "The reply did not pass server validation and the conversation was handed to a human. End your turn without sending anything else.",
		}), nil
	}

	kind := normalizeSupportReplyKind(req.ReplyKind)
	progressState := ""
	if kind == supportReplyKindConversational && replyRunID != "" && s.commandBarService != nil && s.commandBarService.planRepo != nil {
		pending, pendingErr := s.commandBarService.planRepo.HasPendingSupportResult(ctx, meta.WorkspaceID, conversationID, replyRunID)
		if pendingErr != nil {
			return nil, fmt.Errorf("load pending support work: %w", pendingErr)
		}
		if pending {
			progressState = supportAIProgressChecking
		}
	}
	if progressState != "" && turn.Status == "waiting_for_result" {
		return mustJSON(map[string]any{"status": "awaiting_result", "next_action": supportProgressNextAction}), nil
	}
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
		message, err = supportAI.publishAIReply(ctx, meta.WorkspaceID, conversationID, agentID, content, supportReplyModelLabel, 0, gate.Confidence, sources, kind, "", "", progressState, conv.CustomerEmail, conv.CustomerPhone, turn.ID, replyRunID)
	} else {
		message, err = supportAI.publishAIInternalNote(ctx, meta.WorkspaceID, conversationID, agentID, "Suggested reply:\n\n"+content, supportReplyModelLabel, 0, gate.Confidence, sources, kind, "", "", progressState, conv.CustomerEmail, conv.CustomerPhone, turn.ID, replyRunID)
	}
	if errors.Is(err, errSupportTurnSettled) {
		return mustJSON(map[string]any{"status": "suppressed", "next_action": "This customer turn already has an outcome. End your turn."}), nil
	}
	if err != nil {
		return nil, fmt.Errorf("publish reply: %w", err)
	}

	if req.ResolvesConversation && kind == supportReplyKindConfirm {
		if updateErr := resolveSupportAIConversation(ctx, supportAI.conversationRepo.DB(), conv, "confirmed", time.Now().UTC()); updateErr != nil {
			slog.WarnContext(ctx, "mark confirmed resolution failed", "error", updateErr, "conversation_id", conversationID)
		}
	}

	if progressState != "" {
		supportAI.publishProgress(meta.WorkspaceID, conversationID, progressState)
		return mustJSON(map[string]any{"status": "awaiting_result", "message_id": message.ID, "next_action": supportProgressNextAction}), nil
	}

	triggerMessageID := turn.SourceMessageID
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

const supportProgressNextAction = "Acknowledgment delivered; the customer is still waiting for an answer. End this runtime turn and wait for the child result. When it arrives, call send_support_reply with the final answer; do not wait for another visitor message or repeat the acknowledgment."

func supportReplyInternalProcessDisclosures(content string) []string {
	lower := strings.ToLower(content)
	phrases := []string{
		"knowledge base",
		"search_knowledge",
		"search results",
		"i searched",
		"i'm searching",
		"i am searching",
		"let me search",
		"i'll search",
		"retrieval",
		"evidence id",
		"evidence_id",
		"source ranking",
		"child agent",
		"sub-agent",
		"sub agent",
		"agent run",
		"repository inspection",
		"confidence threshold",
		"confidence calculation",
		"internal tooling",
	}
	matches := make([]string, 0)
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			matches = append(matches, phrase)
		}
	}
	return matches
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
		Reason              string   `json:"reason"`
		IssueKey            string   `json:"issue_key"`
		IssueSummary        string   `json:"issue_summary"`
		AttemptedSteps      []string `json:"attempted_steps"`
		UnresolvedQuestions []string `json:"unresolved_questions"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse escalate input: %w", err)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "agent_requested"
	}
	var escalationRunID string
	if meta.TargetType != supportPreviewTarget && meta.RunID != "" {
		conv, err := supportAI.conversationRepo.GetByID(ctx, meta.WorkspaceID, conversationID, "", model.RoleOwner)
		if err != nil {
			return nil, err
		}
		run, err := s.resolveCommandRun(ctx, meta)
		if err != nil {
			return nil, err
		}
		if run != nil {
			escalationRunID = run.ID
		}
		if conv == nil || model.SupportAIConversationBlocked(conv) || (conv.AIControlVersion > 0 && (run == nil || derefString(conv.AIActiveRunID) != run.ID)) {
			return mustJSON(map[string]any{"status": "suppressed", "next_action": "Ownership changed. End your turn."}), nil
		}
	}
	sourceMessageID := s.supportTurnSourceMessageID(ctx, meta.WorkspaceID, conversationID, nil)
	if err := supportAI.EscalateToHumanForMessageWithIssue(ctx, meta.WorkspaceID, conversationID, sourceMessageID, reason, strings.TrimSpace(req.IssueKey), strings.TrimSpace(req.IssueSummary), SupportHandoffBrief{ExpectedRunID: escalationRunID, Issue: req.IssueSummary, AttemptedSteps: req.AttemptedSteps, UnresolvedQuestions: req.UnresolvedQuestions}); err != nil {
		return nil, fmt.Errorf("escalate: %w", err)
	}
	s.settleSupportSource(ctx, meta.WorkspaceID, conversationID, sourceMessageID)
	supportAI.publishTypingIndicator(ctx, meta.WorkspaceID, conversationID, false)
	s.closeEscalatedSupportCommandRun(ctx, meta)
	return mustJSON(map[string]any{
		"status":      "escalated",
		"next_action": "The conversation was handed to a human with an availability-aware message. End your turn without sending anything else.",
	}), nil
}

// closeEscalatedSupportCommandRun synchronously terminates the live runtime
// after ownership passes to a human. Waiting for the pause hook or 30-second
// sweep leaves a window in which the model can issue more searches/replies.
func (s *InternalCommandService) closeEscalatedSupportCommandRun(ctx context.Context, meta model.InternalCommandContext) {
	if s == nil || s.supportRunCloser == nil {
		return
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil || run == nil {
		if err != nil {
			slog.WarnContext(ctx, "close escalated support run: resolve failed", "error", err, "run_id", meta.RunID)
		}
		return
	}
	if _, err := s.supportRunCloser.CancelRun(ctx, run.WorkspaceID, run.ID, ""); err != nil {
		slog.WarnContext(ctx, "close escalated support run failed", "error", err, "run_id", run.ID, "conversation_id", run.TargetID)
	}
}

// settleSupportSource only settles the source captured before handoff. A
// cancellation callback may race with return and a newer customer turn.
func (s *InternalCommandService) settleSupportSource(ctx context.Context, workspaceID, conversationID, sourceMessageID string) {
	if s.supportProcessingRepo == nil || sourceMessageID == "" {
		return
	}
	if err := s.supportProcessingRepo.CompleteSource(ctx, workspaceID, conversationID, sourceMessageID); err != nil {
		slog.WarnContext(ctx, "settle support source", "error", err, "conversation_id", conversationID)
	}
}

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

// consumeSupportReplyBilling intentionally does not charge runtime-delivered
// replies because their model work is already covered by the agent-run usage.
func (s *InternalCommandService) consumeSupportReplyBilling(context.Context, string, string, string) {
}
