//go:build ee

package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsageReservationSweeperKeepsActiveAndReleasesTerminalOrAbsent(t *testing.T) {
	store := &fakeReservationRecoveryStore{
		reservations: []model.AIUsageReservation{{ID: "active", ExecutionID: "run-active"}, {ID: "done", ExecutionID: "run-done"}, {ID: "absent"}},
		active:       map[string]bool{"run-active": true},
	}
	released, err := NewAIUsageReservationSweeper(store).Sweep(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if released != 2 || len(store.released) != 2 || store.released[0] != "done" || store.released[1] != "absent" {
		t.Fatalf("released = %d %#v", released, store.released)
	}
}

type fakeReservationRecoveryStore struct {
	reservations []model.AIUsageReservation
	active       map[string]bool
	released     []string
}

func (f *fakeReservationRecoveryStore) ListStaleReservations(context.Context, time.Time, int) ([]model.AIUsageReservation, error) {
	return f.reservations, nil
}
func (f *fakeReservationRecoveryStore) IsExecutionActive(_ context.Context, id string) (bool, error) {
	return f.active[id], nil
}
func (f *fakeReservationRecoveryStore) Release(_ context.Context, id, _ string) error {
	f.released = append(f.released, id)
	return nil
}
