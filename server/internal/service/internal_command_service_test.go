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

func TestCreateDocumentCommandMetadataAndTargets(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.create_document")
	if !ok {
		t.Fatal("expected docs.create_document definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected docs.create_document to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "create_document" || def.Tool.Category != "Docs" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}
	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "workspace" {
			return
		}
	}
	t.Fatalf("expected docs.create_document to support workspace target, got %#v", def.SupportedTargetTypes)
}

func TestCreateTaskCommandMetadataAndTargets(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("pm.create_task")
	if !ok {
		t.Fatal("expected pm.create_task definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected pm.create_task to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "create_task" || def.Tool.Category != "PM / Tasks" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}

	var supportsWorkspace bool
	var supportsEpic bool
	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "workspace" {
			supportsWorkspace = true
		}
		if targetType == "epic" {
			supportsEpic = true
		}
	}
	if !supportsWorkspace || !supportsEpic {
		t.Fatalf("expected pm.create_task to support workspace and epic targets, got %#v", def.SupportedTargetTypes)
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
