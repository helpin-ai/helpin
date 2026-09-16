package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type docsAssetStore interface {
	PublicURL(key string) string
	DeleteObject(ctx context.Context, key string) error
}

type DocsAssetCleanupEnqueuer interface {
	EnqueueDocsAssetCleanup(ctx context.Context, workspaceID, deletedDocumentID string, candidateAssetKeys []string) error
}

// DocsDocumentDeletionDependencies wires the repositories needed for permanent
// document deletion. Repositories are optional so tests and narrow service
// constructors can opt in incrementally; missing dependencies simply skip that
// part of the graph.
type DocsDocumentDeletionDependencies struct {
	ContentRepo     *repository.DocsContentRepository
	BlockRepo       *repository.DocsBlockRepository
	VersionRepo     *repository.DocsVersionRepository
	LinkRepo        *repository.DocsLinkRepository
	ChunkRepo       *repository.DocsChunkRepository
	HelpcenterRepo  *repository.DocsHelpcenterRepository
	PublicationRepo *repository.DocsHelpcenterPublicationRepository
	SearchRepo      *repository.DocsHelpcenterSearchRepository
	TranslationRepo *repository.DocsHelpcenterTranslationRepository
	AssetStore      docsAssetStore
	CleanupEnqueuer DocsAssetCleanupEnqueuer
}

var docsAssetTokenRE = regexp.MustCompile(`[^\s"'<>]+`)

func (s *DocsDocumentService) deleteDocumentPermanently(ctx context.Context, doc *model.DocsDocument) ([]string, error) {
	// Delegate to the batch path so there's a single synchronous deletion code path.
	return s.deleteDocumentsPermanentlySync(ctx, doc.WorkspaceID, []string{doc.ID})
}

func (s *DocsDocumentService) deleteDocumentRows(ctx context.Context, documentIDs []string) error {
	return deleteDocumentRowsWithDeps(ctx, s.deletionDeps, documentIDs)
}

