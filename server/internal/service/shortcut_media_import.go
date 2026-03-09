package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const shortcutMediaStorageWarning = "Shortcut-hosted media was found in imported content but was not migrated because object storage public URLs are not configured"

var shortcutMediaURLRe = regexp.MustCompile(`https?://[^\s<>"')]+`)

type shortcutImportedAttachmentService interface {
	SupportsPublicURL() bool
	CreateImported(ctx context.Context, req model.CreateAttachmentRequest, workspaceID, userID string, body io.Reader) (*model.AttachmentResponse, error)
}

type shortcutMediaDownloader interface {
	Download(ctx context.Context, rawURL, apiToken string) (*shortcutDownloadedMedia, error)
}

type shortcutDownloadedMedia struct {
	FileName    string
	ContentType string
	Data        []byte
}

type shortcutHTTPMediaDownloader struct {
	client *http.Client
}

func newShortcutHTTPMediaDownloader() *shortcutHTTPMediaDownloader {
	return &shortcutHTTPMediaDownloader{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (d *shortcutHTTPMediaDownloader) Download(ctx context.Context, rawURL, apiToken string) (*shortcutDownloadedMedia, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Shortcut-Token", apiToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Shortcut media request was rejected (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Shortcut media request returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxFileSize {
		return nil, fmt.Errorf("Shortcut media file exceeds the %d byte limit", maxFileSize)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read media: %w", err)
	}
	if int64(len(data)) > maxFileSize {
		return nil, fmt.Errorf("Shortcut media file exceeds the %d byte limit", maxFileSize)
	}

	contentType := normalizeMediaType(resp.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = normalizeMediaType(http.DetectContentType(data))
	}

	return &shortcutDownloadedMedia{
		FileName:    firstNonEmpty(downloadFileNameFromHeader(resp.Header.Get("Content-Disposition")), fileNameFromRawURL(rawURL)),
		ContentType: contentType,
		Data:        data,
	}, nil
}

func (s *PMImportService) importShortcutStoryMedia(ctx context.Context, workspaceID, actorID string, storyIDs []string, apiToken, jobID string, stepNum, totalSteps int) (int, []string) {
	if len(storyIDs) == 0 {
		return 0, nil
	}

	var stories []struct {
		ID          string
		Description *string
	}
	if err := s.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Select("id, description").
		Where("workspace_id = ? AND id IN ?", workspaceID, storyIDs).
		Scan(&stories).Error; err != nil {
		return 0, []string{fmt.Sprintf("Failed to load imported stories for media migration: %s", err.Error())}
	}

	var checklistItems []struct {
		ID      string
		StoryID string
		Text    string
	}
	if err := s.db.WithContext(ctx).
		Model(&model.PMChecklistItem{}).
		Select("id, story_id, text").
		Where("story_id IN ?", storyIDs).
		Order("position ASC, created_at ASC").
		Scan(&checklistItems).Error; err != nil {
		return 0, []string{fmt.Sprintf("Failed to load imported checklist items for media migration: %s", err.Error())}
	}

	attachmentsCreated := 0
	warnings := make([]string, 0)
	processed := 0
	totalItems := len(stories) + len(checklistItems)

	for _, story := range stories {
		if story.Description != nil && strings.TrimSpace(*story.Description) != "" {
			rewritten, created, mediaWarnings := s.rewriteShortcutMediaBody(ctx, workspaceID, actorID, "story", story.ID, *story.Description, apiToken)
			attachmentsCreated += created
			warnings = appendUniqueWarnings(warnings, mediaWarnings)
			if rewritten != *story.Description {
				if err := s.db.WithContext(ctx).Model(&model.PMStory{}).Where("id = ?", story.ID).UpdateColumn("description", rewritten).Error; err != nil {
					warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to update imported story media for story %s: %s", story.ID, err.Error())})
				}
			}
		}

		processed++
		if processed%50 == 0 {
			_ = s.markStep(ctx, jobID, "story_media", stepNum, processed, totalSteps)
		}
	}

	for _, item := range checklistItems {
		if strings.TrimSpace(item.Text) != "" {
			rewritten, created, mediaWarnings := s.rewriteShortcutMediaText(ctx, workspaceID, actorID, "story", item.StoryID, item.Text, apiToken)
			attachmentsCreated += created
			warnings = appendUniqueWarnings(warnings, mediaWarnings)
			if rewritten != item.Text {
				if err := s.db.WithContext(ctx).Model(&model.PMChecklistItem{}).Where("id = ?", item.ID).UpdateColumn("text", rewritten).Error; err != nil {
					warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to update imported checklist media for item %s: %s", item.ID, err.Error())})
				}
			}
		}

		processed++
		if totalItems > 0 && processed%50 == 0 {
			_ = s.markStep(ctx, jobID, "story_media", stepNum, processed, totalSteps)
		}
	}

	return attachmentsCreated, warnings
}

func (s *PMImportService) rewriteShortcutMediaBody(ctx context.Context, workspaceID, actorID, entityType, entityID, body, apiToken string) (string, int, []string) {
	if strings.TrimSpace(body) == "" || !containsShortcutMediaCandidate(body) {
		return body, 0, nil
	}
	if apiToken == "" {
		return body, 0, nil
	}
	if s.attachmentService == nil || !s.attachmentService.SupportsPublicURL() {
		return body, 0, []string{shortcutMediaStorageWarning}
	}
	if s.mediaDownloader == nil {
		return body, 0, []string{"Shortcut-hosted media was found in imported content but no media downloader is configured"}
	}

	root := &html.Node{Type: html.ElementNode, Data: atom.Div.String(), DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(body), root)
	if err != nil {
		return body, 0, []string{fmt.Sprintf("Failed to parse imported %s content for media migration: %s", entityType, err.Error())}
	}

	replacements := make(map[string]string)
	failed := make(map[string]bool)
	attachmentsCreated := 0
	warnings := make([]string, 0)

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for idx := range node.Attr {
				key := strings.ToLower(node.Attr[idx].Key)
				if !shortcutMediaAttrApplies(node.Data, key) {
					continue
				}
				originalURL := strings.TrimSpace(node.Attr[idx].Val)
				if !isShortcutHostedMediaURL(originalURL) {
					continue
				}
				if replacementURL, ok := replacements[originalURL]; ok {
					node.Attr[idx].Val = replacementURL
					continue
				}
				if failed[originalURL] {
					continue
				}

				media, err := s.mediaDownloader.Download(ctx, originalURL, apiToken)
				if err != nil {
					warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to download Shortcut media %q: %s", shortenURL(originalURL), err.Error())})
					failed[originalURL] = true
					continue
				}

				fileName := chooseShortcutMediaFileName(preferredShortcutMediaFileName(node), media.FileName, originalURL, media.ContentType)
				resp, err := s.attachmentService.CreateImported(ctx, model.CreateAttachmentRequest{
					EntityType:  entityType,
					EntityID:    entityID,
					FileName:    fileName,
					FileSize:    int64(len(media.Data)),
					ContentType: media.ContentType,
				}, workspaceID, actorID, bytes.NewReader(media.Data))
				if err != nil {
					warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to store Shortcut media %q: %s", shortenURL(originalURL), err.Error())})
					failed[originalURL] = true
					continue
				}
				if strings.TrimSpace(resp.PublicURL) == "" {
					warnings = appendUniqueWarnings(warnings, []string{shortcutMediaStorageWarning})
					failed[originalURL] = true
					continue
				}

				replacements[originalURL] = resp.PublicURL
				node.Attr[idx].Val = resp.PublicURL
				attachmentsCreated++
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	for _, node := range nodes {
		walk(node)
	}
	if len(replacements) == 0 {
		return body, attachmentsCreated, warnings
	}

	var rendered bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&rendered, node); err != nil {
			return body, attachmentsCreated, appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to render imported %s content after media migration: %s", entityType, err.Error())})
		}
	}
	rewritten := strings.TrimSpace(rendered.String())
	if rewritten == "" {
		return body, attachmentsCreated, warnings
	}
	return rewritten, attachmentsCreated, warnings
}

