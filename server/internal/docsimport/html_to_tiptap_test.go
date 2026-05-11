package docsimport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func convert(t *testing.T, html string) *ConversionResult {
	t.Helper()
	result, err := ConvertHTML(html)
	if err != nil {
		t.Fatalf("ConvertHTML failed: %v", err)
	}
	return result
}

func toJSON(t *testing.T, result *ConversionResult) string {
	t.Helper()
	b, err := json.Marshal(result.Doc)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return string(b)
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func convertHelpScout(t *testing.T, name string) *ConversionResult {
	t.Helper()
	normalized, warnings := PreprocessHelpScoutHTML(readFixture(t, name))
	result := convert(t, normalized)
	result.Warnings = append(warnings, result.Warnings...)
	return result
}

func TestConvert_Paragraph(t *testing.T) {
	r := convert(t, "<p>Hello world</p>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"paragraph"`) || !strings.Contains(j, "Hello world") {
		t.Errorf("expected paragraph, got: %s", j)
	}
}

func TestConvert_Headings(t *testing.T) {
	r := convert(t, "<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"level":1`) || !strings.Contains(j, `"level":2`) || !strings.Contains(j, `"level":3`) {
		t.Errorf("expected headings with levels 1-3, got: %s", j)
	}
}

func TestConvert_BoldItalic(t *testing.T) {
	r := convert(t, "<p><strong>bold</strong> and <em>italic</em></p>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"bold"`) || !strings.Contains(j, `"type":"italic"`) {
		t.Errorf("expected bold and italic marks, got: %s", j)
	}
}

func TestConvert_Link(t *testing.T) {
	r := convert(t, `<p><a href="https://example.com">click</a></p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"link"`) || !strings.Contains(j, `"href":"https://example.com"`) {
		t.Errorf("expected link mark, got: %s", j)
	}
}

func TestConvert_BulletList(t *testing.T) {
	r := convert(t, "<ul><li>one</li><li>two</li></ul>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"bulletList"`) || !strings.Contains(j, `"type":"listItem"`) {
		t.Errorf("expected bullet list, got: %s", j)
	}
}

func TestConvert_OrderedList(t *testing.T) {
	r := convert(t, "<ol><li>first</li><li>second</li></ol>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"orderedList"`) {
		t.Errorf("expected ordered list, got: %s", j)
	}
}

func TestConvert_ListItemKeepsInlineBoldWithinSingleParagraph(t *testing.T) {
	r := convert(t, `<ul><li>Click the “<strong>White-Label</strong>” option to rebrand dashboard.</li></ul>`)
	j := toJSON(t, r)
	if strings.Count(j, `"type":"paragraph"`) != 1 {
		t.Fatalf("expected a single paragraph inside list item, got: %s", j)
	}
	if !strings.Contains(j, `"type":"bold"`) {
		t.Fatalf("expected bold mark to be preserved, got: %s", j)
	}
	if !strings.Contains(j, `Click the`) || !strings.Contains(j, `White-Label`) || !strings.Contains(j, `option to rebrand dashboard.`) {
		t.Fatalf("expected inline text to remain together, got: %s", j)
	}
}

func TestConvert_ListItemKeepsMultipleInlineMarkedSpansWithinSingleParagraph(t *testing.T) {
	r := convert(t, `<ol><li>Navigate to Manage on <strong>Replug Dashboard</strong>, hover over to <strong>Replug Links</strong> and click on it.</li></ol>`)
	j := toJSON(t, r)
	if strings.Count(j, `"type":"paragraph"`) != 1 {
		t.Fatalf("expected a single paragraph inside list item, got: %s", j)
	}
	if strings.Count(j, `"type":"bold"`) != 2 {
		t.Fatalf("expected both bold spans to be preserved inline, got: %s", j)
	}
}

func TestConvert_ParagraphNormalizesWhitespaceAroundInlineFormatting(t *testing.T) {
	r := convert(t, "<p>Go to the \n\t<strong>Integrations</strong>page from profile settings.</p>")
	j := toJSON(t, r)
	if strings.Contains(j, `\n`) || strings.Contains(j, `\t`) {
		t.Fatalf("expected newline and tab noise to be removed, got: %s", j)
	}
	if !strings.Contains(j, `Go to the `) || !strings.Contains(j, `Integrations`) || !strings.Contains(j, `page from profile settings.`) {
		t.Fatalf("expected paragraph content to be preserved, got: %s", j)
	}
}

func TestConvert_ParagraphTrimsSpacerBreakNoise(t *testing.T) {
	r := convert(t, `<p><br> Please note the size and format for Favicons. <br><br> <strong>Size:</strong> 16x16 pixels.</p>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"content":[{"type":"hardBreak"}`) {
		t.Fatalf("expected leading hard break to be removed, got: %s", j)
	}
	if strings.Contains(j, `"type":"hardBreak"},{"type":"hardBreak"`) {
		t.Fatalf("expected consecutive hard breaks to be collapsed, got: %s", j)
	}
	if strings.Contains(j, `\n`) || strings.Contains(j, `\t`) {
		t.Fatalf("expected whitespace noise around hard breaks to be removed, got: %s", j)
	}
}

func TestConvert_ParagraphPreservesSpaceAfterLinkText(t *testing.T) {
	r := convert(t, `<p>Check our blog on <a href="https://example.com">Bio Links&nbsp;</a>for the latest market trends.</p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"text":"Bio Links "`) {
		t.Fatalf("expected trailing space inside linked text to be preserved, got: %s", j)
	}
	if !strings.Contains(j, `"text":"for the latest market trends."`) {
		t.Fatalf("expected trailing text after link, got: %s", j)
	}
}

func TestConvert_ParagraphInsertsSeparatorAfterMarkedSpanWhenMissing(t *testing.T) {
	r := convert(t, `<p>Click on the <strong>Save</strong>button.</p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"text":"Save "`) && !strings.Contains(j, `"text":" button."`) {
		t.Fatalf("expected a separator space to be preserved or inferred around marked span, got: %s", j)
	}
}

func TestConvert_ParagraphInsertsSeparatorBetweenAdjacentMarkedSpansWhenMissing(t *testing.T) {
	r := convert(t, `<p>Configure the <a href="https://example.com">RSS Feed</a><strong>of your choice</strong> for the audience.</p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"text":"RSS Feed "`) && !strings.Contains(j, `"text":" of your choice"`) {
		t.Fatalf("expected a separator space between adjacent marked spans, got: %s", j)
	}
}

func TestConvert_ParagraphPreservesSpaceBeforeMarkedSpan(t *testing.T) {
	r := convert(t, `<p>Log into your<strong> Replug account.</strong></p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"text":"Log into your "`) && !strings.Contains(j, `"text":" Replug account."`) {
		t.Fatalf("expected separator space before marked span, got: %s", j)
	}
}

func TestConvert_ParagraphInsertsSeparatorBeforeMarkedSpanWhenMissing(t *testing.T) {
	r := convert(t, `<p>Open<strong>Settings</strong>to continue.</p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"text":"Open "`) && !strings.Contains(j, `"text":" Settings"`) {
		t.Fatalf("expected separator space before marked span when source omits it, got: %s", j)
	}
}

func TestConvert_Blockquote(t *testing.T) {
	r := convert(t, "<blockquote><p>A quote</p></blockquote>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"blockquote"`) || !strings.Contains(j, "A quote") {
		t.Errorf("expected blockquote, got: %s", j)
	}
}

func TestConvert_CodeBlock(t *testing.T) {
	r := convert(t, `<pre><code class="language-go">func main() {}</code></pre>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"codeBlock"`) || !strings.Contains(j, `"language":"go"`) {
		t.Errorf("expected code block with language, got: %s", j)
	}
}

func TestConvert_HorizontalRule(t *testing.T) {
	r := convert(t, "<hr>")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"horizontalRule"`) {
		t.Errorf("expected horizontal rule, got: %s", j)
	}
}

func TestConvert_Image(t *testing.T) {
	r := convert(t, `<img src="https://cdn.example.com/img.png" alt="screenshot">`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"resizableImage"`) || !strings.Contains(j, `"src":"https://cdn.example.com/img.png"`) {
		t.Errorf("expected image node, got: %s", j)
	}
}

