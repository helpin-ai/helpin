package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderHTML_Paragraph(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<p>Hello world</p>") {
		t.Errorf("expected paragraph, got: %s", got)
	}
}

func TestRenderHTML_Heading(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Getting Started"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<h2 id="getting-started">Getting Started</h2>`) {
		t.Errorf("expected h2 with id, got: %s", got)
	}
}

func TestRenderHTML_BoldItalicStrike(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"bold"}],"text":"bold"},
		{"type":"text","text":" "},
		{"type":"text","marks":[{"type":"italic"}],"text":"italic"},
		{"type":"text","text":" "},
		{"type":"text","marks":[{"type":"strike"}],"text":"strike"}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("missing bold: %s", got)
	}
	if !strings.Contains(got, "<em>italic</em>") {
		t.Errorf("missing italic: %s", got)
	}
	if !strings.Contains(got, "<s>strike</s>") {
		t.Errorf("missing strike: %s", got)
	}
}

func TestRenderHTML_Link(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"link","attrs":{"href":"https://example.com"}}],"text":"click here"}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `href="https://example.com"`) {
		t.Errorf("missing link href: %s", got)
	}
	if !strings.Contains(got, "click here</a>") {
		t.Errorf("missing link text: %s", got)
	}
}

func TestRenderHTML_BulletList(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"bulletList","content":[
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 1"}]}]},
		{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item 2"}]}]}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<li>") {
		t.Errorf("expected list structure, got: %s", got)
	}
	if !strings.Contains(got, "Item 1") || !strings.Contains(got, "Item 2") {
		t.Errorf("missing list items, got: %s", got)
	}
}

func TestRenderHTML_CodeBlock(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println(\"hello\")"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="language-go"`) {
		t.Errorf("missing language class: %s", got)
	}
	if !strings.Contains(got, "fmt.Println") {
		t.Errorf("missing code content: %s", got)
	}
	// Verify HTML escaping in code (Go uses &#34; for quotes)
	if !strings.Contains(got, "&#34;hello&#34;") {
		t.Errorf("code content should be escaped: %s", got)
	}
}

func TestRenderHTML_Image(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://img.example.com/photo.png","alt":"A photo","width":"50%","caption":"Architecture diagram"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="https://img.example.com/photo.png"`) {
		t.Errorf("missing image src: %s", got)
	}
	if !strings.Contains(got, `alt="A photo"`) {
		t.Errorf("missing alt: %s", got)
	}
	if !strings.Contains(got, `style="width:50%"`) {
		t.Errorf("missing width style: %s", got)
	}
	if !strings.Contains(got, `class="docs-image-block"`) {
		t.Errorf("missing image block wrapper class: %s", got)
	}
	if !strings.Contains(got, `<figcaption>Architecture diagram</figcaption>`) {
		t.Errorf("missing image caption: %s", got)
	}
}

func TestRenderHTML_ImageThemeVariants(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"https://img.example.com/light.png","darkSrc":"https://img.example.com/dark.png","alt":"Diagram","width":"100%"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="docs-theme-image-set"`,
		`src="https://img.example.com/light.png"`,
		`src="https://img.example.com/dark.png"`,
		`docs-theme-image-light`,
		`docs-theme-image-dark`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in theme image HTML, got: %s", want, got)
		}
	}
}

func TestRenderHTML_Blockquote(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"A wise quote"}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<blockquote>") || !strings.Contains(got, "A wise quote") {
		t.Errorf("expected blockquote, got: %s", got)
	}
}

func TestRenderHTML_Callout(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"callout","attrs":{"variant":"yellow"},"content":[{"type":"paragraph","content":[{"type":"text","text":"This is a warning."}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<aside class="docs-callout docs-callout--warning"`) {
		t.Errorf("expected callout aside with variant class, got: %s", got)
	}
	if !strings.Contains(got, "This is a warning.") {
		t.Errorf("expected callout content, got: %s", got)
	}
}

