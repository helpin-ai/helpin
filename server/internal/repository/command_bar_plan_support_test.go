package repository

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportConversationPlanQueries(t *testing.T) {
	db := setupCommandBarPlanTestDB(t)
	repo := NewCommandBarPlanRepository(db)
	ctx := context.Background()

	convID := "conv-1"
	otherConvID := "conv-2"
	seed := func(id, status string, conversationID *string, runIDs string) {
		t.Helper()
		plan := &model.CommandBarPlanRecord{
			ID:                    id,
			WorkspaceID:           "ws-1",
			SupportConversationID: conversationID,
			Status:                status,
			Prompt:                "p",
			PageContext:           []byte(`{}`),
			Steps:                 []byte(`[]`),
			RunIDsByStep:          []byte(runIDs),
		}
		if err := db.Create(plan).Error; err != nil {
			t.Fatalf("seed plan %s: %v", id, err)
		}
	}
	seed("plan-1", model.CommandBarPlanStatusRunning, &convID, `{"0":"child-run-1"}`)
	seed("plan-2", model.CommandBarPlanStatusCompleted, &convID, `{"0":"child-run-2"}`)
	seed("plan-3", model.CommandBarPlanStatusRunning, &otherConvID, `{"0":"child-run-3"}`)
	seed("plan-4", model.CommandBarPlanStatusRunning, nil, `{"0":"child-run-4"}`)

	t.Run("find by conversation and run id", func(t *testing.T) {
		plan, err := repo.FindBySupportConversationAndRunID(ctx, "ws-1", convID, "child-run-2")
		if err != nil || plan == nil || plan.ID != "plan-2" {
			t.Fatalf("FindBySupportConversationAndRunID() = %+v (err %v), want plan-2", plan, err)
		}
	})

	t.Run("run from another conversation not found", func(t *testing.T) {
		plan, err := repo.FindBySupportConversationAndRunID(ctx, "ws-1", convID, "child-run-3")
		if err != nil || plan != nil {
			t.Fatalf("FindBySupportConversationAndRunID() = %+v (err %v), want nil", plan, err)
		}
	})

	t.Run("count active only", func(t *testing.T) {
		count, err := repo.CountPlansForSupportConversation(ctx, "ws-1", convID, true)
		if err != nil || count != 1 {
			t.Fatalf("CountPlansForSupportConversation(active) = %d (err %v), want 1", count, err)
		}
	})

	t.Run("count total", func(t *testing.T) {
		count, err := repo.CountPlansForSupportConversation(ctx, "ws-1", convID, false)
		if err != nil || count != 2 {
			t.Fatalf("CountPlansForSupportConversation(total) = %d (err %v), want 2", count, err)
		}
	})
}
