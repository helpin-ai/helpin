package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UnpublishSpace takes every live article in a space off the help center, for
// both audiences. It runs when a space stops being external.
func (s *DocsHelpcenterService) UnpublishSpace(ctx context.Context, spaceID string) error {
	ids, err := s.hcRepo.ListLiveSpaceArticleDocumentIDs(ctx, spaceID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.UnpublishExternally(ctx, id); err != nil {
			return fmt.Errorf("unpublish %s: %w", id, err)
		}
	}
	return nil
}

// MoveSpaceAudience moves a space's live articles to a new audience. Moving
// to signed-in customers removes them from public search; moving back to
// the public rebuilds their search entries from the published snapshot.
func (s *DocsHelpcenterService) MoveSpaceAudience(ctx context.Context, spaceID, audience string) error {
	ids, err := s.hcRepo.ListLiveSpaceArticleDocumentIDs(ctx, spaceID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	toPublic := audience == model.HelpcenterAudiencePublic
	if !toPublic && s.searchRepo != nil {
		if err := s.searchRepo.DeleteArticleEntriesByDocumentIDs(ctx, ids); err != nil {
			return err
		}
	}
	var workspaceID string
	for _, id := range ids {
		if err := s.hcRepo.MoveArticleAudience(ctx, id, audience); err != nil {
			return err
		}
		doc, err := s.docRepo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if doc == nil {
			continue
		}
		workspaceID = doc.WorkspaceID
		if toPublic {
			if err := s.rebuildDefaultLocaleSearchEntries(ctx, doc); err != nil {
				return err
			}
		}
		if s.translationSvc != nil {
			if err := s.translationSvc.RefreshArticleSource(ctx, id); err != nil {
				return fmt.Errorf("refresh translation source for %s: %w", id, err)
			}
		}
		publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", id, doc.WorkspaceID, "")
	}
	if workspaceID != "" {
		s.InvalidateHelpcenterCacheForWorkspace(ctx, workspaceID)
	}
	return nil
}

// rebuildDefaultLocaleSearchEntries rebuilds public search entries from an
// article's default-locale publication.
func (s *DocsHelpcenterService) rebuildDefaultLocaleSearchEntries(ctx context.Context, doc *model.DocsDocument) error {
	cfg, err := s.hcRepo.GetConfig(ctx, doc.WorkspaceID)
	if err != nil {
		return err
	}
	publication, err := s.publicationRepo.GetArticlePublication(ctx, doc.ID, defaultHelpcenterLocale(cfg))
	if err != nil {
		return err
	}
	return s.rebuildArticleSearchEntries(ctx, publication)
}

// publishForCustomers makes a published snapshot live for signed-in portal
// customers only.
func (s *DocsHelpcenterService) publishForCustomers(ctx context.Context, doc *model.DocsDocument) error {
	if err := s.hcRepo.SetPublicPublishedAt(ctx, doc.ID, nil); err != nil {
		return err
	}
	if s.searchRepo != nil {
		if err := s.searchRepo.DeleteArticleEntriesByDocumentIDs(ctx, []string{doc.ID}); err != nil {
			return err
		}
	}
	now := time.Now()
	if err := s.hcRepo.SetCustomerPublishedAt(ctx, doc.ID, &now); err != nil {
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", doc.ID, doc.WorkspaceID, "")
	s.InvalidateHelpcenterCacheForWorkspace(ctx, doc.WorkspaceID)
	return nil
}
