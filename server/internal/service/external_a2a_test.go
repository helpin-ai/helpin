package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const externalA2ATestKey = "0123456789abcdef0123456789abcdef"

type externalA2ATestEnv struct {
	db          *gorm.DB
	svc         *ExternalA2AService
	agents      *AgentService
	comments    *PMCommentService
	runtime     *fakeAgentRuntimeSignalClient
	store       *recordingA2AStore
	server      *httptest.Server
	workspaceID string
	userID      string
	taskID      string
}

type recordingA2AStore struct {
	fakeAttachmentStore
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *recordingA2AStore) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader, _ bool) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	return nil
}

var externalA2ATestMP4 = append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}, bytes.Repeat([]byte{1}, 64)...)

func newExternalA2ATestEnv(t *testing.T) *externalA2ATestEnv {
	t.Helper()
	db := newTestDB(t)
	registerAgentTestUUIDCallback(t, db)
	for _, stmt := range externalA2ATestSchema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	env := &externalA2ATestEnv{db: db, workspaceID: uuid.NewString(), userID: uuid.NewString(), taskID: uuid.NewString()}
	if err := db.Exec(`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id, created_at, updated_at)
		VALUES (?, ?, 1, 'Record a demo', 'wf', 'state', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, env.taskID, env.workspaceID).Error; err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	env.server = httptest.NewServer(mux)
	t.Cleanup(env.server.Close)
	mux.HandleFunc("/hermes/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "Hermes", "description": "Operations agent", "version": "0.21.5",
			"provider":            map[string]any{"organization": "Nous Research"},
			"supportedInterfaces": []map[string]any{{"url": env.server.URL + "/hermes/a2a", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"}},
			"capabilities":        map[string]any{"streaming": true, "pushNotifications": true},
			"skills":              []map[string]any{{"id": "demo", "name": "Demo recording", "description": "Records demos"}},
		})
	})
	mux.HandleFunc("/files/demo.mp4", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(externalA2ATestMP4) })

	activity := NewPMActivityService(repository.NewPMActivityRepository(db))
	env.runtime = &fakeAgentRuntimeSignalClient{}
	env.agents = &AgentService{
		agentRepo:      repository.NewAgentRepository(db),
		runRepo:        repository.NewAgentRunRepository(db),
		runMessageRepo: repository.NewAgentRunMessageRepository(db),
		taskRepo:       repository.NewPMTaskRepository(db),
		activitySvc:    activity,
		// Task runs resolve a delivery target; this task has none, as most do.
		gitService: &GitService{deliveryRepo: repository.NewTaskDeliveryTargetRepository(db), taskRepo: repository.NewPMTaskRepository(db)},
	}
	env.agents.SetAgentRuntimeClient(env.runtime)
	env.agents.SetAgentRuntimeLaunchEnabled(true)
	attachmentRepo := repository.NewPMAttachmentRepository(db)
	env.store = &recordingA2AStore{objects: map[string][]byte{}}
	attachments := NewPMAttachmentService(attachmentRepo, env.store, nil)
	env.comments = NewPMCommentService(repository.NewPMCommentRepository(db), repository.NewPMTaskRepository(db), attachmentRepo, activity, nil, nil, nil, nil)
	svc, err := NewExternalA2AService(repository.NewExternalA2ARepository(db), env.agents, ExternalA2AServiceConfig{
		EncryptionKey: externalA2ATestKey, AllowedPrivateHosts: []string{"127.0.0.1"}, PublicAPIBaseURL: "https://api.example.com/",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetTaskCollaborators(env.comments, attachments)
	env.comments.SetTaskCommentRouter(svc)
	env.svc = svc
	return env
}

func (env *externalA2ATestEnv) connect(t *testing.T, token string) *model.ExternalA2AAgent {
	t.Helper()
	record, err := env.svc.Create(context.Background(), env.workspaceID, env.userID, model.CreateExternalA2AAgentRequest{
		CardURL: env.server.URL + "/hermes", Token: token,
	})
	if err != nil {
		t.Fatalf("create external agent: %v", err)
	}
	return record
}

func (env *externalA2ATestEnv) seedRun(t *testing.T, agentID, runtimeKind, status string, mutate func(*model.AgentRun)) *model.AgentRun {
	t.Helper()
	runtimeName, runtimeID := agentRuntimeName, "rt-"+uuid.NewString()
	run := &model.AgentRun{
		ID: uuid.NewString(), WorkspaceID: env.workspaceID, AgentID: agentID, TaskID: &env.taskID,
		TargetType: "task", TargetID: env.taskID, RuntimeKind: runtimeKind, Status: status,
		ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone, TriggeredByUserID: &env.userID,
		ExternalRuntime: &runtimeName, ExternalRuntimeID: &runtimeID,
		Input: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{}`),
	}
	if mutate != nil {
		mutate(run)
	}
	if err := env.db.Create(run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	return run
}

