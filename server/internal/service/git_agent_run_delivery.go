package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// AgentRunRepositoryDelivery carries the pushed-branch result reported by the
// agent runtime for a delegated run (the OutputSummary "repository" contract
// written by the runtime's push_branch finalize policy).
type AgentRunRepositoryDelivery struct {
	Branch    string
	CommitSHA string
}

// AgentRunRepositoryDeliveryResult describes the pull/merge request ensured
// for a delegated run's pushed working branch.
type AgentRunRepositoryDeliveryResult struct {
	Provider string
	Number   int
	Title    string
	URL      string
}

// FinalizeDelegatedRunDelivery ensures a pull/merge request for a delegated
// run whose working branch the agent runtime already pushed, then records it
// on the task/epic delivery target and the task git link. It is the API-only
// service-layer port of temporalapp recordPushAndEnsureDeliveryPR: branch
// creation and base sync happened runtime-side, so only provider API calls
// and bookkeeping remain. Naturally idempotent — the provider clients reuse
// an existing open PR/MR for the same head/base pair instead of creating a
// duplicate. Returns (nil, nil) when the run resolves to nothing deliverable
// (no branch, branch equals base, or a provider without PR semantics).
func (s *GitService) FinalizeDelegatedRunDelivery(ctx context.Context, run *model.AgentRun, delivery AgentRunRepositoryDelivery) (*AgentRunRepositoryDeliveryResult, error) {
	if s == nil || run == nil {
		return nil, nil
	}
	taskTarget, epicTarget, err := s.delegatedRunDeliveryTargets(ctx, run)
	if err != nil {
		return nil, err
	}
	repo, integration, err := s.delegatedRunRepository(ctx, run, taskTarget, epicTarget)
	if err != nil {
		return nil, err
	}
	head, base := delegatedRunDeliveryBranches(run, taskTarget, epicTarget, repo, delivery)
	if head == "" || head == base {
		return nil, nil
	}
	title, body := s.delegatedRunPullRequestContent(ctx, run, head, base)
	result, err := s.ensureDelegatedRunPullRequest(ctx, integration, repo, head, base, title, body)
	if err != nil {
		s.markDelegatedTaskDeliveryPRFailed(ctx, run, taskTarget)
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	if err := s.recordDelegatedRunDelivery(ctx, run, taskTarget, epicTarget, repo, integration, head, strings.TrimSpace(delivery.CommitSHA), result); err != nil {
		return nil, fmt.Errorf("persist delegated run delivery: %w", err)
	}
	return result, nil
}

// delegatedRunDeliveryTargets loads the delivery-state record for the run
// target, if the target type has one. Missing records are not an error: the
// pull request can still be ensured from the run's own repository fields.
func (s *GitService) delegatedRunDeliveryTargets(ctx context.Context, run *model.AgentRun) (*model.TaskDeliveryTarget, *model.EpicDeliveryTarget, error) {
	switch run.TargetType {
	case "task", "story":
		taskID := delegatedRunTaskID(run)
		if taskID == "" || s.deliveryRepo == nil {
			return nil, nil, nil
		}
		target, err := s.deliveryRepo.GetByTask(ctx, run.WorkspaceID, taskID)
		if err != nil {
			return nil, nil, fmt.Errorf("load task delivery target: %w", err)
		}
		return target, nil, nil
	case "epic":
		if s.epicDeliveryRepo == nil || strings.TrimSpace(run.TargetID) == "" {
			return nil, nil, nil
		}
		target, err := s.epicDeliveryRepo.GetByEpic(ctx, run.WorkspaceID, strings.TrimSpace(run.TargetID))
		if err != nil {
			return nil, nil, fmt.Errorf("load epic delivery target: %w", err)
		}
		return nil, target, nil
	default:
		return nil, nil, nil
	}
}

// delegatedRunRepository resolves the enabled repository and active
// integration the runtime pushed to, from run fields first and the delivery
// target records as fallback.
func (s *GitService) delegatedRunRepository(ctx context.Context, run *model.AgentRun, taskTarget *model.TaskDeliveryTarget, epicTarget *model.EpicDeliveryTarget) (*model.GitRepository, *model.GitIntegration, error) {
	repositoryID := strings.TrimSpace(derefString(run.RepositoryID))
	if repositoryID == "" && taskTarget != nil {
		repositoryID = strings.TrimSpace(derefString(taskTarget.RepositoryID))
	}
	if repositoryID == "" && epicTarget != nil {
		repositoryID = strings.TrimSpace(derefString(epicTarget.RepositoryID))
	}
	if repositoryID == "" && run.TargetType == "repository" {
		repositoryID = strings.TrimSpace(run.TargetID)
	}
	if repositoryID == "" {
		return nil, nil, fmt.Errorf("delegated run %q has no repository to deliver to", run.ID)
	}
	repo, err := s.repoRepo.GetEnabledByID(ctx, run.WorkspaceID, repositoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("load repository %q: %w", repositoryID, err)
	}
	if repo == nil {
		return nil, nil, fmt.Errorf("repository %q is not available", repositoryID)
	}
	integration, err := s.integrationRepo.GetByID(ctx, repo.WorkspaceID, repo.IntegrationID)
	if err != nil {
		return nil, nil, fmt.Errorf("load git integration %q: %w", repo.IntegrationID, err)
	}
	if integration == nil || !integration.Active {
		return nil, nil, fmt.Errorf("git integration is not available")
	}
	return repo, integration, nil
}

// delegatedRunDeliveryBranches resolves the pushed head branch and the base
// branch a pull request should target, preferring the runtime-reported branch.
func delegatedRunDeliveryBranches(run *model.AgentRun, taskTarget *model.TaskDeliveryTarget, epicTarget *model.EpicDeliveryTarget, repo *model.GitRepository, delivery AgentRunRepositoryDelivery) (string, string) {
	headCandidates := []string{delivery.Branch, derefString(run.WorkingBranch)}
	baseCandidates := []string{derefString(run.BaseBranch)}
	if taskTarget != nil {
		headCandidates = append(headCandidates, derefString(taskTarget.WorkingBranch))
		baseCandidates = append(baseCandidates, derefString(taskTarget.BaseBranch))
	}
	if epicTarget != nil {
		headCandidates = append(headCandidates, derefString(epicTarget.EpicBranch))
		baseCandidates = append(baseCandidates, derefString(epicTarget.BaseBranch))
	}
	baseCandidates = append(baseCandidates, repo.DefaultBranch, "main")
	head := strings.TrimSpace(firstNonEmptyString(headCandidates...))
	base := strings.TrimSpace(firstNonEmptyString(baseCandidates...))
	return head, base
}

// ensureDelegatedRunPullRequest creates or reuses the open PR/MR for
// head -> base via the provider API clients already wired into the service.
func (s *GitService) ensureDelegatedRunPullRequest(ctx context.Context, integration *model.GitIntegration, repo *model.GitRepository, head, base, title, body string) (*AgentRunRepositoryDeliveryResult, error) {
	switch strings.ToLower(strings.TrimSpace(integration.Provider)) {
	case "github":
		if s.githubApp == nil {
			return nil, fmt.Errorf("github app is not configured")
		}
		if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
			return nil, fmt.Errorf("git integration has no installation ID")
		}
		owner, repoName, ok := splitRepoFullName(strings.TrimSpace(repo.FullName))
		if !ok {
			return nil, fmt.Errorf("invalid repo full name: %s", repo.FullName)
		}
		pr, err := s.githubApp.EnsurePullRequest(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, githubapp.EnsurePullRequestInput{
			Head:  head,
			Base:  base,
			Title: title,
			Body:  body,
		})
		if err != nil {
			return nil, fmt.Errorf("ensure github pull request: %w", err)
		}
		if pr == nil {
			return nil, fmt.Errorf("github pull request response is empty")
		}
		return &AgentRunRepositoryDeliveryResult{
			Provider: "github",
			Number:   pr.Number,
			Title:    strings.TrimSpace(pr.Title),
			URL:      strings.TrimSpace(pr.HTMLURL),
		}, nil
	case "gitlab":
		return s.ensureDelegatedRunMergeRequest(ctx, integration, repo, head, base, title, body)
	default:
		return nil, nil
	}
}

