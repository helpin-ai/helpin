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
	Outcome               string `json:"outcome"`
	Action                string `json:"action"`
	SourceStatus          string `json:"source_status"`
	DocumentID            string `json:"document_id,omitempty"`
	ProposalID            string `json:"proposal_id,omitempty"`
	HandoffOwner          string `json:"handoff_owner,omitempty"`
	DocumentationEvidence string `json:"documentation_evidence"`
	SourceEvidence        string `json:"source_evidence,omitempty"`
	Summary               string `json:"summary"`
	Recovered             bool   `json:"recovered,omitempty"`
	RecordedAt            string `json:"recorded_at"`
}

type completeSupportCoverageGapRequest struct {
	Outcome               string `json:"outcome"`
	Action                string `json:"action"`
	SourceStatus          string `json:"source_status"`
	DocumentID            string `json:"document_id,omitempty"`
	ProposalID            string `json:"proposal_id,omitempty"`
	HandoffOwner          string `json:"handoff_owner,omitempty"`
	DocumentationEvidence string `json:"documentation_evidence"`
	SourceEvidence        string `json:"source_evidence,omitempty"`
	Summary               string `json:"summary"`
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
			Description: "Record the durable disposition of the current support coverage gap run. Call this exactly once after creating or updating documentation, preparing review-ready work, routing a non-documentation finding, or recording a genuine source blocker.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"outcome": map[string]any{
						"type":        "string",
						"enum":        []string{SupportCoverageAgentOutcomeResolved, SupportCoverageAgentOutcomeReviewReady, SupportCoverageAgentOutcomeRouted, SupportCoverageAgentOutcomeBlocked},
						"description": "resolved fixes and closes the gap; review_ready records a durable draft or proposal while keeping the gap open; routed records a completed non-doc investigation; blocked records unavailable required source context.",
					},
					"action": map[string]any{
						"type": "string",
						"enum": []string{
							SupportCoverageAgentActionDocumentCreated,
							SupportCoverageAgentActionDocumentUpdated,
							SupportCoverageAgentActionProposalSubmitted,
							SupportCoverageAgentActionExistingDocsSufficient,
							SupportCoverageAgentActionFeatureNotFound,
							SupportCoverageAgentActionNonDocGap,
							SupportCoverageAgentActionSourceUnavailable,
						},
						"description": "The durable action or investigation finding that justifies the outcome.",
					},
					"source_status": map[string]any{
						"type":        "string",
						"enum":        []string{SupportCoverageAgentSourceVerified, SupportCoverageAgentSourceNotFound, SupportCoverageAgentSourceNotApplicable, SupportCoverageAgentSourceUnavailable},
						"description": "Whether the relevant product/source-of-truth behavior was verified, not found after inspection, not applicable, or unavailable.",
					},
					"document_id": map[string]any{
						"type":        "string",
						"description": "Required for resolved and review_ready documentation actions. The created, updated, or proposed Helpin Docs document.",
					},
					"proposal_id": map[string]any{
						"type":        "string",
						"description": "Required for proposal_submitted. The persisted Docs proposal returned by publish_document_change_proposal.",
					},
					"handoff_owner": map[string]any{
						"type":        "string",
						"description": "Required for routed and blocked outcomes. The team or role that should continue the work.",
					},
					"documentation_evidence": map[string]any{
						"type":        "string",
						"description": "Required concise record of the current Docs searches and documents reviewed, including that no matching document was found when applicable.",
					},
					"source_evidence": map[string]any{
						"type":        "string",
						"description": "Required unless source_status is not_applicable. Summarize repository, API, policy, or other authoritative source inspection and what was or was not found.",
					},
					"summary": map[string]any{
						"type":        "string",
						"description": "Concise evidence-based description of the completed work or handoff.",
					},
				},
				"required": []string{"outcome", "action", "source_status", "documentation_evidence", "summary"},
			},
		},
		Execute: s.executeCompleteSupportCoverageGap,
	})
}

