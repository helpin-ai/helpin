package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDocsImportCancelTestService(t *testing.T) (*DocsImportService, *repository.DocsImportRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE docs_import_jobs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		space_id TEXT,
		source TEXT NOT NULL DEFAULT 'helpscout',
		status TEXT NOT NULL DEFAULT 'pending',
		total INTEGER NOT NULL DEFAULT 0,
		completed INTEGER NOT NULL DEFAULT 0,
		failed INTEGER NOT NULL DEFAULT 0,
		failures TEXT NOT NULL DEFAULT '[]',
		config TEXT NOT NULL DEFAULT '{}',
		redirect_map TEXT,
		summary TEXT,
		error TEXT,
		payload_encrypted TEXT,
		workflow_id TEXT,
		started_by TEXT NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("migrate docs import job: %v", err)
	}
	repo := repository.NewDocsImportRepository(db)
	return &DocsImportService{importRepo: repo}, repo
}

func TestDocsImportCancelMarksActiveJobInterrupted(t *testing.T) {
	svc, repo := newDocsImportCancelTestService(t)
	job := &model.DocsImportJob{
		ID:          "import-1",
		WorkspaceID: "workspace-1",
		Status:      model.DocsImportStatusRunning,
		Failures:    []byte("[]"),
		Config:      []byte("{}"),
		StartedBy:   "user-1",
	}
	if err := repo.Create(context.Background(), job); err != nil {
		t.Fatalf("create import job: %v", err)
	}

	if err := svc.Cancel(context.Background(), job.ID, job.WorkspaceID); err != nil {
		t.Fatalf("cancel import: %v", err)
	}
	got, err := repo.GetByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get import job: %v", err)
	}
	if got.Status != model.DocsImportStatusInterrupted {
		t.Fatalf("status = %q, want %q", got.Status, model.DocsImportStatusInterrupted)
	}
	if got.CompletedAt == nil {
		t.Fatal("completed_at was not set")
	}
}

func TestDocsImportCancelRejectsAnotherWorkspace(t *testing.T) {
	svc, repo := newDocsImportCancelTestService(t)
	job := &model.DocsImportJob{
		ID:          "import-2",
		WorkspaceID: "workspace-1",
		Status:      model.DocsImportStatusRunning,
		Failures:    []byte("[]"),
		Config:      []byte("{}"),
		StartedBy:   "user-1",
	}
	if err := repo.Create(context.Background(), job); err != nil {
		t.Fatalf("create import job: %v", err)
	}

	if err := svc.Cancel(context.Background(), job.ID, "workspace-2"); err == nil {
		t.Fatal("cancel import succeeded for another workspace")
	}
	got, err := repo.GetByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get import job: %v", err)
	}
	if got.Status != model.DocsImportStatusRunning {
		t.Fatalf("status = %q, want %q", got.Status, model.DocsImportStatusRunning)
	}
}
