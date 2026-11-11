package intercom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/helpin-ai/helpin/server/internal/netguard"
)

// Errors a public help center read can end in. Each one is a condition the
// person who entered the address can act on.
var (
	// ErrNotPublicHelpCenter means the address did not serve an Intercom help
	// center page.
	ErrNotPublicHelpCenter = errors.New("the address is not a public Intercom help center")
	// ErrPublicHelpCenterRestricted means the help center requires sign-in.
	ErrPublicHelpCenterRestricted = errors.New("the help center requires sign-in")
	// ErrPublicHelpCenterUnreachable means the site did not answer usefully.
	ErrPublicHelpCenterUnreachable = errors.New("the help center could not be reached")
	// ErrInvalidPublicURL means the address is not a usable web address.
	ErrInvalidPublicURL = errors.New("the help center address is not valid")
)

const (
	publicUserAgent   = "Mozilla/5.0 (compatible; HelpinImporter/1.0; +https://helpin.ai)"
	publicMaxBody     = 16 << 20
	publicMaxAttempts = 3
	publicMaxRedirect = 5
	// intercomDefaultHost serves help centers that have no custom domain, one
	// per first path segment.
	intercomDefaultHost = "intercom.help"
	// publicGuessedIDMinDigits is the fewest digits a number needs to count
	// as a help center ID on a host that is only guessed to be the help center.
	publicGuessedIDMinDigits = 5
)

var (
	_nextDataPattern = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	// A help center link ends in "/articles/<number>-<slug>"; the number may
	// stand alone.
	_publicArticlePath     = regexp.MustCompile(`/articles/(\d+)(?:-[^/]*)?/?$`)
	_publicCollectionPath  = regexp.MustCompile(`/collections/(\d+)(?:-[^/]*)?/?$`)
	_publicLeadingDigits   = regexp.MustCompile(`^(\d+)`)
	_publicContentSegments = []string{"/collections/", "/articles/"}
)

// PublicClient reads an Intercom help center the way a visitor's browser
// does: no access token, only the pages and article data the site publishes.
type PublicClient struct {
	http *http.Client
	// sleep waits between retries; tests replace it.
	sleep func(context.Context, time.Duration) error
}

// NewPublicClient returns a client that reaches only public addresses, so an
// address typed into an import form cannot make the server read internal
// hosts, including through a redirect.
func NewPublicClient() *PublicClient {
	return NewPublicClientWithHTTP(&http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           netguard.DialContext(false),
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   4,
		},
	})
}

// NewPublicClientWithHTTP builds a client on a caller-supplied transport.
// Tests use it to reach a local server.
func NewPublicClientWithHTTP(client *http.Client) *PublicClient {
	copied := *client
	copied.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= publicMaxRedirect {
			return fmt.Errorf("stopped after %d redirects", publicMaxRedirect)
		}
		return nil
	}
	return &PublicClient{http: &copied, sleep: sleepContext}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// PublicLink is a titled link from a help center's header or footer.
type PublicLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// PublicSocialLink is a social profile from a help center's footer.
type PublicSocialLink struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

// PublicTheme is the branding a help center publishes with every page.
type PublicTheme struct {
	// Color is the brand color as six hex digits without "#".
	Color       string
	SiteName    string
	Headline    string
	Subheadline string
	// LogoURL is the header logo. It is drawn for the colored header, so it is
	// usually the light-on-dark version of the mark.
	LogoURL     string
	FaviconURL  string
	HomeURL     string
	HeaderLinks []PublicLink
	SocialLinks []PublicSocialLink
}

// PublicArticleSummary is an article as a collection page lists it.
type PublicArticleSummary struct {
	// ID is the number in the article's address, which links use.
	ID      string
	Title   string
	URL     string
	Summary string
}

// PublicCollection is a collection with its articles and subcollections in
// the order the help center shows them.
type PublicCollection struct {
	ID          string
	Name        string
	Slug        string
	Description string
	URL         string
	Articles    []PublicArticleSummary
	Children    []PublicCollection
}