func (env *externalA2ATestEnv) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := env.db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func TestExternalA2APreviewAcceptsBaseURL(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	card, err := env.svc.Preview(context.Background(), model.PreviewExternalA2AAgentRequest{CardURL: env.server.URL + "/hermes/"})
	if err != nil {
		t.Fatal(err)
	}
	if card.Name != "Hermes" || card.ProtocolBinding != "JSONRPC" || card.ProviderName != "Nous Research" ||
		card.CardURL != env.server.URL+"/hermes/.well-known/agent-card.json" || len(card.Skills) != 1 || !card.Capabilities.PushNotifications {
		t.Fatalf("card = %#v", card)
	}
	if env.count(t, `SELECT COUNT(*) FROM external_a2a_agents`)+env.count(t, `SELECT COUNT(*) FROM agents`) != 0 {
		t.Fatal("preview must not save anything")
	}
}

func TestExternalA2APreviewRejectsLoopbackWithoutAllowlist(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	strict, err := NewExternalA2AService(env.svc.repo, env.agents, ExternalA2AServiceConfig{EncryptionKey: externalA2ATestKey})
	if err != nil {
		t.Fatal(err)
	}
	_, err = strict.Preview(context.Background(), model.PreviewExternalA2AAgentRequest{CardURL: env.server.URL + "/hermes"})
	var input *ExternalA2AInputError
	if !errors.As(err, &input) {
		t.Fatalf("expected input error for loopback card URL, got %v", err)
	}
	disabled, err := NewExternalA2AService(env.svc.repo, env.agents, ExternalA2AServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := disabled.List(context.Background(), env.workspaceID); !errors.Is(err, ErrExternalA2ADisabled) || ExternalA2AErrorStatus(err) != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 disabled error, got %v", err)
	}
}

func TestExternalA2ACreateEncryptsTokenAndLinksAgent(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	record := env.connect(t, "secret-token-a1b2")

	if record.EncryptedToken == "" || strings.Contains(record.EncryptedToken, "secret-token") || record.TokenHint != "…a1b2" {
		t.Fatalf("token storage = %q hint %q", record.EncryptedToken, record.TokenHint)
	}
	if token, err := env.svc.decryptToken(record); err != nil || token != "secret-token-a1b2" {
		t.Fatalf("decrypt = %q, %v", token, err)
	}
	items, err := env.svc.List(context.Background(), env.workspaceID)
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %v, %v", items, err)
	}
	encoded, _ := json.Marshal(map[string]any{"items": items, "created": record})
	if strings.Contains(string(encoded), "secret-token") || strings.Contains(string(encoded), record.EncryptedToken) || strings.Contains(string(encoded), "agent_card") {
		t.Fatalf("serialized external agent leaks secrets: %s", encoded)
	}
	agent, err := env.agents.agentRepo.GetByID(context.Background(), env.workspaceID, record.AgentID)
	if err != nil || agent == nil {
		t.Fatalf("linked agent = %v, %v", agent, err)
	}
	if agent.RuntimeKind != model.AgentRuntimeKindA2A || agent.TriggerMode != "auto_on_assignment" || agent.Name != "Hermes" ||
		externalA2AAgentIDFromConfig(agent.ExecutionConfig) != record.ID || agent.Provider != nil || agent.Model != nil {
		t.Fatalf("linked agent = %#v (config %s)", agent, agent.ExecutionConfig)
	}
	normalizeAgentRecord(agent)
	if agent.TriggerMode != "auto_on_assignment" || externalA2AAgentIDFromConfig(agent.ExecutionConfig) != record.ID {
		t.Fatalf("normalization dropped external agent settings: %#v", agent)
	}
	runtimeAgent := runtimeAgentFromHelpinAgent(agent, "helpin")
	if runtimeAgent.RuntimeKind != model.AgentRuntimeKindA2A || runtimeAgent.Model != "" || len(runtimeAgent.AllowedTools) != 0 ||
		!strings.Contains(string(runtimeAgent.ExecutionConfig), record.ID) {
		t.Fatalf("runtime agent = %#v", runtimeAgent)
	}
}

