package githubapp

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Repository is the subset of GitHub repository metadata needed by the product.
type Repository struct {
	ID            int64
	FullName      string
	DefaultBranch string
	Private       bool
	Archived      bool
	Permissions   map[string]bool
}

// Installation is the subset of GitHub installation metadata needed for onboarding.
type Installation struct {
	ID           int64
	AppID        int64
	AccountLogin string
	AccountType  string
	HTMLURL      string
}

type Branch struct {
	Name string
}

type Release struct {
	ID              int64
	TagName         string
	Name            string
	HTMLURL         string
	TargetCommitish string
	Draft           bool
	Prerelease      bool
	PublishedAt     *time.Time
}

type PullRequest struct {
	Number    int
	Title     string
	HTMLURL   string
	State     string
	Merged    bool
	HeadRef   string
	BaseRef   string
	HeadSHA   string
	UpdatedAt *time.Time
	MergedAt  *time.Time
}

type PullRequestFile struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Changes   int
	Patch     string
}

type CheckRunAnnotation struct {
	Path            string
	StartLine       int
	EndLine         int
	AnnotationLevel string
	Message         string
	Title           string
}

type CheckRun struct {
	ID            int64
	Name          string
	HTMLURL       string
	Status        string
	Conclusion    string
	StartedAt     *time.Time
	CompletedAt   *time.Time
	OutputTitle   string
	OutputSummary string
	OutputText    string
	Annotations   []CheckRunAnnotation
}

type ListReleasesOptions struct {
	IncludeDrafts      bool
	IncludePrereleases bool
	PerPage            int
	MaxPages           int
}

type CompareCommit struct {
	SHA         string
	Message     string
	HTMLURL     string
	AuthorName  string
	AuthorLogin string
}

type CompareFile struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Changes   int
}

type CompareRefsResult struct {
	HTMLURL      string
	Status       string
	AheadBy      int
	BehindBy     int
	TotalCommits int
	Commits      []CompareCommit
	Files        []CompareFile
}

// Client creates GitHub App JWTs and installation tokens.
type Client struct {
	appID      string
	privateKey *rsa.PrivateKey
	apiBaseURL string
	httpClient *http.Client
}

// NewClient returns a GitHub App client, or nil when config is incomplete.
func NewClient(appID, privateKeyPEM string) (*Client, error) {
	appID = strings.TrimSpace(appID)
	privateKeyPEM = strings.TrimSpace(privateKeyPEM)
	if appID == "" || privateKeyPEM == "" {
		return nil, nil
	}

	privateKey, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	return &Client{
		appID:      appID,
		privateKey: privateKey,
		apiBaseURL: "https://api.github.com",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// MintInstallationToken returns a short-lived installation token.
func (c *Client) MintInstallationToken(ctx context.Context, installationID string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("github app is not configured")
	}
	appJWT, err := c.createAppJWT()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%s/access_tokens", c.apiBaseURL, installationID), nil)
	if err != nil {
		return "", fmt.Errorf("build github installation token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request github installation token: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Token   string `json:"token"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode github installation token response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github installation token failed (%d): %s", resp.StatusCode, payload.Message)
	}
	if payload.Token == "" {
		return "", fmt.Errorf("github installation token response was empty")
	}
	return payload.Token, nil
}

// ListInstallationRepositories returns repositories visible to the installation.
func (c *Client) ListInstallationRepositories(ctx context.Context, installationID string) ([]Repository, error) {
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/installation/repositories?per_page=100", nil)
	if err != nil {
		return nil, fmt.Errorf("build github installation repositories request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github installation repositories: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Repositories []struct {
			ID            int64           `json:"id"`
			FullName      string          `json:"full_name"`
			DefaultBranch string          `json:"default_branch"`
			Private       bool            `json:"private"`
			Archived      bool            `json:"archived"`
			Permissions   map[string]bool `json:"permissions"`
		} `json:"repositories"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github repositories response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github repository sync failed (%d): %s", resp.StatusCode, payload.Message)
	}

	repos := make([]Repository, 0, len(payload.Repositories))
	for _, repo := range payload.Repositories {
		repos = append(repos, Repository{
			ID:            repo.ID,
			FullName:      repo.FullName,
			DefaultBranch: repo.DefaultBranch,
			Private:       repo.Private,
			Archived:      repo.Archived,
			Permissions:   repo.Permissions,
		})
	}
	return repos, nil
}

// GetInstallation returns metadata for a GitHub App installation.
func (c *Client) GetInstallation(ctx context.Context, installationID string) (*Installation, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	appJWT, err := c.createAppJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/app/installations/%s", c.apiBaseURL, installationID), nil)
	if err != nil {
		return nil, fmt.Errorf("build github installation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github installation: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		ID      int64  `json:"id"`
		AppID   int64  `json:"app_id"`
		HTMLURL string `json:"html_url"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github installation response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github installation lookup failed (%d): %s", resp.StatusCode, payload.Message)
	}

	return &Installation{
		ID:           payload.ID,
		AppID:        payload.AppID,
		AccountLogin: payload.Account.Login,
		AccountType:  payload.Account.Type,
		HTMLURL:      payload.HTMLURL,
	}, nil
}

// ListRepositoryBranches returns repository branch names visible to the installation.
func (c *Client) ListRepositoryBranches(ctx context.Context, installationID, owner, repo string) ([]Branch, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	branches := make([]Branch, 0, 64)
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100&page=%d", c.apiBaseURL, owner, repo, page), nil)
		if err != nil {
			return nil, fmt.Errorf("build github branches request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request github branches: %w", err)
		}

		var payload []struct {
			Name string `json:"name"`
		}
		var errorPayload struct {
			Message string `json:"message"`
		}
		if resp.StatusCode >= 300 {
			_ = json.NewDecoder(resp.Body).Decode(&errorPayload)
			resp.Body.Close()
			return nil, fmt.Errorf("github branches failed (%d): %s", resp.StatusCode, errorPayload.Message)
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode github branches response: %w", err)
		}
		resp.Body.Close()

		for _, branch := range payload {
			name := strings.TrimSpace(branch.Name)
			if name == "" {
				continue
			}
			branches = append(branches, Branch{Name: name})
		}
		if len(payload) < 100 {
			break
		}
	}
	return branches, nil
}

