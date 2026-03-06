package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/model"
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

// StoryGitLinkRepository handles DB operations for story git links.
type StoryGitLinkRepository struct {
	db *gorm.DB
}

// NewStoryGitLinkRepository creates a new StoryGitLinkRepository.
func NewStoryGitLinkRepository(db *gorm.DB) *StoryGitLinkRepository {
	return &StoryGitLinkRepository{db: db}
}

// ListByStory returns git links for a story.
func (r *StoryGitLinkRepository) ListByStory(ctx context.Context, workspaceID, storyID string) ([]model.StoryGitLink, error) {
	var links []model.StoryGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND story_id = ?", workspaceID, storyID).Order("created_at DESC").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list story git links: %w", err)
	}
	return links, nil
}

// GetByBranch returns a link by repo+branch.
func (r *StoryGitLinkRepository) GetByBranch(ctx context.Context, workspaceID, repo, branch string) (*model.StoryGitLink, error) {
	var link model.StoryGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND repo = ? AND branch = ?", workspaceID, repo, branch).First(&link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get story git link by branch: %w", err)
	}
	return &link, nil
}

// GetByPR returns a link by repo+PR number.
func (r *StoryGitLinkRepository) GetByPR(ctx context.Context, workspaceID, repo string, prNumber int) (*model.StoryGitLink, error) {
	var link model.StoryGitLink
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND repo = ? AND pr_number = ?", workspaceID, repo, prNumber).First(&link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get story git link by PR: %w", err)
	}
	return &link, nil
}

// Create creates a new link.
func (r *StoryGitLinkRepository) Create(ctx context.Context, link *model.StoryGitLink) error {
	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("create story git link: %w", err)
	}
	return nil
}

// Update saves a link.
func (r *StoryGitLinkRepository) Update(ctx context.Context, link *model.StoryGitLink) error {
	if err := r.db.WithContext(ctx).Save(link).Error; err != nil {
		return fmt.Errorf("update story git link: %w", err)
	}
	return nil
}
