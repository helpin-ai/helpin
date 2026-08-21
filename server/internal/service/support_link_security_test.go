package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeSupportLinkURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		trim    bool
		want    string
		wantErr bool
	}{
		{name: "canonical case and default port", raw: "HTTPS://Example.COM:443", want: "https://example.com/"},
		{name: "keeps non-default port query and encoding", raw: "http://Example.com:8080/a%2Fb?q=One%20Two#part", want: "http://example.com:8080/a%2Fb?q=One%20Two"},
		{name: "punycode host", raw: "https://bücher.example/path", want: "https://xn--bcher-kva.example/path"},
		{name: "trims sentence punctuation", raw: "https://example.com/a).", trim: true, want: "https://example.com/a"},
		{name: "keeps balanced parentheses", raw: "https://example.com/a_(b)", trim: true, want: "https://example.com/a_(b)"},
		{name: "keeps encoded parenthesis", raw: "https://example.com/?q=a%29", trim: true, want: "https://example.com/?q=a%29"},
		{name: "rejects credentials", raw: "https://user:pass@example.com", wantErr: true},
		{name: "rejects other schemes", raw: "ftp://example.com/file", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSupportLinkURL(tt.raw, tt.trim)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalized URL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGoogleWebRiskClientScansAndCaches(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
			t.Errorf("API key header = %q", got)
		}
		query := r.URL.Query()
		if query.Get("uri") != "https://bad.example/path" {
			t.Errorf("uri = %q", query.Get("uri"))
		}
		if got := query["threatTypes"]; len(got) != 3 {
			t.Errorf("threatTypes = %#v", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"threat":{"threatTypes":["SOCIAL_ENGINEERING","MALWARE"],"expireTime":"2026-08-18T12:30:00Z"}}`)),
			Request:    r,
		}, nil
	})}

	client := newGoogleWebRiskClient("test-key", "https://webrisk.test/v1/uris:search", httpClient, func() time.Time { return now })
	first := client.Scan(context.Background(), "https://bad.example/path")
	second := client.Scan(context.Background(), "https://bad.example/path")

	if first.Status != supportLinkSecurityMalicious || second.Status != supportLinkSecurityMalicious {
		t.Fatalf("statuses = %q, %q", first.Status, second.Status)
	}
	if strings.Join(first.ThreatTypes, ",") != "MALWARE,SOCIAL_ENGINEERING" {
		t.Fatalf("threat types = %#v", first.ThreatTypes)
	}
	if calls.Load() != 1 {
		t.Fatalf("lookup calls = %d, want 1", calls.Load())
	}
}

func TestGoogleWebRiskClientFailureIsUnknown(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Request:    r,
		}, nil
	})}
	client := newGoogleWebRiskClient("test-key", "https://webrisk.test/v1/uris:search", httpClient, time.Now)
	got := client.Scan(context.Background(), "https://example.com/")
	if got.Status != supportLinkSecurityUnknown {
		t.Fatalf("status = %q, want unknown", got.Status)
	}
}

func TestGoogleWebRiskClientNoKeyDoesNotCallProvider(t *testing.T) {
	client := newGoogleWebRiskClient("", "http://unused.invalid", &http.Client{}, time.Now)
	got := client.Scan(context.Background(), "https://example.com/")
	if got.Status != supportLinkSecurityUnknown {
		t.Fatalf("status = %q, want unknown", got.Status)
	}
}

func TestGoogleWebRiskClientUsesRepeatedThreatTypeParameters(t *testing.T) {
	requestURL, err := url.Parse(googleWebRiskLookupEndpoint)
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	query := googleWebRiskLookupQuery("https://example.com/")
	requestURL.RawQuery = query.Encode()
	if len(requestURL.Query()["threatTypes"]) != 3 {
		t.Fatalf("query = %q", requestURL.RawQuery)
	}
}
