package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressJSON(t *testing.T) {
	t.Parallel()

	const body = `{"tasks":[{"id":"task-1","name":"Compressed task response"}]}`
	handler := CompressJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))

	t.Run("compresses JSON when the client accepts gzip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		if got := res.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding = %q, want gzip", got)
		}
		reader, err := gzip.NewReader(res.Body)
		if err != nil {
			t.Fatalf("open gzip body: %v", err)
		}
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read gzip body: %v", err)
		}
		if err := reader.Close(); err != nil {
			t.Fatalf("close gzip reader: %v", err)
		}
		if string(decompressed) != body {
			t.Fatalf("decompressed body = %q, want %q", decompressed, body)
		}
		if vary := res.Header().Values("Vary"); !strings.Contains(strings.Join(vary, ","), "Accept-Encoding") {
			t.Fatalf("Vary = %v, want Accept-Encoding", vary)
		}
	})

	t.Run("leaves JSON uncompressed when gzip is not accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		if got := res.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
		if res.Body.String() != body {
			t.Fatalf("body = %q, want %q", res.Body.String(), body)
		}
	})
}
