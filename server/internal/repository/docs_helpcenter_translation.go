package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsHelpcenterTranslationRepository handles multilingual public help-center records.
type DocsHelpcenterTranslationRepository struct {
	db *gorm.DB
}

// NewDocsHelpcenterTranslationRepository creates a new multilingual help-center repository.
func NewDocsHelpcenterTranslationRepository(db *gorm.DB) *DocsHelpcenterTranslationRepository {
	return &DocsHelpcenterTranslationRepository{db: db}
}

func (r *DocsHelpcenterTranslationRepository) UpsertSpaceTranslation(ctx context.Context, translation *model.DocsHelpcenterSpaceTranslation) (*model.DocsHelpcenterSpaceTranslation, error) {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "space_id"}, {Name: "locale"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"workspace_id",
				"name",
				"slug",
				"description",
				"status",
				"source_updated_at",
				"source_synced",
				"published_at",
				"updated_at",
			}),
		}).
		Create(translation).Error; err != nil {
		return nil, fmt.Errorf("upsert helpcenter space translation: %w", err)
	}

	var stored model.DocsHelpcenterSpaceTranslation
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND locale = ?", translation.SpaceID, translation.Locale).
		First(&stored).Error; err != nil {
		return nil, fmt.Errorf("load helpcenter space translation: %w", err)
	}

	return &stored, nil
}

func (r *DocsHelpcenterTranslationRepository) UpsertCollectionTranslation(ctx context.Context, translation *model.DocsHelpcenterCollectionTranslation) (*model.DocsHelpcenterCollectionTranslation, error) {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "collection_id"}, {Name: "locale"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"workspace_id",
				"space_id",
				"name",
				"description",
				"slug",
				"status",
				"source_updated_at",
				"source_synced",
				"published_at",
				"updated_at",
			}),
		}).
		Create(translation).Error; err != nil {
		return nil, fmt.Errorf("upsert helpcenter collection translation: %w", err)
	}

	var stored model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Where("collection_id = ? AND locale = ?", translation.CollectionID, translation.Locale).
		First(&stored).Error; err != nil {
		return nil, fmt.Errorf("load helpcenter collection translation: %w", err)
	}

	return &stored, nil
}

func (r *DocsHelpcenterTranslationRepository) UpsertArticleTranslation(ctx context.Context, translation *model.DocsHelpcenterArticleTranslation) (*model.DocsHelpcenterArticleTranslation, error) {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "document_id"}, {Name: "locale"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"workspace_id",
				"space_id",
				"collection_id",
				"title",
				"slug",
				"excerpt",
				"content",
				"content_text",
				"seo_title",
				"seo_description",
				"status",
				"source_updated_at",
				"source_synced",
				"published_at",
				"view_count",
				"helpful_count",
				"not_helpful_count",
				"updated_at",
			}),
		}).
		Create(translation).Error; err != nil {
		return nil, fmt.Errorf("upsert helpcenter article translation: %w", err)
	}

	var stored model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Where("document_id = ? AND locale = ?", translation.DocumentID, translation.Locale).
		First(&stored).Error; err != nil {
		return nil, fmt.Errorf("load helpcenter article translation: %w", err)
	}

	return &stored, nil
}

func (r *DocsHelpcenterTranslationRepository) GetSpaceTranslation(ctx context.Context, spaceID, locale string) (*model.DocsHelpcenterSpaceTranslation, error) {
	var translation model.DocsHelpcenterSpaceTranslation
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND locale = ?", spaceID, locale).
		First(&translation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter space translation: %w", err)
	}
	return &translation, nil
}

func (r *DocsHelpcenterTranslationRepository) ListSpaceTranslations(ctx context.Context, spaceID string) ([]model.DocsHelpcenterSpaceTranslation, error) {
	var translations []model.DocsHelpcenterSpaceTranslation
	if err := r.db.WithContext(ctx).
		Where("space_id = ?", spaceID).
		Order("locale ASC").
		Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("list helpcenter space translations: %w", err)
	}
	return translations, nil
}

func (r *DocsHelpcenterTranslationRepository) GetCollectionTranslation(ctx context.Context, collectionID, locale string) (*model.DocsHelpcenterCollectionTranslation, error) {
	var translation model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Where("collection_id = ? AND locale = ?", collectionID, locale).
		First(&translation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter collection translation: %w", err)
	}
	return &translation, nil
}

func (r *DocsHelpcenterTranslationRepository) ListCollectionTranslations(ctx context.Context, collectionID string) ([]model.DocsHelpcenterCollectionTranslation, error) {
	var translations []model.DocsHelpcenterCollectionTranslation
	if err := r.db.WithContext(ctx).
		Where("collection_id = ?", collectionID).
		Order("locale ASC").
		Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("list helpcenter collection translations: %w", err)
	}
	return translations, nil
}

