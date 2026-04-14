package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsHelpcenterPublicationRepository stores and reads the latest live article snapshots.
type DocsHelpcenterPublicationRepository struct {
	db *gorm.DB
}

func NewDocsHelpcenterPublicationRepository(db *gorm.DB) *DocsHelpcenterPublicationRepository {
	return &DocsHelpcenterPublicationRepository{db: db}
}

func (r *DocsHelpcenterPublicationRepository) GetArticlePublication(ctx context.Context, documentID, locale string) (*model.DocsHelpcenterArticlePublication, error) {
	var publication model.DocsHelpcenterArticlePublication
	if err := r.db.WithContext(ctx).
		Where("document_id = ? AND locale = ?", documentID, locale).
		First(&publication).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get article publication: %w", err)
	}
	return &publication, nil
}

func (r *DocsHelpcenterPublicationRepository) UpsertArticlePublication(ctx context.Context, publication *model.DocsHelpcenterArticlePublication) (*model.DocsHelpcenterArticlePublication, error) {
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
				"published_at",
				"updated_at",
			}),
		}).
		Create(publication).Error; err != nil {
		return nil, fmt.Errorf("upsert article publication: %w", err)
	}

	return r.GetArticlePublication(ctx, publication.DocumentID, publication.Locale)
}

func (r *DocsHelpcenterPublicationRepository) ListArticlePublicationsByCollection(ctx context.Context, collectionID, locale string) ([]model.DocsHelpcenterArticlePublication, error) {
	var publications []model.DocsHelpcenterArticlePublication
	if err := r.db.WithContext(ctx).
		Where("collection_id = ? AND locale = ?", collectionID, locale).
		Order("published_at DESC").
		Find(&publications).Error; err != nil {
		return nil, fmt.Errorf("list article publications by collection: %w", err)
	}
	return publications, nil
}

func (r *DocsHelpcenterPublicationRepository) ListArticlePublicationsBySpace(ctx context.Context, spaceID, locale string) ([]model.DocsHelpcenterArticlePublication, error) {
	var publications []model.DocsHelpcenterArticlePublication
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND locale = ?", spaceID, locale).
		Order("published_at DESC").
		Find(&publications).Error; err != nil {
		return nil, fmt.Errorf("list article publications by space: %w", err)
	}
	return publications, nil
}

func (r *DocsHelpcenterPublicationRepository) GetArticlePublicationByCollectionSlug(ctx context.Context, collectionID, locale, slug string) (*model.DocsHelpcenterArticlePublication, error) {
	var publications []model.DocsHelpcenterArticlePublication
	if err := r.db.WithContext(ctx).
		Where("collection_id = ? AND locale = ? AND slug = ?", collectionID, locale, slug).
		Limit(2).Find(&publications).Error; err != nil {
		return nil, fmt.Errorf("get article publication by collection slug: %w", err)
	}
	if len(publications) != 1 {
		return nil, nil
	}
	return &publications[0], nil
}

func (r *DocsHelpcenterPublicationRepository) GetArticlePublicationBySpaceSlug(ctx context.Context, spaceID, locale, slug string) (*model.DocsHelpcenterArticlePublication, error) {
	var publication model.DocsHelpcenterArticlePublication
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND locale = ? AND slug = ?", spaceID, locale, slug).
		First(&publication).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get article publication by space slug: %w", err)
	}
	return &publication, nil
}

func (r *DocsHelpcenterPublicationRepository) ArticlePublicationSlugExists(ctx context.Context, spaceID, locale, slug, excludeDocumentID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.DocsHelpcenterArticlePublication{}).
		Where("space_id = ? AND locale = ? AND slug = ?", spaceID, locale, slug).
		Where("document_id <> ?", excludeDocumentID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check article publication slug exists: %w", err)
	}
	return count > 0, nil
}