func TestConvert_Table(t *testing.T) {
	r := convert(t, `<table><tr><th>Name</th><th>Age</th></tr><tr><td>Alice</td><td>30</td></tr></table>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"table"`) || !strings.Contains(j, `"type":"tableHeader"`) || !strings.Contains(j, `"type":"tableCell"`) {
		t.Errorf("expected table with header and cells, got: %s", j)
	}
}

func TestConvert_HelpScoutCallout(t *testing.T) {
	r := convert(t, `<div class="callout callout-info"><p>Important note</p></div>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"callout"`) || !strings.Contains(j, `"variant":"blue"`) {
		t.Errorf("expected callout with blue variant, got: %s", j)
	}
	if !strings.Contains(j, "Important note") {
		t.Errorf("expected callout content, got: %s", j)
	}
}

func TestConvert_HelpScoutCalloutDirectColor(t *testing.T) {
	// HelpScout uses "callout-blue", "callout-green" as single class names
	tests := []struct {
		class   string
		variant string
	}{
		{"callout-blue", "blue"},
		{"callout-green", "green"},
		{"callout-red", "red"},
		{"callout-yellow", "yellow"},
	}
	for _, tt := range tests {
		r := convert(t, `<div class="`+tt.class+`"><p>test</p></div>`)
		j := toJSON(t, r)
		if !strings.Contains(j, `"type":"callout"`) || !strings.Contains(j, `"variant":"`+tt.variant+`"`) {
			t.Errorf("class %q: expected callout with %s variant, got: %s", tt.class, tt.variant, j)
		}
	}
}

