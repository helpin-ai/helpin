package zendesk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// fakeHelpCenter serves the parts of a Zendesk help center the reader uses.
type fakeHelpCenter struct {
	*httptest.Server
	// challengePage answers the home page with a bot challenge, as some help
	// centers do while their API stays open.
	challengePage bool
	// token, when set, is required on every API request.
	token string
	// requests lists the API paths asked for.
	requests []string
}

func newFakeHelpCenter(t *testing.T) *fakeHelpCenter {
	t.Helper()
	fake := &fakeHelpCenter{}
	mux := http.NewServeMux()
	mux.HandleFunc("/hc/", func(w http.ResponseWriter, _ *http.Request) {
		if fake.challengePage {
			w.Header().Set("Cf-Mitigated", "challenge")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`<html><head><title>Acme Help</title><script src="https://static.zdassets.com/hc/assets/hc.js"></script></head></html>`))
	})
	mux.HandleFunc("/api/v2/help_center/", func(w http.ResponseWriter, r *http.Request) {
		fake.requests = append(fake.requests, r.URL.RequestURI())
		if fake.token != "" {
			if user, password, ok := r.BasicAuth(); !ok || user != "agent@acme.com/token" || password != fake.token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		base := fake.URL
		path := strings.TrimPrefix(r.URL.Path, "/api/v2/help_center/")
		var body any
		switch {
		case path == "locales.json":
			body = map[string]any{"locales": []string{"en-us", "de"}, "default_locale": "en-us"}
		case path == "en-us/categories.json":
			body = map[string]any{"categories": []map[string]any{
				{"id": 2, "name": "Billing", "position": 1, "html_url": base + "/hc/en-us/categories/2-Billing"},
				{"id": 1, "name": "Basics", "description": "Start here", "position": 0, "html_url": base + "/hc/en-us/categories/1-Basics"},
				{"id": 3, "name": "Empty", "position": 2, "html_url": base + "/hc/en-us/categories/3-Empty"},
			}}
		case path == "en-us/sections.json":
			body = map[string]any{"sections": []map[string]any{
				{"id": 10, "name": "Setup", "category_id": 1, "position": 0, "html_url": base + "/hc/en-us/sections/10-Setup"},
				{"id": 11, "name": "Advanced setup", "category_id": 1, "parent_section_id": 10, "position": 0, "html_url": base + "/hc/en-us/sections/11-Advanced"},
				{"id": 20, "name": "Invoices", "category_id": 2, "position": 0, "html_url": base + "/hc/en-us/sections/20-Invoices"},
			}}
		case path == "en-us/articles.json" && r.URL.Query().Get("page[after]") == "":
			body = map[string]any{
				"meta":  map[string]any{"has_more": true},
				"links": map[string]any{"next": base + "/api/v2/help_center/en-us/articles.json?page%5Bafter%5D=next&page%5Bsize%5D=100"},
				"articles": []map[string]any{
					{"id": 101, "title": "Second step", "section_id": 10, "position": 1, "html_url": base + "/hc/en-us/articles/101-Second-step"},
					{"id": 100, "title": "First step", "section_id": 10, "position": 0, "html_url": base + "/hc/en-us/articles/100-First-step"},
				},
			}
		case path == "en-us/articles.json":
			body = map[string]any{"articles": []map[string]any{
				{"id": 110, "title": "Single sign-on", "section_id": 11, "position": 0, "html_url": base + "/hc/en-us/articles/110-Single-sign-on"},
				{"id": 200, "title": " ", "section_id": 20, "position": 0, "html_url": base + "/hc/en-us/articles/200"},
				{"id": 201, "title": "Unfinished", "section_id": 20, "draft": true, "html_url": base + "/hc/en-us/articles/201-Unfinished"},
			}}
		case path == "en-us/articles/100.json":
			body = map[string]any{"article": map[string]any{
				"id": 100, "title": "First step", "html_url": base + "/hc/en-us/articles/100-First-step",
				"body": `<p>See <a href="/hc/en-us/articles/101">the second step</a>.</p><p><img src="/hc/article_attachments/5"></p><script>x()</script>`,
			}}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode: %v", err)
		}
	})
	fake.Server = httptest.NewServer(mux)
	t.Cleanup(fake.Close)
	return fake
}

func testReader() (*Reader, *publichelp.Fetcher) {
	fetch := publichelp.NewFetcherWithHTTP(&http.Client{Timeout: 5 * time.Second})
	return New(fetch), fetch
}

func probe(t *testing.T, reader *Reader, fetch *publichelp.Fetcher, address string) (*publichelp.Site, error) {
	t.Helper()
	home, err := fetch.Open(context.Background(), address)
	if err != nil {
		t.Fatal(err)
	}
	return reader.Probe(context.Background(), home)
}

func TestProbe(t *testing.T) {
	fake := newFakeHelpCenter(t)
	reader, fetch := testReader()

	site, err := probe(t, reader, fetch, fake.URL+"/hc/de/articles/5")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if site.Platform != Key || site.BaseURL != fake.URL+"/hc/de/" || site.Locale != "de" || site.Name != "Acme Help" {
		t.Fatalf("site = %+v, want the locale of the address", site)
	}

	// A challenged page tells nothing; the API still answers.
	fake.challengePage = true
	site, err = probe(t, reader, fetch, fake.URL+"/hc/")
	if err != nil || site.BaseURL != fake.URL+"/hc/en-us/" || site.Name != "" {
		t.Fatalf("Probe behind a challenge = %+v, %v; want the default locale", site, err)
	}
}

