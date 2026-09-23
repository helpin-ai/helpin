package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// recordingChatClient records requests and answers with fixed context text.
type recordingChatClient struct {
	mu       sync.Mutex
	requests []llm.ChatRequest
	block    bool
	err      error
}

func (c *recordingChatClient) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	if c.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if c.err != nil {
		return nil, c.err
	}
	return &llm.ChatResponse{Content: "- Acme answers support questions.\nAudience:\n- Support teams", FinishReason: "stop",
		TokensUsed: llm.TokenUsage{InputTokensTotal: 40, OutputTokens: 12}}, nil
}

func (c *recordingChatClient) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

// unconfiguredEnvRouter reports no configured server providers and fails if called.
type unconfiguredEnvRouter struct{ recordingChatClient }

func (*unconfiguredEnvRouter) HasChatProvider(string) bool { return false }

type fakeContextProfiles struct {
	execution *AIProfileExecution
	err       error
	users     []string
}

func (f *fakeContextProfiles) ResolveChatExecution(_ context.Context, _, user string) (*AIProfileExecution, error) {
	f.users = append(f.users, user)
	return f.execution, f.err
}

// countingContextFetcher returns fixed text and counts fetches.
type countingContextFetcher struct {
	mu    sync.Mutex
	text  string
	count int
}

func (f *countingContextFetcher) FetchText(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	return f.text, nil
}

func contextErrorCode(t *testing.T, err error) string {
	t.Helper()
	var contextErr *WorkspaceContextError
	if !errors.As(err, &contextErr) {
		t.Fatalf("error = %v, want WorkspaceContextError", err)
	}
	return contextErr.Code
}

func profileExecution(client llm.Provider) *AIProfileExecution {
	return &AIProfileExecution{
		Selection: &model.AIExecutionSelection{ProfileID: "profile", Source: "workspace",
			Route:  model.AIProfileRoute{ConnectionID: "connection", Model: sdk.RunModel{Provider: "openai", Model: "gpt-5-mini"}},
			Policy: &model.AIExecutionPolicySnapshot{Mode: "community", FundingMode: aiusage.FundingCustomerUnbilled}},
		Client: client,
	}
}

func TestGenerateCompanyProductDescriptionRunsOnWorkspaceProfile(t *testing.T) {
	env := &recordingChatClient{}
	store := &recordingCommunityUsage{}
	completions := NewAICompletionService(env, NewCommunityAIUsage(store), DefaultAICompletionRouteRegistry()).
		SetGovernance(aipolicy.DefaultRegistry(), nil)
	client := &recordingChatClient{}
	profiles := &fakeContextProfiles{execution: profileExecution(client)}
	svc := NewWorkspaceService(nil, nil, nil).
		SetContextGeneratorDependencies(completions, &countingContextFetcher{text: "Acme is a support platform."}).
		SetContextAIProfiles(profiles)

	resp, err := svc.GenerateCompanyProductDescription(context.Background(), "owner", model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme", WebsiteURL: "https://acme.com", WorkspaceID: "workspace",
	})
	if err != nil {
		t.Fatalf("GenerateCompanyProductDescription() error = %v", err)
	}
	if !strings.Contains(resp.CompanyProductContext, "Acme answers support questions") {
		t.Fatalf("context = %q", resp.CompanyProductContext)
	}
	if env.calls() != 0 {
		t.Fatalf("server router called %d times, want the workspace profile only", env.calls())
	}
	if client.calls() != 1 || client.requests[0].Provider != "openai" || client.requests[0].Model != "gpt-5-mini" {
		t.Fatalf("profile requests = %+v, want one openai/gpt-5-mini call", client.requests)
	}
	if len(profiles.users) != 1 || profiles.users[0] != "owner" {
		t.Fatalf("profile resolved for %v, want the requesting user", profiles.users)
	}
	if len(store.entries) != 1 || store.entries[0].Provider != "openai" || store.entries[0].Model != "gpt-5-mini" || store.entries[0].WorkspaceID != "workspace" {
		t.Fatalf("usage = %+v, want one workspace-metered profile completion", store.entries)
	}
}