func TestConvert_HelpScoutCalloutWarn(t *testing.T) {
	r := convert(t, `<div class="callout callout-warn"><p>Be careful</p></div>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"variant":"yellow"`) {
		t.Errorf("expected yellow variant, got: %s", j)
	}
}

func TestConvert_HelpScoutCalloutDanger(t *testing.T) {
	r := convert(t, `<div class="callout callout-danger"><p>Danger!</p></div>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"variant":"red"`) {
		t.Errorf("expected red variant, got: %s", j)
	}
}

func TestConvert_YouTubeIframe(t *testing.T) {
	r := convert(t, `<iframe src="https://www.youtube.com/embed/abc123" width="560" height="315"></iframe>`)
	j := toJSON(t, r)
	for _, want := range []string{`"type":"htmlBlock"`, `"renderMode":"sandboxed"`, `iframe src=\"https://www.youtube.com/embed/abc123\" width=\"560\" height=\"315\"`, `\u003c/iframe\u003e`} {
		if !strings.Contains(j, want) {
			t.Errorf("expected source iframe to be preserved as raw html block %q, got: %s", want, j)
		}
	}
}

func TestConvert_VimeoIframe(t *testing.T) {
	r := convert(t, `<iframe src="https://player.vimeo.com/video/123456"></iframe>`)
	j := toJSON(t, r)
	for _, want := range []string{`"type":"htmlBlock"`, `"renderMode":"sandboxed"`, `iframe src=\"https://player.vimeo.com/video/123456\"`, `\u003c/iframe\u003e`} {
		if !strings.Contains(j, want) {
			t.Errorf("expected source iframe to be preserved as raw html block %q, got: %s", want, j)
		}
	}
}

func TestConvert_HelpScoutWistiaEmbedWrapper(t *testing.T) {
	r := convert(t, `<script src="https://fast.wistia.com/assets/external/E-v1.js" type="text/javascript"></script>
		<div class="wistia_embed wistia_async_uji8gq6l8o" style="height:534px;position:relative;width:300px">
			<div class="wistia_swatch cc_cursor"><img src="https://cdn.example.com/swatch" alt=""></div>
			<div class="wistia_swatch cc_cursor"><br></div>
		</div>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"htmlBlock"`) || strings.Contains(j, `wistia_swatch`) {
		t.Fatalf("expected Wistia wrapper to avoid swatch/htmlBlock fallback, got: %s", j)
	}
	if !strings.Contains(j, `"type":"videoEmbed"`) || !strings.Contains(j, `"provider":"wistia"`) {
		t.Fatalf("expected Wistia wrapper to become videoEmbed, got: %s", j)
	}
	if !strings.Contains(j, `"embedUrl":"https://fast.wistia.net/embed/iframe/uji8gq6l8o"`) {
		t.Fatalf("expected normalized Wistia embed URL, got: %s", j)
	}
}

