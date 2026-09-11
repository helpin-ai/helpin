package commandtools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func TestDocsToolsExposeRenderableNwdiagExamples(t *testing.T) {
	for _, alias := range []string{"create_document", "write_document_content", "insert_document_block", "update_document_block"} {
		t.Run(alias, func(t *testing.T) {
			meta, ok := ToolMetadataForAlias(alias)
			if !ok {
				t.Fatal("Docs tool is missing")
			}
			if !strings.Contains(meta.Description, "nwdiag") || !strings.Contains(meta.Description, "simplediag") {
				t.Fatal("Docs tool must advertise network diagram support")
			}
			properties := meta.InputSchema["properties"].(map[string]any)
			content := properties["content"].(map[string]any)
			description := content["description"].(string)
			var block tiptap.Node
			if content["type"] == "object" {
				_, example, found := strings.Cut(description, "Example: ")
				if !found {
					t.Fatal("Structured content needs a block JSON example")
				}
				raw := json.RawMessage(strings.TrimSuffix(example, "."))
				if err := tiptap.ValidateBlockNode(raw); err != nil {
					t.Fatalf("Advertised JSON example is not accepted by block tools: %v", err)
				}
				if err := json.Unmarshal(raw, &block); err != nil {
					t.Fatal(err)
				}
			} else {
				start := strings.Index(description, "```nwdiag\n")
				if start < 0 {
					t.Fatal("Markdown content needs a fenced nwdiag example")
				}
				end := strings.Index(description[start+3:], "```")
				if end < 0 {
					t.Fatal("Example code fence is not closed")
				}
				raw := tiptap.MarkdownToJSON(description[start : start+3+end+3])
				if err := tiptap.ValidateDocument(raw); err != nil {
					t.Fatalf("Advertised Markdown example is not accepted by document tools: %v", err)
				}
				var doc tiptap.Node
				if err := json.Unmarshal(raw, &doc); err != nil {
					t.Fatal(err)
				}
				if len(doc.Content) != 1 {
					t.Fatalf("Expected one diagram block, got %d", len(doc.Content))
				}
				block = doc.Content[0]
			}
			if block.Type != "codeBlock" || block.Attrs["language"] != "nwdiag" || len(block.Content) != 1 {
				t.Fatalf("Example does not produce the nwdiag editor node: %+v", block)
			}
			if !strings.Contains(block.Content[0].Text, "web01") || !strings.Contains(block.Content[0].Text, "web02") {
				t.Fatalf("Network source was lost: %+v", block.Content)
			}
		})
	}
}
