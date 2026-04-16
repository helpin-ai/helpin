package inboundhtml

import (
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		plainText  string
		wantSubstr []string
		notSubstr  []string
	}{
		{
			name:       "anchor text collapses long href",
			html:       `<p>Try <a href="https://app.example.com/register?utm_campaign=Feature+releases&utm_content=New+Newsletter">Regenerate now</a>.</p>`,
			wantSubstr: []string{"[Regenerate now](", "https://app.example.com/register?utm_campaign=", "utm_content="},
			notSubstr:  []string{"Regenerate now\n", "https://app.example.com/register Regenerate"},
		},
		{
			name: "gmail quoted reply stripped",
			html: `<div>My new reply.</div>` +
				`<div class="gmail_quote"><div>On Mon, wrote:</div>` +
				`<blockquote>Older thread content</blockquote></div>`,
			wantSubstr: []string{"My new reply."},
			notSubstr:  []string{"Older thread content", "On Mon, wrote"},
		},
		{
			name:       "apple mail blockquote stripped",
			html:       `<p>Reply body.</p><blockquote type="cite"><p>Quoted history</p></blockquote>`,
			wantSubstr: []string{"Reply body."},
			notSubstr:  []string{"Quoted history"},
		},
		{
			name:       "tracking pixel dropped",
			html:       `<p>Hello</p><img src="https://track.example.com/x.gif" width="1" height="1" />`,
			wantSubstr: []string{"Hello"},
			notSubstr:  []string{"track.example.com/x.gif"},
		},
		{
			name:       "script and style removed",
			html:       `<style>.x{color:red}</style><script>alert(1)</script><p>Safe text</p>`,
			wantSubstr: []string{"Safe text"},
			notSubstr:  []string{"alert(1)", "color:red"},
		},
		{
			name:      "empty html falls back to plain text",
			html:      "",
			plainText: "fallback body",
			wantSubstr: []string{"fallback body"},
		},
		{
			name:      "whitespace-only html falls back to plain text",
			html:      "   \n\t ",
			plainText: "plain only",
			wantSubstr: []string{"plain only"},
		},
		{
			name:      "empty everywhere returns empty",
			html:      "",
			plainText: "",
		},
		{
			name:       "outlook quoted reply stripped",
			html:       `<p>My reply here.</p><div id="divRplyFwdMsg"><hr><p>From: Sender</p><p>Prior content</p></div>`,
			wantSubstr: []string{"My reply here."},
			notSubstr:  []string{"Prior content", "From: Sender"},
		},
		{
			name:       "yahoo quoted reply stripped",
			html:       `<p>Short reply.</p><div class="yahoo_quoted"><p>Earlier thread</p></div>`,
			wantSubstr: []string{"Short reply."},
			notSubstr:  []string{"Earlier thread"},
		},
		{
			name:       "proton quoted reply stripped",
			html:       `<p>Proton reply.</p><div class="protonmail_quote"><blockquote>Prior</blockquote></div>`,
			wantSubstr: []string{"Proton reply."},
			notSubstr:  []string{"Prior"},
		},
		{
			name:       "multiple quoted clients stripped together",
			html:       `<p>Fresh reply.</p><blockquote type="cite">Apple history</blockquote><div class="gmail_quote">Gmail history</div>`,
			wantSubstr: []string{"Fresh reply."},
			notSubstr:  []string{"Apple history", "Gmail history"},
		},
		{
			name:       "zero-width tracking pixel dropped",
			html:       `<p>Body</p><img src="https://track.example.com/p.gif" width="0" height="0" />`,
			wantSubstr: []string{"Body"},
			notSubstr:  []string{"track.example.com/p.gif"},
		},
		{
			name:       "multiple tracking pixels dropped",
			html:       `<p>Hi</p><img src="https://t1.com/1.gif" width="1" height="1"/><img src="https://t2.com/2.gif" height="1" width="1"/>`,
			wantSubstr: []string{"Hi"},
			notSubstr:  []string{"t1.com", "t2.com"},
		},
		{
			name:       "legitimate image preserved",
			html:       `<p>See <img src="https://cdn.example.com/hero.png" alt="Hero" width="600" height="400" /></p>`,
			wantSubstr: []string{"https://cdn.example.com/hero.png", "Hero"},
		},
		{
			name:       "unordered list preserved as markdown",
			html:       `<ul><li>First</li><li>Second</li><li>Third</li></ul>`,
			wantSubstr: []string{"- First", "- Second", "- Third"},
		},
		{
			name:       "ordered list preserved as markdown",
			html:       `<ol><li>Step one</li><li>Step two</li></ol>`,
			wantSubstr: []string{"1. Step one", "2. Step two"},
		},
		{
			name:       "bold and italic preserved",
			html:       `<p>This is <strong>important</strong> and <em>emphasized</em>.</p>`,
			wantSubstr: []string{"**important**", "*emphasized*"},
		},
		{
			name:       "heading converted",
			html:       `<h2>Section Title</h2><p>Body</p>`,
			wantSubstr: []string{"## Section Title", "Body"},
		},
		{
			name:       "code block preserved",
			html:       `<pre><code>go test ./...</code></pre>`,
			wantSubstr: []string{"go test ./..."},
		},
		{
			name:       "mailto and tel links preserved",
			html:       `<p>Reach <a href="mailto:help@example.com">email</a> or <a href="tel:+15551234">phone</a>.</p>`,
			wantSubstr: []string{"mailto:help@example.com", "tel:+15551234"},
		},
		{
			name:       "javascript href stripped by sanitizer",
			html:       `<p>Click <a href="javascript:alert(1)">here</a></p>`,
			notSubstr:  []string{"javascript:alert", "alert(1)"},
			wantSubstr: []string{"Click", "here"},
		},
		{
			name:       "onclick handler stripped",
			html:       `<p onclick="steal()">Plain text</p>`,
			wantSubstr: []string{"Plain text"},
			notSubstr:  []string{"onclick", "steal()"},
		},
		{
			name:       "form elements stripped",
			html:       `<p>Before</p><form action="/x"><input name="y"/></form><p>After</p>`,
			wantSubstr: []string{"Before", "After"},
			notSubstr:  []string{"<form", "<input"},
		},
		{
			name:       "inline style attributes stripped",
			html:       `<p style="color:red;font-size:40px">Styled text</p>`,
			wantSubstr: []string{"Styled text"},
			notSubstr:  []string{"color:red", "font-size"},
		},
		{
			name:       "html entities decoded",
			html:       `<p>Fish &amp; chips &mdash; yum</p>`,
			wantSubstr: []string{"Fish & chips", "yum"},
			notSubstr:  []string{"&amp;", "&mdash;"},
		},
		{
			name:       "unicode and emoji preserved",
			html:       `<p>Thanks! 🙏 Café, naïve, Zürich.</p>`,
			wantSubstr: []string{"🙏", "Café", "naïve", "Zürich"},
		},
		{
			name:       "nested quoted history removed wholesale",
			html:       `<p>Top reply</p><div class="gmail_quote"><p>Level 1</p><blockquote type="cite"><p>Level 2</p><blockquote type="cite"><p>Level 3</p></blockquote></blockquote></div>`,
			wantSubstr: []string{"Top reply"},
			notSubstr:  []string{"Level 1", "Level 2", "Level 3"},
		},
		{
			name:       "br tags become line breaks",
			html:       `<p>Line one<br>Line two<br>Line three</p>`,
			wantSubstr: []string{"Line one", "Line two", "Line three"},
		},
		{
			name:       "table preserved as markdown table",
			html:       `<table><thead><tr><th>Name</th><th>Role</th></tr></thead><tbody><tr><td>Alice</td><td>Engineer</td></tr></tbody></table>`,
			wantSubstr: []string{"| Name", "Role", "Alice", "Engineer"},
		},
		{
			name:       "iframe stripped entirely",
			html:       `<p>Watch</p><iframe src="https://evil.example.com/x"></iframe>`,
			wantSubstr: []string{"Watch"},
			notSubstr:  []string{"iframe", "evil.example.com"},
		},
		{
			name:       "whitespace-only after stripping falls back to plain text",
			html:       `<div class="gmail_quote"><p>Only quoted content</p></div>`,
			plainText:  "plain reply",
			wantSubstr: []string{"plain reply"},
			notSubstr:  []string{"Only quoted content"},
		},
		{
			name:       "malformed html still produces output",
			html:       `<p>Broken <b>tag <i>still</b> reads</p>`,
			wantSubstr: []string{"Broken", "still", "reads"},
		},
		{
			name:       "long URL in anchor text collapses to display text",
			html:       `<p><a href="https://example.com/very/long/path?a=1&b=2&c=3&d=4&e=5&f=6&g=7">click</a></p>`,
			wantSubstr: []string{"[click]("},
			notSubstr:  []string{"click)click"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Convert(tt.html, tt.plainText)
			for _, want := range tt.wantSubstr {
				if !strings.Contains(got, want) {
					t.Errorf("Convert() missing %q in:\n%s", want, got)
				}
			}
			for _, bad := range tt.notSubstr {
				if strings.Contains(got, bad) {
					t.Errorf("Convert() unexpectedly contains %q in:\n%s", bad, got)
				}
			}
		})
	}
}

func TestConvertTruncatesToLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<p>")
	for range 60_000 {
		sb.WriteByte('a')
	}
	sb.WriteString("</p>")

	got := Convert(sb.String(), "")
	if len(got) > maxMarkdownLen {
		t.Fatalf("Convert() length = %d, want <= %d", len(got), maxMarkdownLen)
	}
}

func TestConvertCollapsesBlankLines(t *testing.T) {
	html := "<p>A</p><p>&nbsp;</p><p>&nbsp;</p><p>&nbsp;</p><p>B</p>"
	got := Convert(html, "")
	if strings.Contains(got, "\n\n\n\n") {
		t.Errorf("Convert() did not collapse blank lines: %q", got)
	}
}
