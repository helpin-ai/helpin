package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	ToolPublishPreview      = "publish_preview"
	ToolPreviewMarkdown     = "preview_md"
	ToolPreviewJSON         = "preview_json"
	ToolPublishPRDDraft     = "publish_prd_draft"
	ToolPublishTaskPlan     = "publish_task_plan"
	ToolPublishTaskPlanDoc  = "publish_task_plan_doc"
	ToolPublishStoryPlan    = "publish_story_plan"
	ToolPublishStoryPlanDoc = "publish_story_plan_doc"

	PreviewFormatMarkdown = "markdown"
	PreviewFormatJSON     = "json"

	RunPreviewArtifactType = "run_preview"
)

type PublishedPreviewRequest struct {
	Slot     string          `json:"slot,omitempty"`
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
	req, err := decodePublishedPreviewRequest(input)
	return executePreviewToolRequest(ctx, ToolPublishPreview, req, err)
}

func toolPreviewMarkdown(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := buildSlotPreviewRequest(input, PreviewFormatMarkdown)
	return executePreviewToolRequest(ctx, ToolPreviewMarkdown, req, err)
}

func toolPreviewJSON(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := buildSlotPreviewRequest(input, PreviewFormatJSON)
	return executePreviewToolRequest(ctx, ToolPreviewJSON, req, err)
}

func toolPublishPRDDraft(_ *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := buildFixedPreviewRequest(input, "prd_draft", PreviewFormatMarkdown)
	return executePreviewToolRequest(nil, ToolPublishPRDDraft, req, err)
}

func toolPublishTaskPlan(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := buildFixedPreviewRequest(input, "task_plan", PreviewFormatJSON)
	if err == nil && previewContentLooksJSONString(req.Content) {
		err = fmt.Errorf("task plan content must be a JSON object with summary and proposed_tasks")
	}
	return executePreviewToolRequest(ctx, ToolPublishTaskPlan, req, err)
}

func toolPublishStoryPlan(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	return toolPublishTaskPlan(ctx, input)
}

func toolPublishTaskPlanDoc(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	req, err := buildFixedPreviewRequest(input, "task_plan_doc", PreviewFormatMarkdown)
	return executePreviewToolRequest(ctx, ToolPublishTaskPlanDoc, req, err)
}

func toolPublishStoryPlanDoc(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	return toolPublishTaskPlanDoc(ctx, input)
}

func executePreviewToolRequest(ctx *ExecutionContext, toolName string, req PublishedPreviewRequest, err error) (string, error) {
	if err != nil {
		return "", wrapPreviewToolError(toolName, err)
	}
	req = applyPreviewContentFallback(ctx, toolName, req)
	preview, err := normalizePublishedPreviewRequest(&req)
	if err != nil {
		return "", wrapPreviewToolError(toolName, err)
	}
	cachePublishedPreview(ctx, *preview)

	return toCompactJSONString(map[string]any{
		"status":    "published",
		"panel_key": preview.PanelKey,
		"title":     preview.Title,
		"format":    preview.Format,
		"replace":   preview.Replace,
	}), nil
}

