package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeBrowserAssetStore struct {
	objects map[string][]byte
	deleted []string
}

func (s *fakeBrowserAssetStore) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader, publicRead bool) error {
	if publicRead {
		return errors.New("browser screenshots must be private")
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if s.objects == nil {
		s.objects = map[string][]byte{}
	}
	s.objects[key] = payload
	return nil
}

func (s *fakeBrowserAssetStore) DeleteObject(_ context.Context, key string) error {
	delete(s.objects, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *fakeBrowserAssetStore) GeneratePresignedInlineGetURL(key string) (string, error) {
	return "https://private-assets.example.com/" + key + "?signed=true", nil
}

func TestAgentRuntimeHostUploadBrowserAssetPersistsMappedRunArtifact(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:agent-runtime-browser-asset?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	createAgentRuntimeBrowserAssetTables(t, db)
	externalRuntime := agentRuntimeName
	externalRuntimeID := "runtime-run-1"
	run := &model.AgentRun{
		ID: "11111111-1111-1111-1111-111111111111", WorkspaceID: "22222222-2222-2222-2222-222222222222",
		AgentID: "33333333-3333-3333-3333-333333333333", TargetType: "workspace", TargetID: "22222222-2222-2222-2222-222222222222",
		RuntimeKind: "native_sdk", Status: model.AgentRunStatusRunning, ExternalRuntime: &externalRuntime, ExternalRuntimeID: &externalRuntimeID,
	}
	if err := db.Exec(`INSERT INTO agent_runs (id, workspace_id, external_runtime, external_runtime_id) VALUES (?, ?, ?, ?)`, run.ID, run.WorkspaceID, externalRuntime, externalRuntimeID).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	store := &fakeBrowserAssetStore{}
	host := NewAgentRuntimeHostService("helpin", repository.NewAgentRunRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetBrowserAssetStore(repository.NewAgentRunArtifactRepository(db), store)
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	asset, err := host.UploadBrowserAsset(context.Background(), AgentRuntimeBrowserAssetUpload{
		AppID: "helpin", RuntimeRunID: externalRuntimeID, ArtifactType: model.AgentRunArtifactTypeBrowserScreenshot,
		Metadata: json.RawMessage(`{"annotated":true,"full_page":false}`),
		FileName: "Settings.png", ContentType: "image/png", Size: int64(len(png)), Body: bytes.NewReader(png),
	})
	if err != nil {
		t.Fatalf("UploadBrowserAsset: %v", err)
	}
	if asset.ArtifactID == "" || asset.ArtifactRef != "helpin-artifact://"+asset.ArtifactID || asset.Visibility != "private" {
		t.Fatalf("unexpected asset: %#v", asset)
	}
	artifacts, err := repository.NewAgentRunArtifactRepository(db).ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("artifacts=%#v err=%v", artifacts, err)
	}
	if artifacts[0].ArtifactType != "browser_screenshot" || artifacts[0].StorageMode != "object" || artifacts[0].ObjectKey == nil {
		t.Fatalf("unexpected artifact: %#v", artifacts[0])
	}
	if strings.Contains(string(artifacts[0].Metadata), "https://") || !strings.Contains(string(artifacts[0].Metadata), `"visibility":"private"`) {
		t.Fatalf("unexpected private artifact metadata: %s", artifacts[0].Metadata)
	}
	content, err := host.BrowserArtifactContentURL(context.Background(), run.WorkspaceID, asset.ArtifactID)
	if err != nil || !strings.Contains(content.URL, "signed=true") {
		t.Fatalf("private content URL=%#v err=%v", content, err)
	}
}

func TestAgentRuntimeHostUploadBrowserAssetRejectsUnsupportedArtifactType(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:agent-runtime-browser-asset-mismatch?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	createAgentRuntimeBrowserAssetTables(t, db)
	externalRuntime := agentRuntimeName
	externalRuntimeID := "runtime-run-mismatch"
	run := &model.AgentRun{
		ID: "44444444-4444-4444-4444-444444444444", WorkspaceID: "55555555-5555-5555-5555-555555555555",
		AgentID: "66666666-6666-6666-6666-666666666666", TargetType: "workspace", TargetID: "55555555-5555-5555-5555-555555555555",
		RuntimeKind: "native_sdk", Status: model.AgentRunStatusRunning, ExternalRuntime: &externalRuntime, ExternalRuntimeID: &externalRuntimeID,
	}
	if err := db.Exec(`INSERT INTO agent_runs (id, workspace_id, external_runtime, external_runtime_id) VALUES (?, ?, ?, ?)`, run.ID, run.WorkspaceID, externalRuntime, externalRuntimeID).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	store := &fakeBrowserAssetStore{}
	host := NewAgentRuntimeHostService("helpin", repository.NewAgentRunRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetBrowserAssetStore(repository.NewAgentRunArtifactRepository(db), store)
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	_, err = host.UploadBrowserAsset(context.Background(), AgentRuntimeBrowserAssetUpload{
		AppID: "helpin", RuntimeRunID: externalRuntimeID, ArtifactType: "arbitrary_file",
		FileName: "shot.png", ContentType: "image/png", Size: int64(len(png)), Body: bytes.NewReader(png),
	})
	if err == nil || !strings.Contains(err.Error(), "artifact_type") {
		t.Fatalf("expected artifact type rejection, got %v", err)
	}
	if len(store.objects) != 0 {
		t.Fatalf("mismatched upload wrote objects: %#v", store.objects)
	}
}

func createAgentRuntimeBrowserAssetTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			external_runtime TEXT,
			external_runtime_id TEXT
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create browser asset test table: %v", err)
		}
	}
}
