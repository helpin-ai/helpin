package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsHelpcenterSearchRepository manages public help-center search index rows.
type DocsHelpcenterSearchRepository struct {
	db *gorm.DB
}

func NewDocsHelpcenterSearchRepository(db *gorm.DB) *DocsHelpcenterSearchRepository {
	return &DocsHelpcenterSearchRepository{db: db}
}

func (r *DocsHelpcenterSearchRepository) ReplaceArticleEntries(ctx context.Context, documentID, locale string, entries []model.DocsHelpcenterSearchEntry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ? AND locale = ?", documentID, locale).Delete(&model.DocsHelpcenterSearchEntry{}).Error; err != nil {
			return fmt.Errorf("delete helpcenter search entries: %w", err)
		}
		for i := range entries {
			entry := entries[i]
			if entry.ID == "" {
				entry.ID = uuid.NewString()
			}
			if entry.DocumentID == "" {
				entry.DocumentID = documentID
			}
			if entry.Locale == "" {
				entry.Locale = locale
			}
			if entry.SearchConfig == "" {
				entry.SearchConfig = "simple"
			}
			if tx.Dialector.Name() == "postgres" {
				if err := tx.Exec(`
					INSERT INTO docs_helpcenter_search_entries
						(id, workspace_id, document_id, locale, entry_key, entry_type, content, section_title, anchor, position, rank_weight, search_config, search_vector, created_at, updated_at)
					VALUES
						(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, to_tsvector(?::regconfig, COALESCE(?, '')), now(), now())
				`,
					entry.ID,
					entry.WorkspaceID,
					entry.DocumentID,
					entry.Locale,
					entry.EntryKey,
					entry.EntryType,
					entry.Content,
					entry.SectionTitle,
					entry.Anchor,
					entry.Position,
					entry.RankWeight,
					entry.SearchConfig,
					entry.SearchConfig,
					entry.Content,
				).Error; err != nil {
					return fmt.Errorf("insert helpcenter search entry: %w", err)
				}
				continue
			}
			if err := tx.Create(&entry).Error; err != nil {
				return fmt.Errorf("insert helpcenter search entry: %w", err)
			}
		}
		return nil
	})
}

func (r *DocsHelpcenterSearchRepository) DeleteArticleEntriesByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsHelpcenterSearchEntry{}).Error; err != nil {
		return fmt.Errorf("delete helpcenter search entries by document ids: %w", err)
	}
	return nil
}