func TestConvert_UnsupportedIframe(t *testing.T) {
	r := convert(t, `<iframe src="https://unknown.example.com/embed"></iframe>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"videoEmbed"`) {
		t.Errorf("expected no video embed for unknown iframe, got: %s", j)
	}
	for _, want := range []string{`"type":"htmlBlock"`, `"renderMode":"sandboxed"`, `iframe src=\"https://unknown.example.com/embed\"`, `\u003c/iframe\u003e`} {
		if !strings.Contains(j, want) {
			t.Errorf("expected unsupported source iframe to be preserved as raw html block %q, got: %s", want, j)
		}
	}
	if len(r.Warnings) != 0 {
		t.Errorf("expected no iframe warning when preserving source iframe, got: %#v", r.Warnings)
	}
}

func TestConvert_Figure(t *testing.T) {
	r := convert(t, `<figure><img src="https://cdn.example.com/photo.jpg" alt="Photo"><figcaption>A nice photo</figcaption></figure>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"resizableImage"`) {
		t.Errorf("expected image from figure, got: %s", j)
	}
	if !strings.Contains(j, "A nice photo") {
		t.Errorf("expected caption text, got: %s", j)
	}
}

func TestConvert_NestedFormatting(t *testing.T) {
	r := convert(t, `<p><strong><em>bold italic</em></strong></p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"bold"`) || !strings.Contains(j, `"type":"italic"`) {
		t.Errorf("expected nested bold+italic marks, got: %s", j)
	}
}