func TestRenderHTML_CalloutDefaultVariant(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"callout","content":[{"type":"paragraph","content":[{"type":"text","text":"No variant."}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `docs-callout--info`) {
		t.Errorf("expected default info variant, got: %s", got)
	}
}

func TestRenderHTML_TaskItemMetadata(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":false,"assigneeName":"Ada","dueDate":"2026-05-01"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Follow up"}]}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-assignee-name="Ada"`, `data-due-date="2026-05-01"`, `<span class="task-item-meta">Ada 2026-05-01</span>`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rendered task item, got: %s", want, got)
		}
	}
}

func TestRenderHTML_ToggleSection(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"toggleSection","attrs":{"title":"More context","open":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"Hidden until expanded."}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<details class="docs-toggle-section" data-toggle-section open`, `<summary>More context</summary>`, `Hidden until expanded.`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rendered toggle, got: %s", want, got)
		}
	}
}

func TestRenderHTML_ToggleSectionWithHelpScoutMetadata(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"toggleSection","attrs":{"title":"Authentication & Setup","icon":"🔑","badgeText":"3 topics","sourceStyle":"helpScoutCard","open":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"How to Get Your API Key"}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-toggle-icon="🔑"`,
		`data-toggle-badge="3 topics"`,
		`data-toggle-style="helpScoutCard"`,
		`<span class="docs-toggle-icon">🔑</span>`,
		`<span class="docs-toggle-badge">3 topics</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rendered Help Scout toggle, got: %s", want, got)
		}
	}
}

func TestRenderHTML_HtmlBlockAllowsSafeDataImage(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"html":"<div class=\"docs-fb-background-grid\"><div class=\"docs-fb-background-card\"><img src=\"data:image/png;base64,iVBORw0KGgo=\" width=\"36\" height=\"36\"><code>106018623298955</code></div></div>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`docs-fb-background-grid`, `src="data:image/png;base64,iVBORw0KGgo="`, `106018623298955`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rendered HTML block, got: %s", want, got)
		}
	}
}

func TestRenderHTML_HtmlBlockAllowsSafePresentationStyles(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"html":"<div style=\"border: 1px solid #e5e7eb; border-radius: 10px; padding: 16px; margin-bottom:16px; display:flex; align-items:flex-start; position:absolute\"><ol style=\"list-style:none; padding:0; margin:0\"><li style=\"margin-bottom:12px\"><span style=\"background: #007BFF;color:#fff;width:24px;height:24px;line-height:24px;text-align:center;display: inline-block;border-radius:50%;font-weight:bold; flex-shrink:0; background-image:url(javascript:alert(1))\">2</span> Click on <strong>Generate API Key</strong>.</li></ol><script>alert(1)</script></div>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`border: 1px solid #e5e7eb`, `border-radius: 10px`, `padding: 16px`, `list-style: none`, `background: #007BFF`, `display: flex`, `align-items: flex-start`, `display: inline-block`, `flex-shrink: 0`, `Generate API Key`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rendered HTML block, got: %s", want, got)
		}
	}
	for _, notWant := range []string{`position:absolute`, `background-image`, `javascript`, `<script`} {
		if strings.Contains(got, notWant) {
			t.Fatalf("expected unsafe style/content %q to be stripped, got: %s", notWant, got)
		}
	}
}

func TestRenderHTML_RawHtmlBlockRendersInlineWithoutIframe(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"renderMode":"sandboxed","html":"<div><table><tbody id=\"models-body\"></tbody></table><script>document.getElementById('models-body').innerHTML='<tr><td>Kling</td></tr>';</script></div>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<div class="docs-html-block docs-html-block--raw">`, `<script>document`, `models-body`, `Kling`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected raw inline HTML block output %q, got: %s", want, got)
		}
	}
	for _, notWant := range []string{`<iframe`, `srcdoc=`, `&lt;script&gt;`} {
		if strings.Contains(got, notWant) {
			t.Fatalf("expected raw inline HTML block to avoid iframe artifact %q, got: %s", notWant, got)
		}
	}
}

func TestRenderHTML_RawHtmlBlockDoesNotInjectFontWrapper(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"renderMode":"sandboxed","html":"<div style=\"font-family: Georgia, serif\">Keep source font</div><p>Fallback text</p>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`Keep source font`, `font-family: Georgia, serif`, `Fallback text`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected raw inline HTML block to contain %q, got: %s", want, got)
		}
	}
	if strings.Contains(got, `font-family: ui-sans-serif`) {
		t.Fatalf("expected raw inline HTML block not to inject an iframe font wrapper, got: %s", got)
	}
}

