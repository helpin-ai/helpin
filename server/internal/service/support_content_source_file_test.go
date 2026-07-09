package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestNormalizeSupportContentFileUploadRequestCreatesQueuedFileSource(t *testing.T) {
	source, err := normalizeSupportContentFileUploadRequest("ws-file-source", model.CreateSupportContentSourceFileUploadRequest{
		Name:        " Product guide ",
		FileName:    " Guide.pdf ",
		FileSize:    42,
		ContentType: "application/pdf",
		StorageKey:  "workspaces/ws-file-source/knowledge-sources/source-1/guide.pdf",
	})
	if err != nil {
		t.Fatalf("normalizeSupportContentFileUploadRequest returned error: %v", err)
	}

	if source.SourceType != model.ContentSourceTypeFile {
		t.Fatalf("SourceType = %q, want %q", source.SourceType, model.ContentSourceTypeFile)
	}
	if source.Name != "Product guide" {
		t.Fatalf("Name = %q, want Product guide", source.Name)
	}
	if source.FileName == nil || *source.FileName != "Guide.pdf" {
		t.Fatalf("FileName = %#v, want Guide.pdf", source.FileName)
	}
	if source.StorageKey == nil || *source.StorageKey != "workspaces/ws-file-source/knowledge-sources/source-1/guide.pdf" {
		t.Fatalf("StorageKey = %#v", source.StorageKey)
	}
	if source.StartURL != "file://Guide.pdf" {
		t.Fatalf("StartURL = %q, want file://Guide.pdf", source.StartURL)
	}
	if source.SyncStatus != model.KnowledgeSourceSyncQueued {
		t.Fatalf("SyncStatus = %q, want queued", source.SyncStatus)
	}
}

func TestExtractUploadedContentTextSupportsPlainText(t *testing.T) {
	text, format, err := extractUploadedContentText([]byte("First line\n\nSecond line"), "text/plain", "notes.txt")
	if err != nil {
		t.Fatalf("extractUploadedContentText returned error: %v", err)
	}
	if format != model.ContentSourceFormatMarkdown {
		t.Fatalf("format = %q, want markdown", format)
	}
	if text != "First line Second line" {
		t.Fatalf("text = %q, want normalized text", text)
	}
}

func TestExtractUploadedContentTextRejectsUnsupportedTypes(t *testing.T) {
	_, _, err := extractUploadedContentText([]byte("hello"), "image/png", "image.png")
	if err == nil {
		t.Fatal("expected unsupported file type error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %q, want unsupported", err.Error())
	}
}
