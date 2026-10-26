package helpscout

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/helpin-ai/helpin/server/internal/netguard"
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

// ImageHTTPClient downloads images named in imported content. It reaches only
// public addresses, so imported HTML cannot make the server fetch internal
// URLs.
var ImageHTTPClient = &http.Client{
	Timeout:   60 * time.Second,
	Transport: &http.Transport{Proxy: nil, DialContext: netguard.DialContext(false), TLSHandshakeTimeout: 10 * time.Second},
}

// downloadAndUpload fetches an image from srcURL and uploads it via the uploader.
func downloadAndUpload(ctx context.Context, srcURL string, uploader ImageUploader, workspaceID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request for %q: %w", srcURL, err)
	}

	resp, err := ImageHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image %q: %w", srcURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image %q: status %d", srcURL, resp.StatusCode)
	}

	// Derive a filename from the URL path.
	filename := filenameFromURL(srcURL)
	body, contentType, err := ImageBody(resp, filename)
	if err != nil {
		return "", fmt.Errorf("download image %q: %w", srcURL, err)
	}

	newURL, err := uploader.UploadImage(ctx, workspaceID, filename, body, contentType)
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

// MaxImageBytes is the largest image an import copies.
const MaxImageBytes = 25 << 20

// ErrNotImage means a download did not return an image.
var ErrNotImage = errors.New("remote URL is not an image")

// ErrImageTooLarge means an image exceeds MaxImageBytes.
var ErrImageTooLarge = errors.New("the image is larger than 25 MB")

// ImageBody checks that resp carries an image and returns its body, which
// fails once it passes MaxImageBytes, with the image's content type. A
// generic or missing type is resolved from the content, then from the file
// name.
func ImageBody(resp *http.Response, filename string) (io.Reader, string, error) {
	if resp.ContentLength > MaxImageBytes {
		return nil, "", ErrImageTooLarge
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	reader := bufio.NewReaderSize(resp.Body, 512)
	if !strings.HasPrefix(contentType, "image/") {
		if contentType != "" && contentType != "application/octet-stream" && contentType != "binary/octet-stream" {
			return nil, "", ErrNotImage
		}
		head, _ := reader.Peek(512)
		if sniffed := http.DetectContentType(head); strings.HasPrefix(sniffed, "image/") {
			contentType = sniffed
		} else if byName := mime.TypeByExtension(strings.ToLower(path.Ext(filename))); strings.HasPrefix(byName, "image/") {
			contentType = byName
		} else {
			return nil, "", ErrNotImage
		}
	}
	return &cappedReader{reader: reader, left: MaxImageBytes}, contentType, nil
}

// cappedReader fails instead of truncating once more than left bytes arrive.
type cappedReader struct {
	reader io.Reader
	left   int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		var probe [1]byte
		n, err := c.reader.Read(probe[:])
		if n > 0 {
			return 0, ErrImageTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.reader.Read(p)
	c.left -= int64(n)
	return n, err
}