// GetReleaseByTag returns one repository release by tag.
func (c *Client) GetReleaseByTag(ctx context.Context, installationID, owner, repo, tag string) (*Release, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	tag = strings.TrimSpace(tag)
	if owner == "" || repo == "" || tag == "" {
		return nil, fmt.Errorf("owner, repo, and tag are required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.apiBaseURL, owner, repo, tag), nil)
	if err != nil {
		return nil, fmt.Errorf("build github release request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github release by tag: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		ID              int64   `json:"id"`
		TagName         string  `json:"tag_name"`
		Name            string  `json:"name"`
		HTMLURL         string  `json:"html_url"`
		TargetCommitish string  `json:"target_commitish"`
		Draft           bool    `json:"draft"`
		Prerelease      bool    `json:"prerelease"`
		PublishedAt     *string `json:"published_at"`
		Message         string  `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github release response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github release lookup failed (%d): %s", resp.StatusCode, payload.Message)
	}
	return decodeGitHubRelease(payload.ID, payload.TagName, payload.Name, payload.HTMLURL, payload.TargetCommitish, payload.Draft, payload.Prerelease, payload.PublishedAt), nil
}

// ListReleases returns paginated repository releases.
func (c *Client) ListReleases(ctx context.Context, installationID, owner, repo string, opts ListReleasesOptions) ([]Release, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	if opts.PerPage <= 0 || opts.PerPage > 100 {
		opts.PerPage = 100
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 5
	}

	releases := make([]Release, 0, opts.PerPage)
	for page := 1; page <= opts.MaxPages; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", c.apiBaseURL, owner, repo, opts.PerPage, page), nil)
		if err != nil {
			return nil, fmt.Errorf("build github releases request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request github releases: %w", err)
		}

		var payload []struct {
			ID              int64   `json:"id"`
			TagName         string  `json:"tag_name"`
			Name            string  `json:"name"`
			HTMLURL         string  `json:"html_url"`
			TargetCommitish string  `json:"target_commitish"`
			Draft           bool    `json:"draft"`
			Prerelease      bool    `json:"prerelease"`
			PublishedAt     *string `json:"published_at"`
		}
		var errorPayload struct {
			Message string `json:"message"`
		}
		if resp.StatusCode >= 300 {
			_ = json.NewDecoder(resp.Body).Decode(&errorPayload)
			resp.Body.Close()
			return nil, fmt.Errorf("github releases failed (%d): %s", resp.StatusCode, errorPayload.Message)
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode github releases response: %w", err)
		}
		resp.Body.Close()

		if len(payload) == 0 {
			break
		}
		for _, item := range payload {
			release := decodeGitHubRelease(item.ID, item.TagName, item.Name, item.HTMLURL, item.TargetCommitish, item.Draft, item.Prerelease, item.PublishedAt)
			if release == nil {
				continue
			}
			if release.Draft && !opts.IncludeDrafts {
				continue
			}
			if release.Prerelease && !opts.IncludePrereleases {
				continue
			}
			releases = append(releases, *release)
		}
		if len(payload) < opts.PerPage {
			break
		}
	}

	return releases, nil
}

// CompareRefs compares two refs in a repository.
func (c *Client) CompareRefs(ctx context.Context, installationID, owner, repo, baseRef, headRef string) (*CompareRefsResult, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	baseRef = strings.TrimSpace(baseRef)
	headRef = strings.TrimSpace(headRef)
	if owner == "" || repo == "" || baseRef == "" || headRef == "" {
		return nil, fmt.Errorf("owner, repo, base_ref, and head_ref are required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", c.apiBaseURL, owner, repo, baseRef, headRef), nil)
	if err != nil {
		return nil, fmt.Errorf("build github compare request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github compare refs: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		HTMLURL      string `json:"html_url"`
		Status       string `json:"status"`
		AheadBy      int    `json:"ahead_by"`
		BehindBy     int    `json:"behind_by"`
		TotalCommits int    `json:"total_commits"`
		Commits      []struct {
			SHA     string `json:"sha"`
			HTMLURL string `json:"html_url"`
			Commit  struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
				} `json:"author"`
			} `json:"commit"`
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"commits"`
		Files []struct {
			Filename  string `json:"filename"`
			Status    string `json:"status"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
			Changes   int    `json:"changes"`
		} `json:"files"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github compare response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github compare refs failed (%d): %s", resp.StatusCode, payload.Message)
	}

	result := &CompareRefsResult{
		HTMLURL:      payload.HTMLURL,
		Status:       payload.Status,
		AheadBy:      payload.AheadBy,
		BehindBy:     payload.BehindBy,
		TotalCommits: payload.TotalCommits,
		Commits:      make([]CompareCommit, 0, len(payload.Commits)),
		Files:        make([]CompareFile, 0, len(payload.Files)),
	}
	for _, commit := range payload.Commits {
		item := CompareCommit{
			SHA:        commit.SHA,
			Message:    commit.Commit.Message,
			HTMLURL:    commit.HTMLURL,
			AuthorName: commit.Commit.Author.Name,
		}
		if commit.Author != nil {
			item.AuthorLogin = commit.Author.Login
		}
		result.Commits = append(result.Commits, item)
	}
	for _, file := range payload.Files {
		result.Files = append(result.Files, CompareFile{
			Filename:  file.Filename,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Changes:   file.Changes,
		})
	}
	return result, nil
}

func decodeGitHubRelease(id int64, tagName, name, htmlURL, targetCommitish string, draft, prerelease bool, publishedAt *string) *Release {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return nil
	}
	release := &Release{
		ID:              id,
		TagName:         tagName,
		Name:            strings.TrimSpace(name),
		HTMLURL:         strings.TrimSpace(htmlURL),
		TargetCommitish: strings.TrimSpace(targetCommitish),
		Draft:           draft,
		Prerelease:      prerelease,
	}
	if publishedAt != nil && strings.TrimSpace(*publishedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*publishedAt)); err == nil {
			release.PublishedAt = &parsed
		}
	}
	return release
}

// MergeBranch merges the head branch into the base branch using the GitHub REST API.
func (c *Client) MergeBranch(ctx context.Context, installationID, owner, repo, base, head, commitMessage string) error {
	if c == nil {
		return fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]string{
		"base":           base,
		"head":           head,
		"commit_message": commitMessage,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/merges", c.apiBaseURL, owner, repo),
		strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build github merge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request github merge: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		// Base already contains head — nothing to merge
		return nil
	}
	if resp.StatusCode == 201 {
		// Merge commit created successfully
		return nil
	}

	var payload struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)

	if resp.StatusCode == 409 {
		return fmt.Errorf("merge conflict: %s", payload.Message)
	}
	return fmt.Errorf("github merge failed (%d): %s", resp.StatusCode, payload.Message)
}

