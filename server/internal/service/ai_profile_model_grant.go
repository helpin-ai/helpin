package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Each opt-in receives a new grant ID so a later opt-in cannot revive a revoked run.
func (s *AIProfileService) SetAgentSharing(ctx context.Context, workspace, user, id string, revision int64, shared bool) (*model.AIProfile, error) {
	if err := s.requireMember(ctx, workspace, user); err != nil {
		return nil, err
	}
	p, err := s.repo.Get(ctx, workspace, id)
	if err != nil {
		return nil, err
	}
	if p == nil || p.Scope != "personal" || derefString(p.UserID) != user || p.Primary.Model.Provider != "openai_chatgpt" {
		return nil, ErrAIConnection
	}
	if p.Revision != revision {
		return nil, repository.ErrAIProfileChanged
	}
	var grant *string
	var account string
	if shared {
		c, err := s.connections.repo.Get(ctx, p.Primary.ConnectionID)
		if err != nil {
			return nil, err
		}
		if ownAIConnection(c, workspace, user) != nil || c.Provider != "openai_chatgpt" || c.Scope != "personal" || c.Status != "connected" || c.AccountID == "" || c.SupersededBy != nil {
			return nil, ErrAIConnection
		}
		if p.AgentGrantID != nil {
			return p, nil
		}
		next := uuid.NewString()
		grant = &next
		account = c.AccountID
	}
	if !shared && p.AgentGrantID == nil {
		return p, nil
	}
	if err := s.repo.SetAgentGrant(ctx, p, grant, account); err != nil {
		return nil, err
	}
	if !shared {
		if err := s.connections.revokeSharedModelRuns(ctx, workspace, p.ID, p.Primary.ConnectionID, *p.AgentGrantID); err != nil {
			slog.ErrorContext(ctx, "runtime revocation queued for retry", "workspace_id", workspace, "profile_id", p.ID, "error", err)
		} else if err := s.connections.repo.CompleteModelGrantRevocation(ctx, *p.AgentGrantID); err != nil {
			slog.WarnContext(ctx, "runtime revocation completed, queue acknowledgement pending", "workspace_id", workspace, "profile_id", p.ID, "error", err)
		}
	}
	return s.repo.Get(ctx, workspace, id)
}

func (s *AIProfileService) resolveGrantedAgentModel(ctx context.Context, workspace string, p *model.AIProfile, source string) (*model.AIExecutionSelection, *sdk.ModelCredential, error) {
	if err := s.checkExecutionRoute(ctx, p.Primary); err != nil {
		return nil, nil, err
	}
	policy, err := s.selectionPolicy(ctx, workspace, p.Primary)
	if err != nil {
		return nil, nil, err
	}
	selection := &model.AIExecutionSelection{AgentGrantID: p.AgentGrantID, ProfileID: p.ID, ProfileRevision: p.Revision, Source: source, Route: p.Primary, ConnectionScope: "personal", OwnerID: p.UserID, Policy: policy}
	_, cred, err := s.connections.grantedModelCredential(ctx, workspace, selection, false)
	if err != nil {
		return nil, nil, err
	}
	return selection, cred, nil
}

func (s *AIConnectionService) grantedModelCredential(ctx context.Context, workspace string, selection *model.AIExecutionSelection, force bool, fingerprint ...string) (*model.AIConnection, *sdk.ModelCredential, error) {
	if !s.Enabled() || selection == nil || derefString(selection.AgentGrantID) == "" || derefString(selection.OwnerID) == "" || selection.ConnectionScope != "personal" {
		return nil, nil, ErrAIConnection
	}
	if err := sdk.ValidateRunModel(&selection.Route.Model); err != nil {
		return nil, nil, err
	}
	return s.readAuthorizedCredential(ctx, force, func(read func(*model.AIConnection) error) error {
		return s.repo.WithLockedAgentGrant(ctx, workspace, selection, func(c *model.AIConnection, profile *model.AIProfile) error {
			if c.WorkspaceID != workspace || c.Scope != "personal" || c.Provider != "openai_chatgpt" || c.SupersededBy != nil || c.UserID == nil || *c.UserID != *selection.OwnerID {
				return ErrAIConnection
			}
			if profile.GrantAccountID == "" || c.AccountID != profile.GrantAccountID || selection.Route.Model.Provider != c.Provider || selection.Route.Model.Endpoint != nil || profile.Primary.Model.Model != selection.Route.Model.Model || profile.Primary.ConnectionID != c.ID || profile.Primary.Model.Provider != c.Provider {
				return ErrAIConnection
			}
			return read(c)
		})
	}, fingerprint...)
}

func (s *AIConnectionService) revokeSharedModelRuns(ctx context.Context, workspace, profileID, connectionID, grantID string) error {
	runs, err := s.repo.BoundRuns(ctx, workspace, "", connectionID)
	if err != nil {
		return err
	}
	var revokeErr error
	for _, run := range runs {
		var input model.AgentRunInputPayload
		if err := decodeAIConnectionRunInput(run.Input, &input); err != nil {
			revokeErr = errors.Join(revokeErr, err)
			continue
		}
		if input.AISelection == nil || input.AISelection.ProfileID != profileID || derefString(input.AISelection.AgentGrantID) != grantID {
			continue
		}
		if id, ok := agentRuntimeRunID(&run); ok {
			revokeErr = errors.Join(revokeErr, s.runtime.RevokeRunModelCredential(ctx, id))
		}
	}
	return revokeErr
}

func (s *AIConnectionService) revokeDepartedMemberRuns(ctx context.Context, workspace, user string) error {
	runs, err := s.repo.ActiveGrantedRunsForMember(ctx, workspace, user)
	if err != nil {
		return err
	}
	var revokeErr error
	for _, run := range runs {
		if id, ok := agentRuntimeRunID(&run); ok {
			revokeErr = errors.Join(revokeErr, s.runtime.RevokeRunModelCredential(ctx, id))
		}
	}
	return revokeErr
}

// Retry the durable revocation queue. Repeated revocation of the same runtime
// run is safe, and a later grant on the same profile has a different ID.
func (s *AIConnectionService) RunModelGrantRevocations(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.drainModelGrantRevocations(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *AIConnectionService) drainModelGrantRevocations(ctx context.Context) {
	batch, err := s.repo.DueModelGrantRevocations(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list pending model grant revocations", "error", err)
		return
	}
	for _, job := range batch {
		if ctx.Err() != nil {
			return
		}
		err := s.revokeSharedModelRuns(ctx, job.WorkspaceID, job.ProfileID, job.ConnectionID, job.GrantID)
		if err == nil {
			err = s.repo.CompleteModelGrantRevocation(ctx, job.GrantID)
		} else {
			err = errors.Join(err, s.repo.DelayModelGrantRevocation(ctx, job.GrantID))
		}
		if err != nil {
			slog.ErrorContext(ctx, "revoke shared model run credentials", "workspace_id", job.WorkspaceID, "profile_id", job.ProfileID, "error", err)
		}
	}
}
