package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestDockPinsPersistPerUserAndRecheckRunAccess(t *testing.T) {
	agent, db := setupAgentRunActivityTest(t)
	if err := db.AutoMigrate(&model.DockPin{}); err != nil {
		t.Fatal(err)
	}
	svc := &DockChatService{chatRepo: repository.NewDockChatRepository(db.DB), agentService: agent}
	owner := "user-1"
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", AgentID: "agent-1", TriggeredByUserID: &owner, Status: model.AgentRunStatusCompleted, Input: []byte(`{}`), OutputSummary: []byte(`{}`)}
	if err := agent.runRepo.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for range 2 {
		if err := svc.SetPin(ctx, "ws-1", owner, "run", run.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := svc.ListPins(ctx, "ws-1", owner)
	if err != nil || len(pins.Runs) != 1 {
		t.Fatalf("pins=%+v err=%v", pins, err)
	}
	if err := svc.SetPin(ctx, "ws-1", "other", "run", run.ID, true); !errors.Is(err, ErrDockRunNotFound) {
		t.Fatalf("unauthorized pin: %v", err)
	}
	other, err := svc.ListPins(ctx, "ws-1", "other")
	if err != nil || len(other.Runs) != 0 {
		t.Fatalf("other pins=%+v err=%v", other, err)
	}
	// A saved pin must not grant continued access after ownership changes.
	if err := db.Model(run).Update("triggered_by_user_id", "other").Error; err != nil {
		t.Fatal(err)
	}
	pins, err = svc.ListPins(ctx, "ws-1", owner)
	if err != nil || len(pins.Runs) != 0 {
		t.Fatalf("revoked pin leaked: %+v %v", pins, err)
	}
	for range 2 {
		if err := svc.SetPin(ctx, "ws-1", owner, "run", run.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := svc.chatRepo.ListPins(ctx, "ws-1", owner)
	if err != nil || len(saved) != 0 {
		t.Fatalf("unpin failed: %+v %v", saved, err)
	}
}

func TestDockChatPinsExcludeArchivedAndOtherWorkspaces(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.DockPin{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, title TEXT, visibility TEXT, archived_at DATETIME, active_run_id TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO dock_chats (id,workspace_id,user_id,title,visibility) VALUES ('chat','ws','me','Pinned chat','private')`).Error; err != nil {
		t.Fatal(err)
	}
	svc := &DockChatService{chatRepo: repository.NewDockChatRepository(db)}
	ctx := context.Background()
	if err := svc.SetPin(ctx, "ws", "me", "chat", "chat", true); err != nil {
		t.Fatal(err)
	}
	pins, err := svc.ListPins(ctx, "ws", "me")
	if err != nil || len(pins.Chats) != 1 {
		t.Fatalf("pins=%+v err=%v", pins, err)
	}
	for _, scope := range [][2]string{{"other", "me"}, {"ws", "other"}} {
		if err := svc.SetPin(ctx, scope[0], scope[1], "chat", "chat", true); !errors.Is(err, ErrDockChatNotFound) {
			t.Fatalf("scope bypass: %v", err)
		}
	}
	if err := db.Exec(`UPDATE dock_chats SET archived_at = CURRENT_TIMESTAMP WHERE id = 'chat'`).Error; err != nil {
		t.Fatal(err)
	}
	pins, err = svc.ListPins(ctx, "ws", "me")
	if err != nil || len(pins.Chats) != 0 {
		t.Fatalf("archived pin visible: %+v %v", pins, err)
	}
	if err := svc.SetPin(ctx, "ws", "me", "chat", "chat", true); !errors.Is(err, ErrDockChatNotFound) {
		t.Fatalf("archived pin accepted: %v", err)
	}
	if err := svc.SetPin(ctx, "ws", "me", "invalid", "chat", true); !errors.Is(err, ErrDockPinKind) {
		t.Fatalf("invalid kind accepted: %v", err)
	}
}
