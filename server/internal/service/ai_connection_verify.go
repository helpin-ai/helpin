package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	aiConnectionTestTimeout   = 20 * time.Second
	aiConnectionTestMaxTokens = 16
	aiConnectionTestPrompt    = "Reply with OK."
)

// AIConnectionChangeObserver reacts after a connection is created, reconnected,
// or disconnected. Errors are logged; the connection change has already committed.
type AIConnectionChangeObserver interface {
	AIConnectionsChanged(ctx context.Context, workspace string) error
}

// AIConnectionTestProviderFactory builds the chat client used by a connection
// test from a resolved credential. Tests replace it with a fake provider.
type AIConnectionTestProviderFactory func(provider string, credential sdk.ModelCredential) (llm.Provider, error)

// SetChangeObserver registers the hook that re-maps derived AI defaults.
func (s *AIConnectionService) SetChangeObserver(observer AIConnectionChangeObserver) *AIConnectionService {
	s.observer = observer
	return s
}

// SetTestProviderFactory replaces the provider clients used by TestConnection.
func (s *AIConnectionService) SetTestProviderFactory(factory AIConnectionTestProviderFactory) *AIConnectionService {
	s.testProviders = factory
	return s
}

// notifyConnectionsChanged only reports shared connections: personal
// connections never back workspace standard profiles.
func (s *AIConnectionService) notifyConnectionsChanged(ctx context.Context, c *model.AIConnection) {
	if s.observer == nil || c == nil || c.Scope != "workspace" || c.UserID != nil {
		return
	}
	if err := s.observer.AIConnectionsChanged(ctx, c.WorkspaceID); err != nil {
		slog.ErrorContext(ctx, "re-map standard AI profiles after connection change failed",
			"error", err, "workspace_id", c.WorkspaceID, "connection_id", c.ID, "provider", c.Provider)
	}
}

// TestConnection makes one minimal completion with the connection's own
// credential and records the outcome. The call uses the customer's key directly,
// not a metered gateway: platform-funded (managed) connections cannot be tested,
// so no allowance is consumed in either edition.
func (s *AIConnectionService) TestConnection(ctx context.Context, workspace, user, id, requestedModel string) (*model.AIConnectionTestResult, error) {
	if !s.Enabled() {
		return nil, ErrAIConnection
	}
	if err := s.authorizeConnectionManagement(ctx, workspace, user, id); err != nil {
		return nil, err
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if len(requestedModel) > 200 || strings.ContainsAny(requestedModel, "\r\n") {
		return nil, errors.New("invalid test model")
	}
	stored, err := s.repo.Get(ctx, id)
	if err != nil || stored == nil {
		return nil, ErrAIConnection
	}
	switch stored.Provider {
	case "openai", "anthropic", "openrouter":
	default:
		return nil, errors.New("connection testing is not supported for this provider")
	}
	connection, credential, err := s.Credential(ctx, workspace, user, id, false)
	if errors.Is(err, ErrAIConnectionUnavailable) {
		result := &model.AIConnectionTestResult{Error: "Reconnect this connection before testing it."}
		return result, s.recordTestResult(ctx, id, result)
	}
	if err != nil {
		return nil, err
	}
	modelName := requestedModel
	if modelName == "" {
		modelName, err = s.connectionTestModel(ctx, workspace, connection)
		if err != nil {
			return nil, err
		}
	}
	factory := s.testProviders
	if factory == nil {
		factory = defaultAIConnectionTestProvider
	}
	provider, err := factory(connection.Provider, *credential)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, aiConnectionTestTimeout)
	defer cancel()
	started := time.Now()
	_, callErr := provider.ChatCompletion(callCtx, llm.ChatRequest{
		Provider: connection.Provider, Model: modelName, MaxTokens: aiConnectionTestMaxTokens,
		Messages: []llm.Message{{Role: "user", Content: aiConnectionTestPrompt}},
	})
	result := &model.AIConnectionTestResult{OK: callErr == nil, Model: modelName, LatencyMS: time.Since(started).Milliseconds()}
	if callErr != nil {
		result.Error = sanitizeAIConnectionTestError(callErr)
		slog.WarnContext(ctx, "AI connection test failed", "workspace_id", workspace, "connection_id", id,
			"provider", connection.Provider, "model", modelName, "reason", result.Error)
	} else {
		slog.InfoContext(ctx, "AI connection test succeeded", "workspace_id", workspace, "connection_id", id,
			"provider", connection.Provider, "model", modelName, "latency_ms", result.LatencyMS)
	}
	return result, s.recordTestResult(ctx, id, result)
}

func (s *AIConnectionService) recordTestResult(ctx context.Context, id string, result *model.AIConnectionTestResult) error {
	var failure *string
	if !result.OK {
		message := result.Error
		failure = &message
	}
	return s.repo.RecordVerification(ctx, id, time.Now().UTC(), failure)
}

// connectionTestModel prefers the workspace default profile's model when that
// profile runs on this connection, then the provider's smallest catalog model.
func (s *AIConnectionService) connectionTestModel(ctx context.Context, workspace string, c *model.AIConnection) (string, error) {
	profile, err := s.repo.DefaultProfile(ctx, workspace)
	if err != nil {
		return "", err
	}
	if profile != nil && profile.Primary.ConnectionID == c.ID && strings.TrimSpace(profile.Primary.Model.Model) != "" {
		return strings.TrimSpace(profile.Primary.Model.Model), nil
	}
	catalog := s.catalog
	if catalog == nil {
		if resolver := loadDefaultAgentModelTierResolver(); resolver != nil {
			catalog = resolver.catalog
		}
	}
	route, ok := smallestCatalogRoute(catalog, c.Provider)
	if !ok {
		return "", errors.New("select a model to test this connection")
	}
	return route.Route, nil
}

func defaultAIConnectionTestProvider(provider string, credential sdk.ModelCredential) (llm.Provider, error) {
	client, err := defaultAIChatClient(sdk.RunModel{Provider: provider}, credential)
	if err != nil {
		return nil, errors.New("connection testing is not supported for this provider")
	}
	return client, nil
}

var aiConnectionTestStatusPattern = regexp.MustCompile(`status (\d{3})`)

// sanitizeAIConnectionTestError maps provider failures to fixed messages so a
// response body (which may echo request data) never reaches the client.
func sanitizeAIConnectionTestError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "The provider did not respond in time."
	}
	status := 0
	var providerErr *llm.ProviderError
	if errors.As(err, &providerErr) && providerErr.StatusCode != 0 {
		status = providerErr.StatusCode
	} else if match := aiConnectionTestStatusPattern.FindStringSubmatch(err.Error()); match != nil {
		if parsed, convErr := strconv.Atoi(match[1]); convErr == nil {
			status = parsed
		}
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "The provider rejected the API key (HTTP " + strconv.Itoa(status) + ")."
	case status == http.StatusPaymentRequired:
		return "The provider account has insufficient credits or quota (HTTP 402)."
	case status == http.StatusNotFound:
		return "The test model is not available to this API key (HTTP 404)."
	case status == http.StatusTooManyRequests:
		return "The provider rate-limited the test. Try again shortly (HTTP 429)."
	case status >= 500:
		return "The provider is temporarily unavailable (HTTP " + strconv.Itoa(status) + ")."
	case status >= 400:
		return "The provider rejected the test request (HTTP " + strconv.Itoa(status) + ")."
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "The provider did not respond in time."
		}
		return "Could not reach the provider."
	}
	return "The provider test failed."
}
