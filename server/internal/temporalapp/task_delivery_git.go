package temporalapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) prepareTaskDelivery(ctx context.Context, state *resolvedRunState) error {
	target := state.deliveryTarget
	if target == nil {
		return fmt.Errorf("task delivery target is missing")
	}
	if state.repository == nil || state.integration == nil {
		if state.resolved.RequiresRepo {
			return fmt.Errorf("task has no delivery target configured")
		}
		return nil
	}

	if target.RepoFullName == nil {
		target.RepoFullName = &state.repository.FullName
	}
	if target.IntegrationID == nil {
		target.IntegrationID = &state.repository.IntegrationID
	}
	effectiveBaseBranch := strings.TrimSpace(derefString(target.BaseBranch))
	if requestedBaseBranch := strings.TrimSpace(derefString(state.run.BaseBranch)); requestedBaseBranch != "" {
		effectiveBaseBranch = requestedBaseBranch
	} else if effectiveBaseBranch == "" {
		effectiveBaseBranch = defaultString(state.repository.DefaultBranch, "main")
		target.BaseBranch = &effectiveBaseBranch
	}

	effectiveWorkingBranch := strings.TrimSpace(derefString(target.WorkingBranch))
	if requestedWorkingBranch := strings.TrimSpace(derefString(state.run.WorkingBranch)); requestedWorkingBranch != "" {
		effectiveWorkingBranch = requestedWorkingBranch
	} else if state.resolved.RequiresRepo && effectiveWorkingBranch == "" {
		effectiveWorkingBranch = buildWorkingBranch(state.task, state.teamDefault, state.workspaceKey)
		target.WorkingBranch = &effectiveWorkingBranch
	}

	if state.resolved.RequiresRepo && state.accessToken == "" {
		return fmt.Errorf("repository access token is not available")
	}

	if state.resolved.RequiresRepo && effectiveWorkingBranch != "" {
		if err := a.ensureRemoteBranch(ctx, state.integration, state.accessToken, state.repository.FullName, effectiveBaseBranch, effectiveWorkingBranch); err != nil {
			return err
		}
	}

	target.LastRunID = &state.run.ID
	if target.RepositoryID != nil {
		state.run.RepositoryID = target.RepositoryID
	}
	state.run.RepoFullName = target.RepoFullName
	if strings.TrimSpace(effectiveBaseBranch) != "" {
		state.run.BaseBranch = &effectiveBaseBranch
	} else {
		state.run.BaseBranch = nil
	}
	if strings.TrimSpace(effectiveWorkingBranch) != "" {
		state.run.WorkingBranch = &effectiveWorkingBranch
	} else {
		state.run.WorkingBranch = nil
	}
	state.run.DeliveryTargetID = &target.ID
	if state.run.TaskQueue == nil || *state.run.TaskQueue == "" {
		queue := state.resolved.Queue
		state.run.TaskQueue = &queue
		state.run.RunnerPool = &queue
	}
	target.DeliveryState = "in_progress"
	if err := a.deliveryRepo.Save(ctx, target); err != nil {
		return err
	}

	return a.upsertGitLink(ctx, state, "", nil)
}

func (a *AgentRunActivities) checkoutRunRef(ctx context.Context, workDir string, state *resolvedRunState) error {
	baseBranch := derefString(state.run.BaseBranch)
	workingBranch := derefString(state.run.WorkingBranch)
	if workingBranch != "" {
		if refName, err := a.fetchRemoteTrackingBranch(ctx, workDir, state.integration, state.accessToken, workingBranch); err == nil {
			if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, refName); err == nil {
				return nil
			}
		}
	}
	if baseBranch == "" {
		baseBranch = defaultString(state.repository.DefaultBranch, "main")
	}
	baseRefName, err := a.fetchRemoteTrackingBranch(ctx, workDir, state.integration, state.accessToken, baseBranch)
	if err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if workingBranch != "" {
		if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, baseRefName); err != nil {
			return fmt.Errorf("checkout working branch: %w", err)
		}
		return nil
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", baseBranch, baseRefName); err != nil {
		return fmt.Errorf("checkout base branch: %w", err)
	}
	return nil
}

