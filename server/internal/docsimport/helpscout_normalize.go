package docsimport

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// calloutVariantMap maps Help Scout callout classes to Helpin callout variants.
var calloutVariantMap = map[string]string{
	"info":    "blue",
	"warn":    "yellow",
	"warning": "yellow",
	"danger":  "red",
	"error":   "red",
	"success": "green",
	"tip":     "green",
}

// isHelpScoutCallout checks if a node is a Help Scout callout div and returns the variant.
func isHelpScoutCallout(n *html.Node) (variant string, ok bool) {
	if n.Type != html.ElementNode {
		return "", false
	}
	switch n.DataAtom {
	case atom.Div, atom.Section, atom.Aside:
	default:
		return "", false
	}
	classes := getAttr(n, "class")
	if classes == "" {
		return "", false
	}

	for _, cls := range strings.Fields(classes) {
		// Direct color class: "callout-blue", "callout-green", "callout-red", "callout-yellow", "callout-grey"
		for _, v := range []string{"blue", "green", "red", "yellow", "grey"} {
			if cls == "callout-"+v {
				return v, true
			}
		}

		// HelpScout semantic classes: "callout-info", "callout-warn", "callout-danger", etc.
		for hsClass, v := range calloutVariantMap {
			if cls == "callout-"+hsClass || cls == "callout--"+hsClass || cls == "hs-callout-"+hsClass {
				return v, true
			}
		}

		// Standalone "callout" or "hs-callout" class (no variant suffix)
		if cls == "callout" || cls == "hs-callout" {
			// Check other classes for variant
			for _, cls2 := range strings.Fields(classes) {
				if cls2 == cls {
					continue
				}
				for hsClass, v := range calloutVariantMap {
					if cls2 == hsClass || cls2 == "callout-"+hsClass {
						return v, true
					}
				}
			}
			return "grey", true
		}
	}

	return "", false
}

// videoProviders maps embed hostnames to provider names and path prefixes.
var videoProviders = map[string]struct {
	provider   string
	pathPrefix string
	embedHost  string
}{
	"www.youtube.com":          {provider: "youtube", pathPrefix: "/embed/", embedHost: "www.youtube.com"},
	"youtube.com":              {provider: "youtube", pathPrefix: "/embed/", embedHost: "www.youtube.com"},
	"www.youtube-nocookie.com": {provider: "youtube", pathPrefix: "/embed/", embedHost: "www.youtube.com"},
	"player.vimeo.com":         {provider: "vimeo", pathPrefix: "/video/", embedHost: "player.vimeo.com"},
	"www.loom.com":             {provider: "loom", pathPrefix: "/embed/", embedHost: "www.loom.com"},
	"loom.com":                 {provider: "loom", pathPrefix: "/embed/", embedHost: "www.loom.com"},
	"www.useloom.com":          {provider: "loom", pathPrefix: "/embed/", embedHost: "www.loom.com"},
	"useloom.com":              {provider: "loom", pathPrefix: "/embed/", embedHost: "www.loom.com"},
	"fast.wistia.net":          {provider: "wistia", pathPrefix: "/embed/iframe/", embedHost: "fast.wistia.net"},
}

// parseVideoIframe checks if an iframe src is a supported video provider.
func parseVideoIframe(src string) (provider, sourceUrl, embedUrl string, ok bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", "", false
	}

	// Normalize protocol
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return "", "", "", false
	}

	// Parse URL properly
	u, err := url.Parse(src)
	if err != nil {
		return "", "", "", false
	}

	for host, info := range videoProviders {
		if u.Host == host && strings.HasPrefix(u.Path, info.pathPrefix) {
			// Normalize to https
			embedURL := "https://" + info.embedHost + u.Path
			if u.RawQuery != "" {
				embedURL += "?" + u.RawQuery
			}
			return info.provider, src, embedURL, true
		}
	}
	return "", "", "", false
}

// ContainsSupportedVideoEmbed reports whether source HTML contains an iframe
// or Help Scout wrapper that can be represented as a native videoEmbed node.
func ContainsSupportedVideoEmbed(rawHTML string) bool {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return false
	}
	return containsSupportedVideoEmbedNode(doc)
}

func containsSupportedVideoEmbedNode(root *html.Node) bool {
	var walk func(*html.Node) bool
	walk = func(node *html.Node) bool {
		if node.Type == html.ElementNode {
			if node.DataAtom == atom.Iframe {
				if _, _, _, ok := parseVideoIframe(getAttr(node, "src")); ok {
					return true
				}
			}
			if _, ok := parseWistiaEmbedContainer(node); ok {
				return true
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if walk(child) {
				return true
			}
		}
		return false
	}
	return walk(root)
}

// ContainsGIFImage reports whether source HTML contains an animated GIF image.
// GIFs must bypass the Markdown/AI conversion path because an omitted Markdown
// image silently removes the animation from the imported article.
func ContainsGIFImage(rawHTML string) bool {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return false
	}
	found := false
	walkElements(findBody(doc), func(node *html.Node) {
		if found || node.Type != html.ElementNode || node.DataAtom != atom.Img {
			return
		}
		found = isGIFImageSource(getAttr(node, "src")) ||
			isGIFImageSource(getAttr(node, "data-src"))
	})
	return found
}

func isGIFImageSource(source string) bool {
	value := strings.TrimSpace(source)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/gif;") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(parsed.Path), ".gif")
}

// getAttr returns the value of an attribute on an HTML node.
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
