package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultShortcutAPIBaseURL = "https://api.app.shortcut.com/api/v3"
	shortcutRateLimit         = 200 // requests per minute
)

var shortcutAPIBaseURL = defaultShortcutAPIBaseURL
var shortcutHTTPClientFactory = func() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

type ShortcutAPIClient struct {
	token   string
	baseURL string
	client  *http.Client
	limiter *rate.Limiter
}

func NewShortcutAPIClient(token string) *ShortcutAPIClient {
	return NewShortcutAPIClientWithBaseURL(token, shortcutAPIBaseURL)
}

func NewShortcutAPIClientWithBaseURL(token, baseURL string) *ShortcutAPIClient {
	return &ShortcutAPIClient{
		token:   token,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  shortcutHTTPClientFactory(),
		// 200 req/min ≈ 3.33 req/sec, burst of 10 for parallel fetches
		limiter: rate.NewLimiter(rate.Every(time.Minute/shortcutRateLimit), 10),
	}
}

// --- API response types ---

type shortcutAPILabel struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type shortcutAPIIteration struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	StartDate   string             `json:"start_date"`
	EndDate     string             `json:"end_date"`
	Status      string             `json:"status"`
	GroupIDs    []string           `json:"group_ids"`
	Labels      []shortcutAPILabel `json:"labels"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
}

type shortcutAPIWorkflowState struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Position int    `json:"position"`
}

type shortcutAPIWorkflow struct {
	ID     int                        `json:"id"`
	Name   string                     `json:"name"`
	States []shortcutAPIWorkflowState `json:"states"`
}

type shortcutAPIEpic struct {
	ID               int                `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Archived         bool               `json:"archived"`
	Started          bool               `json:"started"`
	Completed        bool               `json:"completed"`
	StartedAt        string             `json:"started_at"`
	CompletedAt      string             `json:"completed_at"`
	PlannedStartDate string             `json:"planned_start_date"`
	Deadline         string             `json:"deadline"`
	CreatedAt        string             `json:"created_at"`
	UpdatedAt        string             `json:"updated_at"`
	ObjectiveIDs     []int              `json:"objective_ids"`
	GroupID          string             `json:"group_id"`
	GroupIDs         []string           `json:"group_ids"`
	OwnerIDs         []string           `json:"owner_ids"`
	Labels           []shortcutAPILabel `json:"labels"`
}

type shortcutAPIObjective struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Archived    bool               `json:"archived"`
	Started     bool               `json:"started"`
	Completed   bool               `json:"completed"`
	StartedAt   string             `json:"started_at"`
	CompletedAt string             `json:"completed_at"`
	State       string             `json:"state"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
	Labels      []shortcutAPILabel `json:"labels"`
}

type shortcutAPIMemberProfile struct {
	EmailAddress string `json:"email_address"`
	Name         string `json:"name"`
	MentionName  string `json:"mention_name"`
}

type shortcutAPIMember struct {
	ID      string                   `json:"id"`
	Profile shortcutAPIMemberProfile `json:"profile"`
	Role    string                   `json:"role"`
}

type shortcutAPIComment struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	AuthorID  string `json:"author_id"`
	ParentID  *int   `json:"parent_id"`
	Deleted   bool   `json:"deleted"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type shortcutAPIMemberInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MentionName string `json:"mention_name"`
	Workspace2  struct {
		ID                string `json:"id"`
		Name              string `json:"name"`
		URLSlug           string `json:"url_slug"`
		DefaultWorkflowID int    `json:"default_workflow_id"`
	} `json:"workspace2"`
}

type shortcutAPIGroup struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	MentionName       string   `json:"mention_name"`
	Description       string   `json:"description"`
	Archived          bool     `json:"archived"`
	DefaultWorkflowID int      `json:"default_workflow_id"`
	WorkflowIDs       []int    `json:"workflow_ids"`
	MemberIDs         []string `json:"member_ids"`
}

