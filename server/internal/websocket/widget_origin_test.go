package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	ws "nhooyr.io/websocket"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

// Embedding the unused methods makes any accidental session/history operation
// panic: denied handshakes must do nothing beyond read-only authorization.
type deniedWidgetOriginService struct {
	WidgetService
	ref widgetorigin.Reference
}

func (s *deniedWidgetOriginService) AuthorizeWidgetOrigin(_ context.Context, _ string, ref widgetorigin.Reference) error {
	s.ref = ref
	return fmt.Errorf("origin denied")
}

func TestWidgetSocketRejectsOriginBeforeUpgrade(t *testing.T) {
	for _, tc := range []struct {
		query string
		ref   widgetorigin.Reference
	}{
		{"key=key", widgetorigin.Reference{WidgetKey: "key"}},
		{"session_token=legacy", widgetorigin.Reference{SessionToken: "legacy"}},
		{"key=key&session_token=legacy", widgetorigin.Reference{WidgetKey: "key", SessionToken: "legacy"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			svc := &deniedWidgetOriginService{}
			r := httptest.NewRequest("GET", "/widget/ws?"+tc.query, nil)
			r.Header.Set("Origin", "https://third.example")
			w := httptest.NewRecorder()
			NewWidgetHandler(nil, svc).ServeHTTP(w, r)
			if w.Code != 403 || svc.ref != tc.ref {
				t.Fatalf("status=%d reference=%#v", w.Code, svc.ref)
			}
		})
	}
}

// Embedding the interface keeps the handshake test strict: unexpected service
// operations panic rather than silently bypassing session admission.
type tauriWidgetOriginService struct{ WidgetService }

func (s *tauriWidgetOriginService) AuthorizeWidgetOrigin(_ context.Context, origin string, _ widgetorigin.Reference) error {
	if !widgetorigin.Allowed(origin, []string{"tauri://localhost", "http://tauri.localhost", "https://tauri.localhost"}) {
		return &widgetorigin.DeniedError{InstallationID: "installation-one"}
	}
	return nil
}
func (s *tauriWidgetOriginService) GetWidgetSession(_ context.Context, token string) (*model.SupportWidgetSession, error) {
	return &model.SupportWidgetSession{ID: "session-one", WorkspaceID: "workspace-one", SessionToken: token}, nil
}
func (s *tauriWidgetOriginService) GetVisitorConversations(context.Context, string, string) ([]model.SupportConversation, error) {
	return nil, nil
}
func (s *tauriWidgetOriginService) TouchWidgetSessionActivity(context.Context, string) error {
	return nil
}

func TestWidgetSocketAllowsTauriHandshake(t *testing.T) {
	for _, query := range []string{"key=key", "session_token=legacy"} {
		for _, origin := range []string{"tauri://localhost", "http://tauri.localhost", "https://tauri.localhost"} {
			t.Run(query+"/"+origin, func(t *testing.T) {
				done := make(chan struct{})
				handler := NewWidgetHandler(NewHub(), &tauriWidgetOriginService{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer close(done)
					handler.ServeHTTP(w, r)
				}))
				defer srv.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				conn, resp, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/widget/ws?"+query, &ws.DialOptions{HTTPHeader: http.Header{"Origin": []string{origin}}})
				if err != nil {
					t.Fatalf("handshake failed: %v", err)
				}
				if resp.StatusCode != http.StatusSwitchingProtocols {
					t.Fatalf("status %d", resp.StatusCode)
				}
				if err := conn.Close(ws.StatusNormalClosure, "test complete"); err != nil {
					t.Fatal(err)
				}
				select {
				case <-done:
				case <-ctx.Done():
					t.Fatal("handler did not exit")
				}
			})
		}
	}
}

func TestWidgetSocketRejectsMalformedTauriOrigins(t *testing.T) {
	for _, query := range []string{"key=key", "session_token=legacy"} {
		for _, origins := range [][]string{nil, {"null"}, {"tauri://localhost.evil"}, {"tauri://localhost/path"}, {"tauri://localhost", "https://other.example"}} {
			t.Run(query+"/"+strings.Join(origins, ","), func(t *testing.T) {
				r := httptest.NewRequest("GET", "/widget/ws?"+query, nil)
				for _, origin := range origins {
					r.Header.Add("Origin", origin)
				}
				w := httptest.NewRecorder()
				NewWidgetHandler(nil, &tauriWidgetOriginService{}).ServeHTTP(w, r)
				if w.Code != http.StatusForbidden {
					t.Fatalf("status %d", w.Code)
				}
			})
		}
	}
}
