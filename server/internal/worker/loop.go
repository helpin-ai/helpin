package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Worker is the main job processing loop.
type Worker struct {
	workerID      string
	jobRepo       *repository.AgentJobRepository
	runRepo       *repository.AgentRunRepository
	agentRepo     *repository.AgentRepository
	artifactRepo  *repository.AgentRunArtifactRepository
	storyRepo     *repository.PMStoryRepository
	ticketRepo    *repository.SupportTicketRepository
	commentRepo   *repository.PMCommentRepository
	checklistRepo *repository.PMChecklistItemRepository
	messageRepo   *repository.SupportMessageRepository
	gitIntRepo    *repository.GitIntegrationRepository
	gitLinkRepo   *repository.StoryGitLinkRepository
	runtimes      *RuntimeRegistry
	wsPublisher   *websocket.Publisher
	pollInterval  time.Duration
}

// NewWorker creates a new Worker.
func NewWorker(
	workerID string,
	jobRepo *repository.AgentJobRepository,
	runRepo *repository.AgentRunRepository,
	agentRepo *repository.AgentRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	storyRepo *repository.PMStoryRepository,
	ticketRepo *repository.SupportTicketRepository,
	commentRepo *repository.PMCommentRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	messageRepo *repository.SupportMessageRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	gitLinkRepo *repository.StoryGitLinkRepository,
	runtimes *RuntimeRegistry,
	wsPublisher *websocket.Publisher,
) *Worker {
	return &Worker{
		workerID:      workerID,
		jobRepo:       jobRepo,
		runRepo:       runRepo,
		agentRepo:     agentRepo,
		artifactRepo:  artifactRepo,
		storyRepo:     storyRepo,
		ticketRepo:    ticketRepo,
		commentRepo:   commentRepo,
		checklistRepo: checklistRepo,
		messageRepo:   messageRepo,
		gitIntRepo:    gitIntRepo,
		gitLinkRepo:   gitLinkRepo,
		runtimes:      runtimes,
		wsPublisher:   wsPublisher,
		pollInterval:  5 * time.Second,
	}
}

// Run starts the worker poll loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Printf("[worker=%s] starting poll loop (interval=%s)", w.workerID, w.pollInterval)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker=%s] shutting down", w.workerID)
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	job, err := w.jobRepo.ClaimJob(ctx, w.workerID)
	if err != nil {
		log.Printf("[worker=%s] claim error: %v", w.workerID, err)
		return
	}
	if job == nil {
		return // no work available
	}

	log.Printf("[worker=%s] claimed job %s (run=%s, attempt=%d)", w.workerID, job.ID, job.RunID, job.Attempt)
	w.processJob(ctx, job)
}