func TestGenerateCompanyProductDescriptionFallsBackToServerRoutesWithoutProfile(t *testing.T) {
	env := &fakeWorkspaceContextLLM{}
	completions := NewAICompletionService(env, NewCommunityAIUsage(&recordingCommunityUsage{}), DefaultAICompletionRouteRegistry())
	profiles := &fakeContextProfiles{err: ErrWorkspaceAIUnavailable}
	svc := NewWorkspaceService(nil, nil, nil).
		SetContextGeneratorDependencies(completions, &countingContextFetcher{text: "Acme is a support platform."}).
		SetContextAIProfiles(profiles)

	if _, err := svc.GenerateCompanyProductDescription(context.Background(), "owner", model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme", WebsiteURL: "https://acme.com", WorkspaceID: "workspace",
	}); err != nil {
		t.Fatalf("GenerateCompanyProductDescription() error = %v", err)
	}
	if len(profiles.users) != 1 {
		t.Fatalf("workspace profile consulted %d times, want once", len(profiles.users))
	}
	if env.lastRequest.Provider != "openrouter" {
		t.Fatalf("server route = %q, want openrouter", env.lastRequest.Provider)
	}
}

func TestGenerateCompanyProductDescriptionAIUnavailableSkipsWebsite(t *testing.T) {
	env := &unconfiguredEnvRouter{}
	completions := NewAICompletionService(env, NewCommunityAIUsage(&recordingCommunityUsage{}), DefaultAICompletionRouteRegistry())
	tests := []struct {
		name        string
		workspaceID string
		profiles    workspaceContextProfiles
		llm         workspaceContextLLM
	}{
		{name: "no workspace and no server key", llm: completions},
		{name: "workspace without usable profile", workspaceID: "workspace", profiles: &fakeContextProfiles{err: ErrWorkspaceAIUnavailable}, llm: completions},
		{name: "no completion service", workspaceID: "workspace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &countingContextFetcher{text: "Acme"}
			svc := NewWorkspaceService(nil, nil, nil).SetContextGeneratorDependencies(tt.llm, fetcher)
			if tt.profiles != nil {
				svc.SetContextAIProfiles(tt.profiles)
			}
			_, err := svc.GenerateCompanyProductDescription(context.Background(), "owner", model.GenerateWorkspaceContextDescriptionRequest{
				WorkspaceName: "Acme", WebsiteURL: "https://acme.com", WorkspaceID: tt.workspaceID,
			})
			if code := contextErrorCode(t, err); code != WorkspaceContextErrAIUnavailable {
				t.Fatalf("code = %q, want ai_unavailable", code)
			}
			if fetcher.count != 0 {
				t.Fatalf("website fetched %d times before AI availability was known", fetcher.count)
			}
		})
	}
	if env.calls() != 0 {
		t.Fatalf("unconfigured router called %d times", env.calls())
	}
}

func TestGenerateCompanyProductDescriptionWebsiteUnreadable(t *testing.T) {
	client := &recordingChatClient{}
	svc := NewWorkspaceService(nil, nil, nil).
		SetContextGeneratorDependencies(&fakeWorkspaceContextLLM{}, &countingContextFetcher{text: "   "}).
		SetContextAIProfiles(&fakeContextProfiles{execution: profileExecution(client)})
	_, err := svc.GenerateCompanyProductDescription(context.Background(), "owner", model.GenerateWorkspaceContextDescriptionRequest{
		WorkspaceName: "Acme", WebsiteURL: "https://acme.com",
	})
	if code := contextErrorCode(t, err); code != WorkspaceContextErrWebsiteUnreadable {
		t.Fatalf("code = %q, want website_unreadable", code)
	}
}

