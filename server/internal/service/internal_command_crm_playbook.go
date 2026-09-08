package service

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetCRMPlaybookActions connects typed preparation to the existing command surface.
func (s *InternalCommandService) SetCRMPlaybookActions(actions *CRMPlaybookActionService) {
	s.crmPlaybookActions = actions
}

func (s *InternalCommandService) registerCRMPlaybookCommands() {
	s.register(InternalCommandDefinition{Name: "crm.get_playbook_context", Module: "crm", Tool: mustCommandToolMetadata("crm.get_playbook_context"), Execute: s.executeCRMPlaybookContext})
	s.register(InternalCommandDefinition{Name: "crm.propose_playbook_action", Module: "crm", Mutating: true, Tool: mustCommandToolMetadata("crm.propose_playbook_action"), Execute: s.executeCRMPlaybookProposal})
}

func (s *InternalCommandService) executeCRMPlaybookContext(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmPlaybookActions == nil {
		return nil, ErrCRMPlaybookExecutionNotReady
	}
	if len(input) > 0 {
		var values map[string]json.RawMessage
		if json.Unmarshal(input, &values) != nil || len(values) != 0 {
			return nil, ErrCRMPlaybookInput
		}
	}
	result, err := s.crmPlaybookActions.Context(ctx, meta.WorkspaceID, meta.RunID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func (s *InternalCommandService) executeCRMPlaybookProposal(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmPlaybookActions == nil {
		return nil, ErrCRMPlaybookExecutionNotReady
	}
	result, err := s.crmPlaybookActions.Propose(ctx, meta.WorkspaceID, meta.RunID, input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
