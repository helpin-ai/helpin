package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupPMTriageStore(t *testing.T) (*PMTriageRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pm_triage_store_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces (id) VALUES ('workspace'), ('other')`,
		`CREATE TABLE pm_triage_assessments (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, actor_id TEXT NOT NULL, source_kind TEXT NOT NULL, source_id TEXT NOT NULL, source_hash TEXT NOT NULL, context_hash TEXT NOT NULL, mode TEXT NOT NULL, status TEXT NOT NULL, outcome TEXT NOT NULL, reviewed TEXT NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewPMTriageRepository(db), db
}
func triageAttempt() model.PMTriageAssessment {
	return model.PMTriageAssessment{WorkspaceID: "workspace", ActorID: "actor", SourceKind: "task", SourceID: "task", SourceHash: "source-hash", ContextHash: "context-hash", Mode: "primary"}
}
func TestPMTriageCachesReadyResult(t *testing.T) {
	repo, _ := setupPMTriageStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, triageAttempt(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !first.CallProvider {
		t.Fatal("first assessment not admitted")
	}
	if err := repo.Finish(ctx, "workspace", "actor", first.Assessment.ID, "ready", model.JSONB{"actionable": true}); err != nil {
		t.Fatal(err)
	}
	cached, err := repo.Reserve(ctx, triageAttempt(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if cached.CallProvider || cached.Limited || cached.Assessment.ID != first.Assessment.ID || cached.Assessment.Outcome["actionable"] != true {
		t.Fatalf("did not reuse ready result: %+v", cached)
	}
}
func TestPMTriagePendingDeduplicates(t *testing.T) {
	repo, _ := setupPMTriageStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, triageAttempt(), 10)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := repo.Reserve(ctx, triageAttempt(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.CallProvider || repeated.Assessment.ID != first.Assessment.ID {
		t.Fatal("duplicate provider admission")
	}
}
func TestPMTriageFailedAttemptCountsAgainstCap(t *testing.T) {
	repo, _ := setupPMTriageStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, triageAttempt(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Finish(ctx, "workspace", "actor", first.Assessment.ID, "failed", nil); err != nil {
		t.Fatal(err)
	}
	next := triageAttempt()
	next.ContextHash = "new-context"
	limited, err := repo.Reserve(ctx, next, 1)
	if err != nil {
		t.Fatal(err)
	}
	if limited.CallProvider || !limited.Limited {
		t.Fatal("failed call bypassed cap")
	}
}
func TestPMTriageAbandonedAttemptExpires(t *testing.T) {
	repo, db := setupPMTriageStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, triageAttempt(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.PMTriageAssessment{}).Where("id = ?", first.Assessment.ID).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	next, err := repo.Reserve(ctx, triageAttempt(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if !next.CallProvider || next.Assessment.ID == first.Assessment.ID {
		t.Fatal("abandoned call permanently blocked retry")
	}
	if err := repo.Finish(ctx, "workspace", "actor", first.Assessment.ID, "ready", model.JSONB{}); err == nil {
		t.Fatal("late completion replaced expired attempt")
	}
}
func TestPMTriageAssessmentOwnership(t *testing.T) {
	repo, _ := setupPMTriageStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, triageAttempt(), 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, workspace, actor string }{{"other workspace", "other", "actor"}, {"other actor", "workspace", "other"}} {
		t.Run(tc.name, func(t *testing.T) {
			assessment, err := repo.Get(ctx, tc.workspace, tc.actor, first.Assessment.ID)
			if err != nil {
				t.Fatal(err)
			}
			if assessment != nil {
				t.Fatal("assessment exposed to wrong principal")
			}
			if err := repo.Finish(ctx, tc.workspace, tc.actor, first.Assessment.ID, "ready", nil); err == nil {
				t.Fatal("wrong principal completed assessment")
			}
		})
	}
}
func TestPMTriageCacheSeparatesModeAndActor(t *testing.T) {
	repo, _ := setupPMTriageStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, triageAttempt(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Finish(ctx, "workspace", "actor", first.Assessment.ID, "ready", model.JSONB{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*model.PMTriageAssessment)
	}{
		{"actor", func(a *model.PMTriageAssessment) { a.ActorID = "other" }},
		{"mode", func(a *model.PMTriageAssessment) { a.Mode = "shadow" }},
		{"context", func(a *model.PMTriageAssessment) { a.ContextHash = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := triageAttempt()
			tc.change(&input)
			got, err := repo.Reserve(ctx, input, 10)
			if err != nil {
				t.Fatal(err)
			}
			if !got.CallProvider {
				t.Fatal("reused incompatible assessment")
			}
		})
	}
}
