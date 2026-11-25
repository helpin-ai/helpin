package llmstxt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

func TestArticle_CleansUpEachPlatformsMarkdown(t *testing.T) {
	tests := []struct {
		name            string
		markdown        string
		contentType     string
		wantTitle       string
		wantDescription string
		// want are parts of the HTML that must be there, and absent parts that
		// must not; "BASE" stands for the site's address.
		want   []string
		absent []string
	}{
		{
			name: "Mintlify notes, description, code options, cards and signature",
			markdown: "> ## Documentation Index\n> Fetch the complete documentation index at: BASE/docs/llms.txt\n\n" +
				"> ## Agent Instructions\n> Run `acme instructions` first.\n\n# Send an email\n\n> Learn how to send.\n\n" +
				"Install the SDK.\n\n```ts theme={\"theme\":{\"light\":\"github-light\"}}\nconst a = 1;\n```\n\n" +
				"``` theme={\"theme\":\"x\"}\nplain\n```\n\n" +
				"> [card] [Quickstart](BASE/docs/quickstart.md)\n>\n> Start here\n\n> [tab] First tab\n>\n> ```json theme={null}\n> {}\n> ```\n\n" +
				"<ParamField query=\"limit\" type=\"number\" required>\n  Page size.\n</ParamField>\n\n" +
				"See [domains](/docs/domains.md) and ![shot](images/shot.png).\n\n\n" +
				"This documentation is built and hosted on [Mintlify](https://mintlify.com), a developer documentation platform.",
			contentType: "text/markdown; charset=utf-8", wantTitle: "Send an email", wantDescription: "Learn how to send.",
			want: []string{
				"<p>Install the SDK.</p>", `<pre><code class="language-ts">const a = 1;`, "<pre><code>plain",
				`<a href="BASE/docs/quickstart.md">Quickstart</a>`, "<p>Start here</p>",
				"<p><strong>First tab</strong></p>\n<pre><code class=\"language-json\">{}", "<p><strong>limit</strong></p>\n<p>Page size.</p>",
				`<a href="BASE/docs/domains.md">domains</a>`, `<img src="BASE/docs/images/shot.png" alt="shot"/>`,
			},
			absent: []string{"Documentation Index", "Agent Instructions", "<h1", "Learn how to send", "theme", "[card]", "[tab]", "paramfield", "Mintlify"},
		},
		{
			name:      "Ferndesk title in brackets",
			markdown:  "[Content blocks] \n Press **/** in the editor.\n\n## Every block\n\nText.",
			wantTitle: "Content blocks",
			want:      []string{"<p>Press <strong>/</strong> in the editor.</p>", "<h2>Every block</h2>"},
			absent:    []string{"[Content blocks]", "<h1"},
		},
		{
			name: "GitBook tags",
			markdown: "> For the complete documentation index, see [llms.txt](BASE/docs/llms.txt).\n\n# Quickstart\n\nIntro.\n\n" +
				"{% stepper %}\n{% step %}\n\n#### Create\n\n1. Click **New**.\n   {% endstep %}\n\n{% endstepper %}\n\n" +
				"{% hint style=\"info\" %}\nMind the `{% hint %}` tag.\n\n```bash\nrun it\n```\n{% endhint %}\n\nAfter the hint.\n\n" +
				"{% tabs %}\n{% tab title=\"Visual editor\" icon=\"hand\" %}\nEdit visually.\n{% endtab %}\n{% endtabs %}\n\n" +
				"{% code title=\"config.yml\" overflow=\"wrap\" %}\n\n```yaml\na: 1\n```\n\n{% endcode %}\n\n" +
				"{% embed url=\"https://video.example/watch?v=1\" %}\n\n" +
				"{% content-ref url=\"other.md\" %}\n[other.md](other.md)\n{% endcontent-ref %}\n\n" +
				"<i class=\"fa-arrow-down\">:arrow-down:</i> [Jump](#publish)",
			wantTitle: "Quickstart",
			want: []string{
				"<h4>Create</h4>", "<li>Click <strong>New</strong>.</li>",
				"<blockquote>\n<p>Mind the <code>{% hint %}</code> tag.</p>\n<pre><code class=\"language-bash\">run it\n</code></pre>\n</blockquote>",
				"<p>After the hint.</p>", "<p><strong>Visual editor</strong></p>", "<p>Edit visually.</p>",
				"<p><strong>config.yml</strong></p>", `<pre><code class="language-yaml">a: 1`,
				`<a href="https://video.example/watch?v=1">https://video.example/watch?v=1</a>`,
				`<a href="BASE/docs/other.md">other.md</a>`, `<p><a href="#publish">Jump</a></p>`,
			},
			absent: []string{"{% s", "{% e", "{% t", "{% c", "llms.txt", ":arrow-down:", "<h1"},
		},
		{
			name: "ReadMe components and page lists",
			markdown: "---\nupdatedAt: 2026-01-02T03:04:05.000Z\nagentTools:\n  siteIndex: BASE/llms.txt\n---\n\n# Welcome\n\nHello.\n\n" +
				"<Cards columns=\"2\">\n  <Card title=\"Quick Start\" href=\"/docs/quickstart\" icon=\"fad fa-forward\">\n    Learn to publish\n  </Card>\n</Cards>\n\n" +
				"<Callout icon=\"📘\" theme=\"info\">\n  Heads up.\n</Callout>\n\n" +
				"<Image align=\"center\" alt=\"Diagram\" src=\"https://files.example/diagram.png\" />\n\n" +
				"<PromoCard alt=\"\" header=\"Support\" href=\"BASE/docs/support\" src=\"https://files.example/s.webp\" />\n\n" +
				"Use the `<Card>` component.\n\n# Sub pages\n\n* [Child](BASE/docs/child.md)\n\n# Sibling pages\n\n* [Other](BASE/docs/other.md)",
			wantTitle: "Welcome",
			want: []string{
				"<p>Hello.</p>", `<p><strong><a href="BASE/docs/quickstart">Quick Start</a></strong></p>`, "<p>Learn to publish</p>",
				"<p>Heads up.</p>", `<img src="https://files.example/diagram.png" alt="Diagram"/>`,
				`<strong><a href="BASE/docs/support">Support</a></strong>`, "<code>&lt;Card&gt;</code>",
			},
			absent: []string{"updatedAt", "siteIndex", "<cards", "<card ", "callout", "Sub pages", "Sibling pages", "child.md", "<h1"},
		},
		{
			name: "Archbee front matter and directives",
			markdown: "---\ntitle: Welcome to Acme!\nslug: docs\ndescription: 'All of Acme''s guides.'\n---\n\nIntro text.\n\n" +
				"::::LinkArray{contentSource=\"CUSTOM\"}\n:::LinkArrayItem{headerType=\"IMAGE\" headerImage=\"https://img.example/a.png\"}\n" +
				"### [Product features](BASE/docs/learn)\n:::\n::::\n\n" +
				"::Image[]{src=\"https://img.example/b.png\" size=\"80\" alt=\"Editor\"}\n\n" +
				":::hint{type=\"info\"}\nKeep this.\n:::\n\n### :FontAwesome[]{icon=\"fa-solid fa-inbox\"} Need help?",
			wantTitle: "Welcome to Acme!", wantDescription: "All of Acme's guides.",
			want: []string{
				"<p>Intro text.</p>", `<h3><a href="BASE/docs/learn">Product features</a></h3>`,
				`<img src="https://img.example/b.png" alt="Editor"/>`, "<p>Keep this.</p>", "<h3>Need help?</h3>",
			},
			absent: []string{":::", "LinkArray", "FontAwesome", "slug", "title:"},
		},
		{
			name: "Document360 front matter, note and indented layout",
			markdown: "---\ntitle: \"Quick \\\"start\\\"\"\ndescription: \"Set up fast.\"\nupdated: 2026-06-02T07:10:23Z\n---\n\n" +
				"> ## Documentation Index\n> Fetch the complete documentation index at: BASE/llms.txt\n\n# Quick \"start\"\n\nGet going.\n\n" +
				"  \n  1\n  \n    \n\n## Create a category\n\n    [Categories](BASE/docs/categories) are folders.\n    \n      \n        a\n" +
				"        Navigate to **Documentation**.\n\n- Item\n\n      ```\n      still code in a list\n      ```\n",
			contentType: "text/plain; charset=utf-8", wantTitle: `Quick "start"`, wantDescription: "Set up fast.",
			want: []string{
				"<p>Get going.</p>", `<p><a href="BASE/docs/categories">Categories</a> are folders.</p>`,
				"Navigate to <strong>Documentation</strong>.", "still code in a list",
			},
			absent: []string{"Documentation Index", "<h1", "<pre><code>[Categories]", "<pre><code>a", "updated"},
		},
		{
			name:      "Featurebase alert",
			markdown:  "# Security FAQ\n\nWe take it seriously.\n\n> [!NOTE]\n> **The short answer:** yes.\n\n# General\n\nMore.",
			wantTitle: "Security FAQ",
			want:      []string{"<blockquote>\n<p><strong>Note</strong>\n<strong>The short answer:</strong> yes.</p>\n</blockquote>", "<h1>General</h1>"},
			absent:    []string{"[!NOTE]", "<h1>Security FAQ</h1>"},
		},
		{
			name: "Fern page without a title",
			markdown: "> This page is for Acme CLI.\n\n> For clean Markdown content of this page, append .md to this URL. " +
				"For the complete documentation index, see BASE/learn/llms.txt.\n\n## 4.39.0\n\n> **Tip**\n>\n> Upgrade now.\n\nFixed a bug.",
			wantTitle: "Guide",
			want:      []string{"<h2>4.39.0</h2>", "<p><strong>Tip</strong></p>", "<p>Fixed a bug.</p>"},
			absent:    []string{"This page is for", "llms.txt"},
		},
		{
			name: "Kustomer address and date",
			markdown: "# Analyze data\n\n> Run a prompt.\n\nSource: BASE/docs/guide\n\nLast updated: 2026-11-20T14:28:50.125Z\n\n" +
				"**Use this if** you want an answer.",
			wantTitle: "Analyze data", wantDescription: "Run a prompt.",
			want:   []string{"<p><strong>Use this if</strong> you want an answer.</p>"},
			absent: []string{"Source:", "Last updated", "Run a prompt"},
		},
		{
			name: "Plain front matter without a title and a callout below the title",
			markdown: "---\nauthor: \"Ada\"\nupdated: 2026-10-27T13:46:53.246Z\n---\n\n# Plans and pricing\n\n> **Tip**\n\n" +
				"See [credits](https://example.com/credits \"https://example.com/credits\").",
			wantTitle: "Plans and pricing",
			want:      []string{"<blockquote>\n<p><strong>Tip</strong></p>\n</blockquote>", `<a href="https://example.com/credits" title="https://example.com/credits">credits</a>`},
			absent:    []string{"author", "<h1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, base := newSite(t, map[string]served{"/docs/guide.md": {body: tt.markdown, contentType: tt.contentType}})
			article, err := reader.Article(context.Background(), base+"/docs/guide")
			if err != nil {
				t.Fatalf("Article() error = %v", err)
			}
			if article.Title != tt.wantTitle || article.Description != tt.wantDescription {
				t.Errorf("Article() title = %q, description = %q, want %q, %q", article.Title, article.Description, tt.wantTitle, tt.wantDescription)
			}
			if article.ID != "docs/guide" || article.URL != base+"/docs/guide" {
				t.Errorf("Article() ID = %q, URL = %q, want docs/guide at %s/docs/guide", article.ID, article.URL, base)
			}
			for _, part := range tt.want {
				if !strings.Contains(article.HTML, strings.ReplaceAll(part, "BASE", base)) {
					t.Errorf("Article() HTML lacks %q:\n%s", part, article.HTML)
				}
			}
			for _, part := range tt.absent {
				if strings.Contains(article.HTML, part) {
					t.Errorf("Article() HTML still has %q:\n%s", part, article.HTML)
				}
			}
		})
	}
}

