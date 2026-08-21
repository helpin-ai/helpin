package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublicShareSourceCanAccessAgentRun(t *testing.T) {
	dbName := fmt.Sprintf("file:public_share_source_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_id TEXT,
		target_type TEXT, target_id TEXT, runtime_kind TEXT, model_tier TEXT NOT NULL DEFAULT '', invocation_mode TEXT,
		dock_chat_id TEXT, status TEXT, pause_reason TEXT, approval_state TEXT,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent runs: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL, module_id TEXT, support_conversation_id TEXT,
		active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}

	chats := []model.DockChat{
		{ID: "chat-owned", WorkspaceID: "ws-1", UserID: "user-1", Visibility: model.DockChatVisibilityPrivate},
		{ID: "chat-private", WorkspaceID: "ws-1", UserID: "user-2", Visibility: model.DockChatVisibilityPrivate},
	}
	if err := db.Create(&chats).Error; err != nil {
		t.Fatalf("seed dock chats: %v", err)
	}
	if err := db.Exec(`INSERT INTO agent_runs
		(id, workspace_id, target_type, target_id, runtime_kind, invocation_mode, dock_chat_id, status, pause_reason, approval_state, created_at, updated_at)
		VALUES
		('run-owned-chat', 'ws-1', 'task', '', 'opencode', 'autonomous', 'chat-owned', 'completed', 'none', 'not_required', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('run-private-chat', 'ws-1', 'task', '', 'opencode', 'autonomous', 'chat-private', 'completed', 'none', 'not_required', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('run-standalone', 'ws-1', 'task', '', 'opencode', 'autonomous', NULL, 'completed', 'none', 'not_required', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("seed agent runs: %v", err)
	}

	source := &PublicShareSource{
		dockChatService: &DockChatService{chatRepo: repository.NewDockChatRepository(db)},
		agentService:    &AgentService{runRepo: repository.NewAgentRunRepository(db)},
	}

	tests := []struct {
		name  string
		runID string
		want  bool
	}{
		{name: "owned chat-backed run", runID: "run-owned-chat", want: true},
		{name: "inaccessible chat-backed run", runID: "run-private-chat", want: false},
		{name: "standalone run", runID: "run-standalone", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := source.CanAccessAgentRun(context.Background(), "ws-1", "user-1", tt.runID)
			if err != nil {
				t.Fatalf("CanAccessAgentRun: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CanAccessAgentRun = %v, want %v", got, tt.want)
			}
		})
	}
}