func (s *InternalCommandService) executeCompleteSupportCoverageGap(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.supportCoverageService == nil {
		return nil, fmt.Errorf("support coverage service is not available")
	}
	var req completeSupportCoverageGapRequest
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return nil, fmt.Errorf("parse support coverage outcome input: %w", err)
	}
	normalizeCompleteSupportCoverageGapRequest(&req)
	if err := validateCompleteSupportCoverageGapRequest(req); err != nil {
		return nil, err
	}
	if req.Outcome == SupportCoverageAgentOutcomeResolved || req.Outcome == SupportCoverageAgentOutcomeReviewReady || req.DocumentID != "" {
		if req.DocumentID == "" {
			return nil, fmt.Errorf("document_id is required for %s", req.Outcome)
		}
		if err := s.requireCommandDocumentInWorkspace(ctx, meta.WorkspaceID, req.DocumentID); err != nil {
			return nil, err
		}
	}
	if req.Action == SupportCoverageAgentActionProposalSubmitted {
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
		Outcome: req.Outcome, Action: req.Action, SourceStatus: req.SourceStatus,
		DocumentID: req.DocumentID, ProposalID: req.ProposalID, HandoffOwner: req.HandoffOwner,
		DocumentationEvidence: req.DocumentationEvidence, SourceEvidence: req.SourceEvidence,
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
		"gap_id": meta.TargetID, "outcome": req.Outcome, "action": req.Action,
		"source_status": req.SourceStatus, "status": status,
		"document_id": req.DocumentID, "proposal_id": req.ProposalID,
	}), nil
}

func normalizeCompleteSupportCoverageGapRequest(req *completeSupportCoverageGapRequest) {
	if req == nil {
		return
	}
	req.Outcome = strings.TrimSpace(req.Outcome)
	req.Action = strings.TrimSpace(req.Action)
	req.SourceStatus = strings.TrimSpace(req.SourceStatus)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.ProposalID = strings.TrimSpace(req.ProposalID)
	req.HandoffOwner = strings.TrimSpace(req.HandoffOwner)
	req.DocumentationEvidence = strings.TrimSpace(req.DocumentationEvidence)
	req.SourceEvidence = strings.TrimSpace(req.SourceEvidence)
	req.Summary = strings.TrimSpace(req.Summary)
	legacyHandoff := req.Outcome == SupportCoverageAgentOutcomeHandoff

	// Backward compatibility for in-flight runs that received the first
	// completion schema before this disposition contract was deployed.
	switch req.Outcome {
	case SupportCoverageAgentOutcomeProposalSubmitted:
		req.Outcome = SupportCoverageAgentOutcomeReviewReady
		if req.Action == "" {
			req.Action = SupportCoverageAgentActionProposalSubmitted
		}
	case SupportCoverageAgentOutcomeHandoff:
		req.Outcome = SupportCoverageAgentOutcomeRouted
		if req.Action == "" {
			req.Action = SupportCoverageAgentActionNonDocGap
		}
	}
	if req.Outcome == SupportCoverageAgentOutcomeResolved && req.Action == "" {
		req.Action = SupportCoverageAgentActionDocumentUpdated
	}
	if req.SourceStatus == "" {
		req.SourceStatus = SupportCoverageAgentSourceNotApplicable
	}
	if req.DocumentationEvidence == "" {
		req.DocumentationEvidence = req.Summary
	}
	if legacyHandoff && req.HandoffOwner == "" {
		req.HandoffOwner = "human documentation owner"
	}
}

