package service

import (
	"context"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// A late lifecycle event must not change the backing run or its controls after
// a newer turn has started. Snapshot watermark checks alone only protect text.
func (s *AgentRuntimeProjectionService) ignoreStaleTurnLifecycle(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope) (bool, error) {
	if s.sessionSnapshotRepo == nil || !strings.HasPrefix(event.Type, "run.") || eventDataString(event.Data, "completion_mode") != "explicit" {
		return false, nil
	}
	record, err := s.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || record == nil {
		return false, err
	}
	snapshot, err := model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
	if err != nil || snapshot == nil || snapshot.TurnState == nil {
		return false, err
	}
	state := snapshot.TurnState
	id := eventDataString(event.Data, "turn_id")
	if id == "" {
		return false, nil
	}
	if id != state.TurnID {
		startedAt, err := time.Parse(time.RFC3339Nano, eventDataString(event.Data, "turn_started_at"))
		if err != nil {
			return false, nil
		}
		return !startedAt.After(state.StartedAt), nil
	}
	return state.Phase != "working" && (event.Type == "run.started" || event.Type == "run.resumed"), nil
}
