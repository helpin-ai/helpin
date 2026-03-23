package docsimport

import (
	"strings"

	"golang.org/x/net/html"
)

// calloutVariantMap maps Help Scout callout classes to Helpin callout variants.
var calloutVariantMap = map[string]string{
	"info":   "blue",
	"warn":   "yellow",
	"danger": "red",
}

// isHelpScoutCallout checks if a node is a Help Scout callout div and returns the variant.
func isHelpScoutCallout(n *html.Node) (variant string, ok bool) {
	if n.Type != html.ElementNode || n.Data != "div" {
		return "", false
	}
	classes := getAttr(n, "class")
	if !strings.Contains(classes, "callout") {
		return "", false
	}
	for hsClass, v := range calloutVariantMap {
		if strings.Contains(classes, "callout-"+hsClass) {
			return v, true
		}
	}
	// Has "callout" class but unknown type — default to grey
	if strings.Contains(classes, "callout") {
		return "grey", true
	}
	return "", false
}

// videoProviders maps embed hostnames to provider names and path prefixes.
var videoProviders = map[string]struct {
	provider   string
	pathPrefix string
}{
	"www.youtube.com":  {provider: "youtube", pathPrefix: "/embed/"},
	"player.vimeo.com": {provider: "vimeo", pathPrefix: "/video/"},
	"www.loom.com":     {provider: "loom", pathPrefix: "/embed/"},
	"fast.wistia.net":  {provider: "wistia", pathPrefix: "/embed/iframe/"},
}

// parseVideoIframe checks if an iframe src is a supported video provider.
// Returns provider, sourceUrl, embedUrl if matched.
func parseVideoIframe(src string) (provider, sourceUrl, embedUrl string, ok bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", "", "", false
	}

	// Normalize protocol
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}

	for host, info := range videoProviders {
		// Check if src contains the host and path prefix
		prefix := "https://" + host + info.pathPrefix
		if strings.HasPrefix(src, prefix) {
			return info.provider, src, src, true
		}
		// Also check http
		httpPrefix := "http://" + host + info.pathPrefix
		if strings.HasPrefix(src, httpPrefix) {
			httpsURL := "https://" + host + info.pathPrefix + strings.TrimPrefix(src, httpPrefix)
			return info.provider, src, httpsURL, true
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
