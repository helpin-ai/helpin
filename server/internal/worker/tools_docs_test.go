package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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
