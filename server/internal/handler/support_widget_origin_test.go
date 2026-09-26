package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

type testWidgetOriginAuthorizer func(context.Context, string, widgetorigin.Reference) error

func (f testWidgetOriginAuthorizer) AuthorizeWidgetOrigin(ctx context.Context, origin string, ref widgetorigin.Reference) error {
	return f(ctx, origin, ref)
}

func TestWidgetOriginMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, origin, header string
		status                                   int
		ref                                      widgetorigin.Reference
	}{
		{"tauri config", "GET", "/config?widget_key=key", "", "tauri://localhost", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"tauri identify", "POST", "/identify", `{"api_key":"key"}`, "tauri://localhost", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"opaque", "GET", "/config?widget_key=key", "", "null", "", 403, widgetorigin.Reference{WidgetKey: "key"}},
		{"config", "GET", "/config?widget_key=key", "", "https://site.example", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"installation", "GET", "/settings/install", "", "https://site.example", "", 200, widgetorigin.Reference{InstallationID: "install"}},
		{"identify", "POST", "/identify", `{"api_key":"key","email":"a@example.com"}`, "https://site.example", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"session", "POST", "/session", `{"widget_key":"key"}`, "https://site.example", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"history", "GET", "/messages?session_token=token", "", "https://site.example", "", 200, widgetorigin.Reference{SessionToken: "token"}},
		{"message", "POST", "/messages", `{"session_token":"token","content":"hello"}`, "https://site.example", "", 200, widgetorigin.Reference{SessionToken: "token"}},
		{"attachment", "PATCH", "/attachments/one", `{}`, "https://site.example", "token", 200, widgetorigin.Reference{SessionToken: "token"}},
		{"denied origin", "POST", "/session", `{"widget_key":"key"}`, "https://other.example", "", 403, widgetorigin.Reference{WidgetKey: "key"}},
		{"no origin", "GET", "/config?widget_key=key", "", "", "", 403, widgetorigin.Reference{WidgetKey: "key"}},
		{"mixed tokens", "POST", "/messages?session_token=other", `{"session_token":"token"}`, "https://site.example", "", 400, widgetorigin.Reference{}},
		{"duplicate keys", "GET", "/config?widget_key=key&widget_key=other", "", "https://site.example", "", 400, widgetorigin.Reference{}},
		{"malformed body", "POST", "/messages", `{`, "https://site.example", "", 400, widgetorigin.Reference{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			authorizer := testWidgetOriginAuthorizer(func(_ context.Context, origin string, ref widgetorigin.Reference) error {
				if ref != tc.ref {
					t.Fatalf("reference = %#v, want %#v", ref, tc.ref)
				}
				if !widgetorigin.Allowed(origin, []string{"https://site.example", "tauri://localhost"}) {
					return fmt.Errorf("denied")
				}
				return nil
			})
			router := chi.NewRouter()
			handler := requireWidgetOrigin(authorizer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Body != nil {
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != tc.body {
						t.Fatalf("request body changed: %q, %v", body, err)
					}
				}
				w.WriteHeader(200)
			}))
			router.Handle("/settings/{id}", handler)
			router.Handle("/*", handler)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.header != "" {
				r.Header.Set("X-Session-Token", tc.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if tc.status == 200 {
				if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.origin {
					t.Fatalf("CORS origin = %q, want %q", got, tc.origin)
				}
				wantCache := ""
				if tc.method != "GET" || tc.ref.SessionToken != "" {
					wantCache = "no-store"
				}
				if got := w.Header().Get("Cache-Control"); got != wantCache {
					t.Fatalf("cache policy %q, want %q", got, wantCache)
				}
			}
			if w.Code != tc.status || called != (tc.status == 200) {
				t.Fatalf("status=%d handler called=%v", w.Code, called)
			}
		})
	}
}

func TestWidgetOriginSameOriginGETStillRequiresInstallationAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, site, mode, host string
		allowed                bool
		want                   int
	}{
		{"browser same origin", "same-origin", "cors", "widget.example.test", true, 200},
		{"not allowed installation", "same-origin", "cors", "widget.example.test", false, 403},
		{"cross site", "cross-site", "cors", "widget.example.test", true, 403},
		{"navigation", "same-origin", "navigate", "widget.example.test", true, 403},
		{"unknown host", "same-origin", "cors", "other.example.test", true, 403},
		{"no metadata", "", "", "widget.example.test", true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := testWidgetOriginAuthorizer(func(_ context.Context, origin string, _ widgetorigin.Reference) error {
				if !tc.allowed || origin != "https://widget.example.test" {
					return fmt.Errorf("denied")
				}
				return nil
			})
			h := requireWidgetOrigin(auth, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }), "https://widget.example.test")
			r := httptest.NewRequest("GET", "https://"+tc.host+"/widget/config?widget_key=key", nil)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			r.Header.Set("Sec-Fetch-Mode", tc.mode)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

func TestWidgetOriginTauriCORS(t *testing.T) {
	authorizer := testWidgetOriginAuthorizer(func(_ context.Context, origin string, _ widgetorigin.Reference) error {
		if !widgetorigin.Allowed(origin, []string{"tauri://localhost"}) {
			return fmt.Errorf("denied")
		}
		return nil
	})
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{AllowedOrigins: []string{"*"}, AllowedMethods: []string{"GET", "OPTIONS"}, AllowedHeaders: []string{"Content-Type", "X-Session-Token"}}))
	router.Handle("/config", requireWidgetOrigin(authorizer, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })))
	for _, method := range []string{"OPTIONS", "GET"} {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/config?widget_key=key", nil)
			r.Header.Set("Origin", "tauri://localhost")
			if method == "OPTIONS" {
				r.Header.Set("Access-Control-Request-Method", "GET")
				r.Header.Set("Access-Control-Request-Headers", "X-Session-Token")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("CORS status %d", w.Code)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "tauri://localhost" && got != "*" {
				t.Fatalf("CORS origin %q", got)
			}
		})
	}
}

func TestWidgetOriginAllowAll(t *testing.T) {
	for _, origin := range []string{"", "null", "https://other.example"} {
		t.Run(origin, func(t *testing.T) {
			auth := testWidgetOriginAuthorizer(func(_ context.Context, value string, ref widgetorigin.Reference) error {
				if ref.WidgetKey != "key" || !widgetorigin.Allowed(value, []string{"*"}) {
					return fmt.Errorf("denied")
				}
				return nil
			})
			h := requireWidgetOrigin(auth, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
			r := httptest.NewRequest("GET", "/config?widget_key=key", nil)
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("status %d", w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatal("incorrect CORS origin")
			}
			r.Header.Add("Origin", "https://one.example")
			r.Header.Add("Origin", "https://two.example")
			w = httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatal("multiple origin headers accepted")
			}
		})
	}
}
