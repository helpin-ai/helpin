package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Agents lists policy-compatible agents without exposing workspace credentials.
func (s *CLIService) Agents(ctx context.Context, c *model.CLIConnection) ([]map[string]any, error) {
	actor, err := s.actor(ctx, c.WorkspaceID, c.UserID)
	if err != nil {
		return nil, err
	}
	agents, err := s.agents.ListAgentsForActor(ctx, c.WorkspaceID, actor)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(agents))
	for _, a := range agents {
		_, tools, policyErr := localAgentPolicy(&a, false)
		reason := ""
		if policyErr != nil {
			reason = "This agent has no supported local native execution policy"
		}
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "local_compatible": policyErr == nil, "unsupported_reason": reason, "allowed_tools": tools, "allowed_targets": parseJSONStringSlice(a.AllowedTargets)})
	}
	return out, nil
}

// Admit reserves a request identity and delegates all launch preparation to AgentService.
func (s *CLIService) Admit(ctx context.Context, c *model.CLIConnection, req model.CLIAdmissionRequest) (*model.CLIAdmission, error) {
	if req.ExecutionLocation != "local" || len(req.RequestID) < 8 || len(req.RequestID) > 128 || len(req.Instructions) > 65536 || req.AgentID == "" || len(req.Target) > 256 {
		return nil, ErrCLIInvalid
	}
	current, err := s.connection(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c = current
	actor, err := s.actor(ctx, c.WorkspaceID, c.UserID)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	key := uuid.NewSHA1(uuid.NameSpaceOID, []byte(c.ID+"\x00"+req.RequestID)).String()
	now := time.Now().UTC()
	execution := &model.CLIExecution{ID: key, ConnectionID: c.ID, RequestID: req.RequestID, RequestHash: mcpHashBytes(payload), WorkspaceID: c.WorkspaceID, UserID: c.UserID, RunID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-run:"+key)).String(), Epoch: 1, LeaseExpiresAt: now.Add(15 * time.Minute), CreatedAt: now, UpdatedAt: now}
	created, err := s.repo.Reserve(ctx, execution)
	if err != nil {
		return nil, err
	}
	if !created {
		execution, err = s.repo.Execution(ctx, key)
		if err != nil {
			return nil, err
		}
		if execution == nil || execution.RequestHash != mcpHashBytes(payload) {
			return nil, ErrCLIConflict
		}
		if execution.RevokedAt != nil {
			return nil, ErrCLIUnauthorized
		}
	}
	var run *model.AgentRun
	if created {
		run, err = s.agents.AdmitLocalRun(ctx, c, execution, req, actor)
	} else {
		run, err = s.agents.runRepo.GetByID(ctx, c.WorkspaceID, execution.RunID)
	}
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrCLIConflict
	}
	if !model.IsLocalAgentRun(run) || !model.IsAgentRunActiveStatus(run.Status) {
		return nil, ErrCLIConflict
	}
	if err = s.agents.RequireActorCanUseAgent(ctx, c.WorkspaceID, run.AgentID, actor); err != nil {
		return nil, ErrCLIForbidden
	}
	var input model.AgentRunInputPayload
	if err = json.Unmarshal(run.Input, &input); err != nil {
		return nil, err
	}
	if input.LocalExecution.ConnectionID != c.ID || input.LocalExecution.ExecutionID != execution.ID {
		return nil, ErrCLIForbidden
	}
	snapshot, err := json.Marshal(input.LocalExecution.Admission)
	if err != nil {
		return nil, err
	}
	if err = s.repo.SaveAdmission(ctx, execution.ID, snapshot, mcpHashBytes(snapshot)); err != nil {
		return nil, err
	}
	execution, err = s.repo.Execution(ctx, execution.ID)
	if err != nil {
		return nil, err
	}
	result := input.LocalExecution.Admission
	result.Execution = execution
	return &result, nil
}

// Execution checks grant ownership, current user permissions, and target access.
func (s *CLIService) Execution(ctx context.Context, c *model.CLIConnection, runID string) (*model.CLIExecution, error) {
	return s.execution(ctx, c, runID, false)
}

func (s *CLIService) execution(ctx context.Context, c *model.CLIConnection, runID string, revoking bool) (*model.CLIExecution, error) {
	if _, err := s.connection(ctx, c.ID); err != nil {
		return nil, err
	}
	run, err := s.agents.runRepo.GetByID(ctx, c.WorkspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil || !model.IsLocalAgentRun(run) || (!revoking && !model.IsAgentRunActiveStatus(run.Status)) || derefString(run.TriggeredByUserID) != c.UserID {
		return nil, ErrCLIForbidden
	}
	var input model.AgentRunInputPayload
	if err = json.Unmarshal(run.Input, &input); err != nil {
		return nil, err
	}
	if input.LocalExecution.ConnectionID != c.ID {
		return nil, ErrCLIForbidden
	}
	e, err := s.repo.Execution(ctx, input.LocalExecution.ExecutionID)
	if err != nil {
		return nil, err
	}
	if e == nil || e.ConnectionID != c.ID || e.WorkspaceID != c.WorkspaceID || e.RunID != runID || (!revoking && (e.RevokedAt != nil || e.PolicyHash == "" || len(e.Snapshot) == 0)) {
		return nil, ErrCLIForbidden
	}
	actor, err := s.actor(ctx, c.WorkspaceID, c.UserID)
	if err != nil {
		return nil, err
	}
	if err = s.agents.RequireActorCanUseAgent(ctx, c.WorkspaceID, run.AgentID, actor); err != nil {
		return nil, ErrCLIForbidden
	}
	switch run.TargetType {
	case "task":
		task, err := s.agents.taskRepo.GetRawByID(ctx, run.TargetID)
		if err != nil {
			return nil, err
		}
		if task == nil || task.WorkspaceID != c.WorkspaceID || (actor.Role != "owner" && actor.Role != "admin" && task.TeamID != nil && !actor.IsMemberOfTeam(*task.TeamID)) {
			return nil, ErrCLIForbidden
		}
	case "workspace":
		if run.TargetID != c.WorkspaceID {
			return nil, ErrCLIForbidden
		}
	case "repository":
		repo, err := s.agents.gitService.GetRepositoryByID(ctx, c.WorkspaceID, run.TargetID)
		if err != nil {
			return nil, err
		}
		if repo == nil || repo.Archived || !repo.Selected {
			return nil, ErrCLIForbidden
		}
	default:
		return nil, ErrCLIForbidden
	}
	return e, nil
}

// BindExecution binds exactly one local runtime ID to an unexpired epoch.
func (s *CLIService) BindExecution(ctx context.Context, c *model.CLIConnection, runID string, epoch int64, localID string) (*model.CLIExecution, error) {
	if strings.TrimSpace(localID) == "" || len(localID) > 128 {
		return nil, ErrCLIInvalid
	}
	e, err := s.Execution(ctx, c, runID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.Bind(ctx, e.ID, epoch, localID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCLIConflict
	}
	return s.repo.Execution(ctx, e.ID)
}

// RenewExecution fences expired grants; stale clients cannot renew the new epoch.
func (s *CLIService) RenewExecution(ctx context.Context, c *model.CLIConnection, runID string, epoch int64) (*model.CLIExecution, error) {
	e, err := s.Execution(ctx, c, runID)
	if err != nil {
		return nil, err
	}
	if e.Epoch != epoch {
		return nil, ErrCLIConflict
	}
	ok, err := s.repo.Renew(ctx, e, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCLIConflict
	}
	return s.repo.Execution(ctx, e.ID)
}

// RevokeExecution disables the grant and cancels its undispatched local run.
func (s *CLIService) RevokeExecution(ctx context.Context, c *model.CLIConnection, runID string) error {
	e, err := s.execution(ctx, c, runID, true)
	if err != nil {
		return err
	}
	if err = s.repo.RevokeExecution(ctx, e.ID, time.Now().UTC()); err != nil {
		return err
	}
	run, err := s.agents.runRepo.GetByID(ctx, c.WorkspaceID, runID)
	if err != nil {
		return err
	}
	if run != nil && !model.IsAgentRunActiveStatus(run.Status) {
		return nil
	}
	_, err = s.agents.CancelRun(ctx, c.WorkspaceID, runID, c.UserID)
	return err
}
