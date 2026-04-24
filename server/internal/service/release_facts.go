package service

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var genericPRNumberPattern = regexp.MustCompile(`(?i)(?:#|/pull/)(\d+)`)

type ReleaseFactsService struct {
	integrationRepo *repository.GitIntegrationRepository
	repoRepo        *repository.GitRepositoryRepository
	taskRepo        *repository.PMTaskRepository
	taskLinkRepo    *repository.TaskGitLinkRepository
	commentRepo     *repository.PMCommentRepository
	docsLinkRepo    *repository.DocsLinkRepository
	docsDocRepo     *repository.DocsDocumentRepository
	docsContentRepo *repository.DocsContentRepository
	workspaceRepo   *repository.WorkspaceRepository
	githubApp       *githubapp.Client
}

func NewReleaseFactsService(
	integrationRepo *repository.GitIntegrationRepository,
	repoRepo *repository.GitRepositoryRepository,
	taskRepo *repository.PMTaskRepository,
	taskLinkRepo *repository.TaskGitLinkRepository,
	commentRepo *repository.PMCommentRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsDocRepo *repository.DocsDocumentRepository,
	docsContentRepo *repository.DocsContentRepository,
	workspaceRepo *repository.WorkspaceRepository,
	githubApp *githubapp.Client,
) *ReleaseFactsService {
	return &ReleaseFactsService{
		integrationRepo: integrationRepo,
		repoRepo:        repoRepo,
		taskRepo:        taskRepo,
		taskLinkRepo:    taskLinkRepo,
		commentRepo:     commentRepo,
		docsLinkRepo:    docsLinkRepo,
		docsDocRepo:     docsDocRepo,
		docsContentRepo: docsContentRepo,
		workspaceRepo:   workspaceRepo,
		githubApp:       githubApp,
	}
}

func (s *ReleaseFactsService) ResolveReleaseKind(ctx context.Context, workspaceID, repoFullName, tagName string) (string, error) {
	if s == nil {
		return "unknown", fmt.Errorf("release facts service is not configured")
	}
	repo, integration, owner, repoName, err := s.resolveGitHubRepository(ctx, workspaceID, "", repoFullName)
	if err != nil {
		return "unknown", err
	}
	if repo == nil || integration == nil {
		return "unknown", fmt.Errorf("repository not found")
	}
	current, err := s.githubApp.GetReleaseByTag(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, tagName)
	if err != nil {
		return "unknown", err
	}
	if current == nil {
		return "unknown", nil
	}
	releases, err := s.githubApp.ListReleases(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, githubapp.ListReleasesOptions{
		IncludeDrafts:      false,
		IncludePrereleases: true,
		PerPage:            100,
		MaxPages:           5,
	})
	if err != nil {
		return "unknown", err
	}
	previous := selectPreviousRelease(releases, current)
	return classifyReleaseKind(current, previous), nil
}

