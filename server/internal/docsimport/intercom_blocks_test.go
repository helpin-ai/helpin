package docsimport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/intercom"
)

// intercomBlocks decodes blocks from the JSON a public article publishes, so
// the tests exercise the same decoding the importer does.
func intercomBlocks(t *testing.T, raw string) []intercom.Block {
	t.Helper()
	var blocks []intercom.Block
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		t.Fatalf("decode blocks: %v", err)
	}
	return blocks
}

func convertedJSON(t *testing.T, result IntercomBlockResult) string {
	t.Helper()
	// Compare readable text: leave "&" and "<" as written.
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result.Doc); err != nil {
		t.Fatalf("encode doc: %v", err)
	}
	return strings.TrimSpace(out.String())
}

func TestConvertIntercomBlocks(t *testing.T) {
	tests := []struct {
		name   string
		blocks string
		want   string
	}{
		{
			name:   "paragraph with marks and a link",
			blocks: `[{"type":"paragraph","text":"Open <b>Billing</b> and <i>click</i> <a href=\"https://acme.example/pay\">Pay &amp; go</a>.","class":"no-margin"}]`,
			want: `{"type":"doc","content":[{"type":"paragraph","content":[` +
				`{"type":"text","text":"Open "},{"type":"text","marks":[{"type":"bold"}],"text":"Billing"},` +
				`{"type":"text","text":" and "},{"type":"text","marks":[{"type":"italic"}],"text":"click"},{"type":"text","text":" "},` +
				`{"type":"text","marks":[{"type":"link","attrs":{"href":"https://acme.example/pay","rel":"noopener noreferrer","target":"_blank"}}],"text":"Pay & go"},` +
				`{"type":"text","text":"."}]}]}`,
		},
		{
			name:   "headings keep their anchors and levels",
			blocks: `[{"type":"heading","text":"Create a project","idAttribute":"create-a-project"},{"type":"subheading","text":"Express run","idAttribute":"express-run"},{"type":"subheading3","text":"GitHub"},{"type":"subheading4","text":"Methods"}]`,
			want: `{"type":"doc","content":[` +
				`{"type":"heading","attrs":{"id":"create-a-project","level":2},"content":[{"type":"text","text":"Create a project"}]},` +
				`{"type":"heading","attrs":{"id":"express-run","level":3},"content":[{"type":"text","text":"Express run"}]},` +
				`{"type":"heading","attrs":{"level":4},"content":[{"type":"text","text":"GitHub"}]},` +
				`{"type":"heading","attrs":{"level":5},"content":[{"type":"text","text":"Methods"}]}]}`,
		},
		{
			name:   "spacer paragraphs and empty headings are dropped",
			blocks: `[{"type":"paragraph","text":"\u200b"},{"type":"paragraph","text":" &nbsp; "},{"type":"heading","text":"<br>"},{"type":"paragraph","text":"Kept"}]`,
			want:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Kept"}]}]}`,
		},
		{
			name:   "line breaks inside a paragraph stay, outer ones go",
			blocks: `[{"type":"paragraph","text":"<br>one<br/>two<br>"}]`,
			want:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"one"},{"type":"hardBreak"},{"type":"text","text":"two"}]}]}`,
		},
		{
			name:   "inline code never combines with other marks",
			blocks: `[{"type":"paragraph","text":"Run <b><code>make build</code></b> now"}]`,
			want: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Run "},` +
				`{"type":"text","marks":[{"type":"code"}],"text":"make build"},{"type":"text","text":" now"}]}]}`,
		},
		{
			name:   "an image tag or a stray end tag does not leak formatting",
			blocks: `[{"type":"paragraph","text":"<b>Note <img src=x> this</b> rest</i> <a href=\"https://acme.example\">link <input></a> after"}]`,
			want: `{"type":"doc","content":[{"type":"paragraph","content":[` +
				`{"type":"text","marks":[{"type":"bold"}],"text":"Note "},{"type":"text","marks":[{"type":"bold"}],"text":" this"},` +
				`{"type":"text","text":" rest"},{"type":"text","text":" "},` +
				`{"type":"text","marks":[{"type":"link","attrs":{"href":"https://acme.example","rel":"noopener noreferrer","target":"_blank"}}],"text":"link "},` +
				`{"type":"text","text":" after"}]}]}`,
		},
		{
			name:   "an unclosed inner tag ends with its parent",
			blocks: `[{"type":"paragraph","text":"<b>bold <i>both</b> plain"}]`,
			want: `{"type":"doc","content":[{"type":"paragraph","content":[` +
				`{"type":"text","marks":[{"type":"bold"}],"text":"bold "},{"type":"text","marks":[{"type":"bold"},{"type":"italic"}],"text":"both"},` +
				`{"type":"text","text":" plain"}]}]}`,
		},
		{
			name:   "nested bold is one mark",
			blocks: `[{"type":"paragraph","text":"<b><strong>twice</strong></b>"}]`,
			want:   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","marks":[{"type":"bold"}],"text":"twice"}]}]}`,
		},
		{
			name:   "lists start each item with a paragraph",
			blocks: `[{"type":"unorderedNestedList","items":[{"content":[{"type":"paragraph","text":"one"}]},{"content":[{"type":"image","url":"https://cdn.example/a.png"}]}]},{"type":"orderedNestedList","items":[{"content":[{"type":"paragraph","text":"first"},{"type":"unorderedNestedList","items":[{"content":[{"type":"paragraph","text":"inner"}]}]}]}]}]`,
			want: `{"type":"doc","content":[` +
				`{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},` +
				`{"type":"listItem","content":[{"type":"paragraph"},{"type":"resizableImage","attrs":{"alt":"","src":"https://cdn.example/a.png"}}]}]},` +
				`{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]},` +
				`{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"inner"}]}]}]}]}]}]}`,
		},
		{
			name:   "code keeps line breaks and literal markup",
			blocks: `[{"type":"code","text":"make build<br>./run --flag=&lt;value&gt;"},{"type":"code","text":""}]`,
			want: `{"type":"doc","content":[{"type":"codeBlock","content":[{"type":"text","text":"make build\n./run --flag=<value>"}]},` +
				`{"type":"codeBlock"}]}`,
		},
		{
			name:   "callout colors map to styles",
			blocks: `[{"type":"callout","content":[{"type":"paragraph","text":"Heads up"}],"style":{"backgroundColor":"#feedaf80","borderColor":"#000"}},{"type":"callout","content":[],"style":{"backgroundColor":"#e8e8e880"}}]`,
			want: `{"type":"doc","content":[` +
				`{"type":"callout","attrs":{"variant":"warning"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Heads up"}]}]},` +
				`{"type":"callout","attrs":{"variant":"grey"},"content":[{"type":"paragraph"}]}]}`,
		},
		{
			name:   "table cells always hold a block",
			blocks: `[{"type":"table","rows":[{"cells":[{"content":[{"type":"paragraph","text":"<b>Name</b>"}]},{"content":[]}]}]}]`,
			want: `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[` +
				`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","marks":[{"type":"bold"}],"text":"Name"}]}]},` +
				`{"type":"tableCell","content":[{"type":"paragraph"}]}]}]}]}`,
		},
		{
			name:   "collapsible section takes its title from either summary shape",
			blocks: `[{"type":"collapsibleSection","summary":{"text":"<b>More</b> detail"},"content":[{"type":"paragraph","text":"Inside"}]},{"type":"collapsibleSection","summary":"Plain","content":[]},{"type":"collapsibleSection","content":[]}]`,
			want: `{"type":"doc","content":[` +
				`{"type":"toggleSection","attrs":{"title":"More detail"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Inside"}]}]},` +
				`{"type":"toggleSection","attrs":{"title":"Plain"},"content":[{"type":"paragraph"}]},` +
				`{"type":"toggleSection","attrs":{"title":"Details"},"content":[{"type":"paragraph"}]}]}`,
		},
		{
			name:   "video and rule",
			blocks: `[{"type":"video","provider":"youtube","id":"-8acWGAJsPI"},{"type":"horizontalRule","text":"____"},{"type":"video","provider":"Vimeo","id":76979871}]`,
			want: `{"type":"doc","content":[` +
				`{"type":"videoEmbed","attrs":{"embedUrl":"https://www.youtube.com/embed/-8acWGAJsPI","provider":"youtube","sourceUrl":"https://www.youtube.com/watch?v=-8acWGAJsPI"}},` +
				`{"type":"horizontalRule"},` +
				`{"type":"videoEmbed","attrs":{"embedUrl":"https://player.vimeo.com/video/76979871","provider":"vimeo","sourceUrl":"https://vimeo.com/76979871"}}]}`,
		},
		{
			name:   "empty article is one empty paragraph",
			blocks: `[]`,
			want:   `{"type":"doc","content":[{"type":"paragraph"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertIntercomBlocks(intercomBlocks(t, tt.blocks), IntercomBlockOptions{})
			if got := convertedJSON(t, result); got != tt.want {
				t.Fatalf("converted\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestConvertIntercomBlocksImages(t *testing.T) {
	blocks := intercomBlocks(t, `[
		{"type":"image","url":"https://cdn.example/a.png?signature=1","text":"<b>Dashboard</b> view","width":2448,"height":279,"displayWidth":550,"align":"center"},
		{"type":"image","url":"https://cdn.example/broken.png","displayWidth":"120","align":"justify","linkUrl":"https://acme.example/full"},
		{"type":"image","url":""}
	]`)
	result := ConvertIntercomBlocks(blocks, IntercomBlockOptions{CopyImage: func(source string) string {
		if strings.Contains(source, "broken") {
			return ""
		}
		return "https://files.helpin.example/copied.png"
	}})
	want := `{"type":"doc","content":[` +
		`{"type":"resizableImage","attrs":{"alignment":"center","alt":"Dashboard view","src":"https://files.helpin.example/copied.png","width":"550px"}},` +
		`{"type":"resizableImage","attrs":{"alt":"","linkNewTab":true,"linkUrl":"https://acme.example/full","src":"https://cdn.example/broken.png","width":"120px"}}]}`
	if got := convertedJSON(t, result); got != want {
		t.Fatalf("converted\n got: %s\nwant: %s", got, want)
	}
	if len(result.Images) != 2 || result.Images[0].Stored == "" || result.Images[1].Stored != "" ||
		result.Images[1].Source != "https://cdn.example/broken.png" {
		t.Fatalf("images = %+v", result.Images)
	}
}

func TestConvertIntercomBlocksDropsUnsafeAddresses(t *testing.T) {
	blocks := intercomBlocks(t, `[
		{"type":"paragraph","text":"<a href=\"javascript:alert(1)\">click</a> <a href=\" JaVaScRiPt:alert(1)\">again</a> <a href=\"data:text/html,x\">data</a> <a href=\"mailto:help@acme.example\">mail</a> <a href=\"/pricing\">relative</a>"},
		{"type":"image","url":"javascript:alert(1)"},
		{"type":"image","url":"data:image/png;base64,AAAA"},
		{"type":"image","url":"https://cdn.example/a.png","linkUrl":"javascript:alert(1)"}
	]`)
	copied := 0
	result := ConvertIntercomBlocks(blocks, IntercomBlockOptions{CopyImage: func(string) string { copied++; return "" }})
	got := convertedJSON(t, result)
	lower := strings.ToLower(got)
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") {
		t.Fatalf("an unsafe address survived conversion: %s", got)
	}
	// The link text stays readable, and the safe links stay links.
	for _, want := range []string{`"text":"click"`, `"text":"again"`, `"text":"data"`, `"href":"mailto:help@acme.example"`, `"href":"/pricing"`, `"src":"https://cdn.example/a.png"`} {
		if !strings.Contains(got, want) {
			t.Errorf("converted document lacks %s\n%s", want, got)
		}
	}
	if strings.Contains(got, "linkUrl") || len(result.Images) != 1 || copied != 1 {
		t.Fatalf("images = %+v, copy attempts = %d, doc = %s", result.Images, copied, got)
	}
}

func TestConvertIntercomBlocksWarnings(t *testing.T) {
	blocks := intercomBlocks(t, `[
		{"type":"messengerCard","text":"Card <b>text</b>"},
		{"type":"messengerCard","text":"Again"},
		{"type":"callout","content":[],"style":{"backgroundColor":"#123456"}},
		{"type":"video","provider":"dailymotion","id":"x1"},
		{"type":"paragraph","text":"<mark>hi</mark> there"}
	]`)
	result := ConvertIntercomBlocks(blocks, IntercomBlockOptions{})
	want := `{"type":"doc","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Card "},{"type":"text","marks":[{"type":"bold"}],"text":"text"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"Again"}]},` +
		`{"type":"callout","attrs":{"variant":"info"},"content":[{"type":"paragraph"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"hi"},{"type":"text","text":" there"}]}]}`
	if got := convertedJSON(t, result); got != want {
		t.Fatalf("converted\n got: %s\nwant: %s", got, want)
	}
	var kinds []string
	for _, warning := range result.Warnings {
		kinds = append(kinds, warning.Type)
	}
	wantKinds := []string{WarningIntercomUnknownBlock, WarningIntercomCalloutColor, WarningIntercomVideoDropped, WarningIntercomUnknownInline}
	if strings.Join(kinds, ",") != strings.Join(wantKinds, ",") {
		t.Fatalf("warnings = %v, want each kind once: %v", kinds, wantKinds)
	}
}

func TestConvertIntercomBlocksStopsAtDeepNesting(t *testing.T) {
	raw := `{"type":"paragraph","text":"bottom"}`
	for range 40 {
		raw = `{"type":"callout","content":[` + raw + `]}`
	}
	result := ConvertIntercomBlocks(intercomBlocks(t, "["+raw+"]"), IntercomBlockOptions{})
	if got := convertedJSON(t, result); strings.Contains(got, "bottom") || strings.Count(got, `"callout"`) > intercomBlockMaxDepth+1 {
		t.Fatalf("deep nesting was followed to the end: %d callouts", strings.Count(got, `"callout"`))
	}
}
