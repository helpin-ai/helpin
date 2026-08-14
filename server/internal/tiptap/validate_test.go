package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateDocument(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid document",
			raw:  `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hi"}]}]}`,
		},
		{
			name: "empty document body",
			raw:  `{"type":"doc","content":[]}`,
		},
		{
			name: "document without content key",
			raw:  `{"type":"doc"}`,
		},
		{
			name:    "content as a string instead of child nodes",
			raw:     `{"type":"doc","content":[{"type":"paragraph","content":"Hi"}]}`,
			wantErr: "not valid TipTap document JSON",
		},
		{
			name:    "content as an object instead of child nodes",
			raw:     `{"type":"doc","content":[{"type":"paragraph","content":{"type":"text","text":"Hi"}}]}`,
			wantErr: "not valid TipTap document JSON",
		},
		{
			name:    "nested malformed content",
			raw:     `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":"x"}]}]}`,
			wantErr: "not valid TipTap document JSON",
		},
		{
			name:    "paragraph directly inside ordered list",
			raw:     `{"type":"doc","content":[{"type":"orderedList","content":[{"type":"paragraph","content":[{"type":"text","text":"Step"}]}]}]}`,
			wantErr: "children must be listItem",
		},
		{
			name:    "list item starts with heading",
			raw:     `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"heading","attrs":{"level":2}}]}]}]}`,
			wantErr: "must start with a paragraph",
		},
		{
			name:    "block image inside heading",
			raw:     `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"resizableImage","attrs":{"src":"https://example.test/image.png"}}]}]}`,
			wantErr: "cannot contain block node",
		},
		{
			name:    "duplicate text marks",
			raw:     `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Bold","marks":[{"type":"bold"},{"type":"bold"}]}]}]}`,
			wantErr: "duplicate",
		},
		{
			name:    "code combined with another mark",
			raw:     `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Code","marks":[{"type":"code"},{"type":"bold"}]}]}]}`,
			wantErr: "cannot be combined",
		},
		{
			name:    "child node without a type",
			raw:     `{"type":"doc","content":[{"content":[{"type":"text","text":"Hi"}]}]}`,
			wantErr: "must include type",
		},
		{
			name:    "not a document",
			raw:     `{"type":"paragraph","content":[{"type":"text","text":"Hi"}]}`,
			wantErr: `got "paragraph"`,
		},
		{
			name:    "empty input",
			raw:     "   ",
			wantErr: "content is required",
		},
		{
			name:    "not json",
			raw:     `nonsense`,
			wantErr: "not valid TipTap document JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDocument(json.RawMessage(tt.raw))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDocument() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateDocument() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBlockNode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid paragraph",
			raw:  `{"type":"paragraph","content":[{"type":"text","text":"Hi"}]}`,
		},
		{
			name: "valid atom block without content",
			raw:  `{"type":"resizableImage","attrs":{"src":"https://example.test/a.png","alt":"A"}}`,
		},
		{
			name:    "content as a string",
			raw:     `{"type":"paragraph","content":"Hi"}`,
			wantErr: "not valid TipTap block JSON",
		},
		{
			name:    "missing type",
			raw:     `{"content":[{"type":"text","text":"Hi"}]}`,
			wantErr: "must include type",
		},
		{
			name:    "whole document rejected",
			raw:     `{"type":"doc","content":[{"type":"paragraph"}]}`,
			wantErr: "single block node",
		},
		{
			name:    "empty input",
			raw:     "",
			wantErr: "content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlockNode(json.RawMessage(tt.raw))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBlockNode() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBlockNode() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
