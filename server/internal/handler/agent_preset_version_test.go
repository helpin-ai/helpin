package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// TestAgentHandler_UpdateWorkspacePresetVersion_NotFound verifies that a PUT
// against a missing version id maps the service's not-found sentinel to HTTP 404.
func TestAgentHandler_UpdateWorkspacePresetVersion_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newAgentPresetVersionTestHandler(t)

	body, _ := json.Marshal(model.UpdateWorkspaceAgentPresetVersionRequest{
		Label: stringPtr("Irrelevant"),
	})
	req := newAgentPresetVersionRequest(t, http.MethodPut, "missing-id", body, "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.UpdateWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAgentHandler_UpdateWorkspacePresetVersion_Success(t *testing.T) {
	t.Parallel()

	h, db := newAgentPresetVersionTestHandler(t)
	version := seedAgentPresetVersion(t, db, "ws-test", model.AgentPresetEpicPlanner, "epic_planner_ws_v1", "Workspace v1")

	newLabel := "Workspace v2"
	body, _ := json.Marshal(model.UpdateWorkspaceAgentPresetVersionRequest{
		Label: &newLabel,
	})
	req := newAgentPresetVersionRequest(t, http.MethodPut, version.ID, body, "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.UpdateWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	var preset model.AgentPresetDefinition
	if err := json.Unmarshal(rr.Body.Bytes(), &preset); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if preset.VersionLabel != newLabel {
		t.Fatalf("version label = %q, want %q", preset.VersionLabel, newLabel)
	}
	if preset.ID == nil || *preset.ID != version.ID {
		t.Fatalf("id = %+v, want %q", preset.ID, version.ID)
	}
}

// TestAgentHandler_UpdateWorkspacePresetVersion_InvalidJSON verifies that
// malformed JSON body returns HTTP 400 without reaching the service layer.
func TestAgentHandler_UpdateWorkspacePresetVersion_InvalidJSON(t *testing.T) {
	t.Parallel()

	h, _ := newAgentPresetVersionTestHandler(t)

	req := newAgentPresetVersionRequest(t, http.MethodPut, "any-id", []byte("not-json"), "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.UpdateWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAgentHandler_UpdateWorkspacePresetVersion_ValidationError(t *testing.T) {
	t.Parallel()

	h, db := newAgentPresetVersionTestHandler(t)
	version := seedAgentPresetVersion(t, db, "ws-test", model.AgentPresetCodeBuilder, "code_builder_ws_v1", "Workspace v1")

	blank := "   "
	body, _ := json.Marshal(model.UpdateWorkspaceAgentPresetVersionRequest{
		Label: &blank,
	})
	req := newAgentPresetVersionRequest(t, http.MethodPut, version.ID, body, "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.UpdateWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAgentHandler_UpdateWorkspacePresetVersion_InvalidRouting(t *testing.T) {
	t.Parallel()

	h, db := newAgentPresetVersionTestHandler(t)
	version := seedAgentPresetVersion(t, db, "ws-test", model.AgentPresetEpicPlanner, "epic_planner_ws_v1", "Workspace v1")

	invalidProvider := "bogus-provider"
	body, _ := json.Marshal(model.UpdateWorkspaceAgentPresetVersionRequest{
		Provider: &invalidProvider,
	})
	req := newAgentPresetVersionRequest(t, http.MethodPut, version.ID, body, "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.UpdateWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
}

// TestAgentHandler_DeleteWorkspacePresetVersion_NotFound verifies 404 mapping
// on DELETE for a missing version id.
func TestAgentHandler_DeleteWorkspacePresetVersion_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newAgentPresetVersionTestHandler(t)

	req := newAgentPresetVersionRequest(t, http.MethodDelete, "missing-id", nil, "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.DeleteWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rr.Code, rr.Body.String())
	}
}

// TestAgentHandler_DeleteWorkspacePresetVersion_Pinned verifies that attempting
// to delete a preset version that is pinned to a system agent maps
// ErrWorkspacePresetVersionPinned to HTTP 409.
func TestAgentHandler_DeleteWorkspacePresetVersion_Pinned(t *testing.T) {
	t.Parallel()

	h, db := newAgentPresetVersionTestHandler(t)
	const workspaceID = "ws-test"

	versionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	version := &model.WorkspaceAgentPresetVersion{
		ID:                    uuid.NewString(),
		WorkspaceID:           workspaceID,
		FamilyKey:             model.AgentPresetCodeBuilder,
		VersionKey:            "code_builder_ws_v1",
		Label:                 "Workspace v1",
		RuntimeKind:           "codex",
		ExecutionConfig:       model.JSONBlob(`{}`),
		InstructionSkills:     json.RawMessage(`[]`),
		AllowedTools:          json.RawMessage(`[]`),
		SupportedModes:        json.RawMessage(`["autonomous"]`),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	if err := versionRepo.Create(context.Background(), version); err != nil {
		t.Fatalf("seed version: %v", err)
	}

	agentRepo := repository.NewAgentRepository(db)
	if err := agentRepo.Create(context.Background(), &model.Agent{
		ID:                    uuid.NewString(),
		WorkspaceID:           workspaceID,
		IsSystem:              true,
		Name:                  "Forge",
		PresetKey:             model.AgentPresetCodeBuilder,
		PresetVersionKey:      version.VersionKey,
		Status:                "active",
		RuntimeKind:           "codex",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		ExecutionConfig:       model.JSONBlob(`{}`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	req := newAgentPresetVersionRequest(t, http.MethodDelete, version.ID, nil, workspaceID, "user-1")
	rr := httptest.NewRecorder()

	h.DeleteWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "pinned") {
		t.Fatalf("expected pinned error message, got %s", rr.Body.String())
	}
}

func TestAgentHandler_DeleteWorkspacePresetVersion_Success(t *testing.T) {
	t.Parallel()

	h, db := newAgentPresetVersionTestHandler(t)
	version := seedAgentPresetVersion(t, db, "ws-test", model.AgentPresetReviewAgent, "review_agent_ws_v1", "Workspace Review")

	req := newAgentPresetVersionRequest(t, http.MethodDelete, version.ID, nil, "ws-test", "user-1")
	rr := httptest.NewRecorder()

	h.DeleteWorkspacePresetVersion(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rr.Code, rr.Body.String())
	}
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Fatalf("expected empty body, got %q", rr.Body.String())
	}
}

func newAgentPresetVersionTestHandler(t *testing.T) (*AgentHandler, *gorm.DB) {
	t.Helper()

	dbName := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	schema := []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			tools BLOB NOT NULL DEFAULT '[]',
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT x'5b5d',
			schedule TEXT,
			target_selector TEXT,
			trigger_events BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'never',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_agent_preset_versions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			family_key TEXT NOT NULL,
			version_key TEXT NOT NULL,
			label TEXT NOT NULL,
			description TEXT,
			source_version_key TEXT,
			runtime_kind TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_preamble TEXT,
			instruction_skills BLOB NOT NULL DEFAULT '[]',
			instruction_template_version TEXT NOT NULL DEFAULT '',
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			supported_modes BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_by TEXT,
			updated_by TEXT,
			last_edited_at DATETIME,
			deleted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)

	svc := service.NewAgentService(
		agentRepo,
		workspacePresetVersionRepo,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	)
	svc.SetModelProviderConfig("test-anthropic-key", "test-openai-key", "", "", false, "", "")

	return NewAgentHandler(svc), db
}

func seedAgentPresetVersion(t *testing.T, db *gorm.DB, workspaceID, familyKey, versionKey, label string) *model.WorkspaceAgentPresetVersion {
	t.Helper()

	runtimeKind := "codex"
	if familyKey == model.AgentPresetEpicPlanner {
		runtimeKind = "native_sdk"
	}
	version := &model.WorkspaceAgentPresetVersion{
		ID:                    uuid.NewString(),
		WorkspaceID:           workspaceID,
		FamilyKey:             familyKey,
		VersionKey:            versionKey,
		Label:                 label,
		RuntimeKind:           runtimeKind,
		ExecutionConfig:       model.JSONBlob(`{}`),
		InstructionSkills:     json.RawMessage(`[]`),
		AllowedTools:          json.RawMessage(`[]`),
		SupportedModes:        json.RawMessage(`["autonomous"]`),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	versionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	if err := versionRepo.Create(context.Background(), version); err != nil {
		t.Fatalf("seed version: %v", err)
	}
	return version
}

func newAgentPresetVersionRequest(t *testing.T, method, versionID string, body []byte, workspaceID, actorID string) *http.Request {
	t.Helper()

	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, "/api/pm/agent-preset-versions/"+versionID, nil)
	} else {
		req = httptest.NewRequest(method, "/api/pm/agent-preset-versions/"+versionID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", versionID)

	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = middleware.WithWorkspaceID(ctx, workspaceID)
	ctx = middleware.WithUserID(ctx, actorID)
	return req.WithContext(ctx)
}

func mustExecAgentPresetVersion(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func stringPtr(s string) *string { return &s }
