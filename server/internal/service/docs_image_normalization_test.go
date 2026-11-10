package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func TestDocsSavesNormalizeImages(t *testing.T) {
	for _, mode := range []string{"save", "versioned", "block"} {
		t.Run(mode, func(t *testing.T) {
			db := setupDocsBlockServiceTestDB(t)
			const docID = "10000000-0000-0000-0000-000000000001"
			insertDocsBlockServiceTestDocument(t, db, docID, "20000000-0000-0000-0000-000000000001", false)
			repo, svc, blocks := newDocsBlockServiceTestServices(db)
			ctx := context.Background()
			initial, err := svc.Save(ctx, docID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`), "")
			if err != nil {
				t.Fatal(err)
			}
			image := json.RawMessage(`{"type":"image","attrs":{"src":"https://example.test/picture.png","alt":"Original description","width":320}}`)
			content := json.RawMessage(`{"type":"doc","content":[` + string(image) + `]}`)
			switch mode {
			case "save":
				_, err = svc.Save(ctx, docID, content, "")
			case "versioned":
				_, err = svc.SaveVersioned(ctx, docID, content, "", tiptap.DocumentVersion(initial.Content))
			case "block":
				_, err = blocks.Create(ctx, docID, nil, image, "")
			}
			if err != nil {
				t.Fatal(err)
			}
			saved, err := svc.Get(ctx, docID)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(saved.Content), `"resizableImage"`) || strings.Contains(string(saved.Content), `"type":"image"`) || !strings.Contains(string(saved.Content), "Original description") {
				t.Fatalf("invalid saved image: %s", saved.Content)
			}
			rows, err := repo.ListByDocument(ctx, docID, false)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, r := range rows {
				if r.Type == "resizableImage" {
					found = true
				}
			}
			if !found {
				t.Fatal("addressable blocks were not normalized")
			}
		})
	}
}

func TestDocsRejectInvalidImagesWithoutOverwritingContent(t *testing.T) {
	for _, raw := range []string{
		`{"type":"doc","content":[{"type":"unknownWidget"}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"image"}]}]}`,
		`{"type":"doc","content":[{"type":"image","content":[{"type":"text","text":"Hidden content"}]}]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			db := setupDocsBlockServiceTestDB(t)
			const docID = "10000000-0000-0000-0000-000000000001"
			insertDocsBlockServiceTestDocument(t, db, docID, "20000000-0000-0000-0000-000000000001", false)
			_, svc, _ := newDocsBlockServiceTestServices(db)
			ctx := context.Background()
			before, err := svc.Save(ctx, docID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Keep this"}]}]}`), "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = svc.Save(ctx, docID, json.RawMessage(raw), ""); err == nil {
				t.Fatal("invalid content accepted")
			}
			after, err := svc.Get(ctx, docID)
			if err != nil {
				t.Fatal(err)
			}
			if string(before.Content) != string(after.Content) {
				t.Fatal("invalid save overwrote the original")
			}
		})
	}
}
