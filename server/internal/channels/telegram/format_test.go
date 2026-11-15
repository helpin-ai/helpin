package telegram

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestMarkdownToTelegramHTML(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"inline styles", "**bold** _italic_ ~~gone~~ `x < y`", "<b>bold</b> <i>italic</i> <s>gone</s> <code>x &lt; y</code>"},
		{"link", "See [docs](https://example.com/a?b=1&c=2)", `See <a href="https://example.com/a?b=1&amp;c=2">docs</a>`},
		{"bare url", "Visit https://example.com today", `Visit <a href="https://example.com">https://example.com</a> today`},
		{"unsafe link dropped", "[click](javascript:alert(1))", "click"},
		{"raw html block kept as text", "<script>alert(1)</script>\n\nAfter & done", "&lt;script&gt;alert(1)&lt;/script&gt;\n\nAfter &amp; done"},
		{"heading", "# Refund\n\nApproved", "<b>Refund</b>\n\nApproved"},
		{"code block", "```go\nx := 1\ny := 2\n```", "<pre>x := 1\ny := 2</pre>"},
		{"quote", "> be kind", "<blockquote>be kind</blockquote>"},
		{"paragraphs", "First line\n\nSecond line", "First line\n\nSecond line"},
		{"inline raw html kept as text", "Use the <div> tag", "Use the &lt;div&gt; tag"},
		{"bullet list", "- one\n- two", "• one\n• two"},
		{"ordered list", "1. one\n2. two", "1. one\n2. two"},
		{"image", "![receipt](https://example.com/r.png)", `<a href="https://example.com/r.png">receipt</a>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := markdownToTelegramHTML(tc.in); got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestMarkdownToPlain(t *testing.T) {
	got := markdownToPlain("**Hi** see [docs](https://example.com) or <b>x</b>")
	if want := "Hi see docs (https://example.com) or <b>x</b>"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestEntitiesToMarkdownNestingAndQuotes(t *testing.T) {
	// "bold italic" bold over all, italic over "italic".
	got := entitiesToMarkdown("bold italic", []entity{{Type: "bold", Offset: 0, Length: 11}, {Type: "italic", Offset: 5, Length: 6}})
	if want := "**bold _italic_**"; got != want {
		t.Errorf("nested = %q want %q", got, want)
	}
	got = entitiesToMarkdown("line one\nline two", []entity{{Type: "blockquote", Offset: 0, Length: 17}})
	if want := "> line one\n> line two"; got != want {
		t.Errorf("quote = %q want %q", got, want)
	}
	// Out-of-range and unsafe entities are skipped rather than corrupting text.
	got = entitiesToMarkdown("abc", []entity{{Type: "bold", Offset: 2, Length: 9}, {Type: "text_link", Offset: 0, Length: 3, URL: "javascript:x"}})
	if got != "abc" {
		t.Errorf("invalid entities = %q", got)
	}
	if got := entitiesToMarkdown("  plain  ", nil); got != "plain" {
		t.Errorf("plain = %q", got)
	}
}

func TestSplitMarkdownFitsLimitInOrder(t *testing.T) {
	paragraph := strings.Repeat("a", 60)
	text := strings.Join([]string{paragraph, paragraph, paragraph, paragraph}, "\n\n")
	parts := splitMarkdown(text, 130)
	if len(parts) != 2 {
		t.Fatalf("parts = %d %q", len(parts), parts)
	}
	for _, part := range parts {
		if utf16Len(markdownToPlain(part)) > 130 {
			t.Errorf("part too long: %d", utf16Len(markdownToPlain(part)))
		}
	}
	if strings.Join(parts, "\n\n") != text {
		t.Error("split lost or reordered text")
	}

	// A single paragraph longer than the limit is cut at word breaks; emoji
	// count as two units.
	words := strings.Repeat("😀 word ", 40)
	for _, part := range splitMarkdown(words, 50) {
		if utf16Len(part) > 50 {
			t.Errorf("hard split part too long: %d", utf16Len(part))
		}
		if strings.HasPrefix(part, "ord") {
			t.Errorf("cut inside a word: %q", part)
		}
	}
	if got := splitMarkdown("short", 4096); len(got) != 1 || got[0] != "short" {
		t.Errorf("short = %q", got)
	}
}

// Telegram rejects the whole message on an unknown or unbalanced tag, so every
// rendering must use only its tags, properly nested.
func TestRenderedHTMLUsesOnlyBalancedTelegramTags(t *testing.T) {
	allowed := map[string]bool{"b": true, "i": true, "s": true, "u": true, "code": true, "pre": true, "blockquote": true, "a": true}
	inputs := []string{
		"**bold _both_ end** and `code`",
		"- one\n  - nested **two**\n- three\n\nAfter list",
		"> quote with [link](https://x.example) and **bold**\n> second line",
		"| a | b |\n|---|---|\n| 1 | 2 |",
		"<b>raw</b> <i>tags</i> <a href=\"https://x\">x</a> <img src=x onerror=alert(1)>",
		"```\n<tag> & stuff\n```",
		"# H1\n## H2\n---\n1. a\n2. b",
		"Unclosed **bold and _italic",
		"Emoji 👋🏽 and [link with `code`](https://example.com)",
	}
	for _, in := range inputs {
		out := markdownToTelegramHTML(in)
		tokenizer := html.NewTokenizer(strings.NewReader(out))
		var stack []string
		for {
			kind := tokenizer.Next()
			if kind == html.ErrorToken {
				break
			}
			name, _ := tokenizer.TagName()
			switch kind {
			case html.StartTagToken:
				if !allowed[string(name)] {
					t.Errorf("%q: disallowed <%s> in %q", in, name, out)
				}
				stack = append(stack, string(name))
			case html.EndTagToken:
				if len(stack) == 0 || stack[len(stack)-1] != string(name) {
					t.Errorf("%q: unbalanced </%s> in %q", in, name, out)
					continue
				}
				stack = stack[:len(stack)-1]
			case html.SelfClosingTagToken:
				t.Errorf("%q: self-closing <%s> in %q", in, name, out)
			}
		}
		if len(stack) > 0 {
			t.Errorf("%q: unclosed %v in %q", in, stack, out)
		}
	}
}
