package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	shortcutAPIBaseURL = "https://api.app.shortcut.com/api/v3"
	shortcutRateLimit  = 200 // requests per minute
)

type ShortcutAPIClient struct {
	token   string
	client  *http.Client
	limiter *rate.Limiter
}

func NewShortcutAPIClient(token string) *ShortcutAPIClient {
	return &ShortcutAPIClient{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
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
	ID        int    `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Status    string `json:"status"`
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
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type shortcutAPIObjective struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
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

// shortcutAPIEnrichment holds all bulk-fetched API data keyed by Shortcut ID (as string).
type shortcutAPIEnrichment struct {
	Labels         map[string]shortcutAPILabel
	Iterations     map[string]shortcutAPIIteration
	Epics          map[string]shortcutAPIEpic
	Objectives     map[string]shortcutAPIObjective
	Members        map[string]shortcutAPIMember     // keyed by Shortcut member UUID
	MembersByEmail map[string]shortcutAPIMember     // keyed by lowercase email
}

// --- HTTP helpers ---

func (c *ShortcutAPIClient) get(ctx context.Context, path string, result interface{}) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", shortcutAPIBaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Shortcut-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("shortcut api request %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid Shortcut API token (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("shortcut api %s returned %d: %s", path, resp.StatusCode, snippet)
	}
	return json.Unmarshal(body, result)
}

// --- Individual endpoints ---

func (c *ShortcutAPIClient) ValidateToken(ctx context.Context) error {
	var result json.RawMessage
	return c.get(ctx, "/member", &result)
}

func (c *ShortcutAPIClient) ListLabels(ctx context.Context) ([]shortcutAPILabel, error) {
	var out []shortcutAPILabel
	return out, c.get(ctx, "/labels", &out)
}

func (c *ShortcutAPIClient) ListIterations(ctx context.Context) ([]shortcutAPIIteration, error) {
	var out []shortcutAPIIteration
	return out, c.get(ctx, "/iterations", &out)
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

	wg.Wait()
	return e, warnings
}
