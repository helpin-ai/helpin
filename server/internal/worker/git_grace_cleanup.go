package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	defaultGitGraceCleanupInterval = 24 * time.Hour
	defaultGitGraceRetention       = 30 * 24 * time.Hour
)

// GitGraceCleanup removes expired git repo and integration tombstones.
type GitGraceCleanup struct {
	integrationRepo *repository.GitIntegrationRepository
	repoRepo        *repository.GitRepositoryRepository
	logger          *slog.Logger
	now             func() time.Time
	retention       time.Duration
}

func NewGitGraceCleanup(
	integrationRepo *repository.GitIntegrationRepository,
	repoRepo *repository.GitRepositoryRepository,
) *GitGraceCleanup {
	return &GitGraceCleanup{
		integrationRepo: integrationRepo,
		repoRepo:        repoRepo,
		logger:          slog.Default().With("worker", "git_grace_cleanup"),
		now:             time.Now,
		retention:       defaultGitGraceRetention,
	}
}

func (c *GitGraceCleanup) Start(ctx context.Context) {
	if c == nil || c.integrationRepo == nil || c.repoRepo == nil {
		return
	}

	c.runOnce(ctx)

	ticker := time.NewTicker(defaultGitGraceCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *GitGraceCleanup) runOnce(ctx context.Context) {
	cutoff := c.now().UTC().Add(-c.retention)

	repos, err := c.repoRepo.ListSoftDeletedBefore(ctx, cutoff)
	if err != nil {
		c.logger.ErrorContext(ctx, "list expired git repo tombstones", "error", err)
		return
	}
	deletedRepos := 0
	for _, repo := range repos {
		if err := c.repoRepo.HardDeleteByID(ctx, repo.ID); err != nil {
			c.logger.WarnContext(ctx, "delete expired git repo tombstone",
				"error", err,
				"repo_id", repo.ID,
				"workspace_id", repo.WorkspaceID,
				"integration_id", repo.IntegrationID,
			)
			continue
		}
		deletedRepos++
	}

	integrations, err := c.integrationRepo.ListSoftDeletedBefore(ctx, cutoff)
	if err != nil {
		c.logger.ErrorContext(ctx, "list expired git integration tombstones", "error", err)
		return
	}
	deletedIntegrations := 0
	for _, integration := range integrations {
		deleted, err := c.integrationRepo.HardDeleteByIDIfNoRepositories(ctx, integration.ID)
		if err != nil {
			c.logger.WarnContext(ctx, "delete expired git integration tombstone",
				"error", err,
				"integration_id", integration.ID,
				"organization_id", integration.OrganizationID,
			)
			continue
		}
		if deleted {
			deletedIntegrations++
		}
	}

	if deletedRepos > 0 || deletedIntegrations > 0 {
		c.logger.InfoContext(ctx, "completed git grace cleanup",
			"deleted_repositories", deletedRepos,
			"deleted_integrations", deletedIntegrations,
			"cutoff", cutoff,
		)
	}
}
