package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/auth"
	ws "nhooyr.io/websocket"
)

func TestHandlerVisitorSnapshots(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		trigger string
	}{
		{name: "empty_initial_snapshot"},
		{name: "heartbeat_reconciles_expired_visitor", trigger: `{"type":"support:ping","data":{}}`},
		{name: "opening_conversation_reconciles_expired_visitor", trigger: `{"type":"support:viewing:start","data":{"conversation_id":"conv-1"}}`},
	} {
		initialOnline := scenario.trigger != ""
		name := scenario.name
		t.Run(name, func(t *testing.T) {
			presence, redis := setupRedisPresence(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if initialOnline {
				if err := presence.SetVisitorOnline(ctx, "ws-1", "visitor-1", "conn-1"); err != nil {
					t.Fatal(err)
				}
			}
			hub := NewHub()
			hub.SetPresenceProvider(presence)
			jwt := auth.NewJWTManager("test-secret")
			token, _, err := jwt.GenerateTokenPair("user-1", "test@example.com", false)
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			handler := NewHandler(hub, jwt)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(done)
				handler.ServeHTTP(w, r)
			}))
			defer server.Close()
			conn, _, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"?workspace_id=ws-1&token="+token, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				conn.CloseNow()
				<-done
			}()
			readVisitors := func() []string {
				t.Helper()
				for {
					_, raw, err := conn.Read(ctx)
					if err != nil {
						t.Fatalf("read visitor snapshot: %v", err)
					}
					var message struct {
						Type string `json:"type"`
						Data struct {
							Visitors []string `json:"visitors"`
						} `json:"data"`
					}
					if err := json.Unmarshal(raw, &message); err != nil {
						t.Fatal(err)
					}
					if message.Type == "support:online_visitors" {
						if message.Data.Visitors == nil {
							t.Fatal("visitor list must be an array, including when empty")
						}
						return message.Data.Visitors
					}
				}
			}
			visitors := readVisitors()
			if !initialOnline {
				if len(visitors) != 0 {
					t.Fatalf("expected empty snapshot, got %v", visitors)
				}
				return
			}
			if len(visitors) != 1 || visitors[0] != "visitor-1" {
				t.Fatalf("unexpected initial visitors: %v", visitors)
			}
			redis.FastForward(visitorConnTTL + time.Second)
			if err := conn.Write(ctx, ws.MessageText, []byte(scenario.trigger)); err != nil {
				t.Fatal(err)
			}
			if visitors := readVisitors(); len(visitors) != 0 {
				t.Fatalf("expired visitor still online: %v", visitors)
			}
		})
	}
}
