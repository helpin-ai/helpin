package docsimport

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// NextraRouteForPath derives the public Nextra route for a file path,
// given the detected content root. For example:
//
//	NextraRouteForPath("content", "content/getting-started.mdx") => "/getting-started"
//	NextraRouteForPath("content", "content/guides/index.mdx")    => "/guides"
func NextraRouteForPath(rootPath, filePath string) string {
	// Make path relative to root.
	var rel string
	if rootPath == "." {
		rel = filePath
	} else {
		rel = strings.TrimPrefix(filePath, rootPath+"/")
	}

	// Remove extension.
	ext := path.Ext(rel)
	rel = strings.TrimSuffix(rel, ext)

	// Handle index and page files.
	base := path.Base(rel)
	if base == "index" || base == "page" {
		rel = path.Dir(rel)
		if rel == "." {
			return "/"
		}
	}

	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}

	return rel
}

// RewriteNextraInternalLinks rewrites markdown links that point to
// internal Nextra routes. It uses routeMap (source route -> target
// slug) to resolve links. At preview time, links are rewritten to
// slug-only paths under /articles/. The executor later rewrites them
// to canonical paths with PublicIDs.
func RewriteNextraInternalLinks(content string, currentRoute string, routeMap map[string]string) (string, []Warning) {
	var warnings []Warning

	// Match markdown links: [text](url)
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

	result := linkRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := linkRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		text := sub[1]
		href := sub[2]

		// Skip external links.
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "mailto:") {
			return match
		}

		// Extract hash fragment.
		hash := ""
		if idx := strings.Index(href, "#"); idx >= 0 {
			hash = href[idx:]
			href = href[:idx]
		}

		if href == "" {
			// Pure hash link.
			return fmt.Sprintf("[%s](%s)", text, hash)
		}

		// Resolve the target route.
		var targetRoute string
		if strings.HasPrefix(href, "/") {
			// Absolute internal link.
			targetRoute = href
		} else {
			// Relative link — resolve against current route's directory.
			currentDir := path.Dir(currentRoute)
			targetRoute = path.Clean(currentDir + "/" + href)
			if !strings.HasPrefix(targetRoute, "/") {
				targetRoute = "/" + targetRoute
			}
		}

		// Look up the slug.
		slug, ok := routeMap[targetRoute]
		if !ok {
			warnings = append(warnings, Warning{
				Type:    "broken_internal_link",
				Message: fmt.Sprintf("link to %q (resolved: %s) not found in route map", sub[2], targetRoute),
			})
			return match
		}

		return fmt.Sprintf("[%s](/articles/%s%s)", text, slug, hash)
	})

	return result, warnings
}

// BuildRouteMap creates a mapping from Nextra source routes to article
// slugs for link rewriting.
func BuildRouteMap(articles []ImportArticle) map[string]string {
	m := make(map[string]string, len(articles))
	for _, a := range articles {
		if a.SourceRoute != "" {
			m[a.SourceRoute] = a.Slug
		}
	}
	return m
}
