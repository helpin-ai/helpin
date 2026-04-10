package worker

import (
	"context"
	"slices"
	"strings"
)

func recordLocalEngineerCommit(execCtx *ExecutionContext, artifactWriter interface {
	Save(context.Context, string, string, string, bool)
}, branch, commitSHA, commitMessage string, changedFiles []string) error {
	if execCtx == nil {
		return nil
	}

	branch = strings.TrimSpace(branch)
	commitSHA = strings.TrimSpace(commitSHA)
	commitMessage = strings.TrimSpace(commitMessage)
	changedFiles = slices.Clone(changedFiles)

	execCtx.WorkingBranch = branch
	execCtx.LocalGitCommit = &GitCommitMetadata{
		Branch:        branch,
		CommitSHA:     commitSHA,
		CommitMessage: commitMessage,
		ChangedFiles:  changedFiles,
	}

	persistenceResult := map[string]any{
		"branch":         branch,
		"commit_sha":     commitSHA,
		"commit_message": commitMessage,
		"changed_files":  changedFiles,
		"delivery":       "local_commit",
	}
	if artifactWriter != nil {
		artifactWriter.Save(execCtx.Context, "git_persistence_result", "json", toJSONString(persistenceResult), false)
	}
	return nil
}

func persistExistingEngineerCommitLocally(execCtx *ExecutionContext, artifactWriter interface {
	Save(context.Context, string, string, string, bool)
}, change *engineerCommittedChange) error {
	if execCtx == nil || change == nil {
		return nil
	}

	artifactWriter.Save(execCtx.Context, "diff", "patch", change.Diff, false)
	artifactWriter.Save(execCtx.Context, "file_bundle", "json", toJSONString(change.ChangedFiles), false)

	branch, err := resolveWorkingBranch(execCtx)
	if err != nil {
		return err
	}
	return recordLocalEngineerCommit(execCtx, artifactWriter, branch, change.CommitSHA, change.CommitMessage, change.ChangedFiles)
}
