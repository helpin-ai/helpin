package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/publichelp"
	"github.com/helpin-ai/helpin/server/internal/publichelp/readers"
	"github.com/helpin-ai/helpin/server/internal/publichelp/zendesk"
)

// publicImportSourceSuffix ends the job source of every import that read a
// help center by its address: "intercom_public", "zendesk_public" and so on.
// The source also marks the redirects and article checkpoints of the import,
// so two platforms never share article identifiers.
const publicImportSourceSuffix = "_public"

// isPublicImportSource reports whether a job source is an import by address.
func isPublicImportSource(source string) bool {
	return strings.HasSuffix(source, publicImportSourceSuffix)
}

// publicSource reads the help centers of one platform for an import.
type publicSource interface {
	// source is the job source of this platform's imports.
	source() string
	load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error)
	read(ctx context.Context, articleURL string) (*publicContent, error)
}

// publicContent is an article as its help center published it.
type publicContent struct {
	ID          string
	Title       string
	Description string
	URL         string
	// convert turns the body into a document. copyImage stores an image and
	// returns its new address, or "" to keep the source address.
	convert func(copyImage func(source string) string) (docsimport.Node, []docsimport.Warning, error)
	// text is the body as plain text.
	text func() string
}

// publicCredentials sign the requests to a help center that refuses
// anonymous reads. Only Zendesk takes them.
type publicCredentials struct{ email, token string }

func (s *DocsImportService) publicFetcher() *publichelp.Fetcher {
	if s.publicFetch == nil {
		s.publicFetch = publichelp.NewFetcher()
	}
	return s.publicFetch
}

// publicReaders lists the readers for every platform but Intercom.
func (s *DocsImportService) publicReaders(credentials publicCredentials) []publichelp.Reader {
	all := readers.All(s.publicFetcher())
	if credentials.token == "" {
		return all
	}
	for i, reader := range all {
		if plain, ok := reader.(*zendesk.Reader); ok {
			all[i] = plain.WithToken(credentials.email, credentials.token)
		}
	}
	return all
}

// publicSourceFor returns the source that imports of the given job source
// read from, or nil when no reader has that name.
func (s *DocsImportService) publicSourceFor(jobSource string, credentials publicCredentials) publicSource {
	if jobSource == docsImportSourceIntercomPublic {
		return intercomPublicSource{client: s.publicIntercomClient()}
	}
	for _, reader := range s.publicReaders(credentials) {
		if reader.Key()+publicImportSourceSuffix == jobSource {
			return readerPublicSource{reader: reader, fetch: s.publicFetcher()}
		}
	}
	return nil
}

// detectPublicHelpCenter finds out which platform serves the help center at
// rawURL and returns the source to read it with and its home, with one or
// two requests. Intercom is asked first: it is the most common, and its help
// centers must not be mistaken for a documentation site.
func (s *DocsImportService) detectPublicHelpCenter(ctx context.Context, rawURL string, credentials publicCredentials) (publicSource, *publichelp.Site, error) {
	site, err := s.publicIntercomClient().LoadTheme(ctx, rawURL)
	if err == nil {
		return intercomPublicSource{client: s.publicIntercomClient()}, intercomPublicSite(site), nil
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	if errors.Is(err, intercom.ErrInvalidPublicURL) {
		return nil, nil, err
	}
	// The home address, with any article or collection path cut off, serves
	// the other readers as well.
	home, err := intercom.NormalizePublicURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	reader, found, err := publichelp.Detect(ctx, s.publicFetcher(), s.publicReaders(credentials), home)
	if err != nil {
		return nil, nil, err
	}
	return readerPublicSource{reader: reader, fetch: s.publicFetcher()}, found, nil
}

// intercomPublicSource reads Intercom help centers, whose articles are
// published as blocks rather than HTML.
type intercomPublicSource struct{ client *intercom.PublicClient }

func (intercomPublicSource) source() string { return docsImportSourceIntercomPublic }

func (i intercomPublicSource) load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	site, err := i.client.LoadHelpCenter(ctx, homeURL, progress)
	if err != nil {
		return nil, err
	}
	return intercomPublicSite(site), nil
}

