package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	agentRuntimeName                 = "agent-runtime"
	agentRuntimeExecutionModeDurable = "durable"
)

type AgentRuntimeTargetRef = agentruntime.TargetRef
type AgentRuntimeAgent = agentruntime.Agent
type AgentRuntimeSkillRef = agentruntime.SkillRef
type AgentRuntimeStartRunRequest = agentruntime.StartRunRequest
type AgentRuntimeTurnPolicy = agentruntime.TurnPolicy
type AgentRuntimeResumeRunRequest = agentruntime.ResumeRunRequest
type AgentRuntimeRun = agentruntime.AgentRun
type AgentRuntimeMessage = agentruntime.AgentRunMessage
type AgentRuntimeArtifact = agentruntime.AgentRunArtifact
type AgentRuntimeInteraction = agentruntime.AgentRunInteraction
type AgentRuntimeEventEnvelope = agentruntime.EventEnvelope

// AgentRuntimeClient is Helpin's host-side client for delegated Agent Runtime
// runs. It intentionally uses only /v1 routes; /internal runtime routes are not
// a host contract.
type AgentRuntimeClient struct {
	client       *agentruntime.Client
	baseURL      string
	appID        string
	serviceToken string
	httpClient   *http.Client
}

func NewAgentRuntimeClient(baseURL, appID, token string, httpClient *http.Client) (*AgentRuntimeClient, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, fmt.Errorf("agent runtime app ID is required")
	}
	client, err := agentruntime.NewClient(
		baseURL,
		agentruntime.WithAppID(appID),
		agentruntime.WithServiceToken(token),
		agentruntime.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &AgentRuntimeClient{
		client:       client,
		baseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		appID:        strings.TrimSpace(appID),
		serviceToken: strings.TrimSpace(token),
		httpClient:   httpClient,
	}, nil
}

func (c *AgentRuntimeClient) AppID() string {
	if c == nil {
		return ""
	}
	if c.appID != "" {
		return c.appID
	}
	if c.client == nil {
		return ""
	}
	return c.client.AppID()
}

func (c *AgentRuntimeClient) UpsertAgent(ctx context.Context, agent AgentRuntimeAgent) (*AgentRuntimeAgent, error) {
	if strings.TrimSpace(agent.AppID) == "" {
		agent.AppID = c.AppID()
	}
	return c.client.UpsertAgent(ctx, agent)
}

func (c *AgentRuntimeClient) StartRun(ctx context.Context, req AgentRuntimeStartRunRequest) (*AgentRuntimeRun, error) {
	return c.client.StartRun(ctx, req)
}

func (c *AgentRuntimeClient) GetRun(ctx context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	return c.client.GetRun(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ListMessages(ctx context.Context, runtimeRunID string) ([]AgentRuntimeMessage, error) {
	return c.client.ListMessages(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ListArtifacts(ctx context.Context, runtimeRunID string) ([]AgentRuntimeArtifact, error) {
	return c.client.ListArtifacts(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ListInteractions(ctx context.Context, runtimeRunID string) ([]AgentRuntimeInteraction, error) {
	return c.client.ListInteractions(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ResumeRun(ctx context.Context, runtimeRunID string, req AgentRuntimeResumeRunRequest) (*AgentRuntimeRun, error) {
	return c.client.ResumeRun(ctx, runtimeRunID, req)
}

func (c *AgentRuntimeClient) ApproveRun(ctx context.Context, runtimeRunID string, externalActorID ...string) (*AgentRuntimeRun, error) {
	return c.client.ApproveRun(ctx, runtimeRunID, externalActorID...)
}

func (c *AgentRuntimeClient) RequestChanges(ctx context.Context, runtimeRunID, content string, externalActorID ...string) (*AgentRuntimeRun, error) {
	return c.client.RequestChanges(ctx, runtimeRunID, content, externalActorID...)
}

func (c *AgentRuntimeClient) AppendMessage(ctx context.Context, runtimeRunID, role, content string, externalActorID ...string) error {
	_, err := c.client.AppendMessage(ctx, runtimeRunID, agentruntime.AppendMessageRequest{
		Role:            strings.TrimSpace(role),
		Content:         content,
		ExternalActorID: firstOptionalString(externalActorID),
	})
	return err
}

func (c *AgentRuntimeClient) CancelRun(ctx context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	return c.client.CancelRun(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) StartCodexDeviceCodeAuth(ctx context.Context, runtimeRunID string) (*model.CodexAuthState, error) {
	return c.codexDeviceCodeAuth(ctx, runtimeRunID, "start")
}

func (c *AgentRuntimeClient) CancelCodexDeviceCodeAuth(ctx context.Context, runtimeRunID string) (*model.CodexAuthState, error) {
	return c.codexDeviceCodeAuth(ctx, runtimeRunID, "cancel")
}

func (c *AgentRuntimeClient) codexDeviceCodeAuth(ctx context.Context, runtimeRunID, action string) (*model.CodexAuthState, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("agent runtime client is not configured")
	}
	endpoint := fmt.Sprintf("%s/v1/runs/%s/codex-auth/device-code/%s?app_id=%s",
		c.baseURL,
		url.PathEscape(strings.TrimSpace(runtimeRunID)),
		url.PathEscape(strings.TrimSpace(action)),
		url.QueryEscape(c.AppID()),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if c.serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.serviceToken)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf("agent runtime POST %s returned %d: %s", req.URL.Path, res.StatusCode, strings.TrimSpace(string(body)))
	}
	var state model.CodexAuthState
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func firstOptionalString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
