package helpscoutdocs

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

const testHead = `<head><title>Acme Docs</title>
<link rel="stylesheet" href="//d3eto7onm69fcz.cloudfront.net/assets/stylesheets/launch-1.css"></head>`

const testHome = `<html>` + testHead + `<body><header id="mainNav"><a class="brand" href="/"><span>Acme Docs</span></a>
<ul class="nav"><li><a href="/">Home</a></li>
<li><a href="/collection/1-guides">Guides <b class="caret"></b></a></li>
<li><a href="/collection/7-api">API <b class="caret"></b></a></li>
<li><a href="/collection/9-status">Status <b class="caret"></b></a></li></ul></header>
<section id="docsSearch"></section>
<section class="collection-category"><h2><a href="/collection/1-guides">Guides</a></h2>
<a class="category" href="/category/20-basics"><h3>Basics</h3></a></section></body></html>`

const testGuides = `<html>` + testHead + `<body><section id="main-content"><h1>Most Popular Articles</h1>
<ul class="articleList"><li><a href="/article/31-reports"><span>Reports</span></a></li></ul></section>
<aside id="sidebar"><h3>Categories</h3><ul class="nav nav-list">
<li><a href="/category/20-basics">Basics <i class="icon-arrow"></i></a></li>
<li><a href="/category/21-advanced">Advanced <i class="icon-arrow"></i></a></li>
<li><a href="/category/22-empty">Empty <i class="icon-arrow"></i></a></li>
<li><a href="/category/23-solo">Solo <i class="icon-arrow"></i></a></li></ul></aside></body></html>`

const testAPI = `<html>` + testHead + `<body><aside id="sidebar"><ul class="nav nav-list">
<li><a href="/category/70-endpoints">Endpoints</a></li></ul></aside></body></html>`

const testBasics = `<html>` + testHead + `<body><section id="main-content">
<hgroup id="categoryHead"><h1>The Basics</h1><p class="descrip">Start here</p></hgroup>
<ul class="articleList">
<li><a href="/article/30-welcome"><i class="icon-article-doc"></i><span>Welcome</span></a></li>
<li><a href="/article/31-reports"><span>Reports</span></a></li></ul>
` + testPager + `
</section><aside id="sidebar"><a href="/category/21-advanced">Advanced</a></aside></body></html>`

const testArticle = `<html><head><title>Welcome - Acme Docs</title><meta name="description" content="Say hello."/>
<link rel="stylesheet" href="//d3eto7onm69fcz.cloudfront.net/assets/stylesheets/launch-1.css"></head><body>
<section id="main-content"><article id="fullArticle"><h1 class="title">Welcome</h1>
<a href="javascript:window.print()" class="printArticle" title="Print this article"><i class="icon-print"></i></a>
<p>Hello. See <a href="//docs.example.com/article/31-reports">reports</a> and <a href="/article/32-exports#csv">exports</a>.</p>
<p><img src="//cdn.example.com/a.png"><img data-src="/img/lazy.png" src="data:image/gif;base64,R0lGOD"></p>
</article><div class="articleRatings"><span>Did this answer your question?</span></div>
<section class="articleFoot"><time class="lu" datetime=2026-01-02>Updated on January 2, 2026</time></section>
<section class="related"><h3>Related Articles</h3><ul><li><a href="/article/31-reports">Reports</a></li></ul></section>
</section></body></html>`

const testPager = `<section class="pagination"><ul><li><a href="/category/20-basics?sort=custom">1</a></li>
<li><a href="/category/20-basics/2?sort=custom">2</a></li><li><a href="/category/20-basics/3?sort=custom">3</a></li></ul></section>`

// articleList is a category page that lists "number-slug|title" articles.
func articleList(pager string, items ...string) string {
	var out strings.Builder
	out.WriteString(`<html>` + testHead + `<body><section id="main-content"><ul class="articleList">`)
	for _, item := range items {
		slug, title, _ := strings.Cut(item, "|")
		fmt.Fprintf(&out, `<li><a href="/article/%s"><span>%s</span></a></li>`, slug, title)
	}
	out.WriteString(`</ul>` + pager + `</section></body></html>`)
	return out.String()
}

