package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GitIntegrationRepository handles DB operations for git integrations.
type GitIntegrationRepository struct {
	db *gorm.DB
}

// NewGitIntegrationRepository creates a new GitIntegrationRepository.
func NewGitIntegrationRepository(db *gorm.DB) *GitIntegrationRepository {
	return &GitIntegrationRepository{db: db}
}

func workspaceOrgScopeQuery(db *gorm.DB, workspaceID string) *gorm.DB {
	return db.Where(`
		(
			organization_id = (
				SELECT organization_id
				FROM workspaces
				WHERE id = ?
				LIMIT 1
			)
		)
		OR (organization_id IS NULL AND workspace_id = ?)
	`, workspaceID, workspaceID)
}

// List returns active integrations available to a workspace's organization.
func (r *GitIntegrationRepository) List(ctx context.Context, workspaceID string) ([]model.GitIntegration, error) {
	var integrations []model.GitIntegration
	if err := workspaceOrgScopeQuery(r.db.WithContext(ctx), workspaceID).
		Where("active = ?", true).
		Order("created_at DESC").
		Find(&integrations).Error; err != nil {
		return nil, fmt.Errorf("list git integrations: %w", err)
	}
	return integrations, nil
}

// GetByID returns a single integration visible to a workspace's organization.
func (r *GitIntegrationRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.GitIntegration, error) {
	var integration model.GitIntegration
	if err := workspaceOrgScopeQuery(r.db.WithContext(ctx), workspaceID).
		Where("id = ?", id).
		First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git integration: %w", err)
	}
	return &integration, nil
}

// GetByIDAny returns a single integration without workspace scoping.
func (r *GitIntegrationRepository) GetByIDAny(ctx context.Context, id string) (*model.GitIntegration, error) {
	var integration model.GitIntegration
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git integration by id: %w", err)
	}
	return &integration, nil
}

// Create creates a new integration.
func (r *GitIntegrationRepository) Create(ctx context.Context, integration *model.GitIntegration) error {
	if err := r.db.WithContext(ctx).Create(integration).Error; err != nil {
		return fmt.Errorf("create git integration: %w", err)
	}
	return nil
}

// Update saves an integration.
func (r *GitIntegrationRepository) Update(ctx context.Context, integration *model.GitIntegration) error {
	if err := r.db.WithContext(ctx).Save(integration).Error; err != nil {
		return fmt.Errorf("update git integration: %w", err)
	}
	return nil
}

// GetActiveByOrganization returns the active integration for an organization/provider pair.
func (r *GitIntegrationRepository) GetActiveByOrganization(ctx context.Context, organizationID, provider string) (*model.GitIntegration, error) {
	var integration model.GitIntegration
	if err := r.db.WithContext(ctx).
		Where("organization_id = ? AND provider = ? AND active = ?", organizationID, provider, true).
		Order("created_at DESC").
		First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active integration by organization: %w", err)
	}
	return &integration, nil
}

// SoftDeleteByID soft-deletes an integration.
func (r *GitIntegrationRepository) SoftDeleteByID(ctx context.Context, id string) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.GitIntegration{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"active":     false,
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("soft delete git integration: %w", err)
	}
	return nil
}

// SoftDeleteByOrganizationIfNoRemainingWorkspaces deactivates org integrations when the org has no more workspaces.
func (r *GitIntegrationRepository) SoftDeleteByOrganizationIfNoRemainingWorkspaces(ctx context.Context, organizationID string) error {
	if organizationID == "" {
		return nil
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.GitIntegration{}).
		Where("organization_id = ?", organizationID).
		Where("NOT EXISTS (SELECT 1 FROM workspaces WHERE organization_id = ?)", organizationID).
		Updates(map[string]any{
			"active":     false,
			"deleted_at": gorm.Expr("COALESCE(deleted_at, ?)", now),
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("soft delete git integrations for empty organization: %w", err)
	}
	return nil
}

// GetByInstallationID returns the newest integration row for an app installation, including inactive rows.
func (r *GitIntegrationRepository) GetByInstallationID(ctx context.Context, provider, installationID string) (*model.GitIntegration, error) {
	var integration model.GitIntegration
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ?", provider, installationID).
		Order("active DESC, updated_at DESC, created_at DESC").
		First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get integration by installation id: %w", err)
	}
	return &integration, nil
}