func (i intercomPublicSource) read(ctx context.Context, articleURL string) (*publicContent, error) {
	article, err := i.client.Article(ctx, articleURL)
	if err != nil {
		return nil, err
	}
	return &publicContent{
		ID: article.ID, Title: article.Title, Description: article.Description, URL: article.URL,
		convert: func(copyImage func(string) string) (docsimport.Node, []docsimport.Warning, error) {
			converted := docsimport.ConvertIntercomBlocks(article.Blocks, docsimport.IntercomBlockOptions{CopyImage: copyImage})
			return converted.Doc, converted.Warnings, nil
		},
		text: func() string {
			var b strings.Builder
			writePublicBlocksText(&b, article.Blocks)
			return strings.TrimSpace(b.String())
		},
	}, nil
}

// intercomPublicSite puts an Intercom help center in the shape every
// platform shares. Links keep Intercom's own rules: its addresses carry the
// article number, and an article can be public without sitting in a collection.
func intercomPublicSite(site *intercom.PublicHelpCenter) *publichelp.Site {
	var convert func(collections []intercom.PublicCollection) []publichelp.Collection
	convert = func(collections []intercom.PublicCollection) []publichelp.Collection {
		out := make([]publichelp.Collection, 0, len(collections))
		for _, collection := range collections {
			articles := make([]publichelp.ArticleRef, 0, len(collection.Articles))
			for _, article := range collection.Articles {
				articles = append(articles, publichelp.ArticleRef{ID: article.ID, Title: article.Title, URL: article.URL, Summary: article.Summary})
			}
			out = append(out, publichelp.Collection{
				ID: collection.ID, Name: collection.Name, Slug: collection.Slug, Description: collection.Description,
				URL: collection.URL, Articles: articles, Children: convert(collection.Children),
			})
		}
		return out
	}
	return &publichelp.Site{
		Platform: model.PublicHelpCenterPlatformIntercom, BaseURL: site.BaseURL, Name: site.Name, Locale: site.Locale,
		Collections: convert(site.Collections),
		Resolve: func(href string) (publichelp.LinkTarget, bool) {
			target, ok := site.LinkTarget(href)
			if !ok {
				return publichelp.LinkTarget{}, false
			}
			kind := publichelp.LinkCollection
			if target.Kind == intercom.PublicLinkArticle {
				kind = publichelp.LinkArticle
			}
			return publichelp.LinkTarget{Kind: kind, ID: target.ID, Fragment: target.Fragment, Certain: target.Certain}, true
		},
		ArticleURL: site.ArticleURL,
	}
}

// readerPublicSource reads a platform that publishes its articles as HTML.
type readerPublicSource struct {
	reader publichelp.Reader
	fetch  *publichelp.Fetcher
}

// moved returns the address a page of the help center now has when the given
// one redirects, or "". A help center that gives a rewritten article a new
// address forwards the old one.
func (r readerPublicSource) moved(ctx context.Context, address string) string {
	page, err := r.fetch.Open(ctx, address)
	if err != nil || page.Status != http.StatusOK || page.URL.String() == address {
		return ""
	}
	return page.URL.String()
}

func (r readerPublicSource) source() string { return r.reader.Key() + publicImportSourceSuffix }

func (r readerPublicSource) load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	return r.reader.Load(ctx, homeURL, progress)
}

func (r readerPublicSource) read(ctx context.Context, articleURL string) (*publicContent, error) {
	article, err := r.reader.Article(ctx, articleURL)
	if err != nil {
		return nil, err
	}
	return &publicContent{
		ID: article.ID, Title: article.Title, Description: article.Description, URL: article.URL,
		convert: func(copyImage func(string) string) (docsimport.Node, []docsimport.Warning, error) {
			converted, err := docsimport.ConvertHTML(article.HTML)
			if err != nil {
				return docsimport.Node{}, nil, fmt.Errorf("convert article %s: %w", article.ID, err)
			}
			copyDocumentImages(&converted.Doc, copyImage)
			return converted.Doc, converted.Warnings, nil
		},
		text: func() string { return strings.TrimSpace(htmlToPlainText(article.HTML)) },
	}, nil
}

// copyDocumentImages replaces the address of every image in a document with
// the one copyImage returns. An image that could not be copied keeps its
// source address.
func copyDocumentImages(node *docsimport.Node, copyImage func(source string) string) {
	if copyImage == nil {
		return
	}
	if node.Type == "resizableImage" {
		if source, ok := node.Attrs["src"].(string); ok && strings.HasPrefix(source, "http") {
			if stored := copyImage(source); stored != "" {
				node.Attrs["src"] = stored
			}
		}
		return
	}
	for i := range node.Content {
		copyDocumentImages(&node.Content[i], copyImage)
	}
}