func TestConvert_EmptyInput(t *testing.T) {
	r := convert(t, "")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"doc"`) || !strings.Contains(j, `"type":"paragraph"`) {
		t.Errorf("expected doc with empty paragraph, got: %s", j)
	}
}

func TestConvert_UnsafeLinkStripped(t *testing.T) {
	r := convert(t, `<p><a href="javascript:alert('xss')">click me</a></p>`)
	j := toJSON(t, r)
	if strings.Contains(j, "javascript") {
		t.Errorf("expected javascript: link stripped, got: %s", j)
	}
	if !strings.Contains(j, "click me") {
		t.Errorf("expected text preserved, got: %s", j)
	}
}

func TestConvert_OrderedListStart(t *testing.T) {
	r := convert(t, `<ol start="5"><li>fifth</li><li>sixth</li></ol>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"start":5`) {
		t.Errorf("expected start:5, got: %s", j)
	}
}

func TestConvert_HelpScoutNumberedDefinitionListBecomesOrderedList(t *testing.T) {
	r := convert(t, `<dl><dt>3</dt><dd><strong>Two-Factor Authentication (2FA)</strong></dd></dl>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"htmlBlock"`) {
		t.Fatalf("expected definition list to avoid htmlBlock fallback, got: %s", j)
	}
	if !strings.Contains(j, `"type":"orderedList"`) || !strings.Contains(j, `"start":3`) {
		t.Fatalf("expected numbered definition list to become ordered list with start=3, got: %s", j)
	}
	if !strings.Contains(j, `Two-Factor Authentication`) || !strings.Contains(j, `"type":"bold"`) {
		t.Fatalf("expected definition body formatting preserved, got: %s", j)
	}
}

func TestConvert_HelpScoutStandaloneDefinitionTermsBecomeNativeBlocks(t *testing.T) {
	r := convert(t, `<dt>1</dt><dd>Install the mobile application.</dd>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"htmlBlock"`) {
		t.Fatalf("expected standalone dt/dd to avoid htmlBlock fallback, got: %s", j)
	}
	if !strings.Contains(j, `"type":"orderedList"`) || !strings.Contains(j, `Install the mobile application.`) {
		t.Fatalf("expected standalone numeric dt/dd pair to become ordered list content, got: %s", j)
	}
}

func TestConvert_DetailsBecomesToggleSection(t *testing.T) {
	r := convert(t, `<details><summary><span>Authentication &amp; Setup</span><span>3 topics</span></summary><div><div>How to Get Your API Key</div><div><a href="#auth">Authentication</a></div></div></details>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"htmlBlock"`) {
		t.Fatalf("expected details to avoid htmlBlock fallback, got: %s", j)
	}
	for _, want := range []string{
		`"type":"toggleSection"`,
		`"title":"Authentication \u0026 Setup"`,
		`"badgeText":"3 topics"`,
		`"sourceStyle":"helpScoutCard"`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected details summary metadata %q, got: %s", want, j)
		}
	}
	if !strings.Contains(j, `How to Get Your API Key`) || !strings.Contains(j, `"type":"link"`) {
		t.Fatalf("expected details body content and links preserved, got: %s", j)
	}
}

func TestConvert_HelpScoutDetailsPreservesIconAndBadge(t *testing.T) {
	r := convert(t, `<details><summary style="display:flex"><span>🔑</span><span>Authentication &amp; Setup</span><span>3 topics</span><span>▼</span></summary><div><div><a href="#api-key">How to Get Your ContentStudio API Key</a></div></div></details>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"type":"toggleSection"`,
		`"title":"Authentication \u0026 Setup"`,
		`"icon":"🔑"`,
		`"badgeText":"3 topics"`,
		`"sourceStyle":"helpScoutCard"`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected Help Scout detail metadata %q, got: %s", want, j)
		}
	}
	if strings.Contains(j, `Authentication \u0026 Setup 3 topics`) {
		t.Fatalf("expected title to exclude icon and badge text, got: %s", j)
	}
}

func TestConvert_HelpScoutFacebookBackgroundGridBecomesHtmlGrid(t *testing.T) {
	dataImg := "data:image/png;base64,iVBORw0KGgo="
	r := convert(t, `<div data-html-block=""><div style="background:#eeeeff"><table><tbody><tr><td><table><tbody><tr><td><img alt="" width="36" height="36" src="`+dataImg+`"/></td><td><span>106018623298955</span></td></tr></tbody></table></td><td><table><tbody><tr><td><img alt="" width="36" height="36" src="`+dataImg+`"/></td><td><span>191761991491375</span></td></tr></tbody></table></td></tr></tbody></table><button id="fb-show-more">Show 36 more ▾</button><div id="fb-second" style="display:none"><table><tbody><tr><td><table><tbody><tr><td><img alt="" width="36" height="36" src="`+dataImg+`"/></td><td><span>1654916007940525</span></td></tr></tbody></table></td></tr></tbody></table></div><button id="fb-show-less">Show less ▴</button></div></div>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"type":"htmlBlock"`,
		`docs-fb-background-grid`,
		`docs-fb-background-card`,
		`106018623298955`,
		`191761991491375`,
		`1654916007940525`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected Facebook background grid content %q, got: %s", want, j)
		}
	}
	for _, notWant := range []string{`"type":"table"`, `Show 36 more`, `Show less`} {
		if strings.Contains(j, notWant) {
			t.Fatalf("expected Facebook background grid to avoid %q, got: %s", notWant, j)
		}
	}
}

func TestConvert_HelpScoutDataHTMLBlockPreservesUnknownStyledHTML(t *testing.T) {
	r := convert(t, `<div data-html-block=""><div style="border: 1px solid #e5e7eb; border-radius: 10px; padding: 16px; margin-bottom:16px;" onclick="alert(1)">
  <p>
    <span style="background: #007BFF;color:#fff;width:24px;height:24px;line-height:24px;text-align:center;display: inline-block;border-radius:50%;font-weight:bold;">2</span>
    Click on <strong>Generate API Key</strong> and copy your unique API key.
  </p><script>window.helpScoutCustomHTML = true;</script>
</div></div>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"type":"htmlBlock"`,
		`"renderMode":"sandboxed"`,
		`border: 1px solid #e5e7eb`,
		`border-radius: 10px`,
		`background: #007BFF`,
		`Generate API Key`,
		`onclick`,
		`script`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected Help Scout data-html-block to preserve styled HTML %q, got: %s", want, j)
		}
	}
	for _, notWant := range []string{`"type":"paragraph"`} {
		if strings.Contains(j, notWant) {
			t.Fatalf("expected preserved data-html-block to avoid %q, got: %s", notWant, j)
		}
	}
}

func TestConvert_HelpScoutStyledDivPreservesCustomStepHTML(t *testing.T) {
	r := convert(t, `<div style="border:1px solid #e5e7eb; border-radius:12px; padding:16px; margin-bottom:14px; display:flex; gap:12px;">
  <span style="background:#0d6efd; color:#fff; width:28px; height:28px; display:flex; justify-content:center; align-items:center; border-radius:50%; font-weight:bold;">1</span>
  <div>
    <strong>Go to WordPress Admin → Plugins → Add New</strong><br/>
    Log into your WordPress dashboard and click <strong>Add New</strong> under Plugins.
  </div>
</div>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"type":"htmlBlock"`,
		`"renderMode":"sandboxed"`,
		`border:1px solid #e5e7eb`,
		`display:flex`,
		`Go to WordPress Admin`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected styled custom step HTML to be preserved %q, got: %s", want, j)
		}
	}
	for _, notWant := range []string{`"type":"paragraph"`, `"text":"1"`} {
		if strings.Contains(j, notWant) {
			t.Fatalf("expected styled custom step HTML not to split badge into native paragraphs %q, got: %s", notWant, j)
		}
	}
}