type shortcutAPIProject struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TeamID      int    `json:"team_id"`
	Archived    bool   `json:"archived"`
}

type shortcutAPIExternalLink struct {
	Title string
	URL   string
}

type shortcutAPIFile struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type shortcutAPILinkedFile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

type shortcutAPIStoryTask struct {
	ID          int      `json:"id"`
	Description string   `json:"description"`
	Complete    bool     `json:"complete"`
	Position    int      `json:"position"`
	OwnerIDs    []string `json:"owner_ids"`
}

type shortcutAPIStoryLink struct {
	ID        int    `json:"id"`
	ObjectID  int    `json:"object_id"`
	SubjectID int    `json:"subject_id"`
	Verb      string `json:"verb"`
}

type shortcutAPIStory struct {
	ID                  int                     `json:"id"`
	AppURL              string                  `json:"app_url"`
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	StoryType           string                  `json:"story_type"`
	Archived            bool                    `json:"archived"`
	Blocked             bool                    `json:"blocked"`
	Blocker             bool                    `json:"blocker"`
	Completed           bool                    `json:"completed"`
	CreatedAt           string                  `json:"created_at"`
	UpdatedAt           string                  `json:"updated_at"`
	StartedAt           string                  `json:"started_at"`
	CompletedAt         string                  `json:"completed_at"`
	CompletedAtOverride string                  `json:"completed_at_override"`
	MovedAt             string                  `json:"moved_at"`
	Deadline            string                  `json:"deadline"`
	Estimate            *int                    `json:"estimate"`
	EpicID              *int                    `json:"epic_id"`
	IterationID         *int                    `json:"iteration_id"`
	WorkflowID          int                     `json:"workflow_id"`
	WorkflowStateID     int                     `json:"workflow_state_id"`
	GroupID             string                  `json:"group_id"`
	ProjectID           *int                    `json:"project_id"`
	RequestedByID       string                  `json:"requested_by_id"`
	OwnerIDs            []string                `json:"owner_ids"`
	FollowerIDs         []string                `json:"follower_ids"`
	Position            int                     `json:"position"`
	Labels              []shortcutAPILabel      `json:"labels"`
	Tasks               []shortcutAPIStoryTask  `json:"tasks"`
	Comments            []shortcutAPIComment    `json:"comments"`
	StoryLinks          []shortcutAPIStoryLink  `json:"story_links"`
	Files               []shortcutAPIFile       `json:"files"`
	LinkedFiles         []shortcutAPILinkedFile `json:"linked_files"`
	ExternalLinks       []string                `json:"external_links"`
}

type shortcutAPIStorySearchResults struct {
	Data  []shortcutAPIStory `json:"data"`
	Next  string             `json:"next"`
	Total int                `json:"total"`
}

type shortcutAPIStoryQueryParams struct {
	Archived            bool   `json:"archived"`
	IncludesDescription bool   `json:"includes_description"`
	CreatedAtStart      string `json:"created_at_start,omitempty"`
	UpdatedAtStart      string `json:"updated_at_start,omitempty"`
	CreatedAtEnd        string `json:"created_at_end,omitempty"`
	UpdatedAtEnd        string `json:"updated_at_end,omitempty"`
}

type shortcutAPIStorySearchOptions struct {
	CreatedAtStart  string
	UpdatedAtStart  string
	CreatedAtEnd    string
	UpdatedAtEnd    string
	MaxStories      int
	SortField       string
	OnDetailFetched func(processed, total, storyID int)
}

// shortcutAPIEnrichment holds all bulk-fetched API data keyed by Shortcut ID (as string).
type shortcutAPIEnrichment struct {
	Labels         map[string]shortcutAPILabel
	Iterations     map[string]shortcutAPIIteration
	Epics          map[string]shortcutAPIEpic
	Objectives     map[string]shortcutAPIObjective
	Members        map[string]shortcutAPIMember // keyed by Shortcut member UUID
	MembersByEmail map[string]shortcutAPIMember // keyed by lowercase email
	Groups         map[string]shortcutAPIGroup
	Projects       map[string]shortcutAPIProject
	Workflows      map[string]shortcutAPIWorkflow
}