func validateCompleteSupportCoverageGapRequest(req completeSupportCoverageGapRequest) error {
	if req.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	if len([]rune(req.Summary)) > 1200 {
		return fmt.Errorf("summary must be 1200 characters or fewer")
	}
	if req.DocumentationEvidence == "" {
		return fmt.Errorf("documentation_evidence is required")
	}
	if len([]rune(req.DocumentationEvidence)) > 2000 {
		return fmt.Errorf("documentation_evidence must be 2000 characters or fewer")
	}
	switch req.SourceStatus {
	case SupportCoverageAgentSourceVerified, SupportCoverageAgentSourceNotFound, SupportCoverageAgentSourceUnavailable:
		if req.SourceEvidence == "" {
			return fmt.Errorf("source_evidence is required when source_status is %s", req.SourceStatus)
		}
	case SupportCoverageAgentSourceNotApplicable:
	default:
		return fmt.Errorf("source_status must be verified, not_found, not_applicable, or unavailable")
	}
	if len([]rune(req.SourceEvidence)) > 2000 {
		return fmt.Errorf("source_evidence must be 2000 characters or fewer")
	}

	switch req.Outcome {
	case SupportCoverageAgentOutcomeResolved:
		if req.Action != SupportCoverageAgentActionDocumentCreated && req.Action != SupportCoverageAgentActionDocumentUpdated {
			return fmt.Errorf("resolved requires action document_created or document_updated")
		}
		if req.DocumentID == "" {
			return fmt.Errorf("document_id is required for resolved")
		}
		if req.SourceStatus == SupportCoverageAgentSourceNotFound || req.SourceStatus == SupportCoverageAgentSourceUnavailable {
			return fmt.Errorf("resolved requires verified or not_applicable source_status")
		}
	case SupportCoverageAgentOutcomeReviewReady:
		switch req.Action {
		case SupportCoverageAgentActionDocumentCreated, SupportCoverageAgentActionDocumentUpdated:
		case SupportCoverageAgentActionProposalSubmitted:
			if req.ProposalID == "" {
				return fmt.Errorf("proposal_id is required for proposal_submitted")
			}
		default:
			return fmt.Errorf("review_ready requires a document or proposal action")
		}
		if req.DocumentID == "" {
			return fmt.Errorf("document_id is required for review_ready")
		}
	case SupportCoverageAgentOutcomeRouted:
		switch req.Action {
		case SupportCoverageAgentActionExistingDocsSufficient:
			if req.DocumentID == "" {
				return fmt.Errorf("document_id is required when existing docs are sufficient")
			}
		case SupportCoverageAgentActionFeatureNotFound:
			if req.SourceStatus != SupportCoverageAgentSourceNotFound {
				return fmt.Errorf("feature_not_found requires source_status not_found")
			}
		case SupportCoverageAgentActionNonDocGap:
		default:
			return fmt.Errorf("routed requires existing_docs_sufficient, feature_not_found, or non_doc_gap action")
		}
		if req.HandoffOwner == "" {
			return fmt.Errorf("handoff_owner is required for routed")
		}
	case SupportCoverageAgentOutcomeBlocked:
		if req.Action != SupportCoverageAgentActionSourceUnavailable || req.SourceStatus != SupportCoverageAgentSourceUnavailable {
			return fmt.Errorf("blocked requires action source_unavailable and source_status unavailable")
		}
		if req.HandoffOwner == "" {
			return fmt.Errorf("handoff_owner is required for blocked")
		}
	default:
		return fmt.Errorf("outcome must be resolved, review_ready, routed, or blocked")
	}
	return nil
}

func (s *InternalCommandService) persistSupportCoverageGapOutcome(ctx context.Context, meta model.InternalCommandContext, outcome supportCoverageGapOutcomeSummary) error {
	if s.agentRunRepo == nil {
		return fmt.Errorf("agent run repository is not configured")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(outcome)
	if err != nil {
		return fmt.Errorf("encode support coverage outcome: %w", err)
	}
	if err := s.agentRunRepo.UpdateRuntimeSummaryMarker(ctx, run.WorkspaceID, run.ID, supportCoverageGapOutcomeSummaryKey, payload); err != nil {
		return err
	}
	return nil
}
