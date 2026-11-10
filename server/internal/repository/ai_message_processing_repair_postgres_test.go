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

func TestValidationRepairReservationOnPostgres(t *testing.T) {
	dsn := os.Getenv("SUPPORT_REPAIR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SUPPORT_REPAIR_TEST_POSTGRES_DSN to an empty disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := db.Exec(`CREATE TABLE ai_message_processing (
		id text PRIMARY KEY, workspace_id text, status text, reply_message_id text,
		validation_repair_used boolean NOT NULL DEFAULT false, updated_at timestamptz
	)`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DROP TABLE ai_message_processing").Error; err != nil {
			t.Error(err)
		}
	})
	if err := db.Exec("INSERT INTO ai_message_processing (id, workspace_id, status) VALUES ('turn', 'workspace', 'processing')").Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := NewAIMessageProcessingRepository(db)
	if allowed, err := repo.ClaimValidationRepair(ctx, "other-workspace", "turn"); err != nil || allowed {
		t.Fatalf("cross-workspace allowance: %v, %v", allowed, err)
	}
	var granted atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			allowed, err := repo.ClaimValidationRepair(ctx, "workspace", "turn")
			if err != nil {
				t.Error(err)
			}
			if allowed {
				granted.Add(1)
			}
		})
	}
	wg.Wait()
	if granted.Load() != 1 {
		t.Fatalf("granted %d corrections, want one", granted.Load())
	}
	if allowed, err := NewAIMessageProcessingRepository(db).ClaimValidationRepair(ctx, "workspace", "turn"); err != nil || allowed {
		t.Fatalf("allowance did not survive repository restart: %v, %v", allowed, err)
	}
}