// ensureDelegatedRunMergeRequest reuses an open GitLab MR for the branch pair
// or creates one, mirroring temporalapp ensureGitLabMergeRequest through the
// service gitLabClient instead of raw HTTP.
func (s *GitService) ensureDelegatedRunMergeRequest(ctx context.Context, integration *model.GitIntegration, repo *model.GitRepository, head, base, title, body string) (*AgentRunRepositoryDeliveryResult, error) {
	projectID, err := strconv.ParseInt(strings.TrimSpace(repo.ExternalID), 10, 64)
	if err != nil || projectID == 0 {
		return nil, fmt.Errorf("invalid gitlab project id %q", repo.ExternalID)
	}
	token, err := s.gitlabAccessToken(ctx, integration)
	if err != nil {
		return nil, fmt.Errorf("resolve gitlab access token: %w", err)
	}
	client := s.gitLabClientFor(derefString(integration.BaseURL))
	existing, err := client.ListMergeRequests(ctx, token, projectID, head, base)
	if err != nil {
		return nil, fmt.Errorf("list gitlab merge requests: %w", err)
	}
	if len(existing) > 0 {
		mr := existing[0]
		return &AgentRunRepositoryDeliveryResult{
			Provider: "gitlab",
			Number:   mr.IID,
			Title:    strings.TrimSpace(mr.Title),
			URL:      strings.TrimSpace(mr.WebURL),
		}, nil
	}
	created, err := client.CreateMergeRequest(ctx, token, projectID, head, base, title, body)
	if err != nil {
		return nil, fmt.Errorf("create gitlab merge request: %w", err)
	}
	if created == nil {
		return nil, fmt.Errorf("gitlab merge request response is empty")
	}
	return &AgentRunRepositoryDeliveryResult{
		Provider: "gitlab",
		Number:   created.IID,
		Title:    strings.TrimSpace(created.Title),
		URL:      strings.TrimSpace(created.WebURL),
	}, nil
}