func (s *PMImportService) rewriteShortcutMediaText(ctx context.Context, workspaceID, actorID, entityType, entityID, text, apiToken string) (string, int, []string) {
	if strings.TrimSpace(text) == "" || !containsShortcutMediaCandidate(text) {
		return text, 0, nil
	}
	if apiToken == "" {
		return text, 0, nil
	}
	if s.attachmentService == nil || !s.attachmentService.SupportsPublicURL() {
		return text, 0, []string{shortcutMediaStorageWarning}
	}
	if s.mediaDownloader == nil {
		return text, 0, []string{"Shortcut-hosted media was found in imported content but no media downloader is configured"}
	}

	matches := shortcutMediaURLRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return text, 0, nil
	}

	replacements := make(map[string]string)
	failed := make(map[string]bool)
	attachmentsCreated := 0
	warnings := make([]string, 0)

	for _, originalURL := range matches {
		if !isShortcutHostedMediaURL(originalURL) {
			continue
		}
		if _, ok := replacements[originalURL]; ok {
			continue
		}
		if failed[originalURL] {
			continue
		}

		media, err := s.mediaDownloader.Download(ctx, originalURL, apiToken)
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to download Shortcut media %q: %s", shortenURL(originalURL), err.Error())})
			failed[originalURL] = true
			continue
		}

		fileName := chooseShortcutMediaFileName(preferredShortcutMediaFileNameFromText(text, originalURL), media.FileName, originalURL, media.ContentType)
		resp, err := s.attachmentService.CreateImported(ctx, model.CreateAttachmentRequest{
			EntityType:  entityType,
			EntityID:    entityID,
			FileName:    fileName,
			FileSize:    int64(len(media.Data)),
			ContentType: media.ContentType,
		}, workspaceID, actorID, bytes.NewReader(media.Data))
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Failed to store Shortcut media %q: %s", shortenURL(originalURL), err.Error())})
			failed[originalURL] = true
			continue
		}
		if strings.TrimSpace(resp.PublicURL) == "" {
			warnings = appendUniqueWarnings(warnings, []string{shortcutMediaStorageWarning})
			failed[originalURL] = true
			continue
		}

		replacements[originalURL] = resp.PublicURL
		attachmentsCreated++
	}

	if len(replacements) == 0 {
		return text, attachmentsCreated, warnings
	}

	rewritten := text
	for originalURL, replacementURL := range replacements {
		rewritten = strings.ReplaceAll(rewritten, originalURL, replacementURL)
	}
	return rewritten, attachmentsCreated, warnings
}

