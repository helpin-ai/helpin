package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestWriteDocumentContentCommandSupportsDocumentTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}

	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "document" {
			return
		}
	}

	t.Fatalf("expected docs.write_document_content to support target type document, got %#v", def.SupportedTargetTypes)
}

func TestWriteDocumentContentCommandRejectsEmptyContent(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}

	_, err := def.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "document",
		TargetID:    "doc-1",
	}, []byte(`{"document_id":"doc-1","content":{"type":"doc","content":[]}}`))
	if err == nil {
		t.Fatal("expected empty document content to be rejected")
	}
	if !strings.Contains(err.Error(), "content must not be empty") {
		t.Fatalf("expected empty content error, got %v", err)
	}
}

func TestCommandToolMetadataUsesExplicitAliasInsteadOfBoolean(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected docs.write_document_content to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "write_document_content" || def.Tool.Category != "Docs" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}
}

func TestCreateFollowupTasksCommandIsBackendOnlyUntilToolExists(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("pm.create_followup_tasks")
	if !ok {
		t.Fatal("expected pm.create_followup_tasks definition")
	}
	if def.ExposesTool() {
		t.Fatalf("expected pm.create_followup_tasks to remain backend-only, got %#v", def.Tool)
	}
}
