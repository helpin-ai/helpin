package repository

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestCommandBarPlanLockIsExclusiveOnPostgres runs against a disposable
// PostgreSQL (COMMAND_BAR_TEST_POSTGRES_DSN): concurrent advancement of one
// plan is serialized, a waiter gives up after its wait, and other plans are
// not blocked.
func TestCommandBarPlanLockIsExclusiveOnPostgres(t *testing.T) {
	dsn := os.Getenv("COMMAND_BAR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set COMMAND_BAR_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	repo := NewCommandBarPlanRunGroupRepository(db)
	ctx := context.Background()

	var active, maxActive atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			acquired, err := repo.WithPlanLock(ctx, "plan-a", 10*time.Second, func() error {
				n := active.Add(1)
				for {
					current := maxActive.Load()
					if n <= current || maxActive.CompareAndSwap(current, n) {
						break
					}
				}
				time.Sleep(30 * time.Millisecond)
				active.Add(-1)
				return nil
			})
			if err != nil || !acquired {
				t.Errorf("lock acquired=%v err=%v", acquired, err)
			}
		}()
	}
	wg.Wait()
	if maxActive.Load() != 1 {
		t.Fatalf("%d holders ran at once", maxActive.Load())
	}

	held := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = repo.WithPlanLock(ctx, "plan-b", time.Second, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	acquired, err := repo.WithPlanLock(ctx, "plan-b", 200*time.Millisecond, func() error {
		t.Error("ran while another holder had the lock")
		return nil
	})
	if err != nil || acquired {
		t.Fatalf("contended lock acquired=%v err=%v", acquired, err)
	}
	if acquired, err := repo.WithPlanLock(ctx, "plan-c", 0, func() error { return nil }); err != nil || !acquired {
		t.Fatalf("another plan was blocked: acquired=%v err=%v", acquired, err)
	}
	close(release)
}