// GetPullRequest returns current pull request state from GitHub.
func (c *Client) GetPullRequest(ctx context.Context, installationID, owner, repo string, number int) (*PullRequest, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.apiBaseURL, owner, repo, number), nil)
	if err != nil {
		return nil, fmt.Errorf("build github pull request request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github pull request: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Number    int        `json:"number"`
		Title     string     `json:"title"`
		HTMLURL   string     `json:"html_url"`
		State     string     `json:"state"`
		Merged    bool       `json:"merged"`
		UpdatedAt *time.Time `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
		Head      struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github pull request response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github pull request lookup failed (%d): %s", resp.StatusCode, payload.Message)
	}
	return &PullRequest{
		Number:    payload.Number,
		Title:     payload.Title,
		HTMLURL:   payload.HTMLURL,
		State:     payload.State,
		Merged:    payload.Merged,
		HeadRef:   payload.Head.Ref,
		BaseRef:   payload.Base.Ref,
		HeadSHA:   payload.Head.SHA,
		UpdatedAt: payload.UpdatedAt,
		MergedAt:  payload.MergedAt,
	}, nil
}

// GetPullRequestFiles returns changed files for a pull request.
func (c *Client) GetPullRequestFiles(ctx context.Context, installationID, owner, repo string, number int) ([]PullRequestFile, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?per_page=100", c.apiBaseURL, owner, repo, number), nil)
	if err != nil {
		return nil, fmt.Errorf("build github pull request files request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github pull request files: %w", err)
	}
	defer resp.Body.Close()

	var payload []struct {
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Changes   int    `json:"changes"`
		Patch     string `json:"patch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github pull request files response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github pull request files lookup failed (%d)", resp.StatusCode)
	}
	files := make([]PullRequestFile, 0, len(payload))
	for _, file := range payload {
		files = append(files, PullRequestFile{
			Filename:  file.Filename,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Changes:   file.Changes,
			Patch:     file.Patch,
		})
	}
	return files, nil
}

