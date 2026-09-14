package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIProfileBootstrapRepository is used only by the explicit operator bootstrap.
type AIProfileBootstrapRepository struct{ db *gorm.DB }

func NewAIProfileBootstrapRepository(db *gorm.DB) *AIProfileBootstrapRepository {
	return &AIProfileBootstrapRepository{db: db}
}

func (r *AIProfileBootstrapRepository) Transaction(ctx context.Context, workspace string, apply func(*AIProfileBootstrapRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ID string }
		if err := tx.Table("workspaces").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", workspace).Take(&row).Error; err != nil {
			return err
		}
		return apply(NewAIProfileBootstrapRepository(tx))
	})
}

func (r *AIProfileBootstrapRepository) Agents(ctx context.Context, workspace string) ([]model.Agent, error) {
	var agents []model.Agent
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ?", workspace).Order("id").Find(&agents).Error
	return agents, err
}

func (r *AIProfileBootstrapRepository) Connection(ctx context.Context, id string) (*model.AIConnection, error) {
	return NewAIConnectionRepository(r.db).Get(ctx, id)
}

func (r *AIProfileBootstrapRepository) SaveConnection(ctx context.Context, c *model.AIConnection) error {
	return r.db.WithContext(ctx).Save(c).Error
}

// EnsureProfile never resurrects or overwrites a user's edited profile.
func (r *AIProfileBootstrapRepository) EnsureProfile(ctx context.Context, p *model.AIProfile) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(p)
	return result.RowsAffected == 1, result.Error
}

func (r *AIProfileBootstrapRepository) AssignAgent(ctx context.Context, workspace, agent, profile string) error {
	return r.db.WithContext(ctx).Model(&model.Agent{}).Where("workspace_id = ? AND id = ? AND ai_profile_id IS NULL", workspace, agent).
		UpdateColumn("ai_profile_id", profile).Error
}

// EnsureDefault only initializes an absent settings row; a cleared default is
// an intentional user choice and is preserved on subsequent bootstrap runs.
func (r *AIProfileBootstrapRepository) EnsureDefault(ctx context.Context, settings *model.AIWorkspaceSettings) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(settings).Error
}

func (r *AIProfileBootstrapRepository) NonterminalRuns(ctx context.Context, workspace string) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	err := r.db.WithContext(ctx).Select("id", "input").Where("workspace_id = ? AND status IN ?", workspace,
		[]string{"queued", "running", "paused"}).Order("id").Find(&runs).Error
	return runs, err
}