func deleteDocumentRowsWithDeps(ctx context.Context, deps DocsDocumentDeletionDependencies, documentIDs []string) error {
	if deps.PublicationRepo != nil {
		if err := deps.PublicationRepo.DeleteArticlePublicationsByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.SearchRepo != nil {
		if err := deps.SearchRepo.DeleteArticleEntriesByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.TranslationRepo != nil {
		if err := deps.TranslationRepo.DeleteArticleTranslationsByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.HelpcenterRepo != nil {
		if err := deps.HelpcenterRepo.DeleteArticlesByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.ContentRepo != nil {
		if err := deps.ContentRepo.DeleteByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.BlockRepo != nil {
		if err := deps.BlockRepo.DeleteByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.VersionRepo != nil {
		if err := deps.VersionRepo.DeleteByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.LinkRepo != nil {
		if err := deps.LinkRepo.DeleteByDocumentIDs(ctx, documentIDs); err != nil {
			return err
		}
	}
	if deps.ChunkRepo != nil {
		if err := deps.ChunkRepo.DeleteByDocumentIDs(ctx, documentIDs); err != nil {
			return fmt.Errorf("delete docs chunks by documents: %w", err)
		}
	}
	return nil
}

// DeleteDocumentsPermanently hard-deletes a batch of documents in one pass.
// It collects candidate asset keys only from documents being deleted, executes
// bulk row deletes via deleteDocumentRows, hard-deletes all docs at once, then
// enqueues eventual asset cleanup. The HTTP path must stay bounded and must not
// hydrate surviving workspace rows.
//
// All docs in the batch must belong to the same workspace.
func (s *DocsDocumentService) DeleteDocumentsPermanently(ctx context.Context, workspaceID string, documentIDs []string) error {
	candidateKeys, err := s.deleteDocumentsPermanentlySync(ctx, workspaceID, documentIDs)
	if err != nil {
		return err
	}
	s.enqueueAssetCleanupBestEffort(ctx, workspaceID, cleanupDeletedDocumentID(documentIDs), candidateKeys)
	return nil
}

func (s *DocsDocumentService) deleteDocumentsPermanentlySync(ctx context.Context, workspaceID string, documentIDs []string) ([]string, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}

	deleteCandidates, err := s.collectAssetKeysForDeletedDocuments(ctx, workspaceID, documentIDs)
	if err != nil {
		return nil, err
	}
	candidateKeys := sortedAssetKeys(deleteCandidates)

	if err := s.docRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteDocumentRowsWithDeps(ctx, docsDocumentDeletionDepsWithDB(s.deletionDeps, tx, s.useSortKey), documentIDs); err != nil {
			return err
		}
		return repository.NewDocsDocumentRepository(tx, s.useSortKey).HardDeleteByIDs(ctx, documentIDs)
	}); err != nil {
		return nil, err
	}

	return candidateKeys, nil
}

func docsDocumentDeletionDepsWithDB(deps DocsDocumentDeletionDependencies, db *gorm.DB, useSortKey bool) DocsDocumentDeletionDependencies {
	txDeps := deps
	if deps.ContentRepo != nil {
		txDeps.ContentRepo = repository.NewDocsContentRepository(db)
	}
	if deps.BlockRepo != nil {
		txDeps.BlockRepo = repository.NewDocsBlockRepository(db)
	}
	if deps.VersionRepo != nil {
		txDeps.VersionRepo = repository.NewDocsVersionRepository(db)
	}
	if deps.LinkRepo != nil {
		txDeps.LinkRepo = repository.NewDocsLinkRepository(db)
	}
	if deps.ChunkRepo != nil {
		txDeps.ChunkRepo = repository.NewDocsChunkRepository(db)
	}
	if deps.HelpcenterRepo != nil {
		txDeps.HelpcenterRepo = repository.NewDocsHelpcenterRepository(db, useSortKey)
	}
	if deps.PublicationRepo != nil {
		txDeps.PublicationRepo = repository.NewDocsHelpcenterPublicationRepository(db)
	}
	if deps.TranslationRepo != nil {
		txDeps.TranslationRepo = repository.NewDocsHelpcenterTranslationRepository(db)
	}
	return txDeps
}

func (s *DocsDocumentService) collectAssetKeysForDeletedDocuments(ctx context.Context, workspaceID string, documentIDs []string) (map[string]struct{}, error) {
	deps := s.deletionDeps
	keys := map[string]struct{}{}
	if deps.AssetStore == nil {
		return keys, nil
	}

	if deps.ContentRepo != nil {
		contents, err := deps.ContentRepo.ListByDocumentIDs(ctx, documentIDs)
		if err != nil {
			return nil, err
		}
		addDocsContentAssetRefs(keys, workspaceID, deps.AssetStore, contents)
	}
	if deps.BlockRepo != nil {
		blocks, err := deps.BlockRepo.ListByDocumentIDs(ctx, documentIDs)
		if err != nil {
			return nil, err
		}
		addDocsBlockAssetRefs(keys, workspaceID, deps.AssetStore, blocks)
	}
	if deps.VersionRepo != nil {
		versions, err := deps.VersionRepo.ListByDocumentIDs(ctx, documentIDs)
		if err != nil {
			return nil, err
		}
		addDocsVersionAssetRefs(keys, workspaceID, deps.AssetStore, versions)
	}
	if deps.PublicationRepo != nil {
		publications, err := deps.PublicationRepo.ListArticlePublicationsByDocumentIDs(ctx, documentIDs)
		if err != nil {
			return nil, err
		}
		addArticlePublicationAssetRefs(keys, workspaceID, deps.AssetStore, publications)
	}
	if deps.TranslationRepo != nil {
		translations, err := deps.TranslationRepo.ListArticleTranslationsByDocumentIDs(ctx, documentIDs)
		if err != nil {
			return nil, err
		}
		addArticleTranslationAssetRefs(keys, workspaceID, deps.AssetStore, translations)
	}
	return keys, nil
}

func addDocsContentAssetRefs(out map[string]struct{}, workspaceID string, store docsAssetStore, contents []model.DocsContent) {
	for _, content := range contents {
		addOwnedDocsAssetRefs(out, workspaceID, store, string(content.Content))
		if content.ImportSourceHTML != nil {
			addOwnedDocsAssetRefs(out, workspaceID, store, *content.ImportSourceHTML)
		}
	}
}

func addDocsVersionAssetRefs(out map[string]struct{}, workspaceID string, store docsAssetStore, versions []model.DocsVersion) {
	for _, version := range versions {
		addOwnedDocsAssetRefs(out, workspaceID, store, string(version.Content))
	}
}

func addDocsBlockAssetRefs(out map[string]struct{}, workspaceID string, store docsAssetStore, blocks []model.DocsBlock) {
	for _, block := range blocks {
		addOwnedDocsAssetRefs(out, workspaceID, store, string(block.Content))
	}
}

func addArticlePublicationAssetRefs(out map[string]struct{}, workspaceID string, store docsAssetStore, publications []model.DocsHelpcenterArticlePublication) {
	for _, publication := range publications {
		addOwnedDocsAssetRefs(out, workspaceID, store, string(publication.Content))
	}
}

func addArticleTranslationAssetRefs(out map[string]struct{}, workspaceID string, store docsAssetStore, translations []model.DocsHelpcenterArticleTranslation) {
	for _, translation := range translations {
		addOwnedDocsAssetRefs(out, workspaceID, store, string(translation.Content))
	}
}

func addOwnedDocsAssetRefs(out map[string]struct{}, workspaceID string, store docsAssetStore, raw string) {
	if raw == "" {
		return
	}
	raw = strings.ReplaceAll(raw, `\/`, `/`)
	raw = strings.ReplaceAll(raw, `\u0026`, `&`)
	ownedPrefix := "docs-import/" + workspaceID + "/"
	publicPrefix := ""
	if store != nil {
		publicPrefix = strings.TrimRight(store.PublicURL(""), "/") + "/"
	}

	for _, token := range docsAssetTokenRE.FindAllString(raw, -1) {
		token = strings.Trim(token, `"'.,;:!?)\]}{[`)
		if token == "" {
			continue
		}

		key := ""
		if locator, ok := store.(interface {
			DocsImageKeyFromURL(string, string) (string, bool)
		}); ok {
			if privateKey, valid := locator.DocsImageKeyFromURL(token, workspaceID); valid {
				out[privateKey] = struct{}{}
				continue
			}
		}
		switch {
		case publicPrefix != "" && strings.HasPrefix(token, publicPrefix):
			key = strings.TrimPrefix(token, publicPrefix)
		case strings.HasPrefix(token, ownedPrefix):
			key = token
		default:
			continue
		}
		if idx := strings.IndexAny(key, "?#"); idx >= 0 {
			key = key[:idx]
		}
		if decoded, err := url.PathUnescape(key); err == nil {
			key = decoded
		}
		key = strings.TrimLeft(key, "/")
		if strings.HasPrefix(key, ownedPrefix) {
			out[key] = struct{}{}
		}
	}
}

func sortedAssetKeys(candidates map[string]struct{}) []string {
	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cleanupDeletedDocumentID(documentIDs []string) string {
	if len(documentIDs) == 0 {
		return ""
	}
	return documentIDs[0]
}

func (s *DocsDocumentService) enqueueAssetCleanupBestEffort(ctx context.Context, workspaceID, deletedDocumentID string, keys []string) {
	if s.deletionDeps.CleanupEnqueuer == nil || len(keys) == 0 {
		return
	}
	if err := s.deletionDeps.CleanupEnqueuer.EnqueueDocsAssetCleanup(ctx, workspaceID, deletedDocumentID, keys); err != nil {
		slog.WarnContext(ctx, "docs_asset_cleanup_enqueue_failed", "workspace_id", workspaceID, "deleted_document_id", deletedDocumentID, "candidate_asset_count", len(keys), "error", err)
	}
}
