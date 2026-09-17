package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsBlockRepository manages addressable document blocks.
type DocsBlockRepository struct {
	db *gorm.DB
}

func NewDocsBlockRepository(db *gorm.DB) *DocsBlockRepository {
	return &DocsBlockRepository{db: db}
}

func (r *DocsBlockRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *DocsBlockRepository) GetByID(ctx context.Context, id string) (*model.DocsBlock, error) {
	var block model.DocsBlock
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&block).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs block: %w", err)
	}
	return &block, nil
}

func (r *DocsBlockRepository) ListByDocument(ctx context.Context, documentID string, includeDeleted bool) ([]model.DocsBlock, error) {
	var blocks []model.DocsBlock
	query := r.db.WithContext(ctx).Where("document_id = ?", documentID)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	if err := query.Order("sort_key ASC, created_at ASC").Find(&blocks).Error; err != nil {
		return nil, fmt.Errorf("list docs blocks: %w", err)
	}
	return blocks, nil
}

func (r *DocsBlockRepository) ListByDocumentIDs(ctx context.Context, documentIDs []string) ([]model.DocsBlock, error) {
	if len(documentIDs) == 0 {
		return []model.DocsBlock{}, nil
	}
	var blocks []model.DocsBlock
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Find(&blocks).Error; err != nil {
		return nil, fmt.Errorf("list docs blocks by documents: %w", err)
	}
	return blocks, nil
}

func (r *DocsBlockRepository) ListByWorkspaceExcludingDocuments(ctx context.Context, workspaceID string, excludeDocumentIDs []string) ([]model.DocsBlock, error) {
	var blocks []model.DocsBlock
	query := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID)
	if len(excludeDocumentIDs) > 0 {
		query = query.Where("document_id NOT IN ?", excludeDocumentIDs)
	}
	if err := query.Find(&blocks).Error; err != nil {
		return nil, fmt.Errorf("list surviving docs blocks: %w", err)
	}
	return blocks, nil
}

func (r *DocsBlockRepository) DeleteByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsBlock{}).Error; err != nil {
		return fmt.Errorf("delete docs blocks by documents: %w", err)
	}
	return nil
}

// NormalizeDocumentContentTx stamps stable block IDs into supported top-level
// nodes and returns the normalized aggregate document plus block rows.
func (r *DocsBlockRepository) NormalizeDocumentContentTx(ctx context.Context, tx *gorm.DB, documentID string, raw json.RawMessage, actorID string) (json.RawMessage, []model.DocsBlock, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return raw, nil, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw, nil, nil
	}
	if docType, _ := doc["type"].(string); docType != "doc" {
		return raw, nil, nil
	}

	children, ok := doc["content"].([]any)
	if !ok {
		return raw, nil, nil
	}

	workspaceID, err := r.workspaceIDForDocumentTx(ctx, tx, documentID)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]string, 0, len(children))
	for _, child := range children {
		if node, ok := child.(map[string]any); ok {
			if id := blockIDFromNode(node); id != "" {
				ids = append(ids, id)
			}
		}
	}
	collisions, err := r.blockIDsOwnedByOtherDocumentsTx(ctx, tx, documentID, ids)
	if err != nil {
		return nil, nil, err
	}

	blocks := make([]model.DocsBlock, 0, len(children))
	seen := map[string]struct{}{}
	for index, child := range children {
		node, ok := child.(map[string]any)
		if !ok {
			continue
		}
		nodeType, _ := node["type"].(string)
		if strings.TrimSpace(nodeType) == "" {
			continue
		}
		blockID := blockIDFromNode(node)
		if !validUUID(blockID) || collisions[blockID] || hasSeen(seen, blockID) {
			blockID = uuid.NewString()
			setBlockIDOnNode(node, blockID)
		}
		seen[blockID] = struct{}{}
		content, err := json.Marshal(node)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal docs block: %w", err)
		}
		actor := ptrIfNonEmpty(actorID)
		blocks = append(blocks, model.DocsBlock{
			ID:           blockID,
			WorkspaceID:  workspaceID,
			DocumentID:   documentID,
			Type:         nodeType,
			Content:      content,
			ContentText:  extractPlainText(content),
			SortKey:      blockSortKey(index),
			AuthoredBy:   actor,
			LastEditedBy: actor,
		})
	}

	normalized, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal normalized docs content: %w", err)
	}
	return normalized, blocks, nil
}

