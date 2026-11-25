package freshdesk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

const testHead = `<head><title> %s : Acme Support </title><meta property="og:site_name" content="Acme Support"></head>`

const testHome = `<html lang="en">` + testHead + `<body>
<header><a href="/support/solutions">Solutions</a><a href="/support/solutions/900">Pinned</a></header>
<main><h1 class="fw-page-title">Knowledge base</h1>
<section class="fw-category-wrapper"><h2><a href="/support/solutions/100">Guides</a></h2>
  <a class="fw-heading" href="/support/solutions/folders/11"><div>Start (3)</div></a>
  <a href="/support/solutions/articles/1001-first-steps"><div>First steps</div></a></section>
<section class="fw-category-wrapper"><h2><a href="/support/solutions/200">Billing</a></h2></section>
<section class="fw-category-wrapper"><h2><a href="/support/solutions/300">Retired</a></h2></section>
</main><footer><a href="/support/solutions/901">Footer</a></footer></body></html>`

// The current theme: a folder's block on the category page also links the
// folders inside it.
const testGuides = `<html>` + testHead + `<body><main>
<nav aria-label="breadcrumb"><ol class="breadcrumb"><li><a href="/support/solutions">Knowledge base</a></li></ol></nav>
<div class="fw-folder"><a class="fw-heading" href="/support/solutions/folders/11"><div>Start (3)</div></a>
  <ul><li><a href="/support/solutions/folders/12"><div>Deep dive</div></a></li>
  <li><a href="/support/solutions/articles/1001-first-steps"><div>First steps</div></a></li></ul></div>
<div class="fw-folder"><a class="fw-heading" href="/support/solutions/folders/13"><div>Reference (2)</div></a></div>
</main></body></html>`

const testStart = `<html>` + testHead + `<body><main>
<nav aria-label="breadcrumb"><a href="/support/solutions/100">Guides</a></nav>
<ul class="fw-articles">
<li><a href="/support/solutions/folders/12"><div><span class="icon-folder"></span></div><div>Deep dive</div></a></li>
<li><a href="/support/solutions/articles/1001-first-steps" class="row"><div><div><span class="icon-article"></span></div>
  <div class="line-clamp-2">First steps</div></div><div class="fw-modified">
   Modified on Thu, 27 Aug at 12:01 PM </div></a></li>
<li><a href="/support/solutions/articles/1002-second-steps"><div>Second steps</div></a></li></ul>
<nav aria-label="Page Navigation"><ul class="pagination"><li><span>1</span></li>
<li><a href="/support/solutions/folders/11/page/2">2</a></li><li><a href="/support/solutions/folders/11/page/2">Next</a></li></ul></nav>
</main></body></html>`

const testStartPage2 = `<html>` + testHead + `<body><main><ul class="fw-articles">
<li><a href="/support/solutions/articles/1001-first-steps"><div>First steps</div></a></li>
<li><a href="/support/solutions/articles/1003-third-steps"><div>Third steps</div></a></li></ul>
<nav aria-label="Page Navigation"><a href="/support/solutions/folders/11">1</a><span>2</span></nav></main></body></html>`

// The older theme: the name is followed by a count, and folders of the same
// category link each other from a side menu.
const testBilling = `<html>` + testHead + `<body><section class="main content">
<div class="breadcrumb"><a href="/support/solutions">Solution home</a></div><h2 class="heading">Billing</h2>
<section class="article-list"><div class="list-lead"><a href="/support/solutions/folders/21" title="Invoices"> Invoices <span class="item-count">1</span></a></div>
<ul><li><a href="/support/solutions/articles/2001-read-an-invoice">Read an invoice</a></li></ul>
<a href="/support/solutions/folders/21" class="see-more">View all 1</a></section>
<section class="article-list"><div class="list-lead"><a href="/support/solutions/folders/22"> Refunds <span class="item-count">1</span></a></div></section>
</section></body></html>`

func testOldFolder(name, article, title, neighbor string) string {
	return `<html>` + fmt.Sprintf(testHead, name) + `<body><section class="main content">
<div class="breadcrumb"><a href="/support/solutions">Solution home</a><a href="/support/solutions/200">Billing</a></div>
<h2 class="heading">` + name + `</h2><section class="article-list c-list">
<div class="c-row"><a class="c-link" href="/support/solutions/articles/` + article + `">` + title + `</a></div></section></section>
<section class="sidebar"><a href="/support/solutions/folders/` + neighbor + `">Neighbor</a></section></body></html>`
}

