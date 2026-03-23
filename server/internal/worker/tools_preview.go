package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ToolPublishPreview = "publish_preview"

	PreviewFormatMarkdown = "markdown"
	PreviewFormatJSON     = "json"

	RunPreviewArtifactType = "run_preview"
)

type PublishedPreviewRequest struct {
	PanelKey string          `json:"panel_key"`
	Title    string          `json:"title"`
	Format   string          `json:"format"`
	Content  json.RawMessage `json:"content"`
	Replace  *bool           `json:"replace,omitempty"`
}

type PublishedPreview struct {
	PanelKey string          `json:"panel_key"`
	Title    string          `json:"title"`
	Format   string          `json:"format"`
	Content  json.RawMessage `json:"content"`
	Replace  bool            `json:"replace"`
}

func toolPublishPreview(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var req PublishedPreviewRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	preview, err := normalizePublishedPreviewRequest(&req)
	if err != nil {
		return "", err
	}

	payload, _ := json.MarshalIndent(map[string]any{
		"status":    "published",
		"panel_key": preview.PanelKey,
		"title":     preview.Title,
		"format":    preview.Format,
		"replace":   preview.Replace,
	}, "", "  ")
	return string(payload), nil
}

func ExtractPublishedPreviews(toolInvocations []appmodel.ToolInvocation) []PublishedPreview {
	previews := make([]PublishedPreview, 0, len(toolInvocations))
	for _, invocation := range toolInvocations {
		if strings.TrimSpace(invocation.ToolName) != ToolPublishPreview {
			continue
		}

		var req PublishedPreviewRequest
		if err := json.Unmarshal(invocation.Input, &req); err != nil {
			continue
		}
		preview, err := normalizePublishedPreviewRequest(&req)
		if err != nil {
			continue
		}
		previews = append(previews, *preview)
	}
	return previews
}

func ExtractLatestPublishedPreview(toolInvocations []appmodel.ToolInvocation, panelKey string) *PublishedPreview {
	previews := ExtractPublishedPreviews(toolInvocations)
	panelKey = normalizePreviewPanelKey(panelKey)
	for i := len(previews) - 1; i >= 0; i-- {
		if panelKey != "" && previews[i].PanelKey != panelKey {
			continue
		}
		preview := previews[i]
		return &preview
	}
	return nil
}

func normalizePublishedPreviewRequest(req *PublishedPreviewRequest) (*PublishedPreview, error) {
	if req == nil {
		return nil, fmt.Errorf("preview payload is required")
	}

	panelKey := normalizePreviewPanelKey(req.PanelKey)
	if panelKey == "" {
		return nil, fmt.Errorf("panel_key is required")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	format := normalizePreviewFormat(req.Format)
	if format == "" {
		return nil, fmt.Errorf("format must be %q or %q", PreviewFormatMarkdown, PreviewFormatJSON)
	}

	if len(req.Content) == 0 || string(req.Content) == "null" {
		return nil, fmt.Errorf("content is required")
	}

	content, err := normalizePreviewContent(format, req.Content)
	if err != nil {
		return nil, err
	}

	replace := true
	if req.Replace != nil {
		replace = *req.Replace
	}

	return &PublishedPreview{
		PanelKey: panelKey,
		Title:    title,
		Format:   format,
		Content:  content,
		Replace:  replace,
	}, nil
}

func normalizePreviewPanelKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePreviewFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PreviewFormatMarkdown:
		return PreviewFormatMarkdown
	case PreviewFormatJSON:
		return PreviewFormatJSON
	default:
		return ""
	}
}

func normalizePreviewContent(format string, raw json.RawMessage) (json.RawMessage, error) {
	switch format {
	case PreviewFormatMarkdown:
		var content string
		if err := json.Unmarshal(raw, &content); err != nil {
			return nil, fmt.Errorf("markdown content must be a string")
		}
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("markdown content is required")
		}
		normalized, _ := json.Marshal(content)
		return normalized, nil
	case PreviewFormatJSON:
		var content any
		if err := json.Unmarshal(raw, &content); err != nil {
			return nil, fmt.Errorf("json content must be valid JSON")
		}
		normalized, _ := json.Marshal(content)
		return normalized, nil
	default:
		return nil, fmt.Errorf("unsupported preview format %q", format)
	}
}
