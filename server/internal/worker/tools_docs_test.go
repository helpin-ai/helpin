package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestToolListCollections(t *testing.T) {
	parentID := "coll-parent"
	var requestedWorkspaceID string
	var requestedSpaceID *string

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		Services: &ServiceBridge{
			ListCollections: func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsCollection, error) {
				requestedWorkspaceID = workspaceID
				requestedSpaceID = spaceID
				return []model.DocsCollection{
					{
						ID:                 "coll-1",
						Name:               "Product",
						Slug:               "product",
						SpaceID:            "space-1",
						ParentCollectionID: &parentID,
					},
					{
						ID:      "coll-2",
						Name:    "Support",
						Slug:    "support",
						SpaceID: "space-1",
					},
				}, nil
			},
		},
	}

	output, err := toolListCollections(ctx, json.RawMessage(`{"space_id":"space-1"}`))
	if err != nil {
		t.Fatalf("toolListCollections returned error: %v", err)
	}
	if requestedWorkspaceID != "ws-1" {
		t.Fatalf("expected workspace ws-1, got %q", requestedWorkspaceID)
	}
	if requestedSpaceID == nil || *requestedSpaceID != "space-1" {
		t.Fatalf("expected space_id space-1, got %#v", requestedSpaceID)
	}

	var response []struct {
		ID                 string  `json:"id"`
		Name               string  `json:"name"`
		Slug               string  `json:"slug"`
		SpaceID            string  `json:"space_id"`
		ParentCollectionID *string `json:"parent_collection_id,omitempty"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(response) != 2 {
		t.Fatalf("expected two collections, got %#v", response)
	}
	if response[0].ID != "coll-1" || response[0].Name != "Product" || response[0].Slug != "product" || response[0].SpaceID != "space-1" || response[0].ParentCollectionID == nil || *response[0].ParentCollectionID != parentID {
		t.Fatalf("unexpected first collection summary %#v", response[0])
	}
	if response[1].ID != "coll-2" || response[1].ParentCollectionID != nil {
		t.Fatalf("unexpected second collection summary %#v", response[1])
	}
}

func TestToolListCollectionsEmpty(t *testing.T) {
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		Services: &ServiceBridge{
			ListCollections: func(ctx context.Context, workspaceID string, spaceID *string) ([]model.DocsCollection, error) {
				return nil, nil
			},
		},
	}

	output, err := toolListCollections(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("toolListCollections returned error: %v", err)
	}
	if output != "No collections found." {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestToolGetDocumentBlocksReturnsCompactBlocksByDefault(t *testing.T) {
	ctx := &ExecutionContext{
		Context: context.Background(),
		Services: &ServiceBridge{
			ListDocumentBlocks: func(ctx context.Context, documentID string) ([]model.DocsBlock, error) {
				if documentID != "doc-1" {
					t.Fatalf("unexpected document id %q", documentID)
				}
				return []model.DocsBlock{
					{
						ID:          "block-1",
						Type:        "paragraph",
						Revision:    3,
						ContentText: "First block",
						Content:     json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"First block"}]}`),
					},
				}, nil
			},
		},
	}

	output, err := toolGetDocumentBlocks(ctx, json.RawMessage(`{"document_id":"doc-1"}`))
	if err != nil {
		t.Fatalf("toolGetDocumentBlocks returned error: %v", err)
	}

	var response []struct {
		ID          string          `json:"id"`
		Type        string          `json:"type"`
		Revision    int             `json:"revision"`
		ContentText string          `json:"content_text"`
		Content     json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("expected one block, got %#v", response)
	}
	if response[0].ID != "block-1" || response[0].Type != "paragraph" || response[0].Revision != 3 || response[0].ContentText != "First block" {
		t.Fatalf("unexpected compact block %#v", response[0])
	}
	if len(response[0].Content) != 0 {
		t.Fatalf("expected compact block to omit full content, got %s", string(response[0].Content))
	}
}