// --- HTTP helpers ---

func (c *ShortcutAPIClient) get(ctx context.Context, path string, result interface{}) error {
	return c.request(ctx, http.MethodGet, path, nil, result)
}

func (c *ShortcutAPIClient) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.request(ctx, http.MethodPost, path, body, result)
}

func (c *ShortcutAPIClient) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit: %w", err)
		}
		var bodyReader io.Reader
		if body != nil {
			payload, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshal Shortcut API request: %w", err)
			}
			bodyReader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+shortcutAPIPath(path), bodyReader)
		if err != nil {
			return err
		}
		req.Header.Set("Shortcut-Token", c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("shortcut api request %s: %w", path, err)
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
			_ = resp.Body.Close()
			if readErr != nil {
				return fmt.Errorf("read response: %w", readErr)
			}
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return fmt.Errorf("invalid Shortcut API token (HTTP %d)", resp.StatusCode)
			}
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				if result == nil {
					return nil
				}
				return json.Unmarshal(body, result)
			}
			if resp.StatusCode != http.StatusTooManyRequests && (resp.StatusCode < 500 || resp.StatusCode > 599) {
				snippet := string(body)
				if len(snippet) > 200 {
					snippet = snippet[:200]
				}
				return fmt.Errorf("shortcut api %s returned %d: %s", path, resp.StatusCode, snippet)
			}
			lastErr = fmt.Errorf("shortcut api %s returned %d", path, resp.StatusCode)
			wait := shortcutRetryDelay(attempt, resp.Header.Get("Retry-After"))
			if err := sleepShortcutRetry(ctx, wait); err != nil {
				return err
			}
			continue
		}
		if err := sleepShortcutRetry(ctx, shortcutRetryDelay(attempt, "")); err != nil {
			return err
		}
	}
	return lastErr
}

func shortcutAPIPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if parsed, err := url.Parse(raw); err == nil {
			return parsed.RequestURI()
		}
	}
	if strings.HasPrefix(raw, "/api/v3/") {
		return strings.TrimPrefix(raw, "/api/v3")
	}
	return raw
}

func shortcutRetryDelay(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if ts, err := http.ParseTime(retryAfter); err == nil {
			if delay := time.Until(ts); delay > 0 {
				return delay
			}
		}
	}
	delay := time.Duration(250*(1<<attempt)) * time.Millisecond
	if delay > 3*time.Second {
		return 3 * time.Second
	}
	return delay
}

func sleepShortcutRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// --- Individual endpoints ---

func (c *ShortcutAPIClient) ValidateToken(ctx context.Context) error {
	var result json.RawMessage
	return c.get(ctx, "/member", &result)
}

func (c *ShortcutAPIClient) GetCurrentMember(ctx context.Context) (*shortcutAPIMemberInfo, error) {
	var out shortcutAPIMemberInfo
	return &out, c.get(ctx, "/member", &out)
}

func (c *ShortcutAPIClient) ListLabels(ctx context.Context) ([]shortcutAPILabel, error) {
	var out []shortcutAPILabel
	return out, c.get(ctx, "/labels", &out)
}

func (c *ShortcutAPIClient) ListIterations(ctx context.Context) ([]shortcutAPIIteration, error) {
	var out []shortcutAPIIteration
	return out, c.get(ctx, "/iterations", &out)
}

func (c *ShortcutAPIClient) ListIterationStoryIDs(ctx context.Context, iterationID int) ([]int, error) {
	storyIDs := []int{}
	var out []struct {
		ID int `json:"id"`
	}
	if err := c.get(ctx, fmt.Sprintf("/iterations/%d/stories", iterationID), &out); err != nil {
		return nil, err
	}
	for _, story := range out {
		if story.ID != 0 {
			storyIDs = append(storyIDs, story.ID)
		}
	}
	return storyIDs, nil
}