func TestExternalA2AUpdateAndDelete(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	record := env.connect(t, "")
	name, status, token := "Hermes Ops", model.ExternalA2AStatusDisabled, "rotated-token-9z9z"
	updated, err := env.svc.Update(context.Background(), env.workspaceID, record.ID, env.userID, model.UpdateExternalA2AAgentRequest{Name: &name, Status: &status, Token: &token})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.Status != status || updated.TokenHint != "…9z9z" {
		t.Fatalf("updated = %#v", updated)
	}
	agent, _ := env.agents.agentRepo.GetByID(context.Background(), env.workspaceID, record.AgentID)
	if agent == nil || agent.Name != name {
		t.Fatalf("linked agent name not synced: %#v", agent)
	}
	bad := "paused"
	if _, err := env.svc.Update(context.Background(), env.workspaceID, record.ID, env.userID, model.UpdateExternalA2AAgentRequest{Status: &bad}); ExternalA2AErrorStatus(err) != http.StatusBadRequest {
		t.Fatalf("invalid status error = %v", err)
	}
	if err := env.svc.Delete(context.Background(), env.workspaceID, record.ID, env.userID); err != nil {
		t.Fatal(err)
	}
	if env.count(t, `SELECT COUNT(*) FROM agents`)+env.count(t, `SELECT COUNT(*) FROM external_a2a_agents`) != 0 {
		t.Fatal("delete must remove the connection and its linked agent")
	}
	if err := env.svc.Delete(context.Background(), env.workspaceID, record.ID, env.userID); !errors.Is(err, ErrExternalA2ANotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestExternalA2ARuntimeKindIsReservedForConnectedAgents(t *testing.T) {
	if err := validateRuntimeKind(model.AgentRuntimeKindA2A); err == nil {
		t.Fatal("generic create/update must not accept runtime_kind a2a")
	}
	manual := &model.Agent{RuntimeKind: model.AgentRuntimeKindA2A, ExecutionConfig: model.JSONBlob(`{}`)}
	if err := validateRuntimeForAgent(manual); err == nil {
		t.Fatal("a2a agent without an external connection must be rejected")
	}
	native := &model.Agent{RuntimeKind: "native_sdk", ExecutionConfig: externalA2AExecutionConfig("x")}
	if _, err := parseAndValidateExecutionConfig(native); err == nil {
		t.Fatal("native agents must not carry external_a2a_agent_id")
	}
	provider := "openai"
	if _, err := sanitizeExternalA2AAgentUpdate(&model.Agent{RuntimeKind: model.AgentRuntimeKindA2A}, model.UpdateAgentRequest{Provider: &provider}); err == nil {
		t.Fatal("external agents must reject model settings from the generic editor")
	}
}

func TestExternalA2ATargetContextOnlyForExternalRuns(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "secret-token-a1b2")
	if err := env.svc.repo.UpsertTaskContext(ctx, &model.A2ATaskContext{WorkspaceID: env.workspaceID, TaskID: env.taskID, ExternalA2AAgentID: record.ID, ContextID: "ctx-1"}); err != nil {
		t.Fatal(err)
	}
	run := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusRunning, nil)
	req := ExternalA2AContextRequest{RuntimeRunID: *run.ExternalRuntimeID, AgentID: record.AgentID, WorkspaceID: env.workspaceID, TargetType: "task", TargetID: env.taskID}

	data, err := env.svc.TargetContextData(ctx, req)
	if err != nil || data == nil {
		t.Fatalf("data = %v, %v", data, err)
	}
	auth, _ := data["auth"].(map[string]any)
	appendix, _ := data["message_appendix"].(string)
	if data["external_agent_id"] != record.ID || auth["token"] != "secret-token-a1b2" || data["context_id"] != "ctx-1" ||
		data["max_turn_seconds"] != externalA2AMaxTurnSeconds || data["allow_private_network"] != true ||
		!strings.Contains(appendix, "https://api.example.com/api/a2a/uploads") || !strings.Contains(appendix, "Bearer hpa2a_") {
		t.Fatalf("data.a2a = %#v", data)
	}
	again, _ := env.svc.TargetContextData(ctx, req)
	if again["message_appendix"] != appendix || env.count(t, `SELECT COUNT(*) FROM a2a_run_upload_tokens`) != 1 {
		t.Fatal("upload link must be reused across turns")
	}

	hinted := req
	hinted.RuntimeRunID, hinted.HostRunID = "rt-other", run.ID
	if data, _ := env.svc.TargetContextData(ctx, hinted); data != nil {
		t.Fatal("host run hint must not match a run bound to another runtime run")
	}
	other := req
	other.WorkspaceID = uuid.NewString()
	if data, _ := env.svc.TargetContextData(ctx, other); data != nil {
		t.Fatal("run from another workspace must not receive credentials")
	}
	native := env.seedRun(t, record.AgentID, "native_sdk", model.AgentRunStatusRunning, nil)
	if data, _ := env.svc.TargetContextData(ctx, ExternalA2AContextRequest{RuntimeRunID: *native.ExternalRuntimeID}); data != nil {
		t.Fatal("native runs must not receive data.a2a")
	}
	completed := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusCompleted, nil)
	if data, _ := env.svc.TargetContextData(ctx, ExternalA2AContextRequest{RuntimeRunID: *completed.ExternalRuntimeID}); data != nil {
		t.Fatal("terminal runs must not receive data.a2a")
	}
	disabled := model.ExternalA2AStatusDisabled
	if _, err := env.svc.Update(ctx, env.workspaceID, record.ID, env.userID, model.UpdateExternalA2AAgentRequest{Status: &disabled}); err != nil {
		t.Fatal(err)
	}
	if data, _ := env.svc.TargetContextData(ctx, req); data != nil {
		t.Fatal("disabled external agents must not receive data.a2a")
	}
}

