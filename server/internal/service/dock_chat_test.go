package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDockChatListCursorPagination(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_cursor_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	base := time.Date(2026, 8, 10, 7, 0, 0, 0, time.UTC)
	chats := []model.DockChat{
		{ID: "chat-c", WorkspaceID: "ws-1", UserID: "user-1", Title: "C", CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base.Add(2 * time.Hour)},
		{ID: "chat-b", WorkspaceID: "ws-1", UserID: "user-1", Title: "B", CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour)},
		{ID: "chat-a", WorkspaceID: "ws-1", UserID: "user-1", Title: "A", CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour)},
		{ID: "chat-a0-other", WorkspaceID: "ws-2", UserID: "user-1", Title: "Other workspace", CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour)},
	}
	if err := db.Create(&chats).Error; err != nil {
		t.Fatalf("seed dock chats: %v", err)
	}

	service := &DockChatService{chatRepo: repository.NewDockChatRepository(db)}
	first, err := service.ListChats(context.Background(), "ws-1", "user-1", 2, "")
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(first.Chats) != 2 || first.Chats[0].ID != "chat-c" || first.Chats[1].ID != "chat-b" || first.NextCursor == nil {
		t.Fatalf("unexpected first page: %#v", first)
	}
	second, err := service.ListChats(context.Background(), "ws-1", "user-1", 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(second.Chats) != 1 || second.Chats[0].ID != "chat-a" || second.NextCursor != nil {
		t.Fatalf("unexpected second page: %#v", second)
	}
	if _, err := service.ListChats(context.Background(), "ws-1", "user-1", 2, "not-a-cursor"); !errors.Is(err, ErrDockChatInvalidCursor) {
		t.Fatalf("invalid cursor error = %v, want %v", err, ErrDockChatInvalidCursor)
	}
}

func TestComposeDockChatTurn(t *testing.T) {
	t.Run("without page context returns content unchanged", func(t *testing.T) {
		if got := composeDockChatTurn("hello", nil, nil); got != "hello" {
			t.Errorf("composeDockChatTurn() = %q, want %q", got, "hello")
		}
	})

	t.Run("with page context appends block", func(t *testing.T) {
		got := composeDockChatTurn("hello", map[string]interface{}{"entity_type": "task", "entity_id": "t1"}, nil)
		if !strings.HasPrefix(got, "hello\n\n<page_context>") || !strings.HasSuffix(got, "</page_context>") {
			t.Errorf("composeDockChatTurn() = %q, want page context block", got)
		}
		if !strings.Contains(got, `"entity_type":"task"`) {
			t.Errorf("composeDockChatTurn() missing entity data: %q", got)
		}
	})

	t.Run("with references appends structured block", func(t *testing.T) {
		got := composeDockChatTurn("compare these", nil, []model.DockEntityReference{
			{EntityType: "task", EntityID: "task-1", DisplayTitle: "HEL-42 · Checkout"},
			{EntityType: "document", EntityID: "doc-1", DisplayTitle: "Launch requirements"},
		})
		if !strings.Contains(got, `<references>[{"entity_type":"task"`) || !strings.HasSuffix(got, "</references>") {
			t.Errorf("composeDockChatTurn() = %q, want references block", got)
		}
	})
}

func TestNormalizeDockChatReferences(t *testing.T) {
	references, err := normalizeDockChatReferences([]model.DockEntityReference{
		{EntityType: " task ", EntityID: " task-1 ", DisplayTitle: " HEL-42 · Checkout "},
		{EntityType: "task", EntityID: "task-1", DisplayTitle: "Duplicate"},
	})
	if err != nil {
		t.Fatalf("normalizeDockChatReferences returned error: %v", err)
	}
	if len(references) != 1 || references[0].EntityID != "task-1" || references[0].DisplayTitle != "HEL-42 · Checkout" {
		t.Fatalf("normalized references = %#v", references)
	}
	if _, err := normalizeDockChatReferences([]model.DockEntityReference{
		{EntityType: "workspace", EntityID: "ws-1", DisplayTitle: "Workspace"},
	}); err == nil {
		t.Fatal("expected unsupported reference type error")
	}
}

func TestDockChatTitleFromContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "short content unchanged", content: "list open tasks", want: "list open tasks"},
		{name: "first line only", content: "line one\nline two", want: "line one"},
		{
			name:    "long content truncated",
			content: strings.Repeat("a", 100),
			want:    strings.Repeat("a", 60) + "…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dockChatTitleFromContent(tt.content); got != tt.want {
				t.Errorf("dockChatTitleFromContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsDockChatRunExpiredError(t *testing.T) {
	if isChatRunExpiredError(nil) {
		t.Error("isChatRunExpiredError(nil) = true, want false")
	}
	if !isChatRunExpiredError(errors.New("resume run: run idle timeout expired")) {
		t.Error("isChatRunExpiredError() = false for idle timeout error, want true")
	}
	if isChatRunExpiredError(errors.New("run is not paused")) {
		t.Error("isChatRunExpiredError() = true for unrelated error, want false")
	}
}

func TestSameNormalizedToolSet(t *testing.T) {
	if !sameNormalizedToolSet([]string{" read_file ", "ripgrep", "read_file"}, []string{"ripgrep", "read_file"}) {
		t.Fatal("same tool set with whitespace, order, and duplicates was reported stale")
	}
	if sameNormalizedToolSet([]string{"list_repositories", "list_commits"}, []string{"list_repositories", "checkout_repository", "ripgrep", "read_file"}) {
		t.Fatal("old metadata-only repository tool set was reported current")
	}
	if sameNormalizedToolSet([]string{"read_file", "write_file"}, []string{"read_file"}) {
		t.Fatal("run retaining a revoked tool was reported current")
	}
}