func (a *AgentRunActivities) syncBaseIntoWorkingBranch(ctx context.Context, workDir string, state *resolvedRunState) error {
	if state == nil {
		return nil
	}
	baseBranch := strings.TrimSpace(derefString(state.run.BaseBranch))
	workingBranch := strings.TrimSpace(derefString(state.run.WorkingBranch))
	state.branchSync = branchSyncState{
		Status:        "not_applicable",
		BaseBranch:    baseBranch,
		WorkingBranch: workingBranch,
	}
	if baseBranch == "" || workingBranch == "" {
		return nil
	}
	if baseBranch == workingBranch {
		state.branchSync.Status = "same_branch"
		return nil
	}

	baseRefName, err := a.fetchRemoteTrackingBranch(ctx, workDir, state.integration, state.accessToken, baseBranch)
	if err != nil {
		return fmt.Errorf("fetch base branch for sync: %w", err)
	}
	needsMerge, err := a.workingBranchNeedsBaseSync(ctx, workDir, state.integration, state.accessToken, baseRefName, baseBranch, workingBranch)
	if err != nil {
		if errors.Is(err, errBranchSyncUnrelatedHistory) {
			if err := a.recoverUnrelatedWorkingBranch(ctx, workDir, state, baseRefName, baseBranch, workingBranch); err != nil {
				state.branchSync.Status = "unrelated_history"
				return err
			}
			return nil
		}
		return fmt.Errorf("check base sync status: %w", err)
	}
	if !needsMerge {
		state.branchSync.Status = "up_to_date"
		return nil
	}

	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "merge", "--no-ff", "--no-edit", baseRefName); err == nil {
		state.branchSync.Status = "merged"
		return nil
	} else {
		conflictFiles, conflictErr := a.gitMergeConflictFiles(ctx, workDir, state.integration, state.accessToken)
		if conflictErr == nil && len(conflictFiles) > 0 {
			state.branchSync.Status = "conflicted"
			state.branchSync.ConflictFiles = conflictFiles
			if executionRuntimeKind(state) == "codex" {
				slog.WarnContext(ctx, "agent run branch sync produced merge conflicts; handing off to codex",
					"workspace_id", state.run.WorkspaceID,
					"run_id", state.run.ID,
					"base_branch", baseBranch,
					"working_branch", workingBranch,
					"conflict_files", conflictFiles,
				)
				return nil
			}
			_, _ = a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "merge", "--abort")
			return fmt.Errorf("sync base branch into working branch: merge conflicts in %s", strings.Join(conflictFiles, ", "))
		}
		_, _ = a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "merge", "--abort")
		return fmt.Errorf("sync base branch into working branch: %w", err)
	}
}

func (a *AgentRunActivities) ensureRemoteBranch(ctx context.Context, integration *model.GitIntegration, accessToken, repoFullName, baseBranch, workingBranch string) error {
	workDir, err := workerpkg.PrepareWorkspace(ctx, integration, repoFullName, accessToken)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	baseRefName, err := a.fetchRemoteTrackingBranch(ctx, workDir, integration, accessToken, baseBranch)
	if err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if _, err := a.fetchRemoteTrackingBranch(ctx, workDir, integration, accessToken, workingBranch); err == nil {
		return nil
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "checkout", "-B", workingBranch, baseRefName); err != nil {
		return fmt.Errorf("create working branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "push", "-u", "origin", workingBranch); err != nil {
		return fmt.Errorf("push working branch: %w", err)
	}
	return nil
}

func (a *AgentRunActivities) fetchRemoteTrackingBranch(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
	branch string,
) (string, error) {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return "", fmt.Errorf("branch is required")
	}
	refName := "refs/remotes/origin/" + trimmed
	refspec := fmt.Sprintf("refs/heads/%s:%s", trimmed, refName)
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, "fetch", "origin", refspec); err != nil {
		return "", err
	}
	return "origin/" + trimmed, nil
}

func (a *AgentRunActivities) workingBranchNeedsBaseSync(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
	baseRefName string,
	baseBranch string,
	workingBranch string,
) (bool, error) {
	mergeBaseOutput, err := a.runGitInDir(ctx, workDir, integration, accessToken, "merge-base", "HEAD", strings.TrimSpace(baseRefName))
	if mergeBaseHash := extractMergeBaseHash(mergeBaseOutput, errorText(err)); mergeBaseHash != "" {
		mergeBaseOutput = mergeBaseHash
		err = nil
	}
	if err != nil {
		if strings.TrimSpace(mergeBaseOutput) == "" {
			retried, retryErr := a.retryMergeBaseAfterFetchingHistory(ctx, workDir, integration, accessToken, baseRefName, baseBranch, workingBranch)
			if retryErr == nil {
				mergeBaseOutput = retried
				err = nil
			} else if mergeBaseHash := extractMergeBaseHash(retried, errorText(retryErr)); mergeBaseHash != "" {
				mergeBaseOutput = mergeBaseHash
				err = nil
			}
		}
		if mergeBaseHash := extractMergeBaseHash(mergeBaseOutput, errorText(err)); mergeBaseHash != "" {
			mergeBaseOutput = mergeBaseHash
			err = nil
		}
	}
	if err != nil {
		if strings.TrimSpace(mergeBaseOutput) == "" {
			return false, fmt.Errorf("%w", errBranchSyncUnrelatedHistory)
		}
		detail := strings.TrimSpace(mergeBaseOutput)
		if detail == "" {
			detail = strings.TrimSpace(err.Error())
		}
		return false, fmt.Errorf("determine merge base: %s", detail)
	}
	output, err := a.runGitInDir(ctx, workDir, integration, accessToken, "rev-list", "--count", "HEAD.."+strings.TrimSpace(baseRefName))
	if err != nil {
		return false, err
	}
	count, parseErr := strconv.Atoi(strings.TrimSpace(output))
	if parseErr != nil {
		return false, fmt.Errorf("parse rev-list count: %w", parseErr)
	}
	return count > 0, nil
}