const testArticle = `<html><head><title> SEO title : Acme Support </title><meta name="description" content="Start here."></head><body>
<header><a href="/support/home">Home</a></header><main>
<nav aria-label="breadcrumb"><a href="/support/solutions/folders/11">Start</a></nav>
<h1 class="fw-page-title">First steps</h1><p>Modified on: Wed, 21 Jun, 2023 at 10:00 AM</p>
<div class="fw-content fw-content--single-article line-numbers">
<h1 class="fw-page-title">First steps</h1><hr>
<p>Welcome. Read <a href="/support/solutions/articles/1002-second-steps">the next step</a>.</p>
<p><img src="data:image/gif;base64,R0lGOD" data-src="/helpdesk/attachments/55"><img src="https://cdn.example.com/kept.png"></p>
<div class="attachments"><p>Attachments (1)</p><a class="fw-attachment-item" href="/helpdesk/attachments/77">
<div class="fw-attachment-type"><span class="fw-attachment-ext">pdf</span></div><p class="fw-attachment-name">guide.pdf</p></a></div>
</div><div class="fw-feedback"><h2>Was this article helpful?</h2></div>
<aside><h3>Related Articles</h3><a href="/support/solutions/articles/1003-third-steps">Third steps</a></aside>
</main></body></html>`

const testOldArticle = `<html><head><title> Read an invoice : Acme Support </title></head><body>
<section class="help-center"><h2 class="heading">How can we help you today?</h2></section>
<section class="main content" id="article-show-2001"><div class="breadcrumb"><a href="/support/solutions/folders/21">Invoices</a></div>
<h2 class="heading">Read an invoice
  <a href="javascript:print();" class="solution-print--icon"><span class="text-print">Print</span></a></h2>
<p>Modified on: Mon, 6 Mar, 2023 at 1:18 PM</p><hr />
<article class="article-body" id="article-body" rel="image-enlarge"><p>Each invoice lists its lines.</p></article>
<p class="article-vote">Did you find it helpful?</p></section></body></html>`

const testCustomArticle = `<html><head><title> Custom theme : Acme Support </title></head><body>
<main><nav><a href="/support/home">Home</a></nav><h1>Custom theme</h1><p>Only the text matters.</p>
<form action="/support/search"><input name="term"></form></main></body></html>`