// recordDelegatedRunDelivery persists the ensured PR on the delivery target
// and task git link (temporalapp recordPR + upsertGitLink equivalents) and
// publishes the realtime events that refresh delivery UI.
func (s *GitService) recordDelegatedRunDelivery(ctx context.Context, run *model.AgentRun, taskTarget *model.TaskDeliveryTarget, epicTarget *model.EpicDeliveryTarget, repo *model.GitRepository, integration *model.GitIntegration, head, commitSHA string, result *AgentRunRepositoryDeliveryResult) error {
	now := time.Now()
	openStatus := "open"
	if taskTarget != nil {
		taskTarget.ActivePRNumber = &result.Number
		taskTarget.ActivePRTitle = &result.Title
		taskTarget.ActivePRURL = &result.URL
		taskTarget.ActivePRStatus = &openStatus
		taskTarget.WorkingBranch = &head
		if commitSHA != "" {
			taskTarget.LastCommitSHA = &commitSHA
		}
		taskTarget.LastRunID = &run.ID
		taskTarget.DeliveryState = "pr_open"
		taskTarget.LastSyncedAt = &now
		if err := s.deliveryRepo.Save(ctx, taskTarget); err != nil {
			return fmt.Errorf("save task delivery target: %w", err)
		}
		if err := s.upsertDelegatedRunGitLink(ctx, run, taskTarget, repo, integration, head, commitSHA, result); err != nil {
			return err
		}
		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, run.WorkspaceID, "task", taskTarget.TaskID, nil, "updated", strPtr("delivery_target"), nil, &result.URL, nil)
		}
		if s.wsPublisher != nil {
			s.wsPublisher.Publish(websocket.Event{
				Action:      "updated",
				Entity:      "task_delivery_target",
				EntityID:    taskTarget.ID,
				WorkspaceID: run.WorkspaceID,
				ParentType:  "task",
				ParentID:    taskTarget.TaskID,
			})
		}
	}
	if epicTarget != nil {
		epicTarget.FinalPRNumber = &result.Number
		epicTarget.FinalPRTitle = &result.Title
		epicTarget.FinalPRURL = &result.URL
		epicTarget.FinalPRStatus = &openStatus
		if commitSHA != "" {
			epicTarget.LastCommitSHA = &commitSHA
		}
		epicTarget.LastRunID = &run.ID
		epicTarget.DeliveryState = "pr_open"
		epicTarget.LastSyncedAt = &now
		if err := s.epicDeliveryRepo.Save(ctx, epicTarget); err != nil {
			return fmt.Errorf("save epic delivery target: %w", err)
		}
		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, run.WorkspaceID, "epic", epicTarget.EpicID, nil, "updated", strPtr("delivery_target"), nil, &result.URL, nil)
		}
		s.publishEpicDeliveryTargetUpdated(run.WorkspaceID, epicTarget.EpicID, "", epicTarget)
	}
	return nil
}