// ListSoftDeletedBefore returns integrations whose grace window has expired.
func (r *GitIntegrationRepository) ListSoftDeletedBefore(ctx context.Context, cutoff time.Time) ([]model.GitIntegration, error) {
	var integrations []model.GitIntegration
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Order("deleted_at ASC").
		Find(&integrations).Error; err != nil {
		return nil, fmt.Errorf("list soft-deleted git integrations: %w", err)
	}
	return integrations, nil
}

// HardDeleteByIDIfNoRepositories removes an integration once all repo tombstones are gone.
func (r *GitIntegrationRepository) HardDeleteByIDIfNoRepositories(ctx context.Context, id string) (bool, error) {
	result := r.db.WithContext(ctx).
		Where("id = ? AND NOT EXISTS (SELECT 1 FROM git_repositories WHERE integration_id = ?)", id, id).
		Delete(&model.GitIntegration{})
	if result.Error != nil {
		return false, fmt.Errorf("hard delete git integration: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// GitRepositoryRepository handles DB operations for synced repositories.
type GitRepositoryRepository struct {
	db *gorm.DB
}

// NewGitRepositoryRepository creates a new GitRepositoryRepository.
func NewGitRepositoryRepository(db *gorm.DB) *GitRepositoryRepository {
	return &GitRepositoryRepository{db: db}
}

// List returns selected repositories for a workspace.
func (r *GitRepositoryRepository) List(ctx context.Context, workspaceID string) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL AND active = ? AND archived = ? AND selected = ?", workspaceID, true, false, true).
		Order("full_name ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list git repositories: %w", err)
	}
	return repos, nil
}

// ListAll returns the full repository catalog for a workspace.
func (r *GitRepositoryRepository) ListAll(ctx context.Context, workspaceID string) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Order("archived ASC, selected DESC, full_name ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list all git repositories: %w", err)
	}
	return repos, nil
}

// GetByID loads a repository by ID.
func (r *GitRepositoryRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.GitRepository, error) {
	var repo model.GitRepository
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND deleted_at IS NULL AND active = ?", workspaceID, id, true).First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git repository: %w", err)
	}
	return &repo, nil
}

// GetByFullName loads a repository by workspace-scoped full name.
func (r *GitRepositoryRepository) GetByFullName(ctx context.Context, workspaceID, fullName string) (*model.GitRepository, error) {
	var repo model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND lower(full_name) = lower(?) AND deleted_at IS NULL AND active = ?", workspaceID, fullName, true).
		First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git repository by full name: %w", err)
	}
	return &repo, nil
}

// GetByIDAny loads a repository by ID regardless of workspace or lifecycle state.
func (r *GitRepositoryRepository) GetByIDAny(ctx context.Context, id string) (*model.GitRepository, error) {
	var repo model.GitRepository
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git repository by id: %w", err)
	}
	return &repo, nil
}

// Update saves a repository record.
func (r *GitRepositoryRepository) Update(ctx context.Context, repo *model.GitRepository) error {
	if err := r.db.WithContext(ctx).Save(repo).Error; err != nil {
		return fmt.Errorf("update git repository: %w", err)
	}
	return nil
}