func (r *DocsHelpcenterTranslationRepository) GetArticleTranslation(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticleTranslation, error) {
	var translation model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Where("document_id = ? AND locale = ?", documentID, locale).
		First(&translation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get helpcenter article translation: %w", err)
	}
	return &translation, nil
}

func (r *DocsHelpcenterTranslationRepository) ListArticleTranslations(ctx context.Context, documentID string) ([]model.DocsHelpcenterArticleTranslation, error) {
	var translations []model.DocsHelpcenterArticleTranslation
	if err := r.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("locale ASC").
		Find(&translations).Error; err != nil {
		return nil, fmt.Errorf("list helpcenter article translations: %w", err)
	}
	return translations, nil
}

func (r *DocsHelpcenterTranslationRepository) MarkArticleTranslationsNeedsReview(ctx context.Context, documentID, defaultLocale string, sourceUpdatedAt time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticleTranslation{}).
		Where("document_id = ? AND locale <> ?", documentID, defaultLocale).
		Updates(map[string]interface{}{
			"status":            model.DocsHelpcenterTranslationStatusNeedsReview,
			"source_updated_at": sourceUpdatedAt,
			"source_synced":     false,
		}).Error; err != nil {
		return fmt.Errorf("mark article translations needs_review: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterTranslationRepository) SetArticleTranslationStatus(ctx context.Context, documentID, locale, status string, publishedAt *time.Time) error {
	updates := map[string]interface{}{
		"status":       status,
		"published_at": publishedAt,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticleTranslation{}).
		Where("document_id = ? AND locale = ?", documentID, locale).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("set article translation status: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterTranslationRepository) SetSpaceTranslationSlug(ctx context.Context, spaceID, locale string, slug *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterSpaceTranslation{}).
		Where("space_id = ? AND locale = ?", spaceID, locale).
		Update("slug", slug).Error; err != nil {
		return fmt.Errorf("set space translation slug: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterTranslationRepository) SetCollectionTranslationSlug(ctx context.Context, collectionID, locale string, slug *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterCollectionTranslation{}).
		Where("collection_id = ? AND locale = ?", collectionID, locale).
		Update("slug", slug).Error; err != nil {
		return fmt.Errorf("set collection translation slug: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterTranslationRepository) SetArticleTranslationSlug(ctx context.Context, documentID, locale string, slug *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticleTranslation{}).
		Where("document_id = ? AND locale = ?", documentID, locale).
		Update("slug", slug).Error; err != nil {
		return fmt.Errorf("set article translation slug: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterTranslationRepository) SpaceTranslationSlugExists(ctx context.Context, workspaceID, locale, slug, excludeSpaceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterSpaceTranslation{}).
		Where("workspace_id = ? AND locale = ? AND slug = ?", workspaceID, locale, slug).
		Where("space_id <> ?", excludeSpaceID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check space translation slug exists: %w", err)
	}
	return count > 0, nil
}

func (r *DocsHelpcenterTranslationRepository) CollectionTranslationSlugExists(ctx context.Context, spaceID, locale, slug, excludeCollectionID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterCollectionTranslation{}).
		Where("space_id = ? AND locale = ? AND slug = ?", spaceID, locale, slug).
		Where("collection_id <> ?", excludeCollectionID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check collection translation slug exists: %w", err)
	}
	return count > 0, nil
}

func (r *DocsHelpcenterTranslationRepository) ArticleTranslationSlugExists(ctx context.Context, spaceID, locale, slug, excludeDocumentID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticleTranslation{}).
		Where("space_id = ? AND locale = ? AND slug = ?", spaceID, locale, slug).
		Where("document_id <> ?", excludeDocumentID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check article translation slug exists: %w", err)
	}
	return count > 0, nil
}

func (r *DocsHelpcenterTranslationRepository) SetSpaceTranslationStatus(ctx context.Context, spaceID, locale, status string, publishedAt *time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterSpaceTranslation{}).
		Where("space_id = ? AND locale = ?", spaceID, locale).
		Updates(map[string]interface{}{
			"status":       status,
			"published_at": publishedAt,
		}).Error; err != nil {
		return fmt.Errorf("set space translation status: %w", err)
	}
	return nil
}

func (r *DocsHelpcenterTranslationRepository) SetCollectionTranslationStatus(ctx context.Context, collectionID, locale, status string, publishedAt *time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterCollectionTranslation{}).
		Where("collection_id = ? AND locale = ?", collectionID, locale).
		Updates(map[string]interface{}{
			"status":       status,
			"published_at": publishedAt,
		}).Error; err != nil {
		return fmt.Errorf("set collection translation status: %w", err)
	}
	return nil
}

// BackfillDefaultLocaleMirrors creates idempotent default-locale public translation rows from source docs.
func (r *DocsHelpcenterTranslationRepository) BackfillDefaultLocaleMirrors(ctx context.Context, workspaceID string) error {
	var cfg model.DocsHelpcenterConfig
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("load helpcenter config: %w", err)
	}

	defaultLocale := cfg.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	var spaces []model.DocsSpace
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND type = ? AND deleted_at IS NULL", workspaceID, model.SpaceTypeExternalCapable).
		Order("position ASC, created_at ASC").
		Find(&spaces).Error; err != nil {
		return fmt.Errorf("list external spaces for helpcenter translation backfill: %w", err)
	}

	for _, space := range spaces {
		if _, err := r.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
			SpaceID:         space.ID,
			WorkspaceID:     space.WorkspaceID,
			Locale:          defaultLocale,
			Name:            space.Name,
			Slug:            stringPointerOrNil(space.Slug),
			Status:          model.DocsHelpcenterTranslationStatusPublished,
			SourceUpdatedAt: &space.UpdatedAt,
			SourceSynced:    true,
			PublishedAt:     &space.UpdatedAt,
		}); err != nil {
			return err
		}
	}

	var collections []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Joins("JOIN docs_spaces ON docs_spaces.id = docs_collections.space_id").
		Where("docs_collections.workspace_id = ? AND docs_spaces.type = ? AND docs_collections.deleted_at IS NULL AND docs_spaces.deleted_at IS NULL", workspaceID, model.SpaceTypeExternalCapable).
		Order("docs_spaces.position ASC, docs_collections.position ASC, docs_collections.created_at ASC").
		Find(&collections).Error; err != nil {
		return fmt.Errorf("list external collections for helpcenter translation backfill: %w", err)
	}

	for _, collection := range collections {
		if _, err := r.UpsertCollectionTranslation(ctx, &model.DocsHelpcenterCollectionTranslation{
			CollectionID:    collection.ID,
			WorkspaceID:     collection.WorkspaceID,
			SpaceID:         collection.SpaceID,
			Locale:          defaultLocale,
			Name:            collection.Name,
			Description:     collection.Description,
			Slug:            stringPointerOrNil(collection.Slug),
			Status:          model.DocsHelpcenterTranslationStatusPublished,
			SourceUpdatedAt: &collection.UpdatedAt,
			SourceSynced:    true,
			PublishedAt:     &collection.UpdatedAt,
		}); err != nil {
			return err
		}
	}

	type articleMirrorRow struct {
		DocumentID        string
		WorkspaceID       string
		SpaceID           string
		CollectionID      *string
		Title             string
		Excerpt           *string
		Content           []byte
		ContentText       string
		Slug              string
		SEOTitle          *string
		SEODescription    *string
		HelpfulCount      int
		NotHelpfulCount   int
		ViewCount         int
		PublicPublishedAt *time.Time
		UpdatedAt         time.Time
	}

	var articles []articleMirrorRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT
			d.id AS document_id,
			d.workspace_id,
			d.space_id,
			d.collection_id,
			d.title,
			d.excerpt,
			c.content,
			c.content_text,
			ha.slug,
			ha.seo_title,
			ha.seo_description,
			ha.helpful_count,
			ha.not_helpful_count,
			ha.view_count,
			ha.public_published_at,
			d.updated_at
		FROM docs_documents d
		JOIN docs_spaces s ON s.id = d.space_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		LEFT JOIN docs_contents c ON c.document_id = d.id
		WHERE d.workspace_id = ?
		  AND s.type = ?
		  AND d.deleted_at IS NULL
		  AND s.deleted_at IS NULL
		  AND ha.slug != ''
	`, workspaceID, model.SpaceTypeExternalCapable).Scan(&articles).Error; err != nil {
		return fmt.Errorf("list external articles for helpcenter translation backfill: %w", err)
	}

	for _, article := range articles {
		status := model.DocsHelpcenterTranslationStatusDraft
		if article.PublicPublishedAt != nil {
			status = model.DocsHelpcenterTranslationStatusPublished
		}
		if _, err := r.UpsertArticleTranslation(ctx, &model.DocsHelpcenterArticleTranslation{
			DocumentID:      article.DocumentID,
			WorkspaceID:     article.WorkspaceID,
			SpaceID:         article.SpaceID,
			CollectionID:    article.CollectionID,
			Locale:          defaultLocale,
			Title:           article.Title,
			Slug:            stringPointerOrNil(article.Slug),
			Excerpt:         article.Excerpt,
			Content:         article.Content,
			ContentText:     article.ContentText,
			SEOTitle:        article.SEOTitle,
			SEODescription:  article.SEODescription,
			Status:          status,
			SourceUpdatedAt: &article.UpdatedAt,
			SourceSynced:    true,
			PublishedAt:     article.PublicPublishedAt,
			ViewCount:       article.ViewCount,
			HelpfulCount:    article.HelpfulCount,
			NotHelpfulCount: article.NotHelpfulCount,
		}); err != nil {
			return err
		}
	}

	return nil
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
