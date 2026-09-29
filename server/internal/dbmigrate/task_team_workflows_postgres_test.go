//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepairTaskTeamWorkflowsPostgres(t *testing.T) {
	dsn := os.Getenv("AI_PROFILES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AI_PROFILES_TEST_DATABASE_URL is required for an isolated PostgreSQL schema")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	schema := fmt.Sprintf("task_team_workflows_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	_, err = db.ExecContext(ctx, `
CREATE TABLE pm_workflows (id text PRIMARY KEY, workspace_id text, team_id text, created_at timestamptz DEFAULT now());
CREATE TABLE pm_workflow_states (id text PRIMARY KEY, workflow_id text, name text, state_type text, position integer);
CREATE TABLE pm_tasks (id text PRIMARY KEY, workspace_id text, team_id text, workflow_id text, workflow_state_id text,
    archived boolean DEFAULT false, completed boolean DEFAULT false, started boolean DEFAULT false,
    completed_at timestamptz, moved_at timestamptz, updated_at timestamptz DEFAULT '2026-09-25');
INSERT INTO pm_workflows (id, workspace_id, team_id) VALUES
    ('default', 'ws', NULL), ('engineering', 'ws', 'eng'), ('sales', 'ws', 'sales'), ('foreign', 'other', 'eng');
INSERT INTO pm_workflow_states VALUES
    ('old-todo', 'default', 'To Do', 'unstarted', 0),
    ('old-started', 'default', 'In Progress', 'started', 1),
    ('old-review', 'default', 'In Review', 'started', 2),
    ('old-done', 'default', 'Done', 'done', 3),
    ('old-backlog', 'default', 'Backlog', 'backlog', 4),
    ('eng-todo', 'engineering', 'To Do', 'unstarted', 0),
    ('eng-started', 'engineering', 'In Progress', 'started', 1),
    ('eng-review', 'engineering', 'In Review', 'started', 2),
    ('eng-done', 'engineering', 'Closed', 'done', 3),
    ('sales-todo', 'sales', 'To Do', 'unstarted', 0),
    ('foreign-todo', 'foreign', 'To Do', 'unstarted', 0);
INSERT INTO pm_tasks (id, workspace_id, team_id, workflow_id, workflow_state_id) VALUES
    ('todo', 'ws', 'eng', 'default', 'old-todo'),
    ('in-progress', 'ws', 'eng', 'default', 'old-started'),
    ('review', 'ws', 'eng', 'default', 'old-review'),
    ('done', 'ws', 'eng', 'default', 'old-done'),
    ('archived', 'ws', 'eng', 'default', 'old-done'),
    ('unmatched', 'ws', 'eng', 'default', 'old-backlog'),
    ('teamless', 'ws', NULL, 'default', 'old-todo'),
    ('no-workflow', 'ws', 'legacy', 'default', 'old-todo'),
    ('already-correct', 'ws', 'eng', 'engineering', 'eng-todo'),
    ('other-team-workflow', 'ws', 'eng', 'sales', 'sales-todo'),
    ('cross-workspace', 'other', 'eng', 'default', 'old-todo');
UPDATE pm_tasks SET started = true, moved_at = '2026-09-25' WHERE id IN ('in-progress', 'review', 'done', 'archived');
UPDATE pm_tasks SET completed = true, completed_at = '2026-09-26' WHERE id IN ('done', 'archived');
UPDATE pm_tasks SET archived = true WHERE id = 'archived';
CREATE TABLE before_repair AS SELECT * FROM pm_tasks;
CREATE TABLE pm_task_templates (id text PRIMARY KEY, workspace_id text, team_id text, workflow_state_id text,
    name text DEFAULT 'Saved task', updated_at timestamptz DEFAULT '2026-09-25');
INSERT INTO pm_task_templates (id, workspace_id, team_id, workflow_state_id) VALUES
    ('saved-todo', 'ws', 'eng', 'old-todo'), ('saved-review', 'ws', 'eng', 'old-review'),
    ('shared', 'ws', NULL, 'old-todo'), ('unmatched', 'ws', 'eng', 'old-backlog'),
    ('other-workflow', 'ws', 'eng', 'sales-todo'), ('cross-workspace', 'other', 'eng', 'old-todo');
CREATE TABLE pm_recurring_templates (id text PRIMARY KEY, workspace_id text, team_id text, seed_payload jsonb, config jsonb,
    next_run_at timestamptz DEFAULT '2026-10-01', generated_count integer DEFAULT 3,
    updated_at timestamptz DEFAULT '2026-09-25');
INSERT INTO pm_recurring_templates (id, workspace_id, team_id, seed_payload, config) VALUES
    ('recurring', 'ws', 'eng', '{"name":"Weekly report","workflow_id":"default","workflow_state_id":"old-review","priority":"high"}',
     '{"frequency":"weekly","completion_state_ids":["old-done","unknown-state"],"interval":2}'),
    ('trigger-only', 'ws', 'eng', '{"workflow_id":"engineering","workflow_state_id":"eng-todo"}',
     '{"completion_state_ids":["old-done"]}'),
    ('unmatched', 'ws', 'eng', '{"workflow_id":"default","workflow_state_id":"old-backlog"}', '{}'),
    ('teamless', 'ws', NULL, '{"workflow_id":"default","workflow_state_id":"old-todo"}', '{}'),
    ('cross-workspace', 'other', 'eng', '{"workflow_id":"default","workflow_state_id":"old-todo"}', '{}');
CREATE TABLE recurring_before_repair AS SELECT * FROM pm_recurring_templates;
`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("sql/202609290001_repair_task_team_workflows.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i, wantCount := range []int64{5, 0} {
		result, err := db.ExecContext(ctx, string(migration))
		if err != nil {
			t.Fatal(err)
		}
		count, err := result.RowsAffected()
		if err != nil || count != wantCount {
			t.Fatalf("pass %d: changed %d tasks, want %d; err=%v", i, count, wantCount, err)
		}
	}
	for _, tc := range []struct{ id, workflow, state string }{
		{"todo", "engineering", "eng-todo"}, {"in-progress", "engineering", "eng-started"},
		{"review", "engineering", "eng-review"}, {"done", "engineering", "eng-done"},
		{"archived", "engineering", "eng-done"}, {"unmatched", "default", "old-backlog"},
		{"teamless", "default", "old-todo"}, {"no-workflow", "default", "old-todo"},
		{"already-correct", "engineering", "eng-todo"}, {"other-team-workflow", "sales", "sales-todo"},
		{"cross-workspace", "default", "old-todo"},
	} {
		var workflow, state string
		if err := db.QueryRowContext(ctx, "SELECT workflow_id, workflow_state_id FROM pm_tasks WHERE id = $1", tc.id).Scan(&workflow, &state); err != nil {
			t.Fatal(err)
		}
		if workflow != tc.workflow || state != tc.state {
			t.Errorf("%s = %s/%s, want %s/%s", tc.id, workflow, state, tc.workflow, tc.state)
		}
	}
	var changed int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pm_tasks t JOIN before_repair b USING (id)
WHERE (t.workspace_id, t.team_id, t.archived, t.completed, t.started, t.completed_at, t.moved_at)
IS DISTINCT FROM (b.workspace_id, b.team_id, b.archived, b.completed, b.started, b.completed_at, b.moved_at)`).Scan(&changed); err != nil || changed != 0 {
		t.Fatalf("repair changed ownership or lifecycle: count=%d, err=%v", changed, err)
	}

	templateMigration, err := os.ReadFile("sql/202609290002_repair_template_team_workflows.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i, wantCount := range []int64{2, 0} {
		result, err := db.ExecContext(ctx, string(templateMigration))
		if err != nil {
			t.Fatal(err)
		}
		count, err := result.RowsAffected()
		if err != nil || count != wantCount {
			t.Fatalf("template pass %d: changed %d recurrences, want %d; err=%v", i, count, wantCount, err)
		}
	}
	for _, tc := range []struct{ id, state string }{
		{"saved-todo", "eng-todo"}, {"saved-review", "eng-review"}, {"shared", "old-todo"},
		{"unmatched", "old-backlog"}, {"other-workflow", "sales-todo"}, {"cross-workspace", "old-todo"},
	} {
		var state string
		if err := db.QueryRowContext(ctx, "SELECT workflow_state_id FROM pm_task_templates WHERE id = $1", tc.id).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != tc.state {
			t.Errorf("saved template %s: state=%s, want %s", tc.id, state, tc.state)
		}
	}
	var correct bool
	if err := db.QueryRowContext(ctx, `SELECT seed_payload->>'workflow_id' = 'engineering'
    AND seed_payload->>'workflow_state_id' = 'eng-review'
    AND config->'completion_state_ids' = '["eng-done","unknown-state"]'::jsonb
    FROM pm_recurring_templates WHERE id = 'recurring'`).Scan(&correct); err != nil || !correct {
		t.Fatalf("recurring seed/trigger not repaired: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT config->'completion_state_ids' = '["eng-done"]'::jsonb
    FROM pm_recurring_templates WHERE id = 'trigger-only'`).Scan(&correct); err != nil || !correct {
		t.Fatalf("completion trigger was not independently repaired: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pm_recurring_templates t JOIN recurring_before_repair b USING (id)
WHERE (t.workspace_id, t.team_id, t.next_run_at, t.generated_count,
       t.seed_payload - 'workflow_id' - 'workflow_state_id', t.config - 'completion_state_ids')
IS DISTINCT FROM (b.workspace_id, b.team_id, b.next_run_at, b.generated_count,
       b.seed_payload - 'workflow_id' - 'workflow_state_id', b.config - 'completion_state_ids')
OR (t.id NOT IN ('recurring','trigger-only') AND (t.seed_payload, t.config, t.updated_at)
    IS DISTINCT FROM (b.seed_payload, b.config, b.updated_at))`).Scan(&changed); err != nil || changed != 0 {
		t.Fatalf("template repair changed unrelated data: count=%d, err=%v", changed, err)
	}
}