func (a *AgentRunActivities) retryMergeBaseAfterFetchingHistory(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
	baseRefName, baseBranch, workingBranch string,
) (string, error) {
	isShallowOutput, err := a.runGitInDir(ctx, workDir, integration, accessToken, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(isShallowOutput) != "true" {
		return "", fmt.Errorf("repository is not shallow")
	}

	trimmedBase := strings.TrimSpace(baseBranch)
	trimmedWorking := strings.TrimSpace(workingBranch)
	refspecs := make([]string, 0, 2)
	if trimmedBase != "" {
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", trimmedBase, trimmedBase))
	}
	if trimmedWorking != "" && trimmedWorking != trimmedBase {
		refspecs = append(refspecs, fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", trimmedWorking, trimmedWorking))
	}

	args := []string{"fetch", "--update-shallow", "--unshallow", "origin"}
	args = append(args, refspecs...)
	if _, err := a.runGitInDir(ctx, workDir, integration, accessToken, args...); err != nil {
		return "", err
	}

	mergeBaseOutput, err := a.runGitInDir(ctx, workDir, integration, accessToken, "merge-base", "HEAD", strings.TrimSpace(baseRefName))
	if mergeBaseHash := extractMergeBaseHash(mergeBaseOutput, errorText(err)); mergeBaseHash != "" {
		return mergeBaseHash, nil
	}
	if err != nil {
		return mergeBaseOutput, err
	}
	return strings.TrimSpace(mergeBaseOutput), nil
}

func extractMergeBaseHash(texts ...string) string {
	for _, text := range texts {
		fields := strings.Fields(strings.TrimSpace(text))
		for _, field := range fields {
			candidate := strings.TrimSpace(field)
			if len(candidate) < 7 || len(candidate) > 40 {
				continue
			}
			valid := true
			for _, r := range candidate {
				switch {
				case r >= '0' && r <= '9':
				case r >= 'a' && r <= 'f':
				case r >= 'A' && r <= 'F':
				default:
					valid = false
					break
				}
			}
			if valid {
				return candidate
			}
		}
	}
	return ""
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *AgentRunActivities) recoverUnrelatedWorkingBranch(
	ctx context.Context,
	workDir string,
	state *resolvedRunState,
	baseRefName, baseBranch, workingBranch string,
) error {
	if state == nil {
		return fmt.Errorf("sync base branch into working branch: working branch %q does not share history with base branch %q", workingBranch, baseBranch)
	}
	if state.deliveryTarget != nil && state.deliveryTarget.ActivePRNumber != nil {
		state.branchSync.Status = "unrelated_history"
		return fmt.Errorf("sync base branch into working branch: working branch %q does not share history with base branch %q and has an active pull request", workingBranch, baseBranch)
	}

	headSHAOutput, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("sync base branch into working branch: resolve unrelated branch head: %w", err)
	}
	headSHA := strings.TrimSpace(headSHAOutput)
	backupBranch := buildUnrelatedHistoryBackupBranch(headSHA)

	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "branch", "-f", backupBranch, "HEAD"); err != nil {
		return fmt.Errorf("sync base branch into working branch: create backup branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "push", "-u", "origin", backupBranch); err != nil {
		return fmt.Errorf("sync base branch into working branch: push backup branch: %w", err)
	}
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "checkout", "-B", workingBranch, baseRefName); err != nil {
		return fmt.Errorf("sync base branch into working branch: recreate working branch from base: %w", err)
	}
	leaseRef := fmt.Sprintf("--force-with-lease=refs/heads/%s:%s", workingBranch, headSHA)
	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "push", leaseRef, "-u", "origin", workingBranch); err != nil {
		return fmt.Errorf("sync base branch into working branch: reset remote working branch from base: %w", err)
	}

	state.branchSync.Status = "recreated_from_base"
	state.branchSync.BackupBranch = backupBranch
	slog.WarnContext(ctx, "agent run recovered unrelated working branch by recreating it from base",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"base_branch", baseBranch,
		"working_branch", workingBranch,
		"backup_branch", backupBranch,
	)
	return nil
}

