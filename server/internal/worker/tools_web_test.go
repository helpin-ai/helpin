package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatWebSearchQueryIncludesDomainAllowlist(t *testing.T) {
	got := formatWebSearchQuery("pricing benchmarks", []string{"docs.example.com", "https://brave.com/"})
	expected := "(site:docs.example.com OR site:brave.com) pricing benchmarks"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestBuildExaSearchRequestAppliesDefaults(t *testing.T) {
	req, err := buildExaSearchRequest(exaSearchToolInput{
		Query:      "  latest ai safety research  ",
		NumResults: 99,
		Contents:   &exaSearchContentsInput{},
	})
	if err != nil {
		t.Fatalf("buildExaSearchRequest returned error: %v", err)
	}

	if req.Query != "latest ai safety research" {
		t.Fatalf("expected trimmed query, got %q", req.Query)
	}
	if req.Type != "auto" {
		t.Fatalf("expected default type auto, got %q", req.Type)
	}
	if req.NumResults != 10 {
		t.Fatalf("expected num results to clamp to 10, got %d", req.NumResults)
	}
	if req.Contents == nil || req.Contents.Highlights == nil {
		t.Fatalf("expected default highlights contents, got %#v", req.Contents)
	}
	if req.Contents.Highlights.MaxCharacters != 4000 {
		t.Fatalf("expected default highlight size 4000, got %d", req.Contents.Highlights.MaxCharacters)
	}
}

func TestBuildExaSearchRequestRejectsUnsupportedPeopleFilters(t *testing.T) {
	_, err := buildExaSearchRequest(exaSearchToolInput{
		Query:          "distributed systems engineer",
		Category:       "people",
		IncludeDomains: []string{"github.com"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "LinkedIn") {
		t.Fatalf("expected LinkedIn validation error, got %v", err)
	}
}

func TestBuildExaSearchRequestRejectsUnsupportedType(t *testing.T) {
	_, err := buildExaSearchRequest(exaSearchToolInput{
		Query: "agent tooling",
		Type:  "neural",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "type must be one of") {
		t.Fatalf("expected type validation error, got %v", err)
	}
}

func TestBuildExaSearchRequestRejectsMultipleContentModes(t *testing.T) {
	_, err := buildExaSearchRequest(exaSearchToolInput{
		Query: "agent tooling",
		Contents: &exaSearchContentsInput{
			Text:       &exaTextInput{MaxCharacters: 1000},
			Highlights: &exaHighlightsInput{MaxCharacters: 400},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "only one of text, highlights, or summary") {
		t.Fatalf("expected content-mode validation error, got %v", err)
	}
}

func TestExaSearchClientSearchUsesExpectedRequestShape(t *testing.T) {
	var capturedMethod string
	var capturedAuth string
	var payload ExaSearchRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedAuth = r.Header.Get("x-api-key")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requestId":"req-1","searchType":"auto","results":[{"title":"Example","url":"https://example.com","id":"result-1","highlights":["one"]}]}`))
	}))
	defer server.Close()

	client := NewExaSearchClient("test-key")
	client.apiURL = server.URL
	client.httpClient = server.Client()

	response, err := client.Search(context.Background(), ExaSearchRequest{
		Query:      "agent tooling",
		Type:       "auto",
		NumResults: 5,
		Contents: &ExaSearchContents{
			Highlights: &ExaHighlightsConfig{MaxCharacters: 4000},
		},
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if capturedMethod != http.MethodPost {
		t.Fatalf("expected POST request, got %s", capturedMethod)
	}
	if capturedAuth != "test-key" {
		t.Fatalf("expected x-api-key header, got %q", capturedAuth)
	}
	if payload.Query != "agent tooling" || payload.Type != "auto" || payload.NumResults != 5 {
		t.Fatalf("unexpected request payload %#v", payload)
	}
	if response.RequestID != "req-1" || len(response.Results) != 1 {
		t.Fatalf("unexpected response %#v", response)
	}
}

func TestWebSearchExaToolReturnsNormalizedResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"requestId":"req-2",
			"searchType":"fast",
			"results":[
				{
					"title":"Example Result",
					"url":"https://example.com/article",
					"id":"res-1",
					"publishedDate":"2026-04-16T00:00:00.000Z",
					"author":"Author Name",
					"highlights":["important detail"],
					"highlightScores":[0.98]
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewExaSearchClient("test-key")
	client.apiURL = server.URL
	client.httpClient = server.Client()

	registry := NewToolRegistry(nil, client)
	ctx := &ExecutionContext{Context: context.Background()}

	output, err := registry.Execute(ctx, "web_search_exa", json.RawMessage(`{"query":"latest agent tooling","type":"fast"}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var decoded exaToolResponse
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode tool output: %v", err)
	}
	if decoded.RequestID != "req-2" || decoded.SearchType != "fast" || decoded.ResultCount != 1 {
		t.Fatalf("unexpected tool output %#v", decoded)
	}
	if len(decoded.Results) != 1 || decoded.Results[0].URL != "https://example.com/article" {
		t.Fatalf("unexpected normalized results %#v", decoded.Results)
	}
}

func TestToolCatalogAlwaysIncludesExaSearch(t *testing.T) {
	catalog := ListToolCatalog()
	for _, tool := range catalog.Tools {
		if tool.Name == "web_search_exa" {
			return
		}
	}
	t.Fatal("expected web_search_exa in tool catalog")
}