func TestRenderHTML_RawHtmlBlockKeepsSourceIframe(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"renderMode":"sandboxed","html":"<iframe src=\"https://www.youtube.com/embed/abc123\" width=\"560\" height=\"315\"></iframe>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<div class="docs-html-block docs-html-block--raw">`, `<iframe src="https://www.youtube.com/embed/abc123" width="560" height="315"></iframe>`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected raw inline HTML block to keep source iframe %q, got: %s", want, got)
		}
	}
	for _, notWant := range []string{`docs-html-block-frame`, `srcdoc=`} {
		if strings.Contains(got, notWant) {
			t.Fatalf("expected no generated iframe wrapper artifact %q, got: %s", notWant, got)
		}
	}
}

func TestRenderHTML_HeadingUsesSourceIDWhenPresent(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"heading","attrs":{"level":3,"id":"Video-Model-Cost--Plan-Comparison-u_9kX"},"content":[{"type":"text","text":"Video Models & Plan Comparison"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<h3 id="Video-Model-Cost--Plan-Comparison-u_9kX">`) {
		t.Fatalf("expected rendered heading to use source ID, got: %s", got)
	}
	if strings.Contains(got, `id="video-models-plan-comparison"`) {
		t.Fatalf("expected rendered heading not to regenerate ID when source ID exists, got: %s", got)
	}
}

func TestRenderHTML_HeadingRendersAnchorAliases(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"heading","attrs":{"level":3,"id":"Video-Model-Generation--Plan-Comparison-m-vqd","anchorAliases":["Video-Model-Cost--Plan-Comparison-u_9kX"]},"content":[{"type":"text","text":"Video Models & Plan Comparison"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<span id="Video-Model-Cost--Plan-Comparison-u_9kX" class="docs-heading-anchor-alias" aria-hidden="true"></span>`,
		`<h3 id="Video-Model-Generation--Plan-Comparison-m-vqd">`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected heading alias output %q, got: %s", want, got)
		}
	}
}

func TestRenderHTML_FileAttachment(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"fileAttachment","attrs":{"fileName":"report.pdf","contentType":"application/pdf","url":"https://example.com/report.pdf"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="docs-file-attachment"`, `data-content-type="application/pdf"`, `>report.pdf</a>`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in file attachment HTML, got: %s", want, got)
		}
	}
}

func TestRenderHTML_TableOfContents(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"tableOfContents"}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `data-docs-toc`) {
		t.Fatalf("expected toc marker, got: %s", got)
	}
	if strings.Contains(got, `docs-table-of-contents`) {
		t.Fatalf("expected unboxed toc HTML, got: %s", got)
	}
}

func TestRenderHTML_RichEmbed(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"richEmbed","attrs":{"url":"https://github.com/helpin-ai/helpin","provider":"GitHub","title":"GitHub link"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="docs-rich-embed"`) || !strings.Contains(got, `GitHub: GitHub link`) {
		t.Fatalf("expected rich embed HTML, got: %s", got)
	}
}

func TestRenderHTML_AISection(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"aiSection","attrs":{"title":"Support summary","status":"approved"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Generated answer."}]}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Generated answer.") {
		t.Errorf("expected AI section content, got: %s", got)
	}
	if strings.Contains(got, "docs-ai-section") || strings.Contains(got, "Support summary") || strings.Contains(got, "approved") {
		t.Errorf("expected public AI section render to omit editor metadata, got: %s", got)
	}
}

