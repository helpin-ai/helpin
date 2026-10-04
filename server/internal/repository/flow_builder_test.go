package repository

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFlowBuilderLatestRequestExcludesApproval(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "flow.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE agent_run_messages(id TEXT, workspace_id TEXT,dock_chat_id TEXT,role TEXT,actor_user_id TEXT,message_type TEXT,dock_chat_sequence INTEGER,created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id, workspace, chat, kind string
		seq                       int
	}{{"request", "ws", "chat", "prompt", 1}, {"approval", "ws", "chat", "approval", 2}, {"foreign", "other", "chat", "prompt", 3}} {
		if err := db.Exec(`INSERT INTO agent_run_messages VALUES(?,?,?,'user','owner',?,?,CURRENT_TIMESTAMP)`, row.id, row.workspace, row.chat, row.kind, row.seq).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewDockChatRepository(db)
	got, err := repo.LatestFlowBuilderUserMessage(context.Background(), "ws", "chat")
	if err != nil || got != "request" {
		t.Fatalf("got %q err %v", got, err)
	}
	if err := db.Exec(`INSERT INTO agent_run_messages VALUES('change','ws','chat','user','owner','request_changes',4,CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	got, err = repo.LatestFlowBuilderUserMessage(context.Background(), "ws", "chat")
	if err != nil || got != "change" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestFlowBuilderReferencesStayWithinWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "flow.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE pm_tasks(id TEXT,workspace_id TEXT,name TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO pm_tasks VALUES('task','other','Private task')`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewDockChatRepository(db)
	exists, err := repo.FlowBuilderReferenceExists(context.Background(), "ws", "task", "task")
	if err != nil || exists {
		t.Fatalf("cross-workspace ref accepted: %v", err)
	}
	label, err := repo.FlowBuilderReferenceLabel(context.Background(), "ws", "task", "task")
	if err != nil || label != "" {
		t.Fatalf("cross-workspace label exposed: %q %v", label, err)
	}
	if _, err := repo.FlowBuilderReferenceExists(context.Background(), "ws", "untrusted_table", "task"); err == nil {
		t.Fatal("unknown table accepted")
	}
}
