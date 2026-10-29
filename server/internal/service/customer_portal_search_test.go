package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPortalSearchCoversVisibleMessagesOnly(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	conversationID := p.bobConversationID(t)
	add := func(senderType, content string, internal bool, messageType, metadata string) {
		t.Helper()
		p.exec(t, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, is_internal, message_type, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), p.ws.ID, conversationID, senderType, content, internal, messageType, metadata, time.Now())
	}
	add("user", "The **March** totals are fixed in the new export.", false, "reply", "{}")
	add("user", "Secret margin numbers for the team.", true, "note", "{}")
	add("user", "Emailed copy with a hidden remark.", false, "reply", `{"delivery_mode":"email_only"}`)

	results, err := p.svc.SearchRequests(ctx, p.bobIn, "march tot", "mine", "")
	if err != nil || len(results) != 1 || results[0].Reference != p.bobRequest.Reference || results[0].Snippet != "The March totals are fixed in the new export." {
		t.Fatalf("message search = %#v, %v", results, err)
	}
	for _, query := range []string{"secret", "margin", "hidden remark"} {
		if results, err := p.svc.SearchRequests(ctx, p.bobIn, query, "mine", ""); err != nil || len(results) != 0 {
			t.Errorf("%q found non-portal text: %#v, %v", query, results, err)
		}
	}
	if results, err := p.svc.SearchRequests(ctx, p.bobIn, "missing", "mine", ""); err != nil || len(results) != 1 || results[0].Snippet != "" {
		t.Fatalf("subject search = %#v, %v", results, err)
	}
	// A colleague finds it in the company list, never in their own.
	if results, _ := p.svc.SearchRequests(ctx, p.aliceIn, "march", "company", p.acme); len(results) != 1 {
		t.Fatalf("company search = %#v", results)
	}
	if results, _ := p.svc.SearchRequests(ctx, p.aliceIn, "march", "mine", ""); len(results) != 0 {
		t.Fatalf("colleague's own search found Bob's request: %#v", results)
	}
	for _, query := range []string{"m", " "} {
		if _, err := p.svc.SearchRequests(ctx, p.bobIn, query, "mine", ""); !errors.Is(err, ErrPortalSearchInvalid) {
			t.Errorf("query %q error = %v", query, err)
		}
	}
	if _, err := p.svc.SearchRequests(ctx, p.bobIn, "march", "everything", ""); !errors.Is(err, ErrPortalScopeInvalid) {
		t.Fatalf("bad scope error = %v", err)
	}
}

func TestPortalSearchSnippet(t *testing.T) {
	long := "Start " + string(make([]rune, 0)) + "word "
	for i := 0; i < 40; i++ {
		long += "filler "
	}
	long += "needle in the haystack " + long
	snippet := portalSearchSnippet(long, "needle")
	if snippet[:3] != "…" || len([]rune(snippet)) > 2*portalSnippetRadius+portalSnippetRadius+2 {
		t.Fatalf("snippet = %q", snippet)
	}
	if got := portalSearchSnippet("Short text", "missing"); got != "Short text" {
		t.Fatalf("snippet without a hit = %q", got)
	}
}
