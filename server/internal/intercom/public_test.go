package intercom_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/intercom/intercomtest"
)

func newTestPublicClient() *intercom.PublicClient {
	// The fake listens on loopback, which the production client refuses.
	return intercom.NewPublicClientWithHTTP(&http.Client{Timeout: 10 * time.Second})
}

func seedPublicSite() *intercomtest.PublicSite {
	site := intercomtest.NewPublicSite()
	site.Color, site.Headline = "0b17bf", "How can we help?"
	site.Logo, site.Favicon = site.URL+"/assets/logo.png", site.URL+"/assets/favicon.png"
	site.Articles["11"] = intercomtest.PublicArticle{ID: "11", Slug: "getting-started", Title: "Getting started", Description: "First steps",
		Blocks: []map[string]any{{"type": "paragraph", "text": "Hello"}}}
	site.Articles["12"] = intercomtest.PublicArticle{ID: "12", Slug: "invoices", Title: "Invoices"}
	site.Articles["13"] = intercomtest.PublicArticle{ID: "13", Slug: "refunds", Title: "Refunds"}
	site.Collections = []intercomtest.PublicCollection{
		{ID: "1", Slug: "basics", Name: "Basics", Description: "Start here", ArticleIDs: []string{"11"}},
		{ID: "2", Slug: "billing", Name: "Billing", ArticleIDs: []string{"12"}, Children: []intercomtest.PublicCollection{
			{ID: "3", Slug: "refunds", Name: "Refunds", ArticleIDs: []string{"13"}},
		}},
	}
	return site
}