func wrapPreviewToolError(toolName string, err error) error {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	switch {
	case strings.HasPrefix(message, "parse input:"):
		return fmt.Errorf("%s input must be valid JSON: %s", toolName, strings.TrimSpace(strings.TrimPrefix(message, "parse input:")))
	case message == "preview payload is required":
		return fmt.Errorf("%s input must be a JSON object", toolName)
	case message == "panel_key is required":
		switch toolName {
		case ToolPreviewMarkdown, ToolPreviewJSON:
			return fmt.Errorf("%s is missing slot; include \"slot\" to choose the preview panel", toolName)
		case ToolPublishPreview:
			return fmt.Errorf("%s is missing panel_key; include \"panel_key\" or use preview_md/preview_json with \"slot\"", toolName)
		default:
			return fmt.Errorf("%s could not determine which preview panel to publish; include the expected content and title", toolName)
		}
	case message == "title is required":
		return fmt.Errorf("%s is missing title; include \"title\"", toolName)
	case message == "raw tool arguments wrapper is not supported":
		return fmt.Errorf("%s input must be a JSON object with structured fields; do not send a raw string wrapper", toolName)
	case message == "content is required", message == "markdown content is required":
		return fmt.Errorf("%s is missing content; %s", toolName, previewToolContentHint(toolName))
	case message == "markdown content must be a string":
		return fmt.Errorf("%s content must be a markdown string in \"content\"", toolName)
	case message == "json content must be valid JSON":
		return fmt.Errorf("%s content must be valid JSON in \"content\"", toolName)
	case message == "task plan content is empty":
		return fmt.Errorf("%s is missing content; include the task plan JSON object in \"content\"", toolName)
	case message == "task plan content must be a JSON object with summary and proposed_tasks":
		return fmt.Errorf("%s content must be a JSON object with summary and proposed_tasks", toolName)
	case message == "task plan content must include a non-empty summary":
		return fmt.Errorf("%s content must include a non-empty summary", toolName)
	case message == "task plan content must include proposed_tasks as an array":
		return fmt.Errorf("%s content must include proposed_tasks as an array", toolName)
	case message == "task plan content must include at least one proposed task":
		return fmt.Errorf("%s content must include at least one proposed task", toolName)
	case message == "task plan content proposed_tasks entries must be task objects with fields like name, description, task_type, acceptance_criteria, and dependency_refs":
		return fmt.Errorf("%s requires content.proposed_tasks to be an array of task objects, for example [{\"name\":\"...\",\"description\":\"...\",\"task_type\":\"feature\",\"acceptance_criteria\":[\"...\"],\"dependency_refs\":[]}]", toolName)
	case strings.HasPrefix(message, "task plan proposed_tasks are invalid:"):
		return fmt.Errorf("%s %s", toolName, strings.TrimSpace(strings.TrimPrefix(message, "task plan")))
	case strings.HasPrefix(message, "format must be"):
		return fmt.Errorf("%s format must be %q or %q", toolName, PreviewFormatMarkdown, PreviewFormatJSON)
	default:
		return err
	}
}

func previewToolContentHint(toolName string) string {
	switch toolName {
	case ToolPreviewMarkdown, ToolPublishPRDDraft, ToolPublishTaskPlanDoc, ToolPublishStoryPlanDoc:
		return "include markdown in \"content\""
	case ToolPreviewJSON:
		return "include a JSON value in \"content\""
	case ToolPublishTaskPlan, ToolPublishStoryPlan:
		return "include the task plan JSON object in \"content\""
	case ToolPublishPreview:
		return "include markdown or JSON in \"content\""
	default:
		return "include the preview body in \"content\""
	}
}

func applyPreviewContentFallback(ctx *ExecutionContext, toolName string, req PublishedPreviewRequest) PublishedPreviewRequest {
	if !previewToolAllowsContentFallback(toolName) {
		return req
	}
	if len(req.Content) != 0 && string(req.Content) != "null" {
		return req
	}

	panelKey := normalizePreviewPanelKey(req.PanelKey)
	if panelKey == "" {
		panelKey = normalizePreviewPanelKey(req.Slot)
	}
	if panelKey == "" {
		return req
	}

	previous := latestPublishedPreviewForContext(ctx, panelKey)
	if previous != nil && len(previous.Content) != 0 && string(previous.Content) != "null" {
		req.Content = append(json.RawMessage(nil), previous.Content...)
		if strings.TrimSpace(req.Title) == "" {
			req.Title = previous.Title
		}
		if strings.TrimSpace(req.Format) == "" {
			req.Format = previous.Format
		}
		if req.Replace == nil {
			replace := previous.Replace
			req.Replace = &replace
		}
		return req
	}

	format := normalizePreviewFormat(req.Format)
	if format == PreviewFormatMarkdown && ctx != nil {
		if draft := strings.TrimSpace(ctx.CurrentAssistantText); draft != "" {
			req.Content, _ = json.Marshal(draft)
			if strings.TrimSpace(req.Title) == "" {
				req.Title = defaultPreviewTitle(panelKey)
			}
			return req
		}
		if draft, ok := latestTaskPlanDocumentMarkdown(ctx, panelKey); ok {
			req.Content, _ = json.Marshal(draft)
			if strings.TrimSpace(req.Title) == "" {
				req.Title = defaultPreviewTitle(panelKey)
			}
		}
	}
	return req
}

