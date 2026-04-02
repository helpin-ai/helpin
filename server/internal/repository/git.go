package repository

import (
	"context"
	"fmt"
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

// List returns all integrations for a workspace.
func (r *GitIntegrationRepository) List(ctx context.Context, workspaceID string) ([]model.GitIntegration, error) {
	var integrations []model.GitIntegration
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").Find(&integrations).Error; err != nil {
		return nil, fmt.Errorf("list git integrations: %w", err)
	}
	return integrations, nil
}

// GetByID returns a single integration.
func (r *GitIntegrationRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.GitIntegration, error) {
	var integration model.GitIntegration
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git integration: %w", err)
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

// GetByInstallationID returns the active integration for an app installation.
func (r *GitIntegrationRepository) GetByInstallationID(ctx context.Context, provider, installationID string) (*model.GitIntegration, error) {
	var integration model.GitIntegration
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND active = ?", provider, installationID, true).
		First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get integration by installation id: %w", err)
	}
	return &integration, nil
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
		Where("workspace_id = ? AND archived = ? AND selected = ?", workspaceID, false, true).
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
		Where("workspace_id = ?", workspaceID).
		Order("archived ASC, selected DESC, full_name ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("list all git repositories: %w", err)
	}
	return repos, nil
}

// GetByID loads a repository by ID.
func (r *GitRepositoryRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.GitRepository, error) {
	var repo model.GitRepository
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git repository: %w", err)
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

// UpsertMany replaces the synced repository set for an integration.
func (r *GitRepositoryRepository) UpsertMany(ctx context.Context, workspaceID, integrationID string, repos []model.GitRepository) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.GitRepository{}).
			Where("workspace_id = ? AND integration_id = ?", workspaceID, integrationID).
			Update("archived", true).Error; err != nil {
			return fmt.Errorf("archive previous repos: %w", err)
		}

		for i := range repos {
			repos[i].WorkspaceID = workspaceID
			repos[i].IntegrationID = integrationID
			repos[i].Archived = false
			repos[i].UpdatedAt = now

			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "workspace_id"},
					{Name: "integration_id"},
					{Name: "external_id"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"provider",
					"full_name",
					"default_branch",
					"permissions",
					"private",
					"archived",
					"selected",
					"updated_at",
				}),
			}).Create(&repos[i]).Error; err != nil {
				return fmt.Errorf("upsert git repository: %w", err)
			}
		}
		return nil
	})
}

// TaskDeliveryTargetRepository handles current delivery target state for tasks.
type TaskDeliveryTargetRepository struct {
	db *gorm.DB
}

// NewTaskDeliveryTargetRepository creates a new TaskDeliveryTargetRepository.
func NewTaskDeliveryTargetRepository(db *gorm.DB) *TaskDeliveryTargetRepository {
	return &TaskDeliveryTargetRepository{db: db}
}

// GetByStory returns the delivery target for a task.
func (r *TaskDeliveryTargetRepository) GetByStory(ctx context.Context, workspaceID, storyID string) (*model.TaskDeliveryTarget, error) {
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

// ListByStory returns git links for a task.
func (r *TaskGitLinkRepository) ListByStory(ctx context.Context, workspaceID, storyID string) ([]model.TaskGitLink, error) {
	var links []model.TaskGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND task_id = ?", workspaceID, storyID).Order("created_at DESC").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story git links: %w", err)
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
