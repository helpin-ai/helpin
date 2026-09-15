package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type cliTestAuthz struct{ revoked, mfa bool }

func (a *cliTestAuthz) ResolveActor(_ context.Context, ws, user string) (*authorization.Actor, error) {
	if a.revoked || ws != "ws-1" || user != "user-1" {
		return nil, authorization.ErrNotAMember
	}
	return &authorization.Actor{UserID: user, WorkspaceID: ws, Role: "owner"}, nil
}
func (a *cliTestAuthz) Can(*authorization.Actor, authorization.Permission) bool { return !a.revoked }
func (a *cliTestAuthz) CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error) {
	return !a.revoked, nil
}
func (a *cliTestAuthz) WorkspaceMFAPolicy(context.Context, string, string) (model.WorkspaceMFAPolicy, error) {
	return model.WorkspaceMFAPolicy{EnforceTwoFactor: a.mfa}, nil
}

type cliTestWorkspaces struct{}

func (cliTestWorkspaces) List(context.Context, string, string) ([]model.WorkspaceWithRole, error) {
	return nil, nil
}

func setupCLIService(t *testing.T) (*CLIService, *gorm.DB, *cliTestAuthz, *fakeAgentRuntimeSignalClient) {
	t.Helper()
	db := setupCodingDelegationTestDB(t)
	now := time.Now().UTC()
	seedCodingDelegationAgent(t, db, model.AgentPresetCodeBuilder, model.InvocationModeAutonomous, now)
	seedCodingDelegationTaskAndDelivery(t, db, now)
	if err := db.AutoMigrate(&model.CLIConnection{}, &model.CLIToken{}, &model.CLIExecution{}); err != nil {
		t.Fatal(err)
	}
	client := &fakeAgentRuntimeSignalClient{}
	agents := newCodingDelegationService(t, db, client)
	authz := &cliTestAuthz{}
	svc, err := NewCLIService(repository.NewCLIRepository(db), cliTestWorkspaces{}, authz, agents, CLIConfig{Enabled: true, PublicBaseURL: "https://helpin.example", AppBaseURL: "https://helpin.example"})
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, authz, client
}
func cliTestQuery(s *CLIService) (model.CLIAuthorizationQuery, string) {
	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	return model.CLIAuthorizationQuery{ClientID: cliClientID, Resource: s.resource(), RedirectURI: "http://127.0.0.1:34567/callback", ResponseType: "code", Scope: cliScope, State: strings.Repeat("s", 32), CodeChallenge: base64.RawURLEncoding.EncodeToString(sum[:]), CodeChallengeMethod: "S256"}, verifier
}
func cliTestLogin(t *testing.T, s *CLIService) (*model.CLIConnection, *CLITokenResponse) {
	t.Helper()
	q, verifier := cliTestQuery(s)
	redirect, err := s.Authorize(context.Background(), "user-1", model.CLIConsentDecision{Query: q, WorkspaceID: "ws-1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(redirect)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := s.Exchange(context.Background(), "authorization_code", u.Query().Get("code"), cliClientID, s.resource(), q.RedirectURI, verifier)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Authenticate(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	return c, tokens
}
func TestCLIOAuthBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.CLIAuthorizationQuery)
	}{
		{"remote redirect", func(q *model.CLIAuthorizationQuery) { q.RedirectURI = "http://example.com/callback" }},
		{"userinfo", func(q *model.CLIAuthorizationQuery) { q.RedirectURI = "http://attacker@127.0.0.1:123/callback" }},
		{"wrong resource", func(q *model.CLIAuthorizationQuery) { q.Resource = "https://helpin.example/mcp" }},
		{"MCP scope", func(q *model.CLIAuthorizationQuery) { q.Scope = "helpin.agents.run" }},
		{"plain PKCE", func(q *model.CLIAuthorizationQuery) { q.CodeChallengeMethod = "plain" }},
		{"unregistered client", func(q *model.CLIAuthorizationQuery) { q.ClientID = "another-client" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, _ := setupCLIService(t)
			q, _ := cliTestQuery(s)
			tc.mutate(&q)
			if _, err := s.AuthorizationURL(q); !errors.Is(err, ErrCLIInvalid) {
				t.Fatalf("request accepted: %v", err)
			}
		})
	}
}
func TestCLICodeIsSingleUseAndRequiresPKCE(t *testing.T) {
	s, _, _, _ := setupCLIService(t)
	q, v := cliTestQuery(s)
	raw, err := s.Authorize(context.Background(), "user-1", model.CLIConsentDecision{Query: q, WorkspaceID: "ws-1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	code := u.Query().Get("code")
	if _, err = s.Exchange(context.Background(), "authorization_code", code, cliClientID, s.resource(), q.RedirectURI, strings.Repeat("x", 43)); !errors.Is(err, ErrCLIUnauthorized) {
		t.Fatal("bad verifier accepted")
	}
	if _, err = s.Exchange(context.Background(), "authorization_code", code, cliClientID, s.resource(), q.RedirectURI, v); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Exchange(context.Background(), "authorization_code", code, cliClientID, s.resource(), q.RedirectURI, v); !errors.Is(err, ErrCLIUnauthorized) {
		t.Fatal("code replay accepted")
	}
}
func TestCLIRefreshReplayRevokesConsent(t *testing.T) {
	s, _, _, _ := setupCLIService(t)
	_, tokens := cliTestLogin(t, s)
	next, err := s.Exchange(context.Background(), "refresh_token", tokens.RefreshToken, cliClientID, s.resource(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Exchange(context.Background(), "refresh_token", tokens.RefreshToken, cliClientID, s.resource(), "", ""); !errors.Is(err, ErrCLIUnauthorized) {
		t.Fatal("refresh replay accepted")
	}
	if _, err = s.Authenticate(context.Background(), next.AccessToken); !errors.Is(err, ErrCLIUnauthorized) {
		t.Fatal("replay did not revoke family")
	}
}
func TestCLIMembershipAndMFARechecked(t *testing.T) {
	s, _, authz, _ := setupCLIService(t)
	q, _ := cliTestQuery(s)
	authz.mfa = true
	if _, err := s.Authorize(context.Background(), "user-1", model.CLIConsentDecision{Query: q, WorkspaceID: "ws-1"}, false); !errors.Is(err, ErrCLIForbidden) {
		t.Fatal("MFA not enforced")
	}
	_, tokens := cliTestLogin(t, s)
	authz.revoked = true
	if _, err := s.Authenticate(context.Background(), tokens.AccessToken); !errors.Is(err, ErrCLIForbidden) {
		t.Fatal("revoked membership retained access")
	}
}
func TestCLIAdmissionDoesNotDispatchAndIsIdempotent(t *testing.T) {
	s, db, _, client := setupCLIService(t)
	c, _ := cliTestLogin(t, s)
	req := model.CLIAdmissionRequest{RequestID: "request-123", AgentID: "agent-1", Target: "task:task-1", Instructions: "Fix the task locally", ExecutionLocation: "local"}
	first, err := s.Admit(context.Background(), c, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Admit(context.Background(), c, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.RunID != second.RunID || first.Execution.ID != second.Execution.ID {
		t.Fatal("duplicate admission")
	}
	if len(client.startRunCalls) != 0 || len(client.upsertAgents) != 0 {
		t.Fatal("cloud worker dispatched")
	}
	var count int64
	if err = db.Model(&model.AgentRun{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("run count %d %v", count, err)
	}
	run, err := s.agents.runRepo.GetByID(context.Background(), c.WorkspaceID, first.RunID)
	if err != nil || !model.IsLocalAgentRun(run) || run.ExternalRuntimeID != nil || run.DeliveryTargetID != nil {
		t.Fatalf("invalid local run %+v %v", run, err)
	}
	if !strings.Contains(first.Context, "Fix login redirect") {
		t.Fatal("shared target context missing")
	}
	req.Instructions = "different request"
	if _, err = s.Admit(context.Background(), c, req); !errors.Is(err, ErrCLIConflict) {
		t.Fatal("idempotency payload conflict accepted")
	}
	if r := s.agents.reconcileStuckRun(context.Background(), run); r.Status != "queued" {
		t.Fatal("local run reconciled as cloud failure")
	}
}
func TestCLIAdmissionKeepsUsagePreflight(t *testing.T) {
	s, db, _, client := setupCLIService(t)
	s.agents.aiUsageMeter = &AIUsageMeter{}
	c, _ := cliTestLogin(t, s)
	_, err := s.Admit(context.Background(), c, model.CLIAdmissionRequest{RequestID: "usage-123", AgentID: "agent-1", Target: "task:task-1", ExecutionLocation: "local"})
	if err == nil || !strings.Contains(err.Error(), "AI usage lifecycle") {
		t.Fatalf("preflight bypassed: %v", err)
	}
	var count int64
	if err = db.Model(&model.AgentRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 || len(client.startRunCalls) != 0 {
		t.Fatal("denied launch persisted or dispatched")
	}
}
func TestCLILeaseFencesExpiredAndRevokedExecutions(t *testing.T) {
	s, db, _, _ := setupCLIService(t)
	c, _ := cliTestLogin(t, s)
	a, err := s.Admit(context.Background(), c, model.CLIAdmissionRequest{RequestID: "lease-123", AgentID: "agent-1", Target: "task:task-1", ExecutionLocation: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(context.Background(), c, a.RunID, 1, "local-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(context.Background(), c, a.RunID, 1, "local-2"); !errors.Is(err, ErrCLIConflict) {
		t.Fatal("second local run stole grant")
	}
	if err = db.Model(&model.CLIExecution{}).Where("id = ?", a.Execution.ID).Update("lease_expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindExecution(context.Background(), c, a.RunID, 1, "local-1"); !errors.Is(err, ErrCLIConflict) {
		t.Fatal("expired lease accepted")
	}
	e, err := s.RenewExecution(context.Background(), c, a.RunID, 1)
	if err != nil || e.Epoch != 2 || e.LocalRunID != "" {
		t.Fatalf("epoch not fenced: %+v %v", e, err)
	}
	if _, err = s.BindExecution(context.Background(), c, a.RunID, 1, "local-1"); !errors.Is(err, ErrCLIConflict) {
		t.Fatal("stale epoch accepted")
	}
	other := *c
	other.ID = "another-consent"
	if _, err = s.Execution(context.Background(), &other, a.RunID); err == nil {
		t.Fatal("cross-connection access")
	}
	if err = s.RevokeExecution(context.Background(), c, a.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RenewExecution(context.Background(), c, a.RunID, 2); err == nil {
		t.Fatal("revoked execution renewed")
	}
}

// NewCLIIntegrationFixture exposes the seeded test service to external HTTP tests.
func NewCLIIntegrationFixture(t *testing.T, baseURL string) (*CLIService, *gorm.DB, func() int) {
	t.Helper()
	s, db, _, client := setupCLIService(t)
	s.config.PublicBaseURL, s.config.AppBaseURL = baseURL, baseURL
	return s, db, func() int { return len(client.startRunCalls) + len(client.upsertAgents) }
}

func TestCLIRejectsWrongWorkspaceAndPreservesReadOnlyPolicy(t *testing.T) {
	s, _, _, _ := setupCLIService(t)
	c, _ := cliTestLogin(t, s)
	_, err := s.Admit(context.Background(), c, model.CLIAdmissionRequest{RequestID: "wrong-ws-123", AgentID: "agent-1", Target: "workspace:other", ExecutionLocation: "local"})
	if !errors.Is(err, ErrCLIForbidden) {
		t.Fatalf("cross-workspace target: %v", err)
	}
	a := &model.Agent{ID: "readonly", RuntimeKind: "native_sdk", ExecutionConfig: model.JSONBlob(`{"workspace":{"access":"read_only"}}`), AllowedTools: []byte(`["read_files","write_file"]`)}
	snapshot, allowed, err := localAgentPolicy(a, false)
	if err != nil || len(allowed) != 1 || allowed[0] != "read_files" || !strings.Contains(string(snapshot.ExecutionConfig), "read_only") {
		t.Fatalf("policy expanded: %+v %v %v", snapshot, allowed, err)
	}
}

func TestCLIExpiredAccessAndLogout(t *testing.T) {
	s, db, _, _ := setupCLIService(t)
	_, tokens := cliTestLogin(t, s)
	if err := db.Model(&model.CLIToken{}).Where("hash = ?", mcpHash(tokens.AccessToken)).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(context.Background(), tokens.AccessToken); !errors.Is(err, ErrCLIUnauthorized) {
		t.Fatalf("expired access: %v", err)
	}
	if err := s.RevokeToken(context.Background(), tokens.RefreshToken, cliClientID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exchange(context.Background(), "refresh_token", tokens.RefreshToken, cliClientID, s.resource(), "", ""); !errors.Is(err, ErrCLIUnauthorized) {
		t.Fatalf("revoked refresh: %v", err)
	}
}
