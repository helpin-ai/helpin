package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const agentRuntimeName = "agent-runtime"

// AgentRuntimeClient is Helpin's host-side client for delegated Agent Runtime runs.
// It intentionally uses only /v1 routes; /internal runtime routes are not a host contract.
type AgentRuntimeClient struct {
	baseURL    string
	appID      string
	token      string
	httpClient *http.Client
}

type AgentRuntimeTargetRef struct {
	Type     string         `json:"type"`
	ID       string         `json:"id"`
	Display  map[string]any `json:"display,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type AgentRuntimeStartRunRequest struct {
	AppID           string                `json:"app_id"`
	HostRunID       string                `json:"host_run_id,omitempty"`
	AgentID         string                `json:"agent_id"`
	Target          AgentRuntimeTargetRef `json:"target"`
	Instructions    string                `json:"instructions,omitempty"`
	AllowedTools    []string              `json:"allowed_tools,omitempty"`
	ExternalActorID string                `json:"external_actor_id,omitempty"`
	Mode            string                `json:"mode,omitempty"`
	ExecutionMode   string                `json:"execution_mode,omitempty"`
	Trigger         map[string]any        `json:"trigger,omitempty"`
	Metadata        map[string]any        `json:"metadata,omitempty"`
	TurnPolicy      map[string]any        `json:"turn_policy,omitempty"`
}

type AgentRuntimeResumeRunRequest struct {
	Intent          string          `json:"intent"`
	Content         string          `json:"content,omitempty"`
	ResponsePayload json.RawMessage `json:"response_payload,omitempty"`
	ExternalActorID string          `json:"external_actor_id,omitempty"`
}

type AgentRuntimeRun struct {
	ID              string          `json:"id"`
	AppID           string          `json:"app_id"`
	HostRunID       string          `json:"host_run_id,omitempty"`
	AgentID         string          `json:"agent_id"`
	Target          json.RawMessage `json:"target"`
	RuntimeKind     string          `json:"runtime_kind"`
	ExecutionMode   string          `json:"execution_mode"`
	InvocationMode  string          `json:"invocation_mode"`
	Status          string          `json:"status"`
	PauseReason     string          `json:"pause_reason,omitempty"`
	ApprovalState   string          `json:"approval_state,omitempty"`
	Input           json.RawMessage `json:"input,omitempty"`
	OutputSummary   json.RawMessage `json:"output_summary,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	ExternalActorID string          `json:"external_actor_id,omitempty"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at,omitempty"`
}

type AgentRuntimeMessage struct {
	ID              string          `json:"id"`
	Role            string          `json:"role"`
	Content         string          `json:"content"`
	MessageType     string          `json:"message_type"`
	ContentBlocks   json.RawMessage `json:"content_blocks,omitempty"`
	ToolInvocations json.RawMessage `json:"tool_invocations,omitempty"`
	SequenceNo      int             `json:"sequence_no"`
	CreatedAt       time.Time       `json:"created_at,omitempty"`
}

type AgentRuntimeArtifact struct {
	ID            string          `json:"id"`
	ArtifactType  string          `json:"artifact_type"`
	Format        string          `json:"format"`
	StorageMode   string          `json:"storage_mode"`
	InlineContent string          `json:"inline_content,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	SequenceNo    int             `json:"sequence_no"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
}

type AgentRuntimeInteraction struct {
	ID                   string          `json:"id"`
	RuntimeKind          string          `json:"runtime_kind"`
	InteractionKind      string          `json:"interaction_kind"`
	Status               string          `json:"status"`
	Title                string          `json:"title,omitempty"`
	Summary              string          `json:"summary,omitempty"`
	RequestPayload       json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload      json.RawMessage `json:"response_payload,omitempty"`
	ResolvedByExternalID string          `json:"resolved_by_external_id,omitempty"`
	ResolvedAt           *time.Time      `json:"resolved_at,omitempty"`
	CreatedAt            time.Time       `json:"created_at,omitempty"`
	UpdatedAt            time.Time       `json:"updated_at,omitempty"`
}

