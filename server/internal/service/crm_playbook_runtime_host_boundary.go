package service

import (
	"context"
	"fmt"
	"strings"
)

// Metadata can demand a stricter boundary, but cannot confer authority. Always
// inspect the stored host run as well: removing metadata must not downgrade a
// Playbook run into ordinary Beacon execution or current-catalogue skill lookup.
func (s *AgentRuntimeHostService) rejectUnclaimedCRMPlaybookCallback(ctx context.Context, runtimeRunID string, metadata ...map[string]interface{}) error {
	for _, values := range metadata {
		for _, key := range []string{"crm_playbook", "crm_playbook_connection_id", "crm_playbook_situation_id"} {
			if _, ok := values[key]; ok {
				return fmt.Errorf("%w: %s", ErrAgentRuntimeHostForbidden, ErrCRMPlaybookExecutionNotReady)
			}
		}
	}
	if s == nil || s.runRepo == nil {
		return nil
	}
	runtimeRunID = strings.TrimSpace(runtimeRunID)
	if runtimeRunID != "" {
		run, err := s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, runtimeRunID)
		if err != nil {
			return err
		}
		if run != nil {
			if err := rejectUnclaimedCRMPlaybookRun(run.Input); err != nil {
				return fmt.Errorf("%w: Playbook run requires guarded execution", ErrAgentRuntimeHostForbidden)
			}
		}
	}
	// Callbacks can arrive before StartRun's remote-ID mapping has been saved.
	// The host-run hint can only restrict access here; it never enables a run.
	if hostRunID := runtimeMetadataString("helpin_run_id", metadata...); hostRunID != "" {
		run, err := s.runRepo.GetByIDAny(ctx, hostRunID)
		if err != nil {
			return err
		}
		if run != nil {
			if err := rejectUnclaimedCRMPlaybookRun(run.Input); err != nil {
				return fmt.Errorf("%w: Playbook run requires guarded execution", ErrAgentRuntimeHostForbidden)
			}
		}
	}
	return nil
}
