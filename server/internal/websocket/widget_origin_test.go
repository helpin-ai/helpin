package websocket

import (
	"context"
	"fmt"
	"net/http/httptest"
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
