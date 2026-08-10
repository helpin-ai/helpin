package service

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAISectionRunToolsForAgentUsesSafeDocumentResearchSubset(t *testing.T) {
	agent := &model.Agent{
		AllowedTools: json.RawMessage(`[
			"write_document_content",
			"search_documents",
			"read_document",
			"web_search_exa",
			"fetch_url",
			"publish_ai_section_candidate",
			"create_task",
			"list_contacts"
		]`),
	}

	tools := aiSectionRunToolsForAgent(agent)
	for _, expected := range []string{"search_documents", "read_document", "web_search_exa", "fetch_url", "publish_ai_section_candidate", "list_contacts"} {
		if !slices.Contains(tools, expected) {
			t.Fatalf("expected %q in AI section tools, got %v", expected, tools)
		}
	}
	for _, forbidden := range []string{"write_document_content", "create_task"} {
		if slices.Contains(tools, forbidden) {
			t.Fatalf("did not expect mutation tool %q in AI section tools, got %v", forbidden, tools)
		}
	}
}

func TestValidateAISectionInstructionsRequiresPrompt(t *testing.T) {
	_, err := validateAISectionInstructions(nil)
	if err == nil {
		t.Fatal("expected nil instructions to be rejected")
	}
	blank := "   "
	_, err = validateAISectionInstructions(&blank)
	if err == nil {
		t.Fatal("expected blank instructions to be rejected")
	}
}

func TestValidateAISectionInstructionsTrimsPrompt(t *testing.T) {
	raw := "  check https://usermaven.com/pricing and update pricing  "
	got, err := validateAISectionInstructions(&raw)
	if err != nil {
		t.Fatalf("validate instructions: %v", err)
	}
	if got != "check https://usermaven.com/pricing and update pricing" {
		t.Fatalf("instructions = %q", got)
	}
}

func TestAISectionContentEqualForApprovalIgnoresReviewMetadata(t *testing.T) {
	snapshot := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Pricing","status":"draft"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)
	current := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{
			"blockId":"block-1",
			"title":"Pricing",
			"status":"needs_review",
			"ownerAgentId":"agent-1",
			"lastGeneratedAt":"2026-04-30T00:00:00Z",
			"model":"openai/gpt-5",
			"sourceCount":2
		},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)

	if !aiSectionContentEqualForApproval(snapshot, current) {
		t.Fatal("expected review metadata-only changes to be approval-compatible")
	}
}

func TestAISectionContentEqualForApprovalIgnoresTitleMetadata(t *testing.T) {
	snapshot := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Pricing","status":"draft"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)
	current := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Updated pricing","status":"draft"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)

	if !aiSectionContentEqualForApproval(snapshot, current) {
		t.Fatal("expected title-only changes to be approval-compatible")
	}
}

func TestAISectionContentEqualForApprovalRejectsBodyEdits(t *testing.T) {
	snapshot := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Pricing","status":"draft"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)
	current := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Pricing","status":"needs_review"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Manually edited pricing"}]}]
	}`)

	if aiSectionContentEqualForApproval(snapshot, current) {
		t.Fatal("expected body edits to keep candidate stale")
	}
}

func TestAISectionCandidateContentWithStatusMarksApproved(t *testing.T) {
	raw := json.RawMessage(`{"type":"aiSection","attrs":{"status":"needs_review"},"content":[{"type":"paragraph"}]}`)

	approved, err := aiSectionCandidateContentWithStatus(raw, model.DocsAISectionCandidateStatusApproved)
	if err != nil {
		t.Fatalf("mark approved: %v", err)
	}
	var decoded struct {
		Attrs map[string]any `json:"attrs"`
	}
	if err := json.Unmarshal(approved, &decoded); err != nil {
		t.Fatalf("decode approved content: %v", err)
	}
	if decoded.Attrs["status"] != "approved" {
		t.Fatalf("status = %v, want approved", decoded.Attrs["status"])
	}
}

func TestAISectionCandidateContentWithStatusPreservesCurrentTitle(t *testing.T) {
	candidate := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Old title","status":"needs_review"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"New pricing"}]}]
	}`)
	current := json.RawMessage(`{
		"type":"aiSection",
		"attrs":{"blockId":"block-1","title":"Current title","status":"draft"},
		"content":[{"type":"paragraph","content":[{"type":"text","text":"Old pricing"}]}]
	}`)

	approved, err := aiSectionCandidateContentWithStatusForCurrent(candidate, current, model.DocsAISectionCandidateStatusApproved)
	if err != nil {
		t.Fatalf("mark approved: %v", err)
	}
	var decoded struct {
		Attrs map[string]any `json:"attrs"`
	}
	if err := json.Unmarshal(approved, &decoded); err != nil {
		t.Fatalf("decode approved content: %v", err)
	}
	if decoded.Attrs["title"] != "Current title" {
		t.Fatalf("title = %v, want Current title", decoded.Attrs["title"])
	}
	if decoded.Attrs["status"] != "approved" {
		t.Fatalf("status = %v, want approved", decoded.Attrs["status"])
	}
}

func TestAISectionCandidateContentWithStatusRejectsInvalidJSON(t *testing.T) {
	_, err := aiSectionCandidateContentWithStatus(json.RawMessage(`{`), model.DocsAISectionCandidateStatusApproved)
	if err == nil || errors.Is(err, ErrDocsStaleBlockRevision) {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestAISectionRunToolsForAgentDoesNotGrantUnavailableTools(t *testing.T) {
	agent := &model.Agent{
		AllowedTools: json.RawMessage(`["search_documents"]`),
	}

	tools := aiSectionRunToolsForAgent(agent)
	if len(tools) != 2 || tools[0] != "search_documents" || tools[1] != "publish_ai_section_candidate" {
		t.Fatalf("expected search_documents plus publish tool, got %v", tools)
	}
}
