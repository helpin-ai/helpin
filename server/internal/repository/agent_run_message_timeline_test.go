package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentRunMessageRepositoryDockChatTimelineSpansRunsAndPaginates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:dock-message-timeline-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats ( flow_builder TEXT,
execution_enabled boolean NOT NULL DEFAULT false,
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		next_message_sequence INTEGER DEFAULT 0,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_run_messages (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		dock_chat_id TEXT,
		dock_chat_sequence INTEGER,
		client_message_id TEXT,
		delivery_status TEXT NOT NULL DEFAULT 'sent',
		actor_user_id TEXT,
		runtime_message_id TEXT,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		message_type TEXT NOT NULL,
		content_blocks TEXT,
		turn_segments TEXT,
		tool_invocations TEXT,
		token_usage TEXT,
		sequence_no INTEGER NOT NULL,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create run messages: %v", err)
	}
	const workspaceID = "00000000-0000-0000-0000-000000000001"
	const chatID = "00000000-0000-0000-0000-000000000002"
	if err := db.Exec(`INSERT INTO dock_chats (id, workspace_id, updated_at) VALUES (?, ?, ?)`, chatID, workspaceID, time.Now()).Error; err != nil {
		t.Fatalf("insert dock chat: %v", err)
	}

	repo := NewAgentRunMessageRepository(db)
	ctx := context.Background()
	clientID := "00000000-0000-0000-0000-000000000010"
	messages := []*model.AgentRunMessage{
		{ID: "00000000-0000-0000-0000-000000000101", WorkspaceID: workspaceID, RunID: "run-1", DockChatID: stringPtr(chatID), ClientMessageID: stringPtr(clientID), DeliveryStatus: "sent", Role: "user", Content: "same", MessageType: "prompt", SequenceNo: 1},
		{ID: "00000000-0000-0000-0000-000000000102", WorkspaceID: workspaceID, RunID: "run-1", DockChatID: stringPtr(chatID), DeliveryStatus: "sent", Role: "assistant", Content: "first reply", MessageType: "assistant_turn", SequenceNo: 2},
		{ID: "00000000-0000-0000-0000-000000000103", WorkspaceID: workspaceID, RunID: "run-2", DockChatID: stringPtr(chatID), DeliveryStatus: "sent", Role: "user", Content: "same", MessageType: "prompt", SequenceNo: 1},
	}
	for _, message := range messages {
		if err := repo.Create(ctx, message); err != nil {
			t.Fatalf("create message %s: %v", message.ID, err)
		}
	}
	if got := []int64{*messages[0].DockChatSequence, *messages[1].DockChatSequence, *messages[2].DockChatSequence}; got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("unexpected timeline sequences: %v", got)
	}
	// An insert failure after allocation must roll the counter back. Retrying
	// the same object must allocate again rather than reuse an uncommitted ID.
	rejected := *messages[0]
	rejected.DockChatSequence = nil
	if err := repo.Create(ctx, &rejected); err == nil {
		t.Fatal("duplicate primary key insert succeeded")
	}
	if rejected.DockChatSequence != nil {
		t.Fatal("failed insert retained an uncommitted sequence")
	}
	var nextSequence int64
	if err := db.Raw(`SELECT next_message_sequence FROM dock_chats WHERE id = ?`, chatID).Scan(&nextSequence).Error; err != nil {
		t.Fatal(err)
	}
	if nextSequence != 3 {
		t.Fatalf("failed insert left a coverage gap: counter = %d", nextSequence)
	}

	page, before, err := repo.ListByDockChat(ctx, workspaceID, chatID, nil, 2)
	if err != nil {
		t.Fatalf("list latest page: %v", err)
	}
	if len(page) != 2 || page[0].Content != "first reply" || page[1].RunID != "run-2" {
		t.Fatalf("unexpected latest page: %#v", page)
	}
	if before == nil || *before != 2 {
		t.Fatalf("unexpected next cursor: %v", before)
	}
	earlier, next, err := repo.ListByDockChat(ctx, workspaceID, chatID, before, 2)
	if err != nil {
		t.Fatalf("list earlier page: %v", err)
	}
	if len(earlier) != 1 || earlier[0].ID != messages[0].ID || next != nil {
		t.Fatalf("unexpected earlier page: %#v next=%v", earlier, next)
	}

	existing, err := repo.GetByClientMessageID(ctx, workspaceID, clientID)
	if err != nil || existing == nil || existing.ID != messages[0].ID {
		t.Fatalf("get by client id: message=%#v err=%v", existing, err)
	}

	if err := db.Exec(`UPDATE dock_chats SET next_message_sequence = NULL WHERE id = ?`, chatID).Error; err != nil {
		t.Fatalf("clear dock chat sequence: %v", err)
	}
	recovered := &model.AgentRunMessage{
		ID:             "00000000-0000-0000-0000-000000000104",
		WorkspaceID:    workspaceID,
		RunID:          "run-2",
		DockChatID:     stringPtr(chatID),
		DeliveryStatus: "sent",
		Role:           "assistant",
		Content:        "recovered reply",
		MessageType:    "assistant_turn",
		SequenceNo:     2,
	}
	if err := repo.Create(ctx, recovered); err != nil {
		t.Fatalf("create message after null counter: %v", err)
	}
	if recovered.DockChatSequence == nil || *recovered.DockChatSequence != 4 {
		t.Fatalf("recovered sequence = %v, want 4", recovered.DockChatSequence)
	}

	turn, err := repo.ListDockChatTurnThroughMessage(ctx, workspaceID, chatID, recovered.ID)
	if err != nil {
		t.Fatalf("list turn through message: %v", err)
	}
	if len(turn) != 1 || turn[0].ID != recovered.ID {
		t.Fatalf("unexpected turn detail: %#v", turn)
	}
	crossChat, err := repo.ListDockChatTurnThroughMessage(ctx, workspaceID, "00000000-0000-0000-0000-000000000099", recovered.ID)
	if err != nil || len(crossChat) != 0 {
		t.Fatalf("cross-chat detail leaked: %#v err=%v", crossChat, err)
	}
	for i, kind := range []string{"approval", "approval_request_resolution", "review_checkpoint_resolution"} {
		decision := &model.AgentRunMessage{ID: fmt.Sprintf("decision-%d", i), WorkspaceID: workspaceID, RunID: "run-2", DockChatID: stringPtr(chatID), Role: "user", MessageType: kind, Content: "Approved. Continue.", DeliveryStatus: "sent", SequenceNo: 3 + i}
		if err := repo.Create(ctx, decision); err != nil {
			t.Fatal(err)
		}
	}
	answer := &model.AgentRunMessage{ID: "final-answer", WorkspaceID: workspaceID, RunID: "run-2", DockChatID: stringPtr(chatID), Role: "assistant", MessageType: "assistant_final", Content: "Done", DeliveryStatus: "sent", SequenceNo: 6}
	if err := repo.Create(ctx, answer); err != nil {
		t.Fatal(err)
	}
	turn, err = repo.ListDockChatTurnThroughMessage(ctx, workspaceID, chatID, answer.ID)
	if err != nil || len(turn) != 5 || turn[0].ID != recovered.ID || turn[4].ID != answer.ID {
		t.Fatalf("approval incorrectly split work detail: %+v err=%v", turn, err)
	}
}
