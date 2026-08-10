package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestInsertDocumentImageUsesPrivateWorkspaceArtifactReference(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	artifactID := "40000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)
	if err := db.Exec(`CREATE TABLE agent_run_artifacts (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL,
		artifact_type TEXT NOT NULL, format TEXT NOT NULL, storage_mode TEXT NOT NULL,
		inline_content TEXT, object_key TEXT, metadata BLOB NOT NULL DEFAULT '{}',
		sequence_no INTEGER NOT NULL DEFAULT 0, created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create artifact table: %v", err)
	}
	if err := db.Exec(`INSERT INTO agent_run_artifacts
		(id, workspace_id, run_id, artifact_type, format, storage_mode, object_key, metadata)
		VALUES (?, ?, 'run-1', ?, 'png', 'object', 'private/runtime/run-1/screenshot.png', ?)`,
		artifactID, workspaceID, model.AgentRunArtifactTypeBrowserScreenshot, []byte(`{}`)).Error; err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	svc.SetAgentRunDependencies(nil, repository.NewAgentRunArtifactRepository(db))
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: workspaceID,
		ActorID:     "30000000-0000-0000-0000-000000000001",
		TargetType:  "document",
		TargetID:    documentID,
	}, "docs.insert_document_image", json.RawMessage(`{
		"document_id":"10000000-0000-0000-0000-000000000001",
		"artifact_id":"40000000-0000-0000-0000-000000000001",
		"alt":"Settings page",
		"caption":"Configure the integration"
	}`))
	if err != nil {
		t.Fatalf("insert document image: %v", err)
	}
	if strings.Contains(string(output), "object_key") || strings.Contains(string(output), "screenshot.png") {
		t.Fatalf("output leaked storage details: %s", output)
	}
	blocks, err := blockRepo.ListByDocument(context.Background(), documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Type != "resizableImage" {
		t.Fatalf("unexpected blocks: %#v", blocks)
	}
	content := string(blocks[0].Content)
	if !strings.Contains(content, `"artifactId":"`+artifactID+`"`) || !strings.Contains(content, `"src":"helpin://artifacts/`+artifactID+`"`) {
		t.Fatalf("private artifact reference missing from block: %s", content)
	}
}

func TestInsertDocumentArtifactCreatesPrivateVideoBlock(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	documentID := "10000000-0000-0000-0000-000000000002"
	workspaceID := "20000000-0000-0000-0000-000000000002"
	artifactID := "40000000-0000-0000-0000-000000000002"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)
	if err := db.Exec(`CREATE TABLE agent_run_artifacts (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL,
		artifact_type TEXT NOT NULL, format TEXT NOT NULL, storage_mode TEXT NOT NULL,
		inline_content TEXT, object_key TEXT, metadata BLOB NOT NULL DEFAULT '{}',
		sequence_no INTEGER NOT NULL DEFAULT 0, created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create artifact table: %v", err)
	}
	metadata := []byte(`{"file_name":"org-github-connect-demo.mp4","content_type":"video/mp4","capture":{"record_audio":false}}`)
	if err := db.Exec(`INSERT INTO agent_run_artifacts
		(id, workspace_id, run_id, artifact_type, format, storage_mode, object_key, metadata)
		VALUES (?, ?, 'run-1', ?, 'mp4', 'object', 'private/runtime/run-1/replay.mp4', ?)`,
		artifactID, workspaceID, model.AgentRunArtifactTypeBrowserRecording, metadata).Error; err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	svc.SetAgentRunDependencies(nil, repository.NewAgentRunArtifactRepository(db))
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: workspaceID,
		ActorID:     "30000000-0000-0000-0000-000000000002",
		TargetType:  "document",
		TargetID:    documentID,
	}, "docs.insert_document_artifact", json.RawMessage(`{
		"document_id":"10000000-0000-0000-0000-000000000002",
		"artifact_id":"40000000-0000-0000-0000-000000000002",
		"description":"Organization GitHub connection flow",
		"caption":"Login through the GitHub authorization redirect"
	}`))
	if err != nil {
		t.Fatalf("insert document artifact: %v", err)
	}
	if strings.Contains(string(output), "object_key") || strings.Contains(string(output), "replay.mp4") || !strings.Contains(string(output), `"block_type":"artifactVideo"`) {
		t.Fatalf("unexpected output: %s", output)
	}
	blocks, err := blockRepo.ListByDocument(context.Background(), documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Type != "artifactVideo" {
		t.Fatalf("unexpected blocks: %#v", blocks)
	}
	content := string(blocks[0].Content)
	for _, want := range []string{
		`"artifactId":"` + artifactID + `"`,
		`"src":"helpin://artifacts/` + artifactID + `"`,
		`"fileName":"org-github-connect-demo.mp4"`,
		`"contentType":"video/mp4"`,
		`"description":"Organization GitHub connection flow"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("private video block missing %q: %s", want, content)
		}
	}
}

func TestInsertDocumentArtifactRejectsUnknownFieldsAndUnsupportedTypes(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	documentID := "10000000-0000-0000-0000-000000000003"
	workspaceID := "20000000-0000-0000-0000-000000000003"
	artifactID := "40000000-0000-0000-0000-000000000003"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)
	if err := db.Exec(`CREATE TABLE agent_run_artifacts (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL,
		artifact_type TEXT NOT NULL, format TEXT NOT NULL, storage_mode TEXT NOT NULL,
		inline_content TEXT, object_key TEXT, metadata BLOB NOT NULL DEFAULT '{}',
		sequence_no INTEGER NOT NULL DEFAULT 0, created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create artifact table: %v", err)
	}
	if err := db.Exec(`INSERT INTO agent_run_artifacts
		(id, workspace_id, run_id, artifact_type, format, storage_mode, object_key, metadata)
		VALUES (?, ?, 'run-1', 'unsupported_archive', 'zip', 'object', 'private/archive.zip', ?)`,
		artifactID, workspaceID, []byte(`{}`)).Error; err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	_, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	svc.SetAgentRunDependencies(nil, repository.NewAgentRunArtifactRepository(db))
	meta := model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "document", TargetID: documentID}
	base := `"document_id":"` + documentID + `","artifact_id":"` + artifactID + `","description":"Archive"`
	if _, err := svc.Execute(context.Background(), meta, "docs.insert_document_artifact", json.RawMessage(`{`+base+`,"content_type":"application/zip"}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict input rejection, got %v", err)
	}
	if _, err := svc.Execute(context.Background(), meta, "docs.insert_document_artifact", json.RawMessage(`{`+base+`}`)); err == nil || !strings.Contains(err.Error(), `artifact type "unsupported_archive" cannot be inserted`) {
		t.Fatalf("expected unsupported type rejection, got %v", err)
	}
}

func TestInsertDocumentImageRejectsCrossWorkspaceArtifact(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)
	if err := db.Exec(`CREATE TABLE agent_run_artifacts (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL,
		artifact_type TEXT NOT NULL, format TEXT NOT NULL, storage_mode TEXT NOT NULL,
		inline_content TEXT, object_key TEXT, metadata BLOB NOT NULL DEFAULT '{}',
		sequence_no INTEGER NOT NULL DEFAULT 0, created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create artifact table: %v", err)
	}
	if err := db.Exec(`INSERT INTO agent_run_artifacts
		(id, workspace_id, run_id, artifact_type, format, storage_mode, object_key, metadata)
		VALUES ('artifact-other', 'workspace-other', 'run-1', ?, 'png', 'object', 'private/other.png', ?)`,
		model.AgentRunArtifactTypeBrowserScreenshot, []byte(`{}`)).Error; err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	_, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	svc.SetAgentRunDependencies(nil, repository.NewAgentRunArtifactRepository(db))
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: workspaceID,
		TargetType:  "document",
		TargetID:    documentID,
	}, "docs.insert_document_image", json.RawMessage(`{
		"document_id":"10000000-0000-0000-0000-000000000001",
		"artifact_id":"artifact-other",
		"alt":"Should not render"
	}`))
	if err == nil || !strings.Contains(err.Error(), "artifact not found") {
		t.Fatalf("expected scoped artifact rejection, got %v", err)
	}
}
