package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessagePermalinkFallback(t *testing.T) {
	for _, tc := range []struct{ name, permalink, code string }{
		{name: "missing message", code: "message_not_found"},
		{name: "bot lost access", code: "not_in_channel"},
		{name: "another site", permalink: "https://slack.com.example.org/archives/C1/p101"},
		{name: "different message", permalink: "https://acme.slack.com/archives/C1/p102"},
		{name: "non-archive path", permalink: "https://acme.slack.com/redirect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"ok": tc.code == "", "permalink": tc.permalink, "error": tc.code})
			}))
			defer fake.Close()
			c := NewClient()
			c.BaseURL = fake.URL + "/"
			if link, err := c.MessagePermalink(context.Background(), "fixture", "C1", "10.1"); err != nil || link != "" {
				t.Fatalf("unexpected source %q, error %v", link, err)
			}
		})
	}
}
