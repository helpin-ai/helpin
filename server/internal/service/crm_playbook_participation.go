package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Preview reads qualifying existing Signals only; it never executes a Flow or creates a customer process.
func (s *CRMPlaybookService) Preview(ctx context.Context, ws, id, versionID string, revision int64, page, size int) (*model.CRMPlaybookPreview, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMRead)
	if err != nil {
		return nil, err
	}
	if !validSituationID(id) || (versionID != "" && !validSituationID(versionID)) || revision < 1 {
		return nil, ErrCRMPlaybookInput
	}
	page, size, err = playbookPage(page, size)
	if err != nil {
		return nil, err
	}
	result, err := s.store.Preview(ctx, ws, id, actor.WorkspaceMemberID, versionID, revision, page, size)
	if err == nil && result == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	return result, err
}

// Participants queries the same work used by Signals; no copied lifecycle or owners exist.
func (s *CRMPlaybookService) Participants(ctx context.Context, ws, id string, filters model.CRMSituationListFilters) (*model.CRMSituationList, error) {
	if _, err := s.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	filters.PlaybookID = id
	if filters.Scope == "" {
		filters.Scope = "all"
	}
	if filters.State == "" {
		filters.State = "all"
	}
	return s.situations.List(ctx, ws, filters)
}

// Apply attaches a published policy after explicit confirmation and fresh eligibility checks.
func (s *CRMPlaybookService) Apply(ctx context.Context, ws, id string, req model.ApplyCRMPlaybookRequest) (*model.CRMSituationCommandResult, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMEdit)
	if err != nil {
		return nil, err
	}
	req.CommandKey = strings.TrimSpace(req.CommandKey)
	if !validSituationID(id) || !validSituationID(req.SituationID) || !validSituationID(req.VersionID) ||
		!validPlaybookKey(req.CommandKey) || req.ExpectedPlaybookRevision < 1 || req.ExpectedSituationRevision < 1 || !req.Confirmed {
		return nil, ErrCRMPlaybookInput
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, struct {
		PlaybookID string
		Request    model.ApplyCRMPlaybookRequest
	}{PlaybookID: id, Request: req})
	if err != nil {
		return nil, err
	}
	result, err := s.store.Apply(ctx, ws, id, actor.WorkspaceMemberID, req, fingerprint)
	if err == nil && result == nil {
		return nil, ErrCRMSituationNotFound
	}
	return result, err
}

// AssessMilestone records explicitly attributed progress, not execution success or an inferred outcome.
func (s *CRMPlaybookService) AssessMilestone(ctx context.Context, ws, id, situationID string, req model.CRMPlaybookMilestoneRequest) (*model.CRMSituationCommandResult, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMEdit)
	if err != nil {
		return nil, err
	}
	req.CommandKey, req.Summary = strings.TrimSpace(req.CommandKey), strings.TrimSpace(req.Summary)
	if !validSituationID(id) || !validSituationID(situationID) || !validPlaybookKey(req.CommandKey) || req.ExpectedRevision < 1 ||
		!validPlaybookMilestoneKey(req.MilestoneKey) || req.Summary == "" || len(req.Summary) > 2000 ||
		(req.Status != "pending" && req.Status != "achieved" && req.Status != "not_applicable") {
		return nil, ErrCRMPlaybookInput
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, struct {
		PlaybookID string
		Request    model.CRMPlaybookMilestoneRequest
	}{PlaybookID: id, Request: req})
	if err != nil {
		return nil, err
	}
	result, err := s.store.AssessMilestone(ctx, ws, id, situationID, actor.WorkspaceMemberID, req, fingerprint)
	if err == nil && result == nil {
		return nil, ErrCRMSituationNotFound
	}
	return result, err
}
