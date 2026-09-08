package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type scheduledEventStore interface {
	ClaimNext(context.Context, time.Time, time.Duration) (*model.AutomationScheduledEvent, error)
	Retry(context.Context, model.AutomationScheduledEvent, string, bool, time.Time) error
	DeliveryFinished(context.Context, model.AutomationScheduledEvent) (bool, error)
}

// ScheduledEventHandler consumes one fenced claim. Implementations commit their
// receipt and local domain effect atomically through the scheduled-event store.
// A successful delivery is not permission for an untracked external side effect.
type ScheduledEventHandler interface {
	HandleScheduledEvent(context.Context, model.AutomationScheduledEvent, time.Time) error
}

// AutomationScheduledEventService dispatches durable events through explicitly
// registered product handlers. It is not a second agent executor or Flow builder.
type AutomationScheduledEventService struct {
	store       scheduledEventStore
	handlers    map[string]ScheduledEventHandler
	now         func() time.Time
	maintenance func(context.Context) error
}

// SetMaintenance attaches bounded product recovery to this existing worker tick.
func (s *AutomationScheduledEventService) SetMaintenance(maintenance func(context.Context) error) *AutomationScheduledEventService {
	s.maintenance = maintenance
	return s
}

// NewAutomationScheduledEventService snapshots handlers at startup; unknown kinds fail closed.
func NewAutomationScheduledEventService(store scheduledEventStore, handlers map[string]ScheduledEventHandler) *AutomationScheduledEventService {
	registered := make(map[string]ScheduledEventHandler, len(handlers))
	for kind, handler := range handlers {
		registered[kind] = handler
	}
	return &AutomationScheduledEventService{store: store, handlers: registered, now: time.Now}
}

// DispatchDue drains a bounded batch. Database leases survive worker restarts;
// per-event retry state, rather than Temporal activity attempts, bounds redelivery.
func (s *AutomationScheduledEventService) DispatchDue(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if s.maintenance != nil {
		maintenanceCtx, stop := context.WithTimeout(ctx, 10*time.Second)
		err := s.maintenance(maintenanceCtx)
		stop()
		if err != nil {
			slog.WarnContext(ctx, "scheduled work maintenance incomplete", "error", err)
		}
	}
	for range 100 {
		if err := ctx.Err(); err != nil {
			return err
		}
		event, err := s.store.ClaimNext(ctx, s.now().UTC(), 2*time.Minute)
		if err != nil {
			return fmt.Errorf("claim scheduled event: %w", err)
		}
		if event == nil {
			return nil
		}
		if event.Status == model.AutomationEventFailed {
			continue
		}
		handler := s.handlers[event.Kind]
		code, permanent := "delivery_failed", false
		if handler == nil {
			err, code, permanent = errors.New("no registered scheduled-event handler"), "unsupported_event", true
		} else {
			err = handler.HandleScheduledEvent(ctx, *event, s.now().UTC())
		}
		if err == nil {
			var finished bool
			finished, err = s.store.DeliveryFinished(ctx, *event)
			if err == nil && !finished {
				err, code = errors.New("scheduled event handler did not acknowledge delivery"), "delivery_incomplete"
			}
		}
		if err == nil || errors.Is(err, repository.ErrAutomationEventLeaseLost) {
			continue
		}
		slog.WarnContext(ctx, "scheduled event delivery failed", "event_id", event.ID,
			"workspace_id", event.WorkspaceID, "kind", event.Kind, "attempt", event.Attempts, "error", err)
		if err := s.store.Retry(ctx, *event, code, permanent, s.now().UTC()); err != nil && !errors.Is(err, repository.ErrAutomationEventLeaseLost) {
			return fmt.Errorf("record scheduled event retry: %w", err)
		}
	}
	return nil
}
