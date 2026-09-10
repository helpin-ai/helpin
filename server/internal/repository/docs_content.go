package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsContentRepository handles DB operations for document content.
type DocsContentRepository struct {
	db        *gorm.DB
	blockRepo *DocsBlockRepository
}

// NewDocsContentRepository creates a new DocsContentRepository.
func NewDocsContentRepository(db *gorm.DB) *DocsContentRepository {
	return &DocsContentRepository{db: db}
}

// SetBlockRepository enables compatibility dual-writes into docs_blocks for
// all content upserts, including legacy callers that still use this repository
// directly.
func (r *DocsContentRepository) SetBlockRepository(blockRepo *DocsBlockRepository) {
	r.blockRepo = blockRepo
}

// GetByDocumentID returns content for a document.
func (r *DocsContentRepository) GetByDocumentID(ctx context.Context, documentID string) (*model.DocsContent, error) {
	var content model.DocsContent
	if err := r.db.WithContext(ctx).Where("document_id = ?", documentID).First(&content).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs content: %w", err)
	}
	return &content, nil
}

// ListByDocumentIDs returns content records for the provided documents.
func (r *DocsContentRepository) ListByDocumentIDs(ctx context.Context, documentIDs []string) ([]model.DocsContent, error) {
	if len(documentIDs) == 0 {
		return []model.DocsContent{}, nil
	}
	var contents []model.DocsContent
	if err := r.db.WithContext(ctx).
		Where("document_id IN ?", documentIDs).
		Find(&contents).Error; err != nil {
		return nil, fmt.Errorf("list docs content by documents: %w", err)
	}
	return contents, nil
}

// ListByWorkspaceExcludingDocuments returns surviving content records in a workspace.
func (r *DocsContentRepository) ListByWorkspaceExcludingDocuments(ctx context.Context, workspaceID string, excludeDocumentIDs []string) ([]model.DocsContent, error) {
	var contents []model.DocsContent
	query := r.db.WithContext(ctx).
		Model(&model.DocsContent{}).
		Select("docs_contents.*").
		Joins("JOIN docs_documents dd ON dd.id = docs_contents.document_id").
		Where("dd.workspace_id = ? AND dd.deleted_at IS NULL", workspaceID)
	if len(excludeDocumentIDs) > 0 {
		query = query.Where("docs_contents.document_id NOT IN ?", excludeDocumentIDs)
	}
	if err := query.Find(&contents).Error; err != nil {
		return nil, fmt.Errorf("list surviving docs content: %w", err)
	}
	return contents, nil
}

// DeleteByDocumentIDs hard-deletes content rows for the provided documents.
func (r *DocsContentRepository) DeleteByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsContent{}).Error; err != nil {
		return fmt.Errorf("delete docs content by documents: %w", err)
	}
	return nil
}

// Upsert creates or updates content for a document. Also extracts content_text and computes word_count.
// Touches the parent document's updated_at so timestamps stay current.
func (r *DocsContentRepository) Upsert(ctx context.Context, documentID string, content json.RawMessage) (*model.DocsContent, error) {
	return r.UpsertWithActor(ctx, documentID, content, "")
}

// UpsertWithActor creates or updates content for a document and, when the
// block repository is wired, synchronizes addressable block rows in the same
// transaction.
func (r *DocsContentRepository) UpsertWithActor(ctx context.Context, documentID string, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	_, saved, err := r.transformContent(ctx, documentID, "", actorID, func(json.RawMessage) (json.RawMessage, error) { return content, nil })
	return saved, err
}

func (r *DocsContentRepository) upsertTx(ctx context.Context, tx *gorm.DB, documentID string, content json.RawMessage) error {
	contentText := extractPlainText(content)
	wordCount := countWords(contentText)

	var existing model.DocsContent
	err := tx.WithContext(ctx).Where("document_id = ?", documentID).First(&existing).Error

	if err == nil {
		updates := map[string]interface{}{
			"content":      content,
			"content_text": contentText,
			"word_count":   wordCount,
		}
		if err := tx.WithContext(ctx).Model(&model.DocsContent{}).Where("document_id = ?", documentID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update docs content: %w", err)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		c := &model.DocsContent{
			DocumentID:  documentID,
			Content:     content,
			ContentText: contentText,
			WordCount:   wordCount,
		}
		if err := tx.WithContext(ctx).Create(c).Error; err != nil {
			return fmt.Errorf("create docs content: %w", err)
		}
	} else {
		return fmt.Errorf("get docs content for upsert: %w", err)
	}

	// Touch the parent document's updated_at
	if err := tx.WithContext(ctx).Model(&model.DocsDocument{}).Where("id = ?", documentID).
		Update("updated_at", time.Now().UTC()).Error; err != nil {
		return fmt.Errorf("touch document updated_at: %w", err)
	}

	return nil
}

