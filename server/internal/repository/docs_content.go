package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsContentRepository handles DB operations for document content.
type DocsContentRepository struct {
	db *gorm.DB
}

// NewDocsContentRepository creates a new DocsContentRepository.
func NewDocsContentRepository(db *gorm.DB) *DocsContentRepository {
	return &DocsContentRepository{db: db}
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

// Upsert creates or updates content for a document. Also extracts content_text and computes word_count.
// Touches the parent document's updated_at so timestamps stay current.
func (r *DocsContentRepository) Upsert(ctx context.Context, documentID string, content json.RawMessage) (*model.DocsContent, error) {
	contentText := extractPlainText(content)
	wordCount := countWords(contentText)

	existing, err := r.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		updates := map[string]interface{}{
			"content":      content,
			"content_text": contentText,
			"word_count":   wordCount,
		}
		if err := r.db.WithContext(ctx).Model(&model.DocsContent{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update docs content: %w", err)
		}
	} else {
		c := &model.DocsContent{
			DocumentID:  documentID,
			Content:     content,
			ContentText: contentText,
			WordCount:   wordCount,
		}
		if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
			return nil, fmt.Errorf("create docs content: %w", err)
		}
	}

	// Touch the parent document's updated_at
	if err := r.db.WithContext(ctx).Model(&model.DocsDocument{}).Where("id = ?", documentID).
		Update("updated_at", gorm.Expr("NOW()")).Error; err != nil {
		return nil, fmt.Errorf("touch document updated_at: %w", err)
	}

	return r.GetByDocumentID(ctx, documentID)
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
						if t == "paragraph" || t == "heading" || t == "bulletList" || t == "orderedList" || t == "blockquote" || t == "codeBlock" || t == "listItem" {
							sb.WriteString(" ")
						}
					}
				}
			}
		}
	}
}

func countWords(text string) int {
	return len(strings.Fields(text))
}
