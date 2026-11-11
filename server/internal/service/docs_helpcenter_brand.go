package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"

	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/netguard"
)

// ErrHelpcenterBrandInvalid wraps a brand import or asset download problem
// that the person who entered the address can fix. Its message is safe to
// show.
var ErrHelpcenterBrandInvalid = errors.New("invalid brand import")

const (
	// helpcenterAssetMaxBytes matches the limit of a direct asset upload.
	helpcenterAssetMaxBytes = 2 << 20
	brandPageMaxBytes       = 4 << 20
	brandUserAgent          = "Mozilla/5.0 (compatible; HelpinImporter/1.0; +https://helpin.ai)"
)

// _helpcenterAssetTypes are the image types a help center asset may have.
var _helpcenterAssetTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true,
	"image/svg+xml": true, "image/x-icon": true, "image/vnd.microsoft.icon": true,
}

// brandPageHTTPClient reads public web pages for brand import. It reaches
// only public addresses, including after a redirect. Tests replace it.
var brandPageHTTPClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		Proxy: nil, DialContext: netguard.DialContext(false),
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second,
	},
}

func brandInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrHelpcenterBrandInvalid, fmt.Sprintf(format, args...))
}

// UploadAssetFromURL downloads an image from a public address and stores it
// as a help center asset, with the same type and size rules as a file upload.
func (s *DocsHelpcenterService) UploadAssetFromURL(ctx context.Context, workspaceID, assetType, sourceURL string) (string, error) {
	if s.s3Client == nil {
		return "", fmt.Errorf("file storage not configured")
	}
	contentType, body, err := fetchHelpcenterAsset(ctx, helpscout.ImageHTTPClient, sourceURL)
	if err != nil {
		return "", err
	}
	return s.UploadAsset(ctx, workspaceID, assetType, contentType, int64(len(body)), bytes.NewReader(body))
}

// fetchHelpcenterAsset downloads an image and returns its real content type
// and bytes. The type comes from the bytes when the server mislabels them.
func fetchHelpcenterAsset(ctx context.Context, client *http.Client, sourceURL string) (string, []byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return "", nil, brandInvalid("enter the full address of an image, starting with https://")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", nil, brandInvalid("enter the full address of an image, starting with https://")
	}
	req.Header.Set("User-Agent", brandUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		if errors.Is(err, netguard.ErrNonPublicAddress) {
			return "", nil, brandInvalid("the image address is not on the public internet")
		}
		return "", nil, brandInvalid("the image could not be downloaded. Check the address and try again")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, brandInvalid("the image could not be downloaded (the site answered %d)", resp.StatusCode)
	}
	if resp.ContentLength > helpcenterAssetMaxBytes {
		return "", nil, brandInvalid("the image is larger than 2 MB")
	}
	reader, contentType, err := helpscout.ImageBody(resp, path.Base(parsed.Path))
	if err != nil {
		return "", nil, brandInvalid("the address did not return an image")
	}
	body, err := io.ReadAll(io.LimitReader(reader, helpcenterAssetMaxBytes+1))
	if err != nil {
		return "", nil, brandInvalid("the image could not be downloaded. Check the address and try again")
	}
	if len(body) > helpcenterAssetMaxBytes {
		return "", nil, brandInvalid("the image is larger than 2 MB")
	}
	if len(body) == 0 {
		return "", nil, brandInvalid("the address returned an empty file")
	}
	if !_helpcenterAssetTypes[contentType] {
		return "", nil, brandInvalid("only PNG, JPEG, WebP, SVG, and ICO images are allowed")
	}
	return contentType, body, nil
}

// HelpcenterBrandImporter proposes help center branding from a public help
// center or website. It only reads; applying a proposal is a separate step.
type HelpcenterBrandImporter struct {
	intercom *intercom.PublicClient
	pages    *http.Client
}

// NewHelpcenterBrandImporter creates an importer that reaches only public
// addresses.
func NewHelpcenterBrandImporter() *HelpcenterBrandImporter {
	return &HelpcenterBrandImporter{intercom: intercom.NewPublicClient(), pages: brandPageHTTPClient}
}

