package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func persistRuntimeSummaryMarker(ctx context.Context, repo agentRuntimeProjectionRunRepository, run *model.AgentRun, key string) error {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(run.OutputSummary, &body); err != nil {
		return err
	}
	value, ok := body[key]
	if !ok {
		return fmt.Errorf("runtime summary marker %q missing", key)
	}
	return repo.UpdateRuntimeSummaryMarker(ctx, run.WorkspaceID, run.ID, key, value)
}
