package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentRunRepositoryFindActiveNonDockByTargetSkipsDockChatRuns(t *testing.T) {
	tests := []struct {
		name       string
		seedPlain  bool
		wantRunID  string
		wantNoRuns bool
	}{
		{name: "only a dock chat run is active", wantNoRuns: true},
		{name: "an older non-dock run is still found", seedPlain: true, wantRunID: "run-plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openAgentRunListTestDB(t)
			repo := NewAgentRunRepository(db)
			if tt.seedPlain {
				seedAgentRunListTestRow(t, db, "run-plain", "workspace-1", "agent-1",
					model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, `{}`)
				if err := db.Exec(`UPDATE agent_runs SET created_at = ? WHERE id = 'run-plain'`,
					time.Now().UTC().Add(-time.Hour)).Error; err != nil {
					t.Fatalf("age plain run: %v", err)
				}
			}
			seedAgentRunListTestRow(t, db, "run-dock", "workspace-1", "agent-1",
				model.AgentRunStatusPaused, model.AgentRunPauseReasonUserMessage, `{}`)
			if err := db.Exec(`UPDATE agent_runs SET dock_chat_id = 'chat-1' WHERE id = 'run-dock'`).Error; err != nil {
				t.Fatalf("attach dock chat: %v", err)
			}

			run, err := repo.FindActiveNonDockByTarget(context.Background(), "workspace-1", "workspace", "workspace-1")
			if err != nil {
				t.Fatalf("FindActiveNonDockByTarget: %v", err)
			}
			if tt.wantNoRuns {
				if run != nil {
					t.Fatalf("FindActiveNonDockByTarget returned %q, want no run", run.ID)
				}
				return
			}
			if run == nil || run.ID != tt.wantRunID {
				t.Fatalf("FindActiveNonDockByTarget returned %v, want %q", run, tt.wantRunID)
			}

			newest, err := repo.FindActiveByTarget(context.Background(), "workspace-1", "workspace", "workspace-1")
			if err != nil || newest == nil || newest.ID != "run-dock" {
				t.Fatalf("FindActiveByTarget = %v, %v; want the newest (dock) run unchanged", newest, err)
			}
		})
	}
}
