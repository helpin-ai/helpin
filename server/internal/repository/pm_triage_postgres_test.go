//go:build integration

package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type pmTriagePostgresEnv struct {
	db                                  *gorm.DB
	workspace, actor, task, label, team string
	revision                            time.Time
}

func setupPMTriagePostgres(t *testing.T) pmTriagePostgresEnv {
	t.Helper()
	dsn := os.Getenv("PM_TRIAGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PM_TRIAGE_TEST_DATABASE_URL is required for isolated PostgreSQL tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "pm_triage_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err != nil {
			t.Error(err)
		} else if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		sqlAdmin, err := admin.DB()
		if err != nil {
			t.Error(err)
		} else if err := sqlAdmin.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, statement := range []string{
		`CREATE TABLE workspaces (id uuid PRIMARY KEY)`,
		`CREATE TABLE users (id uuid PRIMARY KEY)`,
		`CREATE TABLE pm_tasks (id uuid PRIMARY KEY, workspace_id uuid, team_id uuid, name text, description text, display_id integer, archived boolean NOT NULL DEFAULT false, updated_at timestamptz NOT NULL)`,
		`CREATE TABLE pm_labels (id uuid PRIMARY KEY, workspace_id uuid, team_id uuid, name text, description text, archived boolean NOT NULL DEFAULT false)`,
		`CREATE TABLE pm_task_labels (task_id uuid NOT NULL,label_id uuid NOT NULL,created_at timestamptz NOT NULL,PRIMARY KEY(task_id,label_id))`,
		`CREATE TABLE pm_activity_log (id uuid PRIMARY KEY,workspace_id uuid,entity_type text,entity_id uuid,actor_id uuid,event_type text,action text,field_name text,old_value text,new_value text,metadata jsonb,created_at timestamptz)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../dbmigrate/sql/202609170002_pm_triage.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Exec(string(migration)).Error; err != nil {
			t.Fatal(err)
		}
	}
	env := pmTriagePostgresEnv{db: db, workspace: uuid.NewString(), actor: uuid.NewString(), task: uuid.NewString(), label: uuid.NewString(), team: uuid.NewString(), revision: time.Now().UTC().Truncate(time.Microsecond)}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO workspaces VALUES (?)`, []any{env.workspace}},
		{`INSERT INTO users VALUES (?)`, []any{env.actor}},
		{`INSERT INTO pm_tasks(id,workspace_id,team_id,name,description,updated_at) VALUES (?,?,?,?,?,?)`, []any{env.task, env.workspace, env.team, "Export fails", "Invoice CSV fails", env.revision}},
		{`INSERT INTO pm_labels(id,workspace_id,team_id,name) VALUES (?,?,?,?)`, []any{env.label, env.workspace, env.team, "Exports"}},
	} {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	return env
}
func (env pmTriagePostgresEnv) attempt(hash string) model.PMTriageAssessment {
	return model.PMTriageAssessment{WorkspaceID: env.workspace, ActorID: env.actor, SourceKind: "task", SourceID: env.task, SourceHash: "source", ContextHash: hash, Mode: "primary"}
}

func TestPMTriagePostgresAdmissionCap(t *testing.T) {
	env := setupPMTriagePostgres(t)
	repo := NewPMTriageRepository(env.db)
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	var failures []error
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, err := repo.Reserve(context.Background(), env.attempt(fmt.Sprint(index)), 3)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, err)
			} else if result.CallProvider {
				admitted++
			}
		}(i)
	}
	wg.Wait()
	if len(failures) > 0 {
		t.Fatal(failures)
	}
	if admitted != 3 {
		t.Fatalf("concurrent calls admitted %d, cap 3", admitted)
	}
}
func TestPMTriagePostgresDuplicateAdmission(t *testing.T) {
	env := setupPMTriagePostgres(t)
	repo := NewPMTriageRepository(env.db)
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := repo.Reserve(context.Background(), env.attempt("same"), 10)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Error(err)
			} else if result.CallProvider {
				admitted++
			}
		}()
	}
	wg.Wait()
	if admitted != 1 {
		t.Fatalf("same context admitted %d times", admitted)
	}
}
func TestPMTriagePostgresLabelSuppressionAndRevision(t *testing.T) {
	env := setupPMTriagePostgres(t)
	ctx := context.Background()
	repo := NewPMTriageRepository(env.db)
	admission, err := repo.Reserve(ctx, env.attempt("labels"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Finish(ctx, env.workspace, env.actor, admission.Assessment.ID, "ready", model.JSONB{}); err != nil {
		t.Fatal(err)
	}
	tasks := NewPMTaskRepository(env.db)
	scope := PMTriageScope{WorkspaceID: env.workspace, TeamIDs: []string{env.team}}
	apply := func(revision time.Time) []string {
		t.Helper()
		added, err := tasks.ApplyTriageLabels(ctx, scope, env.task, env.actor, admission.Assessment.ID, revision, []string{env.label})
		if err != nil {
			t.Fatal(err)
		}
		return added
	}
	if len(apply(env.revision.Add(-time.Second))) != 0 {
		t.Fatal("stale revision applied labels")
	}
	if len(apply(env.revision)) != 1 {
		t.Fatal("current revision did not apply label")
	}
	if err := tasks.RemoveLabelWithTriageSuppression(ctx, env.workspace, env.task, env.label); err != nil {
		t.Fatal(err)
	}
	if len(apply(env.revision)) != 0 {
		t.Fatal("manual removal was undone")
	}
	var events int64
	if err := env.db.Model(&model.PMActivityLog{}).Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("unexpected automatic activity count %d", events)
	}
}

func TestPMTriagePostgresCandidateRanking(t *testing.T) {
	env := setupPMTriagePostgres(t)
	specific := uuid.NewString()
	for i := 0; i < 20; i++ {
		if err := env.db.Exec("INSERT INTO pm_tasks(id,workspace_id,team_id,name,description,updated_at) VALUES(?,?,?,?,?,?)", uuid.NewString(), env.workspace, env.team, "Page error", "", env.revision).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := env.db.Exec("INSERT INTO pm_tasks(id,workspace_id,team_id,name,description,updated_at) VALUES(?,?,?,?,?,?)", specific, env.workspace, env.team, "OAuth callback", "", env.revision.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	candidates, err := NewPMTaskRepository(env.db).FindTriageCandidates(context.Background(), PMTriageScope{WorkspaceID: env.workspace, TeamIDs: []string{env.team}}, env.task, "Page error OAuth callback")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 21 || candidates[0].ID != specific {
		t.Fatalf("unexpected PostgreSQL ranking: %+v", candidates)
	}
}
