package repository

import (
	"context"
	"fmt"
)

// HelpcenterMirrorGap counts default-locale mirror rows a workspace's public
// help center content lacks.
type HelpcenterMirrorGap struct {
	WorkspaceID string
	Spaces      int64
	Collections int64
	Articles    int64
}

// Total returns the number of missing mirror rows.
func (g HelpcenterMirrorGap) Total() int64 { return g.Spaces + g.Collections + g.Articles }

// missingMirrorsSQL follows the eligibility rules of
// DocsHelpcenterTranslationService.EnsureDefaultLocaleMirrorsForSpace.
const missingMirrorsSQL = `
WITH cfg AS (
	SELECT workspace_id, COALESCE(NULLIF(default_locale, ''), 'en') AS locale
	FROM docs_helpcenter_configs
	WHERE (? = '' OR workspace_id::text = ?)
), spaces AS (
	SELECT s.id, s.workspace_id, cfg.locale
	FROM docs_spaces s JOIN cfg ON cfg.workspace_id = s.workspace_id
	WHERE s.type = 'external_capable' AND s.deleted_at IS NULL
)
SELECT cfg.workspace_id,
	(SELECT COUNT(*) FROM spaces sp
	 WHERE sp.workspace_id = cfg.workspace_id AND NOT EXISTS (
		SELECT 1 FROM docs_helpcenter_space_translations t
		WHERE t.space_id = sp.id AND t.locale = sp.locale)) AS spaces,
	(SELECT COUNT(*) FROM docs_collections c JOIN spaces sp ON sp.id = c.space_id
	 WHERE sp.workspace_id = cfg.workspace_id AND c.deleted_at IS NULL AND NOT EXISTS (
		SELECT 1 FROM docs_helpcenter_collection_translations t
		WHERE t.collection_id = c.id AND t.locale = sp.locale)) AS collections,
	(SELECT COUNT(*) FROM docs_documents d
	 JOIN spaces sp ON sp.id = d.space_id
	 JOIN docs_helpcenter_articles a ON a.document_id = d.id
	 WHERE sp.workspace_id = cfg.workspace_id AND d.deleted_at IS NULL
		AND d.status = 'published' AND a.slug <> '' AND a.public_published_at IS NOT NULL
		AND NOT EXISTS (
			SELECT 1 FROM docs_helpcenter_article_translations t
			WHERE t.document_id = d.id AND t.locale = sp.locale)) AS articles
FROM cfg
ORDER BY cfg.workspace_id`

// ListMirrorGaps reports, per help center, how many default-locale mirror rows
// are missing. An empty workspaceID checks every help center. PostgreSQL only.
func (r *DocsHelpcenterTranslationRepository) ListMirrorGaps(ctx context.Context, workspaceID string) ([]HelpcenterMirrorGap, error) {
	var gaps []HelpcenterMirrorGap
	if err := r.db.WithContext(ctx).Raw(missingMirrorsSQL, workspaceID, workspaceID).Scan(&gaps).Error; err != nil {
		return nil, fmt.Errorf("list helpcenter mirror gaps: %w", err)
	}
	return gaps, nil
}