func TestRenderHTML_EntityEmbed(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"entityEmbed","attrs":{"entityType":"support_conversation","entityId":"conv-1","title":"Refund request","displayId":"42","status":"open"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="docs-entity-embed"`) {
		t.Errorf("expected entity embed wrapper, got: %s", got)
	}
	if !strings.Contains(got, `data-entity-type="support_conversation"`) {
		t.Errorf("expected support entity type, got: %s", got)
	}
	if !strings.Contains(got, `data-entity-id="conv-1"`) {
		t.Errorf("expected entity id, got: %s", got)
	}
	if !strings.Contains(got, "Refund request") {
		t.Errorf("expected entity title, got: %s", got)
	}
}

func TestRenderHTML_CitationBlock(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"citationBlock","attrs":{"title":"Sources","sources":[{"sourceType":"docs_chunk","sourceId":"chunk-1","title":"Refund policy","excerpt":"Refunds are available within 30 days.","url":"https://help.example/refunds","confidence":0.87},{"sourceType":"support_conversation","sourceId":"conv-1","title":"Hidden transcript","access":"redacted","excerpt":"private"}]}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`class="docs-citation-block"`,
		`data-source-type="docs_chunk"`,
		`Refund policy`,
		`87%`,
		`Restricted source`,
		`Hidden because this viewer cannot access the underlying source.`,
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("expected %q in citation block HTML, got: %s", fragment, got)
		}
	}
	if strings.Contains(got, "private") {
		t.Errorf("expected redacted excerpt to be hidden, got: %s", got)
	}
}

func TestRenderHTML_VideoEmbed(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"videoEmbed","attrs":{"provider":"youtube","sourceUrl":"https://www.youtube.com/watch?v=abc123","embedUrl":"https://www.youtube.com/embed/abc123"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<iframe src="https://www.youtube.com/embed/abc123"`) {
		t.Errorf("expected youtube iframe, got: %s", got)
	}
	if !strings.Contains(got, `docs-video-embed`) {
		t.Errorf("expected video embed wrapper, got: %s", got)
	}
	if !strings.Contains(got, `referrerpolicy="strict-origin-when-cross-origin"`) {
		t.Errorf("expected YouTube-compatible referrer policy, got: %s", got)
	}
}

func TestRenderHTML_VideoEmbedUnsafeURL(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"videoEmbed","attrs":{"provider":"evil","sourceUrl":"https://evil.com","embedUrl":"https://evil.com/hack"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "iframe") {
		t.Errorf("expected no iframe for unsafe URL, got: %s", got)
	}
}

func TestRenderHTML_HTMLBlock(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"html":"<div class=\"custom\"><p>Safe content</p></div>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `docs-html-block`) {
		t.Errorf("expected html block wrapper, got: %s", got)
	}
	if !strings.Contains(got, "Safe content") {
		t.Errorf("expected safe content preserved, got: %s", got)
	}
}

func TestRenderHTML_HTMLBlockSanitizesScript(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"htmlBlock","attrs":{"html":"<p>Hello</p><script>alert('xss')</script>"}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "script") {
		t.Errorf("expected script stripped, got: %s", got)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected safe text preserved, got: %s", got)
	}
}

func TestRenderHTML_Excalidraw(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"excalidraw","attrs":{"title":"Checkout flow","scene":{"elements":[{"id":"a","type":"rectangle"}],"appState":{},"files":{}}}}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="docs-excalidraw-block"`) {
		t.Errorf("expected excalidraw block wrapper, got: %s", got)
	}
	if !strings.Contains(got, "Checkout flow") {
		t.Errorf("expected title fallback, got: %s", got)
	}
	if strings.Contains(got, "rectangle") {
		t.Errorf("expected scene JSON not to be rendered, got: %s", got)
	}
}

func TestRenderHTML_HorizontalRule(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"horizontalRule"}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<hr>") {
		t.Errorf("expected hr, got: %s", got)
	}
}

func TestRenderHTML_NestedMarks(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"bold"},{"type":"italic"}],"text":"bold italic"}
	]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<strong><em>bold italic</em></strong>") {
		t.Errorf("expected nested marks, got: %s", got)
	}
}

func TestRenderHTML_Empty(t *testing.T) {
	got, err := RenderHTML(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty string, got: %s", got)
	}
}

func TestRenderHTML_XSSPrevention(t *testing.T) {
	input := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"<script>alert('xss')</script>"}]}]}`
	got, err := RenderHTML(json.RawMessage(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("XSS not prevented: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag, got: %s", got)
	}
}
