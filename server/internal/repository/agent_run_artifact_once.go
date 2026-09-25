package repository

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm/clause"
)

// CreateOnce preserves the first captured outcome under callback retries.
func (r *AgentRunArtifactRepository) CreateOnce(ctx context.Context, artifact *model.AgentRunArtifact) error {
	sanitizeAgentRunArtifactForPostgres(artifact)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(artifact).Error
}
