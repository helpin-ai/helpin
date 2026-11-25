package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/channels/slack"
)

func TestSlackDraftThreadCoverage(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		count               int
		notice              string
	}{
		{"complete", `{"ok":true,"messages":[{"ts":"1","text":"root"}],"response_metadata":{"next_cursor":"more"}}`, `{"ok":true,"messages":[{"ts":"2","text":"selected"}]}`, 2, ""},
		{"fallback", `{"ok":false,"error":"missing_scope"}`, "", 1, "selected message only"},
		{"partial", `{"ok":true,"messages":[{"ts":"1","text":"root"}],"response_metadata":{"next_cursor":"more"}}`, `{"ok":false,"error":"ratelimited"}`, 2, "part of this thread"},
		{"selected only on partial page", `{"ok":true,"messages":[{"ts":"2","text":"selected"}],"has_more":true}`, "", 1, "selected message only"},
		{"repeated cursor", `{"ok":true,"messages":[{"ts":"1","text":"root"}],"response_metadata":{"next_cursor":"more"}}`, `{"ok":true,"messages":[{"ts":"1","text":"root"}],"response_metadata":{"next_cursor":"more"}}`, 2, "part of this thread"},
		{"no cursor", `{"ok":true,"messages":[{"ts":"1","text":"root"}],"has_more":true}`, "", 2, "part of this thread"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("cursor") == "more" {
					fmt.Fprint(w, tc.second)
				} else {
					fmt.Fprint(w, tc.first)
				}
			}))
			defer ts.Close()
			api := slack.NewClient()
			api.BaseURL = ts.URL + "/"
			history, notice := (&SlackService{api: api}).draftThreadHistory(context.Background(), "token", slack.Message{Channel: "C1", ThreadTS: "1", TS: "2", Text: "selected"})
			if len(history) != tc.count || (tc.notice == "" && notice != "") || !strings.Contains(notice, tc.notice) || history[len(history)-1].Content != "selected" {
				t.Fatalf("history=%+v notice=%q", history, notice)
			}
		})
	}
}

func TestSlackReferencedConversation(t *testing.T) {
	const first = "11111111-1111-4111-8111-111111111111"
	const second = "22222222-2222-4222-8222-222222222222"
	const prefix = "https://app.helpin.ai/w/example/support/"
	for _, tc := range []struct {
		name, text, root, ts, want string
		lookup, fail               bool
	}{
		{"thread follow-up", "What's the latest?", "<" + prefix + first + "|ticket>", "1", first, true, false},
		{"explicit overrides parent", prefix + second, prefix + first, "1", second, false, false},
		{"two conversations", "Summarize this", prefix + first + " " + prefix + second, "1", "", true, true},
		{"adjacent message", "Summarize this", prefix + first, "0", "", true, false},
		{"repeated link", "Summarize this", prefix + first + " " + prefix + first, "1", first, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				q := r.URL.Query()
				if r.URL.Path != "/conversations.history" || q.Get("channel") != "current" || q.Get("latest") != "1" || q.Get("inclusive") != "true" || q.Get("limit") != "1" {
					t.Errorf("unexpected request %s", r.URL)
				}
				fmt.Fprintf(w, `{"ok":true,"messages":[{"ts":%q,"text":%q}]}`, tc.ts, tc.root)
			}))
			defer ts.Close()
			api := slack.NewClient()
			api.BaseURL = ts.URL + "/"
			got, err := (&SlackService{api: api}).referencedConversation(context.Background(), "token", slack.Message{Channel: "original", CurrentChannelID: "current", ThreadTS: "1", TS: "2", Text: tc.text})
			if got != tc.want || (err != nil) != tc.fail || (calls == 1) != tc.lookup {
				t.Fatalf("id=%q error=%v calls=%d", got, err, calls)
			}
		})
	}
}
