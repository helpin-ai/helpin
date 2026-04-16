package docsimport

import (
	"strings"
	"testing"
)

// TestStripCodeFenceModifiers_PreservesBody guards against a prior
// regression where stripCodeFenceModifiers used \s+ and ate the
// newline after the opening fence, silently wiping the first line of
// the code block. Real-world case: Google Ads URL templates with
// curly-brace placeholders like {lpurl}, {campaignid}, etc.
func TestStripCodeFenceModifiers_PreservesBody(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain fence preserves content unchanged",
			in:   "```javascript\nvar x = 1;\n```",
			want: "```javascript\nvar x = 1;\n```",
		},
		{
			name: "single content line with curly braces survives",
			in:   "```javascript\n{lpurl}?foo={bar}\n```",
			want: "```javascript\n{lpurl}?foo={bar}\n```",
		},
		{
			name: "modifiers on fence line are stripped",
			in:   "```html copy\n<p>hi</p>\n```",
			want: "```html\n<p>hi</p>\n```",
		},
		{
			name: "filename modifier is stripped",
			in:   "```ts filename=\"index.ts\"\nexport {}\n```",
			want: "```ts\nexport {}\n```",
		},
		{
			name: "line-highlight modifier is stripped",
			in:   "```js {1,3-5}\na\nb\nc\n```",
			want: "```js\na\nb\nc\n```",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripCodeFenceModifiers(tc.in)
			if got != tc.want {
				t.Errorf("mismatch\nin:   %q\ngot:  %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseNextraMDX_CurlyBracesInCodeBlock is the end-to-end guard:
// a real code fence with curly-brace content must survive the full
// Nextra MDX pipeline, not just the individual stripper.
func TestParseNextraMDX_CurlyBracesInCodeBlock(t *testing.T) {
	src := []byte(`---
title: Google Ads
---

### UTM tracking template

` + "```javascript\n{lpurl}?utm_source=googleads&utm_campaign={campaignid}&um_cl={gclid}\n```" + `

End.
`)
	doc, _, err := ParseNextraMDX("test.mdx", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(doc.Body, "{lpurl}") || !strings.Contains(doc.Body, "{gclid}") {
		t.Fatalf("code block body was stripped during import:\n%s", doc.Body)
	}
}