func TestGenerateCompanyProductDescriptionTimeoutAndFailureCodes(t *testing.T) {
	tests := []struct {
		name   string
		client *recordingChatClient
		want   string
	}{
		{name: "model exceeds deadline", client: &recordingChatClient{block: true}, want: WorkspaceContextErrTimeout},
		{name: "provider rejects", client: &recordingChatClient{err: errors.New("provider status 500: secret detail")}, want: WorkspaceContextErrGenerationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completions := NewAICompletionService(&recordingChatClient{}, NewCommunityAIUsage(&recordingCommunityUsage{}), DefaultAICompletionRouteRegistry())
			svc := NewWorkspaceService(nil, nil, nil).
				SetContextGeneratorDependencies(completions, &countingContextFetcher{text: "Acme is a support platform."}).
				SetContextAIProfiles(&fakeContextProfiles{execution: profileExecution(tt.client)})
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			_, err := svc.GenerateCompanyProductDescription(ctx, "owner", model.GenerateWorkspaceContextDescriptionRequest{
				WorkspaceName: "Acme", WebsiteURL: "https://acme.com", WorkspaceID: "workspace",
			})
			if code := contextErrorCode(t, err); code != tt.want {
				t.Fatalf("code = %q, want %q (err %v)", code, tt.want, err)
			}
		})
	}
}

func TestGenerateCompanyProductDescriptionRejectsNonPublicWebsite(t *testing.T) {
	for _, raw := range []string{"", "ftp://acme.com", "http://localhost:8080", "http://127.0.0.1", "http://10.1.2.3",
		"http://169.254.169.254/latest/meta-data", "http://[::1]/", "http://100.64.0.1", "http://printer.local"} {
		t.Run(raw, func(t *testing.T) {
			fetcher := &countingContextFetcher{text: "Acme"}
			svc := NewWorkspaceService(nil, nil, nil).SetContextGeneratorDependencies(&fakeWorkspaceContextLLM{}, fetcher)
			_, err := svc.GenerateCompanyProductDescription(context.Background(), "owner", model.GenerateWorkspaceContextDescriptionRequest{WebsiteURL: raw})
			var validation *WorkspaceContextValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error = %v, want validation error", err)
			}
			if fetcher.count != 0 {
				t.Fatal("rejected website was fetched")
			}
		})
	}
}

type staticContextResolver map[string][]netip.Addr

func (r staticContextResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return r[host], nil
}

func TestWorkspaceContextHTTPClientBlocksPrivateAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<p>internal secret</p>"))
	}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resolver := staticContextResolver{
		"intranet.example": {netip.MustParseAddr("10.0.0.5")},
		"metadata.example": {netip.MustParseAddr("169.254.169.254")},
		"loopback.example": {netip.MustParseAddr("127.0.0.1")},
	}
	fetcher := HTTPWorkspaceContextFetcher{Client: newWorkspaceContextHTTPClient(resolver)}
	for _, target := range []string{
		server.URL,
		"http://intranet.example/",
		"http://metadata.example/latest/meta-data",
		"http://loopback.example:" + parsed.Port() + "/",
	} {
		text, err := fetcher.FetchText(context.Background(), target)
		if err == nil || strings.Contains(text, "internal secret") {
			t.Fatalf("FetchText(%q) = %q, %v; want blocked", target, text, err)
		}
	}
	client := newWorkspaceContextHTTPClient(resolver)
	redirect := httptest.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data", nil)
	if err := client.CheckRedirect(redirect, []*http.Request{httptest.NewRequest(http.MethodGet, "https://acme.com", nil)}); err == nil {
		t.Fatal("redirect to the metadata address was allowed")
	}
	redirect = httptest.NewRequest(http.MethodGet, "http://intranet.example/", nil)
	if err := client.CheckRedirect(redirect, []*http.Request{httptest.NewRequest(http.MethodGet, "https://acme.com", nil)}); err == nil {
		t.Fatal("redirect to a host resolving to a private address was allowed")
	}
}

func TestFetchWorkspaceContextPagesIsConcurrentAndOrdered(t *testing.T) {
	release := make(chan struct{})
	var started sync.WaitGroup
	started.Add(len(workspaceContextCandidates("https://acme.com")))
	fetcher := fetcherFunc(func(ctx context.Context, rawURL string) (string, error) {
		started.Done()
		<-release
		return "text for " + rawURL, nil
	})
	done := make(chan string, 1)
	go func() { done <- fetchWorkspaceContextPages(context.Background(), fetcher, "https://acme.com") }()
	started.Wait() // every page request is in flight at once
	close(release)
	text := <-done
	if strings.Index(text, "text for https://acme.com\n") > strings.Index(text, "text for https://acme.com/about") {
		t.Fatalf("pages out of order: %q", text)
	}
}