func containsShortcutMediaCandidate(body string) bool {
	lowered := strings.ToLower(body)
	return strings.Contains(lowered, "media.app.shortcut.com") || strings.Contains(lowered, "api.app.shortcut.com/api/v3/files/")
}

func isShortcutHostedMediaURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	pathValue := strings.ToLower(u.Path)
	switch host {
	case "media.app.shortcut.com":
		return strings.Contains(pathValue, "/attachments/files/")
	case "api.app.shortcut.com":
		return strings.HasPrefix(pathValue, "/api/v3/files/")
	default:
		return false
	}
}

func shortcutMediaAttrApplies(nodeName, attrKey string) bool {
	switch strings.ToLower(nodeName) {
	case "img":
		return attrKey == "src"
	case "a":
		return attrKey == "href"
	default:
		return false
	}
}

func preferredShortcutMediaFileName(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Data == "img" {
		if alt := htmlAttr(node, "alt"); alt != "" {
			return sanitizeImportedFileName(alt)
		}
	}
	if title := htmlAttr(node, "title"); title != "" {
		return sanitizeImportedFileName(title)
	}
	text := strings.TrimSpace(nodeTextContent(node))
	if text != "" && !strings.Contains(text, "://") {
		return sanitizeImportedFileName(text)
	}
	return ""
}

func preferredShortcutMediaFileNameFromText(text, rawURL string) string {
	for _, prefix := range []string{`!\[`, `\[`} {
		pattern := prefix + `([^\]]*)\]\(` + regexp.QuoteMeta(rawURL) + `\)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) == 2 {
			if fileName := sanitizeImportedFileName(matches[1]); fileName != "" {
				return fileName
			}
		}
	}
	return ""
}

func htmlAttr(node *html.Node, key string) string {
	key = strings.ToLower(key)
	for _, attr := range node.Attr {
		if strings.ToLower(attr.Key) == key {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func nodeTextContent(node *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			buf.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return buf.String()
}

func chooseShortcutMediaFileName(preferred, downloaded, rawURL, contentType string) string {
	for _, candidate := range []string{preferred, downloaded, fileNameFromRawURL(rawURL)} {
		name := sanitizeImportedFileName(candidate)
		if name == "" {
			continue
		}
		if path.Ext(name) == "" {
			if ext := fileExtensionForContentType(contentType); ext != "" {
				name += ext
			}
		}
		return name
	}
	ext := fileExtensionForContentType(contentType)
	if ext == "" {
		ext = ".bin"
	}
	return "shortcut-media" + ext
}

func fileNameFromRawURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return sanitizeImportedFileName(path.Base(u.Path))
}

func sanitizeImportedFileName(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"`)
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = path.Base(raw)
	raw = strings.Map(func(r rune) rune {
		switch {
		case r < 32:
			return -1
		case strings.ContainsRune(`/:*?"<>|`, r):
			return '-'
		default:
			return r
		}
	}, raw)
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "." {
		return ""
	}
	return raw
}

func normalizeMediaType(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(value)
	}
	return strings.ToLower(mediaType)
}

func downloadFileNameFromHeader(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(raw)
	if err != nil {
		return ""
	}
	if value := sanitizeImportedFileName(params["filename"]); value != "" {
		return value
	}
	if value := sanitizeImportedFileName(params["filename*"]); value != "" {
		return value
	}
	return ""
}

func fileExtensionForContentType(contentType string) string {
	extensions, err := mime.ExtensionsByType(contentType)
	if err != nil || len(extensions) == 0 {
		return ""
	}
	return extensions[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func shortenURL(rawURL string) string {
	if len(rawURL) <= 120 {
		return rawURL
	}
	return rawURL[:117] + "..."
}

func appendUniqueWarnings(existing, additions []string) []string {
	for _, warning := range additions {
		if strings.TrimSpace(warning) == "" {
			continue
		}
		duplicate := false
		for _, current := range existing {
			if current == warning {
				duplicate = true
				break
			}
		}
		if !duplicate {
			existing = append(existing, warning)
		}
	}
	return existing
}
