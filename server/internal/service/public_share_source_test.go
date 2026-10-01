package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
 coverage_gap_id TEXT, initial_context TEXT,
execution_enabled boolean NOT NULL DEFAULT false,
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

func TestPublicDockMessagesExcludeInternalData(t *testing.T) {
	actor := "private-user"
	messages := []model.AgentRunMessage{
		{ID: "question", Role: "user", MessageType: "prompt", WorkspaceID: "private-workspace", RunID: "private-run", ActorUserID: &actor,
			Content: "Check Sentry.\n<page_context>{\"id\":\"private-context\"}</page_context>"},
		{ID: "progress", Role: "assistant", MessageType: "assistant_progress", Content: "private-progress"},
		{ID: "answer", Role: "assistant", MessageType: "assistant_final", Content: "**The findings.**\n<!-- helpin_follow_up_suggestions [\"private-suggestion\"] -->",
			ContentBlocks: json.RawMessage(`[{"type":"text","text":"private-block"}]`)},
		{ID: "tool", Role: "tool", MessageType: "tool_result", Content: "private-tool-result"},
		{ID: "child", Role: "user", MessageType: "user_reply", Content: "<child_run_result>private-child</child_run_result>"},
		{ID: "system", Role: "system", Content: "private-system"},
	}
	got := publicDockMessages(messages)
	if len(got) != 2 {
		t.Fatalf("want only question and answer, got %d messages", len(got))
	}
	if got[0].Content != "Check Sentry." || got[1].Content != "**The findings.**" {
		t.Fatalf("unexpected visible content: %#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-") {
		t.Fatalf("private data in public JSON: %s", encoded)
	}
	var rows []map[string]any
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		for key := range row {
			if key != "id" && key != "role" && key != "content" {
				t.Fatalf("unexpected public field %q", key)
			}
		}
	}
	if !strings.Contains(messages[0].Content, "page_context") {
		t.Fatal("mutated stored message")
	}
}

func TestPublicDockMessagesStripContextAndIncompleteMarkers(t *testing.T) {
	for _, tag := range []string{"page_context", "references", "attachments", "source_attachments", "attachment_analysis", "previous_conversation", "child_run_result"} {
		for _, suffix := range []string{"</" + tag + ">", ""} {
			t.Run(tag+suffix, func(t *testing.T) {
				got := publicDockMessages([]model.AgentRunMessage{{ID: "user", Role: "user", MessageType: "user_reply", Content: "Visible question\n<" + tag + ">private-context" + suffix}})
				if len(got) != 1 || got[0].Content != "Visible question" {
					t.Fatalf("context leaked: %#v", got)
				}
			})
		}
	}
	got := publicDockMessages([]model.AgentRunMessage{{ID: "answer", Role: "assistant", MessageType: "assistant_final", Content: "Answer\n<!-- helpin_follow_up_suggestions ["}})
	if len(got) != 1 || got[0].Content != "Answer" {
		t.Fatalf("partial marker leaked: %#v", got)
	}
}

func TestPublicDockMessagesOnlyExposeConversationText(t *testing.T) {
	for _, message := range []model.AgentRunMessage{
		{Role: "assistant", MessageType: "assistant_progress", Content: "internal progress"},
		{Role: "assistant", MessageType: "reasoning", Content: "internal reasoning"},
		{Role: "assistant", MessageType: "status", Content: "internal status"},
		{Role: "assistant", MessageType: "future_runtime_type", Content: "internal metadata"},
		{Role: "user", MessageType: "approval_request_resolution", Content: "internal decision"},
		{Role: "tool", MessageType: "tool_result", Content: "internal tool result"},
	} {
		t.Run(message.MessageType, func(t *testing.T) {
			if got := publicDockMessages([]model.AgentRunMessage{message}); len(got) != 0 {
				t.Fatalf("internal record exposed: %#v", got)
			}
		})
	}
	for _, messageType := range []string{"", "message", "assistant_turn", "assistant_final"} {
		t.Run("visible-"+messageType, func(t *testing.T) {
			const content = "**Answer**\n\n`config/sentry.php`\n\n- First finding"
			got := publicDockMessages([]model.AgentRunMessage{{ID: "answer", Role: "assistant", MessageType: messageType, Content: content}})
			if len(got) != 1 || got[0].Content != content {
				t.Fatalf("lost visible answer: %#v", got)
			}
		})
	}
}