func previewToolAllowsContentFallback(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case ToolPublishTaskPlan, ToolPublishStoryPlan:
		return false
	default:
		return true
	}
}

func latestTaskPlanDocumentMarkdown(ctx *ExecutionContext, panelKey string) (string, bool) {
	if ctx == nil || ctx.Task == nil || ctx.Services == nil || ctx.Services.GetDocumentContent == nil {
		return "", false
	}
	if normalizePreviewPanelKey(panelKey) != "task_plan_doc" {
		return "", false
	}
	documentID := ""
	if ctx.Task.PlanDocumentID != nil {
		documentID = strings.TrimSpace(*ctx.Task.PlanDocumentID)
	}
	if documentID == "" {
		return "", false
	}
	content, err := ctx.Services.GetDocumentContent(ctx.Context, documentID)
	if err != nil {
		return "", false
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", false
	}
	return content, true
}

func cachePublishedPreview(ctx *ExecutionContext, preview PublishedPreview) {
	if ctx == nil || strings.TrimSpace(preview.PanelKey) == "" {
		return
	}
	if ctx.PublishedPreviews == nil {
		ctx.PublishedPreviews = make(map[string]PublishedPreview)
	}
	ctx.PublishedPreviews[normalizePreviewPanelKey(preview.PanelKey)] = preview
}

func latestPublishedPreviewForContext(ctx *ExecutionContext, panelKey string) *PublishedPreview {
	if ctx == nil {
		return nil
	}
	panelKey = normalizePreviewPanelKey(panelKey)
	if panelKey == "" {
		return nil
	}
	if ctx.PublishedPreviews != nil {
		if preview, ok := ctx.PublishedPreviews[panelKey]; ok {
			copied := preview
			return &copied
		}
	}

	invocations := make([]appmodel.ToolInvocation, 0)
	for _, message := range ctx.ConversationHistory {
		for _, block := range message.Blocks {
			if block.Type != ExecutionBlockTypeToolResult || !isPreviewToolName(block.ToolName) || len(block.Input) == 0 {
				continue
			}
			invocations = append(invocations, appmodel.ToolInvocation{
				ToolName: block.ToolName,
				Input:    block.Input,
			})
		}
	}
	return ExtractLatestPublishedPreview(invocations, panelKey)
}

func ExtractPublishedPreviews(toolInvocations []appmodel.ToolInvocation) []PublishedPreview {
	previews := make([]PublishedPreview, 0, len(toolInvocations))
	for _, invocation := range toolInvocations {
		if !isPreviewToolName(invocation.ToolName) {
			continue
		}
		preview, err := previewFromToolInvocation(invocation)
		if err != nil {
			continue
		}
		previews = append(previews, *preview)
	}
	return previews
}

func isPreviewToolName(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case ToolPublishPreview, ToolPreviewMarkdown, ToolPreviewJSON, ToolPublishPRDDraft, ToolPublishTaskPlan, ToolPublishTaskPlanDoc, ToolPublishStoryPlan, ToolPublishStoryPlanDoc:
		return true
	default:
		return false
	}
}

