package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/intercom/intercomtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var _brandTestPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestFetchHelpcenterAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/logo.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(_brandTestPNG)
		case "/assets/favicon":
			// No extension and a wrong label, as help center CDNs serve it.
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(_brandTestPNG)
		case "/logo.svg":
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
		case "/page.png":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>not an image</html>"))
		case "/anim.gif":
			w.Header().Set("Content-Type", "image/gif")
			_, _ = w.Write([]byte("GIF89a\x01\x00\x01\x00"))
		case "/huge.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(append(append([]byte{}, _brandTestPNG...), make([]byte, helpcenterAssetMaxBytes)...))
		case "/empty.png":
			w.Header().Set("Content-Type", "image/png")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tests := []struct {
		name, url, wantType, wantErr string
	}{
		{name: "declared png", url: server.URL + "/logo.png", wantType: "image/png"},
		{name: "mislabeled image is sniffed", url: server.URL + "/assets/favicon?v=2", wantType: "image/png"},
		{name: "svg", url: server.URL + "/logo.svg", wantType: "image/svg+xml"},
		{name: "html page", url: server.URL + "/page.png", wantErr: "did not return an image"},
		{name: "unsupported image type", url: server.URL + "/anim.gif", wantErr: "only PNG, JPEG, WebP, SVG, and ICO"},
		{name: "too large", url: server.URL + "/huge.png", wantErr: "larger than 2 MB"},
		{name: "empty", url: server.URL + "/empty.png", wantErr: "empty file"},
		{name: "missing", url: server.URL + "/missing.png", wantErr: "answered 404"},
		{name: "not http", url: "file:///etc/passwd", wantErr: "starting with https://"},
		{name: "credentials", url: "https://user:pass@example.com/a.png", wantErr: "starting with https://"},
		{name: "blank", url: "  ", wantErr: "starting with https://"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentType, body, err := fetchHelpcenterAsset(context.Background(), server.Client(), tt.url)
			if tt.wantErr != "" {
				if !errors.Is(err, ErrHelpcenterBrandInvalid) || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want ErrHelpcenterBrandInvalid containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || contentType != tt.wantType || len(body) == 0 {
				t.Fatalf("got %q, %d bytes, %v; want %q", contentType, len(body), err, tt.wantType)
			}
		})
	}
}

func TestFetchHelpcenterAssetRefusesInternalAddresses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The production client is the image client, which dials public
	// addresses only.
	_, _, err := fetchHelpcenterAsset(ctx, brandPageHTTPClient, "http://127.0.0.1:1/logo.png")
	if !errors.Is(err, ErrHelpcenterBrandInvalid) || !strings.Contains(err.Error(), "public internet") {
		t.Fatalf("error = %v, want a public-internet refusal", err)
	}
}

