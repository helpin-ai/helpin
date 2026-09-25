package service

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetAIProfileService freezes a shared selection in each newly reviewed setup.
func (s *CRMPlaybookService) SetAIProfileService(profiles *AIProfileService) *CRMPlaybookService {
	s.aiProfiles = profiles
	return s
}

// ReviewSharedSelection captures configuration without reading credentials or
// refreshing OAuth. CRM must execute this reviewed route or request a new review.
func (s *AIProfileService) ReviewSharedSelection(ctx context.Context, workspace, agentProfile string) (*model.AIExecutionSelection, error) {
	id := agentProfile
	if id == "" {
		settings, err := s.repo.Settings(ctx, workspace)
		if err != nil {
			return nil, err
		}
		id = derefString(settings.DefaultProfileID)
	}
	if id == "" {
		return nil, errors.New("configure a shared AI profile before reviewing this playbook")
	}
	p, err := s.repo.Get(ctx, workspace, id)
	if err != nil {
		return nil, err
	}
	if p == nil || p.Scope != "workspace" || p.UserID != nil {
		return nil, ErrAIConnection
	}
	if err := s.validateRoute(ctx, workspace, "", "workspace", &p.Primary); err != nil {
		return nil, err
	}
	return &model.AIExecutionSelection{ProfileID: p.ID, ProfileRevision: p.Revision, ConnectionScope: "workspace", Route: p.Primary, Source: "reviewed"}, nil
}

func (s *CRMPlaybookService) compileConnectionWithAIProfile(ctx context.Context, source model.CRMPlaybookConnectionSource) (model.CRMPlaybookConnectionSnapshot, string, error) {
	if s.aiProfiles == nil {
		return compileCRMPlaybookConnection(source)
	}
	selection, err := s.aiProfiles.ReviewSharedSelection(ctx, source.Agent.WorkspaceID, derefString(source.Agent.AIProfileID))
	if err != nil {
		return model.CRMPlaybookConnectionSnapshot{}, "", err
	}
	source.Agent.Provider, source.Agent.Model = &selection.Route.Model.Provider, &selection.Route.Model.Model
	source.Agent.ExecutionConfig, err = executionConfigWithModelControls(source.Agent.ExecutionConfig, selection.Route.Model.Controls)
	if err != nil {
		return model.CRMPlaybookConnectionSnapshot{}, "", err
	}
	snapshot, _, err := compileCRMPlaybookConnection(source)
	if err != nil {
		return snapshot, "", err
	}
	snapshot.AISelection = selection
	fingerprint, err := connectionContentFingerprint(snapshot)
	return snapshot, fingerprint, err
}
