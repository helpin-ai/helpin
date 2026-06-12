package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupCommandBarPlanTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:command_bar_plan_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE command_bar_plans (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			status TEXT NOT NULL DEFAULT 'running',
			prompt TEXT NOT NULL,
			page_context BLOB NOT NULL DEFAULT '{}',
			steps BLOB NOT NULL DEFAULT '[]',
			run_ids_by_step BLOB NOT NULL DEFAULT '{}',
			current_step_index INTEGER NOT NULL DEFAULT 0,
			run_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			cancelled_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func pageContext(t *testing.T, entityType, entityID string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"entity_type": entityType, "entity_id": entityID})
	if err != nil {
		t.Fatalf("marshal page_context: %v", err)
	}
	return raw
}

func seedPlan(t *testing.T, db *gorm.DB, id, actorID, entityType, entityID string, createdAt time.Time) {
	t.Helper()
	actor := actorID
	rec := &model.CommandBarPlanRecord{
		ID:          id,
		WorkspaceID: "ws-1",
		ActorID:     &actor,
		Status:      model.CommandBarPlanStatusRunning,
		Prompt:       "do the thing",
		PageContext:  pageContext(t, entityType, entityID),
		Steps:        json.RawMessage(`[]`),
		RunIDsByStep: json.RawMessage(`{}`),
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed plan %s: %v", id, err)
	}
}

func TestCommandBarPlanRepositoryListByEntity(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	repo := NewCommandBarPlanRepository(db)
	ctx := context.Background()

	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	// Two plans for epic E, created by different actors, at different times.
	seedPlan(t, db, "plan-new", "actor-a", "epic", "epic-E", base.Add(2*time.Hour))
	seedPlan(t, db, "plan-old", "actor-b", "epic", "epic-E", base)
	// Noise: a different epic and a task target.
	seedPlan(t, db, "plan-other-epic", "actor-a", "epic", "epic-F", base.Add(time.Hour))
	seedPlan(t, db, "plan-task", "actor-a", "task", "epic-E", base.Add(time.Hour))

	got, err := repo.ListByEntity(ctx, "ws-1", "epic", "epic-E", 20)
	if err != nil {
		t.Fatalf("ListByEntity: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 plans for epic-E, got %d (%v)", len(got), planIDs(got))
	}
	// Newest first, regardless of which actor triggered them.
	if got[0].ID != "plan-new" || got[1].ID != "plan-old" {
		t.Errorf("expected [plan-new plan-old] newest-first, got %v", planIDs(got))
	}
	// Cross-actor visibility: the actor-b plan must be returned even though we
	// passed no actor filter.
	if got[1].ActorID == nil || *got[1].ActorID != "actor-b" {
		t.Errorf("expected plan-old to belong to actor-b, got %+v", got[1].ActorID)
	}
}

func TestCommandBarPlanRepositoryListByEntityExcludesOtherTargets(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	repo := NewCommandBarPlanRepository(db)
	ctx := context.Background()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	seedPlan(t, db, "plan-task", "actor-a", "task", "task-1", now)
	seedPlan(t, db, "plan-other-epic", "actor-a", "epic", "epic-F", now)

	got, err := repo.ListByEntity(ctx, "ws-1", "epic", "epic-E", 20)
	if err != nil {
		t.Fatalf("ListByEntity: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no plans for epic-E, got %v", planIDs(got))
	}
}

func planIDs(plans []model.CommandBarPlanRecord) []string {
	ids := make([]string, 0, len(plans))
	for _, p := range plans {
		ids = append(ids, p.ID)
	}
	return ids
}
