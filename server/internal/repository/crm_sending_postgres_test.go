package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestSendingPostgresConcurrentClaims requires a disposable PostgreSQL database.
func TestSendingPostgresConcurrentClaims(t *testing.T) {
	dsn := os.Getenv("CRM_SENDING_TEST_DSN")
	if dsn == "" {
		t.Skip("CRM_SENDING_TEST_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&model.CRMMailboxSendingPolicy{}, &model.CRMEmailSendReservation{}); err != nil {
		t.Fatal(err)
	}
	r := NewCRMEmailRepository(db)
	now := time.Now().UTC()
	a := &model.CRMEmailAccount{ID: "test-account", WorkspaceID: "test-workspace", Provider: "gmail", EmailAddress: fmt.Sprintf("concurrent-%d@example.test", now.UnixNano())}
	defer db.Where("mailbox_key = ?", mailboxKey(a)).Delete(&model.CRMEmailSendReservation{})
	defer db.Where("mailbox_key = ?", mailboxKey(a)).Delete(&model.CRMMailboxSendingPolicy{})
	if err = r.UpdateSendingPolicy(context.Background(), a, model.CRMMailboxSendingPolicy{DailyLimit: 5, ManualReserve: 1, MinIntervalSeconds: 60}); err != nil {
		t.Fatal(err)
	}
	var accepted atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			err := r.ReserveSend(context.Background(), a, model.CRMEmailSendReservation{ID: fmt.Sprintf("%s-%d", a.EmailAddress, i)}, 0, now)
			if err == nil {
				accepted.Add(1)
				return
			}
			var capacity *SendCapacityError
			if !errors.As(err, &capacity) {
				t.Errorf("unexpected reservation error: %v", err)
			}
		}(i)
	}
	group.Wait()
	if accepted.Load() != 5 {
		t.Fatalf("concurrent claims bypassed shared budget: %d accepted", accepted.Load())
	}
}
