// Package publichelp reads public help centers and documentation sites the way
// a visitor's browser does: no access token, only what the site publishes.
// Each platform has a Reader in a subpackage; this package holds what they
// share: the neutral shape of a help center, fetching, and detection.
package publichelp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// Errors a public help center read can end in. Each one is a condition the
// person who entered the address can act on.
var (
	// ErrNotHelpCenter means the address is not a help center a reader knows.
	ErrNotHelpCenter = errors.New("the address is not a supported help center")
	// ErrRestricted means the help center requires sign-in.
	ErrRestricted = errors.New("the help center requires sign-in")
	// ErrChallenged means the site answered with a bot challenge.
	ErrChallenged = errors.New("the help center blocks automated reading")
	// ErrUnreachable means the site did not answer usefully.
	ErrUnreachable = errors.New("the help center could not be reached")
	// ErrInvalidURL means the address is not a usable web address.
	ErrInvalidURL = errors.New("the help center address is not valid")
	// ErrNotFound means the page no longer exists.
	ErrNotFound = errors.New("not found")
)

// PlatformError is a failure on a help center whose platform is known, so the
// caller can say what that platform needs.
type PlatformError struct {
	Platform string
	Err      error
}

func (e *PlatformError) Error() string { return e.Platform + ": " + e.Err.Error() }

// Unwrap returns the underlying condition.
func (e *PlatformError) Unwrap() error { return e.Err }

// ArticleRef is an article as a help center lists it.
type ArticleRef struct {
	// ID identifies the article within its help center and stays the same
	// between reads; it is the checkpoint of an import.
	ID      string
	Title   string
	URL     string
	Summary string
}

// Collection is a group of articles with its subgroups, in the order the help
// center shows them.
type Collection struct {
	ID          string
	Name        string
	Slug        string
	Description string
	// URL is the collection's own page, or "" when it has none.
	URL      string
	Articles []ArticleRef
	Children []Collection
}

// Article is one article's content as HTML with absolute links and images.
type Article struct {
	ID          string
	Title       string
	Description string
	URL         string
	HTML        string
}

// LinkKind says what a link into a help center points at.
type LinkKind string

// Link targets inside a help center.
const (
	LinkArticle    LinkKind = "article"
	LinkCollection LinkKind = "collection"
)

// LinkTarget is a link that points at an article or collection of a help
// center.
type LinkTarget struct {
	Kind LinkKind
	ID   string
	// Fragment is the "#section" part without "#".
	Fragment string
	// Certain is false when the link only might belong to this help center.
	Certain bool
}

// Site is one language of a public help center.
type Site struct {
	// Platform names the software behind the help center, such as "zendesk".
	Platform string
	// BaseURL is the home address; a reader's Load accepts it.
	BaseURL     string
	Name        string
	Locale      string
	Collections []Collection

	// Hosts are further hosts that serve this help center, besides BaseURL's.
	Hosts []string
	// MatchLink recognizes a link on one of the site's hosts by its shape, for
	// platforms whose addresses carry the ID. Links it does not recognize are
	// looked up by address among the listed articles and collections.
	MatchLink func(link *url.URL) (LinkKind, string, bool)
	// Resolve replaces the whole link lookup when set.
	Resolve func(href string) (LinkTarget, bool)
	// ArticleURL gives the address of an article known only by its ID, for
	// platforms where an article can be public without being listed. Nil when
	// every public article is listed.
	ArticleURL func(id string) string

	indexOnce sync.Once
	index     map[string]LinkTarget
}

// LinkTarget reports whether href points at an article or collection of this
// help center. Relative links never do: readers return absolute links, so a
// relative one was already rewritten.
func (s *Site) LinkTarget(href string) (LinkTarget, bool) {
	if s.Resolve != nil {
		return s.Resolve(href)
	}
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return LinkTarget{}, false
	}
	if !s.ownsHost(parsed.Hostname()) {
		return LinkTarget{}, false
	}
	if s.MatchLink != nil {
		if kind, id, ok := s.MatchLink(parsed); ok {
			return LinkTarget{Kind: kind, ID: id, Fragment: parsed.Fragment, Certain: true}, true
		}
	}
	s.indexOnce.Do(s.buildIndex)
	target, ok := s.index[LinkKey(parsed)]
	if !ok {
		return LinkTarget{}, false
	}
	target.Fragment = parsed.Fragment
	return target, true
}

