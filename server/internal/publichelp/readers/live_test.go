package readers

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/publichelp"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// TestLiveHelpCenters reads real help centers end to end: detection, the
// article listing, and a few articles converted to documents. It runs only
// when PUBLICHELP_LIVE lists addresses, separated by spaces:
//
//	PUBLICHELP_LIVE="https://help.example.com" go test ./internal/publichelp/readers/ -run Live -v
func TestLiveHelpCenters(t *testing.T) {
	addresses := strings.Fields(os.Getenv("PUBLICHELP_LIVE"))
	if len(addresses) == 0 {
		t.Skip("PUBLICHELP_LIVE is not set")
	}
	fetch := publichelp.NewFetcher()
	for _, address := range addresses {
		t.Run(address, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			started := time.Now()
			reader, found, err := publichelp.Detect(ctx, fetch, All(fetch), address)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			t.Logf("detected %s (%s) %q at %s in %s", found.Platform, reader.Key(), found.Name, found.BaseURL, time.Since(started).Round(time.Millisecond))
			started = time.Now()
			requests := 0
			site, err := reader.Load(ctx, found.BaseURL, func() { requests++ })
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			refs, collections := liveCount(site.Collections, 0, t)
			t.Logf("loaded %d articles in %d collections with %d requests in %s", len(refs), collections, requests, time.Since(started).Round(time.Millisecond))
			if len(refs) == 0 {
				t.Fatal("no articles listed")
			}
			seen := map[string]bool{}
			for _, ref := range refs {
				if seen[ref.ID] {
					t.Errorf("article ID %q is listed twice", ref.ID)
				}
				seen[ref.ID] = true
				if target, ok := site.LinkTarget(ref.URL); !ok || target.Kind != publichelp.LinkArticle || target.ID != ref.ID {
					t.Errorf("link to %s resolves to %+v, %v", ref.URL, target, ok)
				}
			}
			// Three articles from across the listing.
			for _, i := range []int{0, len(refs) / 2, len(refs) - 1} {
				ref := refs[i]
				article, err := reader.Article(ctx, ref.URL)
				if err != nil {
					t.Errorf("article %s: %v", ref.URL, err)
					continue
				}
				converted, err := docsimport.ConvertHTML(article.HTML)
				if err != nil {
					t.Errorf("convert %s: %v", ref.URL, err)
					continue
				}
				content, err := converted.Doc.ToJSON()
				if err != nil {
					t.Errorf("encode %s: %v", ref.URL, err)
					continue
				}
				if err := tiptap.ValidateDocument(content); err != nil {
					t.Errorf("invalid document for %s: %v", ref.URL, err)
				}
				links := 0
				if _, _, err := docsimport.RewriteTiptapLinks(content, func(href string) (string, bool) {
					if _, ok := site.LinkTarget(href); ok {
						links++
					}
					return "", false
				}); err != nil {
					t.Errorf("scan links of %s: %v", ref.URL, err)
				}
				rendered, err := tiptap.RenderHTML(content)
				if err != nil {
					t.Errorf("render %s: %v", ref.URL, err)
				}
				text := liveText(rendered)
				t.Logf("  %q (listed as %q, id %s): %d chars of HTML, %d of text, %d images, %d internal links, %d warnings\n    %.160q",
					article.Title, ref.Title, article.ID, len(article.HTML), len(text), strings.Count(string(content), `"resizableImage"`), links, len(converted.Warnings), text)
				if article.ID != ref.ID {
					t.Errorf("article %s reads back as ID %q, listed as %q", ref.URL, article.ID, ref.ID)
				}
				// A page that is only a video or a picture has no text to check.
				if len(text) < 40 && !strings.Contains(string(content), `"videoEmbed"`) && !strings.Contains(string(content), `"resizableImage"`) {
					t.Errorf("article %s has almost no content: %s", ref.URL, content)
				}
			}
		})
	}
}

// liveText is the visible text of rendered HTML.
func liveText(rendered string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rendered))
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(doc.Text()), " ")
}

func liveCount(collections []publichelp.Collection, depth int, t *testing.T) ([]publichelp.ArticleRef, int) {
	var refs []publichelp.ArticleRef
	count := 0
	for _, collection := range collections {
		count++
		if depth == 0 {
			t.Logf("  collection %q: %d articles, %d subcollections", collection.Name, len(collection.Articles), len(collection.Children))
		}
		refs = append(refs, collection.Articles...)
		below, nested := liveCount(collection.Children, depth+1, t)
		refs = append(refs, below...)
		count += nested
	}
	return refs, count
}