func (s *ReleaseFactsService) GetReleaseContext(ctx context.Context, workspaceID string, req model.GetReleaseContextRequest) (*model.ReleaseContextResult, error) {
	if s == nil {
		return nil, fmt.Errorf("release facts service is not configured")
	}
	req.RepositoryID = strings.TrimSpace(req.RepositoryID)
	req.RepoFullName = strings.TrimSpace(req.RepoFullName)
	req.TagName = strings.TrimSpace(req.TagName)
	if req.TagName == "" {
		return nil, fmt.Errorf("tag_name is required")
	}
	if req.MaxCommits <= 0 || req.MaxCommits > 200 {
		req.MaxCommits = 100
	}
	if req.MaxFiles <= 0 || req.MaxFiles > 500 {
		req.MaxFiles = 200
	}

	repo, integration, owner, repoName, err := s.resolveGitHubRepository(ctx, workspaceID, req.RepositoryID, req.RepoFullName)
	if err != nil {
		return nil, err
	}
	current, err := s.githubApp.GetReleaseByTag(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, req.TagName)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("release not found")
	}

	releases, err := s.githubApp.ListReleases(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, githubapp.ListReleasesOptions{
		IncludeDrafts:      false,
		IncludePrereleases: true,
		PerPage:            100,
		MaxPages:           5,
	})
	if err != nil {
		return nil, err
	}
	previous := selectPreviousRelease(releases, current)
	result := &model.ReleaseContextResult{
		RepositoryID:   repo.ID,
		RepoFullName:   repo.FullName,
		CurrentRelease: toModelReleaseSummary(current),
		ReleaseKind:    classifyReleaseKind(current, previous),
		Warnings:       []string{},
	}
	if previous != nil {
		summary := toModelReleaseSummary(previous)
		result.PreviousRelease = &summary
	} else {
		result.Warnings = append(result.Warnings, "previous_release_not_found")
	}

	if previous != nil {
		compare, err := s.githubApp.CompareRefs(ctx, strings.TrimSpace(*integration.InstallationID), owner, repoName, previous.TagName, current.TagName)
		if err != nil {
			return nil, err
		}
		result.CompareURL = strings.TrimSpace(compare.HTMLURL)

		commits := compare.Commits
		if len(commits) > req.MaxCommits {
			commits = commits[:req.MaxCommits]
			result.Warnings = append(result.Warnings, "commits_truncated")
		}
		for _, commit := range commits {
			result.Commits = append(result.Commits, model.ReleaseCommitSummary{
				SHA:         strings.TrimSpace(commit.SHA),
				Message:     firstNonEmptyLine(commit.Message),
				AuthorName:  strings.TrimSpace(commit.AuthorName),
				AuthorLogin: strings.TrimSpace(commit.AuthorLogin),
				URL:         strings.TrimSpace(commit.HTMLURL),
			})
		}

		prs := extractPullRequests(compare.Commits)
		result.PullRequests = prs

		if req.IncludeChangedFiles {
			files := compare.Files
			if len(files) > req.MaxFiles {
				files = files[:req.MaxFiles]
				result.Warnings = append(result.Warnings, "files_truncated")
			}
			for _, file := range files {
				result.ChangedFiles = append(result.ChangedFiles, model.ReleaseChangedFileSummary{
					Path:      strings.TrimSpace(file.Filename),
					Status:    strings.TrimSpace(file.Status),
					Additions: file.Additions,
					Deletions: file.Deletions,
					Changes:   file.Changes,
				})
			}
		}

		taskMatches, err := s.FindTasksForGitChanges(ctx, workspaceID, model.FindTasksForGitChangesRequest{
			RepositoryID: repo.ID,
			RepoFullName: repo.FullName,
			PRNumbers:    pullRequestNumbers(prs),
			CommitSHAs:   commitSHAs(compare.Commits),
			Texts:        commitMessages(compare.Commits),
		})
		if err != nil {
			return nil, err
		}
		result.RelatedTaskMatches = taskMatches.Matches
	}

	if len(result.Warnings) == 0 {
		result.Warnings = nil
	}
	return result, nil
}

