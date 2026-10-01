package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"gorm.io/gorm"
)

var ErrCoverageDraftReview = repository.ErrCoverageDraftReview

// CoverageSuggestionReview carries the proposal and destination approved by the reviewer.
type CoverageSuggestionReview struct {
	Route              string          `json:"route"`
	TargetDocumentID   string          `json:"target_document_id"`
	TargetSpaceID      string          `json:"target_space_id"`
	TargetCollectionID *string         `json:"target_collection_id"`
	Title              *string         `json:"title"`
	Content            json.RawMessage `json:"content"`
}

func (s *SupportCoverageDraftService) ApplyReviewedSuggestion(ctx context.Context, workspaceID, suggestionID, userID string, review CoverageSuggestionReview) error {
	effects := &coverageReviewEffects{}
	err := s.coverageRepo.WithCoverageProposal(ctx, workspaceID, suggestionID, func(tx *gorm.DB, suggestion *model.SupportGapSuggestion) error {
		if review.Title != nil {
			suggestion.Title = strings.TrimSpace(*review.Title)
		}
		if strings.TrimSpace(suggestion.Title) == "" {
			return fmt.Errorf("%w: article title is required", ErrCoverageDraftReview)
		}
		if review.Content != nil {
			suggestion.Content = review.Content
		}
		if err := tiptap.ValidateDocument(suggestion.Content); err != nil {
			return fmt.Errorf("%w: invalid proposal content: %v", ErrCoverageDraftReview, err)
		}
		if strings.TrimSpace(tiptap.RichTextToMarkdown(string(suggestion.Content))) == "" {
			return fmt.Errorf("%w: proposal content is empty", ErrCoverageDraftReview)
		}
		if review.TargetSpaceID != "" {
			suggestion.TargetSpaceID = &review.TargetSpaceID
		}
		if review.TargetCollectionID != nil {
			suggestion.TargetCollectionID = review.TargetCollectionID
		}
		if review.TargetDocumentID != "" {
			suggestion.TargetDocumentID = &review.TargetDocumentID
		}
		if review.Route != "" {
			suggestion.SuggestionType = review.Route
		}
		if suggestion.SuggestionType != model.SupportCoverageSuggestionCreateArticle && suggestion.SuggestionType != model.SupportCoverageSuggestionUpdateArticle {
			return fmt.Errorf("%w: unsupported save route", ErrCoverageDraftReview)
		}
		bound := s.bindReviewTransaction(tx, effects)
		if suggestion.SuggestionType == model.SupportCoverageSuggestionCreateArticle {
			if suggestion.TargetSpaceID == nil || *suggestion.TargetSpaceID == "" {
				return fmt.Errorf("%w: choose a docs space", ErrCoverageDraftReview)
			}
			if err := bound.coverageRepo.ValidateCoverageDraftDestination(ctx, workspaceID, *suggestion.TargetSpaceID, suggestion.TargetCollectionID); err != nil {
				return err
			}
			suggestion.TargetDocumentID = nil
			return bound.applyCreateArticle(ctx, suggestion, userID)
		}
		suggestion.TargetSpaceID, suggestion.TargetCollectionID = nil, nil
		return bound.applyUpdateArticle(ctx, suggestion, userID)
	})
	if err == nil {
		effects.publish(ctx, s, userID)
	}
	return err
}

func (s *SupportCoverageDraftService) bindReviewTransaction(tx *gorm.DB, effects *coverageReviewEffects) *SupportCoverageDraftService {
	bound := *s
	bound.reviewEffects = effects
	bound.coverageRepo = repository.NewSupportCoverageRepository(tx)
	useSortKey := s.documentSvc != nil && s.documentSvc.useSortKey
	docRepo := repository.NewDocsDocumentRepository(tx, useSortKey)
	contentRepo := repository.NewDocsContentRepository(tx)
	contentRepo.SetBlockRepository(repository.NewDocsBlockRepository(tx))
	if s.documentSvc != nil {
		documents := *s.documentSvc
		documents.wsPublisher = nil
		documents.productAnalyticsEmitter = productAnalyticsEmitter{}
		documents.docRepo, documents.spaceRepo = docRepo, repository.NewDocsSpaceRepository(tx)
		bound.documentSvc = &documents
	}
	if s.contentSvc != nil {
		content := *s.contentSvc
		content.wsPublisher, content.translationSvc, content.notificationService, content.agentService = nil, nil, nil, nil
		content.contentRepo, content.docRepo = contentRepo, docRepo
		bound.contentSvc = &content
	}
	if s.versionSvc != nil {
		versions := *s.versionSvc
		versions.wsPublisher = nil
		versions.versionRepo, versions.contentRepo, versions.docRepo = repository.NewDocsVersionRepository(tx), contentRepo, docRepo
		bound.versionSvc = &versions
	}
	return &bound
}

func coverageReviewSnapshotLabel() *string {
	label := "Before coverage additions"
	return &label
}

// Emit updates only after the document and proposal receipt have both committed.
type coverageReviewEffects struct {
	created      *model.DocsDocument
	content      *model.DocsContent
	version      *model.DocsVersion
	previousText string
}

func (e *coverageReviewEffects) publish(ctx context.Context, s *SupportCoverageDraftService, actorID string) {
	if e.created != nil && s.documentSvc != nil {
		doc := e.created
		publishWorkspaceEventWithParent(s.documentSvc.wsPublisher, "created", "docs_document", doc.ID, doc.WorkspaceID, actorID, "docs_space", doc.SpaceID, nil)
		s.documentSvc.trackProductEvent(ctx, ProductAnalyticsEvent{SemanticKey: "document_created:" + doc.ID, UserID: actorID, WorkspaceID: doc.WorkspaceID, Name: "document_created", Source: "api", OccurredAt: doc.CreatedAt, Attributes: map[string]any{"entity_id": doc.ID, "space_id": doc.SpaceID, "module": "docs"}})
	}
	if e.content != nil && s.contentSvc != nil {
		s.contentSvc.afterContentSave(ctx, e.content.DocumentID, actorID, e.previousText, e.content)
	}
	if e.version != nil && s.versionSvc != nil {
		s.versionSvc.publishVersionEvent(ctx, "created", e.version, actorID)
	}
}
