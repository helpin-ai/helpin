package llmstxt

import (
	"cmp"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

const (
	// versionsHeading is the section where Fern lists the versions of a
	// product, and defaultVersion the description of the one people see.
	versionsHeading = "Versions"
	defaultVersion  = "default version"
	// indexesHeading is the section where Mintlify repeats every index file
	// of the site, the other languages' included.
	indexesHeading = "Indexes"
	// fullDump is the file that holds every page in one; it is not an index.
	fullDump = "llms-full.txt"
)

var (
	_heading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	// A heading that is a link to the section's own page: "[Name](address)".
	_headingLink = regexp.MustCompile(`^\[(.+)\]\(\s*<?([^\s)>]+)>?\s*\)$`)
	// The standard entry: `- [Title](address): description`.
	_entry = regexp.MustCompile(`^(\s*)[-*+]\s+\[(.+?)\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)\s*(?:[:–—-]\s*)?(.*)$`)
	// Ferndesk's entry: `- [Title]: address`.
	_referenceEntry = regexp.MustCompile(`^(\s*)[-*+]\s+\[(.+?)\]:\s*<?([^\s>]+)>?\s*$`)
	// An entry that is only an address: `- https://example.com/page.md`.
	_bareEntry = regexp.MustCompile(`^(\s*)[-*+]\s+<?(https?://[^\s>]+)>?\s*$`)
	// What follows the name of a section or linked index: Mintlify adds the
	// size, "Documentation (207 pages)", and Archbee the path, "APIs (/apis)".
	_nameSuffix = regexp.MustCompile(`\s*\((?:\d+ pages?|/\S*)\)$`)
	// Plain ends a description with "(author: Ada, updated: 2026-01-02)".
	_byline      = regexp.MustCompile(`\s*\(author: [^()]*\)$`)
	_notSlugChar = regexp.MustCompile(`[^a-z0-9]+`)
)

// node is a collection while the index is read.
type node struct {
	name, description, url string
	articles               []publichelp.ArticleRef
	children               []*node
	// index is the address of a further index file that lists this node's
	// pages. preview holds the pages the linking index showed below the link;
	// they stand in when that file cannot be used.
	index   string
	preview []publichelp.ArticleRef
}

// parsed is what one index file holds besides the collections and pages
// added to its node.
type parsed struct {
	title string
	// entries counts the links to pages and further indexes of the site, and
	// prose the lines of other text.
	entries, prose int
	// linked are the nodes whose pages a further index file lists.
	linked []*node
}

// level is an open heading while an index is parsed.
type level struct {
	depth int
	node  *node
}

// parseIndex reads an index file served at base into root: headings become
// nested collections, and list entries pages or links to further indexes.
// Links to other hosts and to files that are not pages are left out.
func parseIndex(body []byte, base *url.URL, root *node) parsed {
	var out parsed
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	hasDefaultVersion := strings.Contains(text, "): "+defaultVersion)
	open := []level{{depth: 0, node: root}}
	current := root
	// under is the last link to a further index and the indent of its line.
	var under *node
	underIndent := 0
	skipIndexes, onlyDefault := false, false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if match := _heading.FindStringSubmatch(trimmed); match != nil {
			under = nil
			name, link := match[2], ""
			if linked := _headingLink.FindStringSubmatch(name); linked != nil {
				name, link = linked[1], linked[2]
			}
			if len(match[1]) == 1 && out.title == "" && out.entries == 0 && len(open) == 1 {
				out.title = name
				continue
			}
			for len(open) > 1 && open[len(open)-1].depth >= len(match[1]) {
				open = open[:len(open)-1]
			}
			parent := open[len(open)-1].node
			// What comes before this section already lists the site's own
			// language, so its links would only add the other languages.
			skipIndexes = strings.EqualFold(name, indexesHeading) && out.entries > 0
			onlyDefault = strings.EqualFold(name, versionsHeading) && hasDefaultVersion
			if onlyDefault {
				// The versions are not a level of their own.
				current = parent
				continue
			}
			current = &node{name: _nameSuffix.ReplaceAllString(name, "")}
			if target := resolve(base, link); target != nil {
				current.url = pageURL(target).String()
			}
			parent.children = append(parent.children, current)
			open = append(open, level{depth: len(match[1]), node: current})
			continue
		}
		indent, title, href, summary, ok := entry(line)
		if !ok {
			switch {
			case !strings.HasPrefix(trimmed, ">"):
				out.prose++
			case current != root && len(current.articles) == 0 && len(current.children) == 0:
				// HelpKit describes a section in a quote below its heading.
				quote := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
				current.description = strings.TrimSpace(current.description + " " + quote)
			}
			continue
		}
		target := resolve(base, href)
		if target == nil {
			continue
		}
		file := strings.ToLower(path.Base(target.Path))
		switch {
		case strings.HasSuffix(file, ".txt") && file != fullDump:
			if skipIndexes {
				continue
			}
			out.entries++
			if onlyDefault && summary != defaultVersion {
				continue
			}
			under, underIndent = &node{name: _nameSuffix.ReplaceAllString(title, ""), index: target.String()}, indent
			if !onlyDefault {
				under.description = summary
			}
			current.children = append(current.children, under)
			out.linked = append(out.linked, under)
		case strings.HasSuffix(file, ".md"):
			out.entries++
			page := pageURL(target)
			ref := publichelp.ArticleRef{
				ID: pageID(page), URL: page.String(), Summary: _byline.ReplaceAllString(summary, ""),
				Title: cmp.Or(title, publichelp.TitleFromSlug(path.Base(page.Path))),
			}
			if under != nil && indent > underIndent {
				under.preview = append(under.preview, ref)
				continue
			}
			under = nil
			current.articles = append(current.articles, ref)
		}
	}
	return out
}

