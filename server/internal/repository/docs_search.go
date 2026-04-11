package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsSearchRepository handles full-text search for documents.
type DocsSearchRepository struct {
	db *gorm.DB
}

// NewDocsSearchRepository creates a new DocsSearchRepository.
func NewDocsSearchRepository(db *gorm.DB) *DocsSearchRepository {
	return &DocsSearchRepository{db: db}
}

// DocsSearchResult is a search result with rank score.
type DocsSearchResult struct {
	model.DocsDocument
	Rank float64 `json:"rank"`
}

// Search performs a Postgres full-text search across document titles and content.
func (r *DocsSearchRepository) Search(ctx context.Context, workspaceID, query string, spaceIDs []string, status *string, limit int) ([]DocsSearchResult, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Convert user query to tsquery format.
	tsQuery := toTSQuery(query)

	sql := `
		SELECT d.*, ts_rank(
			setweight(to_tsvector('english', COALESCE(d.title, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE(c.content_text, '')), 'B'),
			to_tsquery('english', ?)
		) AS rank
		FROM docs_documents d
		LEFT JOIN docs_contents c ON c.document_id = d.id
		WHERE d.workspace_id = ?
		  AND d.deleted_at IS NULL
		  AND (
			to_tsvector('english', COALESCE(d.title, '')) ||
			to_tsvector('english', COALESCE(c.content_text, ''))
		  ) @@ to_tsquery('english', ?)
	`
	args := []interface{}{tsQuery, workspaceID, tsQuery}

	if len(spaceIDs) > 0 {
		sql += " AND d.space_id IN (?)"
		args = append(args, spaceIDs)
	}
	if status != nil && *status != "" {
		sql += " AND d.status = ?"
		args = append(args, *status)
	} else {
		// By default, exclude archived documents from search results.
		sql += " AND d.status != 'archived'"
	}

	sql += " ORDER BY rank DESC LIMIT ?"
	args = append(args, limit)

	var results []DocsSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("docs search: %w", err)
	}
	return results, nil
}

// PublicSearch searches published help center article translations for a single locale.
func (r *DocsSearchRepository) PublicSearch(ctx context.Context, workspaceID, locale, query, spaceSlug string, limit int) ([]model.PublicSearchResultResponse, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	needle := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	dbQuery := r.db.WithContext(ctx).
		Table("docs_helpcenter_article_translations hat").
		Select(`
			hat.document_id AS id,
			COALESCE(p.title, hat.title) AS title,
			COALESCE(p.slug, hat.slug) AS slug,
			hat.locale AS locale,
			COALESCE(p.excerpt, hat.excerpt) AS excerpt,
			COALESCE(p.collection_id, hat.collection_id) AS collection_id,
			ct.name AS collection_name,
			ct.slug AS collection_slug,
			st.slug AS space_slug,
			st.name AS space_name
		`).
		Joins("JOIN docs_documents d ON d.id = hat.document_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = hat.document_id").
		Joins("JOIN docs_helpcenter_configs cfg ON cfg.workspace_id = hat.workspace_id").
		Joins("LEFT JOIN docs_helpcenter_article_publications p ON p.document_id = hat.document_id AND p.locale = hat.locale").
		Joins(`
			JOIN docs_helpcenter_space_translations st
				ON st.space_id = hat.space_id
				AND st.locale = hat.locale
				AND st.status = ?
				AND st.published_at IS NOT NULL
		`, model.DocsHelpcenterTranslationStatusPublished).
		Joins(`
			LEFT JOIN docs_helpcenter_collection_translations ct
				ON ct.collection_id = COALESCE(p.collection_id, hat.collection_id)
				AND ct.locale = hat.locale
				AND ct.status = ?
				AND ct.published_at IS NOT NULL
		`, model.DocsHelpcenterTranslationStatusPublished).
		Where(`
			hat.workspace_id = ?
			AND hat.locale = ?
			AND d.deleted_at IS NULL
			AND d.status = ?
			AND ha.public_published_at IS NOT NULL
			AND (
				(
					hat.status = ? AND hat.published_at IS NOT NULL
					AND (p.document_id IS NOT NULL OR hat.locale = cfg.default_locale)
				)
				OR (hat.locale = cfg.default_locale AND p.document_id IS NOT NULL)
			)
			AND (
				LOWER(COALESCE(p.title, hat.title, '')) LIKE ?
				OR LOWER(COALESCE(p.content_text, hat.content_text, '')) LIKE ?
			)
		`, workspaceID, locale, model.DocsHelpcenterTranslationStatusPublished, model.DocStatusPublished, needle, needle)

	if spaceSlug != "" {
		dbQuery = dbQuery.Where("st.slug = ?", spaceSlug)
	}

	dbQuery = dbQuery.
		Order("COALESCE(p.updated_at, hat.updated_at) DESC").
		Limit(limit)

	var results []model.PublicSearchResultResponse
	if err := dbQuery.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("docs public search: %w", err)
	}

	// Build a localized ancestor-path string for every result that has
	// an owning collection. The loop is bounded by (limit × depth ≤ 3)
	// and reuses a per-call cache so sibling results under the same
	// collection don't re-query the ancestor chain.
	if len(results) > 0 {
		pathCache := make(map[string]*string, len(results))
		for i := range results {
			if results[i].CollectionID == nil || *results[i].CollectionID == "" {
				continue
			}
			if cached, ok := pathCache[*results[i].CollectionID]; ok {
				results[i].CollectionAncestorPath = cached
				continue
			}
			path, err := r.buildLocalizedCollectionPath(ctx, *results[i].CollectionID, locale)
			if err != nil {
				return nil, err
			}
			pathCache[*results[i].CollectionID] = path
			results[i].CollectionAncestorPath = path
		}
	}

	return results, nil
}

