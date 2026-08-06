package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// registerDocsRuntimeToolCommands registers command-backed variants of the
// native docs search and product publish tools.
func (s *InternalCommandService) registerDocsRuntimeToolCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "docs.insert_document_image",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "support_coverage_gap"},
		Tool:                 mustCommandToolMetadata("docs.insert_document_image"),
		Execute:              s.executeInsertDocumentImage,
	})
	s.register(InternalCommandDefinition{
		Name:     "docs.search_documents",
		Module:   "docs",
		Mutating: false,
		Tool:     mustCommandToolMetadata("docs.search_documents"),
		Execute:  s.executeSearchDocuments,
	})
	s.register(InternalCommandDefinition{
		Name:     "docs.publish_prd_draft",
		Module:   "docs",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "docs.publish_prd_draft",
			Alias:       "publish_prd_draft",
			Category:    "Docs",
			Description: "Publish the current PRD markdown draft for epic planner review.",
			InputSchema: commandPreviewMarkdownSchema("Required. The full markdown PRD draft body under review."),
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.executePublishRunPreview(ctx, meta, agentcontract.ToolPublishPRDDraft, input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:     "docs.publish_task_plan_doc",
		Module:   "docs",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "docs.publish_task_plan_doc",
			Alias:       "publish_task_plan_doc",
			Category:    "Docs",
			Description: "Publish the current task planning document markdown for review. Always include the full markdown draft in \"content\"; do not send title-only payloads.",
			InputSchema: commandPreviewMarkdownSchema("Required. The full markdown task planning document body under review, for example \"# Outcome\\n...\"."),
		},
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return s.executePublishRunPreview(ctx, meta, agentcontract.ToolPublishTaskPlanDoc, input)
		},
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.publish_document_change_proposal",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "support_coverage_gap"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "docs.publish_document_change_proposal",
			Alias:       "publish_document_change_proposal",
			Category:    "Docs",
			Description: "Submit a proposed Docs document or block change for review in Docs. This persists a Docs proposal; after success, finish without calling request_approval.",
			InputSchema: commandDocumentChangeProposalSchema(),
		},
		Execute: s.executePublishDocumentChangeProposal,
	})
}

func (s *InternalCommandService) executeInsertDocumentImage(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.docsBlockService == nil {
		return nil, fmt.Errorf("docs block service is not available")
	}
	if s.agentRunArtifactRepo == nil {
		return nil, fmt.Errorf("agent run artifact repository is not available")
	}
	var req struct {
		DocumentID   string  `json:"document_id"`
		ArtifactID   string  `json:"artifact_id"`
		AfterBlockID *string `json:"after_block_id,omitempty"`
		Alt          string  `json:"alt"`
		Caption      *string `json:"caption,omitempty"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse insert document image input: %w", err)
	}
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.ArtifactID = strings.TrimSpace(req.ArtifactID)
	req.Alt = strings.TrimSpace(req.Alt)
	if req.DocumentID == "" || req.ArtifactID == "" || req.Alt == "" {
		return nil, fmt.Errorf("document_id, artifact_id, and alt are required")
	}
	if err := s.requireCommandDocumentInWorkspace(ctx, meta.WorkspaceID, req.DocumentID); err != nil {
		return nil, err
	}
	artifact, err := s.agentRunArtifactRepo.GetByIDAndWorkspace(ctx, meta.WorkspaceID, req.ArtifactID)
	if err != nil {
		return nil, err
	}
	if artifact == nil || artifact.ArtifactType != model.AgentRunArtifactTypeBrowserScreenshot || artifact.StorageMode != "object" || artifact.ObjectKey == nil || strings.TrimSpace(*artifact.ObjectKey) == "" {
		return nil, fmt.Errorf("private browser screenshot artifact not found")
	}

	attrs := map[string]any{
		"src":        artifactReference(artifact.ID),
		"artifactId": artifact.ID,
		"alt":        req.Alt,
		"width":      "100%",
		"height":     "auto",
		"alignment":  "center",
	}
	if req.Caption != nil && strings.TrimSpace(*req.Caption) != "" {
		attrs["caption"] = strings.TrimSpace(*req.Caption)
	}
	block, err := json.Marshal(map[string]any{"type": "resizableImage", "attrs": attrs})
	if err != nil {
		return nil, fmt.Errorf("encode document image block: %w", err)
	}
	content, err := s.docsBlockService.Create(ctx, req.DocumentID, req.AfterBlockID, block, meta.ActorID)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{
		"document_id":  req.DocumentID,
		"content_id":   content.ID,
		"artifact_id":  artifact.ID,
		"artifact_ref": artifactReference(artifact.ID),
		"visibility":   "private",
	}), nil
}

func (s *InternalCommandService) executeSearchDocuments(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Query  string `json:"query"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse search documents input: %w", err)
		}
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit < 1 || req.Limit > 20 {
		return nil, fmt.Errorf("limit must be between 1 and 20")
	}
	if req.Offset < 0 {
		return nil, fmt.Errorf("offset must be zero or greater")
	}
	if s.docsSearchRepo == nil {
		return nil, fmt.Errorf("docs search is not available")
	}
	results, err := s.docsSearchRepo.Search(ctx, meta.WorkspaceID, query, nil, nil, req.Offset+req.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("search documents: %w", err)
	}
	type docsSearchHit struct {
		ID           string `json:"id"`
		MarkdownLink string `json:"markdown_link"`
		Title        string `json:"title"`
	}
	start := min(req.Offset, len(results))
	end := min(start+req.Limit, len(results))
	hits := make([]docsSearchHit, 0, end-start)
	for _, result := range results[start:end] {
		hits = append(hits, docsSearchHit{ID: result.ID, MarkdownLink: helpinMarkdownLink(result.Title, "documents", result.ID), Title: result.Title})
	}
	response := commandPaginationOutput(int64(len(results)), req.Offset, req.Limit, len(hits))
	response["documents"] = hits
	return mustJSON(response), nil
}