func TestNormalizePublicURL(t *testing.T) {
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{name: "bare host", in: "docs.acme.com", want: "https://docs.acme.com/"},
		{name: "locale home", in: "https://docs.acme.com/en", want: "https://docs.acme.com/en/"},
		{name: "article page", in: "https://docs.acme.com/en/articles/12-invoices?x=1#top", want: "https://docs.acme.com/en/"},
		{name: "collection page", in: " https://intercom.help/acme/en/collections/2-billing ", want: "https://intercom.help/acme/en/"},
		{name: "empty", in: "  ", wantErr: true},
		{name: "not http", in: "ftp://docs.acme.com", wantErr: true},
		{name: "credentials", in: "https://user:pass@docs.acme.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := intercom.NormalizePublicURL(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizePublicURL(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err != nil {
				if !errors.Is(err, intercom.ErrInvalidPublicURL) {
					t.Fatalf("error = %v, want ErrInvalidPublicURL", err)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("NormalizePublicURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadHelpCenterReadsTreeInSourceOrder(t *testing.T) {
	site := seedPublicSite()
	defer site.Close()

	progress := 0
	// The bare host redirects to the locale home, as a real help center does.
	got, err := loadFromLoopback(t, site, site.URL, func() { progress++ })
	if err != nil {
		t.Fatalf("LoadHelpCenter: %v", err)
	}
	if got.BaseURL != site.Home() || got.Name != "Acme Help Center" || got.Locale != "en" {
		t.Fatalf("site = %q %q %q", got.BaseURL, got.Name, got.Locale)
	}
	if got.Theme.Color != "0b17bf" || got.Theme.Headline != "How can we help?" || got.Theme.LogoURL == "" || got.Theme.FaviconURL == "" {
		t.Fatalf("theme = %+v", got.Theme)
	}
	if len(got.Theme.HeaderLinks) != 1 || got.Theme.HeaderLinks[0].Title != "API Reference" ||
		len(got.Theme.SocialLinks) != 1 || got.Theme.SocialLinks[0].Provider != "github" {
		t.Fatalf("links = %+v %+v", got.Theme.HeaderLinks, got.Theme.SocialLinks)
	}
	if len(got.Collections) != 2 || got.Collections[0].Name != "Basics" || got.Collections[1].Name != "Billing" {
		t.Fatalf("collections = %+v", got.Collections)
	}
	billing := got.Collections[1]
	if len(billing.Articles) != 1 || billing.Articles[0].ID != "12" || billing.Articles[0].Title != "Invoices" {
		t.Fatalf("billing articles = %+v", billing.Articles)
	}
	if len(billing.Children) != 1 || billing.Children[0].ID != "3" || billing.Children[0].Articles[0].ID != "13" {
		t.Fatalf("billing children = %+v", billing.Children)
	}
	if got.Collections[0].Description != "Start here" || got.Collections[0].Articles[0].Summary != "First steps" {
		t.Fatalf("basics = %+v", got.Collections[0])
	}
	if progress != 3 {
		t.Fatalf("progress calls = %d, want one per page (3)", progress)
	}
}

// loadFromLoopback loads a fake site. NormalizePublicURL refuses loopback
// names, so the fake is addressed through a public-looking host that the
// test client resolves to it.
func loadFromLoopback(t *testing.T, site *intercomtest.PublicSite, address string, progress func()) (*intercom.PublicHelpCenter, error) {
	t.Helper()
	return newTestPublicClient().LoadHelpCenter(context.Background(), address, progress)
}

func TestLoadHelpCenterErrors(t *testing.T) {
	t.Run("not a help center", func(t *testing.T) {
		site := seedPublicSite()
		defer site.Close()
		site.NotHelpCenter = true
		if _, err := loadFromLoopback(t, site, site.Home(), nil); !errors.Is(err, intercom.ErrNotPublicHelpCenter) {
			t.Fatalf("error = %v, want ErrNotPublicHelpCenter", err)
		}
	})
	t.Run("restricted", func(t *testing.T) {
		site := seedPublicSite()
		defer site.Close()
		site.Restricted = true
		if _, err := loadFromLoopback(t, site, site.Home(), nil); !errors.Is(err, intercom.ErrPublicHelpCenterRestricted) {
			t.Fatalf("error = %v, want ErrPublicHelpCenterRestricted", err)
		}
	})
	t.Run("missing page", func(t *testing.T) {
		site := seedPublicSite()
		defer site.Close()
		if _, err := loadFromLoopback(t, site, site.URL+"/fr/", nil); !errors.Is(err, intercom.ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("brief outage is retried", func(t *testing.T) {
		site := seedPublicSite()
		defer site.Close()
		site.FailNext = 1
		if _, err := loadFromLoopback(t, site, site.Home(), nil); err != nil {
			t.Fatalf("LoadHelpCenter after one 503: %v", err)
		}
	})
	t.Run("lasting outage fails", func(t *testing.T) {
		site := seedPublicSite()
		defer site.Close()
		site.FailNext = 10
		if _, err := loadFromLoopback(t, site, site.Home(), nil); !errors.Is(err, intercom.ErrPublicHelpCenterUnreachable) {
			t.Fatalf("error = %v, want ErrPublicHelpCenterUnreachable", err)
		}
	})
}

func TestPublicClientRefusesInternalAddresses(t *testing.T) {
	site := seedPublicSite()
	defer site.Close()

	// The production client must not read a loopback or private address,
	// however it is spelled, and must not have sent a request to find out.
	for _, address := range []string{site.Home(), "http://localhost/en/", "http://10.0.0.5/en/", "http://169.254.169.254/latest/meta-data/"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := intercom.NewPublicClient().LoadHelpCenter(ctx, address, nil)
		cancel()
		if !errors.Is(err, intercom.ErrInvalidPublicURL) {
			t.Errorf("LoadHelpCenter(%q) error = %v, want ErrInvalidPublicURL", address, err)
		}
	}
	if got := site.RequestCount("/"); got != 0 {
		t.Fatalf("the fake received %d requests from the guarded client", got)
	}
}

func TestPublicArticle(t *testing.T) {
	site := seedPublicSite()
	defer site.Close()
	client := newTestPublicClient()

	// The bare number is enough; the published address supplies the slug.
	article, err := client.Article(context.Background(), site.URL+"/en/articles/11")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if article.ID != "11" || article.Title != "Getting started" || article.Description != "First steps" ||
		article.URL != site.ArticleURL("11") || len(article.Blocks) != 1 || article.Blocks[0].Type != "paragraph" {
		t.Fatalf("article = %+v", article)
	}
	if _, err := client.Article(context.Background(), site.URL+"/en/articles/999"); !errors.Is(err, intercom.ErrNotFound) {
		t.Fatalf("missing article error = %v, want ErrNotFound", err)
	}
}

func TestPublicHelpCenterLinkTarget(t *testing.T) {
	site := seedPublicSite()
	defer site.Close()
	loaded, err := loadFromLoopback(t, site, site.Home(), nil)
	if err != nil {
		t.Fatalf("LoadHelpCenter: %v", err)
	}
	tests := []struct {
		name string
		href string
		want intercom.PublicLinkTarget
		ok   bool
	}{
		{name: "article on the help center host", href: site.URL + "/en/articles/12-invoices#pay",
			want: intercom.PublicLinkTarget{Kind: intercom.PublicLinkArticle, ID: "12", Fragment: "pay", Certain: true}, ok: true},
		{name: "collection", href: site.URL + "/en/collections/2-billing",
			want: intercom.PublicLinkTarget{Kind: intercom.PublicLinkCollection, ID: "2", Certain: true}, ok: true},
		{name: "own name on the shared host", href: "https://intercom.help/acme/en/articles/12-invoices",
			want: intercom.PublicLinkTarget{Kind: intercom.PublicLinkArticle, ID: "12", Certain: true}, ok: true},
		{name: "another company on the shared host", href: "https://intercom.help/other/en/articles/12-x"},
		{name: "shared host name prefix is not a match", href: "https://intercom.help/acme-labs/en/articles/12-x"},
		{name: "another site", href: "https://support.example.org/en/articles/12-invoices"},
		{name: "already rewritten relative link", href: "/articles/12-invoices-abc"},
		{name: "anchor", href: "#section"},
		{name: "mail", href: "mailto:help@acme.example"},
		{name: "own host, not an article", href: site.URL + "/en/"},
		{name: "article number deeper in the path is not a help center link", href: site.URL + "/en/articles/12-invoices/print"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := loaded.LinkTarget(tt.href)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("LinkTarget(%q) = %+v, %v; want %+v, %v", tt.href, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPublicSourceID(t *testing.T) {
	for in, want := range map[string]string{
		"https://docs.acme.com/en/articles/5563688-getting-started": "5563688",
		"https://docs.acme.com/en/articles/5563688":                 "5563688",
		"https://docs.acme.com/en/articles/no-number/":              "no-number",
	} {
		if got := intercom.PublicSourceID(in); got != want {
			t.Errorf("PublicSourceID(%q) = %q, want %q", in, got, want)
		}
	}
}