// ListBySpaceWithImportHTML returns all content records that have stored import HTML for a given space.
func (r *DocsContentRepository) ListBySpaceWithImportHTML(ctx context.Context, spaceID string) ([]model.DocsContent, error) {
	var contents []model.DocsContent
	if err := r.db.WithContext(ctx).
		Joins("JOIN docs_documents dd ON dd.id = docs_contents.document_id").
		Where("dd.space_id = ? AND docs_contents.import_source_html IS NOT NULL AND docs_contents.import_source_html != ''", spaceID).
		Find(&contents).Error; err != nil {
		return nil, fmt.Errorf("list reconvertible docs: %w", err)
	}
	return contents, nil
}

// ListImportSourceObjectIDs returns completed imported source IDs in a space.
func (r *DocsContentRepository) ListImportSourceObjectIDs(
	ctx context.Context,
	spaceID string,
	sourceSystem string,
) ([]string, error) {
	var sourceIDs []string
	if err := r.db.WithContext(ctx).
		Model(&model.DocsContent{}).
		Joins("JOIN docs_documents dd ON dd.id = docs_contents.document_id").
		Where("dd.space_id = ? AND docs_contents.import_source_system = ?", spaceID, sourceSystem).
		Where("docs_contents.import_source_object_id IS NOT NULL").
		Pluck("docs_contents.import_source_object_id", &sourceIDs).Error; err != nil {
		return nil, fmt.Errorf("list imported source object ids: %w", err)
	}
	return sourceIDs, nil
}

// UpdateImportProvenance sets the import source fields on a content record.
func (r *DocsContentRepository) UpdateImportProvenance(ctx context.Context, contentID, sourceHTML, sourceSystem, sourceObjectID string) error {
	if err := r.db.WithContext(ctx).Model(&model.DocsContent{}).Where("id = ?", contentID).Updates(map[string]interface{}{
		"import_source_html":      sourceHTML,
		"import_source_system":    sourceSystem,
		"import_source_object_id": sourceObjectID,
	}).Error; err != nil {
		return fmt.Errorf("update import provenance: %w", err)
	}
	return nil
}

// extractPlainText extracts plain text from TipTap/ProseMirror JSON content.
// Walks the node tree and concatenates all text node values.
func extractPlainText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	var sb strings.Builder
	extractTextFromNode(node, &sb)
	return sb.String()
}

func extractTextFromNode(node map[string]json.RawMessage, sb *strings.Builder) {
	// If this node has "text", it's a text node.
	if textRaw, ok := node["text"]; ok {
		var text string
		if err := json.Unmarshal(textRaw, &text); err == nil {
			sb.WriteString(text)
		}
	}

	// Extract text from htmlBlock attrs.html (strip tags, keep text).
	if attrsRaw, ok := node["attrs"]; ok {
		var attrs map[string]json.RawMessage
		if err := json.Unmarshal(attrsRaw, &attrs); err == nil {
			if htmlRaw, ok := attrs["html"]; ok {
				var htmlStr string
				if err := json.Unmarshal(htmlRaw, &htmlStr); err == nil && htmlStr != "" {
					sb.WriteString(stripHTMLTags(htmlStr))
					sb.WriteString(" ")
				}
			}
		}
	}

	// Recurse into "content" array.
	if contentRaw, ok := node["content"]; ok {
		var children []map[string]json.RawMessage
		if err := json.Unmarshal(contentRaw, &children); err == nil {
			for _, child := range children {
				extractTextFromNode(child, sb)
				// Add space between block-level elements.
				if nodeType, ok := child["type"]; ok {
					var t string
					if err := json.Unmarshal(nodeType, &t); err == nil {
						if t == "paragraph" || t == "heading" || t == "bulletList" || t == "orderedList" || t == "blockquote" || t == "codeBlock" || t == "listItem" || t == "callout" || t == "htmlBlock" {
							sb.WriteString(" ")
						}
					}
				}
			}
		}
	}
}

// stripHTMLTags removes HTML tags and returns plain text content.
func stripHTMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			sb.WriteByte(' ')
			continue
		}
		if !inTag {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func countWords(text string) int {
	return len(strings.Fields(text))
}
