package llmstxt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// served is one answer of the test site. "BASE" in a body stands for the
// site's address.
type served struct {
	body        string
	contentType string
	// status is the response status, 200 when left out.
	status int
	header map[string]string
	// markdownOnly serves body to a request that accepts Markdown, and an HTML
	// page to any other.
	markdownOnly bool
}

const htmlPage = "<!DOCTYPE html><html><head><title>Acme Help</title></head><body>page</body></html>"

// newSite starts a site that serves pages by path and answers 404 elsewhere.
func newSite(t *testing.T, pages map[string]served) (*Reader, string) {
	t.Helper()
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		for name, value := range page.header {
			w.Header().Set(name, value)
		}
		body, contentType := page.body, page.contentType
		if page.markdownOnly && !strings.Contains(r.Header.Get("Accept"), "text/markdown") {
			body, contentType = htmlPage, "text/html"
		}
		if contentType == "" {
			contentType = "text/plain; charset=utf-8"
			if strings.HasPrefix(body, "<!DOCTYPE") {
				contentType = "text/html; charset=utf-8"
			}
		}
		w.Header().Set("Content-Type", contentType)
		if page.status != 0 {
			w.WriteHeader(page.status)
		}
		if _, err := w.Write([]byte(strings.ReplaceAll(body, "BASE", base))); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	base = server.URL
	return New(publichelp.NewFetcherWithHTTP(server.Client())), base
}

func probe(t *testing.T, pages map[string]served, homePath string) (*publichelp.Site, string, error) {
	t.Helper()
	reader, base := newSite(t, pages)
	home, err := publichelp.NewFetcherWithHTTP(http.DefaultClient).Open(context.Background(), base+homePath)
	if err != nil {
		t.Fatalf("open home: %v", err)
	}
	site, err := reader.Probe(context.Background(), home)
	return site, base, err
}

const twoPages = "# Acme Docs\n\n- [One](BASE/docs/one.md): First\n- [Two](BASE/docs/two.md)\n"

func TestProbe(t *testing.T) {
	tests := []struct {
		name         string
		pages        map[string]served
		home         string
		wantBase     string
		wantPlatform string
		wantName     string
	}{
		{
			name:     "index next to the home page",
			pages:    map[string]served{"/docs": {body: htmlPage}, "/docs/llms.txt": {body: twoPages}},
			home:     "/docs",
			wantBase: "/docs", wantPlatform: "docs", wantName: "Acme Docs",
		},
		{
			name: "index named by the X-Llms-Txt header",
			pages: map[string]served{
				"/kb/start":    {body: htmlPage, header: map[string]string{"X-Llms-Txt": "/kb/llms.txt", "X-Mintlify-Client-Version": "1"}},
				"/kb/llms.txt": {body: twoPages},
			},
			home:     "/kb/start",
			wantBase: "/kb", wantPlatform: "mintlify", wantName: "Acme Docs",
		},
		{
			name: "index named by the Link header",
			pages: map[string]served{
				"/guide/intro":    {body: htmlPage, header: map[string]string{"Link": `</a.css>; rel=preload, </guide/llms.txt>; rel="llms-txt"`}},
				"/guide/llms.txt": {body: twoPages},
			},
			home:     "/guide/intro",
			wantBase: "/guide", wantPlatform: "docs", wantName: "Acme Docs",
		},
		{
			name: "index at the root of the host",
			pages: map[string]served{
				"/help":     {body: strings.Replace(htmlPage, "page", `<img src="https://files.plain.com/a.png">`, 1)},
				"/llms.txt": {body: "- [One](/article/one.md)\n- [Two](/article/two.md)\n", contentType: "text/markdown"},
			},
			home:     "/help",
			wantBase: "", wantPlatform: "plain", wantName: "Acme Help",
		},
		{
			name: "section index that names the index of the whole site",
			pages: map[string]served{
				"/learn/home":          {body: htmlPage, header: map[string]string{"X-Fern-Renderer": "static"}},
				"/learn/home/llms.txt": {body: "> For the complete documentation index, see BASE/learn/llms.txt.\n\n# Home\n\n- [Home](BASE/learn/home.md)\n"},
				"/learn/llms.txt":      {body: "# Fern Docs\n\n- [SDKs](BASE/learn/sdks/llms.txt)\n- [Docs](BASE/learn/docs/llms.txt)\n"},
			},
			home:     "/learn/home",
			wantBase: "/learn", wantPlatform: "fern", wantName: "Fern Docs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site, base, err := probe(t, tt.pages, tt.home)
			if err != nil {
				t.Fatalf("Probe() error = %v", err)
			}
			if site.BaseURL != base+tt.wantBase || site.Platform != tt.wantPlatform || site.Name != tt.wantName {
				t.Errorf("Probe() = %s %q at %s, want %s %q at %s",
					site.Platform, site.Name, site.BaseURL, tt.wantPlatform, tt.wantName, base+tt.wantBase)
			}
		})
	}
}

