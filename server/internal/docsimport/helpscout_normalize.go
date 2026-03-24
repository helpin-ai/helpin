package docsimport

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
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
	if n.Type != html.ElementNode || n.Data != "div" {
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
}{
	"www.youtube.com":       {provider: "youtube", pathPrefix: "/embed/"},
	"youtube.com":           {provider: "youtube", pathPrefix: "/embed/"},
	"www.youtube-nocookie.com": {provider: "youtube", pathPrefix: "/embed/"},
	"player.vimeo.com":      {provider: "vimeo", pathPrefix: "/video/"},
	"www.loom.com":          {provider: "loom", pathPrefix: "/embed/"},
	"fast.wistia.net":       {provider: "wistia", pathPrefix: "/embed/iframe/"},
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
			embedURL := "https://" + host + u.Path
			if u.RawQuery != "" {
				embedURL += "?" + u.RawQuery
			}
			return info.provider, src, embedURL, true
		}
	}
	return "", "", "", false
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