// PublicHelpCenter is one locale of a public Intercom help center.
type PublicHelpCenter struct {
	// BaseURL is the locale's home address and always ends in "/".
	BaseURL     string
	Name        string
	Locale      string
	Theme       PublicTheme
	Collections []PublicCollection

	hosts         map[string]bool
	registrable   string
	defaultPrefix string
}

// PublicArticle is one article's content.
type PublicArticle struct {
	ID          string
	Title       string
	Description string
	URL         string
	Blocks      []Block
}

// Block is one node of an article body in Intercom's published block format.
type Block struct {
	Type        string         `json:"type"`
	Text        string         `json:"text"`
	IDAttribute string         `json:"idAttribute"`
	URL         string         `json:"url"`
	LinkURL     string         `json:"linkUrl"`
	Align       string         `json:"align"`
	Width       flexibleNumber `json:"width"`
	Height      flexibleNumber `json:"height"`
	ShownWidth  flexibleNumber `json:"displayWidth"`
	Provider    string         `json:"provider"`
	ID          flexibleString `json:"id"`
	Language    string         `json:"language"`
	Content     []Block        `json:"content"`
	Items       []BlockItem    `json:"items"`
	Rows        []BlockRow     `json:"rows"`
	Style       BlockStyle     `json:"style"`
	Summary     BlockSummary   `json:"summary"`
}

// BlockItem is one entry of a list block.
type BlockItem struct {
	Content []Block `json:"content"`
}

// BlockRow is one row of a table block.
type BlockRow struct {
	Cells []BlockItem `json:"cells"`
}

// BlockStyle carries the colors of a callout block.
type BlockStyle struct {
	BackgroundColor string `json:"backgroundColor"`
}

// BlockSummary is the always-visible line of a collapsible block. Intercom
// publishes it as an object and, on older articles, as a bare string.
type BlockSummary struct {
	Text string
}

// UnmarshalJSON accepts both published shapes of a summary.
func (s *BlockSummary) UnmarshalJSON(data []byte) error {
	var text string
	if json.Unmarshal(data, &text) == nil {
		s.Text = text
		return nil
	}
	var object struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		// An unexpected shape costs the summary, never the article.
		return nil
	}
	s.Text = object.Text
	return nil
}

// flexibleNumber reads a number that Intercom publishes as a number or as a
// quoted string, and is zero for anything else.
type flexibleNumber float64

// UnmarshalJSON implements json.Unmarshaler.
func (n *flexibleNumber) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if value, err := strconv.ParseFloat(trimmed, 64); err == nil {
		*n = flexibleNumber(value)
	}
	return nil
}

// flexibleString reads an identifier that may be published as a string or as
// a number.
type flexibleString string

// UnmarshalJSON implements json.Unmarshaler.
func (s *flexibleString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		*s = flexibleString(text)
		return nil
	}
	*s = flexibleString(trimmed)
	return nil
}

// DisplayWidth is the width in pixels the article shows an image at, or 0.
func (b Block) DisplayWidth() int { return int(b.ShownWidth) }

// NormalizePublicURL turns what someone pasted into the address of a help
// center home: it adds a missing scheme, drops query and fragment, and steps
// up from an article or collection page. Whether the host may be reached at
// all is decided when the client connects, not here.
func NormalizePublicURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: enter the help center address", ErrInvalidPublicURL)
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("%w: check the address and try again", ErrInvalidPublicURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("%w: the address must start with https://", ErrInvalidPublicURL)
	}
	parsed.RawQuery, parsed.Fragment, parsed.RawFragment = "", "", ""
	parsed.Path = helpCenterHomePath(parsed.Path)
	return parsed.String(), nil
}