// UpsertRepository creates or reactivates a repo claim.
func (r *GitRepositoryRepository) UpsertRepository(ctx context.Context, repo *model.GitRepository) error {
	if repo == nil {
		return fmt.Errorf("repository is required")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var live model.GitRepository
		err := tx.
			Where("integration_id = ? AND external_id = ? AND deleted_at IS NULL", repo.IntegrationID, repo.ExternalID).
			First(&live).Error
		switch {
		case err == nil:
			if strings.TrimSpace(live.WorkspaceID) != strings.TrimSpace(repo.WorkspaceID) {
				return fmt.Errorf("repository already claimed by workspace %s", live.WorkspaceID)
			}
			live.Provider = repo.Provider
			live.FullName = repo.FullName
			live.DefaultBranch = repo.DefaultBranch
			live.Permissions = repo.Permissions
			live.Private = repo.Private
			live.Archived = repo.Archived
			live.Selected = repo.Selected
			live.Active = repo.Active
			live.DeletedAt = nil
			live.UpdatedAt = now
			if err := tx.Save(&live).Error; err != nil {
				return fmt.Errorf("update git repository: %w", err)
			}
			repo.ID = live.ID
			repo.CreatedAt = live.CreatedAt
			repo.UpdatedAt = live.UpdatedAt
			return nil
		case err != nil && err != gorm.ErrRecordNotFound:
			return fmt.Errorf("load live git repository for upsert: %w", err)
		}

		var tombstone model.GitRepository
		err = tx.
			Where("integration_id = ? AND external_id = ? AND workspace_id = ? AND deleted_at IS NOT NULL", repo.IntegrationID, repo.ExternalID, repo.WorkspaceID).
			Order("updated_at DESC, created_at DESC").
			First(&tombstone).Error
		switch {
		case err == nil:
			tombstone.Provider = repo.Provider
			tombstone.FullName = repo.FullName
			tombstone.DefaultBranch = repo.DefaultBranch
			tombstone.Permissions = repo.Permissions
			tombstone.Private = repo.Private
			tombstone.Archived = repo.Archived
			tombstone.Selected = repo.Selected
			tombstone.Active = repo.Active
			tombstone.DeletedAt = nil
			tombstone.UpdatedAt = now
			if err := tx.Save(&tombstone).Error; err != nil {
				return fmt.Errorf("reactivate git repository: %w", err)
			}
			repo.ID = tombstone.ID
			repo.CreatedAt = tombstone.CreatedAt
			repo.UpdatedAt = tombstone.UpdatedAt
			return nil
		case err != nil && err != gorm.ErrRecordNotFound:
			return fmt.Errorf("load tombstoned git repository for upsert: %w", err)
		}

		repo.CreatedAt = now
		repo.UpdatedAt = now
		if err := tx.Create(repo).Error; err != nil {
			return fmt.Errorf("create git repository: %w", err)
		}
		return nil
	})
}

// ListByWorkspaceAndIntegration returns all live repo claims for a workspace/integration pair.
func (r *GitRepositoryRepository) ListByWorkspaceAndIntegration(ctx context.Context, workspaceID, integrationID string) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND integration_id = ? AND deleted_at IS NULL", workspaceID, integrationID).
		Order("full_name ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list git repositories by workspace and integration: %w", err)
	}
	return repos, nil
}

// ListActiveByIntegration returns all live repo claims for an integration.
func (r *GitRepositoryRepository) ListActiveByIntegration(ctx context.Context, integrationID string) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("integration_id = ? AND deleted_at IS NULL AND active = ?", integrationID, true).
		Order("full_name ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list active git repositories by integration: %w", err)
	}
	return repos, nil
}

// CountActiveByIntegration returns the number of live repo claims for an integration.
func (r *GitRepositoryRepository) CountActiveByIntegration(ctx context.Context, integrationID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.GitRepository{}).
		Where("integration_id = ? AND deleted_at IS NULL AND active = ?", integrationID, true).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count active git repositories by integration: %w", err)
	}
	return count, nil
}

// GetByExternalID returns the newest repo row for an installation repo id.
func (r *GitRepositoryRepository) GetByExternalID(ctx context.Context, integrationID, externalID string) (*model.GitRepository, error) {
	var repo model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("integration_id = ? AND external_id = ?", integrationID, externalID).
		Order("deleted_at IS NULL DESC, updated_at DESC, created_at DESC").
		First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git repository by external id: %w", err)
	}
	return &repo, nil
}

// GetActiveByExternalID returns the live repo claim for an installation repo id.
func (r *GitRepositoryRepository) GetActiveByExternalID(ctx context.Context, integrationID, externalID string) (*model.GitRepository, error) {
	var repo model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("integration_id = ? AND external_id = ? AND deleted_at IS NULL AND active = ?", integrationID, externalID, true).
		First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get active git repository by external id: %w", err)
	}
	return &repo, nil
}

// GetClaimedByByExternalID returns the live workspace claim for an installation repo id.
func (r *GitRepositoryRepository) GetClaimedByByExternalID(ctx context.Context, integrationID, externalID string) (*model.GitAvailableRepoClaim, error) {
	row := &model.GitAvailableRepoClaim{}
	if err := r.db.WithContext(ctx).
		Table("git_repositories gr").
		Select("gr.workspace_id, w.name AS workspace_name, gr.id AS repo_id").
		Joins("JOIN workspaces w ON w.id = gr.workspace_id").
		Where("gr.integration_id = ? AND gr.external_id = ? AND gr.deleted_at IS NULL AND gr.active = ?", integrationID, externalID, true).
		Scan(row).Error; err != nil {
		return nil, fmt.Errorf("get claimed workspace by external id: %w", err)
	}
	if row.WorkspaceID == "" {
		return nil, nil
	}
	return row, nil
}

