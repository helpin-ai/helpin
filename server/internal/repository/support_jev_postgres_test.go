//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportJevPostgresConcurrentAdmission(t *testing.T) {
	for _, scenario := range []string{"duplicate", "shared cap", "abandoned"} {
		t.Run(scenario, func(t *testing.T) {
			env := setupPMTriagePostgres(t)
			if err := env.db.AutoMigrate(&model.SupportConversationTriageEvent{}); err != nil {
				t.Fatal(err)
			}
			repo := NewSupportJevRepository(env.db)
			conversation := uuid.NewString()
			ctx := context.Background()
			var abandonedID string
			if scenario == "abandoned" {
				first, err := repo.Reserve(ctx, env.workspace, conversation, "hash", 3)
				if err != nil {
					t.Fatal(err)
				}
				abandonedID = first.Event.ID
				if err := env.db.Model(first.Event).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
					t.Fatal(err)
				}
			}
			var wg sync.WaitGroup
			var mu sync.Mutex
			calls, limited := 0, 0
			for i := range 12 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					source, hash := conversation, "hash"
					if scenario == "shared cap" {
						source, hash = uuid.NewString(), fmt.Sprint(i)
					}
					admission, err := repo.Reserve(ctx, env.workspace, source, hash, 3)
					if err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					defer mu.Unlock()
					if admission.CallProvider {
						calls++
					}
					if admission.Limited {
						limited++
					}
				}(i)
			}
			wg.Wait()
			wantCalls, wantLimited := 1, 0
			if scenario == "shared cap" {
				wantCalls, wantLimited = 3, 9
			}
			if calls != wantCalls || limited != wantLimited {
				t.Fatalf("admission calls=%d limited=%d; want %d/%d", calls, limited, wantCalls, wantLimited)
			}
			if abandonedID != "" {
				if err := repo.Finish(ctx, env.workspace, abandonedID, json.RawMessage(`{"status":"ok"}`)); err == nil {
					t.Fatal("late completion settled abandoned attempt")
				}
			}
		})
	}
}
