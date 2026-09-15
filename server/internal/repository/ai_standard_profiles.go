package repository

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIStandardProfileRepository provisions standard workspace profiles.
type AIStandardProfileRepository struct{ db *gorm.DB }

func NewAIStandardProfileRepository(db *gorm.DB) *AIStandardProfileRepository {
	return &AIStandardProfileRepository{db: db}
}

func (r *AIStandardProfileRepository) Transaction(ctx context.Context, workspace string, apply func(*AIStandardProfileRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ID string }
		if err := tx.Table("workspaces").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", workspace).Take(&row).Error; err != nil {
			return err
		}
		return apply(NewAIStandardProfileRepository(tx))
	})
}

func (r *AIStandardProfileRepository) Workspaces(ctx context.Context) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Table("workspaces").Order("id").Pluck("id", &ids).Error
	return ids, err
}

func (r *AIStandardProfileRepository) Connection(ctx context.Context, id string) (*model.AIConnection, error) {
	// Coordinate with credential refresh and disconnect, which lock this row.
	var connection model.AIConnection
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&connection).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &connection, err
}

// ManagedConnection reuses the existing EE default, including a disconnected
// one. A disconnected default must not cause provisioning to create a bypass.
func (r *AIStandardProfileRepository) ManagedConnection(ctx context.Context, workspace, provider string) (*model.AIConnection, error) {
	var connection model.AIConnection
	query := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND provider = ? AND scope = 'workspace' AND user_id IS NULL AND funding = 'managed' AND superseded_by IS NULL", workspace, provider)
	// Prefer a pre-existing managed connection over the later generated copy.
	err := query.Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "CASE WHEN id = ? THEN 1 ELSE 0 END, created_at, id", Vars: []any{model.StandardAIConnectionID(workspace, provider)}}}).Take(&connection).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &connection, err
}

func (r *AIStandardProfileRepository) SaveConnection(ctx context.Context, c *model.AIConnection) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// EnsureProfile never resurrects or overwrites a user's edited profile.
func (r *AIStandardProfileRepository) EnsureProfile(ctx context.Context, p *model.AIProfile) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(p)
	return result.RowsAffected == 1, result.Error
}

// EnsureDefault only initializes an absent settings row; a cleared default is
// an intentional user choice and is preserved on subsequent provisioning.
func (r *AIStandardProfileRepository) EnsureDefault(ctx context.Context, settings *model.AIWorkspaceSettings) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(settings).Error
}
