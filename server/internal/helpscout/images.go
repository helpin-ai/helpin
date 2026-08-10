package helpscout

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ImageUploader abstracts S3 upload for testing.
type ImageUploader interface {
	UploadImage(ctx context.Context, workspaceID, filename string, data io.Reader, contentType string) (string, error)
}

var downloadAndUploadImage = downloadAndUpload

// ExtractImageURLs parses HTML and returns unique image URLs found in <img src="..."> tags.
func ExtractImageURLs(rawHTML string) []string {
	doc, err := htmlpkgParse(rawHTML)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var urls []string
	walkElements(findBody(doc), func(n *html.Node) {
		if n.Type != html.ElementNode || n.DataAtom != atom.Img {
			return
		}
		if _, src := getImageSource(n); src != "" {
			if _, ok := seen[src]; ok {
				return
			}
			seen[src] = struct{}{}
			urls = append(urls, src)
		}
	})
	return urls
}

// ReplaceImageURLs replaces all occurrences of old URLs with new URLs in the HTML string.
func ReplaceImageURLs(rawHTML string, urlMap map[string]string) string {
	doc, err := htmlpkgParse(rawHTML)
	if err != nil {
		return rawHTML
	}
	walkElements(findBody(doc), func(n *html.Node) {
		if n.Type != html.ElementNode || n.DataAtom != atom.Img {
			return
		}
		key, src := getImageSource(n)
		if src == "" {
			return
		}
		if newURL, ok := urlMap[src]; ok {
			setAttr(n, key, newURL)
		}
	})
	return renderBodyChildren(doc)
}

// ProcessImagesDetailed downloads images referenced in HTML, re-uploads them
// via the provided uploader, and returns the rewritten HTML plus any URLs that
// had to be kept because rewrite failed.
func ProcessImagesDetailed(ctx context.Context, rawHTML string, uploader ImageUploader, workspaceID string) (string, []string, error) {
	urls := ExtractImageURLs(rawHTML)
	if len(urls) == 0 {
		return rawHTML, nil, nil
	}

	urlMap := make(map[string]string, len(urls))
	var kept []string
	for _, srcURL := range urls {
		newURL, err := downloadAndUploadImage(ctx, srcURL, uploader, workspaceID)
		if err != nil {
			slog.WarnContext(ctx, "skip image download",
				"url", srcURL,
				"error", err,
			)
			kept = append(kept, srcURL)
			continue
		}
		urlMap[srcURL] = newURL
	}

	if len(urlMap) == 0 {
		return rawHTML, kept, nil
	}
	return ReplaceImageURLs(rawHTML, urlMap), kept, nil
}

func htmlpkgParse(raw string) (*html.Node, error) {
	return html.Parse(strings.NewReader(raw))
}

func findBody(doc *html.Node) *html.Node {
	var body *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if body != nil {
				return
			}
		}
	}
	walk(doc)
	if body != nil {
		return body
	}
	return doc
}

func walkElements(n *html.Node, fn func(*html.Node)) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			fn(c)
			walkElements(c, fn)
		}
	}
}

func renderBodyChildren(doc *html.Node) string {
	var builder strings.Builder
	for child := findBody(doc).FirstChild; child != nil; child = child.NextSibling {
		_ = html.Render(&builder, child)
	}
	return builder.String()
}

func getImageSource(n *html.Node) (string, string) {
	if src := getAttr(n, "src"); src != "" {
		return "src", src
	}
	if src := getAttr(n, "data-src"); src != "" {
		return "data-src", src
	}
	return "", ""
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}

// downloadAndUpload fetches an image from srcURL and uploads it via the uploader.
func downloadAndUpload(ctx context.Context, srcURL string, uploader ImageUploader, workspaceID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request for %q: %w", srcURL, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image %q: %w", srcURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image %q: status %d", srcURL, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Derive a filename from the URL path.
	filename := filenameFromURL(srcURL)

	newURL, err := uploader.UploadImage(ctx, workspaceID, filename, resp.Body, contentType)
	if err != nil {
		return "", fmt.Errorf("upload image %q: %w", srcURL, err)
	}
	return newURL, nil
}

// filenameFromURL extracts the last path segment from a URL as the filename.
// Falls back to "image" if the URL has no usable path.
func filenameFromURL(rawURL string) string {
	// Find last slash.
	idx := strings.LastIndex(rawURL, "/")
	if idx >= 0 && idx < len(rawURL)-1 {
		name := rawURL[idx+1:]
		// Strip query string if present.
		if qi := strings.Index(name, "?"); qi >= 0 {
			name = name[:qi]
		}
		if name != "" {
			return name
		}
	}
	return "image"
}