func buildUnrelatedHistoryBackupBranch(headSHA string) string {
	shortSHA := strings.TrimSpace(headSHA)
	if len(shortSHA) > 12 {
		shortSHA = shortSHA[:12]
	}
	if shortSHA == "" {
		shortSHA = "unknown"
	}
	return fmt.Sprintf("helpin-backup/unrelated-history/%s-%s", time.Now().UTC().Format("20060102150405"), shortSHA)
}

func (a *AgentRunActivities) gitMergeConflictFiles(
	ctx context.Context,
	workDir string,
	integration *model.GitIntegration,
	accessToken string,
) ([]string, error) {
	output, err := a.runGitInDir(ctx, workDir, integration, accessToken, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		files = append(files, trimmed)
	}
	slices.Sort(files)
	return files, nil
}

func (a *AgentRunActivities) recordPush(ctx context.Context, state *resolvedRunState, branch, sha string) error {
	if state.deliveryTarget == nil {
		return nil
	}
	state.deliveryTarget.WorkingBranch = &branch
	state.deliveryTarget.LastCommitSHA = &sha
	state.deliveryTarget.LastRunID = &state.run.ID
	state.deliveryTarget.DeliveryState = "pushed"
	now := time.Now()
	state.deliveryTarget.LastSyncedAt = &now
	state.run.WorkingBranch = &branch
	state.run.LastHeartbeatAt = &now
	if err := a.deliveryRepo.Save(ctx, state.deliveryTarget); err != nil {
		return err
	}
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	return a.upsertGitLink(ctx, state, sha, nil)
}

func (a *AgentRunActivities) recordPushAndEnsureDeliveryPR(ctx context.Context, state *resolvedRunState, branch, sha string) error {
	if err := a.recordPush(ctx, state, branch, sha); err != nil {
		return err
	}
	pr, err := a.ensureDeliveryPullRequest(ctx, state, branch)
	if err != nil {
		baseBranch := ""
		if state != nil && state.run != nil {
			baseBranch = strings.TrimSpace(derefString(state.run.BaseBranch))
		}
		if baseBranch == "" && state != nil && state.deliveryTarget != nil {
			baseBranch = strings.TrimSpace(derefString(state.deliveryTarget.BaseBranch))
		}
		slog.WarnContext(ctx, "failed to ensure delivery pull request after push",
			"workspace_id", safeRunWorkspaceID(state),
			"run_id", safeRunID(state),
			"branch", branch,
			"base_branch", baseBranch,
			"error", err,
		)
		if a.artifactRepo != nil && state != nil && state.run != nil {
			_, _ = a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", map[string]any{
				"delivery":    "pr_failed",
				"branch":      branch,
				"base_branch": baseBranch,
				"error":       err.Error(),
			})
		}
		if state != nil && state.deliveryTarget != nil {
			now := time.Now()
			state.deliveryTarget.DeliveryState = "pr_failed"
			state.deliveryTarget.LastRunID = &state.run.ID
			state.deliveryTarget.LastSyncedAt = &now
			if saveErr := a.deliveryRepo.Save(ctx, state.deliveryTarget); saveErr != nil {
				slog.WarnContext(ctx, "failed to persist pr_failed delivery state after pull request error",
					"workspace_id", safeRunWorkspaceID(state),
					"run_id", safeRunID(state),
					"branch", branch,
					"error", saveErr,
				)
			}
		}
		return nil
	}
	if pr == nil {
		return nil
	}
	if err := a.recordPR(ctx, state, pr.Metadata, pr.Title); err != nil {
		if a.artifactRepo != nil && state != nil && state.run != nil {
			_, _ = a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", map[string]any{
				"delivery":    "pr_persistence_failed",
				"created":     !pr.Existing,
				"branch":      branch,
				"base_branch": strings.TrimSpace(pr.Metadata.Base),
				"number":      pr.Metadata.Number,
				"url":         pr.Metadata.URL,
				"title":       pr.Title,
				"error":       err.Error(),
			})
		}
		return fmt.Errorf("persist delivery pull request after upstream create/reuse: %w", err)
	}
	if a.artifactRepo != nil && state != nil && state.run != nil {
		_, _ = a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", map[string]any{
			"delivery":    "pr_open",
			"created":     !pr.Existing,
			"branch":      branch,
			"base_branch": strings.TrimSpace(pr.Metadata.Base),
			"number":      pr.Metadata.Number,
			"url":         pr.Metadata.URL,
			"title":       pr.Title,
		})
	}
	return nil
}

