package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) executeSupportPreviewCommand(ctx context.Context, run *model.AgentRun, meta model.InternalCommandContext, def InternalCommandDefinition, input json.RawMessage) (json.RawMessage, error) {
	snapshot, err := supportPreviewSnapshot(run)
	if err != nil {
		return nil, err
	}
	if !model.IsAgentRunActiveStatus(run.Status) || time.Since(run.CreatedAt) > 3*time.Minute {
		return nil, fmt.Errorf("preview is no longer active")
	}
	var stored model.AgentRunInputPayload
	if err = json.Unmarshal(run.Input, &stored); err != nil {
		return nil, err
	}
	if def.Tool == nil || !slices.Contains(supportPreviewTools, def.Tool.Alias) || !slices.Contains(stored.AllowedTools, def.Tool.Alias) {
		return nil, fmt.Errorf("tool %s is unavailable in support preview", def.Name)
	}
	if s.agentRunArtifactRepo == nil {
		return nil, fmt.Errorf("preview result storage unavailable")
	}
	existing, err := s.agentRunArtifactRepo.GetByIDAndWorkspace(ctx, run.WorkspaceID, supportPreviewResultID(run.ID))
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return mustJSON(map[string]any{"status": "preview_complete", "next_action": "End your turn. This preview already has an outcome."}), nil
	}
	switch def.Name {
	case "support.search_knowledge":
		return def.Execute(ctx, meta, input)
	case "support.send_reply":
		return s.executeSupportSendReply(ctx, meta, input)
	case "support.skip_reply":
		req, err := parseSupportSkipReply(input)
		if err != nil {
			return nil, err
		}
		response := supportPreviewResponse(run, snapshot)
		response.FinalDecision = "no_reply"
		response.FinalReason = req.Reason
		return s.storeSupportPreviewOutcome(ctx, run, response)
	case "support.escalate_to_human":
		var req struct {
			Reason string `json:"reason"`
		}
		if err = json.Unmarshal(input, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.Reason) == "" {
			return nil, errCommandInput("reason is required")
		}
		response := supportPreviewResponse(run, snapshot)
		response.FinalDecision = "handoff"
		response.FinalReason = req.Reason
		return s.storeSupportPreviewOutcome(ctx, run, response)
	case "support.list_conversation_messages", "support.get_conversation":
		var req struct {
			ConversationID string `json:"conversation_id"`
			Limit          int    `json:"limit"`
			Offset         int    `json:"offset"`
		}
		if err = json.Unmarshal(input, &req); err != nil {
			return nil, err
		}
		if req.ConversationID != "" && req.ConversationID != run.TargetID {
			return nil, fmt.Errorf("preview can only read its own conversation snapshot")
		}
		if def.Name == "support.get_conversation" {
			return mustJSON(previewConversationData(run)), nil
		}
		if req.Limit == 0 {
			req.Limit = 20
		}
		if req.Limit < 1 || req.Limit > 100 || req.Offset < 0 {
			return nil, fmt.Errorf("invalid pagination")
		}
		messages := previewMessages(snapshot)
		end := max(0, len(messages)-req.Offset)
		start := max(0, end-req.Limit)
		response := commandPaginationOutput(int64(len(messages)), req.Offset, req.Limit, end-start)
		response["messages"] = messages[start:end]
		return mustJSON(response), nil
	default:
		return nil, fmt.Errorf("tool unavailable in support preview")
	}
}

func previewConversationData(run *model.AgentRun) map[string]interface{} {
	return runtimeSupportConversationContextData(&model.SupportConversation{ID: run.TargetID, WorkspaceID: run.WorkspaceID, Subject: "Support preview", Status: "open", Channel: "widget"})
}

func supportEvidenceFromRows(rows []model.SupportRunEvidence) []KnowledgeSearchResult {
	results := make([]KnowledgeSearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, KnowledgeSearchResult{ID: row.EvidenceID, ReferenceID: row.ReferenceID, SourceType: row.SourceType, SourceID: row.SourceID, DocumentID: row.DocumentID, Title: row.Title, URL: row.URL, IsInternal: row.IsInternal, Content: row.Content, LexicalScore: row.LexicalScore, VectorScore: row.VectorScore, CombinedScore: row.CombinedScore})
	}
	return results
}

func (s *InternalCommandService) captureSupportPreviewReply(ctx context.Context, meta model.InternalCommandContext, kind string, contract *AIResponseContract) (json.RawMessage, error) {
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	snapshot, err := supportPreviewSnapshot(run)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || s.supportRunEvidenceRepo == nil {
		return nil, fmt.Errorf("preview evidence unavailable")
	}
	rows, err := s.supportRunEvidenceRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: kind, Contract: contract, Evidence: supportEvidenceFromRows(rows), Threshold: snapshot.ConfidenceThreshold})
	response := supportPreviewResponse(run, snapshot)
	response.FinalDecision = "answer"
	response.FinalReason = "validated_reply"
	if !gate.OK {
		response.FinalDecision = "handoff"
		response.FinalReason = gate.EscalationReason
	}
	response.Answer = &model.SupportAIPreviewAnswer{Content: contract.Content, CanAnswer: gate.OK, SourceDocIDs: contract.SourceDocIDs, LLMConfidence: contract.Confidence, GroundedConfidence: gate.Confidence, Provider: response.Provider, Model: response.Model, ValidationOutcome: gate.ValidationOutcome}
	return s.storeSupportPreviewOutcome(ctx, run, response)
}

func (s *InternalCommandService) storeSupportPreviewOutcome(ctx context.Context, run *model.AgentRun, response *model.SupportAIPreviewResponse) (json.RawMessage, error) {
	rows, err := s.supportRunEvidenceRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	response.Retrieval.Results = previewSearchResults(supportEvidenceFromRows(rows))
	response.Retrieval.ResultCount = len(rows)
	response.Status = "completed"
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	content := string(encoded)
	artifact := &model.AgentRunArtifact{ID: supportPreviewResultID(run.ID), WorkspaceID: run.WorkspaceID, RunID: run.ID, ArtifactType: "support_preview_result", Format: "json", StorageMode: "inline", InlineContent: &content, Metadata: json.RawMessage(`{}`)}
	if err = s.agentRunArtifactRepo.CreateOnce(ctx, artifact); err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"status": "preview_complete", "decision": response.FinalDecision, "reason": response.FinalReason, "next_action": "End your turn. The outcome was captured; no customer was contacted or updated."}), nil
}
