package crawler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCloudflareCrawlSendsURLPatternsForPathAndHostRules(t *testing.T) {
	var received struct {
		Options struct {
			ExcludePatterns []string `json:"excludePatterns"`
			IncludePatterns []string `json:"includePatterns"`
		} `json:"options"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		writeCloudflarePatternFixture(t, w, `{"success":true,"result":"job"}`)
	}))
	defer server.Close()
	client := &CloudflareCrawlClient{accountID: "account", apiToken: "test", baseURL: server.URL, client: server.Client()}
	_, err := client.StartCrawl(context.Background(), model.SupportContentSource{
		StartURL:        "https://example.com",
		IncludePatterns: model.DocsStringArray{"/docs/*"},
		ExcludePatterns: model.DocsStringArray{"/blog", "/blog/*", "*://blog.*/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"*://*/blog", "*://*/blog/**", "*://blog.*/**"}; !reflect.DeepEqual(received.Options.ExcludePatterns, want) {
		t.Fatalf("exclude patterns = %v, want %v", received.Options.ExcludePatterns, want)
	}
	if want := []string{"*://*/docs/**"}; !reflect.DeepEqual(received.Options.IncludePatterns, want) {
		t.Fatalf("include patterns = %v, want %v", received.Options.IncludePatterns, want)
	}
}

func TestCloudflareCrawlDoesNotDeliverExcludedPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			writeCloudflarePatternFixture(t, w, `{"success":true,"result":"job"}`)
			return
		}
		writeCloudflarePatternFixture(t, w, `{"success":true,"result":{"id":"job","status":"completed","records":[{"url":"https://example.com/blog?category=news","status":"completed","markdown":"Blog listing"},{"url":"https://blog.example.com/post","status":"completed","markdown":"Blog post"},{"url":"https://example.com/docs","status":"completed","markdown":"Product docs"}]}}`)
	}))
	defer server.Close()
	client := &CloudflareCrawlClient{accountID: "account", apiToken: "test", baseURL: server.URL, client: server.Client()}
	var delivered []string
	count, err := crawlWithCloudflare(context.Background(), client, model.SupportContentSource{
		StartURL: "https://example.com", IncludeSubdomains: true,
		ExcludePatterns: model.DocsStringArray{"/blog", "/blog/*", "*://blog.*/**"},
	}, slog.Default(), func(record CrawlRecord) error { delivered = append(delivered, record.URL); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !reflect.DeepEqual(delivered, []string{"https://example.com/docs"}) {
		t.Fatalf("delivered %d pages: %v", count, delivered)
	}
}

func writeCloudflarePatternFixture(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Error(err)
	}
}
