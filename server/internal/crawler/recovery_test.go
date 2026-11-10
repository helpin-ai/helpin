package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCrawlWWWRedirectChecksDestinationRobots(t *testing.T) {
	for _, disallow := range []bool{false, true} {
		t.Run(fmt.Sprint(disallow), func(t *testing.T) {
			var destinationRobots, pages int
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					if r.URL.Hostname() == "www.example.test" {
						destinationRobots++
						if disallow {
							fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
						}
					}
					return
				}
				if r.URL.Hostname() == "example.test" {
					http.Redirect(w, r, "http://www.example.test/", http.StatusMovedPermanently)
					return
				}
				pages++
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<html><body><h1>Guide</h1><p>Configure your account from the settings page.</p></body></html>")
			}))
			defer proxy.Close()
			n, err := crawlWithColly(context.Background(), model.SupportContentSource{StartURL: "http://example.test", CrawlSource: "links"}, []string{proxy.URL}, newTestLogger(t), func(CrawlRecord) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if disallow {
				want = 0
			}
			if n != want || pages != want || destinationRobots != 1 {
				t.Fatalf("indexed=%d fetched=%d destination robots=%d", n, pages, destinationRobots)
			}
		})
	}
}

func TestCrawlHTTPFailureIsRetriedByFreshCrawl(t *testing.T) {
	for _, status := range []int{429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			attempt := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					return
				}
				attempt++
				if attempt == 1 {
					w.WriteHeader(status)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<html><body><p>Recovered documentation for connecting your account.</p></body></html>")
			}))
			defer srv.Close()
			source := model.SupportContentSource{StartURL: srv.URL, CrawlSource: "links"}
			if n, err := crawlWithColly(context.Background(), source, nil, newTestLogger(t), func(CrawlRecord) error { return nil }); err == nil || n != 0 {
				t.Fatalf("failed request reported success: n=%d err=%v", n, err)
			}
			if n, err := crawlWithColly(context.Background(), source, nil, newTestLogger(t), func(CrawlRecord) error { return nil }); err != nil || n != 1 {
				t.Fatalf("fresh crawl did not recover: n=%d err=%v", n, err)
			}
		})
	}
}

func TestCrawlRedirectToVisitedPageDoesNotFailRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			return
		case "/legacy":
			http.Redirect(w, r, "/guide", http.StatusMovedPermanently)
			return
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><p>Documentation home.</p><a href="/guide">Guide</a><a href="/legacy">Old guide URL</a></body></html>`)
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><p>Connect your account from the settings page.</p></body></html>")
		}
	}))
	defer srv.Close()
	n, err := crawlWithColly(context.Background(), model.SupportContentSource{StartURL: srv.URL, CrawlSource: "links", CrawlDepth: 2}, nil, newTestLogger(t), func(CrawlRecord) error { return nil })
	if err != nil || n != 2 {
		t.Fatalf("canonical duplicate must be deduplicated, not fail refresh: n=%d err=%v", n, err)
	}
}
