package hubspot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

const testHome = `<html lang="en"><head><title>Acme Help</title></head><body class="kb-body-wrapper">
<div class="hs-kb-home-list hs-kb-home-list--tiles"><div class="hs-kb-home-list__category">
<a class="hs-kb-home-list__category-link unstyled" href="/en/getting-started?hsLang=en">
<h3 class="hs-kb-home-list__text__category-title">Getting started</h3>
<p class="hs-kb-home-list__text__category-description">First steps</p></a></div></div></body></html>`

const testCrumbs = `<ol class="hs-kb-breadcrumbs"><li class="hs-kb-breadcrumb_item"><a href="/en?hsLang=en">Acme Help</a></li>
<li class="hs-kb-breadcrumb_item"><a href="/en/getting-started?hsLang=en" data-slug="/getting-started">Getting started</a></li></ol>`

const testMenu = `<aside class="hs-kb-sidebar"><ul class="hs-kb-category_accordion_nav_list">
<li><a class="hs-kb-category-accordion_link" href="#main-content">This one</a></li>
<li><a class="hs-kb-category-accordion_link" href="/en/getting-started#main-content">Getting started</a>
<ul><li><a class="hs-kb-category-accordion_link " href="/en/getting-started#basics">Basics</a></li></ul></li>
<li><a class="hs-kb-category-accordion_link" href="/en/hidden">Hidden</a></li></ul></aside>`

const testCategory = `<html lang="en"><body><main id="main-content">` + testCrumbs + `<section class="hs-kb-content">
<section class="hs-kb-category-listing" id="/getting-started"><h1>Getting started</h1><p>Everything to begin.</p></section>
<ul class="hs-kb-category-article-list"><li><a class="hs-kb-category-article-list__link" href="/en/welcome">Welcome</a></li></ul>
<section class="hs-kb-subcategory-listing" id="basics"><h2>Basics</h2><ul class="hs-kb-category-article-list">
<li><a class="hs-kb-category-article-list__link" href="/en/articles/12-setup?hsLang=en">Set up</a></li>
<li><a class="hs-kb-category-article-list__link" href="/en/welcome">Welcome</a></li></ul></section>
<section class="hs-kb-subcategory-listing" id="repeats"><h2>Repeats</h2><ul class="hs-kb-category-article-list">
<li><a class="hs-kb-category-article-list__link" href="/en/welcome">Welcome</a></li></ul></section>
</section>` + testMenu + `</main></body></html>`

const testHidden = `<html lang="en"><body><main id="main-content"><section class="hs-kb-content">
<section class="hs-kb-category-listing" id="/hidden"><h1>Hidden things</h1></section>
<ul class="hs-kb-category-article-list"><li><a class="hs-kb-category-article-list__link" href="/en/secret">Secret</a></li>
<li><a class="hs-kb-category-article-list__link" href="/en/articles/12-setup">Set up</a></li></ul></section>` + testMenu + `</main></body></html>`

const testArticle = `<html lang="en"><head><title>Welcome</title><meta name="description" content="Meta words."></head>
<body><main id="main-content">` + testCrumbs + `<article class="knowledgebase-post"><div class="row-fluid">
<div class="hs_cos_wrapper"><h1 data-hs-cos-type="inline_text_field">Welcome to Acme</h1></div>
<div class="hs_cos_wrapper"><h2 data-hs-cos-type="inline_text_field">Say hello.</h2></div>
<div class="hs_cos_wrapper"><div data-hs-cos-type="inline_richtext_field"><h2>First</h2>
<p>See <a href="/en/articles/12-setup?hsLang=en#keys">set up</a>.</p><img src="/hs-fs/hubfs/a.png?width=300"></div></div>
</div></article><div class="hs-kb-feedback"><div id="ka-feedback-form-container" article-id="77"></div></div>
</main></body></html>`

// testPage wraps an article's words the way a loose article page does.
func testPage(lang, body string) string {
	return `<html lang="` + lang + `"><body><article class="knowledgebase-post"><h1>` + body + `</h1><p>Text.</p></article></body></html>`
}