// testSite serves a small Docs site and returns its address.
func testSite(t *testing.T) (string, *Reader) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := server.URL
		pages := map[string]string{
			"/":                      testHome,
			"/collection/1-guides":   testGuides,
			"/collection/7-api":      testAPI,
			"/category/20-basics":    testBasics,
			"/category/20-basics/2":  articleList(testPager, "32-exports|Exports", "30-welcome|Welcome"),
			"/category/20-basics/3":  articleList(testPager, "33-imports|Imports"),
			"/category/21-advanced":  articleList("", "31-reports|Reports", "40-webhooks|Webhooks"),
			"/category/22-empty":     articleList(""),
			"/category/70-endpoints": articleList("", "40-webhooks|Webhooks", "71-list-users|List users"),
			"/article/30-welcome":    testArticle,
			"/article/60-solo-guide": strings.ReplaceAll(testArticle, "Welcome", "Solo guide"),
			"/article/90-no-body":    `<html>` + testHead + `<body><section id="main-content"></section></body></html>`,
			"/article/91-private":    `<html>` + testHead + `<body><form><input type="password"></form></body></html>`,
			"/sitemap.xml": `<urlset><url><loc>` + origin + `</loc></url><url><loc>` + origin + `/collection/1-guides</loc></url>` +
				`<url><loc>` + origin + `/article/30-welcome</loc></url><url><loc>` + origin + `/article/50-unlisted-guide</loc></url></urlset>`,
		}
		switch r.URL.Path {
		case "/article/30":
			http.Redirect(w, r, "/article/30-welcome", http.StatusMovedPermanently)
		case "/category/23-solo":
			http.Redirect(w, r, "/article/60-solo-guide", http.StatusFound)
		case "/collection/9-status", "/article/92-elsewhere":
			http.Redirect(w, r, "/", http.StatusFound)
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
	return server.URL, New(publichelp.NewFetcherWithHTTP(server.Client()))
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
	origin, reader := testSite(t)
	home, err := publichelp.NewFetcherWithHTTP(http.DefaultClient).Open(context.Background(), origin+"/")
	if err != nil {
		t.Fatalf("open home: %v", err)
	}
	site, err := reader.Probe(context.Background(), home)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if site.Platform != Key || site.BaseURL != origin+"/" || site.Name != "Acme Docs" {
		t.Errorf("Probe() = %+v", site)
	}

	rejected := []struct {
		name   string
		status int
		body   string
	}{
		{name: "plain website", status: http.StatusOK, body: `<html><body><h1>Acme</h1></body></html>`},
		{name: "other platform", status: http.StatusOK, body: `<html><body><div class="csh-wrapper"></div></body></html>`},
		{name: "stylesheet host on another kind of page", status: http.StatusOK, body: `<html>` + testHead + `<body><p>Hi</p></body></html>`},
		{name: "not served", status: http.StatusForbidden, body: testHome},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			page := &publichelp.Page{URL: home.URL, Status: tt.status, Header: http.Header{}, Body: []byte(tt.body)}
			if _, err := reader.Probe(context.Background(), page); !errors.Is(err, publichelp.ErrNotHelpCenter) {
				t.Errorf("Probe() error = %v, want ErrNotHelpCenter", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	origin, reader := testSite(t)
	requests := 0
	site, err := reader.Load(context.Background(), origin+"/", func() { requests++ })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// "Reports" and "Webhooks" are in two categories and stay in the first;
	// the category of one article forwards to it; the collection that leads
	// off the site lists no category and is left out.
	want := `collection-1 Guides []
  category-20 The Basics [30 Welcome, 31 Reports, 32 Exports, 33 Imports]
  category-21 Advanced [40 Webhooks]
  category-23 Solo [60 Solo guide]
collection-7 API []
  category-70 Endpoints [71 List users]
other Other articles [50 Unlisted guide]
`
	if got := outline(site.Collections, ""); got != want {
		t.Errorf("Load() collections =\n%s\nwant\n%s", got, want)
	}
	// Home, sitemap, three collections, five categories and two more pages.
	if requests != 12 {
		t.Errorf("Load() made %d requests, want 12", requests)
	}
	basics := site.Collections[0].Children[0]
	if basics.URL != origin+"/category/20-basics" || basics.Description != "Start here" || basics.Slug != "basics" {
		t.Errorf("category = %+v", basics)
	}
	for href, want := range map[string]publichelp.LinkTarget{
		origin + "/article/30-welcome":       {Kind: publichelp.LinkArticle, ID: "30", Certain: true},
		origin + "/article/30-old-name#more": {Kind: publichelp.LinkArticle, ID: "30", Fragment: "more", Certain: true},
		origin + "/article/30":               {Kind: publichelp.LinkArticle, ID: "30", Certain: true},
		origin + "/category/20-basics/2":     {Kind: publichelp.LinkCollection, ID: "category-20", Certain: true},
		origin + "/collection/1-guides":      {Kind: publichelp.LinkCollection, ID: "collection-1", Certain: true},
	} {
		if got, ok := site.LinkTarget(href); !ok || got != want {
			t.Errorf("LinkTarget(%q) = %+v, %v", href, got, ok)
		}
	}
	if _, ok := site.LinkTarget(origin + "/search?query=x"); ok {
		t.Error("LinkTarget() matched the search page")
	}
}

func TestLoad_Canceled(t *testing.T) {
	origin, reader := testSite(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Load(ctx, origin+"/", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Load() error = %v, want context.Canceled", err)
	}
}

func TestArticle(t *testing.T) {
	origin, reader := testSite(t)
	// The address without its slug forwards to the article.
	article, err := reader.Article(context.Background(), origin+"/article/30")
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if article.ID != "30" || article.Title != "Welcome" || article.Description != "Say hello." || article.URL != origin+"/article/30-welcome" {
		t.Errorf("Article() = %q %q %q %q", article.ID, article.Title, article.Description, article.URL)
	}
	for _, want := range []string{
		`href="http://docs.example.com/article/31-reports"`, `href="` + origin + `/article/32-exports#csv"`,
		`src="http://cdn.example.com/a.png"`, `src="` + origin + `/img/lazy.png"`,
	} {
		if !strings.Contains(article.HTML, want) {
			t.Errorf("Article() HTML lacks %s:\n%s", want, article.HTML)
		}
	}
	for _, chrome := range []string{"<h1", "print", "Did this answer", "Updated on", "Related Articles", "data:image"} {
		if strings.Contains(article.HTML, chrome) {
			t.Errorf("Article() HTML keeps %q:\n%s", chrome, article.HTML)
		}
	}
}

func TestArticle_Unreadable(t *testing.T) {
	origin, reader := testSite(t)
	tests := []struct {
		name, path string
		want       error
	}{
		{name: "missing page", path: "/article/99-gone", want: publichelp.ErrNotFound},
		{name: "article that forwards elsewhere", path: "/article/92-elsewhere", want: publichelp.ErrNotFound},
		{name: "page without an article", path: "/article/90-no-body", want: publichelp.ErrNotHelpCenter},
		{name: "sign-in form", path: "/article/91-private", want: publichelp.ErrRestricted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reader.Article(context.Background(), origin+tt.path); !errors.Is(err, tt.want) {
				t.Errorf("Article() error = %v, want %v", err, tt.want)
			}
		})
	}
}
