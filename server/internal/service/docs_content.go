package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsContentService handles business logic for document content.
type DocsContentService struct {
	contentRepo    *repository.DocsContentRepository
	docRepo        *repository.DocsDocumentRepository
	translationSvc *DocsHelpcenterTranslationService
	wsPublisher    *websocket.Publisher
}

// NewDocsContentService creates a new DocsContentService.
func NewDocsContentService(contentRepo *repository.DocsContentRepository, docRepo *repository.DocsDocumentRepository, wsPublisher *websocket.Publisher) *DocsContentService {
	return &DocsContentService{contentRepo: contentRepo, docRepo: docRepo, wsPublisher: wsPublisher}
}

func (s *DocsContentService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

// Get returns the content for a document.
func (s *DocsContentService) Get(ctx context.Context, documentID string) (*model.DocsContent, error) {
	return s.contentRepo.GetByDocumentID(ctx, documentID)
}

// Save creates or updates document content.
// Automatically extracts content_text and computes word_count in the repository layer.
func (s *DocsContentService) Save(ctx context.Context, documentID string, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	saved, err := s.contentRepo.UpsertWithActor(ctx, documentID, content, actorID)
	if err != nil {
		return nil, err
	}
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after content save", "document_id", documentID, "error", err)
		}
	}
	if s.docRepo != nil {
		if doc, err := s.docRepo.GetByID(ctx, documentID); err == nil && doc != nil {
			publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, doc.WorkspaceID, actorID)
		}
	}
	return saved, nil
}

// ListBySpaceWithImportHTML returns content records that have stored import HTML for a space.
func (s *DocsContentService) ListBySpaceWithImportHTML(ctx context.Context, spaceID string) ([]model.DocsContent, error) {
	return s.contentRepo.ListBySpaceWithImportHTML(ctx, spaceID)
}

// SetImportProvenance stores the original import HTML and source metadata on a content record.
// This is a snapshot for reconversion/debugging — not the live source of truth.
func (s *DocsContentService) SetImportProvenance(ctx context.Context, contentID, sourceHTML, sourceSystem, sourceObjectID string) {
	s.contentRepo.UpdateImportProvenance(ctx, contentID, sourceHTML, sourceSystem, sourceObjectID)
}
