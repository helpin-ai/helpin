package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildHelpcenterSearchEntriesCreatesSectionAwareEntries(t *testing.T) {
	excerpt := "Start accepting customer payments without manual review."
	publication := model.DocsHelpcenterArticlePublication{
		DocumentID:  "doc-search-1",
		WorkspaceID: "ws-search-1",
		Locale:      "en",
		Title:       "Payment setup",
		Excerpt:     &excerpt,
		Content: json.RawMessage(`{
			"type":"doc",
			"content":[
				{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Stripe setup"}]},
				{"type":"paragraph","content":[{"type":"text","text":"Connect Stripe and verify the webhook secret."}]},
				{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Stripe setup"}]},
				{"type":"paragraph","content":[{"type":"text","text":"Use test mode before switching live payments on."}]}
			]
		}`),
		ContentText: "Stripe setup Connect Stripe and verify the webhook secret. Stripe setup Use test mode before switching live payments on.",
	}

	entries := BuildHelpcenterSearchEntries(publication)

	if len(entries) != 6 {
		t.Fatalf("len(entries) = %d, want 6: %+v", len(entries), entries)
	}
	if entries[0].EntryType != model.DocsHelpcenterSearchEntryTypeTitle || entries[0].Content != "Payment setup" {
		t.Fatalf("first entry = %+v, want title entry", entries[0])
	}
	if entries[1].EntryType != model.DocsHelpcenterSearchEntryTypeExcerpt || entries[1].Content != excerpt {
		t.Fatalf("second entry = %+v, want excerpt entry", entries[1])
	}

	firstHeading := entries[2]
	if firstHeading.EntryType != model.DocsHelpcenterSearchEntryTypeHeading || firstHeading.Anchor == nil || *firstHeading.Anchor != "stripe-setup" {
		t.Fatalf("first heading = %+v, want anchor stripe-setup", firstHeading)
	}
	firstBody := entries[3]
	if firstBody.EntryType != model.DocsHelpcenterSearchEntryTypeBody || firstBody.SectionTitle == nil || *firstBody.SectionTitle != "Stripe setup" || firstBody.Anchor == nil || *firstBody.Anchor != "stripe-setup" {
		t.Fatalf("first body = %+v, want body attached to first heading", firstBody)
	}

	secondHeading := entries[4]
	if secondHeading.EntryType != model.DocsHelpcenterSearchEntryTypeHeading || secondHeading.Anchor == nil || *secondHeading.Anchor != "stripe-setup-2" {
		t.Fatalf("second heading = %+v, want deduped anchor stripe-setup-2", secondHeading)
	}
	secondBody := entries[5]
	if secondBody.EntryType != model.DocsHelpcenterSearchEntryTypeBody || secondBody.Anchor == nil || *secondBody.Anchor != "stripe-setup-2" {
		t.Fatalf("second body = %+v, want body attached to deduped heading", secondBody)
	}
}

func TestBuildHelpcenterSearchSnippetEscapesHTMLAndMarksTerms(t *testing.T) {
	snippet := BuildHelpcenterSearchSnippet("Reset <script>alert('x')</script> customer passwords safely.", "customer password")

	if snippet != "Reset &lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt; <mark>customer</mark> <mark>password</mark>s safely." {
		t.Fatalf("snippet = %q", snippet)
	}
}
