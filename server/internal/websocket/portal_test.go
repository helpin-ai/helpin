package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ws "nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPortalSocketSlug(t *testing.T) {
	for path, want := range map[string]string{
		"/api/public/portal/acme/ws":       "acme",
		"/api/public/portal/acme/requests": "",
		"/api/public/portal//ws":           "",
		"/api/public/portal/a/b/ws":        "",
		"/api/ws":                          "",
	} {
		got, ok := PortalSocketSlug(path)
		if got != want || ok != (want != "") {
			t.Errorf("PortalSocketSlug(%q) = %q, %v", path, got, ok)
		}
	}
}

func TestPortalPayloadOnlySignalsOwnCustomerVisibleActivity(t *testing.T) {
	scope := &PortalScope{References: map[string]string{"conv-a": "req_a"}}
	on, off := true, false
	for _, tc := range []struct {
		name            string
		event           Event
		customerVisible bool
		want            *portalMessage
	}{
		{"public message", Event{Entity: "support_conversation_message", ParentID: "conv-a"}, true, &portalMessage{Type: "request:changed", Reference: "req_a"}},
		{"internal note", Event{Entity: "support_conversation_message", ParentID: "conv-a"}, false, nil},
		{"another customer's request", Event{Entity: "support_conversation_message", ParentID: "conv-b"}, true, nil},
		{"status change", Event{Entity: "support_conversation", Action: "updated", EntityID: "conv-a"}, false, &portalMessage{Type: "request:changed", Reference: "req_a"}},
		{"agent typing", Event{Entity: "support_conversation", Action: "typing_started", EntityID: "conv-a", ActorID: "user-1"}, false, &portalMessage{Type: "request:typing", Reference: "req_a", Typing: &on}},
		{"agent stopped typing", Event{Entity: "support_conversation", Action: "typing_stopped", EntityID: "conv-a", ActorID: "user-1"}, false, &portalMessage{Type: "request:typing", Reference: "req_a", Typing: &off}},
		{"visitor typing", Event{Entity: "support_conversation", Action: "typing_started", EntityID: "conv-a", ActorID: "widget:s1"}, false, nil},
		{"staff viewing", Event{Entity: "support_conversation", Action: "viewing_started", EntityID: "conv-a", ActorID: "user-1"}, false, nil},
		{"targeted staff event", Event{Entity: "support_conversation", Action: "updated", EntityID: "conv-a", TargetUserID: "user-1"}, false, nil},
		{"AI drafting", Event{Entity: SupportAIResponseStreamEntity, Action: "response_started", ParentID: "conv-a"}, false, &portalMessage{Type: "request:typing", Reference: "req_a", Typing: &on}},
		{"AI done", Event{Entity: SupportAIResponseStreamEntity, Action: "response_completed", ParentID: "conv-a"}, false, &portalMessage{Type: "request:typing", Reference: "req_a", Typing: &off}},
		{"PM event", Event{Entity: "pm_task", Action: "updated", EntityID: "conv-a"}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := portalPayload(scope, tc.event, tc.customerVisible)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("unexpected signal: %s", got)
				}
				return
			}
			want, _ := json.Marshal(tc.want)
			if string(got) != string(want) {
				t.Fatalf("signal = %s, want %s", got, want)
			}
		})
	}
}

// A portal client must never fall through to the internal-client rules.
func TestPortalClientReceivesOnlySignalsOnTheWire(t *testing.T) {
	hub := NewHub()
	ready := make(chan struct{})
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		client := &Client{Conn: conn, WorkspaceID: "ws-1", UserID: "portal:identity", Portal: &PortalScope{References: map[string]string{"conv-a": "req_a"}}}
		hub.Register(client)
		defer hub.Unregister(client)
		close(ready)
		<-done
	}))
	defer srv.Close()
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	<-ready
	note := json.RawMessage(`{"id":"m1","conversation_id":"conv-a","content":"PRIVATE_NOTE","sender_type":"user","message_type":"reply","is_internal":true}`)
	reply := json.RawMessage(`{"id":"m2","conversation_id":"conv-a","content":"PUBLIC_REPLY","sender_type":"user","message_type":"reply"}`)
	go func() {
		hub.Broadcast(Event{Entity: "support_conversation_message", Action: "created", EntityID: "m1", ParentID: "conv-a", WorkspaceID: "ws-1", Data: note})
		hub.Broadcast(Event{Entity: "pm_task", Action: "updated", EntityID: "task-1", WorkspaceID: "ws-1"})
		hub.Broadcast(Event{Entity: "support_conversation_message", Action: "created", EntityID: "m3", ParentID: "conv-b", WorkspaceID: "ws-1", Data: reply})
		hub.Broadcast(Event{Entity: "support_conversation", Action: "viewing_started", EntityID: "conv-a", WorkspaceID: "ws-1", ActorID: "user-1"})
		hub.Broadcast(Event{Entity: "support_conversation_message", Action: "created", EntityID: "m2", ParentID: "conv-a", WorkspaceID: "ws-1", Data: reply})
	}()
	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != `{"type":"request:changed","reference":"req_a"}` {
		t.Fatalf("first portal message = %s", payload)
	}
	for _, leaked := range []string{"PRIVATE_NOTE", "PUBLIC_REPLY", "conv-a", "task-1"} {
		if strings.Contains(string(payload), leaked) {
			t.Fatalf("portal message leaked %q: %s", leaked, payload)
		}
	}
}

type fakePortalAuthenticator struct {
	grant *model.PortalSocketGrant
	err   error
}

func (f fakePortalAuthenticator) AuthenticatePortalSocket(context.Context, string, string) (*model.PortalSocketGrant, error) {
	return f.grant, f.err
}

func TestPortalHandlerAuthenticatesAndChecksOrigin(t *testing.T) {
	grant := &model.PortalSocketGrant{WorkspaceID: "ws-1", IdentityID: "identity", References: map[string]string{"conv-a": "req_a"}}
	cookie := &http.Cookie{Name: portalSessionCookie, Value: "secret"}
	for _, tc := range []struct {
		name     string
		auth     fakePortalAuthenticator
		cookie   *http.Cookie
		origin   string
		wantCode int
	}{
		{"no session", fakePortalAuthenticator{grant: grant}, nil, "https://app.example.com", http.StatusUnauthorized},
		{"invalid session", fakePortalAuthenticator{err: ErrPortalSocketUnauthorized}, cookie, "https://app.example.com", http.StatusUnauthorized},
		{"lookup failure", fakePortalAuthenticator{err: errors.New("database unavailable")}, cookie, "https://app.example.com", http.StatusServiceUnavailable},
		{"foreign origin", fakePortalAuthenticator{grant: grant}, cookie, "https://evil.example", http.StatusForbidden},
		{"portal origin", fakePortalAuthenticator{grant: grant}, cookie, "https://app.example.com", http.StatusSwitchingProtocols},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewPortalHandler(NewHub(), tc.auth, "https://app.example.com")
			srv := httptest.NewServer(handler)
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			header := http.Header{"Origin": []string{tc.origin}}
			if tc.cookie != nil {
				header.Set("Cookie", tc.cookie.String())
			}
			conn, resp, _ := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/public/portal/acme/ws", &ws.DialOptions{HTTPHeader: header})
			if conn != nil {
				defer conn.CloseNow()
			}
			if resp == nil || resp.StatusCode != tc.wantCode {
				code := 0
				if resp != nil {
					code = resp.StatusCode
				}
				t.Fatalf("status = %d, want %d", code, tc.wantCode)
			}
		})
	}
}
