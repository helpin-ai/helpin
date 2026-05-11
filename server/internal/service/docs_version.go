package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsVersionService handles business logic for document versioning.
type DocsVersionService struct {
	versionRepo *repository.DocsVersionRepository
	contentRepo *repository.DocsContentRepository
	docRepo     *repository.DocsDocumentRepository
	wsPublisher *websocket.Publisher
}

// NewDocsVersionService creates a new DocsVersionService.
func NewDocsVersionService(versionRepo *repository.DocsVersionRepository, contentRepo *repository.DocsContentRepository, docRepo *repository.DocsDocumentRepository, wsPublisher *websocket.Publisher) *DocsVersionService {
	return &DocsVersionService{versionRepo: versionRepo, contentRepo: contentRepo, docRepo: docRepo, wsPublisher: wsPublisher}
}

// List returns all versions for a document.
func (s *DocsVersionService) List(ctx context.Context, documentID string) ([]model.DocsVersion, error) {
	return s.versionRepo.ListByDocument(ctx, documentID)
}

// Get returns a single version by ID (for preview).
func (s *DocsVersionService) Get(ctx context.Context, id string) (*model.DocsVersion, error) {
	return s.versionRepo.GetByID(ctx, id)
}

// CreateSnapshot creates a manual version snapshot from current content.
func (s *DocsVersionService) CreateSnapshot(ctx context.Context, documentID, userID string, label *string) (*model.DocsVersion, error) {
	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("no content to snapshot")
	}
	wc := repository.WordCount(content.ContentText)
	version, err := s.versionRepo.Create(ctx, documentID, userID, content.Content, content.ContentText, label, model.VersionTypeManual, wc)
	s.publishVersionEvent(ctx, "created", version, userID)
	return version, err
}

// SnapshotOnPublish creates a version snapshot labeled "Published" with type=publish.
func (s *DocsVersionService) SnapshotOnPublish(ctx context.Context, documentID, userID string) (*model.DocsVersion, error) {
	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("no content to snapshot")
	}
	label := "Published"
	wc := repository.WordCount(content.ContentText)
	version, err := s.versionRepo.Create(ctx, documentID, userID, content.Content, content.ContentText, &label, model.VersionTypePublish, wc)
	s.publishVersionEvent(ctx, "created", version, userID)
	return version, err
}

// createInternalSnapshot creates a version snapshot with a specific type (used internally for revert flow).
func (s *DocsVersionService) createInternalSnapshot(ctx context.Context, documentID, userID string, label *string, versionType string) (*model.DocsVersion, error) {
	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("no content to snapshot")
	}
	wc := repository.WordCount(content.ContentText)
	version, err := s.versionRepo.Create(ctx, documentID, userID, content.Content, content.ContentText, label, versionType, wc)
	s.publishVersionEvent(ctx, "created", version, userID)
	return version, err
}

// Revert replaces current content with a previous version's content and creates before/after revert snapshots.
func (s *DocsVersionService) Revert(ctx context.Context, documentID, versionID, userID string) (*model.DocsContent, error) {
	version, err := s.versionRepo.GetByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, fmt.Errorf("version not found")
	}
	if version.DocumentID != documentID {
		return nil, fmt.Errorf("version does not belong to this document")
	}

	// Snapshot current state before reverting.
	beforeLabel := "Before revert"
	_, _ = s.createInternalSnapshot(ctx, documentID, userID, &beforeLabel, model.VersionTypeRevert)

	// Overwrite current content with the version's content.
	updated, err := s.contentRepo.UpsertWithActor(ctx, documentID, version.Content, userID)
	if err != nil {
		return nil, err
	}

	// Create a "Reverted" snapshot with the timestamp of the target version.
	revertLabel := fmt.Sprintf("Reverted to version from %s", version.CreatedAt.Format("Jan 2, 2006 15:04"))
	wc := repository.WordCount(version.ContentText)
	revertVersion, _ := s.versionRepo.Create(ctx, documentID, userID, version.Content, version.ContentText, &revertLabel, model.VersionTypeRevert, wc)
	s.publishVersionEvent(ctx, "created", revertVersion, userID)
	if s.docRepo != nil {
		if doc, err := s.docRepo.GetByID(ctx, documentID); err == nil && doc != nil {
			publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, doc.WorkspaceID, userID)
		}
	}

	return updated, nil
}

// UpdateLabel renames the label of a manual version snapshot. System labels cannot be renamed.
func (s *DocsVersionService) UpdateLabel(ctx context.Context, versionID string, label *string) (*model.DocsVersion, error) {
	version, err := s.versionRepo.GetByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, fmt.Errorf("version not found")
	}
	if version.VersionType != model.VersionTypeManual {
		return nil, fmt.Errorf("only manual snapshots can be renamed")
	}
	updated, err := s.versionRepo.UpdateLabel(ctx, versionID, label)
	actorID := ""
	if updated != nil {
		actorID = updated.CreatedBy
	}
	s.publishVersionEvent(ctx, "updated", updated, actorID)
	return updated, err
}

// autoSnapshotInterval is the minimum time between auto snapshots.
const autoSnapshotInterval = 20 * time.Minute

// MaybeAutoSnapshot checks if enough time has passed and content has meaningfully changed,
// then creates an auto snapshot if appropriate. Called during content saves.
func (s *DocsVersionService) MaybeAutoSnapshot(ctx context.Context, documentID, userID string) {
	latest, err := s.versionRepo.GetLatestByDocument(ctx, documentID)
	if err != nil {
		return
	}

	// If there are no versions at all, create the first auto snapshot.
	if latest == nil {
		content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
		if err != nil || content == nil {
			return
		}
		wc := repository.WordCount(content.ContentText)
		if wc < 5 {
			return // too little content
		}
		label := "Auto snapshot"
		version, _ := s.versionRepo.Create(ctx, documentID, userID, content.Content, content.ContentText, &label, model.VersionTypeAuto, wc)
		s.publishVersionEvent(ctx, "created", version, userID)
		return
	}

	// Check if enough time has passed.
	if time.Since(latest.CreatedAt) < autoSnapshotInterval {
		return
	}

	// Check if content has meaningfully changed.
	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil || content == nil {
		return
	}

	currentWC := repository.WordCount(content.ContentText)

	// Meaningful change: word count difference > 10 or content text differs significantly.
	wcDiff := currentWC - latest.WordCount
	if wcDiff < 0 {
		wcDiff = -wcDiff
	}
	if wcDiff < 10 && content.ContentText == latest.ContentText {
		return // no meaningful change
	}

	label := "Auto snapshot"
	version, _ := s.versionRepo.Create(ctx, documentID, userID, content.Content, content.ContentText, &label, model.VersionTypeAuto, currentWC)
	s.publishVersionEvent(ctx, "created", version, userID)
}

func (s *DocsVersionService) publishVersionEvent(ctx context.Context, action string, version *model.DocsVersion, actorID string) {
	if s.docRepo == nil || version == nil {
		return
	}
	doc, err := s.docRepo.GetByID(ctx, version.DocumentID)
	if err != nil || doc == nil {
		return
	}
	publishWorkspaceEventWithParent(s.wsPublisher, action, "docs_version", version.ID, doc.WorkspaceID, actorID, "docs_document", version.DocumentID, nil)
}
