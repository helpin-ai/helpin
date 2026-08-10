package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSanitizeAgentRunMessageForPostgresRemovesNullCharacters(t *testing.T) {
	message := &model.AgentRunMessage{
		Content:         "created task\x00with scanner output",
		ContentBlocks:   json.RawMessage(`[{"type":"tool_result","text":"secret\u0000match"}]`),
		TurnSegments:    json.RawMessage(`[{"kind":"assistant_message","assistant_message":{"content":"triage\u0000done"}}]`),
		ToolInvocations: json.RawMessage(`[{"tool_name":"scan_gitleaks","result":{"bad\u0000key":"finding\u0000value"}}]`),
		TokenUsage:      json.RawMessage(`{"input_tokens":1,"note":"usage\u0000metadata"}`),
	}

	sanitizeAgentRunMessageForPostgres(message)

	if strings.ContainsRune(message.Content, '\x00') {
		t.Fatalf("content still contains null character: %q", message.Content)
	}

	for name, raw := range map[string]json.RawMessage{
		"content_blocks":   message.ContentBlocks,
		"turn_segments":    message.TurnSegments,
		"tool_invocations": message.ToolInvocations,
		"token_usage":      message.TokenUsage,
	} {
		if strings.Contains(string(raw), `\u0000`) {
			t.Fatalf("%s still contains postgres-rejected null escape: %s", name, string(raw))
		}
		if strings.ContainsRune(string(raw), '\x00') {
			t.Fatalf("%s still contains raw null character: %s", name, string(raw))
		}
		if !json.Valid(raw) {
			t.Fatalf("%s is not valid json after sanitization: %s", name, string(raw))
		}
	}
}

func TestSanitizePostgresJSONRawMessageDropsInvalidJSON(t *testing.T) {
	raw := sanitizePostgresJSONRawMessage(json.RawMessage(`{"unterminated"`), nil)
	if raw != nil {
		t.Fatalf("expected invalid json to be dropped, got %s", string(raw))
	}
}

func TestAgentRunArtifactCreateSanitizesInlineContentAndMetadata(t *testing.T) {
	db := setupSanitizeTestDB(t, `CREATE TABLE agent_run_artifacts (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		artifact_type TEXT NOT NULL,
		format TEXT NOT NULL DEFAULT 'text',
		storage_mode TEXT NOT NULL DEFAULT 'inline',
		inline_content TEXT,
		object_key TEXT,
		metadata TEXT NOT NULL DEFAULT '{}',
		sequence_no INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME
	)`)
	repo := NewAgentRunArtifactRepository(db)

	content := "Tool output with binary\x00data"
	artifact := &model.AgentRunArtifact{
		ID:            "artifact-1",
		WorkspaceID:   "workspace-1",
		RunID:         "run-1",
		ArtifactType:  "tool_log",
		Format:        "text",
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage(`{"bad\u0000key":"bad\u0000value"}`),
		SequenceNo:    1,
	}

	if err := repo.Create(context.Background(), artifact); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if artifact.InlineContent == nil || strings.ContainsRune(*artifact.InlineContent, '\x00') {
		t.Fatalf("inline content was not sanitized: %#v", artifact.InlineContent)
	}
	if strings.Contains(string(artifact.Metadata), `\u0000`) || !json.Valid(artifact.Metadata) {
		t.Fatalf("metadata was not sanitized: %s", string(artifact.Metadata))
	}
}

func TestCodingSessionStateSnapshotUpsertSanitizesPayload(t *testing.T) {
	db := setupSanitizeTestDB(t, codingSessionSnapshotTestSchema)
	repo := NewCodingSessionStateSnapshotRepository(db)

	snapshot := &model.CodingSessionStateSnapshot{
		ID:              "snapshot-1",
		WorkspaceID:     "workspace-1",
		RunID:           "run-1",
		SnapshotPayload: json.RawMessage(`{"live_turn_segments":[{"kind":"tool_call","tool_call":{"result":"bad\u0000value"}}]}`),
	}

	if err := repo.Upsert(context.Background(), snapshot); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if strings.Contains(string(snapshot.SnapshotPayload), `\u0000`) || !json.Valid(snapshot.SnapshotPayload) {
		t.Fatalf("snapshot payload was not sanitized: %s", string(snapshot.SnapshotPayload))
	}
}

func TestCodingSessionStateSnapshotUpsertReplacesInvalidPayload(t *testing.T) {
	db := setupSanitizeTestDB(t, codingSessionSnapshotTestSchema)
	repo := NewCodingSessionStateSnapshotRepository(db)

	snapshot := &model.CodingSessionStateSnapshot{
		ID:              "snapshot-1",
		WorkspaceID:     "workspace-1",
		RunID:           "run-1",
		SnapshotPayload: json.RawMessage(`{"unterminated"`),
	}

	if err := repo.Upsert(context.Background(), snapshot); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if got := string(snapshot.SnapshotPayload); got != "{}" {
		t.Fatalf("expected invalid snapshot payload to become {}, got %s", got)
	}
}

const codingSessionSnapshotTestSchema = `CREATE TABLE coding_session_state_snapshots (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	schema_version TEXT NOT NULL DEFAULT 'helpin.coding_session.stream.v1',
	through_sequence INTEGER NOT NULL DEFAULT 0,
	snapshot_payload TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME,
	updated_at DATETIME,
	UNIQUE(workspace_id, run_id)
)`

func setupSanitizeTestDB(t *testing.T, schemas ...string) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:sanitize_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, schema := range schemas {
		if err := db.Exec(schema).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}
