package temporalapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/worker"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
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

const (
	planningSessionLookupTimeout  = 3 * time.Second
	planningSessionLookupInterval = 100 * time.Millisecond
)

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
	session, err := loadPlanningSessionWithRetry(ctx, sessionID, planningSessionLookupTimeout, planningSessionLookupInterval, a.sessionRepo.GetByID)
	if err != nil {
		return "", fmt.Errorf("load planning session %s: %w", sessionID, err)
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
	execCtx := newPlanningExecutionContext(ctx, workDir, worker.PlanningTurnKindMessage, 1)
	return a.runTurn(ctx, sessionID, execCtx)
}

// RunInitialTurnActivity executes the initial automatic turn with watchdog-aware metadata.
func (a *PlanningSessionActivities) RunInitialTurnActivity(ctx context.Context, sessionID, workDir string, attempt int) error {
	execCtx := newPlanningExecutionContext(ctx, workDir, worker.PlanningTurnKindInitial, attempt)
	if err := a.runTurn(ctx, sessionID, execCtx); err != nil {
		if errors.Is(err, worker.ErrInitialResponseTimeout) {
			return temporal.NewNonRetryableApplicationError(worker.ErrInitialResponseTimeout.Error(), worker.ErrInitialResponseTimeout.Error(), err)
		}
		return err
	}
	return nil
}

// FinalizeTurnActivity runs the finalization turn and writes the spec.
func (a *PlanningSessionActivities) FinalizeTurnActivity(ctx context.Context, sessionID, workDir, actorID string) error {
	execCtx := newPlanningExecutionContext(ctx, workDir, worker.PlanningTurnKindMessage, 1)
	return a.finalize(ctx, sessionID, actorID, execCtx)
}

// MarkSessionAbandonedActivity marks a planning session as abandoned (e.g. after a fatal initial turn failure).
// The flow workflow's reconciliation timer will detect the abandoned session and cancel the flow.
func (a *PlanningSessionActivities) MarkSessionAbandonedActivity(ctx context.Context, sessionID string) error {
	session, err := a.sessionRepo.GetByID(ctx, sessionID)
	if err != nil || session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if session.Status != model.PlanningSessionStatusCompleted {
		now := time.Now()
		session.Status = model.PlanningSessionStatusAbandoned
		if session.CompletedAt == nil {
			session.CompletedAt = &now
		}
		if err := a.sessionRepo.Update(ctx, session); err != nil {
			return err
		}
	}

	if epicWithStats, err := a.epicRepo.GetByID(ctx, session.EpicID); err == nil && epicWithStats != nil {
		epic := &epicWithStats.Epic
		epic.PlanningState = model.EpicPlanningStateNotStarted
		if epic.ActivePlanningSessionID != nil && *epic.ActivePlanningSessionID == session.ID {
			epic.ActivePlanningSessionID = nil
		}
		_ = a.epicRepo.Update(ctx, epic)
	}

	slog.Info("planning session: marked as abandoned after initial turn failure", "session_id", sessionID)
	return nil
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

func newPlanningExecutionContext(ctx context.Context, workDir, turnKind string, attempt int) *worker.ExecutionContext {
	return &worker.ExecutionContext{
		Context:             ctx,
		WorkDir:             workDir,
		PlanningTurnKind:    turnKind,
		PlanningTurnAttempt: attempt,
		Heartbeat: func(stage string) error {
			activity.RecordHeartbeat(ctx, stage)
			return nil
		},
	}
}

func loadPlanningSessionWithRetry(
	ctx context.Context,
	sessionID string,
	timeout time.Duration,
	interval time.Duration,
	lookup func(context.Context, string) (*model.PlanningSession, error),
) (*model.PlanningSession, error) {
	if timeout <= 0 {
		timeout = planningSessionLookupTimeout
	}
	if interval <= 0 {
		interval = planningSessionLookupInterval
	}
	deadline := time.Now().Add(timeout)
	attempt := 1
	var lastErr error

	for {
		session, err := lookup(ctx, sessionID)
		if err == nil && session != nil {
			return session, nil
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("planning session not found")
		}

		if time.Now().After(deadline) {
			return nil, lastErr
		}

		slog.Warn("planning session activity lookup retrying",
			"session_id", sessionID,
			"attempt", attempt,
			"error", err,
		)
		attempt++

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}
