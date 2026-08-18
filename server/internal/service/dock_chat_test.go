package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type dockChatMemberRepo struct{}

func (dockChatMemberRepo) GetMembership(_ context.Context, _, userID string) (*authorization.MemberInfo, error) {
	return &authorization.MemberInfo{ID: "member-" + userID, Role: model.RoleMember, Status: "active"}, nil
}

func (dockChatMemberRepo) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return nil, nil
}

type dockChatModuleRepo map[string][]model.ModuleID

func (r dockChatModuleRepo) ListAccessibleModules(_ context.Context, _ string, memberID string, _ []string) ([]model.ModuleID, error) {
	return r[memberID], nil
}

type scriptedDockChatTitleLLM struct {
	response string
	err      error
	requests []llm.ChatRequest
	metered  []AIUsageMeteringContext
}

func (s *scriptedDockChatTitleLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.requests = append(s.requests, req)
	if metering, ok := AIUsageMeteringFromContext(ctx); ok {
		s.metered = append(s.metered, metering)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{Content: s.response}, nil
}

func TestDockChatListCursorPagination(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_cursor_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	base := time.Date(2026, 8, 10, 7, 0, 0, 0, time.UTC)
	emptySupportConversationID := "conversation-empty"
	chats := []model.DockChat{
		{ID: "chat-empty-support", WorkspaceID: "ws-1", UserID: "user-1", SupportConversationID: &emptySupportConversationID, CreatedAt: base.Add(3 * time.Hour), UpdatedAt: base.Add(3 * time.Hour)},
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

func TestDockChatVisibilityScopesListAndReadAccess(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_visibility_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT,
		support_conversation_id TEXT, active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	now := time.Now().UTC()
	crm := model.ModuleCRM
	support := model.ModuleSupport
	chats := []model.DockChat{
		{ID: "mine", WorkspaceID: "ws-1", UserID: "user-2", Title: "Mine", Visibility: model.DockChatVisibilityPrivate, CreatedAt: now, UpdatedAt: now},
		{ID: "other-private", WorkspaceID: "ws-1", UserID: "user-1", Title: "Private", Visibility: model.DockChatVisibilityPrivate, CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
		{ID: "workspace", WorkspaceID: "ws-1", UserID: "user-1", Title: "Workspace", Visibility: model.DockChatVisibilityWorkspace, CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now},
		{ID: "crm", WorkspaceID: "ws-1", UserID: "user-1", Title: "CRM", Visibility: model.DockChatVisibilityModule, ModuleID: &crm, CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now},
		{ID: "support", WorkspaceID: "ws-1", UserID: "user-1", Title: "Support", Visibility: model.DockChatVisibilityModule, ModuleID: &support, CreatedAt: now.Add(-4 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&chats).Error; err != nil {
		t.Fatalf("seed chats: %v", err)
	}
	authz := authorization.NewAuthzService(db, dockChatMemberRepo{}, dockChatModuleRepo{
		"member-user-2": {model.ModuleCRM},
	})
	svc := &DockChatService{chatRepo: repository.NewDockChatRepository(db), authz: authz}

	listed, err := svc.ListChats(context.Background(), "ws-1", "user-2", 20, "")
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	got := make(map[string]bool, len(listed.Chats))
	for _, chat := range listed.Chats {
		got[chat.ID] = true
	}
	for _, expected := range []string{"mine", "workspace", "crm"} {
		if !got[expected] {
			t.Errorf("missing visible chat %q from %#v", expected, got)
		}
	}
	for _, hidden := range []string{"other-private", "support"} {
		if got[hidden] {
			t.Errorf("listed hidden chat %q", hidden)
		}
	}
	if _, err := svc.GetChat(context.Background(), "ws-1", "user-2", "crm"); err != nil {
		t.Fatalf("read CRM-shared chat: %v", err)
	}
	if _, err := svc.GetChat(context.Background(), "ws-1", "user-2", "support"); !errors.Is(err, ErrDockChatNotFound) {
		t.Fatalf("support chat error = %v, want not found", err)
	}
	if _, err := svc.UpdateChat(context.Background(), "ws-1", "user-2", "crm", model.UpdateDockChatRequest{Title: strPtr("Changed")}); !errors.Is(err, ErrDockChatNotFound) {
		t.Fatalf("shared chat update error = %v, want not found", err)
	}
	if _, err := svc.OwnedActiveRunForChat(context.Background(), "ws-1", "user-2", "crm"); !errors.Is(err, ErrDockChatNotFound) {
		t.Fatalf("shared chat mutation error = %v, want not found", err)
	}
}

func TestDockChatCreateReusesSupportConversationChat(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_support_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT,
		last_message_at DATETIME, archived_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	conversationID := "conversation-42"
	existing := model.DockChat{
		ID: "chat-existing", WorkspaceID: "ws-1", UserID: "user-1",
		Title: "Refund request", SupportConversationID: &conversationID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed dock chat: %v", err)
	}

	service := &DockChatService{chatRepo: repository.NewDockChatRepository(db)}
	chat, err := service.CreateChat(context.Background(), "ws-1", "user-1", model.CreateDockChatRequest{
		SupportConversationID: &conversationID,
	})
	if err != nil {
		t.Fatalf("create support chat: %v", err)
	}
	if chat.ID != existing.ID {
		t.Fatalf("chat ID = %q, want existing %q", chat.ID, existing.ID)
	}
}

func TestDockChatFindSupportConversationChatDoesNotCreateMissingRow(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_find_support_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT,
		last_message_at DATETIME, archived_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}

	service := &DockChatService{chatRepo: repository.NewDockChatRepository(db)}
	chat, err := service.FindSupportConversationChat(context.Background(), "ws-1", "user-1", "conversation-missing")
	if err != nil {
		t.Fatalf("find support chat: %v", err)
	}
	if chat != nil {
		t.Fatalf("chat = %#v, want nil", chat)
	}
	var count int64
	if err := db.Model(&model.DockChat{}).Count(&count).Error; err != nil {
		t.Fatalf("count dock chats: %v", err)
	}
	if count != 0 {
		t.Fatalf("dock chat count = %d, want 0", count)
	}
}

func TestDockChatCreatePreservesArchivedSupportConversationChat(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_archived_support_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT,
		last_message_at DATETIME, archived_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_dock_chats_support_conversation
		ON dock_chats (workspace_id, user_id, support_conversation_id)
		WHERE archived_at IS NULL`).Error; err != nil {
		t.Fatalf("create active support chat index: %v", err)
	}
	conversationID := "conversation-archived"
	archivedAt := time.Now().Add(-time.Hour).UTC()
	existing := model.DockChat{
		ID: "chat-archived", WorkspaceID: "ws-1", UserID: "user-1",
		Title: "Archived refund request", SupportConversationID: &conversationID,
		ArchivedAt: &archivedAt, CreatedAt: archivedAt.Add(-time.Hour), UpdatedAt: archivedAt,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed archived dock chat: %v", err)
	}

	service := &DockChatService{chatRepo: repository.NewDockChatRepository(db)}
	created, err := service.CreateChat(context.Background(), "ws-1", "user-1", model.CreateDockChatRequest{
		SupportConversationID: &conversationID,
	})
	if err != nil {
		t.Fatalf("create replacement support chat: %v", err)
	}
	if created.ID == existing.ID {
		t.Fatalf("chat ID = %q, want a fresh chat", created.ID)
	}
	var archived model.DockChat
	if err := db.First(&archived, "id = ?", existing.ID).Error; err != nil {
		t.Fatalf("reload archived chat: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("archived chat was unexpectedly restored")
	}
	var count int64
	if err := db.Model(&model.DockChat{}).Count(&count).Error; err != nil {
		t.Fatalf("count dock chats: %v", err)
	}
	if count != 2 {
		t.Fatalf("dock chat count = %d, want 2", count)
	}
}

func TestDockChatListHydratesActiveRunStatus(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_status_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT,
		pause_reason TEXT, approval_state TEXT, execution_stage TEXT,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent runs: %v", err)
	}
	now := time.Date(2026, 8, 10, 7, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO agent_runs (id, workspace_id, status, pause_reason, approval_state, created_at)
		VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)`,
		"run-running", "ws-1", model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, "not_required", now,
		"run-paused", "ws-1", model.AgentRunStatusPaused, model.AgentRunPauseReasonUserMessage, "not_required", now,
		"run-completed", "ws-1", model.AgentRunStatusCompleted, model.AgentRunPauseReasonNone, "not_required", now,
	).Error; err != nil {
		t.Fatalf("seed agent runs: %v", err)
	}
	runningID := "run-running"
	pausedID := "run-paused"
	completedID := "run-completed"
	chats := []model.DockChat{
		{ID: "chat-running", WorkspaceID: "ws-1", UserID: "user-1", Title: "Running", ActiveRunID: &runningID, CreatedAt: now.Add(3 * time.Minute), UpdatedAt: now.Add(3 * time.Minute)},
		{ID: "chat-paused", WorkspaceID: "ws-1", UserID: "user-1", Title: "Paused", ActiveRunID: &pausedID, CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(2 * time.Minute)},
		{ID: "chat-completed", WorkspaceID: "ws-1", UserID: "user-1", Title: "Completed", ActiveRunID: &completedID, CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
		{ID: "chat-empty", WorkspaceID: "ws-1", UserID: "user-1", Title: "Empty", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&chats).Error; err != nil {
		t.Fatalf("seed dock chats: %v", err)
	}

	service := &DockChatService{
		chatRepo: repository.NewDockChatRepository(db),
		runRepo:  repository.NewAgentRunRepository(db),
	}
	result, err := service.ListChats(context.Background(), "ws-1", "user-1", 10, "")
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	want := map[string]string{
		"chat-running":   model.AgentRunStatusRunning,
		"chat-paused":    model.AgentRunStatusPaused,
		"chat-completed": model.AgentRunStatusCompleted,
		"chat-empty":     "",
	}
	for _, chat := range result.Chats {
		if chat.ActiveRunStatus != want[chat.ID] {
			t.Errorf("chat %s active run status = %q, want %q", chat.ID, chat.ActiveRunStatus, want[chat.ID])
		}
	}
}

func TestDockChatGenerateTitleUsesSemanticCompletion(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_title_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	now := time.Date(2026, 8, 10, 7, 0, 0, 0, time.UTC)
	chat := model.DockChat{ID: "chat-1", WorkspaceID: "ws-1", UserID: "user-1", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&chat).Error; err != nil {
		t.Fatalf("seed dock chat: %v", err)
	}
	provider := &scriptedDockChatTitleLLM{response: `{"title":"  Investigate HLP-42 latency.  "}`}
	service := (&DockChatService{chatRepo: repository.NewDockChatRepository(db)}).SetTitleLLM(provider)

	updated, err := service.GenerateTitle(context.Background(), "ws-1", "user-1", "chat-1", model.GenerateDockChatTitleRequest{
		Content: "Can you investigate why HLP-42 has become slow after the latest deployment?",
	})
	if err != nil {
		t.Fatalf("generate title: %v", err)
	}
	if updated.Title != "Investigate HLP-42 latency" {
		t.Errorf("title = %q, want semantic title", updated.Title)
	}
	if len(provider.requests) != 1 || !provider.requests[0].JSONMode || provider.requests[0].MaxTokens != 80 {
		t.Fatalf("unexpected title request: %#v", provider.requests)
	}
	if len(provider.metered) != 1 || provider.metered[0].FeatureKey != BillingFeatureDockChatTitle {
		t.Fatalf("unexpected title metering: %#v", provider.metered)
	}
}

func TestDockChatTitleFromPageContextUsesSourceIdentity(t *testing.T) {
	tests := []struct {
		name string
		ctx  map[string]interface{}
		want string
	}{
		{
			name: "support",
			ctx: map[string]interface{}{
				"entity_type":   "support_conversation",
				"entity_id":     "91cee9ac-959b-4066-b613-5b9847095a97",
				"display_id":    "#482",
				"display_title": "Refund request",
			},
			want: "Support · #482 · Refund request",
		},
		{
			name: "fallback support title defers to semantic title when no public identity is loaded",
			ctx: map[string]interface{}{
				"entity_type":   "support_conversation",
				"entity_id":     "conv-42",
				"display_title": "Conversation conv-42",
			},
			want: "",
		},
		{
			name: "task uses task key",
			ctx: map[string]interface{}{
				"entity_type":   "task",
				"entity_id":     "91cee9ac-959b-4066-b613-5b9847095a97",
				"display_id":    "ENG-124",
				"display_title": "Fix login timeout",
			},
			want: "Tasks · ENG-124 · Fix login timeout",
		},
		{
			name: "crm uses public record id",
			ctx: map[string]interface{}{
				"entity_type":   "crm_contact",
				"entity_id":     "91cee9ac-959b-4066-b613-5b9847095a97",
				"display_id":    "CON-381",
				"display_title": "Maya Singh",
			},
			want: "CRM · CON-381 · Maya Singh",
		},
		{
			name: "epic omits opaque uuid",
			ctx: map[string]interface{}{
				"entity_type":   "epic",
				"entity_id":     "91cee9ac-959b-4066-b613-5b9847095a97",
				"display_title": "Q3 onboarding",
			},
			want: "Tasks · Q3 onboarding",
		},
		{
			name: "document omits opaque uuid",
			ctx: map[string]interface{}{
				"entity_type":   "document",
				"entity_id":     "91cee9ac-959b-4066-b613-5b9847095a97",
				"display_title": "API authentication",
			},
			want: "Docs · API authentication",
		},
		{
			name: "workspace context defers to the semantic user-message title",
			ctx: map[string]interface{}{
				"entity_type":   "workspace",
				"entity_id":     "fb464f68-0c05-4cbd-b571-5254bc88211f",
				"display_title": "ContentStudio",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dockChatTitleFromPageContext(tt.ctx); got != tt.want {
				t.Fatalf("dockChatTitleFromPageContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDockChatGenerateTitlePreservesManualTitle(t *testing.T) {
	dbName := fmt.Sprintf("file:dock_chat_manual_title_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT, support_conversation_id TEXT, active_run_id TEXT, last_message_at DATETIME, archived_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock chats: %v", err)
	}
	now := time.Date(2026, 8, 10, 7, 0, 0, 0, time.UTC)
	chat := model.DockChat{ID: "chat-1", WorkspaceID: "ws-1", UserID: "user-1", Title: "My release notes", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&chat).Error; err != nil {
		t.Fatalf("seed dock chat: %v", err)
	}
	provider := &scriptedDockChatTitleLLM{response: `{"title":"Replacement"}`}
	service := (&DockChatService{chatRepo: repository.NewDockChatRepository(db)}).SetTitleLLM(provider)

	updated, err := service.GenerateTitle(context.Background(), "ws-1", "user-1", "chat-1", model.GenerateDockChatTitleRequest{Content: "Replace it"})
	if err != nil {
		t.Fatalf("generate title: %v", err)
	}
	if updated.Title != "My release notes" {
		t.Errorf("title = %q, want manual title preserved", updated.Title)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("LLM called %d times for named chat, want 0", len(provider.requests))
	}
}

func TestComposeDockChatTurn(t *testing.T) {
	t.Run("without page context returns content unchanged", func(t *testing.T) {
		if got := composeDockChatTurn("hello", nil, nil, nil, nil, ""); got != "hello" {
			t.Errorf("composeDockChatTurn() = %q, want %q", got, "hello")
		}
	})

	t.Run("with page context appends block", func(t *testing.T) {
		got := composeDockChatTurn("hello", map[string]interface{}{"entity_type": "task", "entity_id": "t1"}, nil, nil, nil, "")
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
		}, nil, nil, "")
		if !strings.Contains(got, `<references>[{"entity_type":"task"`) || !strings.HasSuffix(got, "</references>") {
			t.Errorf("composeDockChatTurn() = %q, want references block", got)
		}
	})

	t.Run("with media includes metadata and analysis", func(t *testing.T) {
		got := composeDockChatTurn("inspect this", nil, nil, []dockChatMediaAttachment{{
			ID: "attachment-1", FileName: "error.png", FileType: "image/png", FileSize: 2048,
		}}, nil, "The screenshot shows a 500 error.")
		if !strings.Contains(got, `<attachments>[{"id":"attachment-1","file_name":"error.png","file_type":"image/png","file_size":2048}]</attachments>`) {
			t.Fatalf("composeDockChatTurn() missing attachment metadata: %q", got)
		}
		if !strings.Contains(got, "<attachment_analysis>The screenshot shows a 500 error.</attachment_analysis>") {
			t.Fatalf("composeDockChatTurn() missing attachment analysis: %q", got)
		}
	})

	t.Run("with source media keeps only metadata in the transcript", func(t *testing.T) {
		got := composeDockChatTurn("inspect the thread", nil, nil, nil, []dockChatMediaAttachment{{
			ID: "support-image-1", FileName: "error.png", FileType: "image/png", FileSize: 2048, Source: "support_conversation", URL: "https://temporary.example/error.png",
		}}, "The customer screenshot shows a 500 error.")
		if !strings.Contains(got, `<source_attachments>[{"id":"support-image-1","file_name":"error.png","file_type":"image/png","file_size":2048,"source":"support_conversation"}]</source_attachments>`) {
			t.Fatalf("composeDockChatTurn() must persist source metadata but not the temporary URL: %q", got)
		}
		if strings.Contains(got, "temporary.example") {
			t.Fatalf("composeDockChatTurn() leaked temporary source URL: %q", got)
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
