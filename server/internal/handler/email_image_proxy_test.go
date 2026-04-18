package handler

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateImageURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"https ok", "https://cdn.example.com/pic.png", false},
		{"http ok", "http://cdn.example.com/pic.png", false},
		{"ftp rejected", "ftp://files.example.com/pic.png", true},
		{"javascript rejected", "javascript:alert(1)", true},
		{"empty host rejected", "https:///pic.png", true},
		{"empty rejected", "", true},
		{"malformed rejected", "://", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateImageURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateImageURL(%q) err = %v, wantErr = %v", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestIsDisallowedIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.5", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"169.254.169.254", true}, // AWS metadata — classic SSRF target
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false}, // example.com
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isDisallowedIP(net.ParseIP(tt.ip))
			if got != tt.want {
				t.Errorf("isDisallowedIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestEmailImageProxyBadURL(t *testing.T) {
	h := NewEmailImageProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/email/image-proxy?url=ftp://x.example.com/a.png", nil)
	w := httptest.NewRecorder()
	h.Proxy(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmailImageProxyMissingURL(t *testing.T) {
	h := NewEmailImageProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/email/image-proxy", nil)
	w := httptest.NewRecorder()
	h.Proxy(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "url query parameter") {
		t.Errorf("expected helpful error, got %q", w.Body.String())
	}
}

func TestEmailImageProxyForwardsImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	}))
	t.Cleanup(upstream.Close)

	// Upstream is 127.0.0.1 which SSRF guard blocks — expected.
	h := NewEmailImageProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/email/image-proxy?url="+upstream.URL, nil)
	w := httptest.NewRecorder()
	h.Proxy(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (loopback blocked by SSRF guard), got %d: %s", w.Code, w.Body.String())
	}
}

func TestEmailImageProxyRejectsNonImageContentType(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<script>alert(1)</script>"))
	}))
	t.Cleanup(upstream.Close)

	// Same loopback SSRF guard — this confirms the guard fires first before
	// the content-type check, which is the desired ordering.
	h := NewEmailImageProxyHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/email/image-proxy?url="+upstream.URL, nil)
	w := httptest.NewRecorder()
	h.Proxy(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}
