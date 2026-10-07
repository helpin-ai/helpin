package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/agent-runtime-go/chatgptauth"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type modelListTransport func(*http.Request) (*http.Response, error)

func (f modelListTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConnectionModelsRefreshCacheAndAuthorization(t *testing.T) {
	profiles, primary, _ := setupAIProfileTest(t)
	s := profiles.connections
	calls := 0
	fail := false
	s.modelHTTP = &http.Client{Transport: modelListTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://api.openai.com/v1/models" || r.Header.Get("Authorization") != "Bearer Primary-key" {
			t.Fatalf("unexpected discovery request: %s", r.URL)
		}
		if fail {
			return nil, errors.New("provider failed with secret")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-new-model"},{"id":"text-embedding-3-small"},{"id":"gpt-image-1"}]}`)), Header: make(http.Header)}, nil
	})}
	ctx := context.Background()
	first, err := s.ConnectionModels(ctx, "workspace", "owner", primary.ConnectionID, false)
	if err != nil || len(first.Models) != 1 || first.Models[0].ID != "gpt-new-model" {
		t.Fatalf("discovery: %+v %v", first, err)
	}
	if _, err := s.ConnectionModels(ctx, "workspace", "owner", primary.ConnectionID, false); err != nil || calls != 1 {
		t.Fatalf("cache missed: %d %v", calls, err)
	}
	fail = true
	stale, err := s.ConnectionModels(ctx, "workspace", "owner", primary.ConnectionID, true)
	if err != nil || !stale.Stale || len(stale.Models) != 1 || strings.Contains(stale.Warning, "secret") {
		t.Fatalf("lost cache or leaked error: %+v %v", stale, err)
	}
	if _, err := s.ConnectionModels(ctx, "workspace", "outsider", primary.ConnectionID, false); err == nil {
		t.Fatal("nonmember read discovery")
	}
	if _, err := s.ConnectionModels(ctx, "workspace", "teammate", primary.ConnectionID, true); err == nil {
		t.Fatal("member refreshed shared credentials")
	}
	personal, err := s.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Mine", Provider: "openai", APIKey: "private"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConnectionModels(ctx, "workspace", "teammate", personal.Connection.ID, false); err == nil {
		t.Fatal("member read personal models")
	}
}

func TestDiscoverModelsFiltersAndPaginates(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: modelListTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"data":[{"id":"claude-new","display_name":"Claude New"}],"has_more":true,"last_id":"claude-new"}`
		if calls == 2 {
			if r.URL.Query().Get("after_id") != "claude-new" {
				t.Fatal("missing cursor")
			}
			body = `{"data":[{"id":"claude-other"}],"has_more":false}`
		}
		if r.Header.Get("x-api-key") != "key" {
			t.Fatal("missing auth")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	result, err := discoverProviderModels(context.Background(), client, "anthropic", "key")
	if err != nil || len(result) != 2 || calls != 2 {
		t.Fatalf("pagination: %+v %v", result, err)
	}
	client.Transport = modelListTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"vendor/agent","name":"Agent","supported_parameters":["tools"],"architecture":{"output_modalities":["text"]}},{"id":"vendor/image","supported_parameters":[],"architecture":{"output_modalities":["image"]}}]}`)), Header: make(http.Header)}, nil
	})
	result, err = discoverProviderModels(context.Background(), client, "openrouter", "key")
	if err != nil || len(result) != 1 || result[0].ID != "vendor/agent" {
		t.Fatalf("capability filter: %+v %v", result, err)
	}
}

func TestConnectionModelsExpiredCacheRefreshes(t *testing.T) {
	profiles, primary, _ := setupAIProfileTest(t)
	s := profiles.connections
	old := time.Now().Add(-24 * time.Hour)
	if err := s.repo.SaveDiscoveredModels(context.Background(), primary.ConnectionID, []model.DiscoveredAIModel{{ID: "old", Name: "Old"}}, old); err != nil {
		t.Fatal(err)
	}
	s.modelHTTP = &http.Client{Transport: modelListTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-new"}]}`)), Header: make(http.Header)}, nil
	})}
	result, err := s.ConnectionModels(context.Background(), "workspace", "owner", primary.ConnectionID, false)
	if err != nil || len(result.Models) != 1 || result.Models[0].ID != "gpt-new" {
		t.Fatalf("expired cache: %+v %v", result, err)
	}
}