// GetCheckRun returns a check run with output and annotations.
func (c *Client) GetCheckRun(ctx context.Context, installationID, owner, repo string, checkRunID int64) (*CheckRun, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/check-runs/%d", c.apiBaseURL, owner, repo, checkRunID), nil)
	if err != nil {
		return nil, fmt.Errorf("build github check run request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github check run: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		HTMLURL     string     `json:"html_url"`
		Status      string     `json:"status"`
		Conclusion  string     `json:"conclusion"`
		StartedAt   *time.Time `json:"started_at"`
		CompletedAt *time.Time `json:"completed_at"`
		Output      struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Text    string `json:"text"`
		} `json:"output"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github check run response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github check run lookup failed (%d): %s", resp.StatusCode, payload.Message)
	}

	annotations, err := c.getCheckRunAnnotations(ctx, token, owner, repo, checkRunID)
	if err != nil {
		return nil, err
	}
	return &CheckRun{
		ID:            payload.ID,
		Name:          payload.Name,
		HTMLURL:       payload.HTMLURL,
		Status:        payload.Status,
		Conclusion:    payload.Conclusion,
		StartedAt:     payload.StartedAt,
		CompletedAt:   payload.CompletedAt,
		OutputTitle:   payload.Output.Title,
		OutputSummary: payload.Output.Summary,
		OutputText:    payload.Output.Text,
		Annotations:   annotations,
	}, nil
}

func (c *Client) getCheckRunAnnotations(ctx context.Context, token, owner, repo string, checkRunID int64) ([]CheckRunAnnotation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/check-runs/%d/annotations?per_page=50", c.apiBaseURL, owner, repo, checkRunID), nil)
	if err != nil {
		return nil, fmt.Errorf("build github check run annotations request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github check run annotations: %w", err)
	}
	defer resp.Body.Close()

	var payload []struct {
		Path            string `json:"path"`
		StartLine       int    `json:"start_line"`
		EndLine         int    `json:"end_line"`
		AnnotationLevel string `json:"annotation_level"`
		Message         string `json:"message"`
		Title           string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github check run annotations response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github check run annotations lookup failed (%d)", resp.StatusCode)
	}
	annotations := make([]CheckRunAnnotation, 0, len(payload))
	for _, annotation := range payload {
		annotations = append(annotations, CheckRunAnnotation{
			Path:            annotation.Path,
			StartLine:       annotation.StartLine,
			EndLine:         annotation.EndLine,
			AnnotationLevel: annotation.AnnotationLevel,
			Message:         annotation.Message,
			Title:           annotation.Title,
		})
	}
	return annotations, nil
}

func (c *Client) createAppJWT() (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    c.appID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	})
	signed, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}
	return signed, nil
}

func parsePrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	privateKeyPEM = normalizePrivateKeyPEM(privateKeyPEM)

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid github app private key")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse github app private key: %w", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github app private key must be RSA, got %T", keyAny)
	}
	return key, nil
}

func normalizePrivateKeyPEM(privateKeyValue string) string {
	privateKeyValue = strings.ReplaceAll(strings.TrimSpace(privateKeyValue), `\n`, "\n")
	if privateKeyValue == "" || strings.Contains(privateKeyValue, "BEGIN ") {
		return privateKeyValue
	}

	decoded, ok := decodeBase64PrivateKey(privateKeyValue)
	if !ok {
		return privateKeyValue
	}

	decodedValue := strings.ReplaceAll(strings.TrimSpace(string(decoded)), `\n`, "\n")
	if strings.Contains(decodedValue, "BEGIN ") {
		return decodedValue
	}
	return privateKeyValue
}

func decodeBase64PrivateKey(privateKeyValue string) ([]byte, bool) {
	compact := strings.Join(strings.Fields(privateKeyValue), "")
	if compact == "" {
		return nil, false
	}

	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(compact)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

// InstallationIDString normalizes a numeric installation ID for storage.
func InstallationIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}
