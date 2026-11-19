package service

import (
	"context"
	"encoding/json"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/agent-runtime-go/chatgptauth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Exercise actual profile admission, including sibling profiles on one login.
func TestSubscriptionModelGrant(t *testing.T) {
	s, _, _, db := setupAIProfileTestDB(t)
	ctx := context.Background()
	if err := db.Exec("CREATE TABLE ai_model_grant_revocations(grant_id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	c := &model.AIConnection{ID: "subscription", WorkspaceID: "workspace", UserID: strPtr("owner"), Scope: "personal", Provider: "openai_chatgpt", Funding: "customer", Status: "connected", Name: "My plan", AccountID: "account"}
	token := chatgptauth.Token{AccessToken: "fixture-token", AccountID: "account", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.connections.seal(c, aiConnectionSecret{Token: &token}); err != nil {
		t.Fatal(err)
	}
	if err := s.connections.repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	create := func(name string) *model.AIProfile {
		t.Helper()
		p, err := s.EnableModel(ctx, "workspace", "owner", model.EnableAIModelRequest{ConnectionID: c.ID, Model: name})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	shared, private := create("shared-model"), create("private-model")
	if _, err := s.SetAgentSharing(ctx, "workspace", "teammate", shared.ID, shared.Revision, true); err == nil {
		t.Fatal("teammate shared owner's model")
	}
	shared, err := s.SetAgentSharing(ctx, "workspace", "owner", shared.ID, shared.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users(id TEXT PRIMARY KEY, full_name TEXT); INSERT INTO users VALUES ('owner','Waqar')").Error; err != nil {
		t.Fatal(err)
	}
	listed, err := s.List(ctx, "workspace", "teammate")
	if err != nil || len(listed) != 1 || listed[0].ID != shared.ID || listed[0].SubscriptionName != "My plan · Waqar" || !listed[0].PrimaryPolicy.Allowed {
		t.Fatalf("shared listing: %+v, %v", listed, err)
	}
	var accepted *model.AIExecutionSelection
	for _, user := range []string{"teammate", ""} {
		selected, cred, err := s.Resolve(ctx, "workspace", user, AIProfileSelectionRequest{AgentProfileID: shared.ID, Unattended: user == ""})
		if err != nil || cred.ConnectionID != c.ID || selected.Route.Model.Model != "shared-model" {
			t.Fatalf("shared model resolution: %v", err)
		}
		accepted = selected
	}
	if _, _, err := s.Resolve(ctx, "workspace", "teammate", AIProfileSelectionRequest{ProfileID: private.ID}); err == nil {
		t.Fatal("private sibling was accessible")
	}
	if _, _, err := s.connections.Credential(ctx, "workspace", "teammate", c.ID, false); err == nil {
		t.Fatal("teammate has raw connection access")
	}
	if _, _, err := s.Resolve(ctx, "workspace", "teammate", AIProfileSelectionRequest{ProfileID: shared.ID, Direct: true}); err == nil {
		t.Fatal("agent grant authorized direct credential use")
	}
	svc := (&AgentService{}).SetAIProfileService(s)
	if err := svc.validateSharedAIProfile(ctx, "workspace", &shared.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.validateSharedAIProfile(ctx, "workspace", &private.ID); err == nil {
		t.Fatal("private sibling accepted as agent default")
	}
	forged := *accepted
	forged.Route.Model.Model = "private-model"
	if _, err := s.Restore(ctx, "workspace", "teammate", &forged); err == nil {
		t.Fatal("changed model accepted")
	}
	if _, err := s.Restore(ctx, "other", "teammate", accepted); err == nil {
		t.Fatal("cross-workspace restore accepted")
	}
	revoked := make(chan bool, 1)
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/runs/runtime-test/model-credential" {
			http.NotFound(w, r)
			return
		}
		revoked <- true
		w.WriteHeader(204)
	}))
	defer runtime.Close()
	s.connections.runtime, err = NewAgentRuntimeClient(runtime.URL, "helpin", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(model.AgentRunInputPayload{AISelection: accepted, ModelConnectionID: c.ID, ModelName: accepted.Route.Model.Model, ModelProvider: c.Provider})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO agent_runs (id,workspace_id,triggered_by_user_id,input,status,external_runtime,external_runtime_id) VALUES (?,?,?,?,?,?,?)", "run", "workspace", "teammate", []byte(payload), "running", agentRuntimeName, "runtime-test").Error; err != nil {
		t.Fatal(err)
	}
	refresh := sdk.ModelCredentialRefreshRequest{AppID: "helpin", RunID: "runtime-test", HostRunID: "run", AccountID: c.AccountID, ConnectionID: c.ID, Provider: c.Provider}
	if _, err = s.connections.RefreshRun(ctx, refresh); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	wrongRun := refresh
	wrongRun.RunID = "other-run"
	if _, err = s.connections.RefreshRun(ctx, wrongRun); err == nil {
		t.Fatal("unbound runtime refreshed")
	}
	shared, err = s.SetAgentSharing(ctx, "workspace", "owner", shared.ID, shared.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-revoked:
	default:
		t.Fatal("active runtime credential was not revoked")
	}
	if _, err = s.SetAgentSharing(ctx, "workspace", "owner", shared.ID, shared.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.connections.RefreshRun(ctx, refresh); err == nil {
		t.Fatal("old grant refreshed after re-sharing")
	}
	if _, err = s.Restore(ctx, "workspace", "teammate", accepted); err == nil {
		t.Fatal("revoked shared model restored")
	}
	if err := db.Exec("UPDATE workspace_members SET status='revoked' WHERE user_id='owner'").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(ctx, "workspace", "", AIProfileSelectionRequest{AgentProfileID: shared.ID, Unattended: true}); err == nil {
		t.Fatal("departed owner subscription used for a scheduled run")
	}
}

func TestSubscriptionRevocationQueueTargetsOnlyRevokedGrant(t *testing.T) {
	s, db := setupAIConnectionTest(t)
	ctx := context.Background()
	if err := db.Exec("CREATE TABLE ai_model_grant_revocations(grant_id TEXT PRIMARY KEY,workspace_id TEXT,profile_id TEXT,connection_id TEXT,retry_at datetime); INSERT INTO ai_model_grant_revocations VALUES ('old','workspace','profile','connection','2000-01-01')").Error; err != nil {
		t.Fatal(err)
	}
	for _, epoch := range []string{"old", "new"} {
		input, err := json.Marshal(model.AgentRunInputPayload{AISelection: &model.AIExecutionSelection{ProfileID: "profile", AgentGrantID: strPtr(epoch)}, ModelConnectionID: "connection"})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO agent_runs (id,workspace_id,input,status,external_runtime,external_runtime_id) VALUES (?,?,?,?,?,?)", epoch, "workspace", []byte(input), "running", agentRuntimeName, epoch).Error; err != nil {
			t.Fatal(err)
		}
	}
	var calls atomic.Int32
	rt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/runs/old/model-credential" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(204)
	}))
	defer rt.Close()
	var err error
	s.runtime, err = NewAgentRuntimeClient(rt.URL, "helpin", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.drainModelGrantRevocations(ctx)
	var pending int64
	if err := db.Table("ai_model_grant_revocations").Count(&pending).Error; err != nil || pending != 1 {
		t.Fatalf("runtime failure lost the job: %d %v", pending, err)
	}
	if err := db.Exec("UPDATE ai_model_grant_revocations SET retry_at='2000-01-01'").Error; err != nil {
		t.Fatal(err)
	}
	s.drainModelGrantRevocations(ctx)
	if err := db.Table("ai_model_grant_revocations").Count(&pending).Error; err != nil || pending != 0 || calls.Load() != 2 {
		t.Fatalf("retry: %d pending, %d calls, %v", pending, calls.Load(), err)
	}
}
