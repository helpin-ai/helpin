package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestReleaseFactsServiceFindTasksForGitChangesMatchesDeterministically(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC()

	mustExec(t, db, `CREATE TABLE IF NOT EXISTS task_git_links (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		integration_id TEXT NOT NULL,
		repository_id TEXT,
		run_id TEXT,
		provider TEXT NOT NULL,
		base_url TEXT,
		repo TEXT NOT NULL,
		branch TEXT,
		pr_number INTEGER,
		pr_title TEXT,
		pr_url TEXT,
		pr_status TEXT,
		commit_sha TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `INSERT INTO workspaces (id, name, slug, workspace_key, owner_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"ws-1", "Workspace", "workspace", "HLP", "owner-1", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"state-1", "wf-1", "In Progress", model.PMStateTypeStarted, 0, now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, priority, position, started, completed, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "ws-1", 123, "Improve releases", model.PMTaskTypeFeature, "wf-1", "state-1", model.PMTaskPriorityHigh, 0, false, false, now, now)
	mustExec(t, db, `INSERT INTO task_git_links (id, workspace_id, task_id, integration_id, provider, repo, branch, pr_number, commit_sha, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"link-1", "ws-1", "task-1", "integration-1", "github", "acme/api", "helpin/hlp-123", 42, "abc123", now, now)

	svc := NewReleaseFactsService(
		nil,
		nil,
		repository.NewPMTaskRepository(db),
		repository.NewTaskGitLinkRepository(db),
		nil,
		nil,
		nil,
		nil,
		repository.NewWorkspaceRepository(db),
		nil,
	)

	result, err := svc.FindTasksForGitChanges(context.Background(), "ws-1", model.FindTasksForGitChangesRequest{
		RepoFullName: "acme/api",
		PRNumbers:    []int{42},
		CommitSHAs:   []string{"abc123"},
		Branches:     []string{"helpin/hlp-123"},
		Texts:        []string{"Implements HLP-123 release improvements"},
	})
	if err != nil {
		t.Fatalf("FindTasksForGitChanges returned error: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected one match, got %#v", result.Matches)
	}
	match := result.Matches[0]
	if match.TaskID != "task-1" || match.TaskKey != "HLP-123" || match.Confidence != "high" {
		t.Fatalf("unexpected match %#v", match)
	}
	for _, evidence := range []string{"pull_request_number", "commit_sha", "branch", "task_key"} {
		if !slices.Contains(match.MatchedBy, evidence) {
			t.Fatalf("expected evidence %q in %#v", evidence, match.MatchedBy)
		}
	}
	if len(result.Unmatched["pr_numbers"]) != 0 || len(result.Unmatched["commit_shas"]) != 0 || len(result.Unmatched["branches"]) != 0 || len(result.Unmatched["texts"]) != 0 {
		t.Fatalf("expected no unmatched evidence, got %#v", result.Unmatched)
	}
}

func TestReleaseFactsServiceGetTaskContextIncludesDocsAndGitLinks(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC()

	mustExec(t, db, `CREATE TABLE IF NOT EXISTS docs_documents (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		collection_id TEXT,
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		visibility TEXT NOT NULL,
		owner_id TEXT,
		team_id TEXT,
		template_key TEXT,
		excerpt TEXT,
		icon TEXT,
		tags TEXT,
		position INTEGER NOT NULL DEFAULT 0,
		is_pinned BOOLEAN NOT NULL DEFAULT 0,
		is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
		share_token TEXT,
		is_locked BOOLEAN NOT NULL DEFAULT 0,
		locked_by TEXT,
		last_reviewed_at DATETIME,
		next_review_at DATETIME,
		published_at DATETIME,
		created_by TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS docs_contents (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		content TEXT,
		content_text TEXT,
		word_count INTEGER NOT NULL DEFAULT 0,
		import_source_html TEXT,
		import_source_system TEXT,
		import_source_object_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS docs_links (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		document_id TEXT NOT NULL,
		linked_object_type TEXT NOT NULL,
		linked_object_id TEXT NOT NULL,
		link_context TEXT NOT NULL,
		created_by TEXT NOT NULL,
		created_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS task_git_links (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		integration_id TEXT NOT NULL,
		repository_id TEXT,
		run_id TEXT,
		provider TEXT NOT NULL,
		base_url TEXT,
		repo TEXT NOT NULL,
		branch TEXT,
		pr_number INTEGER,
		pr_title TEXT,
		pr_url TEXT,
		pr_status TEXT,
		commit_sha TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `INSERT INTO workspaces (id, name, slug, workspace_key, owner_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"ws-1", "Workspace", "workspace", "HLP", "owner-1", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"state-1", "wf-1", "Done", model.PMStateTypeDone, 0, now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, description, task_type, workflow_id, workflow_state_id, priority, position, started, completed, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "ws-1", 321, "Ship notes", "release note body", model.PMTaskTypeFeature, "wf-1", "state-1", model.PMTaskPriorityMedium, 0, true, true, now, now)
	mustExec(t, db, `INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, position, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"doc-1", "ws-1", "space-1", "Release Notes", model.DocStatusPublished, model.SpaceVisibilityWorkspaceWide, 0, "user-1", now, now)
	contentJSON, _ := json.Marshal(map[string]any{"type": "doc", "content": []any{}})
	mustExec(t, db, `INSERT INTO docs_contents (id, document_id, content, content_text, word_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"content-1", "doc-1", []byte(contentJSON), "Release notes body", 3, now, now)
	mustExec(t, db, `INSERT INTO docs_links (id, workspace_id, document_id, linked_object_type, linked_object_id, link_context, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"doc-link-1", "ws-1", "doc-1", "task", "task-1", "release_context", "user-1", now)
	mustExec(t, db, `INSERT INTO task_git_links (id, workspace_id, task_id, integration_id, provider, repo, branch, pr_number, pr_title, commit_sha, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"git-link-1", "ws-1", "task-1", "integration-1", "github", "acme/api", "helpin/hlp-321", 99, "Ship notes", "def456", now, now)

	svc := NewReleaseFactsService(
		nil,
		nil,
		repository.NewPMTaskRepository(db),
		repository.NewTaskGitLinkRepository(db),
		repository.NewPMCommentRepository(db),
		repository.NewDocsLinkRepository(db),
		repository.NewDocsDocumentRepository(db),
		repository.NewDocsContentRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
	)

	result, err := svc.GetTaskContext(context.Background(), "ws-1", model.GetTaskContextRequest{
		TaskIDs:                []string{"task-1"},
		IncludeLinkedDocs:      true,
		IncludeDocumentContent: true,
		IncludeGitLinks:        true,
	})
	if err != nil {
		t.Fatalf("GetTaskContext returned error: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("expected one task, got %#v", result.Tasks)
	}
	task := result.Tasks[0]
	if task.TaskKey != "HLP-321" || task.Status != "Done" || !strings.Contains(task.DescriptionText, "release note body") {
		t.Fatalf("unexpected task context %#v", task)
	}
	if len(task.LinkedDocs) != 1 || task.LinkedDocs[0].DocumentID != "doc-1" || task.LinkedDocs[0].ContentText != "Release notes body" {
		t.Fatalf("unexpected linked docs %#v", task.LinkedDocs)
	}
	if len(task.GitLinks) != 1 || task.GitLinks[0].Repo != "acme/api" || task.GitLinks[0].CommitSHA == nil || *task.GitLinks[0].CommitSHA != "def456" {
		t.Fatalf("unexpected git links %#v", task.GitLinks)
	}
}