func (s *ReleaseFactsService) FindTasksForGitChanges(ctx context.Context, workspaceID string, req model.FindTasksForGitChangesRequest) (*model.FindTasksForGitChangesResult, error) {
	if s == nil {
		return nil, fmt.Errorf("release facts service is not configured")
	}
	repoFullName := strings.TrimSpace(req.RepoFullName)
	if strings.TrimSpace(req.RepositoryID) != "" {
		repo, err := s.repoRepo.GetByID(ctx, workspaceID, strings.TrimSpace(req.RepositoryID))
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		repoFullName = strings.TrimSpace(repo.FullName)
	} else if repoFullName == "" {
		return nil, fmt.Errorf("repo_full_name or repository_id is required")
	}

	prNumbers := uniqueInts(req.PRNumbers, 50)
	commitSHAs := uniqueStringsWithLimit(req.CommitSHAs, 200)
	branches := uniqueStringsWithLimit(req.Branches, 50)
	texts := uniqueNonEmptyStringsWithLimit(req.Texts, 100)
	if len(prNumbers) == 0 && len(commitSHAs) == 0 && len(branches) == 0 && len(texts) == 0 {
		return nil, fmt.Errorf("at least one evidence array is required")
	}

	matchState := map[string]*model.GitChangeTaskMatch{}
	matchedPRs := map[int]struct{}{}
	matchedSHAs := map[string]struct{}{}
	matchedBranches := map[string]struct{}{}
	matchedTaskKeys := map[string]struct{}{}

	if len(prNumbers) > 0 {
		links, err := s.taskLinkRepo.ListByRepoAndPRs(ctx, workspaceID, repoFullName, prNumbers)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if link.PRNumber == nil {
				continue
			}
			matchedPRs[*link.PRNumber] = struct{}{}
			addTaskMatchEvidence(matchState, link.TaskID, "pull_request_number", map[string]any{"pr_number": *link.PRNumber}, "high")
		}
	}

	if len(commitSHAs) > 0 {
		links, err := s.taskLinkRepo.ListByRepoAndCommitSHAs(ctx, workspaceID, repoFullName, commitSHAs)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if link.CommitSHA == nil {
				continue
			}
			sha := strings.TrimSpace(*link.CommitSHA)
			matchedSHAs[sha] = struct{}{}
			addTaskMatchEvidence(matchState, link.TaskID, "commit_sha", map[string]any{"commit_sha": sha}, "high")
		}
	}

	if len(branches) > 0 {
		links, err := s.taskLinkRepo.ListByRepoAndBranches(ctx, workspaceID, repoFullName, branches)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if link.Branch == nil {
				continue
			}
			branch := strings.TrimSpace(*link.Branch)
			matchedBranches[branch] = struct{}{}
			addTaskMatchEvidence(matchState, link.TaskID, "branch", map[string]any{"branch": branch}, "high")
		}
	}

	displayIDs, taskKeys, err := s.extractWorkspaceTaskKeys(ctx, workspaceID, texts)
	if err != nil {
		return nil, err
	}
	if len(displayIDs) > 0 {
		tasks, err := s.taskRepo.ListByDisplayIDs(ctx, workspaceID, displayIDs)
		if err != nil {
			return nil, err
		}
		workspaceKey := s.workspaceKey(ctx, workspaceID)
		for _, task := range tasks {
			taskKey := model.FormatTaskKey(workspaceKey, task.DisplayID)
			matchedTaskKeys[taskKey] = struct{}{}
			addTaskMatchEvidence(matchState, task.ID, "task_key", map[string]any{"task_key": taskKey}, "medium")
		}
	}

	taskIDs := make([]string, 0, len(matchState))
	for taskID := range matchState {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, taskIDs)
	if err != nil {
		return nil, err
	}
	workspaceKey := s.workspaceKey(ctx, workspaceID)
	taskKeyByID := make(map[string]string, len(tasks))
	for _, task := range tasks {
		taskKeyByID[task.ID] = model.FormatTaskKey(workspaceKey, task.DisplayID)
	}

	matches := make([]model.GitChangeTaskMatch, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		match := matchState[taskID]
		match.TaskKey = taskKeyByID[taskID]
		matches = append(matches, *match)
	}

	return &model.FindTasksForGitChangesResult{
		Matches: matches,
		Unmatched: map[string][]any{
			"pr_numbers":  unmatchedInts(prNumbers, matchedPRs),
			"commit_shas": unmatchedStrings(commitSHAs, matchedSHAs),
			"branches":    unmatchedStrings(branches, matchedBranches),
			"texts":       unmatchedTaskKeyTexts(taskKeys, matchedTaskKeys),
		},
	}, nil
}

