package temporalapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestLoadPlanningSessionWithRetryEventuallySucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	session, err := loadPlanningSessionWithRetry(
		context.Background(),
		"session-1",
		50*time.Millisecond,
		time.Millisecond,
		func(ctx context.Context, sessionID string) (*model.PlanningSession, error) {
			attempts++
			if attempts < 3 {
				return nil, nil
			}
			return &model.PlanningSession{ID: sessionID}, nil
		},
	)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if session == nil || session.ID != "session-1" {
		t.Fatalf("unexpected session: %#v", session)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestLoadPlanningSessionWithRetryReturnsUnderlyingError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database unavailable")
	_, err := loadPlanningSessionWithRetry(
		context.Background(),
		"session-1",
		5*time.Millisecond,
		time.Millisecond,
		func(ctx context.Context, sessionID string) (*model.PlanningSession, error) {
			return nil, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}