// executePublishRunPreview persists a run preview artifact for fixed-panel
// publish tools. It reuses the native preview normalization so the artifact
// payload is identical to what the native executors extract from tool
// invocations, keeping the approval/apply pipeline contract intact.
func (s *InternalCommandService) executePublishRunPreview(ctx context.Context, meta model.InternalCommandContext, toolName string, input json.RawMessage) (json.RawMessage, error) {
	if s.agentRunArtifactRepo == nil {
		return nil, fmt.Errorf("run preview storage is not configured")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	previews := agentcontract.ExtractPublishedPreviews([]model.ToolInvocation{{ToolName: toolName, Input: input}})
	if len(previews) == 0 {
		return nil, fmt.Errorf("%s is missing content; include the markdown body in \"content\"", toolName)
	}
	for _, preview := range previews {
		if err := s.appendCommandRunArtifact(ctx, run, agentcontract.RunPreviewArtifactType, "json", preview); err != nil {
			return nil, err
		}
	}
	preview := previews[len(previews)-1]
	return mustJSON(map[string]any{
		"status":    "published",
		"panel_key": preview.PanelKey,
		"title":     preview.Title,
		"format":    preview.Format,
		"replace":   preview.Replace,
	}), nil
}

func (s *InternalCommandService) appendCommandRunArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	seq, err := s.agentRunArtifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	text := string(content)
	return s.agentRunArtifactRepo.Create(ctx, &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: &text,
		Metadata:      json.RawMessage(`{"source":"internal_command"}`),
		SequenceNo:    seq,
	})
}