func (c *ShortcutAPIClient) ListWorkflows(ctx context.Context) ([]shortcutAPIWorkflow, error) {
	var out []shortcutAPIWorkflow
	return out, c.get(ctx, "/workflows", &out)
}

func (c *ShortcutAPIClient) ListEpics(ctx context.Context) ([]shortcutAPIEpic, error) {
	var out []shortcutAPIEpic
	return out, c.get(ctx, "/epics", &out)
}

func (c *ShortcutAPIClient) ListObjectives(ctx context.Context) ([]shortcutAPIObjective, error) {
	var out []shortcutAPIObjective
	return out, c.get(ctx, "/objectives", &out)
}

func (c *ShortcutAPIClient) ListMembers(ctx context.Context) ([]shortcutAPIMember, error) {
	var out []shortcutAPIMember
	return out, c.get(ctx, "/members", &out)
}

func (c *ShortcutAPIClient) ListGroups(ctx context.Context) ([]shortcutAPIGroup, error) {
	var out []shortcutAPIGroup
	return out, c.get(ctx, "/groups", &out)
}

func (c *ShortcutAPIClient) ListProjects(ctx context.Context) ([]shortcutAPIProject, error) {
	var out []shortcutAPIProject
	return out, c.get(ctx, "/projects", &out)
}

func (c *ShortcutAPIClient) SearchStories(ctx context.Context, query, next string) (*shortcutAPIStorySearchResults, error) {
	path := "/search/stories"
	if strings.TrimSpace(next) != "" {
		if strings.HasPrefix(next, "/") || strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
			path = next
		} else {
			path = "/search/stories?" + next
		}
	} else {
		values := url.Values{}
		values.Set("query", query)
		values.Set("detail", "full")
		values.Set("page_size", "250")
		path += "?" + values.Encode()
	}
	var out shortcutAPIStorySearchResults
	return &out, c.get(ctx, path, &out)
}

func (c *ShortcutAPIClient) ListAllStories(ctx context.Context, opts shortcutAPIStorySearchOptions) ([]shortcutAPIStory, int, error) {
	archivedStories, err := c.QueryStories(ctx, true, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("query archived stories: %w", err)
	}
	activeStories, err := c.QueryStories(ctx, false, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("query active stories: %w", err)
	}
	seen := map[int]struct{}{}
	discovered := make([]shortcutAPIStory, 0, len(activeStories)+len(archivedStories))
	for _, story := range append(activeStories, archivedStories...) {
		if _, ok := seen[story.ID]; ok {
			continue
		}
		seen[story.ID] = struct{}{}
		discovered = append(discovered, story)
	}
	sortShortcutAPIStories(discovered, opts.SortField)
	if opts.MaxStories > 0 && len(discovered) > opts.MaxStories {
		discovered = discovered[:opts.MaxStories]
	}
	stories := make([]shortcutAPIStory, 0, len(discovered))
	for idx, story := range discovered {
		fullStory, err := c.GetStory(ctx, story.ID)
		if err != nil {
			return nil, len(stories), fmt.Errorf("fetch story %d: %w", story.ID, err)
		}
		stories = append(stories, *fullStory)
		if opts.OnDetailFetched != nil {
			opts.OnDetailFetched(idx+1, len(discovered), story.ID)
		}
	}
	return stories, len(stories), nil
}

func (c *ShortcutAPIClient) QueryStories(ctx context.Context, archived bool, opts shortcutAPIStorySearchOptions) ([]shortcutAPIStory, error) {
	var out []shortcutAPIStory
	err := c.post(ctx, "/stories/search", shortcutAPIStoryQueryParams{
		Archived:            archived,
		IncludesDescription: true,
		CreatedAtStart:      opts.CreatedAtStart,
		UpdatedAtStart:      opts.UpdatedAtStart,
		CreatedAtEnd:        opts.CreatedAtEnd,
		UpdatedAtEnd:        opts.UpdatedAtEnd,
	}, &out)
	return out, err
}

