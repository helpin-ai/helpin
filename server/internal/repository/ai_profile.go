package repository

import (
	"context"
	"errors"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAIProfileInUse requires agents to be reassigned before a profile is deleted.
var ErrAIProfileInUse = errors.New("AI profile is used by agents; change their AI profile before deleting it")

// ErrAIProfileChanged prevents lost edits and stale default changes.
var ErrAIProfileChanged = errors.New("AI profile changed; reload and retry")

type AIProfileRepository struct{ db *gorm.DB }

func NewAIProfileRepository(db *gorm.DB) *AIProfileRepository { return &AIProfileRepository{db: db} }

func (r *AIProfileRepository) Get(ctx context.Context, workspace, id string) (*model.AIProfile, error) {
	var p model.AIProfile
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND deleted_at IS NULL", workspace, id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *AIProfileRepository) List(ctx context.Context, workspace, user string) ([]model.AIProfile, error) {
	profiles := []model.AIProfile{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND (scope = ? OR user_id = ?) AND deleted_at IS NULL", workspace, "workspace", user).Order("name,id").Find(&profiles).Error
	return profiles, err
}

func (r *AIProfileRepository) Create(ctx context.Context, p *model.AIProfile) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *AIProfileRepository) Update(ctx context.Context, p *model.AIProfile, expected int64) error {
	// Use struct fields so GORM applies the JSON serializers to both routes.
	result := r.db.WithContext(ctx).Model(p).Where("workspace_id = ? AND revision = ? AND deleted_at IS NULL", p.WorkspaceID, expected).
		Select("name", "primary", "fallback", "revision", "updated_at").Updates(p)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAIProfileChanged
	}
	return nil
}

func (r *AIProfileRepository) Delete(ctx context.Context, workspace, id string, expected int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.AIProfile{}).Where("workspace_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL", workspace, id, expected).
			Updates(map[string]any{"deleted_at": time.Now().UTC(), "revision": expected + 1})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAIProfileChanged
		}
		var count int64
		if err := tx.Model(&model.Agent{}).Where("workspace_id = ? AND ai_profile_id = ?", workspace, id).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrAIProfileInUse
		}
		return tx.Model(&model.AIWorkspaceSettings{}).Where("workspace_id = ? AND default_profile_id = ?", workspace, id).Update("default_profile_id", nil).Error
	})
}

func (r *AIProfileRepository) Settings(ctx context.Context, workspace string) (*model.AIWorkspaceSettings, error) {
	s := model.AIWorkspaceSettings{WorkspaceID: workspace}
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspace).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return &s, err
}

func (r *AIProfileRepository) SetDefault(ctx context.Context, workspace string, profile *model.AIProfile) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		settings := model.AIWorkspaceSettings{WorkspaceID: workspace, UpdatedAt: time.Now().UTC()}
		if profile != nil {
			var p model.AIProfile
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND scope = ? AND user_id IS NULL AND deleted_at IS NULL", workspace, profile.ID, "workspace").First(&p).Error; err != nil {
				return err
			}
			if p.Revision != profile.Revision {
				return ErrAIProfileChanged
			}
			settings.DefaultProfileID = &p.ID
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}}, DoUpdates: clause.AssignmentColumns([]string{"default_profile_id", "updated_at"})}).Create(&settings).Error
	})
}

// SetVisibility leaves routes and all referencing agents/defaults intact.
func (r *AIProfileRepository) SetVisibility(ctx context.Context, workspace, id string, revision int64, hidden bool) error {
	result := r.db.WithContext(ctx).Model(&model.AIProfile{}).Where("workspace_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL", workspace, id, revision).
		Updates(map[string]any{"hidden_from_ask_agent": hidden, "revision": revision + 1, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAIProfileChanged
	}
	return nil
}

// EnableModel serializes one-click additions on the connection across replicas.
// Existing variants are retained, including their names, controls and fallbacks.
func (r *AIProfileRepository) EnableModel(ctx context.Context, p *model.AIProfile) (*model.AIProfile, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var connection model.AIConnection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", p.Primary.ConnectionID, p.WorkspaceID).First(&connection).Error; err != nil {
			return err
		}
		var existing []model.AIProfile
		query := tx.Where("workspace_id = ? AND scope = ? AND deleted_at IS NULL", p.WorkspaceID, p.Scope)
		if p.UserID == nil {
			query = query.Where("user_id IS NULL")
		} else {
			query = query.Where("user_id = ?", *p.UserID)
		}
		if err := query.Order("hidden_from_ask_agent,created_at,id").Find(&existing).Error; err != nil {
			return err
		}
		for i := range existing {
			candidate := &existing[i]
			if candidate.Primary.ConnectionID != p.Primary.ConnectionID || candidate.Primary.Model.Model != p.Primary.Model.Model {
				continue
			}
			if candidate.HiddenFromAskAgent {
				result := tx.Model(candidate).Where("revision = ?", candidate.Revision).Updates(map[string]any{"hidden_from_ask_agent": false, "revision": candidate.Revision + 1, "updated_at": time.Now().UTC()})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrAIProfileChanged
				}
			}
			p = candidate
			return nil
		}
		return tx.Create(p).Error
	})
	return p, err
}
