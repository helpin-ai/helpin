package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"gorm.io/gorm"
)

func TestCoverageDraftRejectsSupersededProposal(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	if err := db.Create(&model.SupportCoverageGap{ID: "gap", WorkspaceID: "ws", DedupeKey: "gap", Status: "open", Metadata: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := db.Create(&model.SupportGapSuggestion{ID: "stale", WorkspaceID: "ws", GapID: "gap", SuggestionType: "create_article", Status: "draft", SupersededAt: &now, Metadata: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewSupportCoverageDraftService(repo, nil, nil, nil, nil)
	if err := svc.ApplySuggestion(context.Background(), "ws", "stale", "user"); err == nil || !strings.Contains(err.Error(), "superseded") {
		t.Fatalf("expected stale proposal rejection, got %v", err)
	}
}

func coverageDraftReviewFixture(t *testing.T) (*SupportCoverageDraftService, *DocsContentService, *gorm.DB) {
	t.Helper()
	_, _, db := setupCoverageTestEnv(t)
	docsDB := setupDocsBlockServiceTestDB(t)
	var schema []struct{ SQL string }
	if err := docsDB.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name LIKE 'docs_%'").Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range schema {
		if err := db.Exec(table.SQL).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	repo := repository.NewSupportCoverageRepository(db)
	docRepo := repository.NewDocsDocumentRepository(db)
	_, contentSvc, _ := newDocsBlockServiceTestServices(db)
	docSvc := NewDocsDocumentService(docRepo, nil, nil, false)
	insertDocsBlockServiceTestDocument(t, db, "doc", "ws", false)
	if _, err := contentSvc.Save(ctx, "doc", tiptap.MarkdownToJSON("Existing guidance."), "user"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SupportCoverageGap{ID: "gap", WorkspaceID: "ws", DedupeKey: "gap", Status: "open", Metadata: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	docID := "doc"
	if _, err := repo.CreateSuggestion(ctx, &model.SupportGapSuggestion{ID: "proposal", Title: "Coverage additions", WorkspaceID: "ws", GapID: "gap", SuggestionType: "update_article", Status: "draft", TargetDocumentID: &docID, Content: tiptap.MarkdownToJSON("New verified guidance."), Metadata: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	svc := NewSupportCoverageDraftService(repo, docSvc, contentSvc, nil, nil)
	return svc, contentSvc, db
}

func TestCoverageDraftSaveKeepsGapOpenAndExistingContent(t *testing.T) {
	svc, contentSvc, db := coverageDraftReviewFixture(t)
	ctx := context.Background()
	if err := svc.ApplySuggestion(ctx, "ws", "proposal", "user"); err != nil {
		t.Fatal(err)
	}
	var gap model.SupportCoverageGap
	if err := db.Where("id = ?", "gap").First(&gap).Error; err != nil {
		t.Fatal(err)
	}
	if gap.Status != "open" || gap.ClosedAt != nil {
		t.Fatalf("draft save closed the gap: %+v", gap)
	}
	content, err := contentSvc.Get(ctx, "doc")
	if err != nil {
		t.Fatal(err)
	}
	text := tiptap.RichTextToMarkdown(string(content.Content))
	if !strings.Contains(text, "Existing guidance") || !strings.Contains(text, "New verified guidance") {
		t.Fatalf("lost content: %s", text)
	}
	var linkCount int64
	if err := db.Table("support_coverage_gap_articles").Where("gap_id = ? AND document_id = ?", "gap", "doc").Count(&linkCount).Error; err != nil {
		t.Fatal(err)
	}
	if linkCount != 1 {
		t.Fatal("saved document was not linked for review")
	}
	if err := svc.ApplySuggestion(ctx, "ws", "proposal", "user"); err == nil {
		t.Fatal("applied the same proposal twice")
	}
	current, err := contentSvc.Get(ctx, "doc")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(tiptap.RichTextToMarkdown(string(current.Content)), "New verified guidance") != 1 {
		t.Fatal("repeated application duplicated the additions")
	}
}

func TestCoverageDraftRejectsEmptyGeneratedContent(t *testing.T) {
	if _, err := parseDraftResponse(`{"title":"Empty","content":"  "}`, "Gap", 2); err == nil {
		t.Fatal("accepted an empty generated article")
	}
}

func TestCoverageDraftCreateCommitsWithReceiptOrRollsBack(t *testing.T) {
	for _, rejectReceipt := range []bool{false, true} {
		name := "saved draft"
		if rejectReceipt {
			name = "failed receipt"
		}
		t.Run(name, func(t *testing.T) {
			svc, contentSvc, db := coverageDraftReviewFixture(t)
			orderingDB := setupDocsOrderingTestDB(t)
			var tables []struct{ SQL string }
			if err := orderingDB.Raw("SELECT sql FROM sqlite_master WHERE name IN ('docs_spaces', 'docs_space_teams', 'docs_collections')").Scan(&tables).Error; err != nil {
				t.Fatal(err)
			}
			for _, table := range tables {
				if err := db.Exec(table.SQL).Error; err != nil {
					t.Fatal(err)
				}
			}
			seedDocsSpace(t, db, model.DocsSpace{ID: "space", WorkspaceID: "ws", Name: "Help center", Slug: "help", Visibility: "workspace_wide", Type: "external_capable", CreatedBy: "user"})
			// SQLite has no production UUID default. Assign the new document's ID.
			if err := db.Callback().Create().Before("gorm:create").Register("coverage_create_id_and_receipt", func(tx *gorm.DB) {
				if doc, ok := tx.Statement.Dest.(*model.DocsDocument); ok && doc.ID == "" {
					doc.ID = "new-draft"
				}
				if rejectReceipt && tx.Statement.Table == "support_coverage_gap_articles" {
					tx.AddError(errors.New("receipt unavailable"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			err := svc.ApplyReviewedSuggestion(context.Background(), "ws", "proposal", "user", CoverageSuggestionReview{Route: "create_article", TargetSpaceID: "space"})
			if rejectReceipt {
				if err == nil {
					t.Fatal("saved draft despite failed receipt")
				}
				var count int64
				if err := db.Model(&model.DocsDocument{}).Where("id = ?", "new-draft").Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("partial draft survived rollback: count=%d err=%v", count, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var doc model.DocsDocument
				if err := db.First(&doc, "id = ?", "new-draft").Error; err != nil || doc.Status != "draft" || doc.PublishedAt != nil {
					t.Fatalf("new document was not an unpublished draft: %+v err=%v", doc, err)
				}
				content, err := contentSvc.Get(context.Background(), doc.ID)
				if err != nil || content == nil || !strings.Contains(tiptap.RichTextToMarkdown(string(content.Content)), "New verified guidance") {
					t.Fatalf("draft lost reviewed content: %v", err)
				}
			}
			var gap model.SupportCoverageGap
			if err := db.First(&gap, "id = ?", "gap").Error; err != nil || gap.Status != "open" {
				t.Fatalf("draft creation closed the gap: %+v err=%v", gap, err)
			}
		})
	}
}

func TestCoverageDraftApplyRollsBackIfReceiptCannotBeSaved(t *testing.T) {
	svc, contentSvc, db := coverageDraftReviewFixture(t)
	failure := errors.New("link storage unavailable")
	if err := db.Callback().Create().Before("gorm:create").Register("reject_coverage_receipt", func(tx *gorm.DB) {
		if tx.Statement.Table == "support_coverage_gap_articles" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplySuggestion(context.Background(), "ws", "proposal", "user"); !errors.Is(err, failure) {
		t.Fatalf("expected storage failure, got %v", err)
	}
	content, err := contentSvc.Get(context.Background(), "doc")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(tiptap.RichTextToMarkdown(string(content.Content)), "New verified guidance") {
		t.Fatal("failed receipt left partially saved content")
	}
	var proposal model.SupportGapSuggestion
	if err := db.Where("id = ?", "proposal").First(&proposal).Error; err != nil {
		t.Fatal(err)
	}
	if proposal.Status != "draft" {
		t.Fatalf("proposal became %s despite rollback", proposal.Status)
	}
}

func TestCoverageDraftPersistsExactlyTheReviewedContent(t *testing.T) {
	svc, contentSvc, db := coverageDraftReviewFixture(t)
	title := "Reviewed additions"
	reviewed := tiptap.MarkdownToJSON("The reviewed process.")
	if err := svc.ApplyReviewedSuggestion(context.Background(), "ws", "proposal", "user", CoverageSuggestionReview{Title: &title, Content: reviewed}); err != nil {
		t.Fatal(err)
	}
	content, err := contentSvc.Get(context.Background(), "doc")
	if err != nil {
		t.Fatal(err)
	}
	text := tiptap.RichTextToMarkdown(string(content.Content))
	if !strings.Contains(text, "The reviewed process") || strings.Contains(text, "New verified guidance") {
		t.Fatalf("saved unreviewed content: %s", text)
	}
	var proposal model.SupportGapSuggestion
	if err := db.Where("id = ?", "proposal").First(&proposal).Error; err != nil {
		t.Fatal(err)
	}
	if proposal.Title != title || !strings.Contains(tiptap.RichTextToMarkdown(string(proposal.Content)), "The reviewed process") {
		t.Fatal("receipt does not reflect the reviewed proposal")
	}
}

func TestCoverageDraftRejectsClosedLockedAndMalformedFixes(t *testing.T) {
	for _, scenario := range []string{"closed gap", "locked article", "invalid content"} {
		t.Run(scenario, func(t *testing.T) {
			svc, contentSvc, db := coverageDraftReviewFixture(t)
			review := CoverageSuggestionReview{}
			if scenario == "closed gap" {
				if err := db.Exec("UPDATE support_coverage_gaps SET status='done' WHERE id='gap'").Error; err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "locked article" {
				if err := db.Exec("UPDATE docs_documents SET is_locked=1 WHERE id='doc'").Error; err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "invalid content" {
				review.Content = json.RawMessage(`{"type":"doc","content":"invalid"}`)
			}
			if err := svc.ApplyReviewedSuggestion(context.Background(), "ws", "proposal", "user", review); err == nil {
				t.Fatal("invalid review was accepted")
			}
			content, err := contentSvc.Get(context.Background(), "doc")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(tiptap.RichTextToMarkdown(string(content.Content)), "New verified guidance") {
				t.Fatal("rejected review changed the document")
			}
		})
	}
}