// SoftDeleteByID soft-deletes a repo claim.
func (r *GitRepositoryRepository) SoftDeleteByID(ctx context.Context, id string) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.GitRepository{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"active":     false,
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("soft delete git repository: %w", err)
	}
	return nil
}

// SoftDeleteByIntegration soft-deletes all repo claims for an integration.
func (r *GitRepositoryRepository) SoftDeleteByIntegration(ctx context.Context, integrationID string) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.GitRepository{}).
		Where("integration_id = ? AND deleted_at IS NULL", integrationID).
		Updates(map[string]any{
			"active":     false,
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("soft delete git repositories by integration: %w", err)
	}
	return nil
}

// HardDeleteByWorkspace removes repo claims for a workspace.
func (r *GitRepositoryRepository) HardDeleteByWorkspace(ctx context.Context, workspaceID string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Delete(&model.GitRepository{}).Error; err != nil {
		return fmt.Errorf("hard delete git repositories by workspace: %w", err)
	}
	return nil
}

// ListSoftDeletedBefore returns repo tombstones whose grace window has expired.
func (r *GitRepositoryRepository) ListSoftDeletedBefore(ctx context.Context, cutoff time.Time) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Order("deleted_at ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list soft-deleted git repositories: %w", err)
	}
	return repos, nil
}

// HardDeleteByID deletes a repo tombstone permanently.
func (r *GitRepositoryRepository) HardDeleteByID(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.GitRepository{}).Error; err != nil {
		return fmt.Errorf("hard delete git repository: %w", err)
	}
	return nil
}

// ListAffectedWorkspacesByIntegration returns workspace usage counts for an integration.
func (r *GitRepositoryRepository) ListAffectedWorkspacesByIntegration(ctx context.Context, integrationID string) ([]model.GitIntegrationWorkspaceUsage, error) {
	var rows []model.GitIntegrationWorkspaceUsage
	if err := r.db.WithContext(ctx).
		Table("git_repositories gr").
		Select("gr.workspace_id, w.name AS workspace_name, COUNT(*) AS repo_count").
		Joins("JOIN workspaces w ON w.id = gr.workspace_id").
		Where("gr.integration_id = ? AND gr.deleted_at IS NULL", integrationID).
		Group("gr.workspace_id, w.name").
		Order("w.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list affected workspaces by integration: %w", err)
	}
	return rows, nil
}

// TaskDeliveryTargetRepository handles current delivery target state for tasks.
type TaskDeliveryTargetRepository struct {
	db *gorm.DB
}

// NewTaskDeliveryTargetRepository creates a new TaskDeliveryTargetRepository.
func NewTaskDeliveryTargetRepository(db *gorm.DB) *TaskDeliveryTargetRepository {
	return &TaskDeliveryTargetRepository{db: db}
}

// GetByTask returns the delivery target for a task.
func (r *TaskDeliveryTargetRepository) GetByTask(ctx context.Context, workspaceID, storyID string) (*model.TaskDeliveryTarget, error) {
	var target model.TaskDeliveryTarget
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ?", workspaceID, storyID).
		First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get story delivery target: %w", err)
	}
	return &target, nil
}

// GetByID returns a delivery target by ID.
func (r *TaskDeliveryTargetRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.TaskDeliveryTarget, error) {
	var target model.TaskDeliveryTarget
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get story delivery target: %w", err)
	}
	return &target, nil
}

// Save persists a delivery target, upserting on the task_id unique index.
func (r *TaskDeliveryTargetRepository) Save(ctx context.Context, target *model.TaskDeliveryTarget) error {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"repository_id", "repo_full_name", "integration_id",
				"base_branch", "working_branch", "delivery_state",
				"active_pr_number", "active_pr_title", "active_pr_url", "active_pr_status",
				"last_commit_sha", "last_run_id", "last_synced_at", "updated_at",
			}),
		}).
		Create(target).Error; err != nil {
		return fmt.Errorf("save story delivery target: %w", err)
	}
	return nil
}

// TaskGitLinkRepository handles DB operations for task git links.
type TaskGitLinkRepository struct {
	db *gorm.DB
}

