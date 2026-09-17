package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const supportDecisionQuestion = "Which inbox should handle this customer message?"

// SupportDecisionPolicy binds a reviewed acceptance threshold to an exact deployment and candidate set.
type SupportDecisionPolicy struct {
	WorkspaceID           string    `json:"workspace_id"`
	DeploymentFingerprint string    `json:"deployment_fingerprint"`
	ChoicesHash           string    `json:"choices_hash"`
	Question              string    `json:"question"`
	MinConfidence         float64   `json:"min_confidence"`
	ExpiresAt             time.Time `json:"expires_at"`
}

// SupportDecisionConfig controls the optional local-first triage experiment.
type SupportDecisionConfig struct {
	Mode         string
	URL          string
	Token        string
	Timeout      time.Duration
	WorkspaceIDs []string
	Policies     []SupportDecisionPolicy
}

// SupportDecisionClient calls the local non-generative ranker with a bounded deadline.
type SupportDecisionClient struct {
	config     SupportDecisionConfig
	http       *http.Client
	workspaces map[string]bool
	policies   map[string]SupportDecisionPolicy
}

type supportDecisionChoice struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type supportDecisionResponse struct {
	Choice                *string `json:"choice"`
	Confidence            float64 `json:"confidence"`
	ConfidenceKind        string  `json:"confidence_kind"`
	Abstained             bool    `json:"abstained"`
	DeploymentFingerprint string  `json:"deployment_fingerprint"`
	Scores                []struct {
		ID          string  `json:"id"`
		Probability float64 `json:"probability"`
	} `json:"scores"`
	QueryTruncated *bool `json:"query_truncated"`
}

// NewSupportDecisionClient validates configuration; off leaves existing routing untouched.
func NewSupportDecisionClient(config SupportDecisionConfig) (*SupportDecisionClient, error) {
	if config.Mode == "" || config.Mode == "off" {
		return nil, nil
	}
	if config.Mode != "shadow" && config.Mode != "primary" {
		return nil, errors.New("decision mode must be off, shadow or primary")
	}
	u, err := url.Parse(config.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid decision service URL")
	}
	if config.Token == "" || len(config.WorkspaceIDs) == 0 {
		return nil, errors.New("decision token and explicit workspace allowlist required")
	}
	if config.Timeout <= 0 || config.Timeout > 2*time.Second {
		return nil, errors.New("decision deadline must be positive and at most two seconds")
	}
	c := &SupportDecisionClient{config: config, http: &http.Client{Timeout: config.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, workspaces: map[string]bool{}, policies: map[string]SupportDecisionPolicy{}}
	for _, id := range config.WorkspaceIDs {
		if id = strings.TrimSpace(id); id != "" {
			c.workspaces[id] = true
		}
	}
	if len(c.workspaces) == 0 {
		return nil, errors.New("decision workspace allowlist is empty")
	}
	for _, p := range config.Policies {
		if !c.workspaces[p.WorkspaceID] || len(p.DeploymentFingerprint) != 64 || len(p.ChoicesHash) != 64 || p.Question != supportDecisionQuestion || math.IsNaN(p.MinConfidence) || p.MinConfidence <= 0 || p.MinConfidence > 1 || !p.ExpiresAt.After(time.Now()) {
			return nil, errors.New("invalid or expired decision acceptance policy")
		}
		if _, exists := c.policies[p.WorkspaceID]; exists {
			return nil, errors.New("duplicate decision workspace policy")
		}
		c.policies[p.WorkspaceID] = p
	}
	if config.Mode == "primary" && len(c.policies) == 0 {
		return nil, errors.New("primary decision routing requires a reviewed acceptance policy")
	}
	return c, nil
}

func (c *SupportDecisionClient) enabled(workspace string) bool {
	return c != nil && c.workspaces[workspace]
}

func decisionChoices(options []supportTriageMailboxOption) ([]supportDecisionChoice, string) {
	choices := make([]supportDecisionChoice, 0, len(options))
	for _, option := range options {
		choices = append(choices, supportDecisionChoice{ID: option.Handle, Description: option.Prompt})
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].ID < choices[j].ID })
	// Length-prefixed fields avoid separator collisions and are independent of JSON escaping.
	var canonical strings.Builder
	for _, choice := range choices {
		fmt.Fprintf(&canonical, "%d:%s%d:%s", len(choice.ID), choice.ID, len(choice.Description), choice.Description)
	}
	digest := sha256.Sum256([]byte(canonical.String()))
	return choices, hex.EncodeToString(digest[:])
}

func (c *SupportDecisionClient) decide(ctx context.Context, workspace, input string, options []supportTriageMailboxOption) (*supportDecisionResponse, bool, string, error) {
	choices, choicesHash := decisionChoices(options)
	if len(choices) < 2 || len(choices) > 255 || len(input) > 16000 {
		return nil, false, "input_limits", nil
	}
	payload, err := json.Marshal(struct {
		Context  string                  `json:"context"`
		Question string                  `json:"question"`
		Choices  []supportDecisionChoice `json:"choices"`
	}{input, supportDecisionQuestion, choices})
	if err != nil {
		return nil, false, "encode_error", err
	}
	callCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(c.config.URL, "/")+"/v1/decide", bytes.NewReader(payload))
	if err != nil {
		return nil, false, "request_error", err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, "unavailable", errors.New("local decision request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, "service_error", fmt.Errorf("local decision status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		return nil, false, "invalid_response", errors.New("local decision body invalid")
	}
	var result supportDecisionResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, false, "invalid_response", errors.New("local decision JSON invalid")
	}
	if result.Choice == nil || result.Abstained {
		return &result, false, "abstained", nil
	}
	expected := map[string]bool{}
	for _, choice := range choices {
		expected[choice.ID] = true
	}
	if !expected[*result.Choice] || len(result.Scores) != len(expected) || math.IsNaN(result.Confidence) || result.Confidence < 0 || result.Confidence > 1 {
		return nil, false, "invalid_scores", errors.New("local decision scores invalid")
	}
	sum, maxP, selected := 0.0, 0.0, -1.0
	for _, score := range result.Scores {
		if !expected[score.ID] || math.IsNaN(score.Probability) || score.Probability < 0 || score.Probability > 1 {
			return nil, false, "invalid_scores", errors.New("local decision probabilities invalid")
		}
		delete(expected, score.ID)
		sum += score.Probability
		maxP = math.Max(maxP, score.Probability)
		if score.ID == *result.Choice {
			selected = score.Probability
		}
	}
	if math.Abs(sum-1) > 1e-5 || math.Abs(selected-maxP) > 1e-5 || math.Abs(selected-result.Confidence) > 1e-5 {
		return nil, false, "invalid_scores", errors.New("local decision probabilities inconsistent")
	}
	if c.config.Mode == "shadow" {
		return &result, false, "shadow", nil
	}
	policy, ok := c.policies[workspace]
	if !ok || !policy.ExpiresAt.After(time.Now()) || policy.ChoicesHash != choicesHash || policy.DeploymentFingerprint != result.DeploymentFingerprint {
		return &result, false, "policy_mismatch", nil
	}
	if result.ConfidenceKind != "temperature_scaled" || result.QueryTruncated == nil || *result.QueryTruncated {
		return &result, false, "uncalibrated_or_truncated", nil
	}
	if result.Confidence < policy.MinConfidence {
		return &result, false, "low_confidence", nil
	}
	return &result, true, "accepted", nil
}