func sortShortcutAPIStories(stories []shortcutAPIStory, field string) {
	sort.SliceStable(stories, func(i, j int) bool {
		left := shortcutAPIStorySortTime(stories[i], field)
		right := shortcutAPIStorySortTime(stories[j], field)
		if left.Equal(right) {
			return stories[i].ID > stories[j].ID
		}
		return left.After(right)
	})
}

func shortcutAPIStorySortTime(story shortcutAPIStory, field string) time.Time {
	var raw string
	switch field {
	case "created_at":
		raw = story.CreatedAt
	default:
		raw = story.UpdatedAt
	}
	if ts := parseShortcutAPITimestamp(raw); ts != nil {
		return *ts
	}
	return time.Time{}
}

func (c *ShortcutAPIClient) GetStory(ctx context.Context, storyPublicID int) (*shortcutAPIStory, error) {
	var out shortcutAPIStory
	return &out, c.get(ctx, "/stories/"+strconv.Itoa(storyPublicID), &out)
}

func (c *ShortcutAPIClient) ListStoryComments(ctx context.Context, storyPublicID string) ([]shortcutAPIComment, error) {
	var out []shortcutAPIComment
	return out, c.get(ctx, "/stories/"+storyPublicID+"/comments", &out)
}

// --- Bulk enrichment fetch (Phase 1: ~6 requests) ---

func (c *ShortcutAPIClient) FetchEnrichment(ctx context.Context) (*shortcutAPIEnrichment, []string) {
	e := &shortcutAPIEnrichment{
		Labels:         make(map[string]shortcutAPILabel),
		Iterations:     make(map[string]shortcutAPIIteration),
		Epics:          make(map[string]shortcutAPIEpic),
		Objectives:     make(map[string]shortcutAPIObjective),
		Members:        make(map[string]shortcutAPIMember),
		MembersByEmail: make(map[string]shortcutAPIMember),
		Groups:         make(map[string]shortcutAPIGroup),
		Projects:       make(map[string]shortcutAPIProject),
		Workflows:      make(map[string]shortcutAPIWorkflow),
	}
	var warnings []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	run := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("Shortcut API: failed to fetch %s: %s", name, err.Error()))
				mu.Unlock()
			}
		}()
	}

	run("labels", func() error {
		labels, err := c.ListLabels(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, l := range labels {
			e.Labels[strconv.Itoa(l.ID)] = l
		}
		mu.Unlock()
		return nil
	})

	run("iterations", func() error {
		iterations, err := c.ListIterations(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, it := range iterations {
			e.Iterations[strconv.Itoa(it.ID)] = it
		}
		mu.Unlock()
		return nil
	})

	run("workflows", func() error {
		workflows, err := c.ListWorkflows(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, wf := range workflows {
			e.Workflows[strconv.Itoa(wf.ID)] = wf
		}
		mu.Unlock()
		return nil
	})

	run("epics", func() error {
		epics, err := c.ListEpics(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, ep := range epics {
			e.Epics[strconv.Itoa(ep.ID)] = ep
		}
		mu.Unlock()
		return nil
	})

	run("objectives", func() error {
		objectives, err := c.ListObjectives(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, o := range objectives {
			e.Objectives[strconv.Itoa(o.ID)] = o
		}
		mu.Unlock()
		return nil
	})

	run("members", func() error {
		members, err := c.ListMembers(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, m := range members {
			e.Members[m.ID] = m
			if m.Profile.EmailAddress != "" {
				e.MembersByEmail[normalizeShortcutName(m.Profile.EmailAddress)] = m
			}
		}
		mu.Unlock()
		return nil
	})

	run("groups", func() error {
		groups, err := c.ListGroups(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, g := range groups {
			e.Groups[g.ID] = g
		}
		mu.Unlock()
		return nil
	})

	run("projects", func() error {
		projects, err := c.ListProjects(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, p := range projects {
			e.Projects[strconv.Itoa(p.ID)] = p
		}
		mu.Unlock()
		return nil
	})

	wg.Wait()
	return e, warnings
}