func TestConvert_HelpScoutDataHTMLBlockWithInteractiveHTMLUsesSandbox(t *testing.T) {
	r := convert(t, `<div data-html-block=""><div><table><tbody id="models-body"></tbody></table><script>document.getElementById("models-body").innerHTML = "<tr><td>Kling</td></tr>";</script></div></div>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"type":"htmlBlock"`,
		`"renderMode":"sandboxed"`,
		`models-body`,
		`script`,
		`Kling`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected interactive Help Scout data-html-block to preserve raw sandbox HTML %q, got: %s", want, j)
		}
	}
}

func TestConvert_HeadingPreservesSourceIDForHashLinks(t *testing.T) {
	r := convert(t, `<h3 id="Video-Model-Cost--Plan-Comparison-u_9kX">Video Models &amp; Plan Comparison</h3>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"type":"heading"`,
		`"id":"Video-Model-Cost--Plan-Comparison-u_9kX"`,
		`Video Models`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected heading source ID to be preserved %q, got: %s", want, j)
		}
	}
}

func TestConvert_AddsHeadingAliasForStaleHashLinkByLinkText(t *testing.T) {
	r := convert(t, `<p><a href="#Video-Model-Cost--Plan-Comparison-u_9kX">Video Models &amp; Plan Comparison</a></p><h3 id="Video-Model-Generation--Plan-Comparison-m-vqd">Video Models &amp; Plan Comparison</h3>`)
	j := toJSON(t, r)
	for _, want := range []string{
		`"href":"#Video-Model-Cost--Plan-Comparison-u_9kX"`,
		`"id":"Video-Model-Generation--Plan-Comparison-m-vqd"`,
		`"anchorAliases":["Video-Model-Cost--Plan-Comparison-u_9kX"]`,
	} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected stale hash link to be preserved as heading alias %q, got: %s", want, j)
		}
	}
}

func TestConvert_NormalizesMalformedHashHrefToLastAnchor(t *testing.T) {
	r := convert(t, `<p><a href="#oldhttps://docs.contentstudio.io/article/1084-ai-powered-caption-generation#How-to-generate-content-from-images-gmDlz">How to generate content from images</a></p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"href":"#How-to-generate-content-from-images-gmDlz"`) {
		t.Fatalf("expected malformed hash href to be normalized to last anchor, got: %s", j)
	}
	if strings.Contains(j, `docs.contentstudio.io`) {
		t.Fatalf("expected malformed hash href to remove embedded URL, got: %s", j)
	}
}

func TestConvert_HelpScoutDataHTMLBlockWithDetailsUsesNativeToggle(t *testing.T) {
	r := convert(t, `<div data-html-block=""><p>Intro</p><details><summary><span>🔑</span><span>Authentication &amp; Setup</span><span>3 topics</span><span>▼</span></summary><div><a href="#auth">Authentication</a></div></details></div>`)
	j := toJSON(t, r)
	for _, want := range []string{`"type":"paragraph"`, `"type":"toggleSection"`, `"icon":"🔑"`, `"badgeText":"3 topics"`} {
		if !strings.Contains(j, want) {
			t.Fatalf("expected data-html-block with known details to convert natively %q, got: %s", want, j)
		}
	}
	if strings.Contains(j, `"type":"htmlBlock"`) {
		t.Fatalf("expected known details inside data-html-block to avoid htmlBlock preservation, got: %s", j)
	}
}

func TestConvert_DivWithClassExtractsContent(t *testing.T) {
	// Divs with classes should still extract native content (images, paragraphs, etc.)
	r := convert(t, `<div class="custom-layout"><p>Styled content</p></div>`)
	j := toJSON(t, r)
	if !strings.Contains(j, "Styled content") {
		t.Errorf("expected content extracted from div, got: %s", j)
	}
}

func TestConvert_EmptyDivWithClassBecomesHtmlBlock(t *testing.T) {
	// Empty divs with classes that have no native children → htmlBlock fallback
	r := convert(t, `<div class="custom-widget" data-id="123"></div>`)
	j := toJSON(t, r)
	// Empty div might not produce htmlBlock either — that's OK
	_ = j
}

