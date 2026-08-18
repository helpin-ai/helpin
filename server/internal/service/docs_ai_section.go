package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type DocsAISectionService struct {
	candidateRepo *repository.DocsAISectionCandidateRepository
	blockRepo     *repository.DocsBlockRepository
	blockSvc      *DocsBlockService
	docRepo       *repository.DocsDocumentRepository
	agentSvc      *AgentService
	ruleEngine    *AutomationRuleEngine
	activity      *PMActivityService
}

func NewDocsAISectionService(
	candidateRepo *repository.DocsAISectionCandidateRepository,
	blockRepo *repository.DocsBlockRepository,
	blockSvc *DocsBlockService,
	docRepo *repository.DocsDocumentRepository,
	agentSvc *AgentService,
) *DocsAISectionService {
	return &DocsAISectionService{
		candidateRepo: candidateRepo,
		blockRepo:     blockRepo,
		blockSvc:      blockSvc,
		docRepo:       docRepo,
		agentSvc:      agentSvc,
	}
}

func (s *DocsAISectionService) SetRuleEngine(engine *AutomationRuleEngine) {
	s.ruleEngine = engine
}

func (s *DocsAISectionService) SetActivityService(activity *PMActivityService) {
	s.activity = activity
}

func (s *DocsAISectionService) LatestCandidate(ctx context.Context, workspaceID, documentID, blockID string) (*model.DocsAISectionCandidate, error) {
	if _, _, err := s.loadAISectionBlockForRead(ctx, workspaceID, documentID, blockID); err != nil {
		return nil, err
	}
	return s.candidateRepo.LatestOpen(ctx, documentID, blockID)
}

func (s *DocsAISectionService) Regenerate(ctx context.Context, workspaceID, documentID, blockID, actorID string, req model.RegenerateAISectionRequest) (*model.AISectionCandidateResponse, error) {
	doc, block, err := s.loadAISectionBlock(ctx, workspaceID, documentID, blockID)
	if err != nil {
		return nil, err
	}
	instructions, err := validateAISectionInstructions(req.Instructions)
	if err != nil {
		return nil, err
	}
	agent, err := s.loadAISectionAgent(ctx, workspaceID, strings.TrimSpace(req.AgentID))
	if err != nil {
		return nil, err
	}
	runContext := buildAISectionRunContext(doc, block, instructions)
	run, err := s.agentSvc.StartTargetRun(ctx, workspaceID, "document", documentID, model.StartAgentRunRequest{
		AgentID:           strings.TrimSpace(req.AgentID),
		AdditionalContext: strPtr(runContext),
		AllowedTools:      aiSectionRunToolsForAgent(agent),
		Output: &model.AgentRunOutputContext{
			Type:           "docs_ai_section_candidate",
			IdempotencyKey: blockID,
		},
	}, actorID)
	if err != nil {
		return nil, err
	}
	if existing, err := s.candidateRepo.LatestOpen(ctx, documentID, blockID); err != nil {
		return nil, err
	} else if existing != nil {
		if err := s.candidateRepo.UpdateStatus(ctx, existing.ID, model.DocsAISectionCandidateStatusRejected, actorID); err != nil {
			return nil, err
		}
	}

	s.logAISectionActivity(ctx, doc, block, actorID, "ai_section_regenerated", map[string]interface{}{
		"block_id":     blockID,
		"agent_run_id": run.ID,
	})
	s.emitAISectionEvent(ctx, model.TriggerAISectionRegenerated, doc, blockID, run.ID)
	return &model.AISectionCandidateResponse{Candidate: nil, AgentRun: run}, nil
}

func (s *DocsAISectionService) loadAISectionAgent(ctx context.Context, workspaceID, agentID string) (*model.Agent, error) {
	if s.agentSvc == nil || s.agentSvc.agentRepo == nil {
		return nil, fmt.Errorf("agent service is not configured")
	}
	if strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	agent, err := s.agentSvc.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	return agent, nil
}

func validateAISectionInstructions(raw *string) (string, error) {
	instructions := strings.TrimSpace(derefString(raw))
	if instructions == "" {
		return "", fmt.Errorf("regeneration instructions are required")
	}
	return instructions, nil
}

