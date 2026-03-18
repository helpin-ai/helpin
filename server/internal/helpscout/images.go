package helpscout

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// imgSrcRe matches src attributes inside <img> tags.
var imgSrcRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)

// ImageUploader abstracts S3 upload for testing.
type ImageUploader interface {
	UploadImage(ctx context.Context, workspaceID, filename string, data io.Reader, contentType string) (string, error)
}

// ExtractImageURLs parses HTML and returns unique image URLs found in <img src="..."> tags.
func ExtractImageURLs(html string) []string {
	matches := imgSrcRe.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		u := m[1]
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	return urls
}

// ReplaceImageURLs replaces all occurrences of old URLs with new URLs in the HTML string.
func ReplaceImageURLs(html string, urlMap map[string]string) string {
	for oldURL, newURL := range urlMap {
		html = strings.ReplaceAll(html, oldURL, newURL)
	}
	return html
}

// ProcessImages downloads images referenced in HTML, re-uploads them via the
// provided uploader, and returns the HTML with URLs replaced. Individual image
// failures are logged and skipped (the original URL is kept).
func ProcessImages(ctx context.Context, html string, uploader ImageUploader, workspaceID string) (string, error) {
	urls := ExtractImageURLs(html)
	if len(urls) == 0 {
		return html, nil
	}

	urlMap := make(map[string]string, len(urls))
	for _, srcURL := range urls {
		newURL, err := downloadAndUpload(ctx, srcURL, uploader, workspaceID)
		if err != nil {
			slog.WarnContext(ctx, "skip image download",
				"url", srcURL,
				"error", err,
			)
			continue
		}
		urlMap[srcURL] = newURL
	}

	if len(urlMap) == 0 {
		return html, nil
	}
	return ReplaceImageURLs(html, urlMap), nil
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