// SetBrandImporter replaces the importer that reads public sites. Tests
// point it at a local server.
func (s *DocsHelpcenterService) SetBrandImporter(importer *HelpcenterBrandImporter) {
	s.brandImporter = importer
}

// ProposeBrand reads the branding published at a public help center or
// website address. Nothing is saved.
func (s *DocsHelpcenterService) ProposeBrand(ctx context.Context, sourceURL string) (*model.HelpcenterBrandProposal, error) {
	if s.brandImporter == nil {
		s.brandImporter = NewHelpcenterBrandImporter()
	}
	return s.brandImporter.Propose(ctx, sourceURL)
}

// Propose reads the branding at sourceURL. An Intercom help center publishes
// its name, color, logo, favicon, hero text, and links; any other site is
// read from its page metadata. Nothing is saved.
func (b *HelpcenterBrandImporter) Propose(ctx context.Context, sourceURL string) (*model.HelpcenterBrandProposal, error) {
	site, err := b.intercom.LoadTheme(ctx, sourceURL)
	switch {
	case err == nil:
		return b.proposeFromIntercom(ctx, site), nil
	case errors.Is(err, intercom.ErrInvalidPublicURL):
		return nil, brandInvalid("enter the address of a help center or website, such as https://help.example.com")
	case errors.Is(err, intercom.ErrPublicHelpCenterUnreachable), errors.Is(err, intercom.ErrNotFound):
		return nil, brandInvalid("the site did not respond. Check the address and try again in a moment")
	case ctx.Err() != nil:
		return nil, ctx.Err()
	}
	// Not an Intercom help center (or one behind sign-in): read it as a
	// plain website.
	normalized, err := intercom.NormalizePublicURL(sourceURL)
	if err != nil {
		return nil, brandInvalid("enter the address of a help center or website, such as https://help.example.com")
	}
	page, err := b.readPage(ctx, normalized)
	if err != nil {
		return nil, err
	}
	proposal := &model.HelpcenterBrandProposal{
		Platform:  model.HelpcenterBrandPlatformWebsite,
		SourceURL: page.finalURL.String(),
		BrandName: page.siteName,
		// On a plain website the header logo is drawn for the page's own,
		// usually light, background.
		LogoURL:    page.logoURL,
		FaviconURL: page.faviconURL,
		BrandColor: page.themeColor,
		WebsiteURL: page.finalURL.Scheme + "://" + page.finalURL.Host,
	}
	finishBrandProposal(proposal)
	return proposal, nil
}

func (b *HelpcenterBrandImporter) proposeFromIntercom(ctx context.Context, site *intercom.PublicHelpCenter) *model.HelpcenterBrandProposal {
	theme := site.Theme
	proposal := &model.HelpcenterBrandProposal{
		Platform:        model.HelpcenterBrandPlatformIntercom,
		SourceURL:       site.BaseURL,
		HelpCenterName:  theme.SiteName,
		BrandName:       brandNameFromHelpCenter(theme.SiteName),
		HeroTitle:       theme.Headline,
		HeroSubtitle:    theme.Subheadline,
		FaviconURL:      theme.FaviconURL,
		OfficialDocsURL: site.BaseURL,
		WebsiteURL:      brandWebsiteURL(site.BaseURL, theme.HomeURL),
		// Intercom draws the logo on its colored header, so it is the mark
		// for dark backgrounds.
		LogoDarkURL: theme.LogoURL,
	}
	if color := normalizeBrandColor(theme.Color); color != "" {
		proposal.BrandColor = color
	}
	for i, link := range theme.HeaderLinks {
		label, target := strings.TrimSpace(link.Title), strings.TrimSpace(link.URL)
		if label == "" || !isHTTPURL(target) {
			continue
		}
		proposal.HeaderLinks = append(proposal.HeaderLinks, model.HelpcenterHeaderLink{
			Label: label, URL: target, External: true, Style: "text", Position: i,
		})
	}
	for _, link := range theme.SocialLinks {
		platform := brandSocialPlatform(link.Provider)
		if platform == "" || !isHTTPURL(strings.TrimSpace(link.URL)) {
			continue
		}
		proposal.SocialLinks = append(proposal.SocialLinks, model.HelpcenterSocialLink{Platform: platform, URL: strings.TrimSpace(link.URL)})
	}
	// The company's own site usually shows the logo for light backgrounds.
	if proposal.WebsiteURL != "" {
		if page, err := b.readPage(ctx, proposal.WebsiteURL); err == nil {
			proposal.LogoURL = page.logoURL
			if proposal.FaviconURL == "" {
				proposal.FaviconURL = page.faviconURL
			}
		} else if ctx.Err() == nil {
			slog.DebugContext(ctx, "brand import: company website not readable", "host", hostOfURL(proposal.WebsiteURL), "error", err)
		}
	}
	finishBrandProposal(proposal)
	return proposal
}

