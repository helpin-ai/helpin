package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsHelpcenterLinkTarget is one live public publication of a document that
// an article links to.
type DocsHelpcenterLinkTarget struct {
	DocumentID    string `gorm:"column:document_id"`
	Locale        string `gorm:"column:locale"`
	Slug          string `gorm:"column:slug"`
	PublicID      string `gorm:"column:public_id"`
	DefaultLocale string `gorm:"column:default_locale"`
}

// ListLiveArticleLinkTargets returns the live public publications of the
// given documents in one query. Only documents in workspaceID that are
// published, not deleted and live in the Help Center are returned, with at
// most one row for the workspace default locale and one for locale (when that
// translation is published). Callers pick the locale row when present and the
// default-locale row otherwise.
func (r *DocsHelpcenterRepository) ListLiveArticleLinkTargets(ctx context.Context, workspaceID, locale string, documentIDs []string) ([]DocsHelpcenterLinkTarget, error) {
	if strings.TrimSpace(workspaceID) == "" || len(documentIDs) == 0 {
		return nil, nil
	}
	var rows []DocsHelpcenterLinkTarget
	if err := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_publications p").
		Select("p.document_id, p.locale, p.slug, ha.public_id, cfg.default_locale").
		Joins("JOIN docs_documents d ON d.id = p.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = p.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = d.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_translations hat ON hat.document_id = p.document_id AND hat.locale = p.locale").
		Where(`
			d.workspace_id = ?
			AND p.document_id IN ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND ha.public_id IS NOT NULL AND ha.public_id <> ''
			AND (
				p.locale = cfg.default_locale
				OR (p.locale = ? AND hat.status = ? AND hat.published_at IS NOT NULL)
			)
		`, workspaceID, documentIDs, model.DocStatusPublished, strings.TrimSpace(strings.ToLower(locale)), model.DocsHelpcenterTranslationStatusPublished).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list live helpcenter link targets: %w", err)
	}
	return rows, nil
}