func TestExternalA2ATargetContextAllowsRecentCancellation(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "secret-token-a1b2")
	recent := time.Now().Add(-5 * time.Minute)
	stale := time.Now().Add(-20 * time.Minute)
	cancelled := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusCancelled, func(run *model.AgentRun) { run.CompletedAt = &recent })
	data, err := env.svc.TargetContextData(ctx, ExternalA2AContextRequest{RuntimeRunID: *cancelled.ExternalRuntimeID})
	if err != nil || data == nil {
		t.Fatalf("recently cancelled run needs connection details to cancel remotely: %v, %v", data, err)
	}
	if auth, _ := data["auth"].(map[string]any); auth["token"] != "secret-token-a1b2" || data["message_appendix"] != "" {
		t.Fatalf("cancel context = %#v", data)
	}
	if env.count(t, `SELECT COUNT(*) FROM a2a_run_upload_tokens`) != 0 {
		t.Fatal("cancelled runs must not mint upload links")
	}
	old := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusCancelled, func(run *model.AgentRun) { run.CompletedAt = &stale })
	if data, _ := env.svc.TargetContextData(ctx, ExternalA2AContextRequest{RuntimeRunID: *old.ExternalRuntimeID}); data != nil {
		t.Fatal("stale cancellation must not receive data.a2a")
	}
	failed := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusFailed, func(run *model.AgentRun) { run.CompletedAt = &recent })
	if data, _ := env.svc.TargetContextData(ctx, ExternalA2AContextRequest{RuntimeRunID: *failed.ExternalRuntimeID}); data != nil {
		t.Fatal("failed runs must not receive data.a2a")
	}
}

