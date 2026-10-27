package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetCustomerPublishedAt marks a help center article live for signed-in
// portal customers, or takes it down when publishedAt is nil.
func (r *DocsHelpcenterRepository) SetCustomerPublishedAt(ctx context.Context, documentID string, publishedAt *time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.DocsHelpcenterArticle{}).
		Where("document_id = ?", documentID).
		Update("customer_published_at", publishedAt).Error; err != nil {
		return fmt.Errorf("set customer_published_at: %w", err)
	}
	return nil
}

// ListLiveSpaceArticleDocumentIDs returns the documents in a space that are
// live on the help center for either audience.
func (r *DocsHelpcenterRepository) ListLiveSpaceArticleDocumentIDs(ctx context.Context, spaceID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Table("docs_helpcenter_articles AS ha").
		Joins("JOIN docs_documents d ON d.id = ha.document_id").
		Where("d.space_id = ? AND (ha.public_published_at IS NOT NULL OR ha.customer_published_at IS NOT NULL)", spaceID).
		Pluck("ha.document_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list live space articles: %w", err)
	}
	return ids, nil
}

// MoveArticleAudience moves a live article's publish time to the column of
// the given audience, so it is live for exactly one audience.
func (r *DocsHelpcenterRepository) MoveArticleAudience(ctx context.Context, documentID, audience string) error {
	updates := map[string]interface{}{
		"public_published_at":   nil,
		"customer_published_at": gorm.Expr("COALESCE(public_published_at, customer_published_at)"),
	}
	if audience == model.HelpcenterAudiencePublic {
		updates = map[string]interface{}{
			"public_published_at":   gorm.Expr("COALESCE(customer_published_at, public_published_at)"),
			"customer_published_at": nil,
		}
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsHelpcenterArticle{}).
		Where("document_id = ?", documentID).Updates(updates).Error; err != nil {
		return fmt.Errorf("move article audience: %w", err)
	}
	return nil
}

// customerHelpArticleGate selects live articles of help center spaces for
// signed-in portal customers, joined as d (document), s (space),
// ha (help center article) and p (publication).
const customerHelpArticleGate = `d.workspace_id = ?
	AND d.status = 'published' AND d.deleted_at IS NULL
	AND s.type = 'external_capable' AND s.helpcenter_audience = 'customers' AND s.deleted_at IS NULL
	AND ha.customer_published_at IS NOT NULL`

// CustomerHelpArticle is a live article for signed-in portal customers.
type CustomerHelpArticle struct {
	DocumentID     string          `json:"-"`
	PublicID       string          `json:"id"`
	Title          string          `json:"title"`
	Excerpt        *string         `json:"excerpt,omitempty"`
	Locale         string          `json:"locale"`
	SpaceID        string          `json:"-"`
	SpaceName      string          `json:"space_name"`
	CollectionName *string         `json:"collection_name,omitempty"`
	PublishedAt    time.Time       `json:"published_at"`
	TranslatedAt   *time.Time      `json:"-"`
	Content        json.RawMessage `json:"-"`
}

func (r *DocsHelpcenterRepository) customerHelpArticles(ctx context.Context, workspaceID, defaultLocale, locale string) *gorm.DB {
	if locale == "" {
		locale = defaultLocale
	}
	// A published translation in the reader's locale replaces the default
	// publication; untranslated articles keep the default language.
	return r.db.WithContext(ctx).Table("docs_documents AS d").
		Select(customerHelpArticleColumns).
		Joins("JOIN docs_spaces s ON s.id = d.space_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = d.id").
		Joins("JOIN docs_helpcenter_article_publications p ON p.document_id = d.id AND p.locale = ?", defaultLocale).
		Joins("LEFT JOIN docs_helpcenter_article_publications pl ON pl.document_id = d.id AND pl.locale = ? AND pl.locale <> p.locale", locale).
		Joins("LEFT JOIN docs_collections c ON c.id = d.collection_id AND c.deleted_at IS NULL").
		Where(customerHelpArticleGate, workspaceID)
}

const customerHelpArticleColumns = `d.id AS document_id, ha.public_id, COALESCE(pl.title, p.title) AS title,
	COALESCE(pl.excerpt, p.excerpt) AS excerpt, COALESCE(pl.locale, p.locale) AS locale, s.id AS space_id,
	s.name AS space_name, c.name AS collection_name, p.published_at, pl.published_at AS translated_at`

// ListCustomerHelpArticles lists the live articles for signed-in portal
// customers, in locale where translated and otherwise in the default locale,
// ordered by space, collection and position.
func (r *DocsHelpcenterRepository) ListCustomerHelpArticles(ctx context.Context, workspaceID, defaultLocale, locale string) ([]CustomerHelpArticle, error) {
	var articles []CustomerHelpArticle
	if err := r.customerHelpArticles(ctx, workspaceID, defaultLocale, locale).
		Order("s.position ASC, s.created_at ASC, c.position ASC, d.position ASC, title ASC").
		Scan(&articles).Error; err != nil {
		return nil, fmt.Errorf("list customer help articles: %w", err)
	}
	for i := range articles {
		articles[i].useTranslatedDate()
	}
	return articles, nil
}

// GetCustomerHelpArticle returns one live article for signed-in portal
// customers with its published content, in locale where translated, or nil
// when there is none.
func (r *DocsHelpcenterRepository) GetCustomerHelpArticle(ctx context.Context, workspaceID, defaultLocale, locale, publicID string) (*CustomerHelpArticle, error) {
	var articles []CustomerHelpArticle
	if err := r.customerHelpArticles(ctx, workspaceID, defaultLocale, locale).
		Select(customerHelpArticleColumns+", COALESCE(pl.content, p.content) AS content").
		Where("ha.public_id = ?", publicID).Limit(1).
		Scan(&articles).Error; err != nil {
		return nil, fmt.Errorf("get customer help article: %w", err)
	}
	if len(articles) == 0 {
		return nil, nil
	}
	articles[0].useTranslatedDate()
	return &articles[0], nil
}

// ListCustomerHelpSpaceIDs returns the spaces for signed-in portal customers
// that have live articles.
func (r *DocsHelpcenterRepository) ListCustomerHelpSpaceIDs(ctx context.Context, workspaceID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Table("docs_documents AS d").
		Joins("JOIN docs_spaces s ON s.id = d.space_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = d.id").
		Where(customerHelpArticleGate, workspaceID).
		Distinct().Pluck("s.id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list customer help spaces: %w", err)
	}
	return ids, nil
}

// useTranslatedDate reports a translation's publish time when the article is
// read in translation.
func (a *CustomerHelpArticle) useTranslatedDate() {
	if a.TranslatedAt != nil {
		a.PublishedAt = *a.TranslatedAt
	}
}