func (s *ReleaseFactsService) GetTaskContext(ctx context.Context, workspaceID string, req model.GetTaskContextRequest) (*model.GetTaskContextResult, error) {
	if s == nil {
		return nil, fmt.Errorf("release facts service is not configured")
	}
	taskIDs := uniqueNonEmptyStringsWithLimit(req.TaskIDs, 50)
	if len(taskIDs) == 0 {
		return nil, fmt.Errorf("task_ids is required")
	}

	tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, taskIDs)
	if err != nil {
		return nil, err
	}
	stateIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		stateIDs = append(stateIDs, task.WorkflowStateID)
	}
	stateNames, err := s.taskRepo.ListStateNamesByIDs(ctx, stateIDs)
	if err != nil {
		return nil, err
	}
	workspaceKey := s.workspaceKey(ctx, workspaceID)

	result := &model.GetTaskContextResult{Tasks: make([]model.TaskContextItem, 0, len(tasks))}
	for _, task := range tasks {
		item := model.TaskContextItem{
			TaskID:          task.ID,
			TaskKey:         model.FormatTaskKey(workspaceKey, task.DisplayID),
			Title:           task.Name,
			TaskType:        task.TaskType,
			Priority:        task.Priority,
			DescriptionText: strings.TrimSpace(derefString(task.Description)),
			Status:          stateNames[task.WorkflowStateID],
			WorkflowStateID: task.WorkflowStateID,
			TeamID:          task.TeamID,
			OwnerID:         task.OwnerID,
			OwnerMemberID:   task.OwnerMemberID,
			AssigneeAgentID: task.AssignedAgentID,
			EpicID:          task.EpicID,
		}

		if req.IncludeGitLinks {
			links, err := s.taskLinkRepo.ListByTask(ctx, workspaceID, task.ID)
			if err != nil {
				return nil, err
			}
			for _, link := range links {
				item.GitLinks = append(item.GitLinks, model.TaskContextGitLink{
					RepositoryID: derefString(link.RepositoryID),
					Repo:         link.Repo,
					Branch:       link.Branch,
					PRNumber:     link.PRNumber,
					PRTitle:      link.PRTitle,
					PRURL:        link.PRURL,
					PRStatus:     link.PRStatus,
					CommitSHA:    link.CommitSHA,
				})
			}
		}

		if req.IncludeComments && s.commentRepo != nil {
			comments, err := s.commentRepo.List(ctx, "task", task.ID)
			if err != nil {
				return nil, err
			}
			for _, comment := range comments {
				item.Comments = append(item.Comments, model.TaskContextComment{
					CommentID:  comment.Comment.ID,
					AuthorID:   comment.Author.ID,
					AuthorName: comment.Author.FullName,
					Body:       strings.TrimSpace(comment.Comment.Body),
					ReplyCount: comment.ReplyCount,
				})
			}
		}

		if req.IncludeLinkedDocs {
			links, err := s.docsLinkRepo.ListByObject(ctx, workspaceID, "task", task.ID)
			if err != nil {
				return nil, err
			}
			docIDs := make([]string, 0, len(links))
			linkContextByDocID := make(map[string]string, len(links))
			for _, link := range links {
				docIDs = append(docIDs, link.DocumentID)
				linkContextByDocID[link.DocumentID] = link.LinkContext
			}
			docs, err := s.docsDocRepo.ListByIDs(ctx, workspaceID, docIDs)
			if err != nil {
				return nil, err
			}
			contentByDocID := map[string]string{}
			if req.IncludeDocumentContent {
				contents, err := s.docsContentRepo.ListByDocumentIDs(ctx, docIDs)
				if err != nil {
					return nil, err
				}
				for _, content := range contents {
					contentByDocID[content.DocumentID] = strings.TrimSpace(content.ContentText)
				}
			}
			for _, doc := range docs {
				item.LinkedDocs = append(item.LinkedDocs, model.TaskContextDocument{
					DocumentID:  doc.ID,
					Title:       doc.Title,
					Status:      doc.Status,
					LinkContext: linkContextByDocID[doc.ID],
					ContentText: contentByDocID[doc.ID],
				})
			}
		}

		result.Tasks = append(result.Tasks, item)
	}

	sort.Slice(result.Tasks, func(i, j int) bool {
		return result.Tasks[i].TaskKey < result.Tasks[j].TaskKey
	})
	return result, nil
}

