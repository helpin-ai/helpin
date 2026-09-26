//go:build integration

package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSupportSendPostgresConcurrentClaims(t *testing.T) {
	dsn := os.Getenv("SUPPORT_SEND_TEST_DSN")
	if dsn == "" {
		t.Skip("SUPPORT_SEND_TEST_DSN required")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "send_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { base.Exec("DROP SCHEMA " + schema + " CASCADE"); pool, _ := base.DB(); pool.Close() })
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool, _ := db.DB(); pool.Close() })
	if err := db.AutoMigrate(&model.SupportPendingSend{}); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../dbmigrate/sql/202609240012_support_pending_send_order.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	ws := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for c := 0; c < 10; c++ {
		conv := uuid.NewString()
		for n := 0; n < 4; n++ {
			job := model.SupportPendingSend{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv, UserID: uuid.NewString(), Status: "queued", CreatedAt: now.Add(time.Duration(n) * time.Second), UpdatedAt: now}
			if err := db.Create(&job).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	svc := &SupportInboxService{messageRepo: repository.NewSupportMessageRepository(db)}
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := map[string]string{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job, err := svc.claimPendingSupportSend(context.Background())
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if prior := claimed[job.ConversationID]; prior != "" {
				t.Errorf("same conversation claimed twice: %s, %s", prior, job.ID)
			}
			claimed[job.ConversationID] = job.ID
			if !job.CreatedAt.Equal(now.Truncate(time.Microsecond)) {
				t.Errorf("claim skipped first reply: %v", job.CreatedAt)
			}
		}()
	}
	wg.Wait()
	if len(claimed) != 10 {
		t.Fatalf("claimed %d conversations, want all 10", len(claimed))
	}
}
