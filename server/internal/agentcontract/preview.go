package agentcontract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	PreviewFormatMarkdown  = "markdown"
	PreviewFormatJSON      = "json"
	RunPreviewArtifactType = "run_preview"
)

// PublishedPreviewRequest is the persisted input contract for preview tools.
type PublishedPreviewRequest struct {
	Slot     string          `json:"slot,omitempty"`
	PanelKey string          `json:"panel_key"`
	Title    string          `json:"title"`
	Format   string          `json:"format"`
	Content  json.RawMessage `json:"content"`
	Replace  *bool           `json:"replace,omitempty"`
}

// PublishedPreview is the normalized preview persisted by Helpin.
type PublishedPreview struct {
	PanelKey string          `json:"panel_key"`
	Title    string          `json:"title"`
	Format   string          `json:"format"`
	Content  json.RawMessage `json:"content"`
	Replace  bool            `json:"replace"`
}

// ExtractPublishedPreviews decodes preview contracts from persisted tool calls.
func ExtractPublishedPreviews(invocations []model.ToolInvocation) []PublishedPreview {
	previews := make([]PublishedPreview, 0, len(invocations))
	for _, invocation := range invocations {
		preview, err := previewFromToolInvocation(invocation)
		if err != nil {
			continue
		}
		previews = append(previews, *preview)
	}
	return previews
}

func previewFromToolInvocation(invocation model.ToolInvocation) (*PublishedPreview, error) {
	switch CanonicalToolName(invocation.ToolName) {
	case ToolPreviewMarkdown:
		return normalizePublishedPreviewRequest(buildSlotPreviewRequest(invocation.Input, PreviewFormatMarkdown))
	case ToolPreviewJSON:
		return normalizePublishedPreviewRequest(buildSlotPreviewRequest(invocation.Input, PreviewFormatJSON))
	case ToolPublishPRDDraft:
		return normalizePublishedPreviewRequest(buildFixedPreviewRequest(invocation.Input, "prd_draft", PreviewFormatMarkdown))
	case ToolPublishTaskPlan:
		return normalizePublishedPreviewRequest(buildFixedPreviewRequest(invocation.Input, "task_plan", PreviewFormatJSON))
	case ToolPublishTaskPlanDoc:
		return normalizePublishedPreviewRequest(buildFixedPreviewRequest(invocation.Input, "task_plan_doc", PreviewFormatMarkdown))
	case ToolPublishPreview:
		req, err := decodePublishedPreviewRequest(invocation.Input)
		return normalizePublishedPreviewRequest(req, err)
	default:
		return nil, fmt.Errorf("not a preview tool")
	}
}

func normalizePublishedPreviewRequest(req PublishedPreviewRequest, decodeErr error) (*PublishedPreview, error) {
	if decodeErr != nil {
		return nil, decodeErr
	}
	panelKey := normalizePreviewPanelKey(req.PanelKey)
	if panelKey == "" {
		panelKey = normalizePreviewPanelKey(req.Slot)
	}
	if panelKey == "" {
		panelKey = inferPreviewPanelKey(&req)
	}
	if panelKey == "" {
		return nil, fmt.Errorf("panel_key is required")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = defaultPreviewTitle(panelKey)
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	format := normalizePreviewFormat(req.Format)
	if format == "" {
		format = inferPreviewFormat(&req)
	}
	if format == "" {
		return nil, fmt.Errorf("format must be %q or %q", PreviewFormatMarkdown, PreviewFormatJSON)
	}
	content, err := normalizePreviewContent(format, req.Content)
	if err != nil {
		return nil, err
	}
	if panelKey == "task_plan" && format == PreviewFormatJSON {
		content, err = NormalizeTaskPlanPreviewContent(content)
		if err != nil {
			return nil, err
		}
	}
	replace := true
	if req.Replace != nil {
		replace = *req.Replace
	}
	return &PublishedPreview{PanelKey: panelKey, Title: title, Format: format, Content: content, Replace: replace}, nil
}

type slotPreviewRequest struct {
	Slot    string          `json:"slot"`
	Title   string          `json:"title"`
	Content json.RawMessage `json:"content"`
	Replace *bool           `json:"replace,omitempty"`
}

type fixedPreviewRequest struct {
	Title   string          `json:"title"`
	Content json.RawMessage `json:"content"`
	Replace *bool           `json:"replace,omitempty"`
}

func buildSlotPreviewRequest(input json.RawMessage, format string) (PublishedPreviewRequest, error) {
	input, err := unwrapRawToolArguments(input)
	if err != nil {
		return PublishedPreviewRequest{}, err
	}
	var req slotPreviewRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return PublishedPreviewRequest{}, fmt.Errorf("parse input: %w", err)
	}
	raw := previewObjectMap(input)
	if strings.TrimSpace(req.Slot) == "" {
		for _, key := range []string{"panel_key", "panelKey"} {
			_ = json.Unmarshal(raw[key], &req.Slot)
			if strings.TrimSpace(req.Slot) != "" {
				break
			}
		}
	}
	if strings.TrimSpace(req.Title) == "" {
		_ = json.Unmarshal(raw["panelTitle"], &req.Title)
	}
	if len(req.Content) == 0 {
		req.Content = extractPreviewContent(raw, format)
	}
	return PublishedPreviewRequest{Slot: req.Slot, Title: req.Title, Format: format, Content: req.Content, Replace: req.Replace}, nil
}