func (s *ReleaseFactsService) resolveGitHubRepository(ctx context.Context, workspaceID, repositoryID, repoFullName string) (*model.GitRepository, *model.GitIntegration, string, string, error) {
	if s.repoRepo == nil || s.integrationRepo == nil {
		return nil, nil, "", "", fmt.Errorf("git repositories are not available")
	}
	var (
		repo *model.GitRepository
		err  error
	)
	if strings.TrimSpace(repositoryID) != "" {
		repo, err = s.repoRepo.GetByID(ctx, workspaceID, strings.TrimSpace(repositoryID))
		if err != nil {
			return nil, nil, "", "", err
		}
	} else {
		repo, err = s.repoRepo.GetByFullName(ctx, workspaceID, strings.TrimSpace(repoFullName))
		if err != nil {
			return nil, nil, "", "", err
		}
	}
	if repo == nil {
		return nil, nil, "", "", fmt.Errorf("repository not found")
	}
	integration, err := s.integrationRepo.GetByID(ctx, workspaceID, repo.IntegrationID)
	if err != nil {
		return nil, nil, "", "", err
	}
	if integration == nil {
		return nil, nil, "", "", fmt.Errorf("git integration not found")
	}
	if integration.Provider != "github" {
		return nil, nil, "", "", fmt.Errorf("release facts are not implemented for %s", integration.Provider)
	}
	if integration.InstallationID == nil || strings.TrimSpace(*integration.InstallationID) == "" {
		return nil, nil, "", "", fmt.Errorf("integration has no installation_id")
	}
	owner, repoName, err := splitRepositoryFullName(repo.FullName)
	if err != nil {
		return nil, nil, "", "", err
	}
	if s.githubApp == nil {
		return nil, nil, "", "", fmt.Errorf("github app is not configured")
	}
	return repo, integration, owner, repoName, nil
}

func splitRepositoryFullName(fullName string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(fullName), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("repository full name is invalid")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func selectPreviousRelease(releases []githubapp.Release, current *githubapp.Release) *githubapp.Release {
	if current == nil {
		return nil
	}
	currentTag := strings.TrimSpace(current.TagName)
	currentPublishedAt := current.PublishedAt
	allowPrerelease := current.Prerelease
	for _, release := range releases {
		if strings.TrimSpace(release.TagName) == "" || strings.EqualFold(strings.TrimSpace(release.TagName), currentTag) {
			continue
		}
		if release.Draft {
			continue
		}
		if !allowPrerelease && release.Prerelease {
			continue
		}
		if currentPublishedAt != nil && release.PublishedAt != nil && !release.PublishedAt.Before(*currentPublishedAt) {
			continue
		}
		candidate := release
		return &candidate
	}
	return nil
}

func classifyReleaseKind(current, previous *githubapp.Release) string {
	if current == nil {
		return "unknown"
	}
	if current.Prerelease {
		return "prerelease"
	}
	currentVersion, ok := parseSemverTag(current.TagName)
	if !ok || previous == nil {
		return "unknown"
	}
	previousVersion, ok := parseSemverTag(previous.TagName)
	if !ok {
		return "unknown"
	}
	switch {
	case currentVersion[0] > previousVersion[0]:
		return "major"
	case currentVersion[0] == previousVersion[0] && currentVersion[1] > previousVersion[1]:
		return "minor"
	case currentVersion[0] == previousVersion[0] && currentVersion[1] == previousVersion[1] && currentVersion[2] > previousVersion[2]:
		return "patch"
	default:
		return "unknown"
	}
}

func parseSemverTag(tag string) ([3]int, bool) {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "V")
	if idx := strings.Index(tag, "+"); idx >= 0 {
		tag = tag[:idx]
	}
	if idx := strings.Index(tag, "-"); idx >= 0 {
		return [3]int{}, false
	}
	parts := strings.Split(tag, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var parsed [3]int
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return [3]int{}, false
		}
		parsed[i] = value
	}
	return parsed, true
}

func toModelReleaseSummary(release *githubapp.Release) model.ReleaseSummary {
	if release == nil {
		return model.ReleaseSummary{}
	}
	return model.ReleaseSummary{
		TagName:         strings.TrimSpace(release.TagName),
		Name:            strings.TrimSpace(release.Name),
		URL:             strings.TrimSpace(release.HTMLURL),
		TargetCommitish: strings.TrimSpace(release.TargetCommitish),
		PublishedAt:     release.PublishedAt,
		IsPrerelease:    release.Prerelease,
	}
}

