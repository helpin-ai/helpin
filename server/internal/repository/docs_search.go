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
func (r *DocsSearchRepository) Search(ctx context.Context, workspaceID, query string, spaceIDs []string, docType, status *string, limit int) ([]DocsSearchResult, error) {
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
	if docType != nil && *docType != "" {
		sql += " AND d.doc_type = ?"
		args = append(args, *docType)
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

// PublicSearch searches published help center articles by subdomain.
func (r *DocsSearchRepository) PublicSearch(ctx context.Context, workspaceID, query string, limit int) ([]DocsSearchResult, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	tsQuery := toTSQuery(query)

	sql := `
		SELECT d.*, ts_rank(
			setweight(to_tsvector('english', COALESCE(d.title, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE(c.content_text, '')), 'B'),
			to_tsquery('english', ?)
		) AS rank
		FROM docs_documents d
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		LEFT JOIN docs_contents c ON c.document_id = d.id
		WHERE d.workspace_id = ?
		  AND d.deleted_at IS NULL
		  AND d.status = 'published'
		  AND ha.public_published_at IS NOT NULL
		  AND (
			to_tsvector('english', COALESCE(d.title, '')) ||
			to_tsvector('english', COALESCE(c.content_text, ''))
		  ) @@ to_tsquery('english', ?)
		ORDER BY rank DESC
		LIMIT ?
	`

	var results []DocsSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, tsQuery, workspaceID, tsQuery, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("docs public search: %w", err)
	}
	return results, nil
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
