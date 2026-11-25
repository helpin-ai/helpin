package crisp

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

const testHome = `<html lang="en"><head><meta property="og:site_name" content="Acme Help"><title>Acme Help</title></head>
<body><header><a class="csh-header-main-logo" href="/en/"><img alt="Acme Help"></a>
<form onsubmit="CrispHelpdeskCommon.search()"></form></header>
<div class="csh-home">
<section data-type="frequently_read"><a class="csh-box" href="/en/article/first-steps-a1b2c/">First steps</a></section>
<section data-type="categories">
<a class="csh-box" href="/en/category/setup-abc12/"><span class="csh-category-badge">Setup</span>
  <span class="csh-home-list-label">Install and configure.</span></a>
<a class="csh-box" href="/en/category/billing-def34/"><span class="csh-category-badge">Billing</span></a>
<a class="csh-box" href="/en/category/empty-zzz99/"><span class="csh-category-badge">Empty</span></a>
<a class="csh-box" href="/fr/category/facturation-def34/"><span class="csh-category-badge">Facturation</span></a>
</section></div></body></html>`

const testSetup = `<html lang="en"><body><nav class="csh-navigation">
<div class="csh-navigation-title-list-description">Install and configure.</div></nav>
<div class="csh-category"><aside><a href="/en/category/billing-def34/">Billing</a></aside><section>
<div class="csh-category-section"><h6 class="csh-category-section-title">Install</h6><ul>
 <li class="csh-category-item"><a href="/en/article/first-steps-a1b2c/">
   <h6 class="csh-category-item-title"><span></span>First steps</h6><span class="csh-category-item-meta">Summary text</span></a></li>
 <li class="csh-category-item" data-collapsed="true"><a href="/en/article/add-the-widget-d3e4f/">
   <h6 class="csh-category-item-title">Add the widget</h6></a></li></ul></div>
<div class="csh-category-section"><h6 class="csh-category-section-title">Install</h6><ul>
 <li class="csh-category-item"><a href="/en/article/mobile-apps-g5h6i/"><h6 class="csh-category-item-title">Mobile apps</h6></a></li></ul></div>
<div class="csh-category-section"><ul>
 <li class="csh-category-item"><a href="/en/article/loose-end-j7k8l/"><h6 class="csh-category-item-title">Loose end</h6></a></li></ul></div>
</section></div></body></html>`

const testBilling = `<html lang="en"><body><div class="csh-category"><section>
<a href="/en/article/first-steps-a1b2c/"><h6 class="csh-category-item-title">First steps</h6></a>
<a href="/en/article/invoices-m9n0p/"><h6 class="csh-category-item-title">Invoices</h6></a>
</section></div></body></html>`

const testArticle = `<html lang="en"><head><title>First steps | Acme Help</title><meta name="description" content="Start here."></head>
<body><div class="csh-article"><aside><p>Related articles</p><a href="/en/article/invoices-m9n0p/">Invoices</a></aside>
<article><div class="csh-article-content-header"><a href="/en/category/setup-abc12/">Setup</a><h1>First steps</h1></div>
<div class="csh-article-content-text">
<p><b><strong class="csh-markdown">Welcome</strong></b> to <i><em>Acme</em></i>.</p>
<p>Read <a href="/en/article/invoices-m9n0p/">about invoices</a> next.</p>
<p><img data-src="/static/shot.png" alt="Screenshot"><img src="https://cdn.example.com/kept.png"></p>
<span class="csh-article-content-separate"></span><p class="csh-article-content-updated">Updated on: 01/02/2026</p>
</div></article>
<section class="csh-article-rate"><p>Was this article helpful?</p></section></div></body></html>`

