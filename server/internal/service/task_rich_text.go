package service

import (
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func renderTaskDescriptionRichText(markdown string) string {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return ""
	}
	comments := extractHTMLComments(trimmed)
	rendered, err := tiptap.RenderHTML(tiptap.MarkdownToJSON(trimmed))
	if err != nil {
		slog.Warn("failed to render task markdown to html", "error", err)
		return trimmed
	}
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return trimmed
	}
	if len(comments) > 0 {
		rendered = strings.Join(comments, "") + rendered
	}
	return rendered
}

func normalizeTaskDescriptionRichText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	if looksLikeStoredTaskRichText(trimmed) {
		return strPtr(trimmed)
	}
	return strPtr(renderTaskDescriptionRichText(trimmed))
}

func looksLikeStoredTaskRichText(input string) bool {
	if !strings.Contains(input, "<") || !strings.Contains(input, ">") {
		return false
	}
	lower := strings.ToLower(input)
	for _, marker := range []string{
		"<p", "<div", "<span", "<strong", "<em", "<ul", "<ol", "<li",
		"<h1", "<h2", "<h3", "<h4", "<h5", "<h6", "<br", "<a", "<blockquote",
		"<code", "<pre", "<img", "<table",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