// upsertDelegatedRunGitLink mirrors temporalapp upsertGitLink for the pushed
// branch: create the task git link if missing, otherwise refresh it with the
// run, commit, and PR data.
func (s *GitService) upsertDelegatedRunGitLink(ctx context.Context, run *model.AgentRun, taskTarget *model.TaskDeliveryTarget, repo *model.GitRepository, integration *model.GitIntegration, head, commitSHA string, result *AgentRunRepositoryDeliveryResult) error {
	if s.linkRepo == nil {
		return nil
	}
	link, err := s.linkRepo.GetByBranch(ctx, run.WorkspaceID, repo.FullName, head)
	if err != nil {
		return fmt.Errorf("lookup task git link: %w", err)
	}
	openStatus := "open"
	if link == nil {
		link = &model.TaskGitLink{
			WorkspaceID:   run.WorkspaceID,
			TaskID:        taskTarget.TaskID,
			IntegrationID: integration.ID,
			RepositoryID:  &repo.ID,
			RunID:         &run.ID,
			Provider:      integration.Provider,
			BaseURL:       integration.BaseURL,
			Repo:          repo.FullName,
			Branch:        &head,
			PRNumber:      &result.Number,
			PRTitle:       &result.Title,
			PRURL:         &result.URL,
			PRStatus:      &openStatus,
		}
		if commitSHA != "" {
			link.CommitSHA = &commitSHA
		}
		if err := s.linkRepo.Create(ctx, link); err != nil {
			return fmt.Errorf("create task git link: %w", err)
		}
		return nil
	}
	link.RepositoryID = &repo.ID
	link.RunID = &run.ID
	link.Provider = integration.Provider
	link.BaseURL = integration.BaseURL
	link.IntegrationID = integration.ID
	if commitSHA != "" {
		link.CommitSHA = &commitSHA
	}
	link.PRNumber = &result.Number
	link.PRTitle = &result.Title
	link.PRURL = &result.URL
	link.PRStatus = &openStatus
	if err := s.linkRepo.Update(ctx, link); err != nil {
		return fmt.Errorf("update task git link: %w", err)
	}
	return nil
}

