package geoip

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenDisabled(t *testing.T) {
	if got := Open(Options{}); got != nil {
		t.Fatal("expected no service when path is empty")
	}
}

func TestOpenUsesExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "city.mmdb")
	if err := os.WriteFile(path, testDatabase(t), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Open(Options{Path: path, DownloadURL: "https://unused.test"})
	t.Cleanup(func() { closeService(t, s) })
	assertCountry(t, s)
}

func TestOpenRecoversFromDownloadFailure(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   []byte
	}{
		{name: "server error", status: http.StatusServiceUnavailable, body: []byte("unavailable")},
		{name: "corrupt database", status: http.StatusOK, body: []byte("invalid database")},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var attempts atomic.Int32
			fixture := testDatabase(t)
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				status, body := http.StatusOK, fixture
				if attempts.Add(1) == 1 {
					status, body = scenario.status, scenario.body
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
			})}
			s := openWithRetryInterval(Options{
				Path:        filepath.Join(t.TempDir(), "city.mmdb"),
				DownloadURL: "https://download.test/city.mmdb", HTTPClient: client,
			}, 5*time.Millisecond)
			t.Cleanup(func() { closeService(t, s) })
			deadline := time.After(3 * time.Second)
			for {
				result, err := s.Lookup(netip.MustParseAddr("1.2.3.4"))
				if err != nil {
					t.Fatal(err)
				}
				if result != nil {
					assertCountry(t, s)
					break
				}
				select {
				case <-deadline:
					t.Fatal("GeoIP did not recover")
				case <-time.After(time.Millisecond):
				}
			}
			if got := attempts.Load(); got != 2 {
				t.Fatalf("got %d downloads, want 2", got)
			}
		})
	}
}

func TestOpenDoesNotWaitForDownload(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
		return nil, r.Context().Err()
	})}
	s := Open(Options{Path: filepath.Join(t.TempDir(), "city.mmdb"), DownloadURL: "https://download.test", HTTPClient: client})
	t.Cleanup(func() { closeService(t, s) })
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start")
	}
	if result, err := s.Lookup(netip.MustParseAddr("1.2.3.4")); result != nil || err != nil {
		t.Fatalf("unavailable lookup = %v, %v", result, err)
	}
	closeService(t, s)
	select {
	case <-cancelled:
	default:
		t.Fatal("Close did not cancel the download")
	}
}

func TestCloseStopsRateLimitedRetries(t *testing.T) {
	var attempts atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString("Daily GeoIP database download limit reached"))}, nil
	})}
	s := Open(Options{Path: filepath.Join(t.TempDir(), "city.mmdb"), DownloadURL: "https://download.test", HTTPClient: client})
	t.Cleanup(func() { closeService(t, s) })
	deadline := time.After(3 * time.Second)
	for attempts.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("download did not start")
		case <-time.After(time.Millisecond):
		}
	}
	closeService(t, s)
	if got := attempts.Load(); got != 1 {
		t.Fatalf("got %d downloads, want 1", got)
	}
}

func TestRetryDelay(t *testing.T) {
	now := time.Date(2026, 9, 7, 21, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name string
		err  error
		want time.Duration
	}{
		{"network failure", errors.New("network down"), time.Hour},
		{"daily limit", &downloadError{status: 429}, 12 * time.Hour},
		{"wrapped limit", fmt.Errorf("bootstrap: %w", &downloadError{status: 429}), 12 * time.Hour},
		{"long retry after", &downloadError{status: 429, retryAfter: "86400"}, 24 * time.Hour},
		{"short retry after", &downloadError{status: 429, retryAfter: "60"}, 12 * time.Hour},
		{"date retry after", &downloadError{status: 503, retryAfter: now.Add(2 * time.Hour).Format(http.TimeFormat)}, 2 * time.Hour},
		{"invalid retry after", &downloadError{status: 429, retryAfter: "invalid"}, 12 * time.Hour},
		{"negative retry after", &downloadError{status: 429, retryAfter: "-1"}, 12 * time.Hour},
		{"past retry after", &downloadError{status: 503, retryAfter: now.Add(-time.Hour).Format(http.TimeFormat)}, time.Hour},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryDelay(tt.err, time.Hour, now); got != tt.want {
				t.Fatalf("retry delay = %v, want %v", got, tt.want)
			}
		})
	}
}

func closeService(t *testing.T, s *Service) {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
}

func assertCountry(t *testing.T, s *Service) {
	t.Helper()
	result, err := s.Lookup(netip.MustParseAddr("1.2.3.4"))
	if err != nil || result == nil || result.CountryCode != "US" {
		t.Fatalf("lookup = %+v, %v; want US", result, err)
	}
}

func testDatabase(t *testing.T) []byte {
	t.Helper()
	// Synthetic MMDB: one 24-bit node, both branches point to a US country record.
	// No licensed database or external download is needed for these tests.
	db := append([]byte{0, 0, 17, 0, 0, 17}, make([]byte, 16)...)
	db = append(db, []byte("\xe1\x47country\xe1\x48iso_code\x42US")...)
	db = append(db, []byte("\xab\xcd\xefMaxMind.com\xe3\x4anode_count\xc1\x01\x4brecord_size\xa1\x18\x4aip_version\xa1\x04")...)
	return db
}
