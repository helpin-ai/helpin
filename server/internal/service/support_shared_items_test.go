package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestArticlePublicIDFromLink(t *testing.T) {
	tests := []struct {
		name   string
		link   string
		want   string
		wantOK bool
	}{
		{name: "hosted subdomain article", link: "https://acme.helpin.center/reconnecting-instagram-1a2b3c4d", want: "1a2b3c4d", wantOK: true},
		{name: "custom domain with locale and trailing slash", link: "https://help.acme.com/en/reconnecting-instagram-1a2b3c4d/", want: "1a2b3c4d", wantOK: true},
		{name: "query and fragment are ignored", link: "https://help.acme.com/billing-faq-00ff00ff?ref=chat#refunds", want: "00ff00ff", wantOK: true},
		{name: "bare public id", link: "https://help.acme.com/1a2b3c4d", want: "1a2b3c4d", wantOK: true},
		{name: "ordinary page", link: "https://acme.com/pricing", wantOK: false},
		{name: "slug without public id", link: "https://help.acme.com/getting-started", wantOK: false},
		{name: "site root", link: "https://help.acme.com/", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := articlePublicIDFromLink(tt.link)
			if ok != tt.wantOK {
				t.Fatalf("articlePublicIDFromLink(%q) ok = %v, want %v", tt.link, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("articlePublicIDFromLink(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}

func TestTeamRepliesForSharedItems(t *testing.T) {
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "customer", SenderType: "customer", MessageType: "reply", CreatedAt: base},
		{ID: "note", SenderType: "user", MessageType: "reply", IsInternal: true, CreatedAt: base.Add(time.Minute)},
		{ID: "system", SenderType: "user", MessageType: "system", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "teammate", SenderType: "user", MessageType: "reply", CreatedAt: base.Add(3 * time.Minute)},
		{ID: "ai", SenderType: "ai", MessageType: "reply", CreatedAt: base.Add(4 * time.Minute)},
		{ID: "legacy", SenderType: "agent", CreatedAt: base.Add(5 * time.Minute)},
	}

	got := teamRepliesForSharedItems(messages)

	want := []string{"legacy", "ai", "teammate"}
	if len(got) != len(want) {
		t.Fatalf("got %d replies, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("reply %d = %q, want %q (newest first)", i, got[i].ID, id)
		}
	}
}

func TestSharedArticleLinks(t *testing.T) {
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	replies := []model.SupportMessage{
		{ID: "newer", Content: `See [Reconnecting Instagram](https://help.acme.com/reconnecting-instagram-1a2b3c4d).`, CreatedAt: base.Add(time.Hour)},
		{ID: "older", Content: `<p>Try <a href="https://help.acme.com/reconnecting-instagram-1a2b3c4d">this</a> and https://help.acme.com/billing-00ff00ff, or https://acme.com/pricing.</p>`, CreatedAt: base},
	}

	links := sharedArticleLinks(replies)

	if len(links) != 2 {
		t.Fatalf("got %d links, want 2 distinct articles", len(links))
	}
	if links[0].PublicID != "1a2b3c4d" || links[0].Message.ID != "newer" {
		t.Errorf("first link = %s from %s, want 1a2b3c4d from the newest reply", links[0].PublicID, links[0].Message.ID)
	}
	if links[0].URL != "https://help.acme.com/reconnecting-instagram-1a2b3c4d" {
		t.Errorf("first link URL = %q, want trailing punctuation trimmed", links[0].URL)
	}
	if links[1].PublicID != "00ff00ff" {
		t.Errorf("second link = %s, want 00ff00ff", links[1].PublicID)
	}
}
