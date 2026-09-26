package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func sendingFixture(t *testing.T) (*CRMEmailRepository, *model.CRMEmailAccount, time.Time) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sending-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&model.CRMMailboxSendingPolicy{}, &model.CRMEmailSendReservation{}, &model.CRMEmailSequence{}); err != nil {
		t.Fatal(err)
	}
	return NewCRMEmailRepository(db), &model.CRMEmailAccount{ID: "a", WorkspaceID: "w", EmailAddress: "Owner@Example.com", Provider: "gmail"}, time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
}
func TestSendingSharedBudgetAndManualReserve(t *testing.T) {
	r, a, now := sendingFixture(t)
	ctx := context.Background()
	if err := r.UpdateSendingPolicy(ctx, a, model.CRMMailboxSendingPolicy{DailyLimit: 3, ManualReserve: 1, MinIntervalSeconds: 60}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: fmt.Sprint(i), Automated: true}, 0, now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	same := *a
	same.ID = "other-workspace-account"
	same.WorkspaceID = "other"
	same.EmailAddress = "owner@example.com"
	var blocked *SendCapacityError
	if err := r.ReserveSend(ctx, &same, model.CRMEmailSendReservation{ID: "blocked", Automated: true}, 0, now.Add(2*time.Minute)); !errors.As(err, &blocked) {
		t.Fatalf("reserve bypassed: %v", err)
	}
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "manual"}, 0, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "extra"}, 0, now.Add(3*time.Minute)); !errors.As(err, &blocked) {
		t.Fatal("daily budget bypassed")
	}
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "tomorrow", Automated: true}, 0, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
}
func TestSendingUncertainReservationNeverRepeats(t *testing.T) {
	r, a, now := sendingFixture(t)
	ctx := context.Background()
	request := model.CRMEmailSendReservation{ID: "intent"}
	if err := r.ReserveSend(ctx, a, request, 0, now); err != nil {
		t.Fatal(err)
	}
	if err := r.ReserveSend(ctx, a, request, 0, now.Add(time.Hour)); !errors.Is(err, ErrSendAlreadyReserved) {
		t.Fatalf("duplicate send allowed: %v", err)
	}
	if err := r.FinishSend(ctx, request.ID, "rejected"); err != nil {
		t.Fatal(err)
	}
	if err := r.ReserveSend(ctx, a, request, 0, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}
func TestSendingCooldownAndNewStarts(t *testing.T) {
	r, a, now := sendingFixture(t)
	ctx := context.Background()
	if err := r.db.Create(&model.CRMEmailSequence{ID: "s", WorkspaceID: "w", OwnerID: "o"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "first", SequenceID: "s", FirstEmail: true, Automated: true}, 1, now); err != nil {
		t.Fatal(err)
	}
	var blocked *SendCapacityError
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "second", SequenceID: "s", FirstEmail: true, Automated: true}, 1, now.Add(time.Hour)); !errors.As(err, &blocked) || blocked.Reason != "daily_starts" {
		t.Fatalf("starts cap: %v", err)
	}
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "followup", SequenceID: "s", Automated: true}, 1, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := r.CooldownMailbox(ctx, a, now.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := r.ReserveSend(ctx, a, model.CRMEmailSendReservation{ID: "cooldown"}, 0, now.Add(2*time.Hour)); !errors.As(err, &blocked) || blocked.Reason != "provider_cooldown" {
		t.Fatalf("cooldown: %v", err)
	}
}

func TestSendingRejectedRetryCountsInCurrentWindow(t *testing.T) {
	r, a, now := sendingFixture(t)
	ctx := context.Background()
	request := model.CRMEmailSendReservation{ID: "retry-intent"}
	if err := r.ReserveSend(ctx, a, request, 0, now); err != nil {
		t.Fatal(err)
	}
	if err := r.FinishSend(ctx, request.ID, "rejected"); err != nil {
		t.Fatal(err)
	}
	retryAt := now.Add(25 * time.Hour)
	if err := r.ReserveSend(ctx, a, request, 0, retryAt); err != nil {
		t.Fatal(err)
	}
	capacity, err := r.SendingCapacity(ctx, a, retryAt)
	if err != nil {
		t.Fatal(err)
	}
	if capacity.Used != 1 {
		t.Fatalf("retried send escaped current quota window: used=%d", capacity.Used)
	}
}