func TestArticle_AsksForMarkdownWhereNoTwinIsServed(t *testing.T) {
	reader, base := newSite(t, map[string]served{"/whats-new": {body: "# What's new\n\nA lot.", contentType: "text/markdown", markdownOnly: true}})
	article, err := reader.Article(context.Background(), base+"/whats-new.md")
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if article.Title != "What's new" || article.ID != "whats-new" || !strings.Contains(article.HTML, "<p>A lot.</p>") {
		t.Errorf("Article() = %+v, want the page read as Markdown", article)
	}
}

func TestArticle_Errors(t *testing.T) {
	tests := []struct {
		name    string
		pages   map[string]served
		address string
		wantErr error
	}{
		{
			name:    "HTML served for the twin and for the page",
			pages:   map[string]served{"/articles/1-setup.md": {body: htmlPage}, "/articles/1-setup": {body: htmlPage}},
			address: "/articles/1-setup", wantErr: publichelp.ErrNotHelpCenter,
		},
		{
			name:    "HTML labelled as Markdown",
			pages:   map[string]served{"/a.md": {body: htmlPage, contentType: "text/markdown"}, "/a": {body: htmlPage}},
			address: "/a", wantErr: publichelp.ErrNotHelpCenter,
		},
		{name: "page that is gone", pages: map[string]served{}, address: "/gone", wantErr: publichelp.ErrNotFound},
		{name: "address without a host", pages: map[string]served{}, address: "", wantErr: publichelp.ErrInvalidURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, base := newSite(t, tt.pages)
			if tt.address == "" {
				base = "/only/a/path"
			}
			if _, err := reader.Article(context.Background(), base+tt.address); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Article() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestArticle_ReadsBackEveryListedID(t *testing.T) {
	pages := map[string]served{
		"/llms.txt": {body: "# Acme\n\n## Docs\n\n- [Welcome](BASE/.md)\n- [Mixed Case](BASE/docs/CH1E-Headings.md)\n" +
			"- [Deep](BASE/a/b%20c/d.md)\n- [Section home](BASE/docs.md)\n"},
		"/.md":                   {body: "# Welcome\n\nHome."},
		"/docs/CH1E-Headings.md": {body: "# Mixed Case\n\nHeadings."},
		"/a/b c/d.md":            {body: "# Deep\n\n[Up](../e.md)"},
		"/docs.md":               {body: "---\ntitle: Section home\n---\n\nStart."},
	}
	reader, base := newSite(t, pages)
	site, err := reader.Load(context.Background(), base, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	listed := refs(site.Collections)
	if len(listed) != 4 {
		t.Fatalf("Load() listed %d pages, want 4: %+v", len(listed), listed)
	}
	for _, ref := range listed {
		article, err := reader.Article(context.Background(), ref.URL)
		if err != nil {
			t.Errorf("Article(%s) error = %v", ref.URL, err)
			continue
		}
		if article.ID != ref.ID || article.Title != ref.Title || article.URL != ref.URL {
			t.Errorf("Article(%s) = %q %q at %s, listed as %q %q", ref.URL, article.ID, article.Title, article.URL, ref.ID, ref.Title)
		}
		if target, ok := site.LinkTarget(article.URL); !ok || target.ID != ref.ID {
			t.Errorf("LinkTarget(%s) = %+v, %v, want %s", article.URL, target, ok, ref.ID)
		}
	}
	deep, err := reader.Article(context.Background(), base+"/a/b%20c/d")
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if want := `<a href="` + base + `/a/e.md">Up</a>`; !strings.Contains(deep.HTML, want) {
		t.Errorf("Article() HTML = %s, want %s", deep.HTML, want)
	}
}