func (a *AgentRunActivities) pushCodexLocalCommit(ctx context.Context, workDir string, state *resolvedRunState, execCtx *workerpkg.ExecutionContext) error {
	if execCtx == nil || execCtx.LocalGitCommit == nil {
		return nil
	}
	if state == nil || state.run == nil {
		return fmt.Errorf("push codex commit: missing run state")
	}

	branch := strings.TrimSpace(execCtx.LocalGitCommit.Branch)
	if branch == "" {
		branch = strings.TrimSpace(execCtx.WorkingBranch)
	}
	if branch == "" {
		return fmt.Errorf("push codex commit: working branch is empty")
	}

	sha := strings.TrimSpace(execCtx.LocalGitCommit.CommitSHA)
	if sha == "" {
		return fmt.Errorf("push codex commit: commit sha is empty")
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("pushing_changes")
	}

	if _, err := a.runGitInDir(ctx, workDir, state.integration, state.accessToken, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push repository changes: %w", err)
	}
	if err := a.recordPushAndEnsureDeliveryPR(ctx, state, branch, sha); err != nil {
		return fmt.Errorf("record pushed branch: %w", err)
	}

	if a.artifactRepo != nil && state != nil && state.run != nil {
		payload := map[string]any{
			"branch":              branch,
			"commit_sha":          sha,
			"commit_message":      strings.TrimSpace(execCtx.LocalGitCommit.CommitMessage),
			"changed_files":       slices.Clone(execCtx.LocalGitCommit.ChangedFiles),
			"commit_author_name":  strings.TrimSpace(state.gitIdentity.Name),
			"commit_author_email": strings.TrimSpace(state.gitIdentity.Email),
			"delivery":            "pushed",
		}
		if _, err := a.appendRunArtifact(ctx, state.run, "git_delivery_result", "json", payload); err != nil {
			slog.WarnContext(ctx, "failed to save git delivery result artifact",
				"error", err,
				"run_id", state.run.ID,
				"branch", branch,
			)
		}
	}

	return nil
}

func resolveRunGitIdentity(state *resolvedRunState) (workerpkg.GitIdentity, error) {
	var configured workerpkg.GitIdentity
	if state != nil && state.integration != nil {
		configured.Name = derefString(state.integration.DefaultCommitAuthorName)
		configured.Email = derefString(state.integration.DefaultCommitAuthorEmail)
	}
	resolved := workerpkg.ResolveGitIdentity(configured)
	if state == nil || state.integration == nil || state.repository == nil {
		return resolved, nil
	}
	if strings.EqualFold(strings.TrimSpace(state.integration.Provider), "gitlab") &&
		runMayCreateGitCommits(state) &&
		strings.TrimSpace(configured.Email) == "" {
		return resolved, fmt.Errorf("GitLab commit author email is required for this integration")
	}
	return resolved, nil
}

func runMayCreateGitCommits(state *resolvedRunState) bool {
	if state == nil || state.agent == nil {
		return false
	}
	runtimeKind := strings.TrimSpace(executionRuntimeKind(state))
	if runtimeKind != "codex" && runtimeKind != "opencode" {
		return false
	}
	preset := strings.TrimSpace(state.agent.EffectivePresetKey())
	if preset == "" {
		preset = strings.TrimSpace(state.agent.SourcePresetKey)
	}
	return preset == model.AgentPresetCodeBuilder
}

func (a *AgentRunActivities) ensureDeliveryPullRequest(ctx context.Context, state *resolvedRunState, workingBranch string) (*ensuredDeliveryPR, error) {
	if state == nil || state.run == nil || state.task == nil || state.repository == nil || state.integration == nil || state.deliveryTarget == nil {
		return nil, nil
	}
	provider := strings.TrimSpace(state.integration.Provider)
	if provider != "github" && provider != "gitlab" {
		return nil, nil
	}

	repoFullName := strings.TrimSpace(state.repository.FullName)
	if repoFullName == "" {
		repoFullName = strings.TrimSpace(derefString(state.deliveryTarget.RepoFullName))
	}
	workingBranch = strings.TrimSpace(workingBranch)
	if workingBranch == "" {
		workingBranch = strings.TrimSpace(derefString(state.deliveryTarget.WorkingBranch))
	}
	if workingBranch == "" {
		workingBranch = strings.TrimSpace(derefString(state.run.WorkingBranch))
	}
	baseBranch := strings.TrimSpace(derefString(state.run.BaseBranch))
	if baseBranch == "" {
		baseBranch = strings.TrimSpace(derefString(state.deliveryTarget.BaseBranch))
	}
	if baseBranch == "" {
		baseBranch = defaultString(state.repository.DefaultBranch, "main")
	}
	if repoFullName == "" || workingBranch == "" || baseBranch == "" || workingBranch == baseBranch {
		return nil, nil
	}
	if strings.TrimSpace(state.accessToken) == "" {
		return nil, fmt.Errorf("repository access token is not available for pull request creation")
	}

	title, body := buildDeliveryPullRequestContent(state, baseBranch, workingBranch)
	var pr *ensuredDeliveryPR
	var err error
	if provider == "gitlab" {
		pr, err = ensureGitLabMergeRequest(ctx, state.integration, state.accessToken, state.repository.ExternalID, workingBranch, baseBranch, title, body)
	} else {
		pr, err = ensureGitHubPullRequest(ctx, state.integration, state.accessToken, repoFullName, workingBranch, baseBranch, title, body)
	}
	if err != nil {
		return nil, err
	}
	return pr, nil
}

