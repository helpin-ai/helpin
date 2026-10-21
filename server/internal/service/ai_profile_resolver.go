package service

import (
	"context"
	"errors"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIProfileSelectionRequest is an internal, authorized launch context, never an HTTP DTO.
type AIProfileSelectionRequest struct {
	ProfileID      string
	AgentProfileID string
	Unattended     bool
	// UsePersonalDefault is set only by the new Ask Agent chat launch surface.
	UsePersonalDefault bool
	// Direct means the caller executes the route itself instead of launching it
	// through Agent Runtime, so runtime readiness is only required where the
	// runtime approves the destination (compatible endpoints).
	Direct bool
}

// Resolve selects once before admission. Only a known unavailable connection
// permits fallback; invalid selections and authorization errors are terminal.
func (s *AIProfileService) Resolve(ctx context.Context, workspace, user string, req AIProfileSelectionRequest) (*model.AIExecutionSelection, *sdk.ModelCredential, error) {
	if user != "" || !req.Unattended {
		if err := s.requireMember(ctx, workspace, user); err != nil {
			return nil, nil, err
		}
	}
	id, source := req.ProfileID, "override"
	if id == "" {
		id, source = req.AgentProfileID, "agent"
	}
	if id == "" && req.UsePersonalDefault && !req.Unattended && user != "" {
		settings, err := s.repo.PersonalSettings(ctx, workspace, user)
		if err != nil {
			return nil, nil, err
		}
		id, source = derefString(settings.DefaultProfileID), "personal"
	}
	if id == "" {
		settings, err := s.repo.Settings(ctx, workspace)
		if err != nil {
			return nil, nil, err
		}
		id, source = derefString(settings.DefaultProfileID), "workspace"
	}
	if id == "" {
		return nil, nil, errors.New("configure a workspace AI default or select a profile")
	}
	p, err := s.repo.Get(ctx, workspace, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.authorizeProfile(ctx, workspace, user, p, false); err != nil {
		return nil, nil, err
	}
	if p.Scope == "personal" && (req.Unattended || (source != "override" && source != "personal")) {
		return nil, nil, errors.New("personal profiles require an explicit manual selection")
	}
	if err := s.validatePrimaryRoute(ctx, workspace, user, p.Scope, &p.Primary); err != nil {
		return nil, nil, err
	}
	if p.Fallback != nil {
		if err := s.validateRoute(ctx, workspace, user, p.Scope, p.Fallback); err != nil {
			return nil, nil, err
		}
	}
	selection := &model.AIExecutionSelection{ProfileID: p.ID, ProfileRevision: p.Revision, Source: source, Route: p.Primary}
	if p.Scope == "personal" {
		selection.ProfileOwnerID = p.UserID
	}
	selection.Policy, err = s.selectionPolicy(ctx, workspace, p.Primary)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkSelectedRoute(ctx, p.Primary, req.Direct); err != nil {
		return nil, nil, err
	}
	c, credential, err := s.routeCredential(ctx, workspace, user, req.Unattended, p.Primary)
	if errors.Is(err, ErrAIConnectionUnavailable) && p.Fallback != nil {
		selection.Route, selection.FallbackReason = *p.Fallback, "primary_connection_unavailable"
		selection.Policy, err = s.selectionPolicy(ctx, workspace, *p.Fallback)
		if err != nil {
			return nil, nil, err
		}
		if err := s.checkSelectedRoute(ctx, *p.Fallback, req.Direct); err != nil {
			return nil, nil, err
		}
		c, credential, err = s.routeCredential(ctx, workspace, user, req.Unattended, *p.Fallback)
	}
	if err != nil {
		return nil, nil, err
	}
	selection.ConnectionScope, selection.OwnerID = c.Scope, c.UserID
	return selection, credential, nil
}

// checkSelectedRoute keeps Agent Runtime readiness for runtime launches. A
// direct completion does not use the runtime, except that a compatible
// endpoint must still be one the runtime configuration approves.
func (s *AIProfileService) checkSelectedRoute(ctx context.Context, route model.AIProfileRoute, direct bool) error {
	if direct && route.Model.Provider != "openai_compatible" {
		return nil
	}
	return s.checkExecutionRoute(ctx, route)
}

func (s *AIProfileService) routeCredential(ctx context.Context, workspace, user string, unattended bool, route model.AIProfileRoute) (*model.AIConnection, *sdk.ModelCredential, error) {
	if unattended {
		return s.connections.sharedCredential(ctx, workspace, route.ConnectionID)
	}
	return s.connections.Credential(ctx, workspace, user, route.ConnectionID, false)
}

// Restore reauthorizes the accepted connection without consulting editable profiles
// or trying a fallback. Its selection must come from trusted persisted run data.
func (s *AIProfileService) Restore(ctx context.Context, workspace, user string, selection *model.AIExecutionSelection) (*sdk.ModelCredential, error) {
	if selection == nil {
		return nil, ErrAIConnection
	}
	if user != "" {
		if err := s.requireMember(ctx, workspace, user); err != nil {
			return nil, err
		}
	}
	if err := sdk.ValidateRunModel(&selection.Route.Model); err != nil {
		return nil, err
	}
	if owner, personal := selection.PersonalOwner(); personal && (user == "" || owner != user) {
		return nil, ErrAIConnection
	}
	if err := s.checkExecutionRoute(ctx, selection.Route); err != nil {
		return nil, err
	}
	c, credential, err := s.routeCredential(ctx, workspace, user, selection.ConnectionScope == "workspace", selection.Route)
	if err != nil {
		return nil, err
	}
	if !sameModelEndpoint(c.Endpoint, selection.Route.Model.Endpoint) || c.Provider != selection.Route.Model.Provider || c.Scope != selection.ConnectionScope || derefString(c.UserID) != derefString(selection.OwnerID) {
		return nil, ErrAIConnection
	}
	return credential, nil
}