// testSite serves a small Crisp help center and returns its address.
func testSite(t *testing.T) (string, *Reader) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Crisp-Ray", "test")
		origin := server.URL
		pages := map[string]string{
			"/en/":                            testHome,
			"/en/category/setup-abc12/":       testSetup,
			"/en/category/billing-def34/":     testBilling,
			"/en/category/empty-zzz99/":       `<html lang="en"><body><div class="csh-category"></div></body></html>`,
			"/en/article/first-steps-a1b2c/":  testArticle,
			"/en/article/renamed-a1b2c/":      testArticle,
			"/en/article/no-body-q1q1q/":      `<html lang="en"><body><div class="csh-article"></div></body></html>`,
			"/en/article/members-only-r2r2r/": `<html lang="en"><body><form><input type="password"></form></body></html>`,
			"/sitemap.xml":                    `<sitemapindex><sitemap><loc>` + origin + `/sitemap-articles-1.xml</loc></sitemap></sitemapindex>`,
			"/sitemap-articles-1.xml": `<urlset><url><loc>` + origin + `/en/article/invoices-m9n0p/</loc></url>` +
				`<url><loc>` + origin + `/en/article/hidden-setting-s3s3s/</loc></url>` +
				`<url><loc>` + origin + `/fr/article/factures-m9n0p/</loc></url></urlset>`,
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/en/", http.StatusMovedPermanently)
			return
		}
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
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
	fetch := publichelp.NewFetcherWithHTTP(http.DefaultClient)
	home, err := fetch.Open(context.Background(), origin+"/")
	if err != nil {
		t.Fatalf("open home: %v", err)
	}
	site, err := reader.Probe(context.Background(), home)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if site.Platform != Key || site.BaseURL != origin+"/en/" || site.Name != "Acme Help" || site.Locale != "en" {
		t.Errorf("Probe() = %+v", site)
	}

	rejected := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{name: "plain website", status: http.StatusOK, body: `<html lang="en"><body><h1>Acme</h1></body></html>`},
		{name: "other platform", status: http.StatusOK, body: `<html><body><section id="docsSearch"></section><article id="fullArticle"></article></body></html>`},
		{name: "crisp header without help center markup", status: http.StatusOK, header: http.Header{"X-Crisp-Ray": {"x"}}, body: `<html lang="en"><body>Chat</body></html>`},
		{name: "not served", status: http.StatusForbidden, header: http.Header{"X-Crisp-Ray": {"x"}}, body: testHome},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			page := &publichelp.Page{URL: home.URL, Status: tt.status, Header: tt.header, Body: []byte(tt.body)}
			if page.Header == nil {
				page.Header = http.Header{}
			}
			if _, err := reader.Probe(context.Background(), page); !errors.Is(err, publichelp.ErrNotHelpCenter) {
				t.Errorf("Probe() error = %v, want ErrNotHelpCenter", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	origin, reader := testSite(t)
	requests := 0
	site, err := reader.Load(context.Background(), origin+"/en/", func() { requests++ })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := `category-abc12 Setup [j7k8l Loose end]
  category-abc12-install Install [a1b2c First steps, d3e4f Add the widget]
  category-abc12-install-2 Install [g5h6i Mobile apps]
category-def34 Billing [m9n0p Invoices]
other Other articles [s3s3s Hidden setting]
`
	if got := outline(site.Collections, ""); got != want {
		t.Errorf("Load() collections =\n%s\nwant\n%s", got, want)
	}
	// Home, three categories, the sitemap index and its one file.
	if requests != 6 {
		t.Errorf("Load() made %d requests, want 6", requests)
	}
	if site.Name != "Acme Help" || site.Locale != "en" || site.BaseURL != origin+"/en/" {
		t.Errorf("Load() site = %q %q %q", site.Name, site.Locale, site.BaseURL)
	}
	setup := site.Collections[0]
	if setup.URL != origin+"/en/category/setup-abc12/" || setup.Description != "Install and configure." {
		t.Errorf("category = %q %q", setup.URL, setup.Description)
	}
	// A link is recognized by its code even when the slug changed.
	for href, want := range map[string]publichelp.LinkTarget{
		origin + "/en/article/first-steps-a1b2c/":    {Kind: publichelp.LinkArticle, ID: "a1b2c", Certain: true},
		origin + "/en/article/old-title-a1b2c/#top":  {Kind: publichelp.LinkArticle, ID: "a1b2c", Fragment: "top", Certain: true},
		origin + "/en/category/renamed-setup-abc12/": {Kind: publichelp.LinkCollection, ID: "category-abc12", Certain: true},
	} {
		if got, ok := site.LinkTarget(href); !ok || got != want {
			t.Errorf("LinkTarget(%q) = %+v, %v", href, got, ok)
		}
	}
	if _, ok := site.LinkTarget(origin + "/en/"); ok {
		t.Error("LinkTarget() matched the home page")
	}
}

func TestLoad_Canceled(t *testing.T) {
	origin, reader := testSite(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Load(ctx, origin+"/en/", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Load() error = %v, want context.Canceled", err)
	}
}

func TestArticle(t *testing.T) {
	origin, reader := testSite(t)
	article, err := reader.Article(context.Background(), origin+"/en/article/renamed-a1b2c/")
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if article.ID != "a1b2c" || article.Title != "First steps" || article.Description != "Start here." {
		t.Errorf("Article() = %q %q %q", article.ID, article.Title, article.Description)
	}
	for _, want := range []string{
		`<b>Welcome</b>`, `<i>Acme</i>`,
		`href="` + origin + `/en/article/invoices-m9n0p/"`,
		`src="` + origin + `/static/shot.png"`, `src="https://cdn.example.com/kept.png"`,
	} {
		if !strings.Contains(article.HTML, want) {
			t.Errorf("Article() HTML lacks %s:\n%s", want, article.HTML)
		}
	}
	for _, chrome := range []string{"Updated on", "Related articles", "helpful", "<h1", "Setup", "csh-article-content-separate"} {
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
		{name: "missing page", path: "/en/article/gone-t4t4t/", want: publichelp.ErrNotFound},
		{name: "page without an article", path: "/en/article/no-body-q1q1q/", want: publichelp.ErrNotHelpCenter},
		{name: "sign-in form", path: "/en/article/members-only-r2r2r/", want: publichelp.ErrRestricted},
		{name: "not an article address", path: "/en/category/setup-abc12/", want: publichelp.ErrInvalidURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reader.Article(context.Background(), origin+tt.path); !errors.Is(err, tt.want) {
				t.Errorf("Article() error = %v, want %v", err, tt.want)
			}
		})
	}
}