// testSite serves a small knowledge base under "/en" and returns its address.
func testSite(t *testing.T) (string, *Reader, *publichelp.Fetcher) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := server.URL
		entry := func(path string) string { return `<url><loc>` + origin + path + `</loc></url>` }
		pages := map[string]string{
			"/en":                   testHome,
			"/en/getting-started":   testCategory,
			"/en/hidden":            testHidden,
			"/en/welcome":           testArticle,
			"/en/loose":             testPage("en", "Loose article"),
			"/en/landing":           `<html lang="en"><body class="hs-kb-x"><h1>Not an article</h1></body></html>`,
			"/de/willkommen":        testPage("de", "Willkommen"),
			"/_hcms/mem/login":      `<html><body><form><input type="password"></form></body></html>`,
			"/plain":                `<html><body><h1>A website page</h1></body></html>`,
			"/en/kb-search-results": `<html lang="en"><body>Search</body></html>`,
			"/sitemap.xml": `<urlset>` + entry("/en") + entry("/en/getting-started") + entry("/en/welcome") + entry("/en/loose") +
				entry("/en/landing") + entry("/en/gone") + entry("/en/kb-search-results") + entry("/de/willkommen") + `</urlset>`,
		}
		if r.URL.Path != "/elsewhere" {
			w.Header().Set("X-Hs-Hub-Id", "42")
		}
		switch r.URL.Path {
		case "/":
			http.Redirect(w, r, "/en", http.StatusFound)
		case "/private", "/en/members":
			http.Redirect(w, r, "/_hcms/mem/login?redirect_url=x", http.StatusFound)
		case "/elsewhere":
			fmt.Fprint(w, testHome)
		default:
			body, ok := pages[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			fmt.Fprint(w, body)
		}
	}))
	t.Cleanup(server.Close)
	fetch := publichelp.NewFetcherWithHTTP(server.Client())
	return server.URL, New(fetch), fetch
}

// outline writes a collection tree as "id name [article, article]" lines.
func outline(collections []publichelp.Collection, indent string) string {
	var out strings.Builder
	for _, collection := range collections {
		ids := make([]string, 0, len(collection.Articles))
		for _, article := range collection.Articles {
			ids = append(ids, article.ID+" "+article.Title)
		}
		fmt.Fprintf(&out, "%s%s %s [%s]\n", indent, collection.ID, collection.Name, strings.Join(ids, ", "))
		out.WriteString(outline(collection.Children, indent+"  "))
	}
	return out.String()
}

func TestProbe(t *testing.T) {
	origin, reader, fetch := testSite(t)
	tests := []struct {
		name, path, wantBase, wantName string
		wantErr                        error
	}{
		{name: "home after its redirect", path: "/", wantBase: origin + "/en", wantName: "Acme Help"},
		{name: "an article names the home in its breadcrumbs", path: "/en/welcome", wantBase: origin + "/en"},
		{name: "a knowledge base behind sign-in", path: "/private", wantErr: publichelp.ErrRestricted},
		{name: "another HubSpot page", path: "/plain", wantErr: publichelp.ErrNotHelpCenter},
		{name: "the same page on another platform", path: "/elsewhere", wantErr: publichelp.ErrNotHelpCenter},
		{name: "a missing page", path: "/en/gone", wantErr: publichelp.ErrNotHelpCenter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, err := fetch.Open(context.Background(), origin+tt.path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			site, err := reader.Probe(context.Background(), home)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Probe() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Probe() error = %v", err)
			}
			if site.Platform != Key || site.BaseURL != tt.wantBase || site.Name != tt.wantName || site.Locale != "en" {
				t.Errorf("Probe() = %s %q %q %q, want hubspot %q %q en", site.Platform, site.BaseURL, site.Name, site.Locale, tt.wantBase, tt.wantName)
			}
		})
	}
	// Sign-in is reported as HubSpot's, so the search for a platform ends.
	private, err := fetch.Open(context.Background(), origin+"/private")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	var known *publichelp.PlatformError
	if _, err := reader.Probe(context.Background(), private); !errors.As(err, &known) || known.Platform != Key {
		t.Errorf("Probe() error = %v, want a hubspot platform error", err)
	}
}

