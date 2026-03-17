package temporalapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/worker"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
)

// PlanningTurnFunc executes a streaming agent turn within a planning session.
// It receives the session ID and an ExecutionContext with a cloned repo WorkDir.
type PlanningTurnFunc func(ctx context.Context, sessionID string, execCtx *worker.ExecutionContext) error

// PlanningFinalizeTurnFunc executes the finalization turn and writes the spec.
type PlanningFinalizeTurnFunc func(ctx context.Context, sessionID, actorID string, execCtx *worker.ExecutionContext) error

// PlanningSessionActivities contains the Temporal activities for interactive planning sessions.
// These run in the shared temporal-worker process and publish stream events through
// JetStream so the API websocket hub can relay them to browsers.
type PlanningSessionActivities struct {
	sessionRepo *repository.PlanningSessionRepository
	epicRepo    *repository.PMEpicRepository
	gitIntRepo  *repository.GitIntegrationRepository
	gitRepo     *repository.GitRepositoryRepository
	githubApp   *githubapp.Client
	runTurn     PlanningTurnFunc
	finalize    PlanningFinalizeTurnFunc
}

// NewPlanningSessionActivities creates the activity set for planning sessions.
func NewPlanningSessionActivities(
	sessionRepo *repository.PlanningSessionRepository,
	epicRepo *repository.PMEpicRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	gitRepo *repository.GitRepositoryRepository,
	githubApp *githubapp.Client,
	runTurn PlanningTurnFunc,
	finalize PlanningFinalizeTurnFunc,
) *PlanningSessionActivities {
	return &PlanningSessionActivities{
		sessionRepo: sessionRepo,
		epicRepo:    epicRepo,
		gitIntRepo:  gitIntRepo,
		gitRepo:     gitRepo,
		githubApp:   githubApp,
		runTurn:     runTurn,
		finalize:    finalize,
	}
}

// PrepareWorkspaceActivity clones the planning repository into a temp directory.
func (a *PlanningSessionActivities) PrepareWorkspaceActivity(ctx context.Context, sessionID string) (string, error) {
	session, err := a.sessionRepo.GetByID(ctx, sessionID)
	if err != nil || session == nil {
		return "", fmt.Errorf("planning session not found: %s", sessionID)
	}

	epicWithStats, err := a.epicRepo.GetByID(ctx, session.EpicID)
	if err != nil || epicWithStats == nil {
		return "", fmt.Errorf("epic not found for session: %s", sessionID)
	}
	epic := &epicWithStats.Epic

	// If no planning repository is configured, create an empty workspace.
	if epic.PlanningRepositoryID == nil || *epic.PlanningRepositoryID == "" {
		workDir, err := os.MkdirTemp("", "planning-session-*")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}
		slog.Info("planning session: workspace prepared (no repo)", "session_id", sessionID, "work_dir", workDir)
		return workDir, nil
	}

	// Resolve git integration and repository.
	repo, err := a.gitRepo.GetByID(ctx, epic.WorkspaceID, *epic.PlanningRepositoryID)
	if err != nil || repo == nil {
		return "", fmt.Errorf("planning repository not found")
	}
	integration, err := a.gitIntRepo.GetByID(ctx, epic.WorkspaceID, repo.IntegrationID)
	if err != nil || integration == nil {
		return "", fmt.Errorf("git integration not found for planning repository")
	}

	// Mint access token.
	token, err := a.mintAccessToken(ctx, integration)
	if err != nil {
		return "", fmt.Errorf("git access token: %w", err)
	}

	// Clone repo.
	workDir, err := worker.PrepareWorkspace(ctx, integration, repo.FullName, token)
	if err != nil {
		return "", fmt.Errorf("clone planning repository: %w", err)
	}

	slog.Info("planning session: workspace prepared", "session_id", sessionID, "repo", repo.FullName, "work_dir", workDir)
	return workDir, nil
}

// RunTurnActivity executes a single agent turn (streaming Claude call with tools).
func (a *PlanningSessionActivities) RunTurnActivity(ctx context.Context, sessionID, workDir string) error {
	execCtx := &worker.ExecutionContext{
		Context: ctx,
		WorkDir: workDir,
	}
	return a.runTurn(ctx, sessionID, execCtx)
}

// FinalizeTurnActivity runs the finalization turn and writes the spec.
func (a *PlanningSessionActivities) FinalizeTurnActivity(ctx context.Context, sessionID, workDir, actorID string) error {
	execCtx := &worker.ExecutionContext{
		Context: ctx,
		WorkDir: workDir,
	}
	return a.finalize(ctx, sessionID, actorID, execCtx)
}

// CleanupWorkspaceActivity removes the temporary workspace directory.
func (a *PlanningSessionActivities) CleanupWorkspaceActivity(ctx context.Context, workDir string) error {
	if workDir == "" {
		return nil
	}
	slog.Info("planning session: cleaning up workspace", "work_dir", workDir)
	return os.RemoveAll(workDir)
}

func (a *PlanningSessionActivities) mintAccessToken(ctx context.Context, integration *model.GitIntegration) (string, error) {
	if integration == nil {
		return "", nil
	}
	if integration.CredentialMode == "github_app" && integration.InstallationID != nil && *integration.InstallationID != "" {
		if a.githubApp == nil {
			return "", fmt.Errorf("github app credentials are not configured")
		}
		return a.githubApp.MintInstallationToken(ctx, *integration.InstallationID)
	}
	if integration.AccessToken != "" {
		return integration.AccessToken, nil
	}
	return "", fmt.Errorf("git integration has no usable credentials")
}