func TestToolPublishAISectionCandidateStoresReviewCandidate(t *testing.T) {
	current := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Pricing","status":"draft"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)
	var published *model.DocsAISectionCandidate
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		TargetType:  "document",
		TargetID:    "doc-1",
		Agent:       &model.Agent{Name: "Docs Agent"},
		RunInput: &model.AgentRunInputPayload{
			Output: &model.AgentRunOutputContext{
				Type:           "docs_ai_section_candidate",
				IdempotencyKey: "block-1",
			},
		},
		Services: &ServiceBridge{
			ListDocumentBlocks: func(ctx context.Context, documentID string) ([]model.DocsBlock, error) {
				if documentID != "doc-1" {
					t.Fatalf("documentID = %q, want doc-1", documentID)
				}
				return []model.DocsBlock{{
					ID:          "block-1",
					WorkspaceID: "ws-1",
					DocumentID:  "doc-1",
					Type:        "aiSection",
					Content:     current,
					ContentText: "Old pricing",
					Revision:    2,
				}}, nil
			},
			PublishAISectionCandidate: func(ctx context.Context, workspaceID, documentID, blockID, agentID, runID string, currentContent, candidateContent json.RawMessage, candidateText string, sourceRefs model.JSONB, prompt *string, promptHash *string, modelName *string) (*model.DocsAISectionCandidate, error) {
				published = &model.DocsAISectionCandidate{
					ID:               "candidate-1",
					WorkspaceID:      workspaceID,
					DocumentID:       documentID,
					BlockID:          blockID,
					CurrentContent:   currentContent,
					CandidateContent: candidateContent,
					CandidateText:    candidateText,
					SourceRefs:       sourceRefs,
					PromptHash:       promptHash,
					Model:            modelName,
					CreatedBy:        agentID,
				}
				return published, nil
			},
		},
	}

	output, err := toolPublishAISectionCandidate(ctx, json.RawMessage(`{
		"document_id":"doc-1",
		"block_id":"block-1",
		"content":"Growth is $84/month. Scale is $199/month.",
		"sources":[{"title":"Pricing","url":"https://usermaven.com/pricing"}]
	}`))
	if err != nil {
		t.Fatalf("toolPublishAISectionCandidate returned error: %v", err)
	}
	if !strings.Contains(output, `"candidate_id":"candidate-1"`) {
		t.Fatalf("unexpected output %q", output)
	}
	if published == nil {
		t.Fatal("expected candidate to be published")
	}
	if published.WorkspaceID != "ws-1" || published.DocumentID != "doc-1" || published.BlockID != "block-1" || published.CreatedBy != "agent-1" {
		t.Fatalf("unexpected candidate metadata %#v", published)
	}
	if !strings.Contains(published.CandidateText, "Growth is $84/month") {
		t.Fatalf("candidate text = %q", published.CandidateText)
	}
	var decoded struct {
		Attrs   map[string]any `json:"attrs"`
		Content []struct {
			Type string `json:"type"`
		} `json:"content"`
	}
	if err := json.Unmarshal(published.CandidateContent, &decoded); err != nil {
		t.Fatalf("decode candidate content: %v", err)
	}
	if decoded.Attrs["status"] != "needs_review" || decoded.Attrs["ownerAgentId"] != "agent-1" || decoded.Attrs["ownerAgentName"] != "Docs Agent" {
		t.Fatalf("unexpected attrs %#v", decoded.Attrs)
	}
	if len(decoded.Content) == 0 || decoded.Content[0].Type != "paragraph" {
		t.Fatalf("expected generated paragraph, got %#v", decoded.Content)
	}
	if published.PromptHash == nil || *published.PromptHash == "" {
		t.Fatal("expected prompt hash")
	}
}

func TestToolGetDocumentBlocksCanReturnSelectedFullContent(t *testing.T) {
	ctx := &ExecutionContext{
		Context: context.Background(),
		Services: &ServiceBridge{
			ListDocumentBlocks: func(ctx context.Context, documentID string) ([]model.DocsBlock, error) {
				return []model.DocsBlock{
					{
						ID:          "block-1",
						Type:        "paragraph",
						Revision:    1,
						ContentText: "First",
						Content:     json.RawMessage(`{"type":"paragraph","attrs":{"blockId":"block-1"},"content":[{"type":"text","text":"First"}]}`),
					},
					{
						ID:          "block-2",
						Type:        "heading",
						Revision:    2,
						ContentText: "Second",
						Content:     json.RawMessage(`{"type":"heading","attrs":{"blockId":"block-2","level":2},"content":[{"type":"text","text":"Second"}]}`),
					},
				}, nil
			},
		},
	}

	output, err := toolGetDocumentBlocks(ctx, json.RawMessage(`{"document_id":"doc-1","block_ids":["block-2"],"include_content":true}`))
	if err != nil {
		t.Fatalf("toolGetDocumentBlocks returned error: %v", err)
	}

	var response []struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(response) != 1 || response[0].ID != "block-2" {
		t.Fatalf("expected selected block-2, got %#v", response)
	}
	if !strings.Contains(string(response[0].Content), `"level":2`) || !strings.Contains(string(response[0].Content), `"blockId":"block-2"`) {
		t.Fatalf("expected full block JSON, got %s", string(response[0].Content))
	}
}

