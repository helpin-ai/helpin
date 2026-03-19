package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/worker"
)

func TestExecutePlanningTurnWithInitialOutputTimeout(t *testing.T) {
	t.Parallel()

	_, err := executePlanningTurnWithInitialOutputTimeout(
		context.Background(),
		10*time.Millisecond,
		func(ctx context.Context, onEvent func(worker.ExecutionEvent)) (*worker.ExecutionResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		nil,
	)
	if !errors.Is(err, worker.ErrInitialResponseTimeout) {
		t.Fatalf("expected initial response timeout, got %v", err)
	}
}

func TestExecutePlanningTurnWithInitialOutputTimeoutStopsAfterProgress(t *testing.T) {
	t.Parallel()

	result, err := executePlanningTurnWithInitialOutputTimeout(
		context.Background(),
		50*time.Millisecond,
		func(ctx context.Context, onEvent func(worker.ExecutionEvent)) (*worker.ExecutionResult, error) {
			onEvent(worker.ExecutionEvent{Type: "assistant_message_started"})
			<-time.After(20 * time.Millisecond)
			return &worker.ExecutionResult{AssistantText: "draft ready"}, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result == nil || result.AssistantText != "draft ready" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