type ensuredDeliveryPR struct {
	Metadata workerpkg.PRMetadata
	Title    string
	Existing bool
}

func ensureGitHubPullRequest(
	ctx context.Context,
	integration *model.GitIntegration,
	accessToken, repoFullName, workingBranch, baseBranch, title, body string,
) (*ensuredDeliveryPR, error) {
	if integration == nil || strings.TrimSpace(integration.Provider) != "github" {
		return nil, nil
	}
	repoFullName = strings.TrimSpace(repoFullName)
	workingBranch = strings.TrimSpace(workingBranch)
	baseBranch = strings.TrimSpace(baseBranch)
	accessToken = strings.TrimSpace(accessToken)
	if repoFullName == "" || workingBranch == "" || baseBranch == "" {
		return nil, nil
	}
	if accessToken == "" {
		return nil, fmt.Errorf("github access token is required")
	}

	owner, repo, err := splitRepoFullName(repoFullName)
	if err != nil {
		return nil, err
	}
	apiBase := model.ResolveGitHubAPIBaseURL(integration.BaseURL)

	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", owner+":"+workingBranch)
	query.Set("base", baseBranch)
	query.Set("per_page", "1")

	var existingPayload []struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := doGitHubAPIRequest(ctx, accessToken, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/pulls?%s", apiBase, owner, repo, query.Encode()), nil, &existingPayload); err != nil {
		return nil, err
	}
	if len(existingPayload) > 0 {
		existing := existingPayload[0]
		return &ensuredDeliveryPR{
			Metadata: workerpkg.PRMetadata{
				Provider: "github",
				URL:      strings.TrimSpace(existing.HTMLURL),
				Number:   existing.Number,
				Head:     strings.TrimSpace(existing.Head.Ref),
				Base:     strings.TrimSpace(existing.Base.Ref),
			},
			Title:    strings.TrimSpace(existing.Title),
			Existing: true,
		}, nil
	}

	var createdPayload struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := doGitHubAPIRequest(ctx, accessToken, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/pulls", apiBase, owner, repo), map[string]string{
		"title": title,
		"body":  body,
		"head":  workingBranch,
		"base":  baseBranch,
	}, &createdPayload); err != nil {
		return nil, err
	}
	return &ensuredDeliveryPR{
		Metadata: workerpkg.PRMetadata{
			Provider: "github",
			URL:      strings.TrimSpace(createdPayload.HTMLURL),
			Number:   createdPayload.Number,
			Head:     strings.TrimSpace(createdPayload.Head.Ref),
			Base:     strings.TrimSpace(createdPayload.Base.Ref),
		},
		Title:    strings.TrimSpace(createdPayload.Title),
		Existing: false,
	}, nil
}

