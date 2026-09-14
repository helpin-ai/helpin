package service

import (
	"context"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetAuthorizationService enables permission-checked management of shared connections.
func (s *AIConnectionService) SetAuthorizationService(authz *authorization.AuthzService) {
	s.authz = authz
}

func (s *AIConnectionService) requireConnectionManager(ctx context.Context, workspace, user string) error {
	if s.authz == nil {
		return ErrAIConnection
	}
	actor, err := s.authz.ResolveActor(ctx, workspace, user)
	if err != nil || !s.authz.Can(actor, authorization.PermWorkspaceUpdate) {
		return ErrAIConnection
	}
	return nil
}

func (s *AIConnectionService) authorizeConnectionManagement(ctx context.Context, workspace, user, id string) error {
	c, err := s.repo.Get(ctx, id)
	if err != nil || c == nil || c.WorkspaceID != workspace {
		return ErrAIConnection
	}
	if c.Funding == "managed" {
		return ErrAIConnection
	}
	if c.Scope == "workspace" && c.UserID == nil {
		return s.requireConnectionManager(ctx, workspace, user)
	}
	return ownAIConnection(c, workspace, user)
}

// Access to shared secrets is checked by the calling entry point: active member
// for interactive use, active workspace for the trusted unattended resolver.
func accessAIConnection(c *model.AIConnection, workspace, user string) error {
	if c != nil && c.WorkspaceID == workspace && c.Scope == "workspace" && c.UserID == nil {
		return nil
	}
	return ownAIConnection(c, workspace, user)
}

func (s *AIConnectionService) sharedCredential(ctx context.Context, workspace, id string) (*model.AIConnection, *sdk.ModelCredential, error) {
	if !s.Enabled() {
		return nil, nil, ErrAIConnection
	}
	active, err := s.repo.ActiveWorkspace(ctx, workspace)
	if err != nil || !active {
		return nil, nil, ErrAIConnection
	}
	// Empty user can only authorize a workspace-owned connection.
	return s.loadConnectionCredential(ctx, workspace, "", id, false)
}

func (s *AIConnectionService) boundConnectionRuns(ctx context.Context, workspace, user, id string) ([]model.AgentRun, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil || c == nil || c.WorkspaceID != workspace {
		return nil, ErrAIConnection
	}
	if c.Scope == "workspace" {
		user = ""
	}
	return s.repo.BoundRuns(ctx, workspace, user, id)
}

func (s *AIConnectionService) authorizeAcceptedSelectionOwner(ctx context.Context, run *model.AgentRun, selection *model.AIExecutionSelection) error {
	owner, personal := selection.PersonalOwner()
	if !personal {
		return nil
	}
	if owner == "" || owner != derefString(run.TriggeredByUserID) {
		return ErrAIConnection
	}
	active, err := s.repo.ActiveMember(ctx, run.WorkspaceID, owner)
	if err != nil {
		return err
	}
	if !active {
		return ErrAIConnection
	}
	return nil
}
