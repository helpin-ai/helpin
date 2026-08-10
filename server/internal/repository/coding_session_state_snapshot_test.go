package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCodingSessionStateSnapshotUpsertIfNewerRejectsSequenceRegression(t *testing.T) {
	db := setupSanitizeTestDB(t, codingSessionSnapshotTestSchema)
	repo := NewCodingSessionStateSnapshotRepository(db)
	ctx := context.Background()

	applied, err := repo.UpsertIfNewer(ctx, snapshotAtSequence(10, "newer"))
	if err != nil {
		t.Fatalf("insert newer snapshot: %v", err)
	}
	if !applied {
		t.Fatal("initial snapshot was not applied")
	}
	applied, err = repo.UpsertIfNewer(ctx, snapshotAtSequence(9, "stale"))
	if err != nil {
		t.Fatalf("reject stale snapshot: %v", err)
	}
	if applied {
		t.Fatal("stale snapshot was applied")
	}

	stored, err := repo.GetByRun(ctx, "workspace-1", "run-1")
	if err != nil {
		t.Fatalf("load stored snapshot: %v", err)
	}
	if stored == nil || stored.ThroughSequence != 10 {
		t.Fatalf("stored watermark = %#v, want 10", stored)
	}
	var payload map[string]any
	if err := json.Unmarshal(stored.SnapshotPayload, &payload); err != nil {
		t.Fatalf("decode stored payload: %v", err)
	}
	if payload["value"] != "newer" {
		t.Fatalf("stored payload = %#v, want newer snapshot", payload)
	}

	applied, err = repo.UpsertIfNewer(ctx, snapshotAtSequence(11, "newest"))
	if err != nil {
		t.Fatalf("advance snapshot: %v", err)
	}
	if !applied {
		t.Fatal("newest snapshot was not applied")
	}
}

func snapshotAtSequence(sequence int64, value string) *model.CodingSessionStateSnapshot {
	return &model.CodingSessionStateSnapshot{
		ID:              "snapshot-1",
		WorkspaceID:     "workspace-1",
		RunID:           "run-1",
		ThroughSequence: sequence,
		SnapshotPayload: json.RawMessage(`{"value":"` + value + `"}`),
	}
}