// helpCenterHomePath cuts an article or collection path back to the locale
// home it sits under and ends it in "/".
func helpCenterHomePath(p string) string {
	for _, segment := range _publicContentSegments {
		if i := strings.Index(p, segment); i >= 0 {
			p = p[:i]
		}
	}
	return strings.TrimRight(p, "/") + "/"
}

// LoadHelpCenter reads the structure of the help center at rawURL: its
// branding and every collection with its article list. progress, when set, is
// called after each page so a long read can report that it is alive.
func (c *PublicClient) LoadHelpCenter(ctx context.Context, rawURL string, progress func()) (*PublicHelpCenter, error) {
	normalized, err := NormalizePublicURL(rawURL)
	if err != nil {
		return nil, err
	}
	props, final, err := c.pageProps(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress()
	}
	if props.Home == nil {
		return nil, ErrNotPublicHelpCenter
	}
	if props.Home.AllContentRestricted {
		return nil, ErrPublicHelpCenterRestricted
	}
	site := newPublicHelpCenter(final, props)
	for _, entry := range props.Home.Collections {
		if strings.TrimSpace(entry.URL) == "" {
			continue
		}
		page, _, err := c.pageProps(ctx, entry.URL)
		if err != nil {
			return nil, fmt.Errorf("read collection %q: %w", entry.Name, err)
		}
		if progress != nil {
			progress()
		}
		if page.Collection == nil {
			continue
		}
		site.Collections = append(site.Collections, page.Collection.toPublic(0))
	}
	return site, nil
}

// LoadTheme reads only the branding of the help center at rawURL.
func (c *PublicClient) LoadTheme(ctx context.Context, rawURL string) (*PublicHelpCenter, error) {
	normalized, err := NormalizePublicURL(rawURL)
	if err != nil {
		return nil, err
	}
	props, final, err := c.pageProps(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if props.Home == nil {
		return nil, ErrNotPublicHelpCenter
	}
	return newPublicHelpCenter(final, props), nil
}

// Article reads one article's content from its public address.
func (c *PublicClient) Article(ctx context.Context, articleURL string) (*PublicArticle, error) {
	parsed, err := url.Parse(strings.TrimSpace(articleURL))
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: article address %q", ErrInvalidPublicURL, articleURL)
	}
	parsed.RawQuery, parsed.Fragment, parsed.RawFragment = "", "", ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	canonical := parsed.String()
	body, _, err := c.get(ctx, canonical+".json")
	if err != nil {
		return nil, err
	}
	var raw struct {
		ArticleID   flexibleString `json:"articleId"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Canonical   string         `json:"canonicalUrl"`
		Blocks      *[]Block       `json:"blocks"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Blocks == nil {
		return nil, fmt.Errorf("%w: article %q has no readable content", ErrNotPublicHelpCenter, canonical)
	}
	id := string(raw.ArticleID)
	if id == "" {
		id = PublicSourceID(canonical)
	}
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		title = "Untitled"
	}
	// The address an article was reached at may be the bare number; the one
	// it publishes carries its slug.
	if published := strings.TrimSpace(raw.Canonical); published != "" && PublicSourceID(published) == PublicSourceID(canonical) {
		canonical = published
	}
	return &PublicArticle{ID: id, Title: title, Description: strings.TrimSpace(raw.Description), URL: canonical, Blocks: *raw.Blocks}, nil
}

// ArticleURL is the address of the article with the given ID in this help
// center. Intercom resolves an article by the number alone.
func (h *PublicHelpCenter) ArticleURL(id string) string {
	return h.BaseURL + "articles/" + id
}

// PublicLinkKind says what a link into a help center points at.
type PublicLinkKind string

// Link targets inside a help center.
const (
	PublicLinkArticle    PublicLinkKind = "article"
	PublicLinkCollection PublicLinkKind = "collection"
)