func (s *Site) ownsHost(host string) bool {
	host = bareHost(host)
	if base, err := url.Parse(s.BaseURL); err == nil && bareHost(base.Hostname()) == host {
		return true
	}
	for _, other := range s.Hosts {
		if bareHost(other) == host {
			return true
		}
	}
	return false
}

func (s *Site) buildIndex() {
	s.index = map[string]LinkTarget{}
	var walk func(collections []Collection)
	walk = func(collections []Collection) {
		for _, collection := range collections {
			s.indexURL(collection.URL, LinkTarget{Kind: LinkCollection, ID: collection.ID, Certain: true})
			for _, article := range collection.Articles {
				s.indexURL(article.URL, LinkTarget{Kind: LinkArticle, ID: article.ID, Certain: true})
			}
			walk(collection.Children)
		}
	}
	walk(s.Collections)
}

func (s *Site) indexURL(raw string, target LinkTarget) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return
	}
	if key := LinkKey(parsed); s.index[key].ID == "" {
		s.index[key] = target
	}
}

// LinkKey is the form two addresses of the same page share: the path in
// lower case without a trailing slash or a ".md" or ".html" ending.
func LinkKey(link *url.URL) string {
	path := strings.ToLower(strings.TrimRight(link.Path, "/"))
	for _, ending := range []string{".md", ".html"} {
		path = strings.TrimSuffix(path, ending)
	}
	return path
}

func bareHost(host string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
}

// Reader reads the public help centers of one platform.
type Reader interface {
	// Key names the reader. It marks the import jobs and article checkpoints
	// of the help centers it reads, so it never changes.
	Key() string
	// Probe decides from a help center's home page, with at most a request or
	// two more, whether this reader can read it. It returns the site without
	// collections, or ErrNotHelpCenter.
	Probe(ctx context.Context, home *Page) (*Site, error)
	// Load reads the help center at a home address Probe returned: every
	// collection with its article list. progress, when set, is called after
	// each request, one call at a time, so a long read can report that it is
	// alive.
	Load(ctx context.Context, homeURL string, progress func()) (*Site, error)
	// Article reads one article from the address its listing gave.
	Article(ctx context.Context, articleURL string) (*Article, error)
}

// Serial returns progress wrapped so that calls from several goroutines run
// one at a time. Readers that fetch pages side by side pass their progress
// callback through it, so callers never need a lock. Nil stays nil.
func Serial(progress func()) func() {
	if progress == nil {
		return nil
	}
	var mu sync.Mutex
	return func() {
		mu.Lock()
		defer mu.Unlock()
		progress()
	}
}

// NormalizeURL turns what someone pasted into an address to fetch: it adds a
// missing scheme and drops the query and fragment. Whether the host may be
// reached at all is decided when the fetcher connects.
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: enter the help center address", ErrInvalidURL)
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("%w: check the address and try again", ErrInvalidURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("%w: the address must start with https://", ErrInvalidURL)
	}
	parsed.RawQuery, parsed.Fragment, parsed.RawFragment = "", "", ""
	return parsed.String(), nil
}

// Detect finds the reader for the help center at rawURL. The readers are
// asked in order, so the specific ones come before the general ones.
func Detect(ctx context.Context, fetcher *Fetcher, readers []Reader, rawURL string) (Reader, *Site, error) {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	home, err := fetcher.Open(ctx, normalized)
	if err != nil {
		return nil, nil, err
	}
	for _, reader := range readers {
		site, err := reader.Probe(ctx, home)
		if err == nil {
			return reader, site, nil
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		// A reader that knows the platform but cannot read it ends the search:
		// its error says what the help center needs.
		var known *PlatformError
		if errors.As(err, &known) {
			return nil, nil, err
		}
	}
	if err := home.Err(); err != nil {
		return nil, nil, err
	}
	return nil, nil, ErrNotHelpCenter
}
