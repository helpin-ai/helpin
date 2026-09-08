package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMPlaybookSetupService supplies familiar Flow/Beacon defaults for explicit configuration review.
type CRMPlaybookSetupService struct {
	execution *CRMPlaybookExecutionService
	store     *repository.CRMPlaybookExecutionRepository
	agents    *AgentService
}

func NewCRMPlaybookSetupService(execution *CRMPlaybookExecutionService, store *repository.CRMPlaybookExecutionRepository, agents *AgentService) *CRMPlaybookSetupService {
	return &CRMPlaybookSetupService{execution: execution, store: store, agents: agents}
}

// Prepare creates no run, publication, enrollment or live automation setting.
func (s *CRMPlaybookSetupService) Prepare(ctx context.Context, ws, id string, req model.PrepareCRMPlaybookSetupRequest) (*model.CRMPlaybookConnectionReview, error) {
	actor, err := s.execution.playbooks.authorizeConnection(ctx, ws)
	if err != nil {
		return nil, err
	}
	if !validSituationID(id) || !validSituationID(req.PlaybookVersionID) || req.ExpectedRevision < 1 {
		return nil, ErrCRMPlaybookInput
	}
	if err := s.execution.requireEntitlements(ctx, ws); err != nil {
		return nil, err
	}
	setup, err := s.store.PrepareSetup(ctx, ws, id, actor.UserID, actor.WorkspaceMemberID, req, func() (*model.Agent, error) {
		key := model.AgentPresetCRMOperator
		version := defaultPresetVersionKeyForPresetKey(key)
		preset, ok := agentPresetVersionDefinition(key, version)
		if !ok {
			return nil, ErrCRMPlaybookConnectionUnsupported
		}
		agent := newBuiltInAgentRecord(ws, key, version, preset)
		agent.ID = uuid.NewString()
		if err := s.agents.validateModelRouting(agent); err != nil {
			return nil, err
		}
		return agent, nil
	})
	if err != nil {
		return nil, err
	}
	return s.execution.playbooks.ReviewConnection(ctx, ws, id, model.CRMPlaybookConnectionSelection{PlaybookVersionID: req.PlaybookVersionID, ExpectedRevision: req.ExpectedRevision, FlowID: setup.FlowID, AgentID: setup.AgentID})
}
