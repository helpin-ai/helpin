package intercomtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// PublicArticle is one article of a fake public help center.
type PublicArticle struct {
	ID          string
	Slug        string
	Title       string
	Description string
	// Blocks is the article body in Intercom's published block format.
	Blocks []map[string]any
	// Status, when set, is answered instead of the article.
	Status int
	// DataID, when set, is published as the article's ID in its data instead
	// of the number in its address.
	DataID string
}

// PublicCollection is one collection of a fake public help center.
type PublicCollection struct {
	ID          string
	Slug        string
	Name        string
	Description string
	// ArticleIDs lists the collection's articles in display order.
	ArticleIDs []string
	Children   []PublicCollection
}

// PublicSite is a fake public Intercom help center: a home page and
// collection pages that embed their data the way Intercom does, and each
// article's block data at "<article address>.json".
type PublicSite struct {
	*httptest.Server
	mu sync.Mutex

	Name        string
	Locale      string
	Color       string
	Headline    string
	Logo        string
	Favicon     string
	Collections []PublicCollection
	Articles    map[string]PublicArticle
	// Images maps a path to the content type and body it serves.
	Images map[string]PublicImage
	// Restricted marks the whole help center as requiring sign-in.
	Restricted bool
	// FailNext answers the next N requests with 503.
	FailNext int
	// NotHelpCenter serves plain pages without embedded data.
	NotHelpCenter bool
	Requests      []string
}

// PublicImage is a file the fake serves.
type PublicImage struct {
	ContentType string
	Body        []byte
}

// NewPublicSite starts an empty fake help center in English.
func NewPublicSite() *PublicSite {
	s := &PublicSite{Name: "Acme Help Center", Locale: "en", Articles: map[string]PublicArticle{}, Images: map[string]PublicImage{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// Home is the address of the help center's home page.
func (s *PublicSite) Home() string { return s.URL + "/" + s.Locale + "/" }

// ArticleURL is the public address of an article.
func (s *PublicSite) ArticleURL(id string) string {
	article := s.Articles[id]
	if article.Slug == "" {
		return fmt.Sprintf("%s/%s/articles/%s", s.URL, s.Locale, id)
	}
	return fmt.Sprintf("%s/%s/articles/%s-%s", s.URL, s.Locale, id, article.Slug)
}

// CollectionURL is the public address of a collection.
func (s *PublicSite) CollectionURL(c PublicCollection) string {
	return fmt.Sprintf("%s/%s/collections/%s-%s", s.URL, s.Locale, c.ID, c.Slug)
}

// RequestCount reports how many requests matched the path fragment.
func (s *PublicSite) RequestCount(fragment string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, request := range s.Requests {
		if strings.Contains(request, fragment) {
			count++
		}
	}
	return count
}

func (s *PublicSite) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.Requests = append(s.Requests, r.URL.Path)
	fail := s.FailNext > 0
	if fail {
		s.FailNext--
	}
	s.mu.Unlock()
	if fail {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	if image, ok := s.Images[r.URL.Path]; ok {
		if image.ContentType != "" {
			w.Header().Set("Content-Type", image.ContentType)
		}
		_, _ = w.Write(image.Body)
		return
	}
	prefix := "/" + s.Locale
	switch {
	case r.URL.Path == "/":
		http.Redirect(w, r, prefix+"/", http.StatusFound)
	case r.URL.Path == prefix+"/" || r.URL.Path == prefix:
		s.page(w, map[string]any{"home": map[string]any{
			"allContentRestricted": s.Restricted,
			"collections":          s.homeCollections(),
		}})
	case strings.HasPrefix(r.URL.Path, prefix+"/collections/"):
		id := leadingID(strings.TrimPrefix(r.URL.Path, prefix+"/collections/"))
		for _, collection := range s.Collections {
			if collection.ID == id {
				s.page(w, map[string]any{"collection": s.collectionNode(collection)})
				return
			}
		}
		http.NotFound(w, r)
	case strings.HasPrefix(r.URL.Path, prefix+"/articles/") && strings.HasSuffix(r.URL.Path, ".json"):
		s.article(w, r, leadingID(strings.TrimPrefix(r.URL.Path, prefix+"/articles/")))
	default:
		http.NotFound(w, r)
	}
}

func (s *PublicSite) article(w http.ResponseWriter, r *http.Request, id string) {
	article, ok := s.Articles[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if article.Status != 0 {
		http.Error(w, "unavailable", article.Status)
		return
	}
	blocks := article.Blocks
	if blocks == nil {
		blocks = []map[string]any{}
	}
	dataID := id
	if article.DataID != "" {
		dataID = article.DataID
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "9" + id, "articleId": dataID, "title": article.Title, "description": article.Description,
		"canonicalUrl": s.ArticleURL(id), "blocks": blocks,
	})
}

func (s *PublicSite) homeCollections() []map[string]any {
	out := make([]map[string]any, 0, len(s.Collections))
	for _, collection := range s.Collections {
		out = append(out, map[string]any{"id": collection.ID, "name": collection.Name, "slug": collection.Slug, "url": s.CollectionURL(collection)})
	}
	return out
}

func (s *PublicSite) collectionNode(c PublicCollection) map[string]any {
	summaries := make([]map[string]any, 0, len(c.ArticleIDs))
	for _, id := range c.ArticleIDs {
		article := s.Articles[id]
		summaries = append(summaries, map[string]any{"id": "9" + id, "title": article.Title, "summary": article.Description, "url": s.ArticleURL(id)})
	}
	children := make([]map[string]any, 0, len(c.Children))
	for _, child := range c.Children {
		children = append(children, s.collectionNode(child))
	}
	return map[string]any{
		"id": c.ID, "name": c.Name, "slug": c.Slug, "description": c.Description, "url": s.CollectionURL(c),
		"articleSummaries": summaries, "subcollections": children,
	}
}

// page writes an HTML page that embeds pageProps as Intercom's pages do.
func (s *PublicSite) page(w http.ResponseWriter, pageProps map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if s.NotHelpCenter {
		_, _ = w.Write([]byte("<html><body><h1>Welcome</h1></body></html>"))
		return
	}
	pageProps["theme"] = map[string]any{
		"color": s.Color, "siteName": s.Name, "headline": s.Headline, "locale": s.Locale,
		"logo": s.Logo, "favicon": s.Favicon, "homeUrl": "https://app.acme.example",
		"urlPrefixForDefaultDomain": "https://intercom.help/acme",
	}
	pageProps["helpCenterSite"] = map[string]any{
		"url":         s.URL,
		"headerLinks": []map[string]any{{"title": "API Reference", "url": "https://developers.acme.example"}},
		"footerLinks": map[string]any{"socialLinks": []map[string]any{{"provider": "github", "url": "https://github.com/acme"}}},
	}
	data, err := json.Marshal(map[string]any{"props": map[string]any{"pageProps": pageProps}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><title>%s</title></head><body><div id="__next"></div>`+
		`<script id="__NEXT_DATA__" type="application/json">%s</script></body></html>`, s.Name, data)
}

// leadingID returns the number an address segment such as "12-slug.json"
// starts with.
func leadingID(segment string) string {
	end := 0
	for end < len(segment) && segment[end] >= '0' && segment[end] <= '9' {
		end++
	}
	return segment[:end]
}
