package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type fakeTestProvider struct {
	requests []llm.ChatRequest
	err      error
}

func (p *fakeTestProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.requests = append(p.requests, req)
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: "OK"}, nil
}

func setupAIConnectionVerifyTest(t *testing.T, fake *fakeTestProvider) (*AIConnectionService, string, *gorm.DB) {
	t.Helper()
	s, db := setupAIConnectionTest(t)
	if err := db.AutoMigrate(&model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	s.SetTestProviderFactory(func(string, sdk.ModelCredential) (llm.Provider, error) { return fake, nil })
	login, err := s.Create(context.Background(), "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "openai", APIKey: "sk-private-key"})
	if err != nil {
		t.Fatal(err)
	}
	return s, login.Connection.ID, db
}

func TestAIConnectionTestSucceedsWithSmallestCatalogModel(t *testing.T) {
	fake := &fakeTestProvider{}
	s, id, _ := setupAIConnectionVerifyTest(t, fake)
	ctx := context.Background()
	result, err := s.TestConnection(ctx, "workspace", "owner", id, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Model != "gpt-5.6-luna" || result.Error != "" || result.LatencyMS < 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(fake.requests) != 1 || fake.requests[0].MaxTokens != aiConnectionTestMaxTokens || fake.requests[0].Provider != "openai" {
		t.Fatalf("unexpected provider request: %+v", fake.requests)
	}
	stored, err := s.repo.Get(ctx, id)
	if err != nil || stored.LastVerifiedAt == nil || stored.LastVerificationError != nil {
		t.Fatalf("success was not recorded: %v", err)
	}
}

func TestAIConnectionTestPrefersDefaultProfileModel(t *testing.T) {
	fake := &fakeTestProvider{}
	s, id, db := setupAIConnectionVerifyTest(t, fake)
	ctx := context.Background()
	profile := &model.AIProfile{ID: "profile", WorkspaceID: "workspace", Scope: "workspace", Name: "Default", Revision: 1,
		Primary: model.AIProfileRoute{ConnectionID: id, Model: sdk.RunModel{Provider: "openai", Model: "gpt-5-mini"}}}
	if err := db.Create(profile).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AIWorkspaceSettings{WorkspaceID: "workspace", DefaultProfileID: &profile.ID}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := s.TestConnection(ctx, "workspace", "owner", id, "")
	if err != nil || !result.OK || result.Model != "gpt-5-mini" {
		t.Fatalf("default profile model not used: %+v, %v", result, err)
	}
	result, err = s.TestConnection(ctx, "workspace", "owner", id, "gpt-5.5")
	if err != nil || result.Model != "gpt-5.5" {
		t.Fatalf("requested model not used: %+v, %v", result, err)
	}
}

func TestAIConnectionTestSanitizesAndRecordsFailure(t *testing.T) {
	fake := &fakeTestProvider{err: &llm.ProviderError{Provider: "openai", StatusCode: 401, Message: `{"error":"Incorrect API key provided: sk-private-key"}`}}
	s, id, _ := setupAIConnectionVerifyTest(t, fake)
	ctx := context.Background()
	result, err := s.TestConnection(ctx, "workspace", "owner", id, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || strings.Contains(string(raw), "sk-private-key") || !strings.Contains(result.Error, "rejected the API key") {
		t.Fatalf("unsafe or wrong failure: %s", raw)
	}
	stored, err := s.repo.Get(ctx, id)
	if err != nil || stored.LastVerifiedAt == nil || derefString(stored.LastVerificationError) != result.Error {
		t.Fatalf("failure was not recorded: %v", err)
	}
}

func TestAIConnectionTestRequiresManagementAccess(t *testing.T) {
	fake := &fakeTestProvider{}
	s, id, _ := setupAIConnectionVerifyTest(t, fake)
	ctx := context.Background()
	if _, err := s.TestConnection(ctx, "workspace", "teammate", id, ""); !errors.Is(err, ErrAIConnection) {
		t.Fatalf("teammate tested a personal connection: %v", err)
	}
	managed := &model.AIConnection{ID: "managed", WorkspaceID: "workspace", Scope: "workspace", Funding: "managed", Provider: "openai", Name: "openai (managed)", Status: "connected"}
	if err := s.seal(managed, aiConnectionSecret{APIKey: "platform-key"}); err != nil {
		t.Fatal(err)
	}
	if err := s.repo.Create(ctx, managed); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TestConnection(ctx, "workspace", "owner", managed.ID, ""); !errors.Is(err, ErrAIConnection) {
		t.Fatalf("managed connection tested: %v", err)
	}
	if len(fake.requests) != 0 {
		t.Fatal("unauthorized test reached the provider")
	}
}

func TestAIConnectionTestReportsDisconnectedConnection(t *testing.T) {
	fake := &fakeTestProvider{}
	s, id, _ := setupAIConnectionVerifyTest(t, fake)
	ctx := context.Background()
	if err := s.Disconnect(ctx, "workspace", "owner", id); err != nil {
		t.Fatal(err)
	}
	result, err := s.TestConnection(ctx, "workspace", "owner", id, "")
	if err != nil || result.OK || !strings.Contains(result.Error, "Reconnect") || len(fake.requests) != 0 {
		t.Fatalf("disconnected connection result: %+v, %v", result, err)
	}
}

func TestAIConnectionTestAgainstOpenAICompatibleServer(t *testing.T) {
	var sawKey, sawModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var req struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Error(err)
		}
		sawModel = req.Model
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided: sk-private-key"}}`)
	}))
	defer server.Close()
	s, db := setupAIConnectionTest(t)
	if err := db.AutoMigrate(&model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	s.SetTestProviderFactory(func(_ string, credential sdk.ModelCredential) (llm.Provider, error) {
		return llm.NewOpenAIProvider(credential.APIKey, server.URL, ""), nil
	})
	ctx := context.Background()
	login, err := s.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Router", Provider: "openrouter", APIKey: "sk-private-key"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.TestConnection(ctx, "workspace", "owner", login.Connection.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if sawKey != "Bearer sk-private-key" || sawModel != "openai/gpt-5.6-luna" {
		t.Fatalf("provider request: key sent=%t model=%q", sawKey != "", sawModel)
	}
	if result.OK || strings.Contains(result.Error, "sk-private-key") || !strings.Contains(result.Error, "HTTP 401") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSanitizeAIConnectionTestError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "claude status", err: errors.New(`claude API error (status 401): {"message":"invalid x-api-key sk-ant-secret"}`), want: "The provider rejected the API key (HTTP 401)."},
		{name: "credits", err: &llm.ProviderError{StatusCode: 402, Err: llm.ErrInsufficientCredits}, want: "The provider account has insufficient credits or quota (HTTP 402)."},
		{name: "missing model", err: &llm.ProviderError{StatusCode: 404}, want: "The test model is not available to this API key (HTTP 404)."},
		{name: "rate limit", err: &llm.ProviderError{StatusCode: 429}, want: "The provider rate-limited the test. Try again shortly (HTTP 429)."},
		{name: "outage", err: &llm.ProviderError{StatusCode: 503}, want: "The provider is temporarily unavailable (HTTP 503)."},
		{name: "bad request", err: &llm.ProviderError{StatusCode: 400, Message: "echo sk-secret"}, want: "The provider rejected the test request (HTTP 400)."},
		{name: "timeout", err: fmt.Errorf("send: %w", context.DeadlineExceeded), want: "The provider did not respond in time."},
		{name: "other", err: errors.New("parse response: sk-secret"), want: "The provider test failed."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeAIConnectionTestError(tt.err); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
