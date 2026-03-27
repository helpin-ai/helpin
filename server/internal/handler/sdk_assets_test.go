package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKAssetsHandlerServeSDK(t *testing.T) {
	distDir := t.TempDir()

	libPath := filepath.Join(distDir, "lib.js")
	if err := os.WriteFile(libPath, []byte(`console.log("loader");`), 0o644); err != nil {
		t.Fatalf("write lib.js: %v", err)
	}

	hashedPath := filepath.Join(distDir, "helpin.abc123.js")
	if err := os.WriteFile(hashedPath, []byte(`console.log("sdk");`), 0o644); err != nil {
		t.Fatalf("write hashed js: %v", err)
	}

	cssPath := filepath.Join(distDir, "widget.css")
	if err := os.WriteFile(cssPath, []byte(`body{color:black}`), 0o644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	h := NewSDKAssetsHandler(distDir)

	t.Run("serves loader with short cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sdk/lib.js", nil)
		rec := httptest.NewRecorder()

		h.ServeSDK(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
			t.Fatalf("content-type = %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=300" {
			t.Fatalf("cache-control = %q", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("allow-origin = %q", got)
		}
		if body := rec.Body.String(); !strings.Contains(body, `console.log("loader");`) {
			t.Fatalf("body = %q, want loader bundle", body)
		}
	})

	t.Run("serves hashed asset with immutable cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sdk/helpin.abc123.js", nil)
		rec := httptest.NewRecorder()

		h.ServeSDK(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
			t.Fatalf("content-type = %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("cache-control = %q", got)
		}
		if body := rec.Body.String(); !strings.Contains(body, `console.log("sdk");`) {
			t.Fatalf("body = %q, want sdk bundle", body)
		}
	})

	t.Run("serves css with css content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sdk/widget.css", nil)
		rec := httptest.NewRecorder()

		h.ServeSDK(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
			t.Fatalf("content-type = %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("cache-control = %q", got)
		}
	})

	t.Run("rejects missing asset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sdk/missing.js", nil)
		rec := httptest.NewRecorder()

		h.ServeSDK(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if body := rec.Body.String(); !strings.Contains(body, "not found") {
			t.Fatalf("body = %q, want not found", body)
		}
	})

	t.Run("rejects empty asset path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sdk/", nil)
		rec := httptest.NewRecorder()

		h.ServeSDK(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sdk/../secret.js", nil)
		rec := httptest.NewRecorder()

		h.ServeSDK(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if body := rec.Body.String(); !strings.Contains(body, "not found") {
			t.Fatalf("body = %q, want not found", body)
		}
	})
}