// markDelegatedTaskDeliveryPRFailed mirrors the temporalapp pr_failed
// bookkeeping when the provider call fails, so the delivery UI surfaces the
// failure while the finalizer retry path stays open.
func (s *GitService) markDelegatedTaskDeliveryPRFailed(ctx context.Context, run *model.AgentRun, taskTarget *model.TaskDeliveryTarget) {
	if taskTarget == nil || s.deliveryRepo == nil {
		return
	}
	now := time.Now()
	taskTarget.DeliveryState = "pr_failed"
	taskTarget.LastRunID = &run.ID
	taskTarget.LastSyncedAt = &now
	if err := s.deliveryRepo.Save(ctx, taskTarget); err != nil {
		slog.WarnContext(ctx, "failed to persist pr_failed delivery state for delegated run",
			"error", err,
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"task_id", taskTarget.TaskID,
		)
	}
}

// delegatedRunPullRequestContent ports temporalapp
// buildDeliveryPullRequestContent using service repositories: task key + name
// (or epic name) for the title, and a context block for the body.
func (s *GitService) delegatedRunPullRequestContent(ctx context.Context, run *model.AgentRun, head, base string) (string, string) {
	title := ""
	taskLine := ""
	switch run.TargetType {
	case "task", "story":
		title, taskLine = s.delegatedRunTaskTitle(ctx, run)
	case "epic":
		if s.epicRepo != nil {
			if epicWithStats, err := s.epicRepo.GetByID(ctx, strings.TrimSpace(run.TargetID)); err == nil && epicWithStats != nil {
				if name := strings.TrimSpace(epicWithStats.Epic.Name); name != "" {
					title = "Merge epic: " + name
				}
			}
		}
	}
	if title == "" {
		title = fmt.Sprintf("Automated changes from %s", head)
	}

	lines := []string{
		"Automated pull request opened by Helpin.",
		"",
		"## Context",
	}
	if taskLine != "" {
		lines = append(lines, taskLine)
	}
	if head != "" {
		lines = append(lines, "- Branch: `"+head+"`")
	}
	if base != "" {
		lines = append(lines, "- Base branch: `"+base+"`")
	}
	if strings.TrimSpace(run.ID) != "" {
		lines = append(lines, "- Run ID: `"+run.ID+"`")
	}
	return title, strings.Join(lines, "\n")
}

// delegatedRunTaskTitle builds "KEY-123: Name" for a task/story run when the
// task and workspace key are resolvable, falling back gracefully.
func (s *GitService) delegatedRunTaskTitle(ctx context.Context, run *model.AgentRun) (string, string) {
	taskID := delegatedRunTaskID(run)
	if taskID == "" || s.taskRepo == nil {
		return "", ""
	}
	task, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil || task == nil {
		return "", ""
	}
	taskName := strings.TrimSpace(task.Name)
	taskKey := ""
	if s.workspaceRepo != nil && task.DisplayID > 0 {
		if workspace, err := s.workspaceRepo.GetByID(ctx, run.WorkspaceID); err == nil && workspace != nil && strings.TrimSpace(workspace.WorkspaceKey) != "" {
			taskKey = strings.TrimSpace(model.FormatTaskKey(workspace.WorkspaceKey, task.DisplayID))
		}
	}
	title := taskName
	switch {
	case taskKey != "" && title != "":
		title = fmt.Sprintf("%s: %s", taskKey, title)
	case taskKey != "":
		title = taskKey
	}
	taskLine := ""
	switch {
	case taskKey != "" && taskName != "":
		taskLine = fmt.Sprintf("- Task: %s — %s", taskKey, taskName)
	case taskKey != "":
		taskLine = "- Task: " + taskKey
	case taskName != "":
		taskLine = "- Task: " + taskName
	}
	return title, taskLine
}

// delegatedRunTaskID resolves the task a task/story run targets.
func delegatedRunTaskID(run *model.AgentRun) string {
	if taskID := strings.TrimSpace(run.TargetID); taskID != "" {
		return taskID
	}
	return strings.TrimSpace(derefString(run.TaskID))
}
