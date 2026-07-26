package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	worker "github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeSkillPackageStore struct {
	objects map[string][]byte
	deleted []string
}

func (f *fakeSkillPackageStore) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader, _ bool) error {
	if f.objects == nil {
		f.objects = make(map[string][]byte)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.objects[key] = payload
	return nil
}

func (f *fakeSkillPackageStore) GetObject(_ context.Context, key string) ([]byte, error) {
	payload, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	return append([]byte(nil), payload...), nil
}

func (f *fakeSkillPackageStore) DeleteObject(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	delete(f.objects, key)
	return nil
}

func TestCreateWorkspaceSkillStoresArchiveAndMetadata(t *testing.T) {
	svc, repo, store := newWorkspaceSkillTestService(t)
	ctx := context.Background()

	req := model.CreateWorkspaceSkillRequest{
		Key:               "Repo Planner",
		Title:             stringPtr("Repository Planner"),
		Description:       "Use when planning implementation work inside a repository.",
		Instructions:      "Follow the planning workflow and stop at approval checkpoints.",
		RequiredTools:     []string{"publish_task_plan", "request_approval", "publish_task_plan"},
		SupportedRuntimes: []string{"native_sdk", "codex", "native_sdk"},
		SourceRuntime:     stringPtr("codex"),
	}

	resp, err := svc.CreateWorkspaceSkill(ctx, "ws-1", "user-1", req)
	if err != nil {
		t.Fatalf("CreateWorkspaceSkill returned error: %v", err)
	}
	if resp.SourceKind != model.WorkspaceSkillSourceWorkspace {
		t.Fatalf("expected workspace source kind, got %q", resp.SourceKind)
	}
	if resp.SourceRuntime == nil || *resp.SourceRuntime != "codex" {
		t.Fatalf("expected source runtime codex, got %+v", resp.SourceRuntime)
	}
	if resp.Key != "repo_planner" {
		t.Fatalf("expected normalized key repo_planner, got %q", resp.Key)
	}
	if len(store.objects) != 1 {
		t.Fatalf("expected one stored archive, got %d", len(store.objects))
	}

	stored := mustOnlyStoredArchive(t, store)
	loaded, err := worker.LoadSkillArchive(stored, model.WorkspaceSkillSourceWorkspace)
	if err != nil {
		t.Fatalf("LoadSkillArchive returned error: %v", err)
	}
	if loaded.Key != resp.Key {
		t.Fatalf("expected stored skill key %q, got %q", resp.Key, loaded.Key)
	}
	if loaded.Title != "Repository Planner" {
		t.Fatalf("expected stored title %q, got %q", "Repository Planner", loaded.Title)
	}
	if !containsString(loaded.RequiredTools, "publish_task_plan") || !containsString(loaded.RequiredTools, "request_approval") {
		t.Fatalf("expected required tools in stored archive, got %v", loaded.RequiredTools)
	}

	row, err := repo.GetActiveByKey(ctx, "ws-1", resp.Key)
	if err != nil {
		t.Fatalf("GetActiveByKey returned error: %v", err)
	}
	if row == nil {
		t.Fatal("expected persisted workspace skill")
	}
	if row.PackageObjectKey == "" || row.PackageFileName == "" || row.PackageChecksum == "" {
		t.Fatalf("expected package metadata to be populated, got %+v", row)
	}
	if got := parseJSONStringSlice(json.RawMessage(row.RequiredTools)); len(got) != 2 || got[0] != "publish_task_plan" || got[1] != "request_approval" {
		t.Fatalf("expected sorted unique required tools, got %v", got)
	}
}

func TestImportWorkspaceSkillPreservesArchiveAndAppearsInCatalog(t *testing.T) {
	svc, _, store := newWorkspaceSkillTestService(t)
	ctx := context.Background()

	archive, _, _, err := worker.BuildSkillArchive(worker.SkillDefinition{
		Key:               "external_review",
		Title:             "External Review",
		Description:       "Use when reviewing third-party contribution changes.",
		Instructions:      "Review the change set and provide action-oriented findings.",
		RequiredTools:     []string{"request_changes"},
		SupportedRuntimes: []string{"codex", "opencode"},
	})
	if err != nil {
		t.Fatalf("BuildSkillArchive returned error: %v", err)
	}

	resp, err := svc.ImportWorkspaceSkill(ctx, "ws-1", "user-1", "external_review.zip", bytes.NewReader(archive), int64(len(archive)), stringPtr("codex"))
	if err != nil {
		t.Fatalf("ImportWorkspaceSkill returned error: %v", err)
	}
	if resp.SourceKind != model.WorkspaceSkillSourceImported {
		t.Fatalf("expected imported source kind, got %q", resp.SourceKind)
	}

	stored := mustOnlyStoredArchive(t, store)
	if !bytes.Equal(stored, archive) {
		t.Fatal("expected imported archive bytes to be preserved")
	}

	catalog, err := svc.ListSkillCatalog(ctx, "ws-1")
	if err != nil {
		t.Fatalf("ListSkillCatalog returned error: %v", err)
	}
	if !catalogHasSkill(catalog, "prd_task_plan_approval", "built_in") {
		t.Fatalf("expected built-in skill in merged catalog, got %+v", catalog.Skills)
	}
	if !catalogHasSkill(catalog, "external_review", model.WorkspaceSkillSourceImported) {
		t.Fatalf("expected imported workspace skill in merged catalog, got %+v", catalog.Skills)
	}
}

func TestUpdateWorkspaceSkillRejectsImportedSkill(t *testing.T) {
	svc, repo, _ := newWorkspaceSkillTestService(t)
	ctx := context.Background()

	archive, _, _, err := worker.BuildSkillArchive(worker.SkillDefinition{
		Key:          "external_review",
		Title:        "External Review",
		Description:  "Use when reviewing third-party contribution changes.",
		Instructions: "Review the change set and provide action-oriented findings.",
	})
	if err != nil {
		t.Fatalf("BuildSkillArchive returned error: %v", err)
	}
	resp, err := svc.ImportWorkspaceSkill(ctx, "ws-1", "user-1", "external_review.zip", bytes.NewReader(archive), int64(len(archive)), nil)
	if err != nil {
		t.Fatalf("ImportWorkspaceSkill returned error: %v", err)
	}

	_, err = svc.UpdateWorkspaceSkill(ctx, "ws-1", resp.ID, model.UpdateWorkspaceSkillRequest{Instructions: stringPtr("Updated")})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}

	row, err := repo.GetByID(ctx, "ws-1", resp.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if row == nil || row.SourceKind != model.WorkspaceSkillSourceImported {
		t.Fatalf("expected imported row to remain intact, got %+v", row)
	}
}

func TestUpdateWorkspaceSkillRebuildsArchive(t *testing.T) {
	svc, repo, store := newWorkspaceSkillTestService(t)
	ctx := context.Background()

	created, err := svc.CreateWorkspaceSkill(ctx, "ws-1", "user-1", model.CreateWorkspaceSkillRequest{
		Key:          "repo_planner",
		Description:  "Use when planning repository work.",
		Instructions: "Produce a first-pass plan.",
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSkill returned error: %v", err)
	}

	updated, err := svc.UpdateWorkspaceSkill(ctx, "ws-1", created.ID, model.UpdateWorkspaceSkillRequest{
		Key:               stringPtr("repo_planner_v2"),
		Title:             stringPtr("Repository Planner v2"),
		Description:       stringPtr("Use when planning repository work with updated guidance."),
		Instructions:      stringPtr("Produce an updated plan and call out risks."),
		SupportedRuntimes: []string{"codex", "opencode"},
	})
	if err != nil {
		t.Fatalf("UpdateWorkspaceSkill returned error: %v", err)
	}
	if updated.Key != "repo_planner_v2" {
		t.Fatalf("expected updated key repo_planner_v2, got %q", updated.Key)
	}

	stored := mustOnlyStoredArchive(t, store)
	loaded, err := worker.LoadSkillArchive(stored, model.WorkspaceSkillSourceWorkspace)
	if err != nil {
		t.Fatalf("LoadSkillArchive returned error: %v", err)
	}
	if loaded.Key != "repo_planner_v2" {
		t.Fatalf("expected rebuilt archive key repo_planner_v2, got %q", loaded.Key)
	}
	if loaded.Instructions != "Produce an updated plan and call out risks." {
		t.Fatalf("expected updated instructions, got %q", loaded.Instructions)
	}
	if !containsString(loaded.SupportedRuntimes, "codex") || !containsString(loaded.SupportedRuntimes, "opencode") {
		t.Fatalf("expected updated runtimes, got %v", loaded.SupportedRuntimes)
	}

	row, err := repo.GetByID(ctx, "ws-1", created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if row == nil {
		t.Fatal("expected updated row")
	}
	if row.Key != "repo_planner_v2" {
		t.Fatalf("expected persisted key repo_planner_v2, got %q", row.Key)
	}
	if row.VersionKey == created.VersionKey {
		t.Fatal("expected version key to change after update")
	}
}

func TestDeleteWorkspaceSkillArchivesRecord(t *testing.T) {
	svc, repo, _ := newWorkspaceSkillTestService(t)
	ctx := context.Background()

	created, err := svc.CreateWorkspaceSkill(ctx, "ws-1", "user-1", model.CreateWorkspaceSkillRequest{
		Key:          "repo_planner",
		Description:  "Use when planning repository work.",
		Instructions: "Produce a first-pass plan.",
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSkill returned error: %v", err)
	}

	if err := svc.DeleteWorkspaceSkill(ctx, "ws-1", created.ID); err != nil {
		t.Fatalf("DeleteWorkspaceSkill returned error: %v", err)
	}

	row, err := repo.GetByID(ctx, "ws-1", created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if row == nil || !row.IsArchived {
		t.Fatalf("expected archived row, got %+v", row)
	}

	catalog, err := svc.ListSkillCatalog(ctx, "ws-1")
	if err != nil {
		t.Fatalf("ListSkillCatalog returned error: %v", err)
	}
	if catalogHasSkill(catalog, created.Key, model.WorkspaceSkillSourceWorkspace) {
		t.Fatalf("expected archived skill to be absent from catalog, got %+v", catalog.Skills)
	}
}

func TestCreateWorkspaceSkillCleansUpArchiveOnDatabaseFailure(t *testing.T) {
	db := newWorkspaceSkillTestDB(t)
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail_workspace_skill_create", func(tx *gorm.DB) {
		tx.AddError(errors.New("forced create failure"))
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	repo := repository.NewWorkspaceSkillRepository(db)
	store := &fakeSkillPackageStore{objects: make(map[string][]byte)}
	svc := (&AgentService{}).SetWorkspaceSkillStore(repo, store)
	ctx := context.Background()

	_, err := svc.CreateWorkspaceSkill(ctx, "ws-1", "user-1", model.CreateWorkspaceSkillRequest{
		Key:          "repo_planner",
		Description:  "Use when planning repository work.",
		Instructions: "Produce a first-pass plan.",
	})
	if err == nil || !strings.Contains(err.Error(), "forced create failure") {
		t.Fatal("expected CreateWorkspaceSkill to fail after database close")
	}
	if len(store.objects) != 0 {
		t.Fatalf("expected uploaded archive to be cleaned up, got %d objects", len(store.objects))
	}
	if len(store.deleted) != 1 {
		t.Fatalf("expected one cleanup delete call, got %d", len(store.deleted))
	}
}

func newWorkspaceSkillTestService(t *testing.T) (*AgentService, *repository.WorkspaceSkillRepository, *fakeSkillPackageStore) {
	t.Helper()

	db := newWorkspaceSkillTestDB(t)
	repo := repository.NewWorkspaceSkillRepository(db)
	store := &fakeSkillPackageStore{objects: make(map[string][]byte)}
	svc := (&AgentService{}).SetWorkspaceSkillStore(repo, store)
	return svc, repo, store
}

func newWorkspaceSkillTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:workspace-skills-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statement := `CREATE TABLE workspace_skills (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		source_kind TEXT NOT NULL,
		source_runtime TEXT,
		key TEXT NOT NULL,
		version_key TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		instructions TEXT NOT NULL,
		required_tools BLOB NOT NULL DEFAULT '[]',
		supported_runtimes BLOB NOT NULL DEFAULT '[]',
		interface_config BLOB NOT NULL DEFAULT '{}',
		policy_config BLOB NOT NULL DEFAULT '{}',
		package_object_key TEXT NOT NULL,
		package_file_name TEXT NOT NULL,
		package_size INTEGER NOT NULL DEFAULT 0,
		package_checksum TEXT NOT NULL,
		is_archived BOOLEAN NOT NULL DEFAULT 0,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		UNIQUE(workspace_id, key, is_archived)
	)`
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create workspace_skills table: %v", err)
	}
	return db
}

func mustOnlyStoredArchive(t *testing.T, store *fakeSkillPackageStore) []byte {
	t.Helper()
	if len(store.objects) != 1 {
		t.Fatalf("expected exactly one stored object, got %d", len(store.objects))
	}
	for _, payload := range store.objects {
		return payload
	}
	t.Fatal("expected stored payload")
	return nil
}

func catalogHasSkill(catalog model.SkillCatalogResponse, key, sourceKind string) bool {
	for _, skill := range catalog.Skills {
		if skill.Key == key && skill.SourceKind == sourceKind {
			return true
		}
	}
	return false
}
