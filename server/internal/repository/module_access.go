package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type WorkspaceModuleGrantRepository struct {
	db *gorm.DB
}

func NewWorkspaceModuleGrantRepository(db *gorm.DB) *WorkspaceModuleGrantRepository {
	return &WorkspaceModuleGrantRepository{db: db}
}

func (r *WorkspaceModuleGrantRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.WorkspaceModuleGrant, error) {
	var grants []model.WorkspaceModuleGrant
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("module, subject_type, subject_id").
		Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("list module grants: %w", err)
	}
	return grants, nil
}

func (r *WorkspaceModuleGrantRepository) Upsert(ctx context.Context, grant model.WorkspaceModuleGrant) (*model.WorkspaceModuleGrant, error) {
	if grant.AccessLevel == "" {
		grant.AccessLevel = model.ModuleGrantAccessLevelMember
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "workspace_id"},
				{Name: "module"},
				{Name: "subject_type"},
				{Name: "subject_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"access_level":  grant.AccessLevel,
				"created_by_id": grant.CreatedByID,
				"updated_at":    gorm.Expr("NOW()"),
			}),
		}).
		Create(&grant).Error; err != nil {
		return nil, fmt.Errorf("upsert module grant: %w", err)
	}

	var stored model.WorkspaceModuleGrant
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND module = ? AND subject_type = ? AND subject_id = ?",
			grant.WorkspaceID, grant.Module, grant.SubjectType, grant.SubjectID).
		First(&stored).Error; err != nil {
		return nil, fmt.Errorf("load module grant: %w", err)
	}
	return &stored, nil
}

func (r *WorkspaceModuleGrantRepository) Delete(ctx context.Context, workspaceID, grantID string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, grantID).
		Delete(&model.WorkspaceModuleGrant{}).Error; err != nil {
		return fmt.Errorf("delete module grant: %w", err)
	}
	return nil
}

func (r *WorkspaceModuleGrantRepository) DeleteBySubject(ctx context.Context, workspaceID string, subjectType model.ModuleGrantSubjectType, subjectID string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, subjectType, subjectID).
		Delete(&model.WorkspaceModuleGrant{}).Error; err != nil {
		return fmt.Errorf("delete module grants by subject: %w", err)
	}
	return nil
}

func (r *WorkspaceModuleGrantRepository) ListAccessibleModules(ctx context.Context, workspaceID, workspaceMemberID string, teamIDs []string) ([]model.ModuleID, error) {
	query := r.db.WithContext(ctx).
		Model(&model.WorkspaceModuleGrant{}).
		Where("workspace_id = ?", workspaceID)

	if len(teamIDs) == 0 {
		query = query.Where(
			"(subject_type = ? AND subject_id = ?)",
			model.ModuleGrantSubjectWorkspaceMember, workspaceMemberID,
		)
	} else {
		query = query.Where(
			"(subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?)",
			model.ModuleGrantSubjectWorkspaceMember, workspaceMemberID,
			model.ModuleGrantSubjectTeam, teamIDs,
		)
	}

	var modules []model.ModuleID
	if err := query.Distinct("module").Scan(&modules).Error; err != nil {
		return nil, fmt.Errorf("list accessible modules: %w", err)
	}
	return modules, nil
}

func (r *WorkspaceModuleGrantRepository) TeamBelongsToWorkspace(ctx context.Context, workspaceID, teamID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceTeam{}).
		Where("id = ? AND workspace_id = ?", teamID, workspaceID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check team workspace ownership: %w", err)
	}
	return count > 0, nil
}

func (r *WorkspaceModuleGrantRepository) WorkspaceMemberBelongsToWorkspace(ctx context.Context, workspaceID, workspaceMemberID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceMember{}).
		Where("id = ? AND workspace_id = ?", workspaceMemberID, workspaceID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check workspace member ownership: %w", err)
	}
	return count > 0, nil
}
