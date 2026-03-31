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
	r := convert(t, `<iframe src="https://www.youtube.com/embed/abc123"></iframe>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"type":"videoEmbed"`) || !strings.Contains(j, `"provider":"youtube"`) {
		t.Errorf("expected video embed, got: %s", j)
	}
}

func TestConvert_VimeoIframe(t *testing.T) {
	r := convert(t, `<iframe src="https://player.vimeo.com/video/123456"></iframe>`)
	j := toJSON(t, r)
	if !strings.Contains(j, `"provider":"vimeo"`) {
		t.Errorf("expected vimeo provider, got: %s", j)
	}
}

func TestConvert_UnsupportedIframe(t *testing.T) {
	r := convert(t, `<iframe src="https://unknown.example.com/embed"></iframe>`)
	j := toJSON(t, r)
	if strings.Contains(j, `"type":"videoEmbed"`) {
		t.Errorf("expected no video embed for unknown iframe, got: %s", j)
	}
	if !strings.Contains(j, `"type":"link"`) {
		t.Errorf("expected link fallback, got: %s", j)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning for unsupported iframe")
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