// finishBrandProposal fills what follows from the rest of a proposal and
// explains every gap, so the person reviewing it knows what to add by hand.
func finishBrandProposal(p *model.HelpcenterBrandProposal) {
	switch {
	case p.LogoURL == "" && p.LogoDarkURL != "":
		// Only a light-on-dark logo exists; a dark theme shows it correctly.
		p.ThemeMode = "dark"
		p.Notes = append(p.Notes, "No logo for light backgrounds was found, so the dark theme is suggested. Upload a light-background logo to use the light theme.")
	case p.LogoURL == "" && p.LogoDarkURL == "":
		p.Notes = append(p.Notes, "No logo was found. Upload one in the branding settings.")
	}
	if p.BrandColor == "" {
		p.Notes = append(p.Notes, "No brand color was found. Choose one in the branding settings.")
	}
	if p.FaviconURL == "" {
		p.Notes = append(p.Notes, "No favicon was found. Upload one in the branding settings.")
	}
	if p.BrandName == "" {
		p.Notes = append(p.Notes, "No brand name was found. Enter one in the branding settings.")
	}
}

var _helpCenterNameSuffix = regexp.MustCompile(`(?i)\s*[-|:·]?\s*(help\s*cent(er|re)|support\s*cent(er|re)|knowledge\s*base|help\s*desk|support|docs|documentation|help)\s*$`)

// brandNameFromHelpCenter turns "Acme Help Center" into "Acme". A name that
// is nothing but such a suffix is kept whole.
func brandNameFromHelpCenter(name string) string {
	name = strings.TrimSpace(name)
	if trimmed := strings.TrimSpace(_helpCenterNameSuffix.ReplaceAllString(name, "")); trimmed != "" {
		return trimmed
	}
	return name
}

var _hexColor = regexp.MustCompile(`^#?([0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`)

// normalizeBrandColor returns a "#rrggbb" color, or "" for anything else.
func normalizeBrandColor(value string) string {
	match := _hexColor.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return ""
	}
	hex := strings.ToLower(match[1])
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	return "#" + hex
}

var _brandSocialPlatforms = map[string]string{
	"x": "x", "twitter": "x", "twitter-x": "x", "linkedin": "linkedin", "github": "github",
	"youtube": "youtube", "facebook": "facebook", "instagram": "instagram", "discord": "discord", "slack": "slack",
}

func brandSocialPlatform(provider string) string {
	return _brandSocialPlatforms[strings.ToLower(strings.TrimSpace(provider))]
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Hostname() != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

// brandWebsiteURL picks the company's marketing site: the root of the help
// center's registrable domain, since the "home" link of a help center often
// leads into the product instead. A help center on Intercom's shared host
// has no domain of its own, so its home link is used.
func brandWebsiteURL(helpCenterURL, homeURL string) string {
	if host := hostOfURL(helpCenterURL); host != "" && !strings.EqualFold(host, "intercom.help") {
		if registrable, err := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(host)); err == nil {
			return "https://" + registrable
		}
	}
	if isHTTPURL(homeURL) {
		parsed, _ := url.Parse(homeURL)
		return parsed.Scheme + "://" + parsed.Host
	}
	return ""
}

// brandPage is what a web page says about its brand.
type brandPage struct {
	finalURL   *url.URL
	siteName   string
	themeColor string
	faviconURL string
	logoURL    string
}

