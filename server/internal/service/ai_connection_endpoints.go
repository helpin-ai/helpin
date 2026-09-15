package service

import (
	"context"
	"errors"

	sdk "github.com/helpin-ai/agent-runtime-go"
)

// AvailableEndpoints lists only destinations approved for this Runtime app.
// Users select an ID; browser requests never supply an endpoint URL.
func (s *AIConnectionService) AvailableEndpoints(ctx context.Context, workspace, user string) ([]sdk.ModelEndpoint, error) {
	if !s.Enabled() {
		return nil, ErrAIConnection
	}
	member, err := s.repo.ActiveMember(ctx, workspace, user)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrAIConnection
	}
	capabilities, err := s.runtime.client.GetCapabilities(ctx)
	if err != nil {
		return nil, err
	}
	for _, app := range capabilities.Apps {
		if app.AppID == s.cfg.AppID {
			return app.ModelEndpoints, nil
		}
	}
	return nil, nil
}

func (s *AIConnectionService) resolveEndpoint(ctx context.Context, workspace, user, id string) (*sdk.ModelEndpoint, error) {
	endpoints, err := s.AvailableEndpoints(ctx, workspace, user)
	if err != nil {
		return nil, err
	}
	for _, endpoint := range endpoints {
		if endpoint.ID == id {
			if err := sdk.ValidateModelEndpoint(&endpoint); err != nil {
				return nil, err
			}
			return &endpoint, nil
		}
	}
	return nil, errors.New("select an administrator-approved compatible endpoint")
}

func sameModelEndpoint(a, b *sdk.ModelEndpoint) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