// testPortal serves a small Freshdesk portal and returns its address.
func testPortal(t *testing.T) (string, *Reader) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := server.URL
		pages := map[string]string{
			"/support/solutions":                               fmt.Sprintf(testHome, "Knowledge base"),
			"/support/home":                                    fmt.Sprintf(testHome, "Home | Support"),
			"/fr/support/home":                                 fmt.Sprintf(testHome, "Accueil"),
			"/support/login":                                   `<html><body><form><input type="password"></form></body></html>`,
			"/support/solutions/100":                           fmt.Sprintf(testGuides, "Guides"),
			"/support/solutions/200":                           fmt.Sprintf(testBilling, "Billing"),
			"/support/solutions/300":                           `<html>` + fmt.Sprintf(testHead, "Retired") + `<body><a href="/support/solutions/folders/31">Gone</a></body></html>`,
			"/support/solutions/folders/11":                    fmt.Sprintf(testStart, "Start"),
			"/support/solutions/folders/11/page/2":             fmt.Sprintf(testStartPage2, "Start"),
			"/support/solutions/folders/12":                    testOldFolder("Deep dive", "1004-internals", "Internals", "12"),
			"/support/solutions/folders/13":                    testOldFolder("Reference", "1002-second-steps", "Second steps", "13") + `<a href="/support/solutions/articles/1005-glossary">Glossary</a>`,
			"/support/solutions/folders/21":                    testOldFolder("Invoices", "2001-read-an-invoice", "Read an invoice", "22"),
			"/support/solutions/folders/22":                    testOldFolder("Refunds", "2002-ask-for-a-refund", "Ask for a refund", "21"),
			"/support/solutions/articles/1001-first-steps":     testArticle,
			"/support/solutions/articles/2001-read-an-invoice": testOldArticle,
			"/support/solutions/articles/2002-custom-theme":    testCustomArticle,
			"/support/solutions/articles/2003-nothing":         `<html><body><div>Nothing to read.</div></body></html>`,
			"/support/sitemap.xml": `<urlset><url><loc>` + origin + `/support/solutions/100</loc></url>` +
				`<url><loc>` + origin + `/support/solutions/folders/11</loc></url>` +
				`<url><loc>` + origin + `/support/solutions/articles/1001-first-steps</loc></url>` +
				`<url><loc>` + origin + `/support/solutions/articles/9001-unlisted-note</loc></url>` +
				`<url><loc>` + origin + `/fr/support/solutions/articles/9002-note-en-francais</loc></url></urlset>`,
		}
		w.Header().Set("X-Fw-Ratelimiting-Managed", "true")
		w.Header().Add("X-Ratelimit-Remaining", "299")
		w.Header().Add("X-Ratelimit-Remaining", r.URL.Query().Get("left")+"199.0")
		http.SetCookie(w, &http.Cookie{Name: "_helpkit_session", Value: "x"})
		switch r.URL.Path {
		case "/":
			http.Redirect(w, r, "/support/home", http.StatusFound)
		case "/support/solutions/articles/3001-members-only", "/members":
			http.Redirect(w, r, "/support/login", http.StatusFound)
		case "/support/solutions/articles/3002-too-fast":
			w.Header().Set("Retry-After", "43")
			http.Error(w, "You have exceeded the limit of requests per hour", http.StatusForbidden)
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
	origin, reader := testPortal(t)
	fetch := publichelp.NewFetcherWithHTTP(http.DefaultClient)
	open := func(path string) *publichelp.Page {
		t.Helper()
		page, err := fetch.Open(context.Background(), origin+path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		return page
	}
	home := open("/")
	site, err := reader.Probe(context.Background(), home)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if site.Platform != Key || site.BaseURL != origin+"/support/solutions" || site.Name != "Acme Support" || site.Locale != "en" {
		t.Errorf("Probe() = %+v", site)
	}
	if site, err := reader.Probe(context.Background(), open("/fr/support/home")); err != nil || site.BaseURL != origin+"/fr/support/solutions" {
		t.Errorf("Probe() of a language's home = %+v, %v", site, err)
	}
	// A portal that sets no session cookie still sends the budget header.
	budget := &publichelp.Page{URL: home.URL, Status: http.StatusOK, Header: http.Header{"X-Fw-Ratelimiting-Managed": {"true"}}, Body: home.Body}
	if _, err := reader.Probe(context.Background(), budget); err != nil {
		t.Errorf("Probe() without the cookie error = %v", err)
	}
	var known *publichelp.PlatformError
	if _, err := reader.Probe(context.Background(), open("/members")); !errors.As(err, &known) || !errors.Is(err, publichelp.ErrRestricted) {
		t.Errorf("Probe() of a sign-in page error = %v, want a restricted platform error", err)
	}

	elsewhere, _ := url.Parse("https://www.example.com/")
	rejected := []struct {
		name string
		page *publichelp.Page
	}{
		{name: "plain website", page: &publichelp.Page{URL: elsewhere, Status: http.StatusOK, Header: http.Header{}, Body: []byte(`<html><body><a href="/support/solutions">Help</a></body></html>`)}},
		{name: "other platform", page: &publichelp.Page{URL: elsewhere, Status: http.StatusOK, Header: http.Header{"X-Crisp-Ray": {"x"}}, Body: []byte(`<div class="csh-wrapper"></div>`)}},
		{name: "another Freshworks product", page: &publichelp.Page{URL: elsewhere, Status: http.StatusOK, Header: http.Header{"X-Fw-Ratelimiting-Managed": {"true"}}, Body: []byte(`<html><body>CRM</body></html>`)}},
		{name: "not served", page: &publichelp.Page{URL: home.URL, Status: http.StatusForbidden, Header: home.Header, Body: nil}},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reader.Probe(context.Background(), tt.page); !errors.Is(err, publichelp.ErrNotHelpCenter) || errors.As(err, &known) {
				t.Errorf("Probe() error = %v, want ErrNotHelpCenter", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	origin, reader := testPortal(t)
	requests := 0
	site, err := reader.Load(context.Background(), origin+"/support/solutions", func() { requests++ })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Folder 12 sits inside 11, which links it; 21 and 22 link each other
	// and stay side by side; "Second steps" stays in the folder listing it
	// first; the category whose only folder is gone is left out.
	want := `category-100 Guides []
  11 Start [1001 First steps, 1002 Second steps, 1003 Third steps]
    12 Deep dive [1004 Internals]
  13 Reference [1005 Glossary]
category-200 Billing []
  21 Invoices [2001 Read an invoice]
  22 Refunds [2002 Ask for a refund]
other Other articles [9001 Unlisted note]
`
	if got := outline(site.Collections, ""); got != want {
		t.Errorf("Load() collections =\n%s\nwant\n%s", got, want)
	}
	// Home, sitemap, three categories, six folders and one more page.
	if requests != 12 {
		t.Errorf("Load() made %d requests, want 12", requests)
	}
	if site.Name != "Acme Support" || site.Locale != "en" || site.BaseURL != origin+"/support/solutions" {
		t.Errorf("Load() site = %q %q %q", site.Name, site.Locale, site.BaseURL)
	}
	if got := site.Collections[0].Children[0].URL; got != origin+"/support/solutions/folders/11" {
		t.Errorf("folder URL = %q", got)
	}
	for href, want := range map[string]publichelp.LinkTarget{
		origin + "/support/solutions/articles/1001-first-steps":  {Kind: publichelp.LinkArticle, ID: "1001", Certain: true},
		origin + "/support/solutions/articles/1001":              {Kind: publichelp.LinkArticle, ID: "1001", Certain: true},
		origin + "/en/support/solutions/articles/1001-renamed#a": {Kind: publichelp.LinkArticle, ID: "1001", Fragment: "a", Certain: true},
		origin + "/support/solutions/folders/11/page/2":          {Kind: publichelp.LinkCollection, ID: "11", Certain: true},
		origin + "/support/solutions/100":                        {Kind: publichelp.LinkCollection, ID: "category-100", Certain: true},
	} {
		if got, ok := site.LinkTarget(href); !ok || got != want {
			t.Errorf("LinkTarget(%q) = %+v, %v", href, got, ok)
		}
	}
	if _, ok := site.LinkTarget(origin + "/support/tickets/new"); ok {
		t.Error("LinkTarget() matched the ticket form")
	}
}

func TestLoad_Canceled(t *testing.T) {
	origin, reader := testPortal(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Load(ctx, origin+"/support/solutions", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Load() error = %v, want context.Canceled", err)
	}
}

func TestArticle(t *testing.T) {
	origin, reader := testPortal(t)
	article, err := reader.Article(context.Background(), origin+"/support/solutions/articles/1001-first-steps")
	if err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if article.ID != "1001" || article.Title != "First steps" || article.Description != "Start here." {
		t.Errorf("Article() = %q %q %q", article.ID, article.Title, article.Description)
	}
	for _, want := range []string{
		`href="` + origin + `/support/solutions/articles/1002-second-steps"`,
		`src="` + origin + `/helpdesk/attachments/55"`, `src="https://cdn.example.com/kept.png"`,
		`href="` + origin + `/helpdesk/attachments/77"`, "guide.pdf",
	} {
		if !strings.Contains(article.HTML, want) {
			t.Errorf("Article() HTML lacks %s:\n%s", want, article.HTML)
		}
	}
	for _, chrome := range []string{"<h1", "<hr", "Modified on", "Attachments (1)", ">pdf<", "helpful", "Related Articles", "data:image"} {
		if strings.Contains(article.HTML, chrome) {
			t.Errorf("Article() HTML keeps %q:\n%s", chrome, article.HTML)
		}
	}
}

func TestArticle_Themes(t *testing.T) {
	origin, reader := testPortal(t)
	tests := []struct {
		name, path, title, html string
	}{
		{name: "older theme", path: "/support/solutions/articles/2001-read-an-invoice", title: "Read an invoice", html: "<p>Each invoice lists its lines.</p>"},
		{name: "unknown theme", path: "/support/solutions/articles/2002-custom-theme", title: "Custom theme", html: "<p>Only the text matters.</p>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article, err := reader.Article(context.Background(), origin+tt.path)
			if err != nil {
				t.Fatalf("Article() error = %v", err)
			}
			if article.Title != tt.title || strings.TrimSpace(article.HTML) != tt.html {
				t.Errorf("Article() = %q %q, want %q %q", article.Title, article.HTML, tt.title, tt.html)
			}
		})
	}
}

func TestArticle_Unreadable(t *testing.T) {
	origin, reader := testPortal(t)
	tests := []struct {
		name, path string
		want       error
	}{
		{name: "missing page", path: "/support/solutions/articles/4004-gone", want: publichelp.ErrNotFound},
		{name: "forwarded to sign-in", path: "/support/solutions/articles/3001-members-only", want: publichelp.ErrRestricted},
		{name: "portal out of requests", path: "/support/solutions/articles/3002-too-fast", want: publichelp.ErrUnreachable},
		{name: "page without an article", path: "/support/solutions/articles/2003-nothing", want: publichelp.ErrNotHelpCenter},
		{name: "not an article address", path: "/support/solutions/folders/11", want: publichelp.ErrInvalidURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reader.Article(context.Background(), origin+tt.path); !errors.Is(err, tt.want) {
				t.Errorf("Article() error = %v, want %v", err, tt.want)
			}
		})
	}
}

// A portal that reports few requests left makes the reader wait, and the
// wait ends when the import is stopped.
func TestArticle_LowBudget(t *testing.T) {
	origin, reader := testPortal(t)
	address := origin + "/support/solutions/articles/1001-first-steps?left=-"
	reader.pause = 30 * time.Millisecond
	started := time.Now()
	if _, err := reader.Article(context.Background(), address); err != nil {
		t.Fatalf("Article() error = %v", err)
	}
	if waited := time.Since(started); waited < reader.pause {
		t.Errorf("Article() returned after %s, want a pause of %s", waited, reader.pause)
	}
	reader.pause = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := reader.Article(ctx, address); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Article() error = %v, want context.DeadlineExceeded", err)
	}
}