// readPage fetches a web page and reads its brand metadata.
func (b *HelpcenterBrandImporter) readPage(ctx context.Context, pageURL string) (*brandPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, brandInvalid("enter the address of a help center or website, such as https://help.example.com")
	}
	req.Header.Set("User-Agent", brandUserAgent)
	req.Header.Set("Accept", "text/html")
	resp, err := b.pages.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, netguard.ErrNonPublicAddress) {
			return nil, brandInvalid("the address is not on the public internet")
		}
		return nil, brandInvalid("the site did not respond. Check the address and try again in a moment")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, brandInvalid("the site could not be read (it answered %d)", resp.StatusCode)
	}
	root, err := html.Parse(io.LimitReader(resp.Body, brandPageMaxBytes))
	if err != nil {
		return nil, brandInvalid("the page could not be read")
	}
	return parseBrandPage(root, resp.Request.URL), nil
}

// parseBrandPage reads the brand a page declares in its metadata and finds
// the logo in its header.
func parseBrandPage(root *html.Node, base *url.URL) *brandPage {
	page := &brandPage{finalURL: base}
	var title, ogSiteName, appName, iconHref, appleIconHref string
	var headerLogo string
	var walk func(n *html.Node, inHeader bool)
	walk = func(n *html.Node, inHeader bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if title == "" && n.FirstChild != nil {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				content := strings.TrimSpace(htmlAttr(n, "content"))
				switch strings.ToLower(htmlAttr(n, "property") + htmlAttr(n, "name")) {
				case "og:site_name":
					ogSiteName = content
				case "application-name":
					appName = content
				case "theme-color":
					if page.themeColor == "" {
						page.themeColor = normalizeBrandColor(content)
					}
				}
			case "link":
				rel := " " + strings.ToLower(htmlAttr(n, "rel")) + " "
				href := strings.TrimSpace(htmlAttr(n, "href"))
				switch {
				case href == "":
				case strings.Contains(rel, " apple-touch-icon "):
					appleIconHref = href
				case strings.Contains(rel, " icon ") && iconHref == "":
					iconHref = href
				}
			case "header", "nav":
				inHeader = true
			case "img":
				// Only the page header counts. Further down, an image named
				// "logo" is as likely a customer's logo as the site's own, and
				// a wrong logo is worse than none.
				if src := brandLogoSource(n); src != "" && inHeader && headerLogo == "" {
					headerLogo = src
				}
			}
			if !inHeader && brandHeaderLike(n) {
				inHeader = true
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inHeader)
		}
	}
	walk(root, false)

	page.siteName = firstNonEmpty(ogSiteName, appName, brandNameFromTitle(title))
	// A PNG touch icon is sharper than a 16-pixel favicon.
	page.faviconURL = resolveBrandURL(base, firstNonEmpty(appleIconHref, iconHref))
	page.logoURL = resolveBrandURL(base, headerLogo)
	return page
}

// brandLogoSource returns the address of an image that calls itself a logo.
func brandLogoSource(img *html.Node) string {
	src := strings.TrimSpace(htmlAttr(img, "src"))
	if src == "" || strings.HasPrefix(src, "data:") {
		return ""
	}
	hints := strings.ToLower(htmlAttr(img, "alt") + " " + htmlAttr(img, "class") + " " + htmlAttr(img, "id") + " " + path.Base(src))
	if !strings.Contains(hints, "logo") {
		return ""
	}
	return src
}

// brandHeaderLike reports whether an element is a page header by its name.
func brandHeaderLike(n *html.Node) bool {
	marker := strings.ToLower(htmlAttr(n, "class") + " " + htmlAttr(n, "id") + " " + htmlAttr(n, "role"))
	return strings.Contains(marker, "header") || strings.Contains(marker, "navbar") || strings.Contains(marker, "banner")
}

// brandNameFromTitle takes the site name from a page title such as
// "Pricing | Acme" or "Acme - Test management".
func brandNameFromTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	parts := regexp.MustCompile(`\s+[|\-–—·:]\s+`).Split(title, -1)
	shortest := strings.TrimSpace(parts[0])
	for _, part := range parts[1:] {
		if part = strings.TrimSpace(part); part != "" && len(part) < len(shortest) {
			shortest = part
		}
	}
	return brandNameFromHelpCenter(shortest)
}