// buildLocalizedCollectionPath walks a collection's ancestor chain and
// returns a slash-joined breadcrumb string like "Root / Middle / Self"
// using localized names per the requested locale, falling back to the
// source collection name when no translation exists. Returns nil when
// the collection has no ancestors beyond itself (top-level single node).
func (r *DocsSearchRepository) buildLocalizedCollectionPath(ctx context.Context, collectionID, locale string) (*string, error) {
	// Load the target collection + every ancestor in a single query.
	type nameRow struct {
		ID                 string  `gorm:"column:id"`
		ParentCollectionID *string `gorm:"column:parent_collection_id"`
		SourceName         string  `gorm:"column:source_name"`
		LocaleName         *string `gorm:"column:locale_name"`
	}
	const maxDepth = 4 // depth cap 0..2 + safety margin
	chain := make([]nameRow, 0, maxDepth)
	cursor := collectionID
	for i := 0; i < maxDepth; i++ {
		var row nameRow
		q := r.db.WithContext(ctx).
			Table("docs_collections c").
			Select(`
				c.id AS id,
				c.parent_collection_id AS parent_collection_id,
				c.name AS source_name,
				ct.name AS locale_name
			`).
			Joins(`
				LEFT JOIN docs_helpcenter_collection_translations ct
					ON ct.collection_id = c.id
					AND ct.locale = ?
			`, locale).
			Where("c.id = ? AND c.deleted_at IS NULL", cursor)
		if err := q.Scan(&row).Error; err != nil {
			return nil, fmt.Errorf("load collection for search path: %w", err)
		}
		if row.ID == "" {
			break
		}
		chain = append(chain, row)
		if row.ParentCollectionID == nil || *row.ParentCollectionID == "" {
			break
		}
		cursor = *row.ParentCollectionID
	}

	if len(chain) <= 1 {
		// No ancestors -> no breadcrumb context worth rendering.
		return nil, nil
	}

	// chain is walked leaf -> root. Reverse into top-down order.
	parts := make([]string, 0, len(chain))
	for i := len(chain) - 1; i >= 0; i-- {
		row := chain[i]
		name := row.SourceName
		if row.LocaleName != nil && *row.LocaleName != "" {
			name = *row.LocaleName
		}
		parts = append(parts, name)
	}
	joined := strings.Join(parts, " / ")
	return &joined, nil
}

// toTSQuery converts a user search string to a Postgres tsquery with AND semantics.
func toTSQuery(input string) string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}
	escaped := make([]string, len(words))
	for i, w := range words {
		// Strip non-alphanumeric chars for safety.
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, w)
		if clean != "" {
			escaped[i] = clean + ":*"
		}
	}
	// Filter empty entries.
	var nonEmpty []string
	for _, e := range escaped {
		if e != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}
	return strings.Join(nonEmpty, " & ")
}