func (s *InternalCommandService) executePublishDocumentChangeProposal(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.docsChangeProposalService == nil {
		return nil, fmt.Errorf("document change proposal storage is not available")
	}
	var req struct {
		Scope      string                           `json:"scope"`
		DocumentID string                           `json:"document_id"`
		BlockID    string                           `json:"block_id"`
		Revision   int                              `json:"revision"`
		Content    string                           `json:"content"`
		Summary    string                           `json:"summary"`
		Sources    []model.DocsChangeProposalSource `json:"sources"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse document change proposal input: %w", err)
	}
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	req.DocumentID = strings.TrimSpace(firstNonEmptyCommand(req.DocumentID, currentDocumentTargetID(meta)))
	req.BlockID = strings.TrimSpace(req.BlockID)
	req.Content = strings.TrimSpace(req.Content)
	req.Summary = strings.TrimSpace(req.Summary)
	switch req.Scope {
	case "document", "block":
	default:
		return nil, fmt.Errorf("scope must be document or block")
	}
	if req.DocumentID == "" {
		return nil, fmt.Errorf("document_id is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.Summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if targetID := currentDocumentTargetID(meta); targetID != "" && targetID != req.DocumentID {
		return nil, fmt.Errorf("document_id does not match this run target")
	}

	var content json.RawMessage
	switch req.Scope {
	case "document":
		content = tiptap.MarkdownToJSON(req.Content)
	case "block":
		if req.BlockID == "" {
			return nil, fmt.Errorf("block_id is required for block proposals")
		}
		if req.Revision <= 0 {
			return nil, fmt.Errorf("revision is required for block proposals")
		}
		block, err := s.requireCommandProposalBlock(ctx, meta.WorkspaceID, req.DocumentID, req.BlockID, req.Revision)
		if err != nil {
			return nil, err
		}
		content, err = commandProposalBlockContentFromMarkdown(block.Content, req.Content)
		if err != nil {
			return nil, err
		}
	}

	sources, err := json.Marshal(req.Sources)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal sources: %w", err)
	}
	localRunID := s.bestEffortLocalRunID(ctx, meta)
	created, err := s.docsChangeProposalService.Create(ctx, meta.WorkspaceID, model.CreateDocsChangeProposalRequest{
		Scope:           req.Scope,
		DocumentID:      req.DocumentID,
		BlockID:         stringPtrOrNil(req.BlockID),
		AgentID:         stringPtrOrNil(meta.AgentID),
		AgentRunID:      stringPtrOrNil(localRunID),
		Revision:        req.Revision,
		Summary:         req.Summary,
		ContentMarkdown: req.Content,
		Content:         content,
		Sources:         sources,
		CreatedBy:       firstNonEmptyCommand(meta.AgentID, localRunID, meta.ActorID),
	})
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{
		"status":      "submitted",
		"proposal_id": created.ID,
		"scope":       req.Scope,
		"document_id": req.DocumentID,
		"block_id":    req.BlockID,
		"next_action": "Finish. The proposal is now visible in Docs for a human to apply or discard.",
	}), nil
}

// bestEffortLocalRunID maps the command context run ID (which may be an
// external runtime run ID) to a local agent run ID when possible. Proposals
// remain valid without a run link, so lookup failures are non-fatal.
func (s *InternalCommandService) bestEffortLocalRunID(ctx context.Context, meta model.InternalCommandContext) string {
	if s.agentRunRepo == nil || strings.TrimSpace(meta.RunID) == "" {
		return ""
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil || run == nil {
		return ""
	}
	return run.ID
}

func (s *InternalCommandService) requireCommandProposalBlock(ctx context.Context, workspaceID, documentID, blockID string, revision int) (*model.DocsBlock, error) {
	if s.docsBlockService == nil {
		return nil, fmt.Errorf("document block access is not available")
	}
	blocks, err := s.docsBlockService.List(ctx, workspaceID, documentID)
	if err != nil {
		return nil, fmt.Errorf("load document blocks: %w", err)
	}
	for i := range blocks {
		block := &blocks[i]
		if strings.TrimSpace(block.ID) != blockID {
			continue
		}
		if block.DeletedAt != nil {
			return nil, fmt.Errorf("block not found")
		}
		if block.Revision != revision {
			return nil, fmt.Errorf("revision is stale; fetch the latest block revision")
		}
		return block, nil
	}
	return nil, fmt.Errorf("block not found")
}

// commandProposalBlockContentFromMarkdown converts replacement markdown into a
// single block node, preserving the current block's stable attrs (same rules
// as the native publish_document_change_proposal tool).
func commandProposalBlockContentFromMarkdown(current json.RawMessage, markdown string) (json.RawMessage, error) {
	var currentNode map[string]any
	if err := json.Unmarshal(current, &currentNode); err != nil {
		return nil, fmt.Errorf("parse current block: %w", err)
	}
	var generated struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(tiptap.MarkdownToJSON(markdown), &generated); err != nil {
		return nil, fmt.Errorf("parse proposal markdown: %w", err)
	}
	if len(generated.Content) == 0 {
		return nil, fmt.Errorf("content must not be empty")
	}
	next := generated.Content[0]
	if currentAttrs, _ := currentNode["attrs"].(map[string]any); currentAttrs != nil {
		attrs, _ := next["attrs"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
			next["attrs"] = attrs
		}
		for _, key := range []string{"blockId", "staleState", "staleReason", "staleSource", "staleGapId", "staleMarkedAt"} {
			if value, ok := currentAttrs[key]; ok {
				attrs[key] = value
			}
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal block: %w", err)
	}
	return raw, nil
}

func commandPreviewMarkdownSchema(contentDescription string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Optional preview title.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": contentDescription,
			},
			"replace": map[string]any{"type": "boolean"},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	}
}

func commandDocumentChangeProposalSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"scope": map[string]any{
				"type":        "string",
				"enum":        []string{"document", "block"},
				"description": "Whether the proposal replaces the whole document or one addressable block.",
			},
			"document_id": map[string]any{
				"type":        "string",
				"description": "The document ID from the run context.",
			},
			"block_id": map[string]any{
				"type":        "string",
				"description": "Required when scope is block. The stable block ID to replace.",
			},
			"revision": map[string]any{
				"type":        "integer",
				"description": "Required when scope is block. The current block revision from get_document_blocks.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Replacement markdown. For document scope, provide the full document. For block scope, provide replacement markdown for the focused block only.",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "Short human-readable summary of the proposed change.",
			},
			"sources": map[string]any{
				"type":        "array",
				"description": "Optional source summaries used for the proposal.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":  map[string]any{"type": "string", "enum": []string{"conversation", "document", "url", "agent_run", "coverage_gap"}},
						"id":    map[string]any{"type": "string"},
						"label": map[string]any{"type": "string"},
						"url":   map[string]any{"type": "string"},
					},
					"required":             []string{"type", "label"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"scope", "document_id", "content", "summary"},
		"additionalProperties": false,
	}
}
