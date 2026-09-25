package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func TestMaterializePublicationDocumentLinks(t *testing.T) {
	draft := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Read ","marks":[]},
		{"type":"text","text":"Core concepts","marks":[{"type":"link","attrs":{"href":"helpin://documents/live-doc","target":"_blank"}},{"type":"bold"}]},
		{"type":"text","text":" and "},
		{"type":"text","text":"Billing","marks":[{"type":"link","attrs":{"href":"helpin://documents/draft-doc"}}]},
		{"type":"text","text":" or "},
		{"type":"text","text":"our site","marks":[{"type":"link","attrs":{"href":"https://helpin.ai"}}]}
	]}]}`)
	published, err := materializePublicationDocumentLinks(context.Background(), draft, staticLinkPaths(map[string]string{
		"live-doc": "/articles/core-concepts-ba8b1552",
	}))
	if err != nil {
		t.Fatalf("materializePublicationDocumentLinks() error = %v", err)
	}
	text := string(published)
	for _, want := range []string{`"href":"/articles/core-concepts-ba8b1552"`, `"href":"https://helpin.ai"`, `"type":"bold"`} {
		if !strings.Contains(stripPublishedFrom(t, published), want) {
			t.Errorf("published content missing %s: %s", want, text)
		}
	}
	rendered := stripPublishedFrom(t, published)
	if strings.Contains(rendered, "helpin://") || strings.Contains(rendered, `"target"`) {
		t.Fatalf("published content still has in-app links or targets: %s", rendered)
	}
	if !strings.Contains(rendered, `"text":"Billing"`) {
		t.Fatalf("unlinked text was dropped: %s", rendered)
	}
	html, err := tiptap.RenderHTML(published)
	if err != nil {
		t.Fatalf("RenderHTML() error = %v", err)
	}
	if !strings.Contains(html, `href="/articles/core-concepts-ba8b1552"`) || strings.Contains(html, "helpin://") {
		t.Fatalf("rendered public HTML = %s", html)
	}
	if !publicationContentEqual(draft, published) {
		t.Fatal("rewritten links must not count as unpublished changes")
	}
}

func TestMaterializePublicationDocumentLinksSkipsContentWithoutInternalLinks(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[]}`)
	got, err := materializePublicationDocumentLinks(context.Background(), raw, func(context.Context, []string) (map[string]string, error) {
		t.Fatal("resolver must not run")
		return nil, nil
	})
	if err != nil || string(got) != string(raw) {
		t.Fatalf("got %s, %v; want unchanged", got, err)
	}
}

// stripPublishedFrom returns the snapshot without preserved source nodes, i.e.
// what readers see.
func stripPublishedFrom(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var strip func(any)
	strip = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			if attrs, ok := typed["attrs"].(map[string]any); ok {
				delete(attrs, "publishedFrom")
			}
			for _, child := range typed {
				strip(child)
			}
		case []any:
			for _, child := range typed {
				strip(child)
			}
		}
	}
	strip(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(encoded)
}