func TestProbe_Rejects(t *testing.T) {
	tests := []struct {
		name  string
		pages map[string]served
	}{
		{name: "no index", pages: map[string]served{"/": {body: htmlPage}}},
		{
			name:  "home page behind a sign-in",
			pages: map[string]served{"/": {body: htmlPage, status: http.StatusUnauthorized}, "/llms.txt": {body: twoPages}},
		},
		{name: "HTML page served as the index", pages: map[string]served{"/": {body: htmlPage}, "/llms.txt": {body: htmlPage}}},
		{
			name:  "HTML labelled as text",
			pages: map[string]served{"/": {body: htmlPage}, "/llms.txt": {body: htmlPage, contentType: "text/plain"}},
		},
		{
			name:  "one page only",
			pages: map[string]served{"/": {body: htmlPage}, "/llms.txt": {body: "# Acme\n\n- [One](BASE/one.md)\n"}},
		},
		{
			name: "links to other sites and to HTML pages only",
			pages: map[string]served{"/": {body: htmlPage}, "/llms.txt": {
				body: "# Acme\n\n- [One](https://other.example/one.md)\n- [Two](https://other.example/two.md)\n- [Pricing](BASE/pricing)\n- [Blog](BASE/blog)\n",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := probe(t, tt.pages, "/"); !errors.Is(err, publichelp.ErrNotHelpCenter) {
				t.Fatalf("Probe() error = %v, want ErrNotHelpCenter", err)
			}
		})
	}
}

// outline lists a site's collections and pages, one per line, indented by
// depth: "Name [id] <url> (description)" and "- Title [id] summary".
func outline(collections []publichelp.Collection, base, indent string) string {
	var out strings.Builder
	for _, collection := range collections {
		out.WriteString(indent + collection.Name + " [" + collection.ID + "]")
		if collection.URL != "" {
			out.WriteString(" <" + strings.TrimPrefix(collection.URL, base) + ">")
		}
		if collection.Description != "" {
			out.WriteString(" (" + collection.Description + ")")
		}
		out.WriteString("\n")
		for _, ref := range collection.Articles {
			line := indent + "- " + ref.Title + " [" + ref.ID + "]"
			if ref.URL != base+"/"+strings.TrimPrefix(ref.ID, "/") {
				line += " <" + ref.URL + ">"
			}
			out.WriteString(strings.TrimRight(line+" "+ref.Summary, " ") + "\n")
		}
		out.WriteString(outline(collection.Children, base, indent+"  "))
	}
	return out.String()
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name  string
		pages map[string]served
		home  string
		want  string
	}{
		{
			name: "sections, descriptions and links that are not pages",
			pages: map[string]served{"/llms.txt": {body: `# Acme

> Everything about Acme.

## Guides

- [Start](BASE/guides/start.md): Begin here (author: Ada, updated: 2026-01-02)
  - [Nested](BASE/guides/nested.md)
* [Titled link](BASE/guides/titled.md "BASE/guides/titled")
- [Start again](BASE/guides/start.md): Listed twice
- [Elsewhere](https://other.example/page.md)
- [Contact form](https://forms.example/contact)
- [Full text](BASE/llms-full.txt)

### [Billing](BASE/guides/billing)

> Invoices and
> plans.

- [Invoices](/guides/billing/invoices.md)

## OpenAPI Specs

- [openapi](BASE/openapi.json)
- [asyncapi](/asyncapi.yaml)

# Second product

## Setup

- [Install](BASE/second/install.md)
`}},
			want: `Guides [guides]
- Start [guides/start] Begin here
- Nested [guides/nested]
- Titled link [guides/titled]
  Billing [guides/billing] </guides/billing> (Invoices and plans.)
  - Invoices [guides/billing/invoices]
Second product [second-product]
- Install [second/install]
`,
		},
		{
			name: "Ferndesk entries and bare addresses",
			pages: map[string]served{"/help/llms.txt": {body: `# Help Center - Acme
## Get started
- [What Acme does]: BASE/help/what-acme-does-ab1cd.md
- BASE/help/reset-your-password.md
### Domains
- [Connect a domain]: BASE/help/connect-a-domain-zz9yy.md
`}},
			home: "/help",
			want: `Get started [get-started]
- What Acme does [help/what-acme-does-ab1cd]
- Reset your password [help/reset-your-password]
  Domains [get-started/domains]
  - Connect a domain [help/connect-a-domain-zz9yy]
`,
		},
		{
			name: "flat index grouped by address",
			pages: map[string]served{"/docs/llms.txt": {body: `# Acme

- [Introduction](BASE/docs/introduction.md): Welcome
- [Send email](BASE/docs/api-reference/emails/send.md)
- [Domains](BASE/docs/dashboard/domains.md)
- [Quickstart](BASE/docs/quickstart.md)
- [List emails](BASE/docs/api-reference/emails/list.md)
- [Home](BASE/docs.md)
`}},
			home: "/docs",
			want: `Acme [acme]
- Introduction [docs/introduction] Welcome
- Quickstart [docs/quickstart]
- Home [docs]
Api reference [api-reference]
- Send email [docs/api-reference/emails/send]
- List emails [docs/api-reference/emails/list]
Dashboard [dashboard]
- Domains [docs/dashboard/domains]
`,
		},
		{
			name: "index of indexes with a list of every language",
			pages: map[string]served{
				"/docs/llms.txt": {body: `# Acme

- [Documentation (2 pages)](BASE/docs/_llms/en/documentation.txt): Get started.

## Help center

- [Sign in](BASE/docs/help/sign-in.md)

## Indexes

- [English / Documentation (2 pages)](BASE/docs/_llms/en/documentation.txt)
- [French (3 pages)](BASE/docs/_llms/fr.txt)
`},
				"/docs/_llms/en/documentation.txt": {body: `# Acme: English Documentation

> ## Agent Instructions
> Run the acme command.

## Documentation

### Get started

- [Quickstart](BASE/docs/quickstart.md)

#### CLI

- [Install](BASE/docs/cli/install.md)
`},
				"/docs/_llms/fr.txt": {body: "# Acme\n\n- [Démarrage](BASE/docs/fr/quickstart.md)\n- [Installer](BASE/docs/fr/install.md)\n"},
			},
			home: "/docs",
			want: `Documentation [documentation] (Get started.)
- Quickstart [docs/quickstart]
  CLI [documentation/cli]
  - Install [docs/cli/install]
Help center [help-center]
- Sign in [docs/help/sign-in]
`,
		},
		{
			name: "section indexes with versions and a page served as an index",
			pages: map[string]served{
				"/learn/llms.txt": {body: `# Acme Docs

Instructions for AI Agents:

- For clean Markdown of any page, append .md to the page URL

## Products

- [SDKs](BASE/learn/sdks/llms.txt): Client libraries
  - [Introduction](BASE/learn/sdks/current/introduction.md)
- [OpenAPI](/learn/openapi/overview/llms.txt): Define APIs
  - [Overview](BASE/learn/openapi/overview.md)
- [Gone](BASE/learn/gone/llms.txt)
  - [Kept](BASE/learn/gone/kept.md)
`},
				"/learn/sdks/llms.txt": {body: `> For the complete documentation index, see BASE/learn/llms.txt.

# SDKs

## Versions

- [Current](BASE/learn/sdks/current/llms.txt): default version
- [Legacy](BASE/learn/sdks/legacy/llms.txt)
`},
				"/learn/sdks/current/llms.txt": {body: `# Current

## Docs

- [Introduction](BASE/learn/sdks/current/introduction.md): Start
- [Quickstart](BASE/learn/sdks/current/quickstart.md)
`},
				"/learn/sdks/legacy/llms.txt": {body: "# Legacy\n\n- [Old](BASE/learn/sdks/legacy/old.md)\n- [Older](BASE/learn/sdks/legacy/older.md)\n"},
				"/learn/openapi/overview/llms.txt": {contentType: "text/markdown", body: `# What is OpenAPI?

OpenAPI is a standard for REST APIs.
It is written in JSON or YAML.
Fern reads both.

- [The specification](BASE/learn/openapi/spec.md)
`},
			},
			home: "/learn",
			want: `Products [products]
  SDKs [products/sdks] (Client libraries)
  - Introduction [learn/sdks/current/introduction] Start
  - Quickstart [learn/sdks/current/quickstart]
  OpenAPI [products/openapi] (Define APIs)
  - Overview [learn/openapi/overview]
  Gone [products/gone]
  - Kept [learn/gone/kept]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, base := newSite(t, tt.pages)
			requests := 0
			site, err := reader.Load(context.Background(), base+tt.home, func() { requests++ })
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := outline(site.Collections, base, ""); got != tt.want {
				t.Errorf("Load() collections =\n%s\nwant\n%s", got, tt.want)
			}
			if site.BaseURL != base+tt.home || requests == 0 {
				t.Errorf("Load() BaseURL = %s after %d reported requests, want %s and some", site.BaseURL, requests, base+tt.home)
			}
			for _, ref := range refs(site.Collections) {
				if target, ok := site.LinkTarget(ref.URL + ".md#part"); !ok || target.ID != ref.ID || target.Kind != publichelp.LinkArticle {
					t.Errorf("LinkTarget(%s.md) = %+v, %v, want article %s", ref.URL, target, ok, ref.ID)
				}
			}
		})
	}
}

func refs(collections []publichelp.Collection) []publichelp.ArticleRef {
	var out []publichelp.ArticleRef
	for _, collection := range collections {
		out = append(append(out, collection.Articles...), refs(collection.Children)...)
	}
	return out
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		pages   map[string]served
		wantErr error
	}{
		{name: "no index", pages: map[string]served{}, wantErr: publichelp.ErrNotFound},
		{name: "HTML page served as the index", pages: map[string]served{"/llms.txt": {body: htmlPage}}, wantErr: publichelp.ErrNotHelpCenter},
		{
			name:    "index without pages",
			pages:   map[string]served{"/llms.txt": {body: "# Acme\n\n## Versions\n\n- [v2](BASE/v2/llms.txt): default version\n"}, "/v2/llms.txt": {body: "# Acme\n"}},
			wantErr: publichelp.ErrNotHelpCenter,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, base := newSite(t, tt.pages)
			if _, err := reader.Load(context.Background(), base, nil); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Load() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
	t.Run("invalid address", func(t *testing.T) {
		if _, err := New(publichelp.NewFetcher()).Load(context.Background(), "not a url", nil); !errors.Is(err, publichelp.ErrInvalidURL) {
			t.Fatalf("Load() error = %v, want ErrInvalidURL", err)
		}
	})
}

func TestLoad_NamesThePlatform(t *testing.T) {
	reader, base := newSite(t, map[string]served{
		"/docs":          {body: htmlPage, header: map[string]string{"X-Gitbook-Route-Site": "acme"}},
		"/docs/llms.txt": {body: twoPages},
	})
	site, err := reader.Load(context.Background(), base+"/docs/", nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if site.Platform != "gitbook" || site.Name != "Acme Docs" || site.BaseURL != base+"/docs" {
		t.Errorf("Load() = %s %q at %s, want gitbook \"Acme Docs\" at %s/docs", site.Platform, site.Name, site.BaseURL, base)
	}
}

func TestLoad_StopsFollowingIndexesThatLinkInCircles(t *testing.T) {
	reader, base := newSite(t, map[string]served{
		"/llms.txt":   {body: "# Acme\n\n- [A](BASE/a/llms.txt)\n- [Page](BASE/page.md)\n"},
		"/a/llms.txt": {body: "# A\n\n- [Root](BASE/llms.txt)\n- [Self](BASE/a/llms.txt)\n- [Deep](BASE/a/deep.md)\n"},
	})
	requests := 0
	site, err := reader.Load(context.Background(), base, func() { requests++ })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	ids := []string{}
	for _, ref := range refs(site.Collections) {
		ids = append(ids, ref.ID)
	}
	slices.Sort(ids)
	// Two index files and the home page, which does not exist but was asked.
	if !slices.Equal(ids, []string{"a/deep", "page"}) || requests != 3 {
		t.Errorf("Load() listed %v with %d requests, want [a/deep page] with 3", ids, requests)
	}
}
