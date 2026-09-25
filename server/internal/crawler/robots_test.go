package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCrawlRobotsLinksSitemapsAndRedirects(t *testing.T) {
	var robots, forbidden atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != crawlerUserAgent {
			t.Errorf("user agent = %q", r.UserAgent())
		}
		switch r.URL.Path {
		case "/robots.txt":
			robots.Add(1)
			fmt.Fprint(w, "User-agent: *\nDisallow: /\n\nUser-agent: Helpin-Crawler\nDisallow: /private\nAllow: /private/public\n")
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><p>Public product documentation.</p><a href="/private/link">blocked</a><a href="/redirect">redirect</a><a href="/private/public">allowed</a></body></html>`)
		case "/redirect":
			http.Redirect(w, r, "/private/redirect", http.StatusFound)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>http://%s/private/sitemap</loc></url></urlset>`, r.Host)
		case "/private/public":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><p>Allowed public documentation inside a blocked section.</p></body></html>`)
		default:
			forbidden.Add(1)
			t.Errorf("disallowed request reached server: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	skips := map[string]bool{}
	n, err := crawlWithColly(context.Background(), model.SupportContentSource{StartURL: srv.URL, CrawlSource: "all", CrawlDepth: 3, CrawlLimit: 20}, nil, newTestLogger(t), func(r CrawlRecord) error {
		if r.SkipReason != "" {
			skips[r.URL] = true
			if r.Markdown != "" {
				t.Error("skip includes content")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(skips) != 3 || robots.Load() != 1 || forbidden.Load() != 0 {
		t.Fatalf("pages=%d skips=%v robots=%d forbidden=%d", n, skips, robots.Load(), forbidden.Load())
	}
}

func TestRobotsHTTPStatusPolicy(t *testing.T) {
	for _, status := range []int{200, 404, 403, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var pages atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.WriteHeader(status)
					return
				}
				pages.Add(1)
				fmt.Fprint(w, "public")
			}))
			defer srv.Close()
			transport := newRobotsTransport(context.Background(), http.DefaultTransport, func(string, error) {})
			client := &http.Client{Transport: transport}
			resp, err := client.Get(srv.URL + "/docs")
			if resp != nil {
				resp.Body.Close()
			}
			wantBlocked := status == 429 || status >= 500
			if (err != nil) != wantBlocked {
				t.Fatalf("error=%v", err)
			}
			if wantBlocked && pages.Load() != 0 {
				t.Fatal("fetched before robots recovered")
			}
		})
	}
}

func TestCrawlRobotsUnavailableIsTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	n, err := crawlWithColly(context.Background(), model.SupportContentSource{StartURL: srv.URL, CrawlSource: "links"}, nil, newTestLogger(t), func(CrawlRecord) error { t.Fatal("must not index"); return nil })
	if n != 0 || err == nil || !strings.Contains(err.Error(), "robots.txt unavailable") {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestCloudflareDisallowedRecordsAreReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			io.WriteString(w, `{"success":true,"result":"job"}`)
			return
		}
		records := `[{"url":"https://example.com/docs","status":"completed","markdown":"Public docs"}]`
		if r.URL.Query().Get("status") == "" {
			records = `[{"url":"https://example.com/docs","status":"completed","markdown":"Public docs"},{"url":"https://example.com/private","status":"disallowed"}]`
		}
		fmt.Fprintf(w, `{"success":true,"result":{"id":"job","status":"completed","records":%s}}`, records)
	}))
	defer srv.Close()
	client := NewCloudflareCrawlClient("account", "test-token", srv.URL)
	var skipped int
	n, err := crawlWithCloudflare(context.Background(), client, model.SupportContentSource{StartURL: "https://example.com"}, newTestLogger(t), func(r CrawlRecord) error {
		if r.SkipReason != "" {
			skipped++
		}
		return nil
	})
	if err != nil || n != 1 || skipped != 1 {
		t.Fatalf("pages=%d skipped=%d err=%v", n, skipped, err)
	}
}