func extractPullRequests(commits []githubapp.CompareCommit) []model.ReleasePullRequestSummary {
	seen := map[int]struct{}{}
	result := make([]model.ReleasePullRequestSummary, 0, len(commits))
	for _, commit := range commits {
		for _, match := range genericPRNumberPattern.FindAllStringSubmatch(commit.Message, -1) {
			if len(match) < 2 {
				continue
			}
			number, err := strconv.Atoi(strings.TrimSpace(match[1]))
			if err != nil {
				continue
			}
			if _, exists := seen[number]; exists {
				continue
			}
			seen[number] = struct{}{}
			result = append(result, model.ReleasePullRequestSummary{
				Number: number,
				Title:  firstNonEmptyLine(commit.Message),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Number < result[j].Number })
	return result
}

func pullRequestNumbers(prs []model.ReleasePullRequestSummary) []int {
	numbers := make([]int, 0, len(prs))
	for _, pr := range prs {
		if pr.Number > 0 {
			numbers = append(numbers, pr.Number)
		}
	}
	return numbers
}

func commitSHAs(commits []githubapp.CompareCommit) []string {
	values := make([]string, 0, len(commits))
	for _, commit := range commits {
		if strings.TrimSpace(commit.SHA) != "" {
			values = append(values, strings.TrimSpace(commit.SHA))
		}
	}
	return values
}

func commitMessages(commits []githubapp.CompareCommit) []string {
	values := make([]string, 0, len(commits))
	for _, commit := range commits {
		if strings.TrimSpace(commit.Message) != "" {
			values = append(values, strings.TrimSpace(commit.Message))
		}
	}
	return values
}

func addTaskMatchEvidence(matches map[string]*model.GitChangeTaskMatch, taskID, matchedBy string, refs map[string]any, confidence string) {
	entry, ok := matches[taskID]
	if !ok {
		entry = &model.GitChangeTaskMatch{
			TaskID:      taskID,
			Confidence:  confidence,
			MatchedBy:   []string{},
			MatchedRefs: map[string]any{},
		}
		matches[taskID] = entry
	}
	if !slices.Contains(entry.MatchedBy, matchedBy) {
		entry.MatchedBy = append(entry.MatchedBy, matchedBy)
		sort.Strings(entry.MatchedBy)
	}
	for key, value := range refs {
		entry.MatchedRefs[key] = value
	}
	if entry.Confidence != "high" && confidence == "high" {
		entry.Confidence = "high"
	}
}

func (s *ReleaseFactsService) extractWorkspaceTaskKeys(ctx context.Context, workspaceID string, texts []string) ([]int, []string, error) {
	if len(texts) == 0 {
		return nil, nil, nil
	}
	workspaceKey := s.workspaceKey(ctx, workspaceID)
	if workspaceKey == "" {
		return nil, nil, nil
	}
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(workspaceKey) + `-(\d+)\b`)
	displayIDs := make([]int, 0, len(texts))
	taskKeys := make([]string, 0, len(texts))
	seen := map[int]struct{}{}
	for _, text := range texts {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			displayID, err := strconv.Atoi(strings.TrimSpace(match[1]))
			if err != nil {
				continue
			}
			if _, exists := seen[displayID]; exists {
				continue
			}
			seen[displayID] = struct{}{}
			displayIDs = append(displayIDs, displayID)
			taskKeys = append(taskKeys, model.FormatTaskKey(workspaceKey, displayID))
		}
	}
	sort.Ints(displayIDs)
	sort.Strings(taskKeys)
	return displayIDs, taskKeys, nil
}

func (s *ReleaseFactsService) workspaceKey(ctx context.Context, workspaceID string) string {
	if s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return ""
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || workspace == nil {
		return ""
	}
	return strings.TrimSpace(workspace.WorkspaceKey)
}

func uniqueStringsWithLimit(values []string, max int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func uniqueNonEmptyStringsWithLimit(values []string, max int) []string {
	return uniqueStringsWithLimit(values, max)
}

func uniqueInts(values []int, max int) []int {
	out := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if max > 0 && len(out) >= max {
			break
		}
	}
	sort.Ints(out)
	return out
}

func unmatchedInts(values []int, matched map[int]struct{}) []any {
	out := make([]any, 0)
	for _, value := range values {
		if _, ok := matched[value]; ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

func unmatchedStrings(values []string, matched map[string]struct{}) []any {
	out := make([]any, 0)
	for _, value := range values {
		if _, ok := matched[value]; ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

func unmatchedTaskKeyTexts(taskKeys []string, matched map[string]struct{}) []any {
	out := make([]any, 0)
	for _, value := range taskKeys {
		if _, ok := matched[value]; ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
