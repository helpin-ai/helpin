package model

import "time"

type GetReleaseContextRequest struct {
	RepositoryID        string `json:"repository_id,omitempty"`
	RepoFullName        string `json:"repo_full_name,omitempty"`
	TagName             string `json:"tag_name,omitempty"`
	IncludeChangedFiles bool   `json:"include_changed_files,omitempty"`
	MaxCommits          int    `json:"max_commits,omitempty"`
	MaxFiles            int    `json:"max_files,omitempty"`
}

type ReleaseSummary struct {
	TagName         string     `json:"tag_name,omitempty"`
	Name            string     `json:"name,omitempty"`
	URL             string     `json:"url,omitempty"`
	TargetCommitish string     `json:"target_commitish,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	IsPrerelease    bool       `json:"is_prerelease,omitempty"`
}

type ReleaseCommitSummary struct {
	SHA         string `json:"sha,omitempty"`
	Message     string `json:"message,omitempty"`
	AuthorName  string `json:"author_name,omitempty"`
	AuthorLogin string `json:"author_login,omitempty"`
	URL         string `json:"url,omitempty"`
}

type ReleasePullRequestSummary struct {
	Number int    `json:"number,omitempty"`
	Title  string `json:"title,omitempty"`
	URL    string `json:"url,omitempty"`
}

type ReleaseChangedFileSummary struct {
	Path      string `json:"path,omitempty"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
	Changes   int    `json:"changes,omitempty"`
}

type GitChangeTaskMatch struct {
	TaskID      string         `json:"task_id"`
	TaskKey     string         `json:"task_key,omitempty"`
	Confidence  string         `json:"confidence"`
	MatchedBy   []string       `json:"matched_by,omitempty"`
	MatchedRefs map[string]any `json:"matched_refs,omitempty"`
}

type ReleaseContextResult struct {
	RepositoryID       string                      `json:"repository_id,omitempty"`
	RepoFullName       string                      `json:"repo_full_name,omitempty"`
	CurrentRelease     ReleaseSummary              `json:"current_release"`
	PreviousRelease    *ReleaseSummary             `json:"previous_release,omitempty"`
	ReleaseKind        string                      `json:"release_kind,omitempty"`
	CompareURL         string                      `json:"compare_url,omitempty"`
	Commits            []ReleaseCommitSummary      `json:"commits,omitempty"`
	PullRequests       []ReleasePullRequestSummary `json:"pull_requests,omitempty"`
	ChangedFiles       []ReleaseChangedFileSummary `json:"changed_files,omitempty"`
	RelatedTaskMatches []GitChangeTaskMatch        `json:"related_task_matches,omitempty"`
	Warnings           []string                    `json:"warnings,omitempty"`
}

type FindTasksForGitChangesRequest struct {
	RepositoryID string   `json:"repository_id,omitempty"`
	RepoFullName string   `json:"repo_full_name,omitempty"`
	PRNumbers    []int    `json:"pr_numbers,omitempty"`
	CommitSHAs   []string `json:"commit_shas,omitempty"`
	Branches     []string `json:"branches,omitempty"`
	Texts        []string `json:"texts,omitempty"`
}

type FindTasksForGitChangesResult struct {
	Matches   []GitChangeTaskMatch `json:"matches,omitempty"`
	Unmatched map[string][]any     `json:"unmatched,omitempty"`
}

type GetTaskContextRequest struct {
	TaskIDs                []string `json:"task_ids"`
	TaskKeys               []string `json:"task_keys,omitempty"`
	IncludeLinkedDocs      bool     `json:"include_linked_docs,omitempty"`
	IncludeDocumentContent bool     `json:"include_document_content,omitempty"`
	IncludeComments        bool     `json:"include_comments,omitempty"`
	IncludeGitLinks        bool     `json:"include_git_links,omitempty"`
}

type TaskContextDocument struct {
	DocumentID  string `json:"document_id"`
	Title       string `json:"title"`
	Status      string `json:"status,omitempty"`
	LinkContext string `json:"link_context,omitempty"`
	ContentText string `json:"content_text,omitempty"`
}

type TaskContextComment struct {
	CommentID  string `json:"comment_id"`
	AuthorID   string `json:"author_id,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	Body       string `json:"body,omitempty"`
	ReplyCount int    `json:"reply_count,omitempty"`
}

type TaskContextGitLink struct {
	RepositoryID string  `json:"repository_id,omitempty"`
	Repo         string  `json:"repo,omitempty"`
	Branch       *string `json:"branch,omitempty"`
	PRNumber     *int    `json:"pr_number,omitempty"`
	PRTitle      *string `json:"pr_title,omitempty"`
	PRURL        *string `json:"pr_url,omitempty"`
	PRStatus     *string `json:"pr_status,omitempty"`
	CommitSHA    *string `json:"commit_sha,omitempty"`
}

type TaskContextItem struct {
	Attachments     []TaskAttachment      `json:"attachments"`
	TaskID          string                `json:"task_id"`
	TaskKey         string                `json:"task_key,omitempty"`
	Title           string                `json:"title"`
	TaskType        string                `json:"task_type,omitempty"`
	Priority        string                `json:"priority,omitempty"`
	DescriptionText string                `json:"description_text,omitempty"`
	Status          string                `json:"status,omitempty"`
	WorkflowStateID string                `json:"workflow_state_id,omitempty"`
	TeamID          *string               `json:"team_id,omitempty"`
	OwnerID         *string               `json:"owner_id,omitempty"`
	OwnerMemberID   *string               `json:"owner_member_id,omitempty"`
	AssigneeAgentID *string               `json:"assignee_agent_id,omitempty"`
	EpicID          *string               `json:"epic_id,omitempty"`
	LinkedDocs      []TaskContextDocument `json:"linked_docs,omitempty"`
	GitLinks        []TaskContextGitLink  `json:"git_links,omitempty"`
	Comments        []TaskContextComment  `json:"comments,omitempty"`
}

type GetTaskContextResult struct {
	Tasks []TaskContextItem `json:"tasks"`
}

// GetPullRequestChangesRequest selects one pull request of a workspace repository.
type GetPullRequestChangesRequest struct {
	RepositoryID string `json:"repository_id,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
	PullNumber   int    `json:"pull_number"`
}

// PullRequestChangedFile is one changed file of a pull request with its patch.
type PullRequestChangedFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
}

// PullRequestChangesResult is the pull request description and changed files.
type PullRequestChangesResult struct {
	RepoFullName string                   `json:"repo_full_name"`
	Number       int                      `json:"number"`
	Title        string                   `json:"title,omitempty"`
	Body         string                   `json:"body,omitempty"`
	URL          string                   `json:"url,omitempty"`
	State        string                   `json:"state,omitempty"`
	Merged       bool                     `json:"merged"`
	BaseBranch   string                   `json:"base_branch,omitempty"`
	HeadBranch   string                   `json:"head_branch,omitempty"`
	Files        []PullRequestChangedFile `json:"files"`
	Truncated    bool                     `json:"truncated,omitempty"`
}
