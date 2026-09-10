package commandtools

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestDocumentEditSchemaValidatesOperationContracts(t *testing.T) {
	meta, ok := ToolMetadataForAlias("edit_document")
	if !ok {
		t.Fatal("missing edit_document")
	}
	raw, err := json.Marshal(meta.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, operation string
		valid           bool
	}{
		{"text", `{"type":"replace_text","block_id":"b1","old_text":"old","new_text":"new"}`, true},
		{"remove text", `{"type":"replace_text","block_id":"b1","old_text":"old","new_text":""}`, true},
		{"missing block", `{"type":"replace_text","old_text":"old","new_text":"new"}`, false},
		{"missing old", `{"type":"replace_text","block_id":"b1","new_text":"new"}`, false},
		{"empty old", `{"type":"replace_text","block_id":"b1","old_text":"","new_text":"new"}`, false},
		{"missing new", `{"type":"replace_text","block_id":"b1","old_text":"old"}`, false},
		{"empty block", `{"type":"replace_text","block_id":"","old_text":"old","new_text":"new"}`, false},
		{"mixed fields", `{"type":"replace_text","block_id":"b1","old_text":"old","new_text":"new","content":"bad"}`, false},
		{"range", `{"type":"replace_range","start_block_id":"b1","end_block_id":"b2","content":"new"}`, true},
		{"range missing end", `{"type":"replace_range","start_block_id":"b1","content":"new"}`, false},
		{"delete", `{"type":"delete_range","start_block_id":"b1","end_block_id":"b2"}`, true},
		{"delete with content", `{"type":"delete_range","start_block_id":"b1","end_block_id":"b2","content":"new"}`, false},
		{"insert after", `{"type":"insert","after_block_id":"b1","content":"new"}`, true},
		{"insert before", `{"type":"insert","before_block_id":"b1","content":[{"type":"paragraph"}]}`, true},
		{"insert position", `{"type":"insert","position":"end","content":"new"}`, true},
		{"insert two anchors", `{"type":"insert","after_block_id":"b1","position":"end","content":"new"}`, false},
		{"insert no anchor", `{"type":"insert","content":"new"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input any
			if err := json.Unmarshal([]byte(`{"document_id":"doc","expected_version":"v1","operations":[`+tc.operation+`]}`), &input); err != nil {
				t.Fatal(err)
			}
			if err := resolved.Validate(input); (err == nil) != tc.valid {
				t.Fatalf("valid=%v, got %v", tc.valid, err)
			}
		})
	}
}
