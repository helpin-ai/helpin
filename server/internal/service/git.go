package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// GitService contains git integration business logic.
type GitService struct {
	integrationRepo *repository.GitIntegrationRepository
	linkRepo        *repository.StoryGitLinkRepository
	activitySvc     *PMActivityService
	wsPublisher     *websocket.Publisher
}

// NewGitService creates a new GitService.
func NewGitService(
	integrationRepo *repository.GitIntegrationRepository,
	linkRepo *repository.StoryGitLinkRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *GitService {
	return &GitService{
		integrationRepo: integrationRepo,
		linkRepo:        linkRepo,
		activitySvc:     activitySvc,
		wsPublisher:     wsPublisher,
	}
}

// ListIntegrations returns all git integrations for a workspace.
func (s *GitService) ListIntegrations(ctx context.Context, workspaceID string) ([]model.GitIntegration, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.integrationRepo.List(ctx, workspaceID)
}

// CreateIntegration creates a new git integration.
func (s *GitService) CreateIntegration(ctx context.Context, req model.CreateGitIntegrationRequest, actorID string) (*model.GitIntegration, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.DisplayName) == "" {
		return nil, fmt.Errorf("workspace_id and display_name are required")
	}
	if req.Provider != "github" && req.Provider != "gitlab" {
		return nil, fmt.Errorf("provider must be 'github' or 'gitlab'")
	}
	if req.AccessToken == "" {
		return nil, fmt.Errorf("access_token is required")
	}

	integration := &model.GitIntegration{
		WorkspaceID:    req.WorkspaceID,
		Provider:       req.Provider,
		DisplayName:    strings.TrimSpace(req.DisplayName),
		BaseURL:        req.BaseURL,
		InstallationID: req.InstallationID,
		AccessToken:    req.AccessToken,
		Active:         true,
	}

	if err := s.integrationRepo.Create(ctx, integration); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, integration.WorkspaceID, "git_integration", integration.ID, &actorID, "created", nil, nil, &integration.DisplayName, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "git_integration",
		EntityID:    integration.ID,
		WorkspaceID: integration.WorkspaceID,
		ActorID:     actorID,
	})

	return integration, nil
}

// GetStoryGitLinks returns git links for a story.
func (s *GitService) GetStoryGitLinks(ctx context.Context, workspaceID, storyID string) ([]model.StoryGitLink, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.linkRepo.ListByStory(ctx, workspaceID, storyID)
}

// CreateBranch creates a branch link for a story (records the intent; actual branch creation would call git provider API).
func (s *GitService) CreateBranch(ctx context.Context, workspaceID, storyID string, req model.CreateBranchRequest, actorID string) (*model.StoryGitLink, error) {
	if strings.TrimSpace(req.BranchName) == "" || req.Repo == "" || req.IntegrationID == "" {
		return nil, fmt.Errorf("integration_id, repo, and branch_name are required")
	}

	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, req.IntegrationID)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("git integration not found")
	}

	branchName := strings.TrimSpace(req.BranchName)
	link := &model.StoryGitLink{
		WorkspaceID:   workspaceID,
		StoryID:       storyID,
		IntegrationID: req.IntegrationID,
		Provider:      integration.Provider,
		Repo:          req.Repo,
		Branch:        &branchName,
	}

	if err := s.linkRepo.Create(ctx, link); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "story", storyID, &actorID, "updated", strPtr("branch"), nil, &branchName, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "story_git_link",
		EntityID:    link.ID,
		WorkspaceID: workspaceID,
		ParentType:  "story",
		ParentID:    storyID,
		ActorID:     actorID,
	})

	return link, nil
}

// ProcessWebhookPush handles a push event from a git provider.
func (s *GitService) ProcessWebhookPush(ctx context.Context, workspaceID, repo, branch, commitSHA string) error {
	link, err := s.linkRepo.GetByBranch(ctx, workspaceID, repo, branch)
	if err != nil {
		return err
	}
	if link == nil {
		return nil // no linked story, ignore
	}

	link.CommitSHA = &commitSHA
	if err := s.linkRepo.Update(ctx, link); err != nil {
		return err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "story_git_link",
		EntityID:    link.ID,
		WorkspaceID: workspaceID,
		ParentType:  "story",
		ParentID:    link.StoryID,
	})

	return nil
}

// ProcessWebhookPR handles a PR event from a git provider.
func (s *GitService) ProcessWebhookPR(ctx context.Context, workspaceID, repo string, prNumber int, prURL, prStatus, branch string) error {
	// Try to find by PR number first, then by branch.
	link, err := s.linkRepo.GetByPR(ctx, workspaceID, repo, prNumber)
	if err != nil {
		return err
	}
	if link == nil && branch != "" {
		link, err = s.linkRepo.GetByBranch(ctx, workspaceID, repo, branch)
		if err != nil {
			return err
		}
	}
	if link == nil {
		return nil // no linked story, ignore
	}

	link.PRNumber = &prNumber
	link.PRURL = &prURL
	link.PRStatus = &prStatus

	if err := s.linkRepo.Update(ctx, link); err != nil {
		return err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "story_git_link",
		EntityID:    link.ID,
		WorkspaceID: workspaceID,
		ParentType:  "story",
		ParentID:    link.StoryID,
	})

	return nil
}
