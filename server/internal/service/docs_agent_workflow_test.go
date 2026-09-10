package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func documentWorkflowFixture(t *testing.T, markdown string) (*InternalCommandService, model.InternalCommandContext) {
	t.Helper()
	db := setupDocsBlockServiceTestDB(t)
	meta := model.InternalCommandContext{WorkspaceID: "20000000-0000-0000-0000-000000000001", TargetID: "10000000-0000-0000-0000-000000000001", TargetType: "document", ActorID: "30000000-0000-0000-0000-000000000001"}
	insertDocsBlockServiceTestDocument(t, db, meta.TargetID, meta.WorkspaceID, false)
	_, content, blocks := newDocsBlockServiceTestServices(db)
	if _, err := content.Save(context.Background(), meta.TargetID, tiptap.MarkdownToJSON(markdown), meta.ActorID); err != nil {
		t.Fatal(err)
	}
	svc := NewInternalCommandService(nil, nil, nil, nil, content, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), content.contentRepo)
	svc.SetDocsBlockService(blocks)
	return svc, meta
}

func executeDocumentWorkflow(t *testing.T, svc *InternalCommandService, meta model.InternalCommandContext, tool string, input any) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	output, err := svc.Execute(context.Background(), meta, "docs."+tool, raw)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if len([]rune(string(output))) > documentOutputBudget {
		t.Fatalf("%s exceeded output budget: %d", tool, len(output))
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeDocumentField[T any](t *testing.T, result map[string]json.RawMessage, field string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(result[field], &value); err != nil {
		t.Fatalf("decode %s: %v", field, err)
	}
	return value
}

func TestDocumentWorkflowReadsCompleteContentAndNavigatesSections(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		large      bool
	}{
		{"medium", strings.Repeat("Context with useful details. ", 180), false},
		{"large", strings.Repeat("Context with useful details. ", 1800), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, meta := documentWorkflowFixture(t, "## Context\n\n"+tc.body+"\n\n## Retry policy\n\nThe retry interval is 30 seconds.\n\n### Errors\n\nRetry transient failures.\n\n## Appendix\n\nReferences.")
			result := executeDocumentWorkflow(t, svc, meta, "read_document", map[string]any{"document_id": meta.TargetID})
			if tc.large {
				if got := decodeDocumentField[string](t, result, "mode"); got != "outline" {
					t.Fatalf("mode=%s", got)
				}
				if decodeDocumentField[bool](t, result, "content_complete") {
					t.Fatal("outline claimed full content")
				}
			} else {
				if got := decodeDocumentField[string](t, result, "mode"); got != "full" {
					t.Fatalf("mode=%s", got)
				}
				if !strings.Contains(string(result["blocks"]), strings.TrimSpace(tc.body)) {
					t.Fatal("medium document was clipped")
				}
			}
			outline := executeDocumentWorkflow(t, svc, meta, "read_document", map[string]any{"document_id": meta.TargetID, "mode": "outline"})
			sections := decodeDocumentField[[]documentSection](t, outline, "sections")
			section := sections[1]
			selected := executeDocumentWorkflow(t, svc, meta, "get_document_blocks", map[string]any{"document_id": meta.TargetID, "section_id": section.ID})
			blocks := decodeDocumentField[[]documentReadBlock](t, selected, "blocks")
			if len(blocks) != 4 || blocks[0].ID != section.StartBlockID || blocks[3].ID != section.EndBlockID {
				t.Fatalf("incorrect section boundaries: %+v", blocks)
			}
			search := executeDocumentWorkflow(t, svc, meta, "get_document_blocks", map[string]any{"document_id": meta.TargetID, "query": "RETRY interval"})
			hits := decodeDocumentField[[]documentReadBlock](t, search, "blocks")
			if len(hits) != 3 || !hits[1].Match || hits[0].ID != blocks[0].ID || hits[2].ID != blocks[2].ID {
				t.Fatalf("missing search neighbors: %+v", hits)
			}
		})
	}
}

func TestDocumentWorkflowContinuesOversizedBlocksWithoutLoss(t *testing.T) {
	body := strings.Repeat("Unicode π and quoted \"text\" with <tags>. ", 1700)
	svc, meta := documentWorkflowFixture(t, "```text\n"+body+"\n```\n\nFinal paragraph.")
	var reconstructed strings.Builder
	request := map[string]any{"document_id": meta.TargetID, "mode": "full"}
	var firstCursor string
	sawEnd := false
	for calls := 0; calls < 30; calls++ {
		result := executeDocumentWorkflow(t, svc, meta, "read_document", request)
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(result["blocks"], &items); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if _, ok := item["content_fragment"]; ok {
				fragment := decodeDocumentField[string](t, item, "content_fragment")
				offset := decodeDocumentField[int](t, item, "fragment_offset")
				if offset != len([]rune(reconstructed.String())) {
					t.Fatal("fragment gap or overlap")
				}
				reconstructed.WriteString(fragment)
			} else if strings.Contains(string(item["markdown"]), "Final paragraph.") {
				sawEnd = true
			}
		}
		cursor := decodeDocumentField[*string](t, result, "next_cursor")
		if cursor == nil {
			break
		}
		if firstCursor == "" {
			firstCursor = *cursor
		}
		request = map[string]any{"document_id": meta.TargetID, "cursor": *cursor}
	}
	var block documentReadBlock
	if err := json.Unmarshal([]byte(reconstructed.String()), &block); err != nil {
		t.Fatalf("fragment reconstruction failed: %v", err)
	}
	if !strings.Contains(block.Markdown, body) || !sawEnd {
		t.Fatal("full read lost content")
	}
	if _, err := svc.docsContentService.Save(context.Background(), meta.TargetID, tiptap.MarkdownToJSON("Changed"), meta.ActorID); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"document_id": meta.TargetID, "cursor": firstCursor})
	if _, err := svc.Execute(context.Background(), meta, "docs.read_document", raw); err == nil || !strings.Contains(err.Error(), "document changed") {
		t.Fatalf("stale cursor accepted: %v", err)
	}
}