func TestExternalA2AUploads(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "")
	run := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusRunning, nil)
	token, err := env.svc.runUploadToken(ctx, run, record)
	if err != nil {
		t.Fatal(err)
	}

	grant, err := env.svc.AuthorizeUpload(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	result, err := env.svc.ReceiveUpload(ctx, grant, "../demo.mp4", bytes.NewReader(externalA2ATestMP4))
	if err != nil {
		t.Fatal(err)
	}
	if result.Filename != "demo.mp4" || result.Size != int64(len(externalA2ATestMP4)) {
		t.Fatalf("result = %#v", result)
	}
	var attachment model.PMAttachment
	if err := env.db.First(&attachment, "id = ?", result.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attachment.EntityID != env.taskID || derefString(attachment.UploadedByAgentID) != record.AgentID || !attachment.IsUploaded || attachment.ContentType != "video/mp4" {
		t.Fatalf("attachment = %#v", attachment)
	}

	t.Run("wrong type", func(t *testing.T) {
		if _, err := env.svc.ReceiveUpload(ctx, grant, "tool.exe", strings.NewReader("MZ")); ExternalA2AErrorStatus(err) != http.StatusUnsupportedMediaType {
			t.Fatalf("exe upload = %v", err)
		}
		if _, err := env.svc.ReceiveUpload(ctx, grant, "shot.png", strings.NewReader("<html><script>x</script></html>")); ExternalA2AErrorStatus(err) != http.StatusUnsupportedMediaType {
			t.Fatalf("disguised png = %v", err)
		}
	})
	t.Run("oversize", func(t *testing.T) {
		previous := externalA2AMaxFileBytes
		externalA2AMaxFileBytes = 16
		t.Cleanup(func() { externalA2AMaxFileBytes = previous })
		if _, err := env.svc.ReceiveUpload(ctx, grant, "log.txt", strings.NewReader(strings.Repeat("a", 64))); ExternalA2AErrorStatus(err) != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversize upload = %v", err)
		}
	})
	t.Run("expired", func(t *testing.T) {
		if err := env.db.Exec(`UPDATE a2a_run_upload_tokens SET expires_at = ?`, time.Now().Add(-time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := env.svc.AuthorizeUpload(ctx, token); ExternalA2AErrorStatus(err) != http.StatusUnauthorized {
			t.Fatalf("expired token = %v", err)
		}
		if err := env.db.Exec(`UPDATE a2a_run_upload_tokens SET expires_at = ?`, time.Now().Add(time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
	})
	t.Run("revoked", func(t *testing.T) {
		if err := env.svc.RevokeRunUploadTokens(ctx, run.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := env.svc.AuthorizeUpload(ctx, token); ExternalA2AErrorStatus(err) != http.StatusUnauthorized {
			t.Fatalf("revoked token = %v", err)
		}
	})
	if _, err := env.svc.AuthorizeUpload(ctx, "hpa2a_forged"); ExternalA2AErrorStatus(err) != http.StatusUnauthorized {
		t.Fatalf("forged token = %v", err)
	}
}

func TestExternalA2AProjectionIsIdempotent(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "secret-token-a1b2")
	run := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusRunning, nil)
	projection := NewAgentRuntimeProjectionService(repository.NewAgentRunRepository(env.db))
	projection.SetExternalA2AProjector(env.svc)
	event := AgentRuntimeEventEnvelope{
		Type: agentRuntimeEventA2ATask, RunID: *run.ExternalRuntimeID, HostRunID: run.ID,
		Data: map[string]any{
			"external_agent_id": record.ID, "context_id": "ctx-9", "task_id": "remote-1", "state": "completed",
			"message": "Demo recorded.", "message_id": "helpin-" + run.ID + "-initial",
			"files": []any{map[string]any{"name": "demo.mp4", "media_type": "video/mp4", "url": env.server.URL + "/files/demo.mp4"}},
		},
	}
	for i := 0; i < 2; i++ {
		if err := projection.ApplyEvent(ctx, event); err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
		env.svc.Wait()
	}
	if n := env.count(t, `SELECT COUNT(*) FROM pm_comments WHERE entity_id = ? AND agent_id = ? AND agent_run_id = ?`, env.taskID, record.AgentID, run.ID); n != 1 {
		t.Fatalf("comments = %d, want 1", n)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM pm_attachments WHERE entity_id = ? AND uploaded_by_agent_id = ? AND is_uploaded = 1`, env.taskID, record.AgentID); n != 1 {
		t.Fatalf("attachments = %d, want 1", n)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM a2a_task_contexts WHERE task_id = ? AND context_id = 'ctx-9' AND last_remote_task_id = 'remote-1'`, env.taskID); n != 1 {
		t.Fatalf("contexts = %d, want 1", n)
	}
}

func TestExternalA2ATerminalEventRevokesUploadLinks(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "")
	run := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusRunning, nil)
	if _, err := env.svc.runUploadToken(ctx, run, record); err != nil {
		t.Fatal(err)
	}
	projection := NewAgentRuntimeProjectionService(repository.NewAgentRunRepository(env.db))
	projection.SetExternalA2AProjector(env.svc)
	if err := projection.ApplyEvent(ctx, AgentRuntimeEventEnvelope{Type: "run.cancelled", RunID: *run.ExternalRuntimeID, HostRunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM a2a_run_upload_tokens WHERE revoked_at IS NULL`); n != 0 {
		t.Fatalf("active upload tokens after cancel = %d", n)
	}
}

func TestAutoOnAssignmentStartsExactlyOneRun(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	record := env.connect(t, "")
	tasks := &PMTaskService{agentService: env.agents, logger: slog.Default()}
	for i := 0; i < 2; i++ {
		tasks.dispatchAssignmentRun(context.Background(), env.workspaceID, env.taskID, &record.AgentID, env.userID)
		tasks.WaitForAssignmentRuns()
	}
	if n := env.count(t, `SELECT COUNT(*) FROM agent_runs WHERE agent_id = ? AND target_id = ?`, record.AgentID, env.taskID); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
	if len(env.runtime.startRunCalls) != 1 || len(env.runtime.upsertAgents) != 1 {
		t.Fatalf("runtime starts = %d upserts = %d", len(env.runtime.startRunCalls), len(env.runtime.upsertAgents))
	}
	if got := env.runtime.upsertAgents[0]; got.RuntimeKind != model.AgentRuntimeKindA2A || got.Model != "" {
		t.Fatalf("upserted runtime agent = %#v", got)
	}
	var run model.AgentRun
	if err := env.db.First(&run, "agent_id = ?", record.AgentID).Error; err != nil {
		t.Fatal(err)
	}
	if run.RuntimeKind != model.AgentRuntimeKindA2A || runInputTriggerType(&run) != agentRunTriggerTypeTaskAssigned || strings.Contains(string(run.Input), "ai_usage") {
		t.Fatalf("run = %#v input %s", run, run.Input)
	}
	native := &model.Agent{ID: uuid.NewString(), WorkspaceID: env.workspaceID, Name: "Manual", RuntimeKind: "native_sdk", TriggerMode: "manual", Status: "idle",
		AllowedTools: json.RawMessage(`[]`), AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`["task"]`)}
	if err := env.db.Create(native).Error; err != nil {
		t.Fatal(err)
	}
	if run, err := env.agents.StartAssignmentRun(context.Background(), env.workspaceID, env.taskID, native.ID, env.userID); err != nil || run != nil {
		t.Fatalf("manual agents must not auto-start: %v, %v", run, err)
	}
}

func TestCommentRoutingResumesPausedExternalRun(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "")
	if err := env.db.Exec(`UPDATE pm_tasks SET assigned_agent_id = ? WHERE id = ?`, record.AgentID, env.taskID).Error; err != nil {
		t.Fatal(err)
	}
	run := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusPaused, func(run *model.AgentRun) {
		run.PauseReason = model.AgentRunPauseReasonHumanInput
	})
	comment, err := env.comments.Create(ctx, model.CreateCommentRequest{EntityType: "task", EntityID: env.taskID, Body: "<p>Use the staging account.</p>"}, env.userID, env.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	env.svc.Wait()
	if len(env.runtime.resumeCalls) != 1 {
		t.Fatalf("resume calls = %d, want 1", len(env.runtime.resumeCalls))
	}
	call := env.runtime.resumeCalls[0]
	if call.runID != *run.ExternalRuntimeID || call.req.Content != "Use the staging account." || call.req.ResumeID != comment.Comment.ID {
		t.Fatalf("resume call = %#v", call)
	}
	// Agent comments never route back to the agent.
	agentID := record.AgentID
	if _, err := env.comments.Create(ctx, model.CreateCommentRequest{EntityType: "task", EntityID: env.taskID, Body: "Working on it", AgentID: &agentID}, env.userID, env.workspaceID); err != nil {
		t.Fatal(err)
	}
	env.svc.Wait()
	if len(env.runtime.resumeCalls) != 1 {
		t.Fatalf("agent comment was routed: %d resume calls", len(env.runtime.resumeCalls))
	}
}

func TestCommentMentionStartsExternalRunWhenIdle(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	record := env.connect(t, "")
	if _, err := env.comments.Create(context.Background(), model.CreateCommentRequest{EntityType: "task", EntityID: env.taskID, Body: "<p>@hermes can you record this?</p>"}, env.userID, env.workspaceID); err != nil {
		t.Fatal(err)
	}
	env.svc.Wait()
	if n := env.count(t, `SELECT COUNT(*) FROM agent_runs WHERE agent_id = ?`, record.AgentID); n != 1 {
		t.Fatalf("runs after mention = %d, want 1", n)
	}
	if len(env.runtime.startRunCalls) != 1 || !strings.Contains(env.runtime.startRunCalls[0].Instructions, "can you record this?") {
		t.Fatalf("start calls = %#v", env.runtime.startRunCalls)
	}
}

var externalA2ATestSchema = []string{
	`CREATE TABLE task_delivery_targets (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, task_id TEXT NOT NULL UNIQUE,
		repository_id TEXT, repo_full_name TEXT, integration_id TEXT, base_branch TEXT, working_branch TEXT,
		delivery_state TEXT NOT NULL DEFAULT 'unconfigured', target_source TEXT NOT NULL DEFAULT 'manual',
		source_epic_id TEXT, active_pr_number INTEGER, active_pr_title TEXT, active_pr_url TEXT, active_pr_status TEXT,
		last_commit_sha TEXT, last_run_id TEXT, last_synced_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`,
	`CREATE TABLE agents (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, is_system BOOLEAN NOT NULL DEFAULT 0, name TEXT NOT NULL,
		icon_key TEXT NOT NULL DEFAULT '', preset_key TEXT, preset_version_key TEXT, source_preset_key TEXT,
		source_preset_version_key TEXT, source_template_id TEXT, source_template_key TEXT NOT NULL DEFAULT '',
		template_key TEXT, template_instance_id TEXT, template_version INTEGER, active_version_id TEXT, role TEXT,
		status TEXT NOT NULL, runtime_kind TEXT NOT NULL, model_tier TEXT NOT NULL DEFAULT '', skills BLOB NOT NULL DEFAULT '[]',
		trigger_mode TEXT NOT NULL, provider TEXT, model TEXT, execution_config BLOB NOT NULL DEFAULT '{}', system_prompt TEXT,
		instruction_template_version TEXT NOT NULL DEFAULT '', planning_notes TEXT, monthly_token_budget INTEGER,
		tokens_used_this_month INTEGER NOT NULL DEFAULT 0, active_task_id TEXT, team_id TEXT,
		allowed_tools BLOB NOT NULL DEFAULT '[]', allowed_commands BLOB NOT NULL DEFAULT '[]', allowed_targets BLOB NOT NULL DEFAULT '[]',
		approval_mode TEXT NOT NULL DEFAULT 'never', max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
		default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous', ai_profile_id TEXT, created_at DATETIME, updated_at DATETIME
	)`,
	`CREATE TABLE agent_team_access (agent_id TEXT NOT NULL, team_id TEXT NOT NULL, created_at DATETIME, PRIMARY KEY (agent_id, team_id))`,
	`CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_id TEXT NOT NULL, task_id TEXT, conversation_id TEXT,
		target_type TEXT NOT NULL DEFAULT 'task', target_id TEXT NOT NULL, runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
		model_tier TEXT NOT NULL DEFAULT '', invocation_mode TEXT NOT NULL DEFAULT 'autonomous', parent_run_id TEXT, dock_chat_id TEXT,
		handoff_state TEXT, approval_state TEXT NOT NULL DEFAULT 'not_required', pause_reason TEXT NOT NULL DEFAULT 'none',
		triggered_by_user_id TEXT, status TEXT NOT NULL DEFAULT 'queued', workflow_id TEXT, workflow_run_id TEXT,
		external_runtime TEXT, external_runtime_id TEXT, task_queue TEXT, runner_pool TEXT, agent_version_id TEXT,
		repository_id TEXT, repo_full_name TEXT, base_branch TEXT, working_branch TEXT, delivery_target_id TEXT,
		execution_stage TEXT, last_heartbeat_at DATETIME, input BLOB NOT NULL DEFAULT '{}', output_summary BLOB NOT NULL DEFAULT '{}',
		cached_input_tokens INTEGER NOT NULL DEFAULT 0, input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0,
		tokens_used INTEGER NOT NULL DEFAULT 0, error_message TEXT, started_at DATETIME, completed_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`,
	`CREATE TABLE agent_run_messages (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL, dock_chat_id TEXT, dock_chat_sequence INTEGER,
		client_message_id TEXT, delivery_status TEXT NOT NULL DEFAULT 'sent', actor_user_id TEXT, runtime_message_id TEXT,
		role TEXT NOT NULL, content TEXT NOT NULL, message_type TEXT NOT NULL DEFAULT 'message', content_blocks BLOB,
		turn_segments BLOB, tool_invocations BLOB, token_usage BLOB, sequence_no INTEGER NOT NULL DEFAULT 0, created_at DATETIME
	)`,
	`CREATE TABLE external_a2a_agents (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_id TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '', card_url TEXT NOT NULL, interface_url TEXT NOT NULL,
		protocol_binding TEXT NOT NULL DEFAULT 'JSONRPC', protocol_version TEXT NOT NULL DEFAULT '', provider_name TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '', skills BLOB NOT NULL DEFAULT '[]', capabilities BLOB NOT NULL DEFAULT '{}',
		agent_card BLOB NOT NULL DEFAULT '{}', encrypted_token TEXT NOT NULL DEFAULT '', token_hint TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active', last_checked_at DATETIME, last_error TEXT NOT NULL DEFAULT '',
		allowed_team_ids BLOB NOT NULL DEFAULT '[]', created_by TEXT NOT NULL, created_at DATETIME, updated_at DATETIME
	)`,
	`CREATE TABLE a2a_task_contexts (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, task_id TEXT NOT NULL, external_a2a_agent_id TEXT NOT NULL,
		context_id TEXT NOT NULL, last_remote_task_id TEXT NOT NULL DEFAULT '', created_at DATETIME, updated_at DATETIME,
		UNIQUE (task_id, external_a2a_agent_id)
	)`,
	`CREATE TABLE a2a_run_upload_tokens (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_run_id TEXT NOT NULL, pm_task_id TEXT NOT NULL,
		external_a2a_agent_id TEXT NOT NULL, token_sha256 TEXT NOT NULL UNIQUE, expires_at DATETIME NOT NULL,
		revoked_at DATETIME, bytes_used INTEGER NOT NULL DEFAULT 0, created_at DATETIME
	)`,
	`CREATE TABLE a2a_projected_items (
		agent_run_id TEXT NOT NULL, item_key TEXT NOT NULL, workspace_id TEXT NOT NULL, created_at DATETIME,
		PRIMARY KEY (agent_run_id, item_key)
	)`,
}

func TestRuntimeHostTargetContextAddsA2AData(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	ctx := context.Background()
	record := env.connect(t, "secret-token-a1b2")
	run := env.seedRun(t, record.AgentID, model.AgentRuntimeKindA2A, model.AgentRunStatusRunning, nil)
	native := env.seedRun(t, record.AgentID, "native_sdk", model.AgentRunStatusRunning, nil)
	host := (&AgentRuntimeHostService{runRepo: env.agents.runRepo, taskRepo: env.agents.taskRepo}).SetExternalA2AService(env.svc)
	resolve := func(runtimeRunID string) map[string]any {
		t.Helper()
		resp, err := host.ResolveTargetContext(ctx, agentruntime.TargetContextRequest{
			RunID: runtimeRunID, AgentID: record.AgentID, Target: agentruntime.TargetRef{Type: "task", ID: env.taskID},
		})
		if err != nil {
			t.Fatalf("resolve target context: %v", err)
		}
		return resp.Data
	}
	if data := resolve(*run.ExternalRuntimeID); data["a2a"] == nil || data["workspace_id"] != env.workspaceID {
		t.Fatalf("a2a run context = %#v", data)
	}
	if data := resolve(*native.ExternalRuntimeID); data["a2a"] != nil {
		t.Fatal("native run context must not include data.a2a")
	}
}

func TestGenericAgentRenameSyncsExternalConnectionName(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	record := env.connect(t, "")
	name := "Hermes Night Shift"
	if _, err := env.agents.UpdateAgent(context.Background(), env.workspaceID, record.AgentID, model.UpdateAgentRequest{Name: &name}, env.userID); err != nil {
		t.Fatal(err)
	}
	updated, err := env.svc.get(context.Background(), env.workspaceID, record.ID)
	if err != nil || updated.Name != name {
		t.Fatalf("connection name = %q, %v", updated.Name, err)
	}
}

func TestExternalA2AShutdownCancelsBackgroundWorkAfterDeadline(t *testing.T) {
	env := newExternalA2ATestEnv(t)
	started := make(chan struct{})
	env.svc.dispatch(context.Background(), func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := env.svc.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown = %v, want deadline exceeded", err)
	}
	ran := false
	env.svc.dispatch(context.Background(), func(context.Context) { ran = true })
	env.svc.Wait()
	if ran {
		t.Fatal("no background work may start after shutdown")
	}
}
