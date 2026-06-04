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

func TestProcess(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		plainText     string
		wantHTML      []string
		notHTML       []string
		wantMarkdown  []string
		notMarkdown   []string
	}{
		{
			name:         "remote image src preserved in html",
			html:         `<p>Hi</p><img src="https://cdn.example.com/pic.png" alt="pic" width="100" height="100" />`,
			wantHTML:     []string{`src="https://cdn.example.com/pic.png"`, `alt="pic"`},
			wantMarkdown: []string{"Hi", "cdn.example.com/pic.png"},
		},
		{
			name:         "tracking pixel preserved in html but stripped from markdown",
			html:         `<p>Body</p><img src="https://track.example.com/x.gif" width="1" height="1" />`,
			wantHTML:     []string{"Body", "track.example.com"},
			wantMarkdown: []string{"Body"},
			notMarkdown:  []string{"track.example.com"},
		},
		{
			name:         "gmail quoted reply marked not removed from html",
			html:         `<div>Fresh reply</div><div class="gmail_quote"><blockquote>Old thread</blockquote></div>`,
			wantHTML:     []string{"Fresh reply", `data-helpin-quote="true"`, "Old thread"},
			wantMarkdown: []string{"Fresh reply"},
			notMarkdown:  []string{"Old thread"},
		},
		{
			name:     "cid image src dropped by scheme allowlist",
			html:     `<img src="cid:logo@example" alt="logo" />`,
			wantHTML: []string{`alt="logo"`},
			notHTML:  []string{`src="cid:`},
		},
		{
			name:     "data uri stripped by policy",
			html:     `<img src="data:image/png;base64,AAAA" />`,
			notHTML:  []string{"data:image/png", "AAAA"},
		},
		{
			name:     "script and javascript href stripped",
			html:     `<script>alert(1)</script><a href="javascript:alert(2)">x</a><p>ok</p>`,
			wantHTML: []string{"ok"},
			notHTML:  []string{"alert(1)", "javascript:"},
		},
		{
			name:     "safe inline styles preserved",
			html:     `<p style="color: red; font-weight: bold">hi</p>`,
			wantHTML: []string{"color", "red", "font-weight"},
		},
		{
			name:     "dangerous inline styles stripped",
			html:     `<p style="position: absolute; z-index: 9999">hi</p>`,
			wantHTML: []string{"hi"},
			notHTML:  []string{"position", "z-index"},
		},
		{
			name:         "style block preserved in html but not markdown",
			html:         `<style>.mb_work_text h1{font-size:18px}</style><p class="mb_work_text">Hi</p>`,
			wantHTML:     []string{"<style>", ".mb_work_text h1", "font-size:18px", "</style>"},
			wantMarkdown: []string{"Hi"},
			notMarkdown:  []string{"font-size:18px", "mb_work_text"},
		},
		{
			name:     "style @media rules preserved",
			html:     `<style>@media (max-width: 480px) { .card { display: block; } }</style><div class="card">x</div>`,
			wantHTML: []string{"@media", "max-width: 480px", ".card", "display: block"},
		},
		{
			name:     "style @import rule stripped",
			html:     `<style>@import url("https://evil.example.com/evil.css");.ok{color:red}</style><p>hi</p>`,
			wantHTML: []string{"<style>", ".ok{color:red}", "</style>"},
			notHTML:  []string{"@import", "evil.example.com"},
		},
		{
			name:     "style remote url preserved for rendering fidelity",
			html:     `<style>.hero{background-image:url(https://cdn.example.com/hero.jpg)}</style><p>hi</p>`,
			wantHTML: []string{"<style>", ".hero", "url(https://cdn.example.com/hero.jpg)", "</style>"},
		},
		{
			name:     "style relative url preserved",
			html:     `<style>.local{background-image:url(/img/logo.png)}</style><p>hi</p>`,
			wantHTML: []string{"url(/img/logo.png)"},
		},
		{
			name:     "style position fixed collapsed to static",
			html:     `<style>.overlay{position:fixed;top:0}</style><p>hi</p>`,
			wantHTML: []string{"position:static"},
			notHTML:  []string{"position:fixed"},
		},
		{
			name:     "style position sticky collapsed to static",
			html:     `<style>.bar{position: sticky; top: 0}</style><p>hi</p>`,
			wantHTML: []string{"position:static"},
			notHTML:  []string{"position: sticky", "position:sticky"},
		},
		{
			name:     "script tag still stripped when alongside style",
			html:     `<style>.x{color:red}</style><script>alert(1)</script><p>ok</p>`,
			wantHTML: []string{"<style>", "color:red", "ok"},
			notHTML:  []string{"<script", "alert(1)"},
		},
		{
			name:      "empty html returns empty html and plain markdown fallback",
			html:      "",
			plainText: "fallback",
			wantMarkdown: []string{"fallback"},
		},
		{
			name:         "ordered list survives in both variants",
			html:         `<ol><li>One</li><li>Two</li></ol>`,
			wantHTML:     []string{"<ol", "<li", "One", "Two"},
			wantMarkdown: []string{"1. One", "2. Two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Process(tt.html, tt.plainText)
			for _, w := range tt.wantHTML {
				if !strings.Contains(got.HTML, w) {
					t.Errorf("Process().HTML missing %q in:\n%s", w, got.HTML)
				}
			}
			for _, b := range tt.notHTML {
				if strings.Contains(got.HTML, b) {
					t.Errorf("Process().HTML unexpectedly contains %q in:\n%s", b, got.HTML)
				}
			}
			for _, w := range tt.wantMarkdown {
				if !strings.Contains(got.Markdown, w) {
					t.Errorf("Process().Markdown missing %q in:\n%s", w, got.Markdown)
				}
			}
			for _, b := range tt.notMarkdown {
				if strings.Contains(got.Markdown, b) {
					t.Errorf("Process().Markdown unexpectedly contains %q in:\n%s", b, got.Markdown)
				}
			}
		})
	}
}

func TestProcessHTMLRespectsMaxLen(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<p>")
	for range 300_000 {
		sb.WriteByte('a')
	}
	sb.WriteString("</p>")

	got := Process(sb.String(), "")
	if len(got.HTML) > maxHTMLLen {
		t.Fatalf("Process().HTML length = %d, want <= %d", len(got.HTML), maxHTMLLen)
	}
}
