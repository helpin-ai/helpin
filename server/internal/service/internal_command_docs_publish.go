package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// registerDocsRuntimeToolCommands registers command-backed variants of the
// native docs search and product publish tools.
func (s *InternalCommandService) registerDocsRuntimeToolCommands() {
	s.register(InternalCommandDefinition{
		Name: "docs.edit_document", Module: "docs", Mutating: true,
		Tool: mustCommandToolMetadata("docs.edit_document"), Execute: s.executeEditDocument,
	})

	s.register(InternalCommandDefinition{
		Name:                 "docs.insert_document_artifact",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "support_coverage_gap"},
		Tool:                 mustCommandToolMetadata("docs.insert_document_artifact"),
		Execute:              s.executeInsertDocumentArtifact,
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.insert_document_image",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "support_coverage_gap"},
		Tool:                 mustCommandToolMetadata("docs.insert_document_image"),
		Execute:              s.executeInsertDocumentImage,
	})
	s.register(InternalCommandDefinition{
		Name:                 "docs.insert_document_block",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "support_coverage_gap"},
		Tool:                 mustCommandToolMetadata("docs.insert_document_block"),
		Execute:              s.executeInsertDocumentBlock,
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

type insertDocumentArtifactRequest struct {
	DocumentID   string  `json:"document_id"`
	ArtifactID   string  `json:"artifact_id"`
	AfterBlockID *string `json:"after_block_id,omitempty"`
	Description  string  `json:"description"`
	Caption      *string `json:"caption,omitempty"`
}

func (s *InternalCommandService) executeInsertDocumentArtifact(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req insertDocumentArtifactRequest
	if err := decodeStrictInternalCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse insert document artifact input: %w", err)
	}
	return s.insertDocumentArtifact(ctx, meta, req, "")
}

func (s *InternalCommandService) executeInsertDocumentImage(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		DocumentID   string  `json:"document_id"`
		ArtifactID   string  `json:"artifact_id"`
		AfterBlockID *string `json:"after_block_id,omitempty"`
		Alt          string  `json:"alt"`
		Caption      *string `json:"caption,omitempty"`
	}
	if err := decodeStrictInternalCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse insert document image input: %w", err)
	}
	return s.insertDocumentArtifact(ctx, meta, insertDocumentArtifactRequest{
		DocumentID: req.DocumentID, ArtifactID: req.ArtifactID, AfterBlockID: req.AfterBlockID,
		Description: req.Alt, Caption: req.Caption,
	}, model.AgentRunArtifactTypeBrowserScreenshot)
}

func (s *InternalCommandService) insertDocumentArtifact(ctx context.Context, meta model.InternalCommandContext, req insertDocumentArtifactRequest, requiredArtifactType string) (json.RawMessage, error) {
	if s.docsBlockService == nil {
		return nil, fmt.Errorf("docs block service is not available")
	}
	if s.agentRunArtifactRepo == nil {
		return nil, fmt.Errorf("agent run artifact repository is not available")
	}
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.ArtifactID = strings.TrimSpace(req.ArtifactID)
	req.Description = strings.TrimSpace(req.Description)
	if req.DocumentID == "" || req.ArtifactID == "" || req.Description == "" {
		return nil, errCommandInput("document_id, artifact_id, and description are required")
	}
	if utf8.RuneCountInString(req.Description) > 1000 {
		return nil, errCommandInput("description must be at most 1000 characters")
	}
	if req.Caption != nil && utf8.RuneCountInString(strings.TrimSpace(*req.Caption)) > 2000 {
		return nil, errCommandInput("caption must be at most 2000 characters")
	}
	if err := s.requireCommandDocumentInWorkspace(ctx, meta.WorkspaceID, req.DocumentID); err != nil {
		return nil, err
	}
	artifact, err := s.agentRunArtifactRepo.GetByIDAndWorkspace(ctx, meta.WorkspaceID, req.ArtifactID)
	if err != nil {
		return nil, err
	}
	if artifact == nil || artifact.StorageMode != "object" || artifact.ObjectKey == nil || strings.TrimSpace(*artifact.ObjectKey) == "" {
		return nil, errCommandNotFound("private document artifact")
	}
	if requiredArtifactType != "" && artifact.ArtifactType != requiredArtifactType {
		return nil, errCommandNotFound("private browser screenshot artifact")
	}
	blockType, attrs, err := documentBlockForArtifact(artifact, req.Description)
	if err != nil {
		return nil, err
	}
	if req.Caption != nil && strings.TrimSpace(*req.Caption) != "" {
		attrs["caption"] = strings.TrimSpace(*req.Caption)
	}
	block, err := json.Marshal(map[string]any{"type": blockType, "attrs": attrs})
	if err != nil {
		return nil, fmt.Errorf("encode document artifact block: %w", err)
	}
	content, err := s.docsBlockService.Create(ctx, req.DocumentID, req.AfterBlockID, block, meta.ActorID)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{
		"document_id":   req.DocumentID,
		"content_id":    content.ID,
		"artifact_id":   artifact.ID,
		"artifact_type": artifact.ArtifactType,
		"artifact_ref":  artifactReference(artifact.ID),
		"block_type":    blockType,
		"visibility":    "private",
	}), nil
}