func ensureGitLabMergeRequest(
	ctx context.Context,
	integration *model.GitIntegration,
	accessToken, externalProjectID, workingBranch, baseBranch, title, body string,
) (*ensuredDeliveryPR, error) {
	if integration == nil || strings.TrimSpace(integration.Provider) != "gitlab" {
		return nil, nil
	}
	projectID, err := strconv.ParseInt(strings.TrimSpace(externalProjectID), 10, 64)
	if err != nil || projectID == 0 {
		return nil, fmt.Errorf("invalid gitlab project id")
	}
	workingBranch = strings.TrimSpace(workingBranch)
	baseBranch = strings.TrimSpace(baseBranch)
	accessToken = strings.TrimSpace(accessToken)
	if workingBranch == "" || baseBranch == "" {
		return nil, nil
	}
	if accessToken == "" {
		return nil, fmt.Errorf("gitlab access token is required")
	}
	apiBase := model.ResolveGitLabAPIBaseURL(integration.BaseURL)
	query := url.Values{}
	query.Set("state", "opened")
	query.Set("source_branch", workingBranch)
	query.Set("target_branch", baseBranch)
	query.Set("per_page", "1")

	var existingPayload []struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		WebURL       string `json:"web_url"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}
	if err := doGitLabAPIRequest(ctx, accessToken, http.MethodGet, fmt.Sprintf("%s/projects/%d/merge_requests?%s", apiBase, projectID, query.Encode()), nil, &existingPayload); err != nil {
		return nil, err
	}
	if len(existingPayload) > 0 {
		existing := existingPayload[0]
		return &ensuredDeliveryPR{
			Metadata: workerpkg.PRMetadata{
				Provider: "gitlab",
				URL:      strings.TrimSpace(existing.WebURL),
				Number:   existing.IID,
				Head:     strings.TrimSpace(existing.SourceBranch),
				Base:     strings.TrimSpace(existing.TargetBranch),
			},
			Title:    strings.TrimSpace(existing.Title),
			Existing: true,
		}, nil
	}

	var createdPayload struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		WebURL       string `json:"web_url"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}
	if err := doGitLabAPIRequest(ctx, accessToken, http.MethodPost, fmt.Sprintf("%s/projects/%d/merge_requests", apiBase, projectID), map[string]string{
		"title":         title,
		"description":   body,
		"source_branch": workingBranch,
		"target_branch": baseBranch,
	}, &createdPayload); err != nil {
		return nil, err
	}
	return &ensuredDeliveryPR{
		Metadata: workerpkg.PRMetadata{
			Provider: "gitlab",
			URL:      strings.TrimSpace(createdPayload.WebURL),
			Number:   createdPayload.IID,
			Head:     strings.TrimSpace(createdPayload.SourceBranch),
			Base:     strings.TrimSpace(createdPayload.TargetBranch),
		},
		Title:    strings.TrimSpace(createdPayload.Title),
		Existing: false,
	}, nil
}