func (w *Worker) processJob(ctx context.Context, job *model.AgentJob) {
	// Load the run.
	run, err := w.runRepo.GetByID(ctx, job.WorkspaceID, job.RunID)
	if err != nil || run == nil {
		errMsg := "run not found"
		if err != nil {
			errMsg = err.Error()
		}
		_ = w.jobRepo.FailJob(ctx, job.ID, errMsg)
		return
	}

	if run.Status == "cancelled" {
		_ = w.jobRepo.CompleteJob(ctx, job.ID)
		_ = w.markAgentIdle(ctx, run.WorkspaceID, run.AgentID, run.TokensUsed)
		w.publishRunEvent(run, "updated")
		return
	}

	// Mark run as running.
	if run.TargetType == "" {
		switch {
		case run.StoryID != nil:
			run.TargetType = "story"
			run.TargetID = *run.StoryID
		case run.TicketID != nil:
			run.TargetType = "support_ticket"
			run.TargetID = *run.TicketID
		}
	}

	now := time.Now()
	run.Status = "running"
	run.StartedAt = &now
	_ = w.runRepo.Update(ctx, run)

	w.publishRunEvent(run, "updated")

	// Load agent.
	agent, err := w.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil || agent == nil {
		w.failRun(ctx, job, run, "agent not found")
		return
	}
	agent.Status = "working"
	if run.StoryID != nil {
		agent.ActiveStoryID = run.StoryID
	} else {
		agent.ActiveStoryID = nil
	}
	_ = w.agentRepo.Update(ctx, agent)

	// Load story if present.
	var story *model.PMStory
	storyID := ""
	if run.StoryID != nil {
		storyID = *run.StoryID
		story, _ = w.storyRepo.GetRawByID(ctx, storyID)
	}

	var ticket *model.SupportTicket
	ticketID := ""
	if run.TicketID != nil {
		ticketID = *run.TicketID
		ticket, _ = w.ticketRepo.GetByID(ctx, run.WorkspaceID, ticketID)
	}

	// Load git integration if story has a linked branch.
	var gitIntegration *model.GitIntegration
	var repo string
	if storyID != "" {
		links, _ := w.gitLinkRepo.ListByStory(ctx, run.WorkspaceID, storyID)
		if len(links) > 0 {
			link := links[0]
			repo = link.Repo
			gitIntegration, _ = w.gitIntRepo.GetByID(ctx, run.WorkspaceID, link.IntegrationID)
		}
	}

	// Prepare workspace (clone repo if we have git integration).
	workDir, err := w.prepareWorkspace(ctx, gitIntegration, repo)
	if err != nil {
		w.failRun(ctx, job, run, fmt.Sprintf("prepare workspace: %v", err))
		return
	}
	defer os.RemoveAll(workDir)

	// Parse WORKFLOW.md.
	config := ParseWorkflowConfig(workDir)
	if config == nil {
		config = DefaultWorkflowConfig()
	}

	// Build service bridge.
	bridge := &ServiceBridge{
		AddComment: func(sctx context.Context, wsID, sID, aID, content string) error {
			comment := &model.PMComment{
				EntityType: "story",
				EntityID:   sID,
				AuthorID:   aID,
				Body:       content,
			}
			return w.commentRepo.Create(sctx, comment)
		},
		UpdateStoryState: func(sctx context.Context, wsID, sID, stateID string) error {
			s, err := w.storyRepo.GetRawByID(sctx, sID)
			if err != nil || s == nil {
				return fmt.Errorf("story not found")
			}
			s.WorkflowStateID = stateID
			return w.storyRepo.Update(sctx, s)
		},
		ListChecklist: func(sctx context.Context, wsID, sID string) ([]model.PMChecklistItem, error) {
			return w.checklistRepo.List(sctx, sID)
		},
		ListTicketMessages: func(sctx context.Context, wsID, tID string) ([]model.SupportMessage, error) {
			return w.messageRepo.ListByTicket(sctx, wsID, tID, true)
		},
		UpdateTicketStatus: func(sctx context.Context, wsID, tID, status string) error {
			t, err := w.ticketRepo.GetByID(sctx, wsID, tID)
			if err != nil || t == nil {
				return fmt.Errorf("ticket not found")
			}
			t.Status = status
			return w.ticketRepo.Update(sctx, t)
		},
	}

	runtimeProfile := GetRuntimeProfile(agent.CapabilityProfile)

	execCtx := &ExecutionContext{
		Context:        ctx,
		WorkDir:        workDir,
		WorkspaceID:    run.WorkspaceID,
		AgentID:        run.AgentID,
		RunID:          run.ID,
		TargetType:     run.TargetType,
		TargetID:       run.TargetID,
		StoryID:        storyID,
		TicketID:       ticketID,
		Agent:          agent,
		Story:          story,
		Ticket:         ticket,
		GitIntegration: gitIntegration,
		Repo:           repo,
		Config:         config,
		RuntimeProfile: runtimeProfile,
		AllowedTools:   allowedToolSet(runtimeProfile),
		Services:       bridge,
	}

	runtimeKind := run.RuntimeKind
	if runtimeKind == "" {
		runtimeKind = agent.RuntimeKind
	}
	adapter, err := w.runtimes.Get(runtimeKind)
	if err != nil {
		w.failRun(ctx, job, run, err.Error())
		return
	}

	// Execute the tool loop.
	err = adapter.Execute(execCtx, run)

	completedAt := time.Now()
	if err != nil {
		if err == ErrRunCancelled {
			run.Status = "cancelled"
			run.CompletedAt = &completedAt
			_ = w.runRepo.Update(ctx, run)
			_ = w.jobRepo.CompleteJob(ctx, job.ID)
			_ = w.markAgentIdle(ctx, run.WorkspaceID, run.AgentID, run.TokensUsed)
			w.publishRunEvent(run, "updated")
			return
		}
		w.failRun(ctx, job, run, err.Error())
		return
	}

	// Success.
	run.Status = "completed"
	run.CompletedAt = &completedAt
	run.OutputSummary = json.RawMessage(`{"status":"success"}`)
	_ = w.runRepo.Update(ctx, run)
	_ = w.jobRepo.CompleteJob(ctx, job.ID)

	// Update agent status.
	_ = w.markAgentIdle(ctx, run.WorkspaceID, run.AgentID, run.TokensUsed)

	w.publishRunEvent(run, "completed")
	log.Printf("[worker=%s] run %s completed (tokens=%d)", w.workerID, run.ID[:8], run.TokensUsed)
}