// entry reads a list line that links somewhere, in any of the forms indexes
// use. indent is the width of the white space before the bullet.
func entry(line string) (indent int, title, href, summary string, ok bool) {
	if match := _entry.FindStringSubmatch(line); match != nil {
		return len(match[1]), strings.TrimSpace(match[2]), match[3], strings.TrimSpace(match[4]), true
	}
	if match := _referenceEntry.FindStringSubmatch(line); match != nil {
		return len(match[1]), strings.TrimSpace(match[2]), match[3], "", true
	}
	if match := _bareEntry.FindStringSubmatch(line); match != nil {
		return len(match[1]), "", match[2], "", true
	}
	return 0, "", "", "", false
}

// resolve makes a link of an index absolute. It returns nil for a link that
// leaves the site or is not a web address.
func resolve(base *url.URL, href string) *url.URL {
	if href == "" {
		return nil
	}
	target, err := base.Parse(href)
	if err != nil || (target.Scheme != "https" && target.Scheme != "http") || !sameSite(target, base) {
		return nil
	}
	target.Fragment, target.RawFragment = "", ""
	return target
}

// collections turns what the index files held into the site's collections:
// every page once, where it is listed first, and no collection without a
// page. Pages listed outside any section are grouped by address.
func collections(root *node, siteName, docsRoot string) []publichelp.Collection {
	seen := map[string]bool{}
	var unique func(*node)
	unique = func(n *node) {
		kept := n.articles[:0:0]
		for _, ref := range n.articles {
			address, err := url.Parse(ref.URL)
			if err != nil || ref.ID == "" || seen[publichelp.LinkKey(address)] {
				continue
			}
			seen[publichelp.LinkKey(address)] = true
			kept = append(kept, ref)
		}
		n.articles = kept
		for _, child := range n.children {
			unique(child)
		}
	}
	unique(root)
	return build(tidy(append(group(root.articles, siteName, docsRoot), root.children...)), "")
}

// group arranges the pages of an index without sections by the first part of
// their address below the docs root, in the order the parts first appear.
// Pages directly at the root go into a collection named after the site.
func group(articles []publichelp.ArticleRef, siteName, docsRoot string) []*node {
	var out []*node
	byName := map[string]*node{}
	for _, ref := range articles {
		below := ref.ID
		if docsRoot != "" {
			below = strings.Trim(strings.TrimPrefix(below+"/", docsRoot+"/"), "/")
		}
		name := cmp.Or(siteName, "Documentation")
		if first, _, nested := strings.Cut(below, "/"); nested {
			name = publichelp.TitleFromSlug(first)
		}
		if byName[name] == nil {
			byName[name] = &node{name: name}
			out = append(out, byName[name])
		}
		byName[name].articles = append(byName[name].articles, ref)
	}
	return out
}

// tidy drops the collections that list no page, and folds a collection that
// only wraps another one into it, such as a linked index whose file holds a
// single section.
func tidy(nodes []*node) []*node {
	var out []*node
	for _, n := range nodes {
		n.children = tidy(n.children)
		if len(n.articles) == 0 && len(n.children) == 1 {
			only := n.children[0]
			n.articles, n.children = only.articles, only.children
			n.url, n.description = cmp.Or(n.url, only.url), cmp.Or(n.description, only.description)
		}
		if len(n.articles) > 0 || len(n.children) > 0 {
			out = append(out, n)
		}
	}
	return out
}

// build gives every collection an ID made of its name and the names above
// it; an index has no other identity for a section.
func build(nodes []*node, parentID string) []publichelp.Collection {
	out := make([]publichelp.Collection, 0, len(nodes))
	used := map[string]bool{}
	for _, n := range nodes {
		name := cmp.Or(strings.TrimSpace(n.name), "Untitled")
		slug := cmp.Or(strings.Trim(_notSlugChar.ReplaceAllString(strings.ToLower(name), "-"), "-"), "section")
		for i, base := 2, slug; used[slug]; i++ {
			slug = base + "-" + strconv.Itoa(i)
		}
		used[slug] = true
		id := strings.TrimPrefix(parentID+"/"+slug, "/")
		out = append(out, publichelp.Collection{
			ID: id, Name: name, Slug: slug, Description: n.description, URL: n.url,
			Articles: n.articles, Children: build(n.children, id),
		})
	}
	return out
}