func NewAgentRuntimeClient(baseURL, appID, token string, httpClient *http.Client) (*AgentRuntimeClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("agent runtime base URL is required")
	}
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("agent runtime app ID is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &AgentRuntimeClient{
		baseURL:    baseURL,
		appID:      appID,
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}, nil
}

func (c *AgentRuntimeClient) StartRun(ctx context.Context, req AgentRuntimeStartRunRequest) (*AgentRuntimeRun, error) {
	if strings.TrimSpace(req.AppID) == "" {
		req.AppID = c.appID
	}
	var run AgentRuntimeRun
	if err := c.doJSON(ctx, http.MethodPost, "/v1/runs", nil, req, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *AgentRuntimeClient) GetRun(ctx context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	var run AgentRuntimeRun
	if err := c.doJSON(ctx, http.MethodGet, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID)), agentRuntimeAppQuery(c.appID), nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *AgentRuntimeClient) ListMessages(ctx context.Context, runtimeRunID string) ([]AgentRuntimeMessage, error) {
	var messages []AgentRuntimeMessage
	if err := c.doJSON(ctx, http.MethodGet, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID))+"/messages", agentRuntimeAppQuery(c.appID), nil, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (c *AgentRuntimeClient) ListArtifacts(ctx context.Context, runtimeRunID string) ([]AgentRuntimeArtifact, error) {
	var artifacts []AgentRuntimeArtifact
	if err := c.doJSON(ctx, http.MethodGet, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID))+"/artifacts", agentRuntimeAppQuery(c.appID), nil, &artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (c *AgentRuntimeClient) ListInteractions(ctx context.Context, runtimeRunID string) ([]AgentRuntimeInteraction, error) {
	var interactions []AgentRuntimeInteraction
	if err := c.doJSON(ctx, http.MethodGet, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID))+"/interactions", agentRuntimeAppQuery(c.appID), nil, &interactions); err != nil {
		return nil, err
	}
	return interactions, nil
}

func (c *AgentRuntimeClient) ResumeRun(ctx context.Context, runtimeRunID string, req AgentRuntimeResumeRunRequest) (*AgentRuntimeRun, error) {
	var run AgentRuntimeRun
	if err := c.doJSON(ctx, http.MethodPost, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID))+"/resume", agentRuntimeAppQuery(c.appID), req, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *AgentRuntimeClient) ApproveRun(ctx context.Context, runtimeRunID string, externalActorID ...string) (*AgentRuntimeRun, error) {
	return c.ResumeRun(ctx, runtimeRunID, AgentRuntimeResumeRunRequest{
		Intent:          "approve",
		ExternalActorID: firstOptionalString(externalActorID),
	})
}

func (c *AgentRuntimeClient) RequestChanges(ctx context.Context, runtimeRunID, content string, externalActorID ...string) (*AgentRuntimeRun, error) {
	return c.ResumeRun(ctx, runtimeRunID, AgentRuntimeResumeRunRequest{
		Intent:          "request_changes",
		Content:         content,
		ExternalActorID: firstOptionalString(externalActorID),
	})
}

func (c *AgentRuntimeClient) AppendMessage(ctx context.Context, runtimeRunID, role, content string, externalActorID ...string) error {
	body := map[string]string{"role": role, "content": content}
	if actorID := firstOptionalString(externalActorID); actorID != "" {
		body["external_actor_id"] = actorID
	}
	return c.doJSON(ctx, http.MethodPost, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID))+"/messages", agentRuntimeAppQuery(c.appID), body, nil)
}

func (c *AgentRuntimeClient) CancelRun(ctx context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	var run AgentRuntimeRun
	if err := c.doJSON(ctx, http.MethodPost, "/v1/runs/"+url.PathEscape(strings.TrimSpace(runtimeRunID))+"/cancel", agentRuntimeAppQuery(c.appID), nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (c *AgentRuntimeClient) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if c == nil {
		return fmt.Errorf("agent runtime client is not configured")
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode agent runtime request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call agent runtime: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("agent runtime %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode agent runtime response: %w", err)
	}
	return nil
}

func agentRuntimeAppQuery(appID string) url.Values {
	values := url.Values{}
	values.Set("app_id", strings.TrimSpace(appID))
	return values
}

func firstOptionalString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