func (w *Worker) failRun(ctx context.Context, job *model.AgentJob, run *model.AgentRun, errMsg string) {
	log.Printf("[worker=%s] run %s failed: %s", w.workerID, run.ID[:8], errMsg)

	now := time.Now()
	run.Status = "failed"
	run.CompletedAt = &now
	run.ErrorMessage = &errMsg
	_ = w.runRepo.Update(ctx, run)
	_ = w.jobRepo.FailJob(ctx, job.ID, errMsg)
	if agent, err := w.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID); err == nil && agent != nil {
		agent.Status = "error"
		agent.ActiveStoryID = nil
		_ = w.agentRepo.Update(ctx, agent)
	}

	w.publishRunEvent(run, "updated")
}

func (w *Worker) publishRunEvent(run *model.AgentRun, action string) {
	if w.wsPublisher == nil {
		return
	}
	event := websocket.Event{
		Action:      action,
		Entity:      "agent_run",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
	}
	if run.TargetType != "" && run.TargetID != "" {
		event.ParentType = run.TargetType
		event.ParentID = run.TargetID
	} else if run.StoryID != nil {
		event.ParentType = "story"
		event.ParentID = *run.StoryID
	}
	w.wsPublisher.Publish(event)
}

func (w *Worker) markAgentIdle(ctx context.Context, workspaceID, agentID string, tokens int) error {
	agent, err := w.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "idle"
	agent.ActiveStoryID = nil
	agent.TokensUsedThisMonth += tokens
	return w.agentRepo.Update(ctx, agent)
}

func (w *Worker) prepareWorkspace(ctx context.Context, gitIntegration *model.GitIntegration, repo string) (string, error) {
	workDir, err := os.MkdirTemp("", "agent-workspace-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	if gitIntegration == nil || repo == "" {
		return workDir, nil
	}

	// Build clone URL.
	var cloneURL string
	switch gitIntegration.Provider {
	case "github":
		baseURL := "https://github.com"
		if gitIntegration.BaseURL != nil && *gitIntegration.BaseURL != "" {
			baseURL = *gitIntegration.BaseURL
		}
		cloneURL = fmt.Sprintf("%s/%s.git", baseURL, repo)
	case "gitlab":
		baseURL := "https://gitlab.com"
		if gitIntegration.BaseURL != nil && *gitIntegration.BaseURL != "" {
			baseURL = *gitIntegration.BaseURL
		}
		cloneURL = fmt.Sprintf("%s/%s.git", baseURL, repo)
	default:
		return workDir, nil
	}

	// Clone with token-based auth.
	authURL := cloneURL
	if gitIntegration.AccessToken != "" {
		switch gitIntegration.Provider {
		case "github":
			authURL = fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", gitIntegration.AccessToken, repo)
		case "gitlab":
			authURL = fmt.Sprintf("https://oauth2:%s@gitlab.com/%s.git", gitIntegration.AccessToken, repo)
		}
		if gitIntegration.BaseURL != nil && *gitIntegration.BaseURL != "" {
			// For self-hosted, just use the clone URL with basic auth header.
			authURL = cloneURL
		}
	}

	cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cloneDir := filepath.Join(workDir, "repo")
	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", authURL, cloneDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("git clone failed: %s", string(output))
	}

	return cloneDir, nil
}
