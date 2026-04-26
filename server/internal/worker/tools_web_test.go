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

func TestBuildExaSearchRequestAcceptsNeuralType(t *testing.T) {
	_, err := buildExaSearchRequest(exaSearchToolInput{
		Query: "agent tooling",
		Type:  "neural",
	})
	if err != nil {
		t.Fatalf("expected neural type to be accepted, got %v", err)
	}
}

func TestBuildExaSearchRequestAcceptsDeepLiteType(t *testing.T) {
	req, err := buildExaSearchRequest(exaSearchToolInput{
		Query: "agent tooling",
		Type:  "deep-lite",
	})
	if err != nil {
		t.Fatalf("expected deep-lite type to be accepted, got %v", err)
	}
	if req.Type != "deep-lite" {
		t.Fatalf("expected deep-lite type, got %q", req.Type)
	}
}

func TestParseAllowedWebFetchURLAcceptsBareDomainPaths(t *testing.T) {
	parsed, err := parseAllowedWebFetchURL("docs.example.com/changelog")
	if err != nil {
		t.Fatalf("parseAllowedWebFetchURL returned error: %v", err)
	}
	if parsed.String() != "https://docs.example.com/changelog" {
		t.Fatalf("expected https-normalized URL, got %q", parsed.String())
	}
}

func TestBuildExaSearchRequestRejectsUnsupportedType(t *testing.T) {
	_, err := buildExaSearchRequest(exaSearchToolInput{
		Query: "agent tooling",
		Type:  "unknown",
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

func TestFetchURLToolReturnsExtractedPageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Product Updates</title><meta name="description" content="Latest releases"></head><body><main><h1>April changelog</h1><p>Brand Explorer shipped on 2026-04-04.</p><a href="/changelog">Changelog</a></main><script>ignore()</script></body></html>`))
	}))
	defer server.Close()

	originalClient := webFetchHTTPClient
	originalAllowPrivate := allowPrivateWebFetchHostsForTests
	webFetchHTTPClient = server.Client()
	allowPrivateWebFetchHostsForTests = true
	defer func() {
		webFetchHTTPClient = originalClient
		allowPrivateWebFetchHostsForTests = originalAllowPrivate
	}()

	registry := NewToolRegistry(nil)
	output, err := registry.Execute(&ExecutionContext{Context: context.Background()}, "fetch_url", json.RawMessage(`{"url":"`+server.URL+`","max_characters":200}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var decoded fetchURLToolResponse
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.Title != "Product Updates" || !strings.Contains(decoded.Text, "Brand Explorer shipped") {
		t.Fatalf("unexpected fetched content %#v", decoded)
	}
	if len(decoded.Links) != 1 || decoded.Links[0].URL != server.URL+"/changelog" {
		t.Fatalf("expected normalized changelog link, got %#v", decoded.Links)
	}
}

func TestCrawlURLToolPrioritizesUpdateLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/pricing">Pricing</a><a href="/changelog">Changelog</a><a href="/blog/product-updates">Product updates</a></body></html>`))
	})
	mux.HandleFunc("/changelog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Changelog</title></head><body><p>New GEO tracker released.</p></body></html>`))
	})
	mux.HandleFunc("/blog/product-updates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Updates</title></head><body><p>Portfolio launch notes.</p></body></html>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	originalClient := webFetchHTTPClient
	originalAllowPrivate := allowPrivateWebFetchHostsForTests
	webFetchHTTPClient = server.Client()
	allowPrivateWebFetchHostsForTests = true
	defer func() {
		webFetchHTTPClient = originalClient
		allowPrivateWebFetchHostsForTests = originalAllowPrivate
	}()

	registry := NewToolRegistry(nil)
	output, err := registry.Execute(&ExecutionContext{Context: context.Background()}, "crawl_url", json.RawMessage(`{"url":"`+server.URL+`","max_pages":3,"max_depth":1}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var decoded crawlURLToolResponse
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.PageCount != 3 {
		t.Fatalf("expected 3 crawled pages, got %#v", decoded)
	}
	combined := ""
	for _, page := range decoded.Pages {
		combined += page.Text + "\n"
	}
	if !strings.Contains(combined, "New GEO tracker released") || !strings.Contains(combined, "Portfolio launch notes") {
		t.Fatalf("expected crawled update page text, got %q", combined)
	}
}

func TestFetchURLRejectsPrivateHostsByDefault(t *testing.T) {
	_, err := fetchWebPage(context.Background(), "http://127.0.0.1/changelog", 1000, false)
	if err == nil {
		t.Fatal("expected private host rejection")
	}
	if !strings.Contains(err.Error(), "private or local IP") {
		t.Fatalf("expected private IP error, got %v", err)
	}
}

func TestToolCatalogAlwaysIncludesExaSearch(t *testing.T) {
	catalog := ListToolCatalog()
	found := map[string]bool{}
	for _, tool := range catalog.Tools {
		if tool.Name == "web_search_exa" || tool.Name == "fetch_url" || tool.Name == "crawl_url" {
			found[tool.Name] = true
		}
	}
	for _, name := range []string{"web_search_exa", "fetch_url", "crawl_url"} {
		if !found[name] {
			t.Fatalf("expected %s in tool catalog", name)
		}
	}
}
