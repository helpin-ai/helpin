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
		{"config", "GET", "/config?widget_key=key", "", "https://site.example", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"installation", "GET", "/settings/install", "", "https://site.example", "", 200, widgetorigin.Reference{InstallationID: "install"}},
		{"identify", "POST", "/identify", `{"api_key":"key","email":"a@example.com"}`, "https://site.example", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"session", "POST", "/session", `{"widget_key":"key"}`, "https://site.example", "", 200, widgetorigin.Reference{WidgetKey: "key"}},
		{"message", "POST", "/messages", `{"session_token":"token","content":"hello"}`, "https://site.example", "", 200, widgetorigin.Reference{SessionToken: "token"}},
		{"attachment", "PATCH", "/attachments/one", `{}`, "https://site.example", "token", 200, widgetorigin.Reference{SessionToken: "token"}},
		{"denied origin", "POST", "/session", `{"widget_key":"key"}`, "https://other.example", "", 403, widgetorigin.Reference{WidgetKey: "key"}},
		{"no origin", "GET", "/config?widget_key=key", "", "", "", 403, widgetorigin.Reference{}},
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
				if !widgetorigin.Allowed(origin, []string{"https://site.example"}) {
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
			if w.Code != tc.status || called != (tc.status == 200) {
				t.Fatalf("status=%d handler called=%v", w.Code, called)
			}
		})
	}
}
