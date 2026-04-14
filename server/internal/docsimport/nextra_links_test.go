package docsimport

import (
	"testing"
)

func TestNextraRouteForPath_ContentIndex(t *testing.T) {
	route := NextraRouteForPath("content", "content/index.mdx")
	if route != "/" {
		t.Errorf("expected '/', got %q", route)
	}
}

func TestNextraRouteForPath_ContentGettingStarted(t *testing.T) {
	route := NextraRouteForPath("content", "content/getting-started.mdx")
	if route != "/getting-started" {
		t.Errorf("expected '/getting-started', got %q", route)
	}
}

func TestNextraRouteForPath_NestedIndex(t *testing.T) {
	route := NextraRouteForPath("content", "content/guides/index.mdx")
	if route != "/guides" {
		t.Errorf("expected '/guides', got %q", route)
	}
}

func TestNextraRouteForPath_NestedPage(t *testing.T) {
	route := NextraRouteForPath("content", "content/guides/install.mdx")
	if route != "/guides/install" {
		t.Errorf("expected '/guides/install', got %q", route)
	}
}

func TestNextraRouteForPath_AppDocsPage(t *testing.T) {
	route := NextraRouteForPath("apps/docs/content", "apps/docs/content/getting-started/page.mdx")
	if route != "/getting-started" {
		t.Errorf("expected '/getting-started', got %q", route)
	}
}

func TestNextraRouteForPath_RootDot(t *testing.T) {
	route := NextraRouteForPath(".", "index.mdx")
	if route != "/" {
		t.Errorf("expected '/', got %q", route)
	}
}

func TestNextraRouteForPath_RootDotNested(t *testing.T) {
	route := NextraRouteForPath(".", "guides/install.md")
	if route != "/guides/install" {
		t.Errorf("expected '/guides/install', got %q", route)
	}
}

func TestRewriteNextraInternalLinks_Relative(t *testing.T) {
	routeMap := map[string]string{
		"/getting-started": "getting-started",
		"/guides/install":  "install",
	}
	content := `Check [install guide](./install) for details.`
	result, warnings := RewriteNextraInternalLinks(content, "/guides/setup", routeMap)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if result != `Check [install guide](/articles/install) for details.` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRewriteNextraInternalLinks_ParentRelative(t *testing.T) {
	routeMap := map[string]string{
		"/reference/api": "api",
	}
	content := `See [API](../reference/api) docs.`
	result, warnings := RewriteNextraInternalLinks(content, "/guides/setup", routeMap)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if result != `See [API](/articles/api) docs.` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRewriteNextraInternalLinks_HashPreserved(t *testing.T) {
	routeMap := map[string]string{
		"/guides/install": "install",
	}
	content := `See [install](./install#prerequisites).`
	result, _ := RewriteNextraInternalLinks(content, "/guides/setup", routeMap)
	if result != `See [install](/articles/install#prerequisites).` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRewriteNextraInternalLinks_ExternalUnchanged(t *testing.T) {
	routeMap := map[string]string{}
	content := `Visit [Example](https://example.com) for more.`
	result, _ := RewriteNextraInternalLinks(content, "/guides/setup", routeMap)
	if result != content {
		t.Errorf("external link should not change, got: %q", result)
	}
}

func TestRewriteNextraInternalLinks_MissingLinkWarning(t *testing.T) {
	routeMap := map[string]string{}
	content := `See [missing](./nonexistent) page.`
	_, warnings := RewriteNextraInternalLinks(content, "/guides/setup", routeMap)
	found := false
	for _, w := range warnings {
		if w.Type == "broken_internal_link" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected broken_internal_link warning for missing target")
	}
}

func TestRewriteNextraInternalLinks_AbsoluteInternal(t *testing.T) {
	routeMap := map[string]string{
		"/getting-started": "getting-started",
	}
	content := `See [start](/getting-started).`
	result, warnings := RewriteNextraInternalLinks(content, "/guides/setup", routeMap)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if result != `See [start](/articles/getting-started).` {
		t.Errorf("unexpected result: %q", result)
	}
}
