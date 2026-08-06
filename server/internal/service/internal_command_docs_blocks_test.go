package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestInsertDocumentBlockInsertsMarkdownBetweenBlocks(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First"}]},{"type":"paragraph","content":[{"type":"text","text":"Second"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}

	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	output, err := svc.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: workspaceID,
		ActorID:     "30000000-0000-0000-0000-000000000002",
		TargetType:  "document",
		TargetID:    documentID,
	}, "docs.insert_document_block", json.RawMessage(`{
		"document_id":"`+documentID+`",
		"after_block_id":"`+blocks[0].ID+`",
		"content":"## Middle heading\n\nMiddle body"
	}`))
	if err != nil {
		t.Fatalf("insert document block: %v", err)
	}
	var result struct {
		BlockIDs []string `json:"block_ids"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if len(result.BlockIDs) != 2 {
		t.Fatalf("expected 2 inserted block ids, got %v", result.BlockIDs)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after insert: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"First", "Middle heading", "Middle body", "Second"})
	if blocks[1].ID != result.BlockIDs[0] || blocks[2].ID != result.BlockIDs[1] {
		t.Fatalf("inserted block ids %v do not match blocks %q, %q", result.BlockIDs, blocks[1].ID, blocks[2].ID)
	}
	if blocks[1].Type != "heading" || blocks[2].Type != "paragraph" {
		t.Fatalf("inserted block types = %q, %q; want heading, paragraph", blocks[1].Type, blocks[2].Type)
	}
}

func TestInsertDocumentBlockSupportsStartPosition(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Existing"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}

	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	if _, err := svc.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: workspaceID,
		ActorID:     "30000000-0000-0000-0000-000000000002",
		TargetType:  "document",
		TargetID:    documentID,
	}, "docs.insert_document_block", json.RawMessage(`{
		"document_id":"`+documentID+`",
		"position":"start",
		"content":"Intro"
	}`)); err != nil {
		t.Fatalf("insert document block at start: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"Intro", "Existing"})
}

func TestInsertDocumentBlockRejectsInvalidInput(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	_, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, contentSvc, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(repository.NewDocsDocumentRepository(db), nil, nil, false), repository.NewDocsContentRepository(db))
	svc.SetDocsBlockService(blockSvc)
	meta := model.InternalCommandContext{
		WorkspaceID: workspaceID,
		TargetType:  "document",
		TargetID:    documentID,
	}

	if _, err := svc.Execute(ctx, meta, "docs.insert_document_block", json.RawMessage(`{"document_id":"`+documentID+`","content":""}`)); err == nil || !strings.Contains(err.Error(), "content is required") {
		t.Fatalf("expected content required error, got %v", err)
	}
	if _, err := svc.Execute(ctx, meta, "docs.insert_document_block", json.RawMessage(`{"document_id":"`+documentID+`","content":"Text","position":"middle"}`)); err == nil || !strings.Contains(err.Error(), "position must be start or end") {
		t.Fatalf("expected position error, got %v", err)
	}
	if _, err := svc.Execute(ctx, meta, "docs.insert_document_block", json.RawMessage(`{"document_id":"`+documentID+`","content":"Text","after_block_id":"99999999-0000-0000-0000-000000000009"}`)); err == nil || !strings.Contains(err.Error(), "after block not found") {
		t.Fatalf("expected after block not found error, got %v", err)
	}
}
