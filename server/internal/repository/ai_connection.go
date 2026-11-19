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
	err := r.db.WithContext(ctx).Omit("encrypted_secret", "discovered_models").Where("workspace_id = ? AND (user_id = ? OR scope = ?) AND superseded_by IS NULL", workspace, user, "workspace").Order("created_at DESC").Find(&out).Error
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
	return r.withLockedTx(ctx, id, func(_ *gorm.DB, c *model.AIConnection) error { return fn(c) })
}

// WithLockedAgentGrant holds the connection, model grant and active subscriber
// membership for the duration of a single credential read or token rotation.
func (r *AIConnectionRepository) WithLockedAgentGrant(ctx context.Context, workspace string, selection *model.AIExecutionSelection, fn func(*model.AIConnection, *model.AIProfile) error) error {
	return r.withLockedTx(ctx, selection.Route.ConnectionID, func(tx *gorm.DB, c *model.AIConnection) error {
		var profile model.AIProfile
		lock := clause.Locking{Strength: "SHARE"}
		var member struct{ UserID string }
		if err := tx.Table("workspace_members").Select("user_id").Clauses(lock).Where("workspace_id = ? AND user_id = ? AND status = ?", workspace, selection.OwnerID, model.WorkspaceMemberStatusActive).Take(&member).Error; err != nil {
			return err
		}
		if err := tx.Clauses(lock).Where("id = ? AND workspace_id = ? AND user_id = ? AND scope = 'personal' AND agent_grant_id = ? AND deleted_at IS NULL", selection.ProfileID, workspace, selection.OwnerID, selection.AgentGrantID).Take(&profile).Error; err != nil {
			return err
		}
		return fn(c, &profile)
	})
}

func (r *AIConnectionRepository) withLockedTx(ctx context.Context, id string, fn func(*gorm.DB, *model.AIConnection) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c model.AIConnection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&c).Error; err != nil {
			return err
		}
		if err := fn(tx, &c); err != nil {
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

// DefaultProfile returns the workspace's live default profile, or nil.
func (r *AIConnectionRepository) DefaultProfile(ctx context.Context, workspace string) (*model.AIProfile, error) {
	var settings model.AIWorkspaceSettings
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspace).Take(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && settings.DefaultProfileID == nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var profile model.AIProfile
	err = r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND deleted_at IS NULL", workspace, *settings.DefaultProfileID).Take(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &profile, err
}

// RecordVerification stores a connection test outcome without touching secrets.
func (r *AIConnectionRepository) RecordVerification(ctx context.Context, id string, at time.Time, failure *string) error {
	return r.db.WithContext(ctx).Model(&model.AIConnection{}).Where("id = ?", id).
		Updates(map[string]any{"last_verified_at": at, "last_verification_error": failure}).Error
}

// SharedEmbeddingConnections returns the workspace-owned, live, connected
// connections for the given providers, oldest first. Secrets are included so
// the caller can decrypt them; callers must never serialize the result.
func (r *AIConnectionRepository) SharedEmbeddingConnections(ctx context.Context, workspace string, providers []string) ([]model.AIConnection, error) {
	var out []model.AIConnection
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND scope = ? AND user_id IS NULL AND superseded_by IS NULL AND status = ? AND provider IN ?",
			workspace, "workspace", "connected", providers).
		Order("created_at, id").Find(&out).Error
	return out, err
}

// WorkspaceExists rejects deleted workspaces before loading shared credentials.
// Workspaces have no lifecycle status column; edition policy handles billing gates.
func (r *AIConnectionRepository) WorkspaceExists(ctx context.Context, workspace string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Workspace{}).Where("id = ?", workspace).Count(&n).Error
	return n > 0, err
}

func (r *AIConnectionRepository) SaveDiscoveredModels(ctx context.Context, id string, models []model.DiscoveredAIModel, at time.Time) error {
	c := model.AIConnection{ID: id, DiscoveredModels: models, ModelsFetchedAt: &at}
	return r.db.WithContext(ctx).Model(&c).Select("discovered_models", "models_fetched_at").Updates(&c).Error
}

// ActiveGrantedRunsForMember includes scheduled runs with no triggering member.
func (r *AIConnectionRepository) ActiveGrantedRunsForMember(ctx context.Context, workspace, user string) ([]model.AgentRun, error) {
	var runs []model.AgentRun
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status IN ? AND input->'ai_selection'->>'agent_grant_id' IS NOT NULL AND (input->'ai_selection'->>'owner_id' = ? OR triggered_by_user_id = ?)", workspace, []string{"queued", "running", "paused"}, user, user).Find(&runs).Error
	return runs, err
}

type SubscriptionModelSource struct {
	model.AIConnection `gorm:"embedded"`
	SubscriberName     string
}

// Only project display and policy fields for connections backing shared models.
func (r *AIConnectionRepository) SubscriptionModelSources(ctx context.Context, workspace string, ids []string) (map[string]SubscriptionModelSource, error) {
	sources := make(map[string]SubscriptionModelSource)
	if len(ids) == 0 {
		return sources, nil
	}
	var rows []SubscriptionModelSource
	err := r.db.WithContext(ctx).Table("ai_connections AS c").
		Select("c.id, c.workspace_id, c.user_id, c.provider, c.funding, c.scope, c.status, c.account_id, c.name, u.full_name AS subscriber_name").
		Joins("JOIN users u ON u.id = c.user_id").
		Joins("JOIN workspace_members m ON m.workspace_id = c.workspace_id AND m.user_id = c.user_id AND m.status = ?", model.WorkspaceMemberStatusActive).
		Where("c.workspace_id = ? AND c.id IN ? AND c.provider = 'openai_chatgpt' AND c.scope = 'personal' AND c.superseded_by IS NULL", workspace, ids).Scan(&rows).Error
	for _, row := range rows {
		sources[row.ID] = row
	}
	return sources, err
}
