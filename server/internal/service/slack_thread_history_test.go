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
