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

func TestCodingSessionSnapshotsByChatPreserveRunHistoryAndScope(t *testing.T) {
	db := setupSanitizeTestDB(t, codingSessionSnapshotTestSchema, `CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, dock_chat_id TEXT)`)
	repo := NewCodingSessionStateSnapshotRepository(db)
	for i, values := range [][3]string{{"run-1", "workspace-1", "chat-1"}, {"run-2", "workspace-1", "chat-1"}, {"run-3", "workspace-1", "chat-2"}, {"run-4", "workspace-2", "chat-1"}} {
		if err := db.Exec(`INSERT INTO agent_runs VALUES (?,?,?)`, values[0], values[1], values[2]).Error; err != nil {
			t.Fatal(err)
		}
		row := snapshotAtSequence(int64(i+1), "plan")
		row.ID = "snapshot-" + values[0]
		row.RunID = values[0]
		row.WorkspaceID = values[1]
		if err := repo.Upsert(context.Background(), row); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.ListByDockChat(context.Background(), "workspace-1", "chat-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("wanted both chat runs only, got %#v", rows)
	}
	for _, row := range rows {
		if row.RunID != "run-1" && row.RunID != "run-2" {
			t.Fatal("cross-chat snapshot exposed")
		}
	}
}
