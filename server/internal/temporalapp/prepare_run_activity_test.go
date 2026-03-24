package temporalapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPrepareRunActivityMarksRunRunning(t *testing.T) {
	dbName := fmt.Sprintf("file:prepare-run-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&model.Agent{}, &model.AgentRun{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	agent := &model.Agent{
		ID:          "agent-1",
		WorkspaceID: "workspace-1",
		Name:        "Planner",
		PresetKey:   model.AgentPresetEpicPlanner,
		RuntimeKind: "native_sdk",
		Status:      "idle",
	}
	if err := agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	run := &model.AgentRun{
		ID:            "run-1",
		WorkspaceID:   agent.WorkspaceID,
		AgentID:       agent.ID,
		TargetType:    "noop",
		TargetID:      "target-1",
		Status:        model.AgentRunStatusQueued,
		Input:         []byte(`{}`),
		OutputSummary: []byte(`{}`),
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	activities := &AgentRunActivities{
		runRepo:   runRepo,
		agentRepo: agentRepo,
	}

	if err := activities.PrepareRunActivity(context.Background(), run.ID); err != nil {
		t.Fatalf("prepare run activity: %v", err)
	}

	updated, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated run")
	}
	if updated.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected run status %q, got %q", model.AgentRunStatusRunning, updated.Status)
	}
	if updated.StartedAt == nil {
		t.Fatal("expected started_at to be set")
	}
	if updated.ExecutionStage == nil || *updated.ExecutionStage != "preparing" {
		t.Fatalf("expected execution stage preparing, got %#v", updated.ExecutionStage)
	}
	if updated.LastHeartbeatAt == nil {
		t.Fatal("expected last_heartbeat_at to be set")
	}
}