func previewFromToolInvocation(invocation appmodel.ToolInvocation) (*PublishedPreview, error) {
	switch strings.TrimSpace(invocation.ToolName) {
	case ToolPreviewMarkdown:
		req, err := buildSlotPreviewRequest(invocation.Input, PreviewFormatMarkdown)
		if err != nil {
			return nil, err
		}
		return normalizePublishedPreviewRequest(&req)
	case ToolPreviewJSON:
		req, err := buildSlotPreviewRequest(invocation.Input, PreviewFormatJSON)
		if err != nil {
			return nil, err
		}
		return normalizePublishedPreviewRequest(&req)
	case ToolPublishPRDDraft:
		req, err := buildFixedPreviewRequest(invocation.Input, "prd_draft", PreviewFormatMarkdown)
		if err != nil {
			return nil, err
		}
		return normalizePublishedPreviewRequest(&req)
	case ToolPublishTaskPlan, ToolPublishStoryPlan:
		req, err := buildFixedPreviewRequest(invocation.Input, "task_plan", PreviewFormatJSON)
		if err != nil {
			return nil, err
		}
		return normalizePublishedPreviewRequest(&req)
	case ToolPublishTaskPlanDoc, ToolPublishStoryPlanDoc:
		req, err := buildFixedPreviewRequest(invocation.Input, "task_plan_doc", PreviewFormatMarkdown)
		if err != nil {
			return nil, err
		}
		return normalizePublishedPreviewRequest(&req)
	default:
		req, err := decodePublishedPreviewRequest(invocation.Input)
		if err != nil {
			return nil, err
		}
		return normalizePublishedPreviewRequestWithContext(nil, &req)
	}
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
	return normalizePublishedPreviewRequestWithContext(nil, req)
}

func normalizePublishedPreviewRequestWithContext(ctx *ExecutionContext, req *PublishedPreviewRequest) (*PublishedPreview, error) {
	if req == nil {
		return nil, fmt.Errorf("preview payload is required")
	}

	panelKey := normalizePreviewPanelKey(req.PanelKey)
	if panelKey == "" {
		panelKey = normalizePreviewPanelKey(req.Slot)
	}
	if panelKey == "" {
		panelKey = inferPreviewPanelKeyFromContext(ctx, req)
	}
	if panelKey == "" {
		panelKey = inferPreviewPanelKey(req)
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
		format = inferPreviewFormat(req)
	}
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

	return &PublishedPreview{
		PanelKey: panelKey,
		Title:    title,
		Format:   format,
		Content:  content,
		Replace:  replace,
	}, nil
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
		var legacy PublishedPreviewRequest
		if err := json.Unmarshal(input, &legacy); err == nil && strings.TrimSpace(legacy.PanelKey) != "" {
			req.Slot = legacy.PanelKey
		}
	}
	if strings.TrimSpace(req.Slot) == "" {
		for _, key := range []string{"panel_key", "panelKey"} {
			if value, ok := raw[key]; ok {
				_ = json.Unmarshal(value, &req.Slot)
				if strings.TrimSpace(req.Slot) != "" {
					break
				}
			}
		}
	}
	if strings.TrimSpace(req.Title) == "" {
		for _, key := range []string{"panelTitle"} {
			if value, ok := raw[key]; ok {
				_ = json.Unmarshal(value, &req.Title)
				if strings.TrimSpace(req.Title) != "" {
					break
				}
			}
		}
	}
	if len(req.Content) == 0 {
		req.Content = extractPreviewContent(raw, format)
	}
	return PublishedPreviewRequest{
		Slot:    req.Slot,
		Title:   req.Title,
		Format:  format,
		Content: req.Content,
		Replace: req.Replace,
	}, nil
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
		for _, key := range []string{"panelTitle"} {
			if value, ok := raw[key]; ok {
				_ = json.Unmarshal(value, &req.Title)
				if strings.TrimSpace(req.Title) != "" {
					break
				}
			}
		}
	}
	if len(req.Content) == 0 {
		req.Content = extractPreviewContent(raw, format)
	}
	return PublishedPreviewRequest{
		PanelKey: panelKey,
		Title:    req.Title,
		Format:   format,
		Content:  req.Content,
		Replace:  req.Replace,
	}, nil
}

