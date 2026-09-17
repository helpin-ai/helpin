package repository

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"gorm.io/gorm"
)

// DocsAssetReferenceRepository performs targeted existence checks for imported
// docs assets without hydrating surviving document rows.
type DocsAssetReferenceRepository struct {
	db *gorm.DB
}

func NewDocsAssetReferenceRepository(db *gorm.DB) *DocsAssetReferenceRepository {
	return &DocsAssetReferenceRepository{db: db}
}

func (r *DocsAssetReferenceRepository) HasSurvivingReference(ctx context.Context, workspaceID, deletedDocumentID, assetKey string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("docs asset reference repository is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	deletedDocumentID = strings.TrimSpace(deletedDocumentID)
	assetKey = strings.TrimSpace(assetKey)
	if workspaceID == "" || assetKey == "" {
		return false, nil
	}

	checks := []assetReferenceCheck{
		{
			name: "docs_contents.content",
			sql: fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM docs_contents dc
				JOIN docs_documents dd ON dd.id = dc.document_id
				WHERE dd.workspace_id = ? AND dd.deleted_at IS NULL AND dc.document_id <> ? AND (%s)
				LIMIT 1
			)`, r.likeAnySQL("dc", "content")),
		},
		{
			name: "docs_contents.import_source_html",
			sql: fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM docs_contents dc
				JOIN docs_documents dd ON dd.id = dc.document_id
				WHERE dd.workspace_id = ? AND dd.deleted_at IS NULL AND dc.document_id <> ? AND (%s)
				LIMIT 1
			)`, r.likeAnySQL("dc", "import_source_html")),
		},
		{
			name: "docs_blocks.content",
			sql: fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM docs_blocks db
				JOIN docs_documents dd ON dd.id = db.document_id
				WHERE db.workspace_id = ? AND dd.workspace_id = ? AND dd.deleted_at IS NULL AND db.document_id <> ? AND db.deleted_at IS NULL AND (%s)
				LIMIT 1
			)`, r.likeAnySQL("db", "content")),
			workspaceArgCount: 2,
		},
		{
			name: "docs_versions.content",
			sql: fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM docs_versions dv
				JOIN docs_documents dd ON dd.id = dv.document_id
				WHERE dd.workspace_id = ? AND dd.deleted_at IS NULL AND dv.document_id <> ? AND (%s)
				LIMIT 1
			)`, r.likeAnySQL("dv", "content")),
		},
		{
			name: "docs_helpcenter_article_publications.content",
			sql: fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM docs_helpcenter_article_publications hp
				WHERE hp.workspace_id = ? AND hp.document_id <> ? AND (%s)
				LIMIT 1
			)`, r.likeAnySQL("hp", "content")),
		},
		{
			name: "docs_helpcenter_article_translations.content",
			sql: fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM docs_helpcenter_article_translations ht
				WHERE ht.workspace_id = ? AND ht.document_id <> ? AND (%s)
				LIMIT 1
			)`, r.likeAnySQL("ht", "content")),
		},
	}

	patterns := likePatternsForAssetKey(assetKey)
	for _, check := range checks {
		found, err := r.exists(ctx, check, workspaceID, deletedDocumentID, patterns)
		if err != nil {
			return false, fmt.Errorf("check docs asset reference in %s: %w", check.name, err)
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

type assetReferenceCheck struct {
	name              string
	sql               string
	workspaceArgCount int
}

func (r *DocsAssetReferenceRepository) exists(ctx context.Context, check assetReferenceCheck, workspaceID, deletedDocumentID string, patterns []string) (bool, error) {
	args := make([]any, 0, 2+len(patterns))
	workspaceArgCount := check.workspaceArgCount
	if workspaceArgCount == 0 {
		workspaceArgCount = 1
	}
	for i := 0; i < workspaceArgCount; i++ {
		args = append(args, workspaceID)
	}
	args = append(args, deletedDocumentID)
	for _, pattern := range patterns {
		args = append(args, pattern)
	}
	var exists bool
	if err := r.db.WithContext(ctx).Raw(check.sql, args...).Row().Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *DocsAssetReferenceRepository) likeAnySQL(alias, column string) string {
	expr := fmt.Sprintf("%s.%s", alias, column)
	if r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "postgres" {
		expr = expr + "::text"
	} else {
		expr = "CAST(" + expr + " AS TEXT)"
	}
	parts := make([]string, 0, len(assetReferencePatternKinds()))
	for range assetReferencePatternKinds() {
		parts = append(parts, expr+` LIKE ? ESCAPE '\'`)
	}
	return strings.Join(parts, " OR ")
}

func likePatternsForAssetKey(assetKey string) []string {
	variants := make([]string, 0, 5)
	for _, variant := range []string{
		url.QueryEscape(assetKey),
		assetKey,
		strings.ReplaceAll(assetKey, "/", `\/`),
		pathEscapePreservingSlashes(assetKey),
		strings.ReplaceAll(pathEscapePreservingSlashes(assetKey), "/", `\/`),
	} {
		if variant = strings.TrimSpace(variant); variant != "" {
			variants = append(variants, "%"+escapeAssetReferenceLike(variant)+"%")
		}
	}
	for len(variants) < len(assetReferencePatternKinds()) {
		variants = append(variants, variants[0])
	}
	return variants[:len(assetReferencePatternKinds())]
}

func assetReferencePatternKinds() [5]struct{} {
	return [5]struct{}{}
}

func pathEscapePreservingSlashes(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func escapeAssetReferenceLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