func TestToolGetDocumentBlocksLimitsFullContentFetches(t *testing.T) {
	blocks := make([]model.DocsBlock, maxFullDocumentBlocksToolFetch+1)
	for i := range blocks {
		blocks[i] = model.DocsBlock{
			ID:          "block",
			Type:        "paragraph",
			Revision:    1,
			ContentText: "Body",
			Content:     json.RawMessage(`{"type":"paragraph"}`),
		}
	}
	ctx := &ExecutionContext{
		Context: context.Background(),
		Services: &ServiceBridge{
			ListDocumentBlocks: func(ctx context.Context, documentID string) ([]model.DocsBlock, error) {
				return blocks, nil
			},
		},
	}

	_, err := toolGetDocumentBlocks(ctx, json.RawMessage(`{"document_id":"doc-1","include_content":true}`))
	if err == nil || !strings.Contains(err.Error(), "include_content is limited to 20 blocks") {
		t.Fatalf("expected full content limit error, got %v", err)
	}
}

func TestToolCreateDocumentWithMarkdownContent(t *testing.T) {
	var (
		createdReq     model.CreateDocsDocumentRequest
		createdContent json.RawMessage
	)

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		Services: &ServiceBridge{
			CreateDocument: func(ctx context.Context, workspaceID, userID string, req model.CreateDocsDocumentRequest, content json.RawMessage) (*model.DocsDocument, error) {
				if workspaceID != "ws-1" || userID != "agent-1" {
					t.Fatalf("unexpected create context workspace=%q user=%q", workspaceID, userID)
				}
				createdReq = req
				createdContent = append(json.RawMessage(nil), content...)
				return &model.DocsDocument{
					ID:      "doc-1",
					Title:   req.Title,
					Status:  model.DocStatusDraft,
					SpaceID: req.SpaceID,
				}, nil
			},
		},
	}

	output, err := toolCreateDocument(ctx, json.RawMessage(`{
		"space_id": "space-1",
		"title": "My Document",
		"content": "# Heading\n\nBody text",
		"tags": ["planning"]
	}`))
	if err != nil {
		t.Fatalf("toolCreateDocument returned error: %v", err)
	}
	if createdReq.SpaceID != "space-1" || createdReq.Title != "My Document" || len(createdReq.Tags) != 1 || createdReq.Tags[0] != "planning" {
		t.Fatalf("unexpected create request %#v", createdReq)
	}
	if len(createdContent) == 0 || createdContent[0] != '{' || !strings.Contains(string(createdContent), "Heading") || !strings.Contains(string(createdContent), "Body") || !strings.Contains(string(createdContent), " text") {
		t.Fatalf("expected markdown content to be converted to TipTap JSON, got %s", string(createdContent))
	}
	var response struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Status  string `json:"status"`
		SpaceID string `json:"space_id"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if response.ID != "doc-1" || response.Title != "My Document" || response.Status != model.DocStatusDraft || response.SpaceID != "space-1" {
		t.Fatalf("unexpected output %s", output)
	}
}

func TestToolCreateDocumentWithoutContent(t *testing.T) {
	var createdContent json.RawMessage

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		Services: &ServiceBridge{
			CreateDocument: func(ctx context.Context, workspaceID, userID string, req model.CreateDocsDocumentRequest, content json.RawMessage) (*model.DocsDocument, error) {
				createdContent = append(json.RawMessage(nil), content...)
				return &model.DocsDocument{
					ID:      "doc-2",
					Title:   req.Title,
					Status:  model.DocStatusDraft,
					SpaceID: req.SpaceID,
				}, nil
			},
		},
	}

	output, err := toolCreateDocument(ctx, json.RawMessage(`{
		"space_id": "space-1",
		"title": "Metadata Only"
	}`))
	if err != nil {
		t.Fatalf("toolCreateDocument returned error: %v", err)
	}
	if len(createdContent) != 0 {
		t.Fatalf("expected no initial content, got %s", string(createdContent))
	}
	if !strings.Contains(output, `"id":"doc-2"`) || !strings.Contains(output, `"title":"Metadata Only"`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestToolCreateDocumentRequiresSpaceIDAndTitle(t *testing.T) {
	ctx := &ExecutionContext{Context: context.Background(), WorkspaceID: "ws-1"}

	if _, err := toolCreateDocument(ctx, json.RawMessage(`{"title":"Missing Space"}`)); err == nil || err.Error() != "space_id is required" {
		t.Fatalf("expected space_id is required error, got %v", err)
	}
	if _, err := toolCreateDocument(ctx, json.RawMessage(`{"space_id":"space-1"}`)); err == nil || err.Error() != "title is required" {
		t.Fatalf("expected title is required error, got %v", err)
	}
}

func TestToolCreateDocumentRequiresDocsService(t *testing.T) {
	ctx := &ExecutionContext{Context: context.Background(), WorkspaceID: "ws-1"}

	if _, err := toolCreateDocument(ctx, json.RawMessage(`{"space_id":"space-1","title":"Doc"}`)); err == nil || err.Error() != "docs creation is not available for this agent" {
		t.Fatalf("expected docs creation unavailable error, got %v", err)
	}
}

func TestToolCreateDocumentReusesExistingOutputDocument(t *testing.T) {
	createCalled := false
	documentID := "doc-existing"
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		RunInput: &model.AgentRunInputPayload{
			Output: &model.AgentRunOutputContext{
				Type:           "docs_document",
				SpaceID:        "space-1",
				IdempotencyKey: "release_notes:repo-1:v1.4.0",
			},
		},
		Services: &ServiceBridge{
			GetDocumentKey: func(ctx context.Context, workspaceID, keyType, key string) (*model.DocsDocumentKey, error) {
				if workspaceID != "ws-1" || keyType != model.DocsDocumentKeyTypeReleaseNotes || key != "release_notes:repo-1:v1.4.0" {
					t.Fatalf("unexpected key lookup %q %q %q", workspaceID, keyType, key)
				}
				return &model.DocsDocumentKey{
					WorkspaceID: "ws-1",
					KeyType:     model.DocsDocumentKeyTypeReleaseNotes,
					Key:         key,
					DocumentID:  &documentID,
				}, nil
			},
			GetDocument: func(ctx context.Context, id string) (*model.DocsDocument, error) {
				if id != "doc-existing" {
					t.Fatalf("unexpected document lookup %q", id)
				}
				return &model.DocsDocument{
					ID:      "doc-existing",
					Title:   "Release Notes v1.4.0",
					Status:  model.DocStatusDraft,
					SpaceID: "space-1",
				}, nil
			},
			CreateDocument: func(ctx context.Context, workspaceID, userID string, req model.CreateDocsDocumentRequest, content json.RawMessage) (*model.DocsDocument, error) {
				createCalled = true
				return nil, nil
			},
		},
	}

	output, err := toolCreateDocument(ctx, json.RawMessage(`{"space_id":"space-1","title":"Release Notes v1.4.0"}`))
	if err != nil {
		t.Fatalf("toolCreateDocument returned error: %v", err)
	}
	if createCalled {
		t.Fatal("expected existing document to be reused")
	}
	if !strings.Contains(output, `"id":"doc-existing"`) || !strings.Contains(output, `"title":"Release Notes v1.4.0"`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestToolCreateDocumentPersistsOutputDocumentKey(t *testing.T) {
	var persisted *model.DocsDocumentKey
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunInput: &model.AgentRunInputPayload{
			Output: &model.AgentRunOutputContext{
				Type:           "docs_document",
				SpaceID:        "space-1",
				IdempotencyKey: "release_notes:repo-1:v1.4.0",
			},
		},
		Services: &ServiceBridge{
			GetDocumentKey: func(ctx context.Context, workspaceID, keyType, key string) (*model.DocsDocumentKey, error) {
				return nil, nil
			},
			CreateDocument: func(ctx context.Context, workspaceID, userID string, req model.CreateDocsDocumentRequest, content json.RawMessage) (*model.DocsDocument, error) {
				return &model.DocsDocument{
					ID:      "doc-created",
					Title:   req.Title,
					Status:  model.DocStatusDraft,
					SpaceID: req.SpaceID,
				}, nil
			},
			UpsertDocumentKey: func(ctx context.Context, record *model.DocsDocumentKey) error {
				copied := *record
				persisted = &copied
				return nil
			},
		},
	}

	if _, err := toolCreateDocument(ctx, json.RawMessage(`{"space_id":"space-1","title":"Release Notes v1.4.0"}`)); err != nil {
		t.Fatalf("toolCreateDocument returned error: %v", err)
	}
	if persisted == nil {
		t.Fatal("expected document key to be persisted")
	}
	if persisted.KeyType != model.DocsDocumentKeyTypeReleaseNotes || persisted.Key != "release_notes:repo-1:v1.4.0" || persisted.DocumentID == nil || *persisted.DocumentID != "doc-created" {
		t.Fatalf("unexpected persisted key %#v", persisted)
	}
}

func TestToolCreateDocumentUsesInternalCommandWithConvertedMarkdown(t *testing.T) {
	var called bool

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "docs.create_document" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "workspace" || meta.TargetID != "ws-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				var commandInput struct {
					SpaceID string          `json:"space_id"`
					Title   string          `json:"title"`
					Content json.RawMessage `json:"content"`
				}
				if err := json.Unmarshal(input, &commandInput); err != nil {
					t.Fatalf("unmarshal command input: %v", err)
				}
				if commandInput.SpaceID != "space-1" || commandInput.Title != "Doc" {
					t.Fatalf("unexpected command input %s", string(input))
				}
				if len(commandInput.Content) == 0 || commandInput.Content[0] != '{' || !strings.Contains(string(commandInput.Content), "Converted") {
					t.Fatalf("expected converted TipTap content, got %s", string(commandInput.Content))
				}
				return json.RawMessage(`{"id":"doc-3","title":"Doc","status":"draft","space_id":"space-1"}`), nil
			},
		},
	}

	output, err := toolCreateDocument(ctx, json.RawMessage(`{
		"space_id": "space-1",
		"title": "Doc",
		"content": "Converted markdown"
	}`))
	if err != nil {
		t.Fatalf("toolCreateDocument returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"id":"doc-3","title":"Doc","status":"draft","space_id":"space-1"}` {
		t.Fatalf("unexpected tool output %q", output)
	}
}

