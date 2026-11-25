package publichelp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testFetcher() *Fetcher {
	fetch := NewFetcherWithHTTP(&http.Client{Timeout: 5 * time.Second})
	fetch.sleep = func(context.Context, time.Duration) error { return nil }
	return fetch
}

func TestSiteLinkTarget(t *testing.T) {
	site := &Site{
		BaseURL: "https://help.acme.com/en/",
		Hosts:   []string{"acme.helpdesk.example"},
		Collections: []Collection{{
			ID: "c1", URL: "https://help.acme.com/en/category/billing/",
			Articles: []ArticleRef{{ID: "a1", URL: "https://help.acme.com/en/article/Refunds-x1/"}},
			Children: []Collection{{ID: "c2", Articles: []ArticleRef{{ID: "a2", URL: "https://help.acme.com/guides/setup"}}}},
		}},
	}
	tests := []struct {
		name, href string
		want       LinkTarget
		ok         bool
	}{
		{name: "listed article", href: "https://help.acme.com/en/article/refunds-x1", want: LinkTarget{Kind: LinkArticle, ID: "a1", Certain: true}, ok: true},
		{name: "www and fragment", href: "http://www.help.acme.com/en/article/refunds-x1/#fees", want: LinkTarget{Kind: LinkArticle, ID: "a1", Fragment: "fees", Certain: true}, ok: true},
		{name: "markdown twin on another host", href: "https://acme.helpdesk.example/guides/setup.md", want: LinkTarget{Kind: LinkArticle, ID: "a2", Certain: true}, ok: true},
		{name: "collection", href: "https://help.acme.com/en/category/billing", want: LinkTarget{Kind: LinkCollection, ID: "c1", Certain: true}, ok: true},
		{name: "unknown page", href: "https://help.acme.com/en/article/gone"},
		{name: "another site", href: "https://other.example/en/article/refunds-x1"},
		{name: "relative link was already rewritten", href: "/en/article/refunds-x1"},
		{name: "not a web link", href: "mailto:help@acme.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := site.LinkTarget(tt.href)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("LinkTarget(%q) = %+v, %v; want %+v, %v", tt.href, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSiteLinkTargetByShape(t *testing.T) {
	site := &Site{BaseURL: "https://help.acme.com/", MatchLink: func(link *url.URL) (LinkKind, string, bool) {
		id, ok := strings.CutPrefix(link.Path, "/articles/")
		return LinkArticle, id, ok
	}}
	if got, ok := site.LinkTarget("https://help.acme.com/articles/42#top"); !ok || got != (LinkTarget{Kind: LinkArticle, ID: "42", Fragment: "top", Certain: true}) {
		t.Fatalf("LinkTarget = %+v, %v", got, ok)
	}
}

type fakeReader struct {
	key string
	err error
}

func (f fakeReader) Key() string { return f.key }
func (f fakeReader) Probe(context.Context, *Page) (*Site, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &Site{Platform: f.key}, nil
}
func (fakeReader) Load(context.Context, string, func()) (*Site, error) { return nil, nil }
func (fakeReader) Article(context.Context, string) (*Article, error)   { return nil, nil }

func TestDetect(t *testing.T) {
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status == http.StatusForbidden {
			w.Header().Set("Cf-Mitigated", "challenge")
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	no := fakeReader{key: "no", err: ErrNotHelpCenter}
	yes := fakeReader{key: "yes"}
	locked := fakeReader{key: "locked", err: &PlatformError{Platform: "locked", Err: ErrRestricted}}

	reader, site, err := Detect(context.Background(), testFetcher(), []Reader{no, yes, locked}, server.URL+"/?utm=1#top")
	if err != nil || reader.Key() != "yes" || site.Platform != "yes" {
		t.Fatalf("Detect = %v, %+v, %v; want the first reader that accepts", reader, site, err)
	}
	var known *PlatformError
	if _, _, err := Detect(context.Background(), testFetcher(), []Reader{no, locked, yes}, server.URL); !errors.As(err, &known) || known.Platform != "locked" {
		t.Fatalf("Detect error = %v, want the platform's own error", err)
	}
	if _, _, err := Detect(context.Background(), testFetcher(), []Reader{no}, server.URL); !errors.Is(err, ErrNotHelpCenter) {
		t.Fatalf("Detect error = %v, want ErrNotHelpCenter", err)
	}
	status = http.StatusForbidden
	if _, _, err := Detect(context.Background(), testFetcher(), []Reader{no}, server.URL); !errors.Is(err, ErrChallenged) {
		t.Fatalf("Detect error = %v, want ErrChallenged", err)
	}
	if _, _, err := Detect(context.Background(), testFetcher(), nil, "ftp://help.acme.com"); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("Detect error = %v, want ErrInvalidURL", err)
	}
}

func TestFetcherGet(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/busy":
			if calls++; calls < 3 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write([]byte(" ready \n"))
		case "/gone":
			w.WriteHeader(http.StatusNotFound)
		case "/login":
			w.WriteHeader(http.StatusUnauthorized)
		case "/down":
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer server.Close()
	fetch := testFetcher()
	page, err := fetch.Get(context.Background(), server.URL+"/busy")
	if err != nil || string(page.Body) != "ready" || calls != 3 {
		t.Fatalf("Get(/busy) = %q, %v after %d calls; want a retry until served", page.Body, err, calls)
	}
	for path, want := range map[string]error{"/gone": ErrNotFound, "/login": ErrRestricted, "/down": ErrUnreachable} {
		if _, err := fetch.Get(context.Background(), server.URL+path); !errors.Is(err, want) {
			t.Errorf("Get(%s) error = %v, want %v", path, err, want)
		}
	}
	if _, err := NewFetcher().Get(context.Background(), server.URL+"/busy"); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("a loopback address must be refused, got %v", err)
	}
}

func TestSitemapFollowsIndexes(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			_, _ = w.Write([]byte(`<sitemapindex><sitemap><loc>` + server.URL + `/articles.xml</loc></sitemap><sitemap><loc>` + server.URL + `/broken.xml</loc></sitemap></sitemapindex>`))
		case "/articles.xml":
			_, _ = w.Write([]byte(`<urlset xmlns:xhtml="http://www.w3.org/1999/xhtml"><url><loc> ` + server.URL + `/a </loc><lastmod>2026-01-02</lastmod><xhtml:link rel="alternate" hreflang="fr" href="x"/></url><url><loc>` + server.URL + `/b</loc></url></urlset>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	entries, err := testFetcher().Sitemap(context.Background(), server.URL+"/sitemap.xml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0] != (SitemapEntry{Loc: server.URL + "/a", LastMod: "2026-01-02"}) || entries[1].Loc != server.URL+"/b" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestAbsoluteHTML(t *testing.T) {
	base, _ := url.Parse("https://help.acme.com/en/article/refunds/")
	got := AbsoluteHTML(`<p><a href="../billing">Billing</a> <a href="#fees">Fees</a> <img src="//cdn.acme.com/a.png">`+
		`<img src="data:image/gif;base64,AAAA" data-src="/lazy.png"><script>track()</script></p>`, base)
	for _, want := range []string{
		`href="https://help.acme.com/en/article/billing"`, `href="#fees"`, `src="https://cdn.acme.com/a.png"`, `src="https://help.acme.com/lazy.png"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AbsoluteHTML = %s, want it to contain %s", got, want)
		}
	}
	if strings.Contains(got, "track()") {
		t.Errorf("AbsoluteHTML kept a script: %s", got)
	}
}

func TestMarkdownToHTML(t *testing.T) {
	got, err := MarkdownToHTML("| A | B |\n|---|---|\n| 1 | 2 |\n\n<details><summary>More</summary>x</details>")
	if err != nil || !strings.Contains(got, "<table>") || !strings.Contains(got, "<details>") {
		t.Fatalf("MarkdownToHTML = %q, %v", got, err)
	}
}