// PublicLinkTarget is a link that points at an article or collection of a
// help center.
type PublicLinkTarget struct {
	Kind PublicLinkKind
	ID   string
	// Fragment is the "#section" part without "#".
	Fragment string
	// Certain is false when the link only might belong to this help center:
	// it sits on Intercom's shared host under an unknown name, or on another
	// host of the same company, which may be a blog or the product itself.
	// Such a link is worth trying, but is not this help center's broken link
	// when it leads nowhere.
	Certain bool
}

// LinkTarget reports whether href points at an article or collection of this
// help center, on any of the hosts it is or was served from.
func (h *PublicHelpCenter) LinkTarget(href string) (PublicLinkTarget, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return PublicLinkTarget{}, false
	}
	parsed, err := url.Parse(href)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		// Relative links are already-rewritten Helpin addresses, not source links.
		return PublicLinkTarget{}, false
	}
	certain, ok := h.ownsHost(parsed)
	if !ok {
		return PublicLinkTarget{}, false
	}
	target := PublicLinkTarget{Fragment: parsed.Fragment, Certain: certain}
	if match := _publicArticlePath.FindStringSubmatch(parsed.Path); match != nil {
		target.Kind, target.ID = PublicLinkArticle, match[1]
	} else if match := _publicCollectionPath.FindStringSubmatch(parsed.Path); match != nil {
		target.Kind, target.ID = PublicLinkCollection, match[1]
	} else {
		return PublicLinkTarget{}, false
	}
	// On a guessed host, a short number is a year or a page number far more
	// often than an Intercom ID, which has many digits.
	if !certain && len(target.ID) < publicGuessedIDMinDigits {
		return PublicLinkTarget{}, false
	}
	return target, true
}

// ownsHost reports whether a link's host serves this help center, and whether
// that is certain.
func (h *PublicHelpCenter) ownsHost(link *url.URL) (certain, ok bool) {
	host := strings.ToLower(link.Hostname())
	if host == intercomDefaultHost {
		if h.defaultPrefix != "" {
			owned := strings.HasPrefix(strings.ToLower(link.Path)+"/", h.defaultPrefix+"/")
			return owned, owned
		}
		// The help center's own name on the shared host is unknown, so the
		// link may be another company's.
		return false, true
	}
	if h.hosts[host] {
		return true, true
	}
	// A company that moved its help center (help.example.com to
	// docs.example.com) leaves links on the old name behind. The same domain
	// also serves the company's other sites, so this is only a guess.
	if h.registrable != "" && (host == h.registrable || strings.HasSuffix(host, "."+h.registrable)) {
		return false, true
	}
	return false, false
}

// PublicSourceID returns the number that leads the last path segment of an
// article or collection address, or the whole segment when it has none.
func PublicSourceID(rawURL string) string {
	segment := lastPathSegment(rawURL)
	if match := _publicLeadingDigits.FindString(segment); match != "" {
		return match
	}
	return segment
}

func lastPathSegment(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return segments[len(segments)-1]
}

