package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteJSONWithETag_EmitsETagAndBody verifies that an initial request
// receives a JSON body along with a strong ETag header computed from the body.
func TestWriteJSONWithETag_EmitsETagAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	writeJSONWithETag(rec, req, http.StatusOK, map[string]string{"name": "hello"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header missing")
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag %q should be quoted", etag)
	}
	if body := strings.TrimSpace(rec.Body.String()); !strings.Contains(body, `"hello"`) {
		t.Errorf("body = %q, expected marshalled payload", body)
	}
}

// TestWriteJSONWithETag_MatchingIfNoneMatchReturns304 verifies that a client
// presenting the current ETag in If-None-Match receives a 304 with no body.
func TestWriteJSONWithETag_MatchingIfNoneMatchReturns304(t *testing.T) {
	payload := map[string]string{"name": "hello"}

	first := httptest.NewRecorder()
	writeJSONWithETag(first, httptest.NewRequest(http.MethodGet, "/test", nil), http.StatusOK, payload)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("If-None-Match", etag)
	writeJSONWithETag(second, req, http.StatusOK, payload)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", second.Code)
	}
	if body := second.Body.String(); body != "" {
		t.Errorf("304 response body should be empty, got %q", body)
	}
	if second.Header().Get("ETag") != etag {
		t.Errorf("304 response should retain ETag, got %q", second.Header().Get("ETag"))
	}
}

// TestWriteJSONWithETag_DifferentPayloadsDifferentETags ensures the ETag changes
// when the payload changes — the primary correctness guarantee for revalidation.
func TestWriteJSONWithETag_DifferentPayloadsDifferentETags(t *testing.T) {
	a := httptest.NewRecorder()
	writeJSONWithETag(a, httptest.NewRequest(http.MethodGet, "/", nil), http.StatusOK, map[string]string{"v": "1"})

	b := httptest.NewRecorder()
	writeJSONWithETag(b, httptest.NewRequest(http.MethodGet, "/", nil), http.StatusOK, map[string]string{"v": "2"})

	if a.Header().Get("ETag") == b.Header().Get("ETag") {
		t.Errorf("expected distinct ETags for distinct payloads, both were %q", a.Header().Get("ETag"))
	}
}

// TestWriteJSONWithETag_MismatchingIfNoneMatchSendsFullResponse verifies that a
// stale If-None-Match does not short-circuit; the client receives fresh data.
func TestWriteJSONWithETag_MismatchingIfNoneMatchSendsFullResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `"stale-etag-value"`)

	writeJSONWithETag(rec, req, http.StatusOK, map[string]string{"name": "fresh"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "fresh") {
		t.Errorf("body = %q, expected fresh payload", body)
	}
}

// TestNoStoreOnWrites_SetsHeaderForNonIdempotentMethods verifies that the
// middleware emits Cache-Control: no-store for POST/PUT/PATCH/DELETE and
// leaves GET/HEAD/OPTIONS untouched so their own policies can apply.
func TestNoStoreOnWrites_SetsHeaderForNonIdempotentMethods(t *testing.T) {
	passthrough := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := NoStoreOnWrites(passthrough)

	cases := []struct {
		method   string
		wantHdr  string
	}{
		{http.MethodGet, ""},
		{http.MethodHead, ""},
		{http.MethodOptions, ""},
		{http.MethodPost, "no-store"},
		{http.MethodPut, "no-store"},
		{http.MethodPatch, "no-store"},
		{http.MethodDelete, "no-store"},
	}
	for _, c := range cases {
		t.Run(c.method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, httptest.NewRequest(c.method, "/", nil))
			if got := rec.Header().Get("Cache-Control"); got != c.wantHdr {
				t.Errorf("Cache-Control = %q, want %q", got, c.wantHdr)
			}
		})
	}
}