func TestToolWriteDocumentContentFallsBackToApprovedMarkdownArtifact(t *testing.T) {
	var (
		writtenDocumentID string
		writtenContent    json.RawMessage
	)

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		ArtifactContext: &ArtifactContext{
			Entries: []ArtifactContextEntry{
				{
					Label:   "Approved preview for prd_draft",
					Source:  "approved_preview",
					Status:  "approved_and_persist_prd",
					Format:  PreviewFormatMarkdown,
					Content: "# Problem\n\nApproved draft body",
				},
			},
		},
		Services: &ServiceBridge{
			WriteDocumentContent: func(ctx context.Context, workspaceID, documentID string, content json.RawMessage) error {
				writtenDocumentID = documentID
				writtenContent = append(json.RawMessage(nil), content...)
				return nil
			},
		},
	}

	output, err := toolWriteDocumentContent(ctx, json.RawMessage(`{
		"document_id": "doc-1",
		"content": null
	}`))
	if err != nil {
		t.Fatalf("toolWriteDocumentContent returned error: %v", err)
	}
	if writtenDocumentID != "doc-1" {
		t.Fatalf("expected document id doc-1, got %q", writtenDocumentID)
	}
	var markdown string
	if err := json.Unmarshal(writtenContent, &markdown); err != nil {
		t.Fatalf("expected markdown json string, got %s (%v)", string(writtenContent), err)
	}
	if markdown != "# Problem\n\nApproved draft body" {
		t.Fatalf("unexpected markdown content %q", markdown)
	}
	if output != "Document doc-1 updated." {
		t.Fatalf("unexpected tool output %q", output)
	}
}

