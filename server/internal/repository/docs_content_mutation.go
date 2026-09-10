package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// ErrDocsContentConflict means the caller must read a fresh document snapshot.
var ErrDocsContentConflict = errors.New("document changed; reread the affected section and retry with its version")

// MutateWithActor checks a snapshot and transforms its content under the same lock as all saves.
func (r *DocsContentRepository) MutateWithActor(ctx context.Context, documentID, version, actorID string, transform func(json.RawMessage) (json.RawMessage, error)) (*model.DocsContent, *model.DocsContent, error) {
	if version == "" {
		return nil, nil, fmt.Errorf("expected_version is required")
	}
	return r.transformContent(ctx, documentID, version, actorID, transform)
}

func (r *DocsContentRepository) transformContent(ctx context.Context, documentID, version, actorID string, transform func(json.RawMessage) (json.RawMessage, error)) (*model.DocsContent, *model.DocsContent, error) {
	var before, saved model.DocsContent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.DocsDocument
		// Lock the parent: it exists even before the first content row is created.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "is_locked", "deleted_at").Where("id = ?", documentID).First(&doc).Error; err != nil {
			return fmt.Errorf("lock document: %w", err)
		}
		if doc.DeletedAt != nil {
			return fmt.Errorf("document not found")
		}
		if version != "" && doc.IsLocked {
			return fmt.Errorf("document is locked")
		}
		err := tx.Where("document_id = ?", documentID).First(&before).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if version != "" && tiptap.DocumentVersion(before.Content) != version {
			return ErrDocsContentConflict
		}
		content, err := transform(before.Content)
		if err != nil {
			return err
		}
		if r.blockRepo != nil {
			normalized, blocks, err := r.blockRepo.NormalizeDocumentContentTx(ctx, tx, documentID, content, actorID)
			if err != nil {
				return err
			}
			if len(normalized) > 0 {
				content = normalized
			}
			if err := r.upsertTx(ctx, tx, documentID, content); err != nil {
				return err
			}
			if err := r.blockRepo.SyncDocumentBlocksTx(ctx, tx, documentID, blocks, actorID); err != nil {
				return err
			}
		} else if err := r.upsertTx(ctx, tx, documentID, content); err != nil {
			return err
		}
		if err := tx.Where("document_id = ?", documentID).First(&saved).Error; err != nil {
			return err
		}
		if version != "" && r.blockRepo != nil {
			var rows []model.DocsBlock
			if err := tx.Select("id", "revision").Where("document_id = ? AND deleted_at IS NULL", documentID).Find(&rows).Error; err != nil {
				return err
			}
			saved.BlockRevisions = make(map[string]int, len(rows))
			for _, row := range rows {
				saved.BlockRevisions[row.ID] = row.Revision
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &before, &saved, nil
}

// ReadSnapshot reads content and addressable revisions consistently with document saves.
func (r *DocsContentRepository) ReadSnapshot(ctx context.Context, documentID string) (*model.DocsContent, []model.DocsBlock, error) {
	var content model.DocsContent
	var blocks []model.DocsBlock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.DocsDocument
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Select("id").Where("id = ?", documentID).First(&doc).Error; err != nil {
			return err
		}
		if err := tx.Where("document_id = ?", documentID).First(&content).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if r.blockRepo == nil {
			return nil
		}
		return tx.Where("document_id = ? AND deleted_at IS NULL", documentID).Order("sort_key ASC, id ASC").Find(&blocks).Error
	})
	return &content, blocks, err
}
