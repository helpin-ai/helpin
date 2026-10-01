package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListIndexedDocuments returns eligible documents with persisted index chunks.
func (r *AgentKnowledgeSourceRepository) ListIndexedDocuments(ctx context.Context, workspaceID, spaceID string) ([]model.DocsDocument, error) {
	documents := []model.DocsDocument{}
	err := r.db.WithContext(ctx).Table("docs_documents AS d").
		Select("d.id, d.workspace_id, d.space_id, d.collection_id, d.title").
		Joins("JOIN docs_spaces s ON s.id = d.space_id AND s.workspace_id = d.workspace_id").
		Where("d.workspace_id = ? AND d.space_id = ?", workspaceID, spaceID).
		Where("d.status = ? AND d.deleted_at IS NULL AND s.deleted_at IS NULL", model.DocStatusPublished).
		Where("s.type IN ?", []string{model.SpaceTypeInternal, model.SpaceTypeExternalCapable}).
		Where(`EXISTS (SELECT 1 FROM docs_chunks c WHERE c.document_id = d.id AND c.workspace_id = d.workspace_id AND c.space_id = d.space_id)`).
		Where(`(s.type = ? OR EXISTS (SELECT 1 FROM docs_helpcenter_articles ha WHERE ha.document_id = d.id AND ha.public_published_at IS NOT NULL))`, model.SpaceTypeInternal).
		Order("LOWER(d.title), d.id").Find(&documents).Error
	if err != nil {
		return nil, fmt.Errorf("list indexed documents: %w", err)
	}
	return documents, nil
}
