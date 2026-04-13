package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_ViewHeaders(t *testing.T) {
	h := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/view_headers?source=widget", nil)
	req.Host = "api.helpin.test"
	req.RemoteAddr = "203.0.113.5:43123"
	req.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.1")
	req.Header.Set("CF-IPCountry", "US")
	req.Header.Set("X-Vercel-IP-City", "San Francisco")
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=abc")

	rec := httptest.NewRecorder()
	h.ViewHeaders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body headerViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Path != "/view_headers" {
		t.Fatalf("path = %q, want %q", body.Path, "/view_headers")
	}
	if body.Query != "source=widget" {
		t.Fatalf("query = %q, want %q", body.Query, "source=widget")
	}
	if body.ClientIP != "203.0.113.5" {
		t.Fatalf("client_ip = %q, want %q", body.ClientIP, "203.0.113.5")
	}
	if body.ForwardedIP != "198.51.100.8" {
		t.Fatalf("forwarded_ip = %q, want %q", body.ForwardedIP, "198.51.100.8")
	}
	if got := body.SelectedHeaders["Cf-Ipcountry"]; got != "US" {
		t.Fatalf("selected cf-ipcountry = %q, want %q", got, "US")
	}
	if got := body.SelectedHeaders["X-Vercel-Ip-City"]; got != "San Francisco" {
		t.Fatalf("selected x-vercel-ip-city = %q, want %q", got, "San Francisco")
	}

	headers := make(map[string][]string, len(body.Headers))
	for _, entry := range body.Headers {
		headers[entry.Name] = entry.Values
	}
	if got := headers["Authorization"]; len(got) != 1 || got[0] != "[redacted]" {
		t.Fatalf("authorization header = %v, want [redacted]", got)
	}
	if got := headers["Cookie"]; len(got) != 1 || got[0] != "[redacted]" {
		t.Fatalf("cookie header = %v, want [redacted]", got)
	}
	if got := headers["X-Forwarded-For"]; len(got) != 1 || got[0] != "198.51.100.8, 10.0.0.1" {
		t.Fatalf("x-forwarded-for header = %v", got)
	}
}
