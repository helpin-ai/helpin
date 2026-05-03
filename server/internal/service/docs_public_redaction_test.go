package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestRedactPublicDocsContentRemovesEntityReferenceDetails(t *testing.T) {
	content := &model.DocsContent{
		ID:         "content-1",
		DocumentID: "doc-1",
		Content: json.RawMessage(`{
			"type":"doc",
			"content":[
				{"type":"paragraph","content":[
					{"type":"text","text":"Ask "},
					{"type":"entityMention","attrs":{"entityType":"contact","entityId":"contact-1","label":"Ada Lovelace","displayId":"CON-1","href":"/w/acme/crm/contacts/contact-1"}}
				]},
				{"type":"entityEmbed","attrs":{"entityType":"task","entityId":"task-1","title":"Fix billing sync","displayId":"TASK-7","status":"open","href":"/w/acme/pm/tasks/task-1"}}
			]
		}`),
		ContentText: "Ask Ada Lovelace Fix billing sync",
		WordCount:   6,
	}

	redacted := RedactPublicDocsContent(content)
	if redacted == nil {
		t.Fatal("expected redacted content")
	}
	if string(content.Content) == string(redacted.Content) {
		t.Fatal("expected content copy to be redacted")
	}

	body := string(redacted.Content)
	for _, leaked := range []string{"Ada Lovelace", "Fix billing sync", "contact-1", "task-1", "CON-1", "TASK-7", "/w/acme"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("redacted JSON leaked %q: %s", leaked, body)
		}
	}
	if !strings.Contains(body, publicRedactedEntityLabel) {
		t.Fatalf("expected redacted label in JSON: %s", body)
	}
	if strings.Contains(redacted.ContentText, "Ada Lovelace") || strings.Contains(redacted.ContentText, "Fix billing sync") {
		t.Fatalf("redacted text leaked entity names: %q", redacted.ContentText)
	}
}

func TestRenderPublicDocsHTMLRedactsEntityEmbeds(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"entityEmbed","attrs":{"entityType":"company","entityId":"company-1","title":"Secret Co","displayId":"CO-1","status":"active"}}]}`)

	html, err := RenderPublicDocsHTML(raw)
	if err != nil {
		t.Fatalf("render public html: %v", err)
	}
	for _, leaked := range []string{"Secret Co", "company-1", "CO-1", "active", `data-entity-type="company"`} {
		if strings.Contains(html, leaked) {
			t.Fatalf("redacted HTML leaked %q: %s", leaked, html)
		}
	}
	if !strings.Contains(html, publicRedactedEntityLabel) {
		t.Fatalf("expected redacted label in HTML: %s", html)
	}
}
