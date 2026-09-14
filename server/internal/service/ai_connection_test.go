package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/agent-runtime-go/chatgptauth"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAIConnectionTest(t *testing.T) (*AIConnectionService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "connections.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&model.AIConnection{}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE TABLE workspace_members (workspace_id TEXT,user_id TEXT,status TEXT)`,
		`INSERT INTO workspace_members VALUES ('workspace','owner','active'),('workspace','teammate','active')`,
		`CREATE TABLE agent_runs (id TEXT,workspace_id TEXT,triggered_by_user_id TEXT,input TEXT,status TEXT,external_runtime TEXT,external_runtime_id TEXT,pause_reason TEXT)`,
	} {
		if err = db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewAgentRuntimeClient("https://runtime.example", "helpin", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAIConnectionService(repository.NewAIConnectionRepository(db), catalog, runtime, AIConnectionConfig{EncryptionKey: strings.Repeat("k", 32), ChatGPTEnabled: true, AppID: "helpin"})
	if err != nil {
		t.Fatal(err)
	}
	return service, db
}

func TestAIConnectionOwnershipEncryptionRotationAndRevocation(t *testing.T) {
	s, db := setupAIConnectionTest(t)
	ctx := context.Background()
	result, err := s.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "openai", APIKey: "private-key"})
	if err != nil {
		t.Fatal(err)
	}
	id := result.Connection.ID
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "private-key") || strings.Contains(string(raw), "encrypted_secret") {
		t.Fatal("public secret exposure")
	}
	stored, err := s.repo.Get(ctx, id)
	if err != nil || strings.Contains(string(stored.EncryptedSecret), "private-key") {
		t.Fatal("unencrypted storage")
	}
	for _, scope := range [][2]string{{"workspace", "teammate"}, {"other", "owner"}} {
		if _, _, err = s.Credential(ctx, scope[0], scope[1], id, false); err == nil {
			t.Fatal("cross-owner credential")
		}
	}
	list, err := s.List(ctx, "workspace", "teammate")
	if err != nil || len(list) != 0 {
		t.Fatal("personal connection visible to teammate")
	}
	if _, err = s.Reconnect(ctx, "workspace", "owner", id, "rotated"); err != nil {
		t.Fatal(err)
	}
	_, credential, err := s.Credential(ctx, "workspace", "owner", id, false)
	if err != nil || credential.APIKey != "rotated" {
		t.Fatal("rotation failed")
	}
	if err = db.Exec(`UPDATE workspace_members SET status='revoked' WHERE user_id='owner'`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Credential(ctx, "workspace", "owner", id, false); err == nil {
		t.Fatal("revoked member used connection")
	}
	if err = s.Disconnect(ctx, "workspace", "owner", id); err != nil {
		t.Fatal(err)
	}
	stored, err = s.repo.Get(ctx, id)
	if err != nil || len(stored.EncryptedSecret) != 0 || stored.Status != "disconnected" {
		t.Fatal("disconnect retained secret")
	}
}

func TestAIConnectionConcurrentRefreshRotatesOnce(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	ctx := context.Background()
	var refreshes atomic.Int32
	jwt := func(account string) string {
		return "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"`+account+`"}}`)) + ".signature"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"access_token": jwt("account"), "refresh_token": "rotated-refresh", "expires_in": 3600})
	}))
	defer server.Close()
	var err error
	s.oauth, err = chatgptauth.NewClient(chatgptauth.Config{Issuer: server.URL, AllowLocalHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	connection := &model.AIConnection{ID: "connection", WorkspaceID: "workspace", UserID: "owner", Provider: "openai_chatgpt", Name: "ChatGPT", Status: "connected", AccountID: "account"}
	old := chatgptauth.Token{AccessToken: "old-token", RefreshToken: "old-refresh", AccountID: "account", ExpiresAt: time.Now().Add(time.Hour)}
	if err = s.seal(connection, aiConnectionSecret{Token: &old}); err != nil {
		t.Fatal(err)
	}
	if err = s.repo.Create(ctx, connection); err != nil {
		t.Fatal(err)
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(old.AccessToken)))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, c, err := s.Credential(ctx, "workspace", "owner", connection.ID, true, fingerprint)
			if err != nil || c == nil || c.AccessToken != jwt("account") {
				t.Error("refresh failed")
			}
		}()
	}
	wg.Wait()
	if refreshes.Load() != 1 {
		t.Fatalf("refreshes: %d", refreshes.Load())
	}
	stored, err := s.repo.Get(ctx, connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := s.open(stored)
	if err != nil || secret.Token.RefreshToken != "rotated-refresh" {
		t.Fatal("refresh token not saved atomically")
	}
}

func TestPersonalConnectionLaunchIsOptionalAndManual(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	ctx := context.Background()
	agents := &AgentService{aiConnections: s}
	agent := &model.Agent{Provider: strPtr("openai"), Model: strPtr("gpt-5.6-luna")}
	plain := createRunParams{agent: agent, input: json.RawMessage(`{}`)}
	selection, credential, billing, err := agents.prepareAIConnectionRun(ctx, &plain)
	if err != nil || selection != nil || credential != nil || billing != agent {
		t.Fatal("default launch changed")
	}
	connection, err := s.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "openai", APIKey: "private-key"})
	if err != nil {
		t.Fatal(err)
	}
	params := createRunParams{agent: agent, workspaceID: "workspace", actorID: strPtr("owner"), input: json.RawMessage(`{}`), modelConnectionID: connection.Connection.ID, modelName: "gpt-5.6-luna"}
	selection, credential, billing, err = agents.prepareAIConnectionRun(ctx, &params)
	if err != nil || selection.Model != "gpt-5.6-luna" || credential.APIKey != "private-key" || billing == agent {
		t.Fatalf("launch: %v", err)
	}
	if strings.Contains(string(params.input), "private-key") {
		t.Fatal("secret stored in run input")
	}
	params.trigger = &model.AgentRunTriggerContext{Source: "scheduled"}
	if _, _, _, err = agents.prepareAIConnectionRun(ctx, &params); err == nil {
		t.Fatal("scheduled personal credential accepted")
	}
	run := &model.AgentRun{Input: params.input, TriggeredByUserID: strPtr("owner")}
	if requireAIConnectionRunOwner(run, "teammate") == nil {
		t.Fatal("teammate can resume personal run")
	}
	_, err = s.RefreshRun(ctx, sdk.ModelCredentialRefreshRequest{AppID: "other", HostRunID: "run", ConnectionID: connection.Connection.ID})
	if err == nil {
		t.Fatal("wrong app refresh accepted")
	}
}
