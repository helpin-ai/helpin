package repository

import (
	"context"
	"errors"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AIConnectionRepository struct{ db *gorm.DB }

func NewAIConnectionRepository(db *gorm.DB) *AIConnectionRepository {
	return &AIConnectionRepository{db: db}
}
func (r *AIConnectionRepository) Create(ctx context.Context, c *model.AIConnection) error {
	return r.db.WithContext(ctx).Create(c).Error
}
func (r *AIConnectionRepository) List(ctx context.Context, workspace, user string) ([]model.AIConnection, error) {
	out := []model.AIConnection{}
	err := r.db.WithContext(ctx).Omit("encrypted_secret").Where("workspace_id = ? AND (user_id = ? OR scope = ?) AND superseded_by IS NULL", workspace, user, "workspace").Order("created_at DESC").Find(&out).Error
	return out, err
}
func (r *AIConnectionRepository) Get(ctx context.Context, id string) (*model.AIConnection, error) {
	var c model.AIConnection
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

// WithLocked serializes device polls and refresh-token rotation across replicas.
// Callbacks may return a deferred public error after persisting a reconnect status.
func (r *AIConnectionRepository) WithLocked(ctx context.Context, id string, fn func(*model.AIConnection) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c model.AIConnection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&c).Error; err != nil {
			return err
		}
		if err := fn(&c); err != nil {
			return err
		}
		c.UpdatedAt = time.Now().UTC()
		return tx.Save(&c).Error
	})
}
func (r *AIConnectionRepository) BoundRuns(ctx context.Context, workspace, user, id string) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND input->>'model_connection_id' = ? AND status IN ?", workspace, id, []string{"queued", "running", "paused"})
	if user != "" {
		query = query.Where("triggered_by_user_id = ?", user)
	}
	err := query.Find(&runs).Error
	return runs, err
}
func (r *AIConnectionRepository) Run(ctx context.Context, workspace, id string) (*model.AgentRun, error) {
	var run model.AgentRun
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspace, id).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func (r *AIConnectionRepository) ActiveMember(ctx context.Context, workspace, user string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).Where("workspace_id = ? AND user_id = ? AND status = ?", workspace, user, model.WorkspaceMemberStatusActive).Count(&n).Error
	return n > 0, err
}

// WorkspaceExists rejects deleted workspaces before loading shared credentials.
// Workspaces have no lifecycle status column; edition policy handles billing gates.
func (r *AIConnectionRepository) WorkspaceExists(ctx context.Context, workspace string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Workspace{}).Where("id = ?", workspace).Count(&n).Error
	return n > 0, err
}
