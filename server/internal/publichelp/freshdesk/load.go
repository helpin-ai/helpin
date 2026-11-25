package freshdesk

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// folder is what a folder's pages list.
type folder struct {
	name     string
	articles []publichelp.ArticleRef
	// children are the other folders its pages link: the folders inside it.
	children []link
}

// category is what a category page lists.
type category struct {
	name    string
	folders []link
}

// Load implements publichelp.Reader.
func (r *Reader) Load(ctx context.Context, homeURL string, progress func()) (*publichelp.Site, error) {
	progress = publichelp.Serial(progress)
	base, err := url.Parse(homeURL)
	if err != nil || base.Hostname() == "" {
		return nil, fmt.Errorf("%w: %q", publichelp.ErrInvalidURL, homeURL)
	}
	prefix := portalPrefix(base)
	site := newSite(base.Scheme+"://"+base.Host, prefix)
	home, err := r.page(ctx, site.BaseURL, progress)
	if err != nil {
		return nil, err
	}
	site.Name, site.Locale = siteName(home.doc), strings.TrimSpace(home.doc.Find("html").AttrOr("lang", ""))
	listed := links(content(home.doc), home.url, prefix, kindCategory)

	var sitemap []publichelp.SitemapEntry
	categories := make([]category, len(listed))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrency)
	group.Go(func() error {
		// The sitemap only completes the listing, and a portal may refuse it;
		// without it the pages alone are read.
		sitemap, _ = r.fetch.Sitemap(groupCtx, strings.TrimSuffix(site.BaseURL, homePath)+sitemapPath, progress)
		return groupCtx.Err()
	})
	for i, found := range listed {
		group.Go(func() error {
			page, err := r.page(groupCtx, found.url, progress)
			if err != nil {
				return fmt.Errorf("read category %s: %w", found.id, err)
			}
			categories[i] = category{
				name:    cmp.Or(titleBeforeSite(page.doc), found.title, "Untitled"),
				folders: links(content(page.doc), page.url, prefix, kindFolder),
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	folders, err := r.folders(ctx, categories, prefix, progress)
	if err != nil {
		return nil, err
	}
	site.Collections = tree(listed, categories, folders)
	if other := unlisted(sitemap, base.Hostname(), prefix, site.Collections); len(other) > 0 {
		site.Collections = append(site.Collections, publichelp.Collection{ID: otherID, Name: otherName, Articles: other})
	}
	return site, nil
}

// folders reads the folders of every category level by level: a folder page
// names the folders inside it, which a category page may not list.
func (r *Reader) folders(
	ctx context.Context, categories []category, prefix string, progress func(),
) (map[string]*folder, error) {
	out := map[string]*folder{}
	var level []link
	for _, found := range categories {
		level = append(level, found.folders...)
	}
	for depth := 0; len(level) > 0 && depth < maxDepth; depth++ {
		var pending []link
		queued := map[string]bool{}
		for _, found := range level {
			if out[found.id] == nil && !queued[found.id] {
				queued[found.id] = true
				pending = append(pending, found)
			}
		}
		read := make([]*folder, len(pending))
		group, groupCtx := errgroup.WithContext(ctx)
		group.SetLimit(concurrency)
		for i, found := range pending {
			group.Go(func() error {
				var err error
				if read[i], err = r.folder(groupCtx, found, prefix, progress); err != nil {
					return fmt.Errorf("read folder %s: %w", found.id, err)
				}
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return nil, err
		}
		level = nil
		for i, found := range pending {
			out[found.id] = read[i]
			level = append(level, read[i].children...)
		}
	}
	return out, nil
}

// unlisted returns the articles of the sitemap that no collection lists.
func unlisted(
	sitemap []publichelp.SitemapEntry, host, prefix string, collections []publichelp.Collection,
) []publichelp.ArticleRef {
	seen := map[string]bool{}
	var mark func(collections []publichelp.Collection)
	mark = func(collections []publichelp.Collection) {
		for _, collection := range collections {
			for _, article := range collection.Articles {
				seen[article.ID] = true
			}
			mark(collection.Children)
		}
	}
	mark(collections)
	var out []publichelp.ArticleRef
	for _, entry := range sitemap {
		parsed, err := url.Parse(entry.Loc)
		if err != nil || !strings.EqualFold(parsed.Hostname(), host) {
			continue
		}
		match := _contentPath.FindStringSubmatch(parsed.Path)
		if match == nil || match[2] != kindArticle || !strings.EqualFold(match[1], prefix) || seen[match[3]] {
			continue
		}
		seen[match[3]] = true
		out = append(out, publichelp.ArticleRef{ID: match[3], Title: publichelp.TitleFromSlug(match[4]), URL: entry.Loc})
	}
	return out
}

// folder reads every page of a folder.
func (r *Reader) folder(ctx context.Context, found link, prefix string, progress func()) (*folder, error) {
	out := &folder{name: found.title}
	seen := map[string]bool{}
	next := found.url
	for number := 1; next != "" && number <= maxPages; number++ {
		page, err := r.page(ctx, next, progress)
		// A folder for signed-in visitors, or one deleted since it was
		// listed, costs its own articles and not the rest.
		if number == 1 && (errors.Is(err, publichelp.ErrRestricted) || errors.Is(err, publichelp.ErrNotFound)) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		// The pager links every page; the next one ends in "/page/<number+1>".
		next = ""
		following := strconv.Itoa(number + 1)
		page.doc.Find("a[href]").EachWithBreak(func(_ int, anchor *goquery.Selection) bool {
			target, err := page.url.Parse(strings.TrimSpace(anchor.AttrOr("href", "")))
			if err != nil {
				return true
			}
			match := _contentPath.FindStringSubmatch(target.Path)
			if match == nil || match[2] != kindFolder || match[3] != found.id || match[5] != following {
				return true
			}
			next = target.String()
			return false
		})
		if number == 1 {
			out.name = cmp.Or(titleBeforeSite(page.doc), out.name, "Untitled")
		}
		scope := content(page.doc)
		for _, article := range links(scope, page.url, prefix, kindArticle) {
			if !seen[article.id] {
				seen[article.id] = true
				out.articles = append(out.articles, publichelp.ArticleRef{
					ID: article.id, Title: cmp.Or(article.title, publichelp.TitleFromSlug(article.slug)), URL: article.url,
				})
			}
		}
		for _, child := range links(scope, page.url, prefix, kindFolder) {
			if child.id != found.id && !seen[kindFolder+child.id] {
				seen[kindFolder+child.id] = true
				out.children = append(out.children, child)
			}
		}
	}
	return out, nil
}

// content is a page without the parts every page repeats, whose links say
// nothing about what the page holds.
func content(doc *goquery.Document) *goquery.Selection {
	doc.Find(`header, footer, .breadcrumb, .sol-breadcrumb, [aria-label="breadcrumb"]`).Remove()
	return doc.Selection
}

// tree arranges the categories, their folders and the folders inside those
// in the order the portal shows them. An article or folder listed twice is
// kept where it shows first.
func tree(listed []link, categories []category, folders map[string]*folder) []publichelp.Collection {
	// A folder is inside another when that one's page links it and it does
	// not link back: folders that link each other are neighbors in a menu.
	inside := map[string]bool{}
	for id, parent := range folders {
		for _, child := range parent.children {
			found := folders[child.id]
			if found != nil && !slices.ContainsFunc(found.children, func(back link) bool { return back.id == id }) {
				inside[child.id] = true
			}
		}
	}
	placed := map[string]bool{}
	seen := map[string]bool{}
	var build func(found link, depth int) (publichelp.Collection, bool)
	build = func(found link, depth int) (publichelp.Collection, bool) {
		read := folders[found.id]
		if read == nil || placed[found.id] || depth > maxDepth {
			return publichelp.Collection{}, false
		}
		placed[found.id] = true
		out := publichelp.Collection{ID: found.id, Name: read.name, URL: found.url}
		for _, article := range read.articles {
			if !seen[article.ID] {
				seen[article.ID] = true
				out.Articles = append(out.Articles, article)
			}
		}
		for _, child := range read.children {
			if !inside[child.id] {
				continue
			}
			if below, ok := build(child, depth+1); ok {
				out.Children = append(out.Children, below)
			}
		}
		return out, len(out.Articles) > 0 || len(out.Children) > 0
	}
	var out []publichelp.Collection
	for i, found := range listed {
		collection := publichelp.Collection{ID: categoryPrefix + found.id, Name: categories[i].name, URL: found.url}
		for _, top := range categories[i].folders {
			if inside[top.id] {
				continue
			}
			if below, ok := build(top, 0); ok {
				collection.Children = append(collection.Children, below)
			}
		}
		if len(collection.Children) > 0 {
			out = append(out, collection)
		}
	}
	return out
}