func documentBlockForArtifact(artifact *model.AgentRunArtifact, description string) (string, map[string]any, error) {
	if artifact == nil {
		return "", nil, errCommandNotFound("private document artifact")
	}
	reference := artifactReference(artifact.ID)
	switch artifact.ArtifactType {
	case model.AgentRunArtifactTypeBrowserScreenshot:
		return "resizableImage", map[string]any{
			"src": reference, "artifactId": artifact.ID, "alt": description,
			"width": "100%", "height": "auto", "alignment": "center",
		}, nil
	case model.AgentRunArtifactTypeBrowserRecording:
		fileName, contentType := browserArtifactDisplayMetadata(artifact)
		return "artifactVideo", map[string]any{
			"src": reference, "artifactId": artifact.ID, "description": description,
			"fileName": fileName, "contentType": contentType,
		}, nil
	default:
		return "", nil, fmt.Errorf("artifact type %q cannot be inserted into a document", artifact.ArtifactType)
	}
}

func browserArtifactDisplayMetadata(artifact *model.AgentRunArtifact) (string, string) {
	fileName := "browser-recording.mp4"
	contentType := "video/mp4"
	var metadata map[string]any
	if artifact != nil && json.Unmarshal(artifact.Metadata, &metadata) == nil {
		if value, ok := metadata["file_name"].(string); ok && strings.TrimSpace(value) != "" {
			fileName = strings.TrimSpace(value)
		}
		if value, ok := metadata["content_type"].(string); ok && strings.TrimSpace(value) != "" {
			contentType = strings.TrimSpace(value)
		}
	}
	return fileName, contentType
}

func decodeStrictInternalCommandInput(input json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errCommandInput("input must contain one JSON object")
	}
	return nil
}