func TestParseBrandPage(t *testing.T) {
	base, _ := url.Parse("https://acme.example/pricing/")
	tests := []struct {
		name string
		page string
		want brandPage
	}{
		{
			name: "metadata and a header logo",
			page: `<html><head><title>Pricing | Acme</title>
				<meta property="og:site_name" content="Acme">
				<meta name="theme-color" content="#0B17BF">
				<link rel="icon" href="/favicon.ico"><link rel="apple-touch-icon" href="/touch.png">
				</head><body>
				<img src="/hero-logo-wall.png" alt="Customer logos">
				<header><a href="/"><img class="brand" src="/img/acme-logo.svg" alt="Acme"></a></header></body></html>`,
			want: brandPage{siteName: "Acme", themeColor: "#0b17bf", faviconURL: "https://acme.example/touch.png", logoURL: "https://acme.example/img/acme-logo.svg"},
		},
		{
			name: "site name from the title, logo by class in a navbar div",
			page: `<html><head><title>Acme - Test management for teams</title><link rel="shortcut icon" href="fav.png"></head>
				<body><div class="site-navbar"><img class="navbar-logo" src="https://cdn.acme.example/mark.png"></div></body></html>`,
			want: brandPage{siteName: "Acme", faviconURL: "https://acme.example/pricing/fav.png", logoURL: "https://cdn.acme.example/mark.png"},
		},
		{
			name: "no logo, bad color, inline data image ignored",
			page: `<html><head><title>Acme Help Center</title><meta name="theme-color" content="rgb(1,2,3)"></head>
				<body><header><img alt="logo" src="data:image/png;base64,AAAA"><img src="/team.jpg" alt="Team"></header></body></html>`,
			want: brandPage{siteName: "Acme"},
		},
		{
			name: "short color; a logo outside the header is not trusted",
			page: `<html><head><meta name="application-name" content="Acme App"><meta name="theme-color" content="#abc"></head>
				<body><main><img id="logo" src="logo.png"></main></body></html>`,
			want: brandPage{siteName: "Acme App", themeColor: "#aabbcc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := html.Parse(strings.NewReader(tt.page))
			if err != nil {
				t.Fatal(err)
			}
			got := parseBrandPage(root, base)
			if got.siteName != tt.want.siteName || got.themeColor != tt.want.themeColor ||
				got.faviconURL != tt.want.faviconURL || got.logoURL != tt.want.logoURL {
				t.Fatalf("parseBrandPage = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestBrandNameFromHelpCenter(t *testing.T) {
	for in, want := range map[string]string{
		"Northwind Help Center": "Northwind",
		"Acme Support Centre":   "Acme",
		"Acme | Knowledge Base": "Acme",
		"Acme Docs":             "Acme",
		"Help Center":           "Help Center",
		"  Helpful Robots  ":    "Helpful Robots",
		"":                      "",
	} {
		if got := brandNameFromHelpCenter(in); got != want {
			t.Errorf("brandNameFromHelpCenter(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProposeBrandFromIntercomHelpCenter(t *testing.T) {
	site := intercomtest.NewPublicSite()
	defer site.Close()
	site.Name, site.Color, site.Headline = "Acme Help Center", "0B17BF", "How can we help?"
	site.Logo, site.Favicon = site.URL+"/assets/logo.png", site.URL+"/assets/favicon"
	plain := &http.Client{Timeout: 5 * time.Second}
	importer := &HelpcenterBrandImporter{intercom: intercom.NewPublicClientWithHTTP(plain), pages: plain}

	proposal, err := importer.Propose(context.Background(), site.URL)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if proposal.Platform != model.HelpcenterBrandPlatformIntercom || proposal.SourceURL != site.Home() || proposal.OfficialDocsURL != site.Home() ||
		proposal.HelpCenterName != "Acme Help Center" || proposal.BrandName != "Acme" || proposal.BrandColor != "#0b17bf" ||
		proposal.HeroTitle != "How can we help?" || proposal.FaviconURL != site.Favicon {
		t.Fatalf("proposal = %+v", proposal)
	}
	// Intercom's header logo is the dark-background mark; the company site
	// in this test cannot be reached, so no light logo is found.
	if proposal.LogoDarkURL != site.Logo || proposal.LogoURL != "" || proposal.ThemeMode != "dark" {
		t.Fatalf("logos: light=%q dark=%q theme=%q", proposal.LogoURL, proposal.LogoDarkURL, proposal.ThemeMode)
	}
	if len(proposal.Notes) != 1 || !strings.Contains(proposal.Notes[0], "dark theme") {
		t.Fatalf("notes = %v", proposal.Notes)
	}
	if len(proposal.HeaderLinks) != 1 || proposal.HeaderLinks[0].Label != "API Reference" || !proposal.HeaderLinks[0].External ||
		len(proposal.SocialLinks) != 1 || proposal.SocialLinks[0].Platform != "github" {
		t.Fatalf("links = %+v %+v", proposal.HeaderLinks, proposal.SocialLinks)
	}
}

func TestProposeBrandFromWebsite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Acme - Tools</title><meta name="theme-color" content="#112233">
			<link rel="icon" href="/favicon.png"></head><body><header><img src="/logo.png" alt="Acme logo"></header></body></html>`))
	}))
	defer server.Close()
	plain := &http.Client{Timeout: 5 * time.Second}
	importer := &HelpcenterBrandImporter{intercom: intercom.NewPublicClientWithHTTP(plain), pages: plain}

	proposal, err := importer.Propose(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if proposal.Platform != model.HelpcenterBrandPlatformWebsite || proposal.BrandName != "Acme" || proposal.BrandColor != "#112233" ||
		proposal.LogoURL != server.URL+"/logo.png" || proposal.LogoDarkURL != "" || proposal.FaviconURL != server.URL+"/favicon.png" ||
		proposal.ThemeMode != "" || len(proposal.Notes) != 0 || proposal.OfficialDocsURL != "" {
		t.Fatalf("proposal = %+v", proposal)
	}
}

func TestProposeBrandErrors(t *testing.T) {
	plain := &http.Client{Timeout: 5 * time.Second}
	importer := &HelpcenterBrandImporter{intercom: intercom.NewPublicClientWithHTTP(plain), pages: plain}
	for _, address := range []string{"", "ftp://example.com", "https://user:pw@example.com"} {
		if _, err := importer.Propose(context.Background(), address); !errors.Is(err, ErrHelpcenterBrandInvalid) {
			t.Errorf("Propose(%q) error = %v, want ErrHelpcenterBrandInvalid", address, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The production importer refuses an internal address outright.
	if _, err := NewHelpcenterBrandImporter().Propose(ctx, "http://127.0.0.1:1/"); !errors.Is(err, ErrHelpcenterBrandInvalid) {
		t.Fatalf("internal address error = %v, want ErrHelpcenterBrandInvalid", err)
	}
}
