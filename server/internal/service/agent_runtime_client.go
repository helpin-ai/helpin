package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	agentRuntimeName                 = "agent-runtime"
	agentRuntimeExecutionModeDurable = "durable"
	agentRuntimeTurnCompleteOnFinish = agentruntime.TurnPolicyCompleteOnFinish
	agentRuntimeTurnPauseAfterAssist = agentruntime.TurnPolicyPauseAfterAssist
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
type AgentRuntimeEventListResponse = agentruntime.EventListResponse

// AgentRuntimeClient is Helpin's host-side client for delegated Agent Runtime
// runs. It intentionally uses only /v1 routes; /internal runtime routes are not
// a host contract.
type AgentRuntimeClient struct {
	client *agentruntime.Client
	appID  string
}

func NewAgentRuntimeClient(baseURL, appID, token string, httpClient *http.Client, eventProtocol ...string) (*AgentRuntimeClient, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, fmt.Errorf("agent runtime app ID is required")
	}
	options := []agentruntime.ClientOption{
		agentruntime.WithAppID(appID),
		agentruntime.WithServiceToken(token),
		agentruntime.WithHTTPClient(httpClient),
	}
	if len(eventProtocol) > 0 {
		options = append(options, agentruntime.WithEventProtocol(eventProtocol[0]))
	}
	client, err := agentruntime.NewClient(
		baseURL,
		options...,
	)
	if err != nil {
		return nil, err
	}
	return &AgentRuntimeClient{client: client, appID: strings.TrimSpace(appID)}, nil
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

// UpdateRunMCPCredential rotates only an already-attached server credential;
// the runtime rejects URL or tool-policy changes through this endpoint.
func (c *AgentRuntimeClient) UpdateRunMCPCredential(ctx context.Context, runtimeRunID, serverID string, credential ExternalMCPRunCredential) error {
	_, err := c.client.UpdateRunMCPCredential(ctx, runtimeRunID, serverID, agentruntime.UpdateRunMCPCredentialRequest{Credential: credential})
	return err
}

func (c *AgentRuntimeClient) GetRun(ctx context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	return c.client.GetRun(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ListMessages(ctx context.Context, runtimeRunID string) ([]AgentRuntimeMessage, error) {
	return c.client.ListMessages(ctx, runtimeRunID)
}

// ListV2Events reads the durable ordered event log. NATS remains the low-
// latency path; this endpoint lets the projection repair a missed delivery or
// a consumer restart without reconstructing provider-specific output.
func (c *AgentRuntimeClient) ListV2Events(ctx context.Context, runtimeRunID string, afterSequence int64) (*AgentRuntimeEventListResponse, error) {
	return c.client.ListV2Events(ctx, runtimeRunID, afterSequence)
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
	state, err := c.client.StartCodexDeviceCodeAuth(ctx, runtimeRunID)
	if err != nil {
		return nil, err
	}
	return modelCodexAuthStateFromRuntime(state), nil
}

func (c *AgentRuntimeClient) CancelCodexDeviceCodeAuth(ctx context.Context, runtimeRunID string) (*model.CodexAuthState, error) {
	state, err := c.client.CancelCodexDeviceCodeAuth(ctx, runtimeRunID)
	if err != nil {
		return nil, err
	}
	return modelCodexAuthStateFromRuntime(state), nil
}

func modelCodexAuthStateFromRuntime(state *agentruntime.CodexAuthState) *model.CodexAuthState {
	if state == nil {
		return nil
	}
	return &model.CodexAuthState{
		Provider:        state.Provider,
		AuthMode:        state.AuthMode,
		State:           state.State,
		LoginID:         state.LoginID,
		AuthURL:         state.AuthURL,
		VerificationURL: state.VerificationURL,
		UserCode:        state.UserCode,
		PlanType:        state.PlanType,
		Error:           state.Error,
		UpdatedAt:       state.UpdatedAt,
	}
}

func firstOptionalString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