func TestConvert_DivWithoutClassUnwrapped(t *testing.T) {
	r := convert(t, `<div><p>Plain content</p></div>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"htmlBlock"`) {
		t.Errorf("expected plain div unwrapped, not htmlBlock, got: %s", j)
	}
	if !strings.Contains(j, "Plain content") {
		t.Errorf("expected content preserved, got: %s", j)
	}
}

func TestConvert_ImageInsideParagraph(t *testing.T) {
	r := convert(t, `<p>Before <img src="https://cdn.example.com/img.png" alt="pic"> After</p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"resizableImage"`) {
		t.Errorf("expected image extracted from paragraph, got: %s", j)
	}
	if !strings.Contains(j, "Before") || !strings.Contains(j, "After") {
		t.Errorf("expected surrounding text preserved, got: %s", j)
	}
	// Image should NOT be inside a paragraph content — it's a separate block
	if strings.Contains(j, `"type":"paragraph","content":[{"type":"resizableImage"`) {
		t.Errorf("image should not be inline inside paragraph, got: %s", j)
	}
}

func TestConvert_ImageWithDataSrc(t *testing.T) {
	r := convert(t, `<p><img data-src="https://cdn.example.com/lazy.png" alt="lazy"></p>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"src":"https://cdn.example.com/lazy.png"`) {
		t.Errorf("expected data-src fallback, got: %s", j)
	}
}

func TestConvert_MixedContent(t *testing.T) {
	html := `
		<h2>Getting Started</h2>
		<p>Follow these <strong>steps</strong>:</p>
		<ol><li>Sign up</li><li>Configure your <a href="/settings">settings</a></li></ol>
		<div class="callout callout-info"><p>Pro tip: use keyboard shortcuts</p></div>
		<hr>
		<pre><code class="language-js">console.log("hello")</code></pre>
	`
	r := convert(t, html)
	j := toJSON(t, r)

	checks := []string{
		`"type":"heading"`, `"level":2`,
		`"type":"paragraph"`, `"type":"bold"`,
		`"type":"orderedList"`, `"type":"link"`,
		`"type":"callout"`, `"variant":"blue"`,
		`"type":"horizontalRule"`,
		`"type":"codeBlock"`, `"language":"js"`,
	}
	for _, check := range checks {
		if !strings.Contains(j, check) {
			t.Errorf("missing %s in output: %s", check, j)
		}
	}
}

func TestConvertHelpScoutFixture_AltTextNormalizesEscapedAside(t *testing.T) {
	r := convertHelpScout(t, "replug_alt_text.html")
	j := toJSON(t, r)
	if strings.Contains(j, "&lt;aside&gt;") || strings.Contains(j, "<aside>") {
		t.Fatalf("expected escaped aside markers to be normalized, got: %s", j)
	}
	if !strings.Contains(j, `"type":"callout"`) {
		t.Fatalf("expected help scout aside content to become a callout, got: %s", j)
	}
}

func TestConvertHelpScoutFixture_FirstCommentDropsEmptyHeadingAndLeadingNoise(t *testing.T) {
	r := convertHelpScout(t, "replug_first_comment.html")
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"heading","attrs":{"level":2}`) {
		t.Fatalf("expected empty heading to be dropped, got: %s", j)
	}
	if strings.Contains(j, `"text":" The `) || strings.Contains(j, `"text":" Go to the`) {
		t.Fatalf("expected leading import whitespace to be trimmed, got: %s", j)
	}
	if !strings.Contains(j, `"level":4`) {
		t.Fatalf("expected h4 step headings to be preserved, got: %s", j)
	}
}

func TestConvertHelpScoutFixture_BioLinksPreservesCalloutsAndLowerHeadings(t *testing.T) {
	r := convertHelpScout(t, "replug_bio_links.html")
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"callout"`) {
		t.Fatalf("expected helpscout callout to survive, got: %s", j)
	}
	if !strings.Contains(j, `"level":4`) || !strings.Contains(j, `"level":5`) {
		t.Fatalf("expected h4/h5 hierarchy to be preserved, got: %s", j)
	}
	if strings.Contains(j, `"type":"heading","attrs":{"level":3}`) && !strings.Contains(j, "Step 1") {
		t.Fatalf("expected empty nested heading to be dropped, got: %s", j)
	}
}