func TestToolWriteDocumentContentRequiresContentWithoutApprovedArtifactFallback(t *testing.T) {
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		Services: &ServiceBridge{
			WriteDocumentContent: func(ctx context.Context, workspaceID, documentID string, content json.RawMessage) error {
				return nil
			},
		},
	}

	if _, err := toolWriteDocumentContent(ctx, json.RawMessage(`{
		"document_id": "doc-1",
		"content": null
	}`)); err == nil || err.Error() != "content is required" {
		t.Fatalf("expected content is required error, got %v", err)
	}
}

func TestToolWriteDocumentContentUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	var called bool

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		ArtifactContext: &ArtifactContext{
			Entries: []ArtifactContextEntry{
				{
					Label:   "Approved preview for prd_draft",
					Source:  "approved_preview",
					Status:  "approved_and_persist_prd",
					Format:  PreviewFormatMarkdown,
					Content: "# Problem\n\nApproved draft body",
				},
			},
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "docs.write_document_content" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "document" || meta.TargetID != "doc-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				return json.RawMessage(`{"document_id":"doc-1","content_id":"content-1"}`), nil
			},
		},
	}

	output, err := toolWriteDocumentContent(ctx, json.RawMessage(`{
		"document_id": "doc-1",
		"content": null
	}`))
	if err != nil {
		t.Fatalf("toolWriteDocumentContent returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"document_id":"doc-1","content_id":"content-1"}` {
		t.Fatalf("unexpected tool output %q", output)
	}
}
