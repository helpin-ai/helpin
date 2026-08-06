package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportCoverageGapOutcomeSummaryKey = "support_coverage_gap_outcome"

type supportCoverageGapOutcomeSummary struct {
	Outcome    string `json:"outcome"`
	DocumentID string `json:"document_id,omitempty"`
	ProposalID string `json:"proposal_id,omitempty"`
	Summary    string `json:"summary"`
	RecordedAt string `json:"recorded_at"`
}

func (s *InternalCommandService) registerSupportCoverageCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "support.complete_coverage_gap",
		Module:               "support",
		Mutating:             true,
		SupportedTargetTypes: []string{"support_coverage_gap"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.complete_coverage_gap",
			Alias:       agentcontract.ToolCompleteSupportCoverageGap,
			Category:    "Support / Coverage",
			Description: "Record the durable outcome of the current support coverage gap run. Call this exactly once after creating or updating the document, submitting a persisted proposal, or explicitly handing the work to a human.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"outcome": map[string]any{
						"type":        "string",
						"enum":        []string{SupportCoverageAgentOutcomeResolved, SupportCoverageAgentOutcomeProposalSubmitted, SupportCoverageAgentOutcomeHandoff},
						"description": "resolved after a document was created or updated; proposal_submitted after a durable review proposal; handoff when a human must continue.",
					},
					"document_id": map[string]any{
						"type":        "string",
						"description": "Required for resolved and proposal_submitted. The created or updated Helpin Docs document.",
					},
					"proposal_id": map[string]any{
						"type":        "string",
						"description": "Required for proposal_submitted. The persisted Docs proposal returned by publish_document_change_proposal.",
					},
					"summary": map[string]any{
						"type":        "string",
						"description": "Concise evidence-based description of the completed work or handoff.",
					},
				},
				"required": []string{"outcome", "summary"},
			},
		},
		Execute: s.executeCompleteSupportCoverageGap,
	})
}

func (s *InternalCommandService) executeCompleteSupportCoverageGap(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.supportCoverageService == nil {
		return nil, fmt.Errorf("support coverage service is not available")
	}
	var req struct {
		Outcome    string `json:"outcome"`
		DocumentID string `json:"document_id,omitempty"`
		ProposalID string `json:"proposal_id,omitempty"`
		Summary    string `json:"summary"`
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return nil, fmt.Errorf("parse support coverage outcome input: %w", err)
	}
	req.Outcome = strings.TrimSpace(req.Outcome)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.ProposalID = strings.TrimSpace(req.ProposalID)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if len([]rune(req.Summary)) > 1200 {
		return nil, fmt.Errorf("summary must be 1200 characters or fewer")
	}
	if req.Outcome == SupportCoverageAgentOutcomeProposalSubmitted && req.ProposalID == "" {
		return nil, fmt.Errorf("proposal_id is required for proposal_submitted")
	}
	if req.Outcome == SupportCoverageAgentOutcomeResolved || req.Outcome == SupportCoverageAgentOutcomeProposalSubmitted {
		if req.DocumentID == "" {
			return nil, fmt.Errorf("document_id is required for %s", req.Outcome)
		}
		if err := s.requireCommandDocumentInWorkspace(ctx, meta.WorkspaceID, req.DocumentID); err != nil {
			return nil, err
		}
	}
	if req.Outcome == SupportCoverageAgentOutcomeProposalSubmitted {
		if s.docsChangeProposalService == nil {
			return nil, fmt.Errorf("docs change proposal service is not available")
		}
		if _, err := s.docsChangeProposalService.Get(ctx, meta.WorkspaceID, req.DocumentID, req.ProposalID); err != nil {
			return nil, err
		}
	}
	if err := s.supportCoverageService.RecordAgentOutcome(ctx, meta.WorkspaceID, meta.TargetID, req.Outcome, req.DocumentID); err != nil {
		return nil, err
	}

	outcome := supportCoverageGapOutcomeSummary{
		Outcome: req.Outcome, DocumentID: req.DocumentID, ProposalID: req.ProposalID,
		Summary: req.Summary, RecordedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.persistSupportCoverageGapOutcome(ctx, meta, outcome); err != nil {
		return nil, err
	}
	status := model.SupportCoverageGapStatusOpen
	if req.Outcome == SupportCoverageAgentOutcomeResolved {
		status = model.SupportCoverageGapStatusDone
	}
	return mustJSON(map[string]any{
		"gap_id": meta.TargetID, "outcome": req.Outcome, "status": status,
		"document_id": req.DocumentID, "proposal_id": req.ProposalID,
	}), nil
}

func (s *InternalCommandService) persistSupportCoverageGapOutcome(ctx context.Context, meta model.InternalCommandContext, outcome supportCoverageGapOutcomeSummary) error {
	if s.agentRunRepo == nil {
		return fmt.Errorf("agent run repository is not configured")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return err
	}
	body := map[string]any{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body[supportCoverageGapOutcomeSummaryKey] = outcome
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode support coverage outcome: %w", err)
	}
	if err := s.agentRunRepo.UpdateOutputSummary(ctx, run.ID, payload); err != nil {
		return err
	}
	run.OutputSummary = payload
	return nil
}
