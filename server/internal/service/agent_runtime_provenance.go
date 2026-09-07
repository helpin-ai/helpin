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

	agentruntime "github.com/helpin-ai/agent-runtime-go"
)

// ResumeRunWithProvenance sends the additive origin contract before the next
// published SDK version is available. The embedded request keeps the existing
// public resume shape; no local module replacement or SDK fork is required.
// Deploy the provenance-capable Runtime before enabling host notifications.
func (c *AgentRuntimeClient) ResumeRunWithProvenance(ctx context.Context, runID string, req AgentRuntimeResumeRunRequest, provenance string) (*AgentRuntimeRun, error) {
	provenance = strings.TrimSpace(provenance)
	if provenance != "human" && provenance != "system_notification" {
		return nil, fmt.Errorf("invalid message provenance")
	}
	if provenance == "human" && strings.TrimSpace(req.ExternalActorID) == "" {
		return nil, fmt.Errorf("human messages require external_actor_id")
	}
	if provenance == "system_notification" && (strings.TrimSpace(req.Intent) != "reply" || strings.TrimSpace(req.InteractionID) != "" || len(req.ResponsePayload) > 0) {
		return nil, fmt.Errorf("system notifications require reply intent without an interaction response")
	}
	body, err := json.Marshal(struct {
		AgentRuntimeResumeRunRequest
		MessageProvenance string `json:"message_provenance"`
	}{AgentRuntimeResumeRunRequest: req, MessageProvenance: provenance})
	if err != nil {
		return nil, err
	}
	path := "/v1/runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/resume"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path+"?"+url.Values{"app_id": {c.AppID()}}.Encode(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if c.eventProtocol != "" {
		request.Header.Set(agentruntime.EventProtocolHeader, c.eventProtocol)
	}
	if c.serviceToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.serviceToken)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, err := io.ReadAll(io.LimitReader(response.Body, 8192))
		if err != nil {
			return nil, err
		}
		return nil, &agentruntime.HTTPStatusError{Method: http.MethodPost, Path: path, StatusCode: response.StatusCode, Body: string(body)}
	}
	var run AgentRuntimeRun
	if err := json.NewDecoder(response.Body).Decode(&run); err != nil {
		return nil, err
	}
	return &run, nil
}
