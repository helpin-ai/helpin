package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const skillEditProposalKind = "skill_edit"

type skillTextReplacement struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

// Match every edit against the same original text. Reject overlaps and ambiguous
// matches instead of risking changes outside the passage the user reviewed.
func applySkillReplacements(original string, edits []skillTextReplacement) (string, error) {
	if len(edits) > 20 {
		return "", fmt.Errorf("at most 20 replacements are allowed")
	}
	type span struct {
		start, end int
		text       string
	}
	spans := make([]span, 0, len(edits))
	for _, edit := range edits {
		if edit.OldText == "" || strings.Count(original, edit.OldText) != 1 {
			return "", fmt.Errorf("old_text must match exactly one passage; read more surrounding text to disambiguate")
		}
		start := strings.Index(original, edit.OldText)
		end := start + len(edit.OldText)
		for _, existing := range spans {
			if start < existing.end && end > existing.start {
				return "", fmt.Errorf("replacement passages overlap")
			}
		}
		spans = append(spans, span{start, end, edit.NewText})
	}
	// Apply from right to left so earlier offsets stay stable.
	sort.Slice(spans, func(i, j int) bool { return spans[i].start > spans[j].start })
	for _, edit := range spans {
		original = original[:edit.start] + edit.text + original[edit.end:]
	}
	return original, nil
}

type preparedSkillSaveInput struct {
	ProposalID string `json:"proposal_id"`
	Summary    string `json:"summary"`
}

func (s *InternalCommandService) validateSkillUpdateCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	req, err := s.resolveSkillUpdate(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	if s.dockActionProposalRepo == nil || meta.RunID == "" {
		return nil, fmt.Errorf("skill preparation requires an active agent run")
	}
	changes := []string{}
	if req.Title != nil {
		changes = append(changes, "title")
	}
	if req.Description != nil {
		changes = append(changes, "description")
	}
	if req.Instructions != nil {
		changes = append(changes, "instructions")
	}
	if req.RestoreDefault {
		changes = append(changes, "restore defaults")
	}
	summary := fmt.Sprintf("Update skill %s: %s", req.reviewTitle, strings.Join(changes, ", "))
	// Store the resolved instructions, not a patch that could apply differently later.
	replacements := req.Replacements
	req.Replacements = nil
	spec, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	proposal := &model.DockActionProposal{ID: uuid.NewString(), WorkspaceID: meta.WorkspaceID, DockChatRunID: meta.RunID, ActorID: meta.ActorID, Kind: skillEditProposalKind, Status: model.DockActionProposalStatusPrepared, Summary: summary, Spec: spec, Usage: json.RawMessage(`{}`), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := s.dockActionProposalRepo.Create(ctx, proposal); err != nil {
		return nil, err
	}
	return marshalSkillToolResult(struct {
		Prepared     bool                   `json:"prepared"`
		Saved        bool                   `json:"saved"`
		SaveInput    preparedSkillSaveInput `json:"save_input"`
		Changes      []string               `json:"changed_fields"`
		Replacements []skillTextReplacement `json:"replacements,omitempty"`
		Next         string                 `json:"next_step"`
	}{true, false, preparedSkillSaveInput{proposal.ID, summary}, changes, replacements, "Review the proposed changes, then call update_skill with save_input exactly. Preparation grants no mutation approval."})
}

func (s *InternalCommandService) loadPreparedSkillEdit(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (*model.DockActionProposal, *skillUpdateInput, error) {
	var req preparedSkillSaveInput
	if err := decodeSkillCommand(input, &req); err != nil {
		return nil, nil, fmt.Errorf("prepare the edit with validate_skill_update and use its save_input: %w", err)
	}
	if _, err := uuid.Parse(req.ProposalID); err != nil {
		return nil, nil, fmt.Errorf("a valid prepared proposal_id is required; call validate_skill_update first")
	}
	if s.dockActionProposalRepo == nil || meta.RunID == "" {
		return nil, nil, fmt.Errorf("skill saving requires an active agent run")
	}
	proposal, err := s.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, req.ProposalID)
	if err != nil {
		return nil, nil, err
	}
	if proposal == nil || proposal.Kind != skillEditProposalKind || proposal.ActorID != meta.ActorID || proposal.DockChatRunID != meta.RunID || proposal.Status != model.DockActionProposalStatusPrepared || !proposal.ExpiresAt.After(time.Now()) {
		return nil, nil, fmt.Errorf("prepared skill edit is unavailable or expired; prepare the change again")
	}
	if req.Summary != proposal.Summary {
		return nil, nil, fmt.Errorf("review summary differs from prepared edit; use save_input unchanged")
	}
	resolved, err := s.resolveSkillUpdate(ctx, meta, proposal.Spec)
	if err != nil {
		return nil, nil, err
	}
	return proposal, resolved, nil
}
func (s *InternalCommandService) validatePreparedSkillUpdate(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) error {
	_, _, err := s.loadPreparedSkillEdit(ctx, meta, input)
	return err
}
func (s *InternalCommandService) updateSkillCommand(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	proposal, req, err := s.loadPreparedSkillEdit(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	claimed, err := s.dockActionProposalRepo.ClaimPreparedSkillEdit(ctx, meta.WorkspaceID, proposal.ID, meta.RunID, meta.ActorID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, fmt.Errorf("prepared edit has already been claimed or expired")
	}
	result, err := s.agentService.UpdateWorkspaceSkill(ctx, meta.WorkspaceID, req.SkillID, model.UpdateWorkspaceSkillRequest{ExpectedVersion: req.ExpectedVersion, Title: req.Title, Description: req.Description, Instructions: req.Instructions, RestoreDefault: req.RestoreDefault})
	if err != nil {
		if resetErr := s.dockActionProposalRepo.ResetSkillEditClaim(ctx, meta.WorkspaceID, proposal.ID); resetErr != nil {
			slog.ErrorContext(ctx, "failed to release prepared skill edit", "workspace_id", meta.WorkspaceID, "proposal_id", proposal.ID, "error", resetErr)
		}
		return nil, err
	}
	completed, completeErr := s.dockActionProposalRepo.Complete(ctx, meta.WorkspaceID, proposal.ID, time.Now().UTC())
	if completeErr != nil || !completed {
		slog.ErrorContext(ctx, "failed to complete saved skill proposal", "workspace_id", meta.WorkspaceID, "proposal_id", proposal.ID, "error", completeErr)
	}
	return json.Marshal(map[string]any{"saved": true, "id": result.ID, "skill_id": result.ID, "key": result.Key, "version_key": result.VersionKey})
}