func TestProbeRejectsOtherSites(t *testing.T) {
	asked := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = asked || strings.HasPrefix(r.URL.Path, "/api/")
		_, _ = w.Write([]byte("<html><title>Acme</title></html>"))
	}))
	defer server.Close()
	reader, fetch := testReader()
	if _, err := probe(t, reader, fetch, server.URL); !errors.Is(err, publichelp.ErrNotHelpCenter) {
		t.Fatalf("Probe error = %v, want ErrNotHelpCenter", err)
	}
	if asked {
		t.Fatal("a page with no sign of Zendesk must not cost an API request")
	}
}

func TestProbeKeepsTheTokenFromOtherSites(t *testing.T) {
	signed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := r.BasicAuth()
		signed = signed || ok
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	reader, fetch := testReader()
	if _, err := probe(t, reader.WithToken("agent@acme.com", "secret"), fetch, server.URL); !errors.Is(err, publichelp.ErrNotHelpCenter) {
		t.Fatalf("Probe error = %v, want ErrNotHelpCenter", err)
	}
	if signed {
		t.Fatal("a site with no sign of Zendesk was sent the token")
	}
}

func TestProbeAsksForATokenWhenRefused(t *testing.T) {
	fake := newFakeHelpCenter(t)
	fake.token = "secret"
	reader, fetch := testReader()

	var known *publichelp.PlatformError
	if _, err := probe(t, reader, fetch, fake.URL+"/hc/en-us"); !errors.As(err, &known) || known.Platform != Key || !errors.Is(err, publichelp.ErrRestricted) {
		t.Fatalf("Probe error = %v, want a Zendesk sign-in error", err)
	}
	site, err := probe(t, reader.WithToken(" agent@acme.com ", "secret"), fetch, fake.URL+"/hc/en-us")
	if err != nil || site.Locale != "en-us" {
		t.Fatalf("Probe with a token = %+v, %v", site, err)
	}
}

func TestLoad(t *testing.T) {
	fake := newFakeHelpCenter(t)
	reader, _ := testReader()
	beats := 0
	site, err := reader.Load(context.Background(), fake.URL+"/hc/en-us/", func() { beats++ })
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var lines []string
	var walk func(collections []publichelp.Collection, indent string)
	walk = func(collections []publichelp.Collection, indent string) {
		for _, collection := range collections {
			lines = append(lines, fmt.Sprintf("%s%s %s", indent, collection.ID, collection.Name))
			for _, article := range collection.Articles {
				lines = append(lines, fmt.Sprintf("%s  %s %s", indent, article.ID, article.Title))
			}
			walk(collection.Children, indent+"  ")
		}
	}
	walk(site.Collections, "")
	want := []string{
		"category-1 Basics",
		"  10 Setup",
		"    100 First step",
		"    101 Second step",
		"    11 Advanced setup",
		"      110 Single sign-on",
		"category-2 Billing",
		"  20 Invoices",
		"    200 Untitled",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tree:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	// Categories, sections, and two pages of articles.
	if beats != 4 {
		t.Fatalf("progress was called %d times, want 4", beats)
	}
	for href, want := range map[string]publichelp.LinkTarget{
		fake.URL + "/hc/en-us/articles/101-Renamed#b": {Kind: publichelp.LinkArticle, ID: "101", Fragment: "b", Certain: true},
		fake.URL + "/hc/articles/100":                 {Kind: publichelp.LinkArticle, ID: "100", Certain: true},
		fake.URL + "/hc/en-us/sections/10-Setup":      {Kind: publichelp.LinkCollection, ID: "10", Certain: true},
		fake.URL + "/hc/de/categories/2":              {Kind: publichelp.LinkCollection, ID: "category-2", Certain: true},
	} {
		if got, ok := site.LinkTarget(href); !ok || got != want {
			t.Errorf("LinkTarget(%s) = %+v, %v; want %+v", href, got, ok, want)
		}
	}
	if _, ok := site.LinkTarget(fake.URL + "/hc/en-us/requests/new"); ok {
		t.Error("a request form is not an article")
	}
}

func TestArticle(t *testing.T) {
	fake := newFakeHelpCenter(t)
	reader, _ := testReader()
	article, err := reader.Article(context.Background(), fake.URL+"/hc/en-us/articles/100-Old-slug?utm=1")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if article.ID != "100" || article.Title != "First step" || article.URL != fake.URL+"/hc/en-us/articles/100-First-step" {
		t.Fatalf("article = %+v", article)
	}
	for _, want := range []string{`href="` + fake.URL + `/hc/en-us/articles/101"`, `src="` + fake.URL + `/hc/article_attachments/5"`} {
		if !strings.Contains(article.HTML, want) {
			t.Errorf("body = %s, want it to contain %s", article.HTML, want)
		}
	}
	if strings.Contains(article.HTML, "x()") {
		t.Errorf("body kept a script: %s", article.HTML)
	}
	if _, err := reader.Article(context.Background(), fake.URL+"/hc/en-us/articles/999-Gone"); !errors.Is(err, publichelp.ErrNotFound) {
		t.Errorf("a removed article gives %v, want ErrNotFound", err)
	}
	if _, err := reader.Article(context.Background(), fake.URL+"/hc/en-us/sections/10-Setup"); !errors.Is(err, publichelp.ErrInvalidURL) {
		t.Errorf("a section address gives %v, want ErrInvalidURL", err)
	}
}
