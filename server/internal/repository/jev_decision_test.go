package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestJevDecisionAdmissionLifecycle(t *testing.T) {
	_, db := setupPMTriageStore(t)
	if err := db.Exec(`CREATE TABLE jev_decision_attempts(id TEXT PRIMARY KEY,workspace_id TEXT,feature TEXT,source_id TEXT,input_hash TEXT,mode TEXT,status TEXT,outcome TEXT,created_at DATETIME,updated_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewJevDecisionRepository(db)
	ctx := context.Background()
	input := model.JevDecisionAttempt{WorkspaceID: "workspace", Feature: "meeting_routing", SourceID: "source", InputHash: "hash", Mode: "primary"}
	first, err := repo.Reserve(ctx, input, 2)
	if err != nil || !first.CallProvider {
		t.Fatalf("admission %v %v", first, err)
	}
	pending, err := repo.Reserve(ctx, input, 2)
	if err != nil || pending.CallProvider || pending.Attempt.ID != first.Attempt.ID {
		t.Fatalf("pending deduplication %v %v", pending, err)
	}
	if err := repo.Finish(ctx, "other", first.Attempt.ID, "ready", model.JSONB{}); err == nil {
		t.Fatal("cross-workspace settlement accepted")
	}
	if err := db.Model(&model.JevDecisionAttempt{}).Where("id = ?", first.Attempt.ID).UpdateColumn("created_at", time.Now().UTC().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	retry, err := repo.Reserve(ctx, input, 2)
	if err != nil || !retry.CallProvider || retry.Attempt.ID == first.Attempt.ID {
		t.Fatalf("abandoned attempt not retried: %v %v", retry, err)
	}
	if err := repo.Finish(ctx, "workspace", first.Attempt.ID, "ready", model.JSONB{}); err == nil {
		t.Fatal("stale provider completion accepted")
	}
	if err := repo.Finish(ctx, "workspace", retry.Attempt.ID, "ready", model.JSONB{"choice": "internal"}); err != nil {
		t.Fatal(err)
	}
	cached, err := repo.Reserve(ctx, input, 2)
	if err != nil || cached.CallProvider || cached.Limited || cached.Attempt.ID != retry.Attempt.ID {
		t.Fatalf("cache failed %v %v", cached, err)
	}
	input.InputHash = "new-hash"
	limited, err := repo.Reserve(ctx, input, 2)
	if err != nil || !limited.Limited {
		t.Fatalf("attempts not capped %v %v", limited, err)
	}
	input.Feature = "answer_evidence"
	independent, err := repo.Reserve(ctx, input, 2)
	if err != nil || !independent.CallProvider {
		t.Fatalf("feature budget not independent %v %v", independent, err)
	}
}
