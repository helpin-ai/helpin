package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNotificationEmailLimitPostgresConcurrency(t *testing.T) {
	dsn := os.Getenv("NOTIFICATION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set NOTIFICATION_TEST_POSTGRES_DSN to an isolated test database")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := admin.DB()
	defer sqlDB.Close()
	schema := fmt.Sprintf("notification_limit_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	pool.SetMaxOpenConns(4)
	for _, sql := range []string{
		"CREATE TABLE notifications (id text PRIMARY KEY, recipient_id text, workspace_id text, entity_type text, entity_id text)",
		"CREATE TABLE notification_events (id text PRIMARY KEY, notification_id text)",
		"CREATE TABLE notification_deliveries (id text PRIMARY KEY, notification_event_id text, channel text, status text, delivered_at timestamptz)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name     string
		repeated bool
		want     int32
	}{{"across items", false, 5}, {"same item", true, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, table := range []string{"notification_deliveries", "notification_events", "notifications"} {
				if err := db.Exec("DELETE FROM " + table).Error; err != nil {
					t.Fatal(err)
				}
			}
			var sent atomic.Int32
			var wg sync.WaitGroup
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			errors := make(chan error, 12)
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					id := fmt.Sprintf("item-%d", i)
					if tc.repeated {
						id = "same"
					}
					repo := NewNotificationRepository(db)
					errors <- repo.WithRecipientEmailLock(ctx, "user", func(locked *NotificationRepository) error {
						allowed, err := locked.CanSendIndividualEmail(ctx, "user", "ws", "task", id, time.Now())
						if err != nil || !allowed {
							return err
						}
						// Also exercise an independent repository query while other senders compete for the lock.
						if err := db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
							return err
						}
						key := fmt.Sprintf("row-%d", i)
						if err := locked.db.Exec("INSERT INTO notifications VALUES (?, 'user','ws','task',?)", key, id).Error; err != nil {
							return err
						}
						if err := locked.db.Exec("INSERT INTO notification_events VALUES (?,?)", key, key).Error; err != nil {
							return err
						}
						if err := locked.db.Exec("INSERT INTO notification_deliveries VALUES (?,?,'email','delivered',?)", key, key, time.Now()).Error; err != nil {
							return err
						}
						sent.Add(1)
						return nil
					})
				}(i)
			}
			wg.Wait()
			close(errors)
			for err := range errors {
				if err != nil {
					t.Fatal(err)
				}
			}
			if sent.Load() != tc.want {
				t.Fatalf("sent %d, want %d", sent.Load(), tc.want)
			}
		})
	}
}