// NewTaskGitLinkRepository creates a new TaskGitLinkRepository.
func NewTaskGitLinkRepository(db *gorm.DB) *TaskGitLinkRepository {
	return &TaskGitLinkRepository{db: db}
}

// ListByTask returns git links for a task.
func (r *TaskGitLinkRepository) ListByTask(ctx context.Context, workspaceID, storyID string) ([]model.TaskGitLink, error) {
	var links []model.TaskGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND task_id = ?", workspaceID, storyID).Order("created_at DESC").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story git links: %w", err)
	}
	return links, nil
}

// ListOpenPullRequests returns task links that still believe their PR is open.
func (r *TaskGitLinkRepository) ListOpenPullRequests(ctx context.Context, limit int) ([]model.TaskGitLink, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var links []model.TaskGitLink
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND pr_number IS NOT NULL AND COALESCE(pr_status, 'open') = ?", "github", "open").
		Order("updated_at ASC").
		Limit(limit).
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list open pull request git links: %w", err)
	}
	return links, nil
}

// GetByBranch returns a link by repo+branch.
func (r *TaskGitLinkRepository) GetByBranch(ctx context.Context, workspaceID, repo, branch string) (*model.TaskGitLink, error) {
	var link model.TaskGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND repo = ? AND branch = ?", workspaceID, repo, branch).First(&link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get story git link by branch: %w", err)
	}
	return &link, nil
}

// GetByPR returns a link by repo+PR number.
func (r *TaskGitLinkRepository) GetByPR(ctx context.Context, workspaceID, repo string, prNumber int) (*model.TaskGitLink, error) {
	var link model.TaskGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND repo = ? AND pr_number = ?", workspaceID, repo, prNumber).First(&link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get story git link by PR: %w", err)
	}
	return &link, nil
}

// ListByRepoAndPRs returns links by repo+PR numbers.
func (r *TaskGitLinkRepository) ListByRepoAndPRs(ctx context.Context, workspaceID, repo string, prNumbers []int) ([]model.TaskGitLink, error) {
	if len(prNumbers) == 0 {
		return []model.TaskGitLink{}, nil
	}
	var links []model.TaskGitLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND repo = ? AND pr_number IN ?", workspaceID, repo, prNumbers).
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story git links by PRs: %w", err)
	}
	return links, nil
}

// ListByRepoAndBranches returns links by repo+branch names.
func (r *TaskGitLinkRepository) ListByRepoAndBranches(ctx context.Context, workspaceID, repo string, branches []string) ([]model.TaskGitLink, error) {
	if len(branches) == 0 {
		return []model.TaskGitLink{}, nil
	}
	var links []model.TaskGitLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND repo = ? AND branch IN ?", workspaceID, repo, branches).
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story git links by branches: %w", err)
	}
	return links, nil
}

// ListByRepoAndCommitSHAs returns links by repo+commit SHA.
func (r *TaskGitLinkRepository) ListByRepoAndCommitSHAs(ctx context.Context, workspaceID, repo string, commitSHAs []string) ([]model.TaskGitLink, error) {
	if len(commitSHAs) == 0 {
		return []model.TaskGitLink{}, nil
	}
	var links []model.TaskGitLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND repo = ? AND commit_sha IN ?", workspaceID, repo, commitSHAs).
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story git links by commit shas: %w", err)
	}
	return links, nil
}

// Create creates a new link.
func (r *TaskGitLinkRepository) Create(ctx context.Context, link *model.TaskGitLink) error {
	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("create story git link: %w", err)
	}
	return nil
}

// Update saves a link.
func (r *TaskGitLinkRepository) Update(ctx context.Context, link *model.TaskGitLink) error {
	if err := r.db.WithContext(ctx).Save(link).Error; err != nil {
		return fmt.Errorf("update story git link: %w", err)
	}
	return nil
}

// UpsertByRunAndBranch ensures a historical git link exists for a run/branch tuple.
func (r *TaskGitLinkRepository) UpsertByRunAndBranch(ctx context.Context, link *model.TaskGitLink) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}, {Name: "task_id"}, {Name: "repo"}, {Name: "branch"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"integration_id",
			"repository_id",
			"run_id",
			"provider",
			"pr_number",
			"pr_title",
			"pr_url",
			"pr_status",
			"commit_sha",
			"updated_at",
		}),
	}).Create(link).Error; err != nil {
		return fmt.Errorf("upsert story git link: %w", err)
	}
	return nil
}
