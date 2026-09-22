package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestActiveDockChatsIncludeAllAttentionAndExcludeIdleOrInaccessible(t *testing.T) {
	db := openAgentRunListTestDB(t)
	for i := 0; i < 36; i++ {
		id := fmt.Sprintf("run-%02d", i)
		seedAgentRunListTestRow(t, db, id, "ws", "agent", model.AgentRunStatusPaused, model.AgentRunPauseReasonHumanApproval, `{}`)
		if err := db.Model(&model.AgentRun{}).Where("id = ?", id).Updates(map[string]any{"triggered_by_user_id": "owner", "dock_chat_id": id}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO dock_chats (id,workspace_id,user_id,active_run_id) VALUES (?, 'ws', 'owner', ?)`, id, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, sql := range []string{
		`UPDATE dock_chats SET archived_at = CURRENT_TIMESTAMP WHERE id = 'run-00'`,
		`UPDATE dock_chats SET user_id = 'other' WHERE id = 'run-01'`,
		`UPDATE dock_chats SET workspace_id = 'other' WHERE id = 'run-02'`,
		`UPDATE dock_chats SET active_run_id = 'new-run' WHERE id = 'run-03'`,
		`UPDATE agent_runs SET pause_reason = 'user_message' WHERE id = 'run-04'`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	runs, err := NewAgentRunRepository(db).ListActiveDockRunsForActor(context.Background(), "ws", "owner")
	if err != nil || len(runs) != 31 {
		t.Fatalf("got %d runs, err=%v; want all 31 eligible chats", len(runs), err)
	}
	for _, run := range runs {
		if run.ID < "run-05" {
			t.Fatalf("ineligible chat included: %s", run.ID)
		}
	}
}