type publicPageProps struct {
	Home *struct {
		AllContentRestricted bool `json:"allContentRestricted"`
		Collections          []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"collections"`
	} `json:"home"`
	Collection     *publicCollectionNode `json:"collection"`
	Theme          publicThemeProps      `json:"theme"`
	HelpCenterSite struct {
		CustomDomain string       `json:"customDomain"`
		URL          string       `json:"url"`
		HeaderLinks  []PublicLink `json:"headerLinks"`
		FooterLinks  struct {
			SocialLinks []PublicSocialLink `json:"socialLinks"`
		} `json:"footerLinks"`
	} `json:"helpCenterSite"`
}

type publicThemeProps struct {
	Color                     string `json:"color"`
	SiteName                  string `json:"siteName"`
	HelpCenterName            string `json:"helpCenterName"`
	Headline                  string `json:"headline"`
	Subheadline               string `json:"subheadline"`
	Logo                      string `json:"logo"`
	Favicon                   string `json:"favicon"`
	Locale                    string `json:"locale"`
	HomeURL                   string `json:"homeUrl"`
	CustomDomain              string `json:"customDomain"`
	URLPrefixForDefaultDomain string `json:"urlPrefixForDefaultDomain"`
}

type publicCollectionNode struct {
	ID               flexibleString `json:"id"`
	Name             string         `json:"name"`
	Slug             string         `json:"slug"`
	Description      string         `json:"description"`
	URL              string         `json:"url"`
	ArticleSummaries []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Summary string `json:"summary"`
	} `json:"articleSummaries"`
	Subcollections []publicCollectionNode `json:"subcollections"`
}

// publicCollectionMaxDepth stops a malformed page from recursing without end;
// Intercom nests collections three levels deep.
const publicCollectionMaxDepth = 6

func (n publicCollectionNode) toPublic(depth int) PublicCollection {
	collection := PublicCollection{
		ID:          string(n.ID),
		Name:        strings.TrimSpace(n.Name),
		Slug:        strings.TrimSpace(n.Slug),
		Description: strings.TrimSpace(n.Description),
		URL:         n.URL,
	}
	if collection.ID == "" {
		collection.ID = PublicSourceID(n.URL)
	}
	if collection.Name == "" {
		collection.Name = "Untitled"
	}
	for _, summary := range n.ArticleSummaries {
		if strings.TrimSpace(summary.URL) == "" {
			continue
		}
		title := strings.TrimSpace(summary.Title)
		if title == "" {
			title = "Untitled"
		}
		collection.Articles = append(collection.Articles, PublicArticleSummary{
			ID: PublicSourceID(summary.URL), Title: title, URL: summary.URL, Summary: strings.TrimSpace(summary.Summary),
		})
	}
	if depth >= publicCollectionMaxDepth {
		return collection
	}
	for _, child := range n.Subcollections {
		collection.Children = append(collection.Children, child.toPublic(depth+1))
	}
	return collection
}

func newPublicHelpCenter(final *url.URL, props *publicPageProps) *PublicHelpCenter {
	base := *final
	base.RawQuery, base.Fragment, base.RawFragment = "", "", ""
	base.Path = helpCenterHomePath(base.Path)
	name := strings.TrimSpace(props.Theme.SiteName)
	if name == "" {
		name = strings.TrimSpace(props.Theme.HelpCenterName)
	}
	site := &PublicHelpCenter{
		BaseURL: base.String(),
		Name:    name,
		Locale:  strings.TrimSpace(props.Theme.Locale),
		Theme: PublicTheme{
			Color:       strings.TrimPrefix(strings.TrimSpace(props.Theme.Color), "#"),
			SiteName:    name,
			Headline:    strings.TrimSpace(props.Theme.Headline),
			Subheadline: strings.TrimSpace(props.Theme.Subheadline),
			LogoURL:     strings.TrimSpace(props.Theme.Logo),
			FaviconURL:  strings.TrimSpace(props.Theme.Favicon),
			HomeURL:     strings.TrimSpace(props.Theme.HomeURL),
			HeaderLinks: props.HelpCenterSite.HeaderLinks,
			SocialLinks: props.HelpCenterSite.FooterLinks.SocialLinks,
		},
		hosts: map[string]bool{},
	}
	baseHost := strings.ToLower(base.Hostname())
	for _, host := range []string{baseHost, props.Theme.CustomDomain, props.HelpCenterSite.CustomDomain, hostOf(props.HelpCenterSite.URL)} {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" && host != intercomDefaultHost {
			site.hosts[host] = true
		}
	}
	if baseHost != intercomDefaultHost {
		if registrable, err := publicsuffix.EffectiveTLDPlusOne(baseHost); err == nil {
			site.registrable = registrable
		}
	}
	site.defaultPrefix = defaultDomainPrefix(props.Theme.URLPrefixForDefaultDomain, &base)
	return site
}

// defaultDomainPrefix returns the lowercased "/name" path this help center
// has on Intercom's shared host, or "" when it cannot be told.
func defaultDomainPrefix(published string, base *url.URL) string {
	if parsed, err := url.Parse(strings.TrimSpace(published)); err == nil && strings.EqualFold(parsed.Hostname(), intercomDefaultHost) {
		if first := firstPathSegment(parsed.Path); first != "" {
			return "/" + strings.ToLower(first)
		}
	}
	if strings.EqualFold(base.Hostname(), intercomDefaultHost) {
		if first := firstPathSegment(base.Path); first != "" {
			return "/" + strings.ToLower(first)
		}
	}
	return ""
}

func firstPathSegment(p string) string {
	return strings.Split(strings.Trim(p, "/"), "/")[0]
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// pageProps fetches a help center page and returns the data Intercom embeds
// in it, with the address the request ended at.
func (c *PublicClient) pageProps(ctx context.Context, pageURL string) (*publicPageProps, *url.URL, error) {
	body, final, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, nil, err
	}
	match := _nextDataPattern.FindSubmatch(body)
	if match == nil {
		return nil, nil, ErrNotPublicHelpCenter
	}
	var data struct {
		Props struct {
			PageProps *publicPageProps `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(match[1], &data); err != nil || data.Props.PageProps == nil {
		return nil, nil, ErrNotPublicHelpCenter
	}
	return data.Props.PageProps, final, nil
}

// get fetches a page, retrying brief failures, and returns the body with the
// address the request ended at after redirects.
func (c *PublicClient) get(ctx context.Context, target string) ([]byte, *url.URL, error) {
	var lastErr error
	for attempt := 1; attempt <= publicMaxAttempts; attempt++ {
		body, final, retryAfter, err := c.getOnce(ctx, target)
		if err == nil {
			return body, final, nil
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		lastErr = err
		if retryAfter < 0 || attempt == publicMaxAttempts {
			break
		}
		if err := c.sleep(ctx, retryAfter); err != nil {
			return nil, nil, err
		}
	}
	return nil, nil, lastErr
}

// getOnce performs one request. A negative retryAfter means the failure is
// final.
func (c *PublicClient) getOnce(ctx context.Context, target string) (body []byte, final *url.URL, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, -1, fmt.Errorf("%w: %q", ErrInvalidPublicURL, target)
	}
	req.Header.Set("User-Agent", publicUserAgent)
	req.Header.Set("Accept", "text/html,application/json;q=0.9")
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, netguard.ErrNonPublicAddress) {
			return nil, nil, -1, fmt.Errorf("%w: the address is not on the public internet", ErrInvalidPublicURL)
		}
		return nil, nil, 2 * time.Second, fmt.Errorf("%w: %s", ErrPublicHelpCenterUnreachable, hostOf(target))
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return nil, nil, -1, ErrNotFound
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, nil, -1, ErrPublicHelpCenterRestricted
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return nil, nil, publicRetryDelay(resp.Header.Get("Retry-After")),
			fmt.Errorf("%w: the site answered with status %d", ErrPublicHelpCenterUnreachable, resp.StatusCode)
	default:
		return nil, nil, -1, fmt.Errorf("%w: the site answered with status %d", ErrPublicHelpCenterUnreachable, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, publicMaxBody+1)
	body, err = io.ReadAll(limited)
	if err != nil {
		return nil, nil, 2 * time.Second, fmt.Errorf("%w: the page could not be read", ErrPublicHelpCenterUnreachable)
	}
	if len(body) > publicMaxBody {
		return nil, nil, -1, fmt.Errorf("%w: a page was larger than expected", ErrPublicHelpCenterUnreachable)
	}
	return bytes.TrimSpace(body), resp.Request.URL, 0, nil
}

// publicRetryDelay honors a Retry-After of up to 30 seconds and otherwise
// waits two.
func publicRetryDelay(header string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds > 0 {
		return time.Duration(min(seconds, 30)) * time.Second
	}
	return 2 * time.Second
}