func resolveBrandURL(base *url.URL, ref string) string {
	if ref == "" || base == nil {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "https" && resolved.Scheme != "http" {
		return ""
	}
	return resolved.String()
}

// HelpcenterBrandApplyResult reports what applying a proposal did.
type HelpcenterBrandApplyResult struct {
	Config *model.DocsHelpcenterConfig
	// Notes carries the proposal's notes plus any image that could not be
	// copied.
	Notes []string
}

// ApplyBrandProposal saves a proposal as the workspace's help center
// branding. Each image is copied into Helpin's storage first; one that cannot
// be copied is skipped and reported rather than left pointing at the source
// site. Fields the proposal leaves empty keep their current value.
func (s *DocsHelpcenterService) ApplyBrandProposal(ctx context.Context, workspaceID string, proposal *model.HelpcenterBrandProposal) (*HelpcenterBrandApplyResult, error) {
	if proposal == nil {
		return nil, fmt.Errorf("brand proposal is required")
	}
	result := &HelpcenterBrandApplyResult{Notes: append([]string(nil), proposal.Notes...)}
	req := model.UpdateDocsHelpcenterConfigRequest{}
	copyAsset := func(assetType, label, source string) *string {
		if source == "" {
			return nil
		}
		stored, err := s.UploadAssetFromURL(ctx, workspaceID, assetType, source)
		if err != nil {
			slog.WarnContext(ctx, "brand import: asset not copied", "workspace_id", workspaceID, "asset", assetType, "error", err)
			result.Notes = append(result.Notes, fmt.Sprintf("The %s could not be copied. Upload it in the branding settings.", label))
			return nil
		}
		return &stored
	}
	req.BrandLogoURL = copyAsset("logo", "logo", proposal.LogoURL)
	req.BrandLogoDarkURL = copyAsset("logo_dark", "dark-background logo", proposal.LogoDarkURL)
	req.FaviconURL = copyAsset("favicon", "favicon", proposal.FaviconURL)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if proposal.BrandName != "" {
		req.BrandName = &proposal.BrandName
	}
	if proposal.BrandColor != "" {
		req.BrandColor = &proposal.BrandColor
	}
	// A dark theme is only right while the dark-background logo is the one
	// that exists.
	if proposal.ThemeMode != "" && (proposal.ThemeMode != "dark" || req.BrandLogoDarkURL != nil) {
		req.ThemeMode = &proposal.ThemeMode
	}
	if len(proposal.HeaderLinks) > 0 {
		encoded, err := json.Marshal(proposal.HeaderLinks)
		if err != nil {
			return nil, fmt.Errorf("encode header links: %w", err)
		}
		req.HeaderLinks = encoded
	}
	existing, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(proposal.SocialLinks) > 0 {
		footer := model.HelpcenterFooterConfig{}
		if existing != nil && len(existing.FooterConfig) > 0 {
			if err := json.Unmarshal(existing.FooterConfig, &footer); err != nil {
				footer = model.HelpcenterFooterConfig{}
			}
		}
		footer.SocialLinks = proposal.SocialLinks
		encoded, err := json.Marshal(footer)
		if err != nil {
			return nil, fmt.Errorf("encode footer: %w", err)
		}
		req.FooterConfig = encoded
	}
	if proposal.HeroTitle != "" || proposal.HeroSubtitle != "" {
		homepage := model.HelpcenterHomepageConfig{}
		if existing != nil && len(existing.HomepageConfig) > 0 {
			if err := json.Unmarshal(existing.HomepageConfig, &homepage); err != nil {
				homepage = model.HelpcenterHomepageConfig{}
			}
		}
		if proposal.HeroTitle != "" {
			homepage.HeroTitle = proposal.HeroTitle
		}
		if proposal.HeroSubtitle != "" {
			homepage.HeroSubtitle = proposal.HeroSubtitle
		}
		if homepage.FeaturedCards == nil {
			homepage.FeaturedCards = []model.HomepageFeaturedCard{}
		}
		encoded, err := json.Marshal(homepage)
		if err != nil {
			return nil, fmt.Errorf("encode homepage: %w", err)
		}
		req.HomepageConfig = encoded
	}
	cfg, err := s.UpsertConfig(ctx, workspaceID, req)
	if err != nil {
		return nil, err
	}
	result.Config = cfg
	return result, nil
}