func TestLoad(t *testing.T) {
	origin, reader, _ := testSite(t)
	requests := 0
	site, err := reader.Load(context.Background(), origin+"/en", func() { requests++ })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// The hidden category comes from the menu, an article shows where it is
	// first listed, and the sitemap adds only the loose English article.
	want := `category-en/getting-started Getting started [en/welcome Welcome]
  category-en/getting-started#basics Basics [en/articles/12-setup Set up]
category-en/hidden Hidden things [en/secret Secret]
other Other articles [en/loose Loose article]
`
	if got := outline(site.Collections, ""); got != want {
		t.Errorf("collections =\n%s\nwant\n%s", got, want)
	}
	if site.Name != "Acme Help" || site.BaseURL != origin+"/en" || site.Locale != "en" {
		t.Errorf("site = %q %q %q", site.Name, site.BaseURL, site.Locale)
	}
	if first := site.Collections[0]; first.Description != "Everything to begin." || first.URL != origin+"/en/getting-started" {
		t.Errorf("category = %q %q", first.Description, first.URL)
	}
	// Home, two categories, the sitemap, and the two pages only it names
	// that still answer.
	if requests != 6 {
		t.Errorf("progress calls = %d, want 6", requests)
	}
	links := map[string]publichelp.LinkTarget{
		origin + "/en/articles/12-setup?hsLang=en#keys": {Kind: publichelp.LinkArticle, ID: "en/articles/12-setup", Fragment: "keys", Certain: true},
		origin + "/en/getting-started?hsLang=en#basics": {Kind: publichelp.LinkCollection, ID: "category-en/getting-started", Fragment: "basics", Certain: true},
		origin + "/en/loose":                            {Kind: publichelp.LinkArticle, ID: "en/loose", Certain: true},
	}
	for href, want := range links {
		if got, ok := site.LinkTarget(href); !ok || got != want {
			t.Errorf("LinkTarget(%s) = %+v, %v, want %+v", href, got, ok, want)
		}
	}
	if got, ok := site.LinkTarget(origin + "/en/landing"); ok {
		t.Errorf("LinkTarget(landing) = %+v, want none", got)
	}
}

func TestLoad_Restricted(t *testing.T) {
	origin, reader, _ := testSite(t)
	if _, err := reader.Load(context.Background(), origin+"/private", nil); !errors.Is(err, publichelp.ErrRestricted) {
		t.Errorf("Load() error = %v, want ErrRestricted", err)
	}
}

func TestLoad_Canceled(t *testing.T) {
	origin, reader, _ := testSite(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Load(ctx, origin+"/en", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Load() error = %v, want context.Canceled", err)
	}
}

func TestArticle(t *testing.T) {
	origin, reader, _ := testSite(t)
	article, err := reader.Article(context.Background(), origin+"/en/welcome?hsLang=en")
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if article.ID != "en/welcome" || article.Title != "Welcome to Acme" || article.Description != "Say hello." || article.URL != origin+"/en/welcome" {
		t.Errorf("article = %q %q %q %q", article.ID, article.Title, article.Description, article.URL)
	}
	for _, want := range []string{
		`<h2>First</h2>`, `href="` + origin + `/en/articles/12-setup?hsLang=en#keys"`, `src="` + origin + `/hs-fs/hubfs/a.png?width=300"`,
	} {
		if !strings.Contains(article.HTML, want) {
			t.Errorf("HTML misses %s:\n%s", want, article.HTML)
		}
	}
	for _, unwanted := range []string{"Welcome to Acme", "Say hello.", "ka-feedback"} {
		if strings.Contains(article.HTML, unwanted) {
			t.Errorf("HTML keeps %q:\n%s", unwanted, article.HTML)
		}
	}
	// A theme without rich text fields keeps the whole article, and the
	// description falls back to what the page declares.
	loose, err := reader.Article(context.Background(), origin+"/en/loose")
	if err != nil {
		t.Fatalf("Article(loose) error = %v", err)
	}
	if loose.Title != "Loose article" || !strings.Contains(loose.HTML, "<p>Text.</p>") || strings.Contains(loose.HTML, "<h1>") {
		t.Errorf("loose article = %q %s", loose.Title, loose.HTML)
	}
}

func TestArticle_Unreadable(t *testing.T) {
	origin, reader, _ := testSite(t)
	tests := []struct {
		name, path string
		want       error
	}{
		{name: "a page that is not an article", path: "/en/landing", want: publichelp.ErrNotFound},
		{name: "a removed article", path: "/en/gone", want: publichelp.ErrNotFound},
		{name: "an article behind sign-in", path: "/en/members", want: publichelp.ErrRestricted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reader.Article(context.Background(), origin+tt.path); !errors.Is(err, tt.want) {
				t.Errorf("Article() error = %v, want %v", err, tt.want)
			}
		})
	}
}
