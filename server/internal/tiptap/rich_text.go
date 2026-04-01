package tiptap

import (
	"encoding/json"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type markdownSourceEnvelope struct {
	Markdown string `json:"_markdown_source"`
}

type tiptapDocumentProbe struct {
	Type string `json:"type"`
}

// RichTextToMarkdown normalizes stored rich-text values into markdown for prompts.
func RichTextToMarkdown(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	if markdown, ok := markdownSourceToMarkdown(trimmed); ok {
		return markdown
	}

	if markdown, ok := tiptapJSONToMarkdown(trimmed); ok {
		return markdown
	}

	if looksLikeHTML(trimmed) {
		if markdown, err := htmltomarkdown.ConvertString(trimmed); err == nil {
			if normalized := strings.TrimSpace(markdown); normalized != "" {
				return normalized
			}
		}
	}

	return trimmed
}

func markdownSourceToMarkdown(input string) (string, bool) {
	var envelope markdownSourceEnvelope
	if err := json.Unmarshal([]byte(input), &envelope); err != nil {
		return "", false
	}
	markdown := strings.TrimSpace(envelope.Markdown)
	return markdown, markdown != ""
}

func tiptapJSONToMarkdown(input string) (string, bool) {
	var probe tiptapDocumentProbe
	if err := json.Unmarshal([]byte(input), &probe); err != nil {
		return "", false
	}
	if probe.Type != "doc" {
		return "", false
	}

	rendered, err := RenderHTML(json.RawMessage(input))
	if err != nil || strings.TrimSpace(rendered) == "" {
		return "", false
	}
	markdown, err := htmltomarkdown.ConvertString(rendered)
	if err != nil {
		return "", false
	}
	normalized := strings.TrimSpace(markdown)
	return normalized, normalized != ""
}

func looksLikeHTML(input string) bool {
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