func TestChatGPTConnectionModelsRefreshAccountCatalog(t *testing.T) {
	profiles, _, _ := setupAIProfileTest(t)
	s := profiles.connections
	s.cfg.ChatGPTEnabled = true
	ctx := context.Background()
	c := &model.AIConnection{ID: "chatgpt-models", WorkspaceID: "workspace", Scope: "personal", UserID: strPtr("owner"), Provider: "openai_chatgpt", Name: "ChatGPT", Funding: "customer", Status: "connected"}
	if err := s.seal(c, aiConnectionSecret{Token: &chatgptauth.Token{AccessToken: "test-oauth-access", AccountID: "test-account", ExpiresAt: time.Now().Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := s.repo.SaveDiscoveredModels(ctx, c.ID, []model.DiscoveredAIModel{{ID: "old-choice", Name: "Old choice"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	calls := 0
	body := `{"models":[{"slug":"gpt-6.1-sol","display_name":"GPT-6.1 Sol","visibility":"list"},{"slug":"a-model","display_name":"Another model","visibility":"list"},{"slug":"hidden-choice","visibility":"hidden"},{"slug":"gpt-6.1-sol","visibility":"list"}]}`
	s.modelHTTP = &http.Client{Transport: modelListTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://api.openai.com/v1/models" || r.Header.Get("Authorization") != "Bearer test-oauth-access" {
			t.Fatal("incorrect ChatGPT discovery endpoint or credential")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	cached, err := s.ConnectionModels(ctx, "workspace", "owner", c.ID, false)
	if err != nil || len(cached.Models) != 1 || cached.Models[0].ID != "old-choice" || calls != 0 {
		t.Fatalf("lost fresh account cache: %+v %v", cached, err)
	}
	fresh, err := s.ConnectionModels(ctx, "workspace", "owner", c.ID, true)
	if err != nil || len(fresh.Models) != 2 || fresh.Models[0].ID != "gpt-6.1-sol" || fresh.Models[0].Name != "GPT-6.1 Sol" || fresh.Models[1].ID != "a-model" || calls != 1 || fresh.Source != "provider" {
		t.Fatalf("account discovery: %+v %v", fresh, err)
	}
	if _, err := s.ConnectionModels(ctx, "workspace", "teammate", c.ID, true); err == nil || calls != 1 {
		t.Fatal("another member could refresh a personal catalog")
	}
	body = `{"error":"test-secret-must-not-be-returned"}`
	stale, err := s.ConnectionModels(ctx, "workspace", "owner", c.ID, true)
	if err != nil || len(stale.Models) != 2 || !stale.Stale || strings.Contains(stale.Warning, "secret") || stale.Warning == "" {
		t.Fatalf("refresh failure lost the catalog or exposed upstream data: %+v %v", stale, err)
	}
	body = `{"models":[]}`
	empty, err := s.ConnectionModels(ctx, "workspace", "owner", c.ID, true)
	if err != nil || empty.Source != "provider" || len(empty.Models) != 0 || empty.Stale {
		t.Fatalf("empty account catalog not respected: %+v %v", empty, err)
	}
}

func TestConnectionModelsTracksNewDiscoveriesAcrossRefreshes(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		t.Run(provider, func(t *testing.T) {
			profiles, _, _ := setupAIProfileTest(t)
			s := profiles.connections
			ctx := context.Background()
			c := &model.AIConnection{ID: "discovery", WorkspaceID: "workspace", Scope: "workspace", Provider: provider, Name: "Discovery", Funding: "customer", Status: "connected"}
			if err := s.seal(c, aiConnectionSecret{APIKey: "test-key"}); err != nil {
				t.Fatal(err)
			}
			if err := s.repo.Create(ctx, c); err != nil {
				t.Fatal(err)
			}
			body := `{"data":[{"id":"gpt-existing"}]}`
			s.modelHTTP = &http.Client{Transport: modelListTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			fetch := func(refresh bool) []map[string]any {
				t.Helper()
				result, err := s.ConnectionModels(ctx, "workspace", "owner", c.ID, refresh)
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(result.Models)
				if err != nil {
					t.Fatal(err)
				}
				var rows []map[string]any
				if err := json.Unmarshal(encoded, &rows); err != nil {
					t.Fatal(err)
				}
				return rows
			}
			baseline := fetch(true)
			if baseline[0]["discovered_at"] != nil {
				t.Fatal("initial catalog should establish a baseline")
			}
			body = `{"data":[{"id":"gpt-existing"},{"id":"gpt-new"}]}`
			refreshed := fetch(true)
			if refreshed[0]["discovered_at"] != nil {
				t.Fatal("baseline model was marked new")
			}
			discoveredAt, ok := refreshed[1]["discovered_at"].(string)
			if !ok || discoveredAt == "" {
				t.Fatal("newly discovered model has no discovery time")
			}
			if _, err := time.Parse(time.RFC3339Nano, discoveredAt); err != nil {
				t.Fatal(err)
			}
			if got := fetch(true)[1]["discovered_at"]; got != discoveredAt {
				t.Fatalf("refresh reset discovery time: %v", got)
			}
			if got := fetch(false)[1]["discovered_at"]; got != discoveredAt {
				t.Fatalf("cache lost discovery time: %v", got)
			}
			body = `{"error":"offline"}`
			if got := fetch(true)[1]["discovered_at"]; got != discoveredAt {
				t.Fatalf("failure lost discovery time: %v", got)
			}
		})
	}
}
