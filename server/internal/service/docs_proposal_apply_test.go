package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Reproduces the production apply path for a block-scoped proposal: the tool
// builds the replacement node from markdown, then Apply patches the block.
func TestApplyBlockProposalReplacesBlockContent(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	initial := `{"type":"doc","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Intro paragraph"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"Second paragraph"}]}]}`
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(initial), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	target := blocks[0]

	replacement, err := commandProposalBlockContentFromMarkdown(target.Content, "The URL redirects to the login page.")
	if err != nil {
		t.Fatalf("build proposal content: %v", err)
	}

	if _, err := blockSvc.Patch(ctx, documentID, target.ID, target.Revision, replacement, "30000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatalf("apply proposal: %v", err)
	}

	after, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after apply: %v", err)
	}
	assertBlockTexts(t, after, []string{"The URL redirects to the login page.", "Second paragraph"})
	if after[0].ID != target.ID {
		t.Fatalf("block identity changed on apply: %q -> %q", target.ID, after[0].ID)
	}

	saved, err := contentSvc.Get(ctx, documentID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	// The converter may split a sentence across adjacent text nodes, so assert
	// on the derived plain text rather than the raw JSON.
	if !strings.Contains(saved.ContentText, "The URL redirects to the login page.") {
		t.Fatalf("aggregate missing replacement text: %s", saved.ContentText)
	}
	if !strings.Contains(saved.ContentText, "Second paragraph") {
		t.Fatalf("aggregate lost a sibling block: %s", saved.ContentText)
	}
}

// A proposal whose markdown expands to several blocks must not silently drop
// everything past the first node.
func TestProposalBlockContentFromMultiBlockMarkdown(t *testing.T) {
	current := json.RawMessage(`{"type":"paragraph","attrs":{"blockId":"11111111-1111-4111-8111-111111111111"},"content":[{"type":"text","text":"Old"}]}`)
	_, err := commandProposalBlockContentFromMarkdown(current, "## Heading\n\nBody paragraph")
	if err == nil {
		t.Fatal("expected multi-block proposal markdown to be rejected for a single-block proposal")
	}
	if !strings.Contains(err.Error(), "exactly one block") {
		t.Fatalf("error = %v, want it to require exactly one block", err)
	}
}