func previewObjectMap(input json.RawMessage) map[string]json.RawMessage {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(input, &raw); err != nil {
		return nil
	}
	return raw
}

func extractPreviewContent(raw map[string]json.RawMessage, format string) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	for _, key := range []string{"content", "body", "markdown", "text"} {
		if value, ok := raw[key]; ok && len(value) != 0 && string(value) != "null" {
			return append(json.RawMessage(nil), value...)
		}
	}

	for _, nestedKey := range []string{"preview", "payload"} {
		nested, ok := raw[nestedKey]
		if !ok || len(nested) == 0 || string(nested) == "null" {
			continue
		}
		nestedRaw := previewObjectMap(nested)
		if content := extractPreviewContent(nestedRaw, format); len(content) != 0 {
			return content
		}
		if format == PreviewFormatJSON {
			if stripped := stripPreviewMetaFields(nestedRaw); len(stripped) != 0 {
				return stripped
			}
		}
	}

	if format == PreviewFormatJSON {
		if stripped := stripPreviewMetaFields(raw); len(stripped) != 0 {
			return stripped
		}
	}

	return nil
}

func stripPreviewMetaFields(raw map[string]json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	metaKeys := map[string]bool{
		"slot":           true,
		"panel_key":      true,
		"panelKey":       true,
		"title":          true,
		"panelTitle":     true,
		"format":         true,
		"preview_format": true,
		"replace":        true,
		"content":        true,
		"body":           true,
		"markdown":       true,
		"text":           true,
		"raw":            true,
		"preview":        true,
		"payload":        true,
	}
	filtered := make(map[string]json.RawMessage)
	for key, value := range raw {
		if metaKeys[key] {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	normalized, _ := json.Marshal(filtered)
	return normalized
}

func isRawToolArgumentsWrapper(raw map[string]json.RawMessage) bool {
	if len(raw) != 1 {
		return false
	}
	value, ok := raw["raw"]
	if !ok || len(value) == 0 || string(value) == "null" {
		return false
	}
	var text string
	return json.Unmarshal(value, &text) == nil && strings.TrimSpace(text) != ""
}

func unwrapRawToolArguments(input json.RawMessage) (json.RawMessage, error) {
	raw := previewObjectMap(input)
	if !isRawToolArgumentsWrapper(raw) {
		return input, nil
	}
	var wrapped string
	if err := json.Unmarshal(raw["raw"], &wrapped); err != nil {
		return nil, fmt.Errorf("raw tool arguments wrapper is not supported")
	}
	wrapped = strings.TrimSpace(wrapped)
	if wrapped == "" || !json.Valid([]byte(wrapped)) {
		return nil, fmt.Errorf("raw tool arguments wrapper is not supported")
	}
	if wrapped[0] != '{' {
		return nil, fmt.Errorf("raw tool arguments wrapper is not supported")
	}
	return json.RawMessage(wrapped), nil
}

func normalizePreviewPanelKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
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

func inferPreviewPanelKeyFromContext(ctx *ExecutionContext, req *PublishedPreviewRequest) string {
	if ctx == nil || req == nil {
		return ""
	}

	format := normalizePreviewFormat(req.Format)
	if format == "" {
		format = inferPreviewFormat(req)
	}
	title := strings.ToLower(strings.TrimSpace(req.Title))

	presetKey := ""
	if ctx.Agent != nil {
		presetKey = strings.TrimSpace(ctx.Agent.EffectivePresetKey())
	}

	switch {
	case strings.TrimSpace(ctx.PlanningStage) == appmodel.PlanningStageTaskPlanDoc:
		if format == PreviewFormatMarkdown || strings.Contains(title, "task plan") || strings.Contains(title, "planning doc") || strings.Contains(title, "planning document") {
			return "task_plan_doc"
		}
	case presetKey == appmodel.AgentPresetTaskPlanner:
		if format == PreviewFormatMarkdown || strings.Contains(title, "task plan") || strings.Contains(title, "planning doc") || strings.Contains(title, "planning document") {
			return "task_plan_doc"
		}
	case presetKey == appmodel.AgentPresetEpicPlanner || ctx.Epic != nil || strings.TrimSpace(ctx.TargetType) == "epic":
		switch format {
		case PreviewFormatMarkdown:
			return "prd_draft"
		case PreviewFormatJSON:
			return "task_plan"
		}
		switch {
		case strings.Contains(title, "prd"):
			return "prd_draft"
		case strings.Contains(title, "task plan"):
			return "task_plan"
		}
	}

	return ""
}

func decodePublishedPreviewRequest(input json.RawMessage) (PublishedPreviewRequest, error) {
	var req PublishedPreviewRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return PublishedPreviewRequest{}, fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(req.PanelKey) != "" {
		return req, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(input, &raw); err != nil {
		return req, nil
	}

	if alias, ok := raw["panelKey"]; ok && strings.TrimSpace(req.PanelKey) == "" {
		_ = json.Unmarshal(alias, &req.PanelKey)
	}
	if strings.TrimSpace(req.Title) == "" {
		if alias, ok := raw["panelTitle"]; ok {
			_ = json.Unmarshal(alias, &req.Title)
		}
	}
	if strings.TrimSpace(req.Format) == "" {
		if alias, ok := raw["preview_format"]; ok {
			_ = json.Unmarshal(alias, &req.Format)
		}
	}
	if len(req.Content) == 0 {
		if alias, ok := raw["body"]; ok {
			req.Content = append(json.RawMessage(nil), alias...)
		}
	}

	if strings.TrimSpace(req.PanelKey) != "" {
		return req, nil
	}

	for _, nestedKey := range []string{"preview", "payload"} {
		nested, ok := raw[nestedKey]
		if !ok || len(nested) == 0 || string(nested) == "null" {
			continue
		}
		nestedReq, err := decodePublishedPreviewRequest(nested)
		if err == nil && strings.TrimSpace(nestedReq.PanelKey) != "" {
			return nestedReq, nil
		}
	}

	return req, nil
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

func previewContentLooksJSONString(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var text string
	return json.Unmarshal(raw, &text) == nil
}

func inferPreviewPanelKey(req *PublishedPreviewRequest) string {
	if req == nil {
		return ""
	}

	format := normalizePreviewFormat(req.Format)
	title := strings.ToLower(strings.TrimSpace(req.Title))

	var content map[string]any
	hasJSONContent := json.Unmarshal(req.Content, &content) == nil

	switch format {
	case PreviewFormatJSON:
		if hasJSONContent {
			if _, ok := content["proposed_tasks"]; ok {
				return "task_plan"
			}
		}
		if strings.Contains(title, "task plan") {
			return "task_plan"
		}
	case PreviewFormatMarkdown:
		switch {
		case strings.Contains(title, "prd"):
			return "prd_draft"
		case strings.Contains(title, "task plan"):
			return "task_plan_doc"
		}
	}

	if hasJSONContent {
		if _, ok := content["proposed_tasks"]; ok {
			return "task_plan"
		}
	}
	switch {
	case strings.Contains(title, "prd"):
		return "prd_draft"
	case strings.Contains(title, "task plan"):
		return "task_plan_doc"
	}

	return ""
}

func inferPreviewFormat(req *PublishedPreviewRequest) string {
	if req == nil || len(req.Content) == 0 || string(req.Content) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(req.Content, &text); err == nil {
		if strings.TrimSpace(text) != "" {
			return PreviewFormatMarkdown
		}
	}

	var content any
	if err := json.Unmarshal(req.Content, &content); err == nil {
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