func doGitHubAPIRequest(ctx context.Context, accessToken, method, requestURL string, payload any, out any) error {
	var requestBody []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal github request payload: %w", err)
		}
		requestBody = data
	}
	reqCtx, cancel := context.WithTimeout(ctx, githubAPIRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("Accept", "application/vnd.github+json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errorPayload struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errorPayload)
		return fmt.Errorf("github api failed (%d): %s", resp.StatusCode, strings.TrimSpace(errorPayload.Message))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func doGitLabAPIRequest(ctx context.Context, accessToken, method, requestURL string, payload any, out any) error {
	var requestBody []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal gitlab request payload: %w", err)
		}
		requestBody = data
	}
	reqCtx, cancel := context.WithTimeout(ctx, githubAPIRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("build gitlab request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request gitlab api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var errorPayload struct {
			Message any    `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errorPayload)
		return fmt.Errorf("gitlab api failed (%d): %v %s", resp.StatusCode, errorPayload.Message, strings.TrimSpace(errorPayload.Error))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode gitlab response: %w", err)
	}
	return nil
}

func buildDeliveryPullRequestContent(state *resolvedRunState, baseBranch, workingBranch string) (string, string) {
	taskName := ""
	taskKey := ""
	if state != nil && state.task != nil {
		taskName = strings.TrimSpace(state.task.Name)
		if state.workspaceKey != "" && state.task.DisplayID > 0 {
			taskKey = strings.TrimSpace(model.FormatTaskKey(state.workspaceKey, state.task.DisplayID))
		}
	}

	title := strings.TrimSpace(taskName)
	switch {
	case taskKey != "" && title != "":
		title = fmt.Sprintf("%s: %s", taskKey, title)
	case taskKey != "":
		title = taskKey
	case title == "":
		title = fmt.Sprintf("Automated changes from %s", workingBranch)
	}

	lines := []string{
		"Automated pull request opened by Helpin.",
		"",
		"## Context",
	}
	if taskKey != "" || taskName != "" {
		taskLine := "- Task: "
		switch {
		case taskKey != "" && taskName != "":
			taskLine += fmt.Sprintf("%s — %s", taskKey, taskName)
		case taskKey != "":
			taskLine += taskKey
		default:
			taskLine += taskName
		}
		lines = append(lines, taskLine)
	}
	if state != nil && state.agent != nil && strings.TrimSpace(state.agent.Name) != "" {
		lines = append(lines, "- Agent: "+strings.TrimSpace(state.agent.Name))
	}
	if strings.TrimSpace(workingBranch) != "" {
		lines = append(lines, "- Branch: `"+strings.TrimSpace(workingBranch)+"`")
	}
	if strings.TrimSpace(baseBranch) != "" {
		lines = append(lines, "- Base branch: `"+strings.TrimSpace(baseBranch)+"`")
	}
	if state != nil && state.run != nil && strings.TrimSpace(state.run.ID) != "" {
		lines = append(lines, "- Run ID: `"+state.run.ID+"`")
	}
	return title, strings.Join(lines, "\n")
}

func splitRepoFullName(repoFullName string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(repoFullName), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid repository full name %q", repoFullName)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func safeRunWorkspaceID(state *resolvedRunState) string {
	if state == nil || state.run == nil {
		return ""
	}
	return state.run.WorkspaceID
}

func safeRunID(state *resolvedRunState) string {
	if state == nil || state.run == nil {
		return ""
	}
	return state.run.ID
}

func (a *AgentRunActivities) recordPR(ctx context.Context, state *resolvedRunState, metadata workerpkg.PRMetadata, title string) error {
	if state.deliveryTarget == nil {
		return nil
	}
	state.deliveryTarget.ActivePRNumber = &metadata.Number
	state.deliveryTarget.ActivePRTitle = &title
	state.deliveryTarget.ActivePRURL = &metadata.URL
	openStatus := "open"
	state.deliveryTarget.ActivePRStatus = &openStatus
	state.deliveryTarget.LastRunID = &state.run.ID
	state.deliveryTarget.DeliveryState = "pr_open"
	now := time.Now()
	state.deliveryTarget.LastSyncedAt = &now
	if err := a.deliveryRepo.Save(ctx, state.deliveryTarget); err != nil {
		return err
	}
	return a.upsertGitLink(ctx, state, "", &prUpdate{
		number: metadata.Number,
		title:  title,
		url:    metadata.URL,
		status: openStatus,
	})
}

type prUpdate struct {
	number int
	title  string
	url    string
	status string
}

func (a *AgentRunActivities) upsertGitLink(ctx context.Context, state *resolvedRunState, sha string, pr *prUpdate) error {
	if state.task == nil || state.deliveryTarget == nil || state.repository == nil || state.integration == nil {
		return nil
	}
	branch := derefString(state.deliveryTarget.WorkingBranch)
	if branch == "" {
		return nil
	}

	link, err := a.gitLinkRepo.GetByBranch(ctx, state.run.WorkspaceID, state.repository.FullName, branch)
	if err != nil {
		return err
	}
	if link == nil {
		link = &model.TaskGitLink{
			WorkspaceID:   state.run.WorkspaceID,
			TaskID:        state.task.ID,
			IntegrationID: state.integration.ID,
			RepositoryID:  state.deliveryTarget.RepositoryID,
			RunID:         &state.run.ID,
			Provider:      state.integration.Provider,
			BaseURL:       state.integration.BaseURL,
			Repo:          state.repository.FullName,
			Branch:        &branch,
		}
		if sha != "" {
			link.CommitSHA = &sha
		}
		if pr != nil {
			link.PRNumber = &pr.number
			link.PRTitle = &pr.title
			link.PRURL = &pr.url
			link.PRStatus = &pr.status
		}
		return a.gitLinkRepo.Create(ctx, link)
	}

	link.RepositoryID = state.deliveryTarget.RepositoryID
	link.RunID = &state.run.ID
	link.Provider = state.integration.Provider
	link.BaseURL = state.integration.BaseURL
	link.IntegrationID = state.integration.ID
	if sha != "" {
		link.CommitSHA = &sha
	}
	if pr != nil {
		link.PRNumber = &pr.number
		link.PRTitle = &pr.title
		link.PRURL = &pr.url
		link.PRStatus = &pr.status
	}
	return a.gitLinkRepo.Update(ctx, link)
}

func (a *AgentRunActivities) runGitInDir(ctx context.Context, dir string, integration *model.GitIntegration, accessToken string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmdArgs := append(gitAuthArgs(integration, accessToken), args...)
	cmd := exec.CommandContext(cmdCtx, "git", cmdArgs...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}

func gitAuthArgs(integration *model.GitIntegration, accessToken string) []string {
	if integration == nil || accessToken == "" {
		return nil
	}
	switch integration.Provider {
	case "github":
		auth := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + accessToken))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	case "gitlab":
		auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + accessToken))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	default:
		return nil
	}
}

func buildWorkingBranch(task *model.PMTask, teamDefault *model.PMTeamRepoDefault, workspaceKey string) string {
	return model.BuildTaskWorkingBranch(task, teamDefault, workspaceKey)
}
