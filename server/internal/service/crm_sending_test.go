package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	gmailsync "github.com/helpin-ai/helpin/server/internal/sync"
)

func TestCRMSendingProviderCooldownClassification(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		err      error
		wait     time.Duration
		rejected bool
	}{
		{"retry after", &gmailsync.GmailAPIError{StatusCode: 429, RetryAfter: "120"}, 2 * time.Minute, true},
		{"daily quota", &gmailsync.GmailAPIError{StatusCode: 403, Body: `{"reason":"dailyLimitExceeded"}`}, 24 * time.Hour, true},
		{"rate quota", &gmailsync.GmailAPIError{StatusCode: 403, Body: `{"reason":"userRateLimitExceeded"}`}, 15 * time.Minute, true},
		{"permission", &gmailsync.GmailAPIError{StatusCode: 403, Body: "forbidden"}, 0, true},
		{"request timeout uncertainty", &gmailsync.GmailAPIError{StatusCode: 408}, 0, false},
		{"server uncertainty", &gmailsync.GmailAPIError{StatusCode: 503}, 0, false},
		{"network uncertainty", errors.New("connection reset"), 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			until, limited := providerCooldown(tc.err, now)
			if limited != (tc.wait > 0) || limited && !until.Equal(now.Add(tc.wait)) {
				t.Fatalf("unexpected cooldown %v %v", until, limited)
			}
			if definitiveRejection(tc.err) != tc.rejected {
				t.Fatal("unsafe rejection classification")
			}
		})
	}
}
func TestCRMSendingThrottlePersistsAndUnknownAttemptRemainsReserved(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	repo := repository.NewCRMEmailRepository(db)
	s := &CRMEmailService{emailRepo: repo}
	a := &model.CRMEmailAccount{ID: "a", WorkspaceID: "w", Provider: "gmail", EmailAddress: "owner@example.test"}
	ctx := context.Background()
	id, err := s.reserveEmail(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The request context may expire during the provider call; cleanup must still persist.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	err = s.finishEmailAttempt(canceled, a, id, &gmailsync.GmailAPIError{StatusCode: 429, RetryAfter: "120"}, false)
	var capacity *repository.SendCapacityError
	if !errors.As(err, &capacity) {
		t.Fatalf("throttle not safely deferred: %v", err)
	}
	if _, err = s.reserveEmail(ctx, a, nil); !errors.As(err, &capacity) {
		t.Fatalf("manual email bypassed cooldown: %v", err)
	}
	var reservation model.CRMEmailSendReservation
	if err = db.First(&reservation, "id = ?", id).Error; err != nil || reservation.Status != "rejected" {
		t.Fatalf("rejected attempt retained quota: %+v %v", reservation, err)
	}
	b := *a
	b.EmailAddress = "second@example.test"
	id, err = s.reserveEmail(ctx, &b, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.finishEmailAttempt(ctx, &b, id, errors.New("connection reset"), false)
	reservation = model.CRMEmailSendReservation{}
	err = db.First(&reservation, "id = ?", id).Error
	if err != nil || reservation.Status != "pending" {
		t.Fatalf("uncertain attempt freed quota: %+v %v", reservation, err)
	}
}

func TestCRMSendingRejectionPersistenceFailureStaysUncertain(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	s := &CRMEmailService{emailRepo: repository.NewCRMEmailRepository(db)}
	a := &model.CRMEmailAccount{ID: "a", WorkspaceID: "w", Provider: "gmail", EmailAddress: "owner@example.test"}
	ctx := context.Background()
	id, err := s.reserveEmail(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	persistErr := errors.New("reservation storage unavailable")
	if err := db.Callback().Update().Before("gorm:update").Register("reject_status_write", func(tx *gorm.DB) {
		if tx.Statement.Table == "crm_email_send_reservations" {
			tx.AddError(persistErr)
		}
	}); err != nil {
		t.Fatal(err)
	}
	err = s.finishEmailAttempt(ctx, a, id, &gmailsync.GmailAPIError{StatusCode: 403, Body: "forbidden"}, false)
	if !errors.Is(err, persistErr) || definitiveRejection(err) {
		t.Fatalf("failed persistence must prevent retry classification: %v", err)
	}
	var reservation model.CRMEmailSendReservation
	if err := db.First(&reservation, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if reservation.Status != "pending" {
		t.Fatalf("uncertain reservation released: %s", reservation.Status)
	}
}