func aiSectionRunToolsForAgent(agent *model.Agent) []string {
	if agent == nil {
		return nil
	}
	agentTools := parseJSONStringSlice(agent.AllowedTools)
	if len(agentTools) == 0 {
		return nil
	}
	defaultTools := []string{
		"list_documents",
		"list_collections",
		"read_document",
		"get_document_blocks",
		"search_documents",
		"web_search",
		"fetch_url",
		"crawl_url",
		"publish_ai_section_candidate",
		"list_deals",
		"list_contacts",
		"list_buyer_signals",
	}
	allowedSet := make(map[string]bool, len(agentTools))
	for _, tool := range agentTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	out := make([]string, 0, len(defaultTools))
	seen := map[string]bool{}
	for _, tool := range defaultTools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		if tool != "publish_ai_section_candidate" && !allowedSet[tool] {
			continue
		}
		seen[tool] = true
		out = append(out, tool)
	}
	return out
}

func (s *DocsAISectionService) Approve(ctx context.Context, workspaceID, documentID, blockID, actorID string) (*model.AISectionCandidateResponse, error) {
	doc, block, err := s.loadAISectionBlock(ctx, workspaceID, documentID, blockID)
	if err != nil {
		return nil, err
	}
	candidate, err := s.candidateRepo.LatestOpen(ctx, documentID, blockID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, fmt.Errorf("candidate not found")
	}
	if !aiSectionContentEqualForApproval(candidate.CurrentContent, block.Content) {
		return nil, ErrDocsStaleBlockRevision
	}
	approvedContent, err := aiSectionCandidateContentWithStatusForCurrent(candidate.CandidateContent, block.Content, model.DocsAISectionCandidateStatusApproved)
	if err != nil {
		return nil, err
	}
	content, err := s.blockSvc.Patch(ctx, documentID, blockID, block.Revision, approvedContent, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.candidateRepo.UpdateStatus(ctx, candidate.ID, model.DocsAISectionCandidateStatusApproved, actorID); err != nil {
		return nil, err
	}
	candidate.Status = model.DocsAISectionCandidateStatusApproved
	candidate.ApprovedBy = &actorID
	candidate.CandidateContent = approvedContent
	s.logAISectionActivity(ctx, doc, block, actorID, "ai_section_approved", map[string]interface{}{
		"block_id":     blockID,
		"candidate_id": candidate.ID,
		"agent_run_id": derefString(candidate.AgentRunID),
	})
	s.emitAISectionEvent(ctx, model.TriggerAISectionApproved, doc, blockID, derefString(candidate.AgentRunID))
	return &model.AISectionCandidateResponse{Candidate: candidate, Content: content}, nil
}

func (s *DocsAISectionService) Reject(ctx context.Context, workspaceID, documentID, blockID, actorID string) (*model.AISectionCandidateResponse, error) {
	doc, block, err := s.loadAISectionBlock(ctx, workspaceID, documentID, blockID)
	if err != nil {
		return nil, err
	}
	candidate, err := s.candidateRepo.LatestOpen(ctx, documentID, blockID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, fmt.Errorf("candidate not found")
	}
	if err := s.candidateRepo.UpdateStatus(ctx, candidate.ID, model.DocsAISectionCandidateStatusRejected, actorID); err != nil {
		return nil, err
	}
	candidate.Status = model.DocsAISectionCandidateStatusRejected
	s.logAISectionActivity(ctx, doc, block, actorID, "ai_section_rejected", map[string]interface{}{
		"block_id":     blockID,
		"candidate_id": candidate.ID,
		"agent_run_id": derefString(candidate.AgentRunID),
	})
	return &model.AISectionCandidateResponse{Candidate: candidate}, nil
}

func (s *DocsAISectionService) loadAISectionBlock(ctx context.Context, workspaceID, documentID, blockID string) (*model.DocsDocument, *model.DocsBlock, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, nil, fmt.Errorf("document not found")
	}
	if err := checkLocked(doc); err != nil {
		return nil, nil, err
	}
	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return nil, nil, err
	}
	if block == nil || block.DocumentID != documentID || block.WorkspaceID != workspaceID || block.DeletedAt != nil {
		return nil, nil, fmt.Errorf("block not found")
	}
	if block.Type != "aiSection" {
		return nil, nil, fmt.Errorf("block is not an AI section")
	}
	return doc, block, nil
}

func (s *DocsAISectionService) loadAISectionBlockForRead(ctx context.Context, workspaceID, documentID, blockID string) (*model.DocsDocument, *model.DocsBlock, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, nil, fmt.Errorf("document not found")
	}
	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return nil, nil, err
	}
	if block == nil || block.DocumentID != documentID || block.WorkspaceID != workspaceID || block.DeletedAt != nil {
		return nil, nil, fmt.Errorf("block not found")
	}
	if block.Type != "aiSection" {
		return nil, nil, fmt.Errorf("block is not an AI section")
	}
	return doc, block, nil
}

