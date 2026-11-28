package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// importAttachmentTypes are the file types an import copies, by extension.
// Anything else (HTML, SVG, scripts) stays a link to the source, so a copied
// file can never run in a browser.
var importAttachmentTypes = map[string]string{
	".csv":  "text/csv",
	".txt":  "text/plain",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

var errAttachmentType = errors.New("file type is not copied")

// isSourceAttachmentURL reports whether a link points at a file the source
// help center hosts for an article, such as a downloadable CSV template.
// shortcut: Intercom's attachment host only; add other platforms' hosts when an import from them needs it.
func isSourceAttachmentURL(href string) bool {
	parsed, err := url.Parse(href)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "downloads.intercomcdn.com" {
		return false
	}
	_, ok := importAttachmentTypes[strings.ToLower(path.Ext(parsed.Path))]
	return ok
}

// copyDocumentAttachments points every link to a source-hosted file at the
// copy copyFile returns. A file that could not be copied keeps its link.
func copyDocumentAttachments(node *docsimport.Node, copyFile func(source string) string) {
	for i := range node.Marks {
		mark := &node.Marks[i]
		if mark.Type != "link" {
			continue
		}
		if href, ok := mark.Attrs["href"].(string); ok && isSourceAttachmentURL(href) {
			if stored := copyFile(href); stored != "" {
				mark.Attrs["href"] = stored
			}
		}
	}
	for i := range node.Content {
		copyDocumentAttachments(&node.Content[i], copyFile)
	}
}

// copyAttachment stores a linked file once per import and returns its new
// address, or "" when it could not be copied.
func (r *publicImportRun) copyAttachment(ctx context.Context, source, articleTitle string, failed *[]model.ImportFailedImage) string {
	key := imageCacheKey(source)
	if stored, ok := r.imageCache[key]; ok {
		return stored
	}
	if r.uploader == nil {
		return ""
	}
	r.beat()
	stored, err := importExternalFile(ctx, r.workspaceID, source, r.uploader)
	if err != nil {
		r.s.logger.WarnContext(ctx, "copy imported attachment", "job_id", r.jobID, "file_host", hostOfURL(source), "error", err)
		*failed = append(*failed, model.ImportFailedImage{
			ArticleTitle: articleTitle, URL: stripURLQuery(source),
			Reason: "The attached file could not be copied, so its link still points at the source help center.",
		})
		return ""
	}
	r.imageCache[key] = stored
	return stored
}

// importExternalFile downloads an allowed file type and stores it next to
// imported images. The stored type comes from the extension, never from the
// source server.
func importExternalFile(ctx context.Context, workspaceID, fileURL string, uploader helpscout.ImageUploader) (string, error) {
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("invalid file url: %w", err)
	}
	filename := path.Base(parsed.Path)
	if unescaped, err := url.PathUnescape(filename); err == nil {
		filename = unescaped
	}
	filename = strings.NewReplacer("+", "-", " ", "-", "%", "", "?", "", "#", "", "\\", "").Replace(filename)
	contentType, ok := importAttachmentTypes[strings.ToLower(path.Ext(filename))]
	if !ok {
		return "", errAttachmentType
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("create file request: %w", err)
	}
	resp, err := helpscout.ImageHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: status %d", resp.StatusCode)
	}
	// Expired links on some CDNs answer 200 with an HTML error page.
	if served, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); served == "text/html" {
		return "", fmt.Errorf("download file: source returned an HTML page")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, helpscout.MaxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if len(body) > helpscout.MaxImageBytes {
		return "", fmt.Errorf("file is larger than 25 MB")
	}
	return uploader.UploadImage(ctx, workspaceID, filename, bytes.NewReader(body), contentType)
}