func TestDocumentWorkflowEditsAtomicallyPreservingRichBlocks(t *testing.T) {
	svc, meta := documentWorkflowFixture(t, "**The old** _policy_ remains.\n\n```nwdiag\nnwdiag { network app { web; } }\n```\n\nLast paragraph.")
	snapshot, original, err := svc.docsContentService.contentRepo.ReadSnapshot(context.Background(), meta.TargetID)
	if err != nil {
		t.Fatal(err)
	}
	version := tiptap.DocumentVersion(snapshot.Content)
	input := map[string]any{"document_id": meta.TargetID, "expected_version": version, "operations": []map[string]any{
		{"type": "replace_text", "block_id": original[0].ID, "old_text": "old policy", "new_text": "new guidance"},
		{"type": "insert", "after_block_id": original[2].ID, "content": "## Explanation\n\nUpdated guidance."},
	}}
	result := executeDocumentWorkflow(t, svc, meta, "edit_document", input)
	saved, blocks, err := svc.docsContentService.contentRepo.ReadSnapshot(context.Background(), meta.TargetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 5 || blocks[0].ID != original[0].ID || blocks[0].Revision != original[0].Revision+1 {
		t.Fatalf("wrong edit result: %+v", blocks)
	}
	if string(blocks[1].Content) != string(original[1].Content) || blocks[1].Revision != original[1].Revision || blocks[2].Revision != original[2].Revision {
		t.Fatal("untouched rich content changed")
	}
	if !strings.Contains(string(blocks[0].Content), "new guidance") || !strings.Contains(string(blocks[0].Content), "bold") {
		t.Fatal("text or formatting lost")
	}
	if decodeDocumentField[string](t, result, "version") != tiptap.DocumentVersion(saved.Content) {
		t.Fatal("receipt version does not identify committed content")
	}
	changes := decodeDocumentField[[]map[string]any](t, result, "changed_blocks")
	if len(changes) != 3 || changes[0]["revision"] != float64(blocks[0].Revision) {
		t.Fatalf("missing revision receipt: %+v", changes)
	}
	// A stale version or a later invalid operation must leave the entire batch unapplied.
	for _, tc := range []struct {
		name, version string
		ops           []map[string]any
	}{
		{"stale", version, []map[string]any{{"type": "delete_range", "start_block_id": blocks[2].ID, "end_block_id": blocks[2].ID}}},
		{"invalid later operation", tiptap.DocumentVersion(saved.Content), []map[string]any{{"type": "replace_text", "block_id": blocks[2].ID, "old_text": "Last", "new_text": "Changed"}, {"type": "delete_range", "start_block_id": "missing", "end_block_id": "missing"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"document_id": meta.TargetID, "expected_version": tc.version, "operations": tc.ops})
			if _, err := svc.Execute(context.Background(), meta, "docs.edit_document", raw); err == nil {
				t.Fatal("invalid batch succeeded")
			}
			current, err := svc.docsContentService.Get(context.Background(), meta.TargetID)
			if err != nil {
				t.Fatal(err)
			}
			if tiptap.DocumentVersion(current.Content) != tiptap.DocumentVersion(saved.Content) {
				t.Fatal("failed batch partially changed document")
			}
		})
	}
	otherWorkspace := meta
	otherWorkspace.WorkspaceID = "20000000-0000-0000-0000-000000000099"
	for _, tool := range []string{"read_document", "edit_document"} {
		raw, _ := json.Marshal(input)
		if tool == "read_document" {
			raw, _ = json.Marshal(map[string]any{"document_id": meta.TargetID})
		}
		if _, err := svc.Execute(context.Background(), otherWorkspace, "docs."+tool, raw); err == nil {
			t.Fatalf("%s crossed workspace boundary", tool)
		}
	}

	// Apply a section replacement and deletion against the same snapshot.
	result = executeDocumentWorkflow(t, svc, meta, "edit_document", map[string]any{
		"document_id": meta.TargetID, "expected_version": tiptap.DocumentVersion(saved.Content),
		"operations": []map[string]any{
			{"type": "replace_range", "start_block_id": blocks[3].ID, "end_block_id": blocks[4].ID, "content": "## Revised explanation\n\nFinal guidance."},
			{"type": "delete_range", "start_block_id": blocks[2].ID, "end_block_id": blocks[2].ID},
		},
	})
	_, finalBlocks, err := svc.docsContentService.contentRepo.ReadSnapshot(context.Background(), meta.TargetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(finalBlocks) != 4 || finalBlocks[1].ID != original[1].ID || finalBlocks[2].ID == blocks[3].ID || finalBlocks[3].ContentText != "Final guidance." {
		t.Fatalf("incorrect section replacement: %+v", finalBlocks)
	}
	if len(decodeDocumentField[[]string](t, result, "deleted_block_ids")) != 3 {
		t.Fatal("missing deletion receipt")
	}

}