func jsonRawEqual(a, b json.RawMessage) bool {
	var ca bytes.Buffer
	if err := json.Compact(&ca, a); err != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	var cb bytes.Buffer
	if err := json.Compact(&cb, b); err != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

func aiSectionContentEqualForApproval(candidateSnapshot, current json.RawMessage) bool {
	if jsonRawEqual(candidateSnapshot, current) {
		return true
	}
	return jsonRawEqual(normalizeAISectionForApproval(candidateSnapshot), normalizeAISectionForApproval(current))
}

func normalizeAISectionForApproval(raw json.RawMessage) json.RawMessage {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return bytes.TrimSpace(raw)
	}
	attrs, _ := node["attrs"].(map[string]any)
	for _, key := range []string{
		"title",
		"status",
		"ownerAgentId",
		"ownerAgentName",
		"lastGeneratedAt",
		"model",
		"promptHash",
		"sourceCount",
	} {
		delete(attrs, key)
	}
	normalized, err := json.Marshal(node)
	if err != nil {
		return bytes.TrimSpace(raw)
	}
	return normalized
}

func aiSectionCandidateContentWithStatus(raw json.RawMessage, candidateStatus string) (json.RawMessage, error) {
	return aiSectionCandidateContentWithStatusForCurrent(raw, nil, candidateStatus)
}

func aiSectionCandidateContentWithStatusForCurrent(raw, current json.RawMessage, candidateStatus string) (json.RawMessage, error) {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse AI section candidate: %w", err)
	}
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		node["attrs"] = attrs
	}
	if len(current) > 0 {
		var currentNode map[string]any
		if err := json.Unmarshal(current, &currentNode); err == nil {
			if currentAttrs, _ := currentNode["attrs"].(map[string]any); currentAttrs != nil {
				for _, key := range []string{"blockId", "title"} {
					if value, ok := currentAttrs[key]; ok {
						attrs[key] = value
					}
				}
			}
		}
	}
	switch candidateStatus {
	case model.DocsAISectionCandidateStatusApproved:
		attrs["status"] = "approved"
	case model.DocsAISectionCandidateStatusRejected:
		attrs["status"] = "draft"
	default:
		attrs["status"] = "needs_review"
	}
	encoded, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("marshal AI section candidate: %w", err)
	}
	return encoded, nil
}

func aiSectionTitle(raw json.RawMessage) string {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return "AI section"
	}
	attrs, _ := node["attrs"].(map[string]any)
	title, _ := attrs["title"].(string)
	return firstNonEmptyAISection(title, "AI section")
}

func buildAISectionRunContext(doc *model.DocsDocument, block *model.DocsBlock, instructions string) string {
	return fmt.Sprintf("Regenerate AI section block %s in document %s (%q). Current revision: %d. Instructions: %s. Use available read/search/web tools to verify relevant facts. Then call publish_ai_section_candidate with document_id=%s, block_id=%s, and the replacement section body as markdown. Do not call update_document_block or write_document_content; publish_ai_section_candidate stores a review candidate and the document changes only after human approval.",
		block.ID,
		doc.ID,
		doc.Title,
		block.Revision,
		firstNonEmptyAISection(instructions, "Improve this section."),
		doc.ID,
		block.ID,
	)
}

func (s *DocsAISectionService) emitAISectionEvent(ctx context.Context, triggerType string, doc *model.DocsDocument, blockID, runID string) {
	if s.ruleEngine == nil || doc == nil {
		return
	}
	event := model.AutomationEvent{
		WorkspaceID: doc.WorkspaceID,
		TriggerType: triggerType,
		TargetType:  "ai_section",
		TargetID:    blockID,
		RunID:       runID,
	}
	if doc.TeamID != nil {
		event.TeamID = *doc.TeamID
	}
	s.ruleEngine.EvaluateEvent(ctx, event, nil)
}

func (s *DocsAISectionService) logAISectionActivity(ctx context.Context, doc *model.DocsDocument, block *model.DocsBlock, actorID, action string, metadata map[string]interface{}) {
	if s.activity == nil || doc == nil || block == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["block_type"] = block.Type
	metadata["block_title"] = aiSectionTitle(block.Content)
	if err := s.activity.Log(ctx, doc.WorkspaceID, "doc", doc.ID, optionalActor(actorID), action, nil, nil, nil, metadata); err != nil {
		// Activity should not block document editing or candidate review.
		slog.WarnContext(ctx, "failed to log ai section activity", "error", err, "document_id", doc.ID, "block_id", block.ID, "action", action)
	}
}

func firstNonEmptyAISection(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
