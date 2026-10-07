package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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
