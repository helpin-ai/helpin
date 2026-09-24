package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	agentRuntimeName                   = "agent-runtime"
	agentRuntimeExecutionModeDurable   = "durable"
	agentRuntimeTurnCompleteOnFinish   = agentruntime.TurnPolicyCompleteOnFinish
	agentRuntimeTurnPauseAfterAssist   = agentruntime.TurnPolicyPauseAfterAssist
	agentRuntimeTurnCompletionExplicit = agentruntime.TurnCompletionExplicit
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
type AgentRuntimeToolCall = agentruntime.ToolCall
type AgentRuntimeEventEnvelope = agentruntime.EventEnvelope
type AgentRuntimeEventListResponse = agentruntime.EventListResponse

// AgentRuntimeClient is Helpin's host-side client for delegated Agent Runtime
// runs. It uses public /v1 routes plus the negotiated /v2 event projection;
// /internal runtime routes are not a host contract.
type AgentRuntimeClient struct {
	client        *agentruntime.Client
	baseURL       string
	appID         string
	serviceToken  string
	eventProtocol string
	httpClient    *http.Client
}

func NewAgentRuntimeClient(baseURL, appID, token string, httpClient *http.Client, eventProtocol ...string) (*AgentRuntimeClient, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, fmt.Errorf("agent runtime app ID is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	protocol := ""
	if len(eventProtocol) > 0 {
		protocol = strings.ToLower(strings.TrimSpace(eventProtocol[0]))
	}
	options := []agentruntime.ClientOption{
		agentruntime.WithAppID(appID),
		agentruntime.WithServiceToken(token),
		agentruntime.WithHTTPClient(httpClient),
	}
	if protocol != "" {
		options = append(options, agentruntime.WithEventProtocol(protocol))
	}
	client, err := agentruntime.NewClient(
		baseURL,
		options...,
	)
	if err != nil {
		return nil, err
	}
	return &AgentRuntimeClient{
		client:        client,
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		appID:         strings.TrimSpace(appID),
		serviceToken:  strings.TrimSpace(token),
		eventProtocol: protocol,
		httpClient:    httpClient,
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

// ListV2EventPage opts Helpin into the Runtime's bounded replay extension
// without changing the shared SDK or the default behavior of other apps.
func (c *AgentRuntimeClient) ListV2EventPage(ctx context.Context, runtimeRunID string, afterSequence int64, pageSize int) (*AgentRuntimeEventListResponse, error) {
	if c == nil || c.httpClient == nil || c.baseURL == "" {
		return nil, fmt.Errorf("agent runtime client is not configured")
	}
	if pageSize <= 0 {
		return nil, fmt.Errorf("agent runtime v2 event page size must be positive")
	}
	query := url.Values{}
	query.Set("app_id", c.AppID())
	query.Set("after_sequence", strconv.FormatInt(max(afterSequence, 0), 10))
	query.Set("page_size", strconv.Itoa(pageSize))
	path := "/v2/runs/" + url.PathEscape(strings.TrimSpace(runtimeRunID)) + "/events"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if c.serviceToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.serviceToken)
	}
	if c.eventProtocol != "" {
		request.Header.Set(agentruntime.EventProtocolHeader, c.eventProtocol)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call agent runtime: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, &agentruntime.HTTPStatusError{
			Method:     http.MethodGet,
			Path:       path,
			StatusCode: response.StatusCode,
			Body:       strings.TrimSpace(string(message)),
		}
	}
	var result AgentRuntimeEventListResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode agent runtime response: %w", err)
	}
	return &result, nil
}

func (c *AgentRuntimeClient) ListArtifacts(ctx context.Context, runtimeRunID string) ([]AgentRuntimeArtifact, error) {
	return c.client.ListArtifacts(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ListInteractions(ctx context.Context, runtimeRunID string) ([]AgentRuntimeInteraction, error) {
	return c.client.ListInteractions(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ListToolCalls(ctx context.Context, runtimeRunID string) ([]AgentRuntimeToolCall, error) {
	return c.client.ListToolCalls(ctx, runtimeRunID)
}

func (c *AgentRuntimeClient) ResumeRun(ctx context.Context, runtimeRunID string, req AgentRuntimeResumeRunRequest) (*AgentRuntimeRun, error) {
	if strings.TrimSpace(req.ExternalActorID) != "" && strings.TrimSpace(req.Intent) != model.AgentRunResumeIntentAuthCompleted {
		return c.ResumeRunWithProvenance(ctx, runtimeRunID, req, "human")
	}
	return c.client.ResumeRun(ctx, runtimeRunID, req)
}

func (c *AgentRuntimeClient) ApproveRun(ctx context.Context, runtimeRunID string, externalActorID ...string) (*AgentRuntimeRun, error) {
	return c.ResumeRun(ctx, runtimeRunID, AgentRuntimeResumeRunRequest{
		Intent: model.AgentRunResumeIntentApprove, ExternalActorID: firstOptionalString(externalActorID),
	})
}

func (c *AgentRuntimeClient) RequestChanges(ctx context.Context, runtimeRunID, content string, externalActorID ...string) (*AgentRuntimeRun, error) {
	return c.ResumeRun(ctx, runtimeRunID, AgentRuntimeResumeRunRequest{
		Intent: model.AgentRunResumeIntentRequestChanges, Content: content, ExternalActorID: firstOptionalString(externalActorID),
	})
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

// PauseRun requests a manual pause; the runtime acknowledges it with a later event.
func (c *AgentRuntimeClient) PauseRun(ctx context.Context, runtimeRunID string) (*AgentRuntimeRun, error) {
	if c == nil || c.httpClient == nil || c.baseURL == "" {
		return nil, fmt.Errorf("agent runtime client is not configured")
	}
	query := url.Values{}
	query.Set("app_id", c.AppID())
	path := "/v1/runs/" + url.PathEscape(strings.TrimSpace(runtimeRunID)) + "/pause"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if c.serviceToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.serviceToken)
	}
	if c.eventProtocol != "" {
		request.Header.Set(agentruntime.EventProtocolHeader, c.eventProtocol)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call agent runtime: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, &agentruntime.HTTPStatusError{Method: http.MethodPost, Path: path, StatusCode: response.StatusCode, Body: strings.TrimSpace(string(message))}
	}
	var run AgentRuntimeRun
	if err := json.NewDecoder(response.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode agent runtime response: %w", err)
	}
	return &run, nil
}

func firstOptionalString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
