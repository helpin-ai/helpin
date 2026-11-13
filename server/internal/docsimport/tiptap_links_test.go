package docsimport

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRewriteTiptapLinks(t *testing.T) {
	doc := `{"type":"doc","content":[
		{"type":"paragraph","attrs":{"blockId":"b1"},"content":[
			{"type":"text","text":"See ","marks":[{"type":"bold"}]},
			{"type":"text","text":"invoices","marks":[{"type":"link","attrs":{"href":"https://docs.acme.example/en/articles/12-invoices#pay","target":"_blank","rel":"noopener noreferrer"}},{"type":"bold"}]},
			{"type":"text","text":"elsewhere","marks":[{"type":"link","attrs":{"href":"https://other.example/x","target":"_blank"}}]}
		]},
		{"type":"resizableImage","attrs":{"src":"https://cdn/a.png","width":"550px","linkUrl":"https://docs.acme.example/en/articles/12-invoices","linkNewTab":true,"ratio":1.5}},
		{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[
			{"type":"text","text":"nested","marks":[{"type":"link","attrs":{"href":"https://docs.acme.example/en/articles/12-invoices"}}]}
		]}]}]},
		{"type":"futureNode","custom":{"keep":true},"count":12345678901234567890}
	]}`
	var seen []string
	rewritten, changed, err := RewriteTiptapLinks(json.RawMessage(doc), func(href string) (string, bool) {
		seen = append(seen, href)
		if strings.Contains(href, "/articles/12-") {
			if i := strings.Index(href, "#"); i >= 0 {
				return "/articles/invoices-abc" + href[i:], true
			}
			return "/articles/invoices-abc", true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("RewriteTiptapLinks: %v", err)
	}
	if changed != 3 || len(seen) != 4 {
		t.Fatalf("changed = %d, links seen = %d (%v); want 3 and 4", changed, len(seen), seen)
	}
	got := string(rewritten)
	for _, want := range []string{
		// The rewritten link opens in place: target and rel are gone.
		`{"attrs":{"href":"/articles/invoices-abc#pay"},"type":"link"}`,
		`"linkNewTab":false`, `"linkUrl":"/articles/invoices-abc"`,
		// Untouched links and unknown fields survive as written.
		`{"attrs":{"href":"https://other.example/x","target":"_blank"},"type":"link"}`,
		`"custom":{"keep":true}`, `"count":12345678901234567890`, `"ratio":1.5`, `"blockId":"b1"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten document lacks %s\n%s", want, got)
		}
	}

	// A second pass finds nothing to change and returns the input untouched.
	again, changedAgain, err := RewriteTiptapLinks(rewritten, func(href string) (string, bool) {
		return "", false
	})
	if err != nil || changedAgain != 0 || string(again) != got {
		t.Fatalf("second pass changed = %d, err = %v", changedAgain, err)
	}
}

func TestRewriteTiptapLinksRejectsInvalidJSON(t *testing.T) {
	if _, _, err := RewriteTiptapLinks(json.RawMessage(`{"type":`), func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
}

func TestRewriteTiptapLinksIgnoresSameReplacement(t *testing.T) {
	doc := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"/a","target":"_blank"}}]}]}]}`)
	out, changed, err := RewriteTiptapLinks(doc, func(href string) (string, bool) { return href, true })
	if err != nil || changed != 0 || string(out) != string(doc) {
		t.Fatalf("changed = %d, err = %v, out = %s", changed, err, out)
	}
}
