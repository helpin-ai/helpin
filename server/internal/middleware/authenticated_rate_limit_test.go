package middleware

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/helpin-ai/helpin/server/internal/ratelimit"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedRateLimit(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	handler := AuthenticatedRateLimit(ratelimit.New(client, ratelimit.Config{RequestsPerMinute: 8, ExpensivePerMinute: 1}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dock/runs/1/events" {
			f, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("lost Flusher")
			}
			f.Flush()
			return
		}
		w.WriteHeader(204)
	}))
	request := func(user, method, path, workspace string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		r = r.WithContext(WithUserID(r.Context(), user))
		r.Header.Set("X-Workspace-ID", workspace)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if got := request("alice", "POST", "/api/pm/agent-runs", "ws1").Code; got != 204 {
		t.Fatal(got)
	}
	denied := request("alice", "POST", "/api/pm/agent-runs", "forged-workspace")
	if denied.Code != 429 || denied.Header().Get("Retry-After") != "60" || denied.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("denial: %d %v", denied.Code, denied.Header())
	}
	if got := request("bob", "POST", "/api/pm/agent-runs", "ws1").Code; got != 204 {
		t.Fatalf("another user blocked: %d", got)
	}
	if got := request("alice", "GET", "/api/dock/runs/1/events", "ws1").Code; got != 200 {
		t.Fatalf("stream blocked: %d", got)
	}
	for i := 0; i < 5; i++ {
		if got := request("alice", "GET", "/api/notifications", "").Code; got != 204 {
			t.Fatal(got)
		}
	}
	if got := request("alice", "GET", "/api/notifications", "").Code; got != 429 {
		t.Fatalf("general ceiling: %d", got)
	}
}
func TestExpensiveAPIRequest(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{"POST", "/api/pm/agent-runs", true}, {"POST", "/api/dock/chats/1/messages", true},
		{"POST", "/api/pm/content-sources/1/reindex", true}, {"POST", "/api/support/inbox/rewrite-draft", true},
		{"GET", "/api/pm/agent-runs", false}, {"POST", "/api/dock/runs/1/cancel", false},
		{"POST", "/api/support/inbox/conversations/1/messages", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			if expensiveAPIRequest(httptest.NewRequest(tc.method, tc.path, nil)) != tc.want {
				t.Fatal("incorrect operation class")
			}
		})
	}
}
