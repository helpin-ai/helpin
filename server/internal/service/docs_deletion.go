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
)

type docsAssetStore interface {
	PublicURL(key string) string
	DeleteObject(ctx context.Context, key string) error
}

// DocsDocumentDeletionDependencies wires the repositories needed for permanent
// document deletion. Repositories are optional so tests and narrow service
// constructors can opt in incrementally; missing dependencies simply skip that
// part of the graph.
type DocsDocumentDeletionDependencies struct {
	ContentRepo     *repository.DocsContentRepository
	VersionRepo     *repository.DocsVersionRepository
	LinkRepo        *repository.DocsLinkRepository
	ChunkRepo       *repository.DocsChunkRepository
	HelpcenterRepo  *repository.DocsHelpcenterRepository
	PublicationRepo *repository.DocsHelpcenterPublicationRepository
	TranslationRepo *repository.DocsHelpcenterTranslationRepository
	AssetStore      docsAssetStore
}

var docsAssetTokenRE = regexp.MustCompile(`[^\s"'<>]+`)

func (s *DocsDocumentService) deleteDocumentPermanently(ctx context.Context, doc *model.DocsDocument) error {
	documentIDs := []string{doc.ID}

	deleteCandidates, err := s.collectAssetKeysForDeletedDocuments(ctx, doc.WorkspaceID, documentIDs)
	if err != nil {
		return err
	}
	survivorRefs, err := s.collectAssetKeysForSurvivingDocuments(ctx, doc.WorkspaceID, documentIDs)
	if err != nil {
		return err
	}
	keysToDelete := unreferencedAssetKeys(deleteCandidates, survivorRefs)

	if err := s.deleteDocumentRows(ctx, documentIDs); err != nil {
		return err
	}
	if err := s.docRepo.HardDelete(ctx, doc.ID); err != nil {
		return err
	}

	s.deleteAssetKeysBestEffort(ctx, doc.WorkspaceID, keysToDelete)
	return nil
}

func (s *DocsDocumentService) deleteDocumentRows(ctx context.Context, documentIDs []string) error {
	deps := s.deletionDeps
	if deps.PublicationRepo != nil {
		if err := deps.PublicationRepo.DeleteArticlePublicationsByDocumentIDs(ctx, documentIDs); err != nil {
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
		for _, documentID := range documentIDs {
			if err := deps.ChunkRepo.DeleteByDocumentID(ctx, documentID); err != nil {
				return fmt.Errorf("delete docs chunks by document: %w", err)
			}
		}
	}
	return nil
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

func (s *DocsDocumentService) collectAssetKeysForSurvivingDocuments(ctx context.Context, workspaceID string, excludeDocumentIDs []string) (map[string]struct{}, error) {
	deps := s.deletionDeps
	keys := map[string]struct{}{}
	if deps.AssetStore == nil {
		return keys, nil
	}

	if deps.ContentRepo != nil {
		contents, err := deps.ContentRepo.ListByWorkspaceExcludingDocuments(ctx, workspaceID, excludeDocumentIDs)
		if err != nil {
			return nil, err
		}
		addDocsContentAssetRefs(keys, workspaceID, deps.AssetStore, contents)
	}
	if deps.VersionRepo != nil {
		versions, err := deps.VersionRepo.ListByWorkspaceExcludingDocuments(ctx, workspaceID, excludeDocumentIDs)
		if err != nil {
			return nil, err
		}
		addDocsVersionAssetRefs(keys, workspaceID, deps.AssetStore, versions)
	}
	if deps.PublicationRepo != nil {
		publications, err := deps.PublicationRepo.ListArticlePublicationsByWorkspaceExcludingDocuments(ctx, workspaceID, excludeDocumentIDs)
		if err != nil {
			return nil, err
		}
		addArticlePublicationAssetRefs(keys, workspaceID, deps.AssetStore, publications)
	}
	if deps.TranslationRepo != nil {
		translations, err := deps.TranslationRepo.ListArticleTranslationsByWorkspaceExcludingDocuments(ctx, workspaceID, excludeDocumentIDs)
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

func unreferencedAssetKeys(candidates, survivors map[string]struct{}) []string {
	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		if _, stillReferenced := survivors[key]; !stillReferenced {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (s *DocsDocumentService) deleteAssetKeysBestEffort(ctx context.Context, workspaceID string, keys []string) {
	if s.deletionDeps.AssetStore == nil || len(keys) == 0 {
		return
	}
	for _, key := range keys {
		if err := s.deletionDeps.AssetStore.DeleteObject(ctx, key); err != nil {
			slog.WarnContext(ctx, "failed to delete docs asset", "workspace_id", workspaceID, "asset_key", key, "error", err)
		}
	}
}
