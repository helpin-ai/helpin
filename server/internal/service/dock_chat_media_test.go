package service

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestValidateDockChatMediaSignature(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		wantErr     bool
	}{
		{name: "png", contentType: "image/png", body: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}},
		{name: "mp4", contentType: "video/mp4", body: []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}},
		{name: "mismatched png", contentType: "image/png", body: []byte("not an image"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDockChatMediaSignature(tt.contentType, tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDockChatMediaSignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDockChatAttachmentTypesIncludeDocuments(t *testing.T) {
	for _, contentType := range []string{
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"text/plain",
		"text/markdown",
		"text/csv",
		"application/json",
	} {
		if !isDockChatAttachmentType(contentType) {
			t.Errorf("isDockChatAttachmentType(%q) = false, want true", contentType)
		}
	}
}

func TestExtractUploadedContentTextReadsDOCX(t *testing.T) {
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	document, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create DOCX document: %v", err)
	}
	if _, err := document.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Quarterly report</w:t></w:r></w:p><w:p><w:r><w:t>Revenue increased.</w:t></w:r></w:p></w:body></w:document>`)); err != nil {
		t.Fatalf("write DOCX document: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close DOCX archive: %v", err)
	}

	text, _, err := extractUploadedContentText(payload.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "report.docx")
	if err != nil {
		t.Fatalf("extract DOCX: %v", err)
	}
	if text != "Quarterly report\nRevenue increased." {
		t.Fatalf("DOCX text = %q", text)
	}
}
