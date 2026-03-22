package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
)

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestParseSitemapFromRobots(t *testing.T) {
	tests := []struct {
		name       string
		robotsBody string
		statusCode int
		want       []string
	}{
		{
			name: "single sitemap directive",
			robotsBody: `User-agent: *
Disallow: /private/
Sitemap: https://example.com/sitemap.xml
`,
			statusCode: 200,
			want:       []string{"https://example.com/sitemap.xml"},
		},
		{
			name: "multiple sitemap directives",
			robotsBody: `User-agent: *
Disallow: /private/
Sitemap: https://example.com/sitemap1.xml
Sitemap: https://example.com/sitemap2.xml
`,
			statusCode: 200,
			want:       []string{"https://example.com/sitemap1.xml", "https://example.com/sitemap2.xml"},
		},
		{
			name:       "no sitemap directives",
			robotsBody: "User-agent: *\nDisallow: /\n",
			statusCode: 200,
			want:       nil,
		},
		{
			name:       "robots.txt not found",
			robotsBody: "Not Found",
			statusCode: 404,
			want:       nil,
		},
		{
			name: "case insensitive sitemap directive",
			robotsBody: `sitemap: https://example.com/sitemap.xml
SITEMAP: https://example.com/sitemap2.xml
`,
			statusCode: 200,
			want:       []string{"https://example.com/sitemap.xml", "https://example.com/sitemap2.xml"},
		},
		{
			name: "empty sitemap value is ignored",
			robotsBody: `Sitemap:
Sitemap: https://example.com/sitemap.xml
`,
			statusCode: 200,
			want:       []string{"https://example.com/sitemap.xml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.robotsBody))
			}))
			defer server.Close()

			logger := testLogger(t)
			got := parseSitemapFromRobots(context.Background(), server.Client(), server.URL+"/robots.txt", logger)

			if tt.want == nil {
				if got != nil {
					t.Errorf("parseSitemapFromRobots() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseSitemapFromRobots() returned %d entries, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("parseSitemapFromRobots()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseSitemapFromRobots_ServerError(t *testing.T) {
	// Server that closes connection immediately.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer server.Close()

	logger := testLogger(t)
	got := parseSitemapFromRobots(context.Background(), server.Client(), server.URL+"/robots.txt", logger)
	if got != nil {
		t.Errorf("parseSitemapFromRobots() with connection error = %v, want nil", got)
	}
}

func TestDiscoverSitemapURLs_WithSitemapXML(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page1</loc></url>
  <url><loc>https://example.com/page2</loc></url>
  <url><loc>https://example.com/page3</loc></url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(sitemapXML))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	logger := testLogger(t)
	got := discoverSitemapURLs(context.Background(), server.Client(), server.URL+"/start", logger)

	want := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
	}

	if len(got) != len(want) {
		t.Fatalf("discoverSitemapURLs() returned %d URLs, want %d. Got: %v", len(got), len(want), got)
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("discoverSitemapURLs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverSitemapURLs_FallsBackToRobotsTxt(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/from-robots</loc></url>
</urlset>`

	var sitemapURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			// Return 404 to force robots.txt fallback.
			w.WriteHeader(404)
		case "/robots.txt":
			w.Write([]byte("Sitemap: " + sitemapURL + "/custom-sitemap.xml\n"))
		case "/custom-sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(sitemapXML))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	sitemapURL = server.URL

	logger := testLogger(t)
	got := discoverSitemapURLs(context.Background(), server.Client(), server.URL+"/start", logger)

	if len(got) != 1 {
		t.Fatalf("discoverSitemapURLs() returned %d URLs, want 1. Got: %v", len(got), got)
	}
	if got[0] != "https://example.com/from-robots" {
		t.Errorf("discoverSitemapURLs()[0] = %q, want %q", got[0], "https://example.com/from-robots")
	}
}

func TestDiscoverSitemapURLs_NoSitemapFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	logger := testLogger(t)
	got := discoverSitemapURLs(context.Background(), server.Client(), server.URL+"/start", logger)

	if len(got) != 0 {
		t.Errorf("discoverSitemapURLs() returned %d URLs, want 0. Got: %v", len(got), got)
	}
}

func TestDiscoverSitemapURLs_InvalidStartURL(t *testing.T) {
	logger := testLogger(t)
	got := discoverSitemapURLs(context.Background(), http.DefaultClient, "://invalid", logger)
	if got != nil {
		t.Errorf("discoverSitemapURLs() with invalid URL = %v, want nil", got)
	}
}

func TestDiscoverSitemapURLs_SitemapIndex(t *testing.T) {
	// A sitemap that contains URLs ending in .xml triggers recursive expansion
	// in fetchSitemap. Use a <urlset> that references a child sitemap URL.
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			// Return a urlset where one entry is itself a .xml URL (child sitemap).
			sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + serverURL + `/sitemap-pages.xml</loc></url>
</urlset>`
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(sitemapXML))
		case "/sitemap-pages.xml":
			childSitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/indexed-page</loc></url>
</urlset>`
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(childSitemap))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	logger := testLogger(t)
	got := discoverSitemapURLs(context.Background(), server.Client(), server.URL+"/start", logger)

	if len(got) != 1 {
		t.Fatalf("discoverSitemapURLs() with sitemap index returned %d URLs, want 1. Got: %v", len(got), got)
	}
	if got[0] != "https://example.com/indexed-page" {
		t.Errorf("got[0] = %q, want %q", got[0], "https://example.com/indexed-page")
	}
}

func TestFetchSitemap_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	logger := testLogger(t)
	got := fetchSitemap(context.Background(), server.Client(), server.URL+"/sitemap.xml", logger)
	if len(got) != 0 {
		t.Errorf("fetchSitemap() with 500 status returned %d URLs, want 0", len(got))
	}
}

func TestFetchSitemap_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page</loc></url>
</urlset>`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	logger := testLogger(t)
	got := fetchSitemap(ctx, server.Client(), server.URL+"/sitemap.xml", logger)
	if len(got) != 0 {
		t.Errorf("fetchSitemap() with cancelled context returned %d URLs, want 0", len(got))
	}
}
