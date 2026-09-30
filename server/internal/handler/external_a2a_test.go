package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestExternalA2AHandlerReturns503WhenNotConfigured(t *testing.T) {
	svc, err := service.NewExternalA2AService(nil, nil, service.ExternalA2AServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewExternalA2AHandler(svc)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/workspaces/ws/external-agents", nil))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusServiceUnavailable || body["error"] != "External agents are not configured on this server" {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestExternalA2AUploadRequiresBearerToken(t *testing.T) {
	svc, err := service.NewExternalA2AService(nil, nil, service.ExternalA2AServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewExternalA2AHandler(svc)
	rec := httptest.NewRecorder()
	h.Upload(rec, httptest.NewRequest(http.MethodPost, "/api/a2a/uploads", strings.NewReader("")))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestDecodeExternalA2AJSONRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"card_url":"https://x","token":"t","extra":1}`))
	var target struct {
		CardURL string `json:"card_url"`
		Token   string `json:"token"`
	}
	if err := decodeExternalA2AJSON(httptest.NewRecorder(), req, &target); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

type externalA2AUploadStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *externalA2AUploadStore) HasPublicURL() bool          { return false }
func (s *externalA2AUploadStore) PublicURL(key string) string { return "" }
func (s *externalA2AUploadStore) GeneratePresignedPutURL(key, _ string, _ int64, _ bool) (string, error) {
	return "https://upload.example.com/" + key, nil
}
func (s *externalA2AUploadStore) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader, _ bool) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	return nil
}
func (s *externalA2AUploadStore) GeneratePresignedGetURL(key, _ string) (string, error) {
	return "https://download.example.com/" + key, nil
}
func (s *externalA2AUploadStore) GeneratePresignedInlineGetURL(key string) (string, error) {
	return "https://inline.example.com/" + key, nil
}
func (s *externalA2AUploadStore) GetObject(context.Context, string) ([]byte, error) { return nil, nil }
func (s *externalA2AUploadStore) DeleteObject(context.Context, string) error        { return nil }

func TestExternalA2AUploadStoresMultipartFileOnTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "a2a.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	const (
		workspaceID = "11111111-1111-1111-1111-111111111111"
		agentID     = "22222222-2222-2222-2222-222222222222"
		externalID  = "33333333-3333-3333-3333-333333333333"
		runID       = "44444444-4444-4444-4444-444444444444"
		taskID      = "55555555-5555-5555-5555-555555555555"
		userID      = "66666666-6666-6666-6666-666666666666"
	)
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, agent_id TEXT, task_id TEXT, target_type TEXT,
			target_id TEXT, runtime_kind TEXT, status TEXT, pause_reason TEXT, triggered_by_user_id TEXT,
			external_runtime TEXT, external_runtime_id TEXT, execution_stage TEXT, completed_at DATETIME,
			input BLOB, output_summary BLOB, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE external_a2a_agents (id TEXT PRIMARY KEY, workspace_id TEXT, agent_id TEXT, name TEXT, description TEXT,
			card_url TEXT, interface_url TEXT, protocol_binding TEXT, protocol_version TEXT, provider_name TEXT, version TEXT,
			skills BLOB, capabilities BLOB, agent_card BLOB, encrypted_token TEXT, token_hint TEXT, status TEXT,
			last_checked_at DATETIME, last_error TEXT, allowed_team_ids BLOB, created_by TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE a2a_task_contexts (id TEXT PRIMARY KEY, workspace_id TEXT, task_id TEXT, external_a2a_agent_id TEXT,
			context_id TEXT, last_remote_task_id TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE a2a_run_upload_tokens (id TEXT PRIMARY KEY, workspace_id TEXT, agent_run_id TEXT, pm_task_id TEXT,
			external_a2a_agent_id TEXT, token_sha256 TEXT UNIQUE, expires_at DATETIME, revoked_at DATETIME,
			bytes_used INTEGER NOT NULL DEFAULT 0, created_at DATETIME)`,
		`CREATE TABLE pm_attachments (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, entity_type TEXT,
			entity_id TEXT, file_name TEXT, file_size INTEGER, content_type TEXT, storage_key TEXT NOT NULL DEFAULT '',
			is_uploaded BOOLEAN NOT NULL DEFAULT 0, uploaded_by_id TEXT, uploaded_by_agent_id TEXT, created_at DATETIME)`,
		`INSERT INTO agent_runs VALUES ('` + runID + `', '` + workspaceID + `', '` + agentID + `', '` + taskID + `', 'task', '` + taskID + `',
			'a2a', 'running', 'none', '` + userID + `', 'agent-runtime', 'rt-1', NULL, NULL, X'7b7d', X'7b7d', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO external_a2a_agents VALUES ('` + externalID + `', '` + workspaceID + `', '` + agentID + `', 'Hermes', '',
			'https://hermes.example.com/.well-known/agent-card.json', 'https://hermes.example.com/a2a', 'JSONRPC', '1.0', '', '',
			X'5b5d', X'7b7d', X'7b7d', '', '', 'active', NULL, '', X'5b5d', '` + userID + `', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	agents := service.NewAgentService(nil, nil, repository.NewAgentRunRepository(db), nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc, err := service.NewExternalA2AService(repository.NewExternalA2ARepository(db), agents, service.ExternalA2AServiceConfig{
		EncryptionKey: "0123456789abcdef0123456789abcdef", PublicAPIBaseURL: "https://api.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := &externalA2AUploadStore{objects: map[string][]byte{}}
	svc.SetTaskCollaborators(nil, service.NewPMAttachmentService(repository.NewPMAttachmentRepository(db), store, nil))

	// The upload token reaches the agent only inside the run's target context.
	data, err := svc.TargetContextData(context.Background(), service.ExternalA2AContextRequest{RuntimeRunID: "rt-1"})
	if err != nil || data == nil {
		t.Fatalf("target context = %v, %v", data, err)
	}
	token := regexp.MustCompile(`Bearer (hpa2a_[A-Za-z0-9_-]+)`).FindStringSubmatch(data["message_appendix"].(string))
	if len(token) != 2 {
		t.Fatalf("no upload token in appendix %q", data["message_appendix"])
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "notes.txt")
	_, _ = part.Write([]byte("deployment finished\n"))
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/a2a/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token[1])
	rec := httptest.NewRecorder()
	NewExternalA2AHandler(svc).Upload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var result service.ExternalA2AUploadResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Filename != "notes.txt" || result.Size != 20 || result.ID == "" {
		t.Fatalf("result = %#v, %v", result, err)
	}
	var attachment model.PMAttachment
	if err := db.First(&attachment, "id = ?", result.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attachment.EntityID != taskID || derefTestString(attachment.UploadedByAgentID) != agentID || attachment.UploadedByID != userID || !attachment.IsUploaded {
		t.Fatalf("attachment = %#v", attachment)
	}
	if got := string(store.objects[attachment.StorageKey]); got != "deployment finished\n" {
		t.Fatalf("stored object = %q", got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/a2a/uploads", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer hpa2a_forged")
	NewExternalA2AHandler(svc).Upload(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged token status = %d", rec.Code)
	}
}

func derefTestString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
