package requestmeta

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractClientIP_CloudflarePrefersCFConnectingIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "162.158.95.197:443"
	req.Header.Set("CF-Connecting-IP", "2a01:4f8:c014:8425::1")
	req.Header.Set("CF-Ray", "9ebd2b146dcd3a97-FRA")
	req.Header.Set("X-Forwarded-For", "162.158.95.197")
	req.Header.Set("X-Real-IP", "162.158.95.197")

	clientIP, ok := ExtractClientIP(req)
	if !ok {
		t.Fatal("expected client ip")
	}
	if got := clientIP.Addr.String(); got != "2a01:4f8:c014:8425::1" {
		t.Fatalf("addr = %q, want %q", got, "2a01:4f8:c014:8425::1")
	}
	if clientIP.Source != "cf-connecting-ip" {
		t.Fatalf("source = %q, want %q", clientIP.Source, "cf-connecting-ip")
	}
}

func TestExtractClientIP_CloudflareFallsBackToOriginalForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "162.158.95.197:443"
	req.Header.Set("CF-Ray", "9ebd2b146dcd3a97-FRA")
	req.Header.Set("X-Original-Forwarded-For", "2a01:4f8:c014:8425::1")
	req.Header.Set("X-Forwarded-For", "162.158.95.197")
	req.Header.Set("X-Real-IP", "162.158.95.197")

	clientIP, ok := ExtractClientIP(req)
	if !ok {
		t.Fatal("expected client ip")
	}
	if got := clientIP.Addr.String(); got != "2a01:4f8:c014:8425::1" {
		t.Fatalf("addr = %q, want %q", got, "2a01:4f8:c014:8425::1")
	}
	if clientIP.Source != "x-original-forwarded-for" {
		t.Fatalf("source = %q, want %q", clientIP.Source, "x-original-forwarded-for")
	}
}

func TestExtractClientIP_NonCloudflareUsesForwardedProxyHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:43123"
	req.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.1")
	req.Header.Set("X-Real-IP", "198.51.100.8")

	clientIP, ok := ExtractClientIP(req)
	if !ok {
		t.Fatal("expected client ip")
	}
	if got := clientIP.Addr.String(); got != "198.51.100.8" {
		t.Fatalf("addr = %q, want %q", got, "198.51.100.8")
	}
	if clientIP.Source != "x-real-ip" {
		t.Fatalf("source = %q, want %q", clientIP.Source, "x-real-ip")
	}
}
