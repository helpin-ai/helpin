package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
)

const agentRuntimeName = "agent-runtime"

type AgentRuntimeTargetRef = agentruntime.TargetRef
type AgentRuntimeStartRunRequest = agentruntime.StartRunRequest
type AgentRuntimeResumeRunRequest = agentruntime.ResumeRunRequest
type AgentRuntimeRun = agentruntime.AgentRun
type AgentRuntimeMessage = agentruntime.AgentRunMessage
type AgentRuntimeArtifact = agentruntime.AgentRunArtifact
type AgentRuntimeInteraction = agentruntime.AgentRunInteraction

// AgentRuntimeClient is Helpin's host-side client for delegated Agent Runtime
// runs. It intentionally uses only /v1 routes; /internal runtime routes are not
// a host contract.
type AgentRuntimeClient struct {
	client *agentruntime.Client
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
	return &AgentRuntimeClient{client: client}, nil
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

func firstOptionalString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