func (r *DocsBlockRepository) SyncDocumentBlocksTx(ctx context.Context, tx *gorm.DB, documentID string, blocks []model.DocsBlock, actorID string) error {
	if len(blocks) == 0 {
		now := time.Now().UTC()
		if err := tx.WithContext(ctx).Model(&model.DocsBlock{}).
			Where("document_id = ? AND deleted_at IS NULL", documentID).
			Update("deleted_at", now).Error; err != nil {
			return fmt.Errorf("soft delete document blocks: %w", err)
		}
		return nil
	}

	var existing []model.DocsBlock
	if err := tx.WithContext(ctx).Where("document_id = ?", documentID).Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing docs blocks: %w", err)
	}
	existingByID := make(map[string]model.DocsBlock, len(existing))
	for _, block := range existing {
		existingByID[block.ID] = block
	}

	keep := make([]string, 0, len(blocks))
	now := time.Now().UTC()
	for _, block := range blocks {
		keep = append(keep, block.ID)
		revision := 1
		changed := true
		if existing, ok := existingByID[block.ID]; ok {
			changed = false
			revision = existing.Revision
			// Only content edits (or restoring a deleted block) bump the
			// revision. Position changes reassign every following block's
			// sort key, so counting them would invalidate pending block
			// proposals on untouched blocks.
			if !docsBlockContentEqual(existing.Content, block.Content) || existing.DeletedAt != nil {
				revision++
				changed = true
			}
			block.AuthoredBy = existing.AuthoredBy
			block.LastEditedBy = existing.LastEditedBy
		}
		block.Revision = revision
		block.DeletedAt = nil
		block.UpdatedAt = now
		if changed && strings.TrimSpace(actorID) != "" {
			block.LastEditedBy = &actorID
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"workspace_id":   block.WorkspaceID,
				"document_id":    block.DocumentID,
				"parent_id":      block.ParentID,
				"type":           block.Type,
				"content":        block.Content,
				"content_text":   block.ContentText,
				"sort_key":       block.SortKey,
				"revision":       block.Revision,
				"authored_by":    block.AuthoredBy,
				"last_edited_by": block.LastEditedBy,
				"updated_at":     now,
				"deleted_at":     nil,
			}),
		}).Create(&block).Error; err != nil {
			return fmt.Errorf("upsert docs block: %w", err)
		}
	}

	query := tx.WithContext(ctx).Model(&model.DocsBlock{}).
		Where("document_id = ? AND deleted_at IS NULL", documentID)
	if len(keep) > 0 {
		query = query.Where("id NOT IN ?", keep)
	}
	if err := query.Update("deleted_at", now).Error; err != nil {
		return fmt.Errorf("soft delete removed docs blocks: %w", err)
	}
	return nil
}

func (r *DocsBlockRepository) workspaceIDForDocumentTx(ctx context.Context, tx *gorm.DB, documentID string) (string, error) {
	var doc model.DocsDocument
	if err := tx.WithContext(ctx).Select("workspace_id").Where("id = ?", documentID).First(&doc).Error; err != nil {
		return "", fmt.Errorf("get document workspace for blocks: %w", err)
	}
	return doc.WorkspaceID, nil
}

func (r *DocsBlockRepository) blockIDsOwnedByOtherDocumentsTx(ctx context.Context, tx *gorm.DB, documentID string, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if validUUID(id) {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return out, nil
	}
	var rows []model.DocsBlock
	if err := tx.WithContext(ctx).Select("id").Where("id IN ? AND document_id <> ?", filtered, documentID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("check docs block id collisions: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = true
	}
	return out, nil
}

func blockIDFromNode(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		return ""
	}
	id, _ := attrs["blockId"].(string)
	return strings.TrimSpace(id)
}

func setBlockIDOnNode(node map[string]any, id string) {
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		node["attrs"] = attrs
	}
	attrs["blockId"] = id
}

func validUUID(s string) bool {
	_, err := uuid.Parse(strings.TrimSpace(s))
	return err == nil
}

func hasSeen(seen map[string]struct{}, id string) bool {
	if id == "" {
		return false
	}
	_, ok := seen[id]
	return ok
}

func blockSortKey(index int) string {
	return fmt.Sprintf("%012d", index)
}

// docsBlockContentEqual compares block content semantically. Editor and server
// saves serialize the same node differently (key order, whitespace, null-valued
// default attrs such as textAlign), and treating those round-trips as content
// edits would bump every block's revision and invalidate pending proposals.
func docsBlockContentEqual(a, b json.RawMessage) bool {
	if bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b)) {
		return true
	}
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(stripNullValues(av), stripNullValues(bv))
}

// stripNullValues removes null-valued object entries recursively; ProseMirror
// treats a null attr the same as an absent one.
func stripNullValues(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, entry := range value {
			if entry == nil {
				continue
			}
			out[key] = stripNullValues(entry)
		}
		return out
	case []any:
		out := make([]any, 0, len(value))
		for _, entry := range value {
			out = append(out, stripNullValues(entry))
		}
		return out
	default:
		return v
	}
}

func ptrIfNonEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := strings.TrimSpace(s)
	return &v
}