func buildFixedPreviewRequest(input json.RawMessage, panelKey, format string) (PublishedPreviewRequest, error) {
	input, err := unwrapRawToolArguments(input)
	if err != nil {
		return PublishedPreviewRequest{}, err
	}
	var req fixedPreviewRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return PublishedPreviewRequest{}, fmt.Errorf("parse input: %w", err)
	}
	raw := previewObjectMap(input)
	if strings.TrimSpace(req.Title) == "" {
		_ = json.Unmarshal(raw["panelTitle"], &req.Title)
	}
	if len(req.Content) == 0 {
		req.Content = extractPreviewContent(raw, format)
	}
	return PublishedPreviewRequest{PanelKey: panelKey, Title: req.Title, Format: format, Content: req.Content, Replace: req.Replace}, nil
}

func decodePublishedPreviewRequest(input json.RawMessage) (PublishedPreviewRequest, error) {
	input, err := unwrapRawToolArguments(input)
	if err != nil {
		return PublishedPreviewRequest{}, err
	}
	var req PublishedPreviewRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return PublishedPreviewRequest{}, fmt.Errorf("parse input: %w", err)
	}
	raw := previewObjectMap(input)
	if strings.TrimSpace(req.PanelKey) == "" {
		_ = json.Unmarshal(raw["panelKey"], &req.PanelKey)
	}
	if strings.TrimSpace(req.Title) == "" {
		_ = json.Unmarshal(raw["panelTitle"], &req.Title)
	}
	if strings.TrimSpace(req.Format) == "" {
		_ = json.Unmarshal(raw["preview_format"], &req.Format)
	}
	if len(req.Content) == 0 {
		req.Content = extractPreviewContent(raw, req.Format)
	}
	return req, nil
}

func previewObjectMap(input json.RawMessage) map[string]json.RawMessage {
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(input, &raw)
	return raw
}

func extractPreviewContent(raw map[string]json.RawMessage, format string) json.RawMessage {
	for _, key := range []string{"content", "body", "markdown", "text"} {
		if value := raw[key]; len(value) > 0 && string(value) != "null" {
			return append(json.RawMessage(nil), value...)
		}
	}
	for _, nestedKey := range []string{"preview", "payload"} {
		if nested := raw[nestedKey]; len(nested) > 0 && string(nested) != "null" {
			if content := extractPreviewContent(previewObjectMap(nested), format); len(content) > 0 {
				return content
			}
		}
	}
	if normalizePreviewFormat(format) == PreviewFormatJSON {
		filtered := make(map[string]json.RawMessage)
		for key, value := range raw {
			switch key {
			case "slot", "panel_key", "panelKey", "title", "panelTitle", "format", "preview_format", "replace", "raw", "preview", "payload":
			default:
				filtered[key] = value
			}
		}
		if len(filtered) > 0 {
			content, _ := json.Marshal(filtered)
			return content
		}
	}
	return nil
}

func unwrapRawToolArguments(input json.RawMessage) (json.RawMessage, error) {
	raw := previewObjectMap(input)
	if len(raw) != 1 || len(raw["raw"]) == 0 {
		return input, nil
	}
	var wrapped string
	if err := json.Unmarshal(raw["raw"], &wrapped); err != nil {
		return input, nil
	}
	wrapped = strings.TrimSpace(wrapped)
	if !json.Valid([]byte(wrapped)) || !strings.HasPrefix(wrapped, "{") {
		return nil, fmt.Errorf("raw tool arguments wrapper is not supported")
	}
	return json.RawMessage(wrapped), nil
}

func normalizePreviewContent(format string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("content is required")
	}
	switch format {
	case PreviewFormatMarkdown:
		var content string
		if err := json.Unmarshal(raw, &content); err != nil || strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("markdown content must be a non-empty string")
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

func normalizePreviewPanelKey(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

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

func inferPreviewPanelKey(req *PublishedPreviewRequest) string {
	if req == nil {
		return ""
	}
	var content map[string]any
	hasJSON := json.Unmarshal(req.Content, &content) == nil
	if hasJSON {
		if _, ok := content["proposed_tasks"]; ok {
			return "task_plan"
		}
	}
	title := strings.ToLower(strings.TrimSpace(req.Title))
	if strings.Contains(title, "prd") {
		return "prd_draft"
	}
	if strings.Contains(title, "task plan") {
		if normalizePreviewFormat(req.Format) == PreviewFormatJSON {
			return "task_plan"
		}
		return "task_plan_doc"
	}
	return ""
}

func inferPreviewFormat(req *PublishedPreviewRequest) string {
	if req == nil || len(req.Content) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(req.Content, &text) == nil && strings.TrimSpace(text) != "" {
		return PreviewFormatMarkdown
	}
	var content any
	if json.Unmarshal(req.Content, &content) == nil {
		return PreviewFormatJSON
	}
	return ""
}

func defaultPreviewTitle(panelKey string) string {
	switch normalizePreviewPanelKey(panelKey) {
	case "prd_draft":
		return "PRD Draft"
	case "task_plan":
		return "Task Plan"
	case "task_plan_doc":
		return "Task Planning Document"
	default:
		return ""
	}
}