func (s *InternalCommandService) executeInsertDocumentBlock(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.docsBlockService == nil {
		return nil, fmt.Errorf("docs block service is not available")
	}
	var req struct {
		DocumentID   string  `json:"document_id"`
		Content      string  `json:"content"`
		AfterBlockID *string `json:"after_block_id,omitempty"`
		Position     string  `json:"position"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse insert document block input: %w", err)
	}
	req.DocumentID = strings.TrimSpace(firstNonEmptyCommand(req.DocumentID, currentDocumentTargetID(meta)))
	req.Content = strings.TrimSpace(req.Content)
	req.Position = strings.ToLower(strings.TrimSpace(req.Position))
	if req.DocumentID == "" {
		return nil, errCommandInput("document_id is required")
	}
	if req.Content == "" {
		return nil, errCommandInput("content is required")
	}
	switch req.Position {
	case "", "start", "end":
	default:
		return nil, errCommandInput("position must be start or end")
	}
	if err := s.requireCommandDocumentInWorkspace(ctx, meta.WorkspaceID, req.DocumentID); err != nil {
		return nil, err
	}
	var generated struct {
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(tiptap.MarkdownToJSON(req.Content), &generated); err != nil {
		return nil, fmt.Errorf("parse block markdown: %w", err)
	}
	if len(generated.Content) == 0 {
		return nil, errCommandInput("content must not be empty")
	}
	content, blockIDs, err := s.docsBlockService.CreateBlocks(ctx, req.DocumentID, req.AfterBlockID, req.Position == "start", generated.Content, meta.ActorID)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{
		"document_id": req.DocumentID,
		"content_id":  content.ID,
		"block_ids":   blockIDs,
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
		return nil, errCommandInput("query is required")
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit < 1 || req.Limit > 20 {
		return nil, errCommandInput("limit must be between 1 and 20")
	}
	if req.Offset < 0 {
		return nil, errCommandInput("offset must be zero or greater")
	}
	if s.docsSearchRepo == nil {
		return nil, fmt.Errorf("docs search is not available")
	}
	results, err := s.docsSearchRepo.SearchWithContext(ctx, meta.WorkspaceID, query, req.Offset+req.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("search documents: %w", err)
	}
	type docsSearchHit struct {
		ID           string `json:"id"`
		MarkdownLink string `json:"markdown_link"`
		Title        string `json:"title"`
		MatchBlockID string `json:"match_block_id,omitempty"`
		MatchText    string `json:"match_text,omitempty"`
	}
	start := min(req.Offset, len(results))
	end := min(start+req.Limit, len(results))
	hits := make([]docsSearchHit, 0, end-start)
	for _, result := range results[start:end] {
		hits = append(hits, docsSearchHit{ID: result.ID, MarkdownLink: helpinMarkdownLink(truncateCommandBarText(result.Title, 500), "documents", result.ID), Title: truncateCommandBarText(result.Title, 500), MatchBlockID: result.MatchBlockID, MatchText: truncateCommandBarText(strings.NewReplacer("<b>", "", "</b>", "").Replace(result.MatchText), 500)})
	}
	response := commandPaginationOutput(int64(len(results)), req.Offset, req.Limit, len(hits))
	response["documents"] = hits
	response["_agent_runtime_compaction"] = map[string]any{"exempt": true, "max_runes": documentOutputBudget}
	return marshalBoundedDocument(response)
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
		return nil, errCommandInput("scope must be document or block")
	}
	if req.DocumentID == "" {
		return nil, errCommandInput("document_id is required")
	}
	if req.Content == "" {
		return nil, errCommandInput("content is required")
	}
	if req.Summary == "" {
		return nil, errCommandInput("summary is required")
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
			return nil, errCommandInput("block_id is required for block proposals")
		}
		if req.Revision <= 0 {
			return nil, errCommandInput("revision is required for block proposals")
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
			return nil, errCommandNotFound("block")
		}
		if block.Revision != revision {
			return nil, fmt.Errorf("revision is stale; fetch the latest block revision")
		}
		return block, nil
	}
	return nil, errCommandNotFound("block")
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
		return nil, errCommandInput("content must not be empty")
	}
	// A block proposal replaces exactly one block. Keeping only the first node
	// would silently drop the rest at apply time while the reviewer is shown
	// the full markdown, so reject the mismatch instead.
	if len(generated.Content) > 1 {
		return nil, fmt.Errorf(
			"block proposals must contain exactly one block, but this markdown produced %d; propose one block at a time, or use insert_document_block to add new blocks",
			len(generated.Content),
		)
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
				"description": "Replacement markdown. For document scope, provide the full document. For block scope, provide markdown for exactly one block (a single paragraph, heading, list, or code block) — multi-block markdown is rejected; add new blocks with insert_document_block instead.",
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