type fetcherFunc func(ctx context.Context, rawURL string) (string, error)

func (f fetcherFunc) FetchText(ctx context.Context, rawURL string) (string, error) {
	return f(ctx, rawURL)
}

func TestResolveChatExecutionUsesWorkspaceDefaultProfile(t *testing.T) {
	s, primary, _ := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefault(ctx, "workspace", "owner", p.ID); err != nil {
		t.Fatal(err)
	}
	var gotRoute sdk.RunModel
	var gotKey string
	client := &recordingChatClient{}
	s.CheckRuntimeReadiness().SetChatClientFactory(func(route sdk.RunModel, credential sdk.ModelCredential) (llm.Provider, error) {
		gotRoute, gotKey = route, credential.APIKey
		return client, nil
	})
	execution, err := s.ResolveChatExecution(ctx, "workspace", "teammate")
	if err != nil {
		t.Fatalf("ResolveChatExecution() error = %v (direct execution must not need Agent Runtime)", err)
	}
	if execution.Client != client || gotRoute.Provider != "openai" || gotRoute.Model != "custom-unpriced-model" || gotKey != "Primary-key" {
		t.Fatalf("execution route=%+v key=%q", gotRoute, gotKey)
	}
	if execution.Selection.Policy == nil || execution.Selection.Policy.FundingMode != aiusage.FundingCustomerUnbilled {
		t.Fatalf("policy = %+v, want Community unbilled", execution.Selection.Policy)
	}
	if _, err := s.ResolveChatExecution(ctx, "workspace", "stranger"); !errors.Is(err, ErrWorkspaceAIUnavailable) {
		t.Fatalf("non-member resolution error = %v, want ErrWorkspaceAIUnavailable", err)
	}
}

func TestDefaultAIChatClientSupportsDirectProviders(t *testing.T) {
	credential := sdk.ModelCredential{Type: "api_key", APIKey: "key"}
	for _, provider := range []string{"openai", "openrouter", "anthropic"} {
		if _, err := defaultAIChatClient(sdk.RunModel{Provider: provider, Model: "m"}, credential); err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
	}
	endpoint := &sdk.ModelEndpoint{ID: "local", BaseURL: "http://ollama:11434/v1", AuthMode: "none"}
	if _, err := defaultAIChatClient(sdk.RunModel{Provider: "openai_compatible", Model: "llama", Endpoint: endpoint}, sdk.ModelCredential{Type: "none"}); err != nil {
		t.Fatalf("openai_compatible without auth: %v", err)
	}
	if _, err := defaultAIChatClient(sdk.RunModel{Provider: "openai_chatgpt", Model: "gpt"}, credential); err == nil {
		t.Fatal("ChatGPT subscription must not run as a direct completion")
	}
}

func TestEnvironmentSourcedConnectionIsUnableToVerifyNotNeedsSetup(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name        string
		credentials map[string]string
		want        string
	}{
		{name: "server OpenRouter key", credentials: map[string]string{"openrouter": "env-key"}, want: model.CapabilityUnableToVerify},
		{name: "no server key", credentials: nil, want: model.CapabilityNeedsSetup},
	} {
		t.Run(tt.name, func(t *testing.T) {
			profiles, connections, _ := customerStandardProfilesFixture(t, tt.credentials)
			// Community runs this through the workspace lifecycle on creation.
			if err := profiles.WorkspaceCreated(ctx, "workspace"); err != nil {
				t.Fatal(err)
			}
			capabilities := newCapabilityService(CapabilityConfig{AIConnectionsEnabled: true}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, connections.repo)
			response, err := capabilities.Workspace(ctx, "workspace")
			if err != nil {
				t.Fatal(err)
			}
			got := capabilityByKey(t, response, model.CapabilityKeyAIChat)
			if got.Status != tt.want {
				t.Fatalf("ai_chat = %+v, want %s", got, tt.want)
			}
			if tt.want == model.CapabilityUnableToVerify && (got.Action == nil || got.Action.Kind != model.CapabilityActionTestAIConnection || got.Action.ConnectionID == "") {
				t.Fatalf("action = %+v, want a test action for the connection", got.Action)
			}
		})
	}
}
