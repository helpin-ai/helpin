package aipolicy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrActionRequired means a feature maps to multiple actions and needs an explicit action key.
	ErrActionRequired = errors.New("AI action key is required")
	// ErrActionUnknown means the requested action is not registered.
	ErrActionUnknown = errors.New("AI action is not registered")
	// ErrRouteNotAllowed means policy does not allow the requested provider/model route.
	ErrRouteNotAllowed = errors.New("AI provider/model route is not allowed")
	// ErrInvalidExecutionContext means required audit or billing identity is missing.
	ErrInvalidExecutionContext = errors.New("AI execution context is invalid")
)

// ExecutionContext identifies one governed AI action attempt.
type ExecutionContext struct {
	WorkspaceID    string
	ActionKey      string
	FeatureKey     string
	IdempotencyKey string
	Attempt        int
	Metadata       map[string]interface{}
}

// ExecutionResult contains the mutable audit result of an AI action attempt.
type ExecutionResult struct {
	Status            string
	FailureClass      string
	FailureMessage    string
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CachedInputTokens int
	CompletedAt       time.Time
}

// ExecutionAudit persists the immutable identity and terminal result of governed attempts.
type ExecutionAudit interface {
	Start(ctx context.Context, execution *model.AIActionExecution) (*model.AIActionExecution, error)
	Finish(ctx context.Context, id string, result ExecutionResult) error
}

// ResolveExecution resolves and validates an action and its requested route.
func ResolveExecution(registry *Registry, input ExecutionContext, route Route) (Action, error) {
	if registry == nil {
		return Action{}, fmt.Errorf("%w: registry is required", ErrInvalidExecutionContext)
	}
	key := strings.TrimSpace(input.ActionKey)
	if key == "" {
		if input.FeatureKey == "coverage_gap_analysis" {
			return Action{}, ErrActionRequired
		}
		key = "feature." + strings.TrimSpace(input.FeatureKey) + ".v1"
	}
	action, ok := registry.Lookup(key)
	if !ok {
		return Action{}, fmt.Errorf("%w: %s", ErrActionUnknown, key)
	}
	if strings.TrimSpace(input.FeatureKey) != "" && input.FeatureKey != action.FeatureKey {
		return Action{}, fmt.Errorf("%w: feature does not match action", ErrInvalidExecutionContext)
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" || (action.Chargeable && strings.TrimSpace(input.WorkspaceID) == "") {
		return Action{}, ErrInvalidExecutionContext
	}
	if route.Provider == "" {
		route.Provider = action.DefaultProvider
	}
	if route.Model == "" {
		route.Model = action.DefaultModel
	}
	if !routeAllowed(action, route) {
		return Action{}, fmt.Errorf("%w: %s/%s for %s", ErrRouteNotAllowed, route.Provider, route.Model, key)
	}
	return action, nil
}
