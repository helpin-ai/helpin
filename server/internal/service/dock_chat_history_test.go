package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func chatHistoryFixture(t *testing.T) (*gorm.DB, *repository.AgentRunMessageRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	err = db.Exec(`CREATE TABLE agent_run_messages (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT, dock_chat_id TEXT, dock_chat_sequence INTEGER, sequence_no INTEGER, role TEXT, content TEXT, message_type TEXT, delivery_status TEXT DEFAULT 'sent', created_at DATETIME)`).Error
	if err != nil {
		t.Fatal(err)
	}
	return db, repository.NewAgentRunMessageRepository(db)
}

func insertHistoryMessage(t *testing.T, db *gorm.DB, seq int, role, kind, content string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO agent_run_messages (id, workspace_id, run_id, dock_chat_id, dock_chat_sequence, sequence_no, role, content, message_type) VALUES (?, 'ws', 'research-run', 'chat', ?, ?, ?, ?, ?)`, fmt.Sprint(seq), seq, seq, role, content, kind).Error; err != nil {
		t.Fatal(err)
	}
}

func TestChatCarryForwardPreservesResearchAcrossSuccessors(t *testing.T) {
	db, repo := chatHistoryFixture(t)
	insertHistoryMessage(t, db, 1, "user", "user", "Audit video and image requirements")
	first := "First six networks: " + strings.Repeat("research finding. ", 650)
	second := "Remaining six networks: " + strings.Repeat("another finding. ", 650)
	insertHistoryMessage(t, db, 2, "assistant", "assistant_final", first)
	insertHistoryMessage(t, db, 3, "assistant", "assistant_final", second)
	insertHistoryMessage(t, db, 4, "user", "user", "Create separate frontend and backend Engineering tasks")
	for i := 5; i < 101; i++ {
		insertHistoryMessage(t, db, i, "assistant", "assistant_progress", "Retrying failed team lookup")
	}
	insertHistoryMessage(t, db, 101, "user", "user", "Continue\n<previous_conversation>nested obsolete handoff</previous_conversation>")
	chatID := "chat"
	service := &DockChatService{runMessageRepo: repo}
	result := service.buildCarryForward(context.Background(), &model.AgentRun{ID: "successor-run", WorkspaceID: "ws", DockChatID: &chatID, Status: "failed"})
	for _, want := range []string{"Audit video", strings.TrimSpace(first), strings.TrimSpace(second), "separate frontend and backend", "Continue", "Never repeat an external mutation"} {
		if !strings.Contains(result, want) {
			t.Errorf("missing substantive history: %.80s", want)
		}
	}
	for _, unwanted := range []string{"Retrying failed", "nested obsolete"} {
		if strings.Contains(result, unwanted) {
			t.Errorf("included %q", unwanted)
		}
	}
}

func TestChatHistoryExcerptIsUTF8AndRetrievable(t *testing.T) {
	text := strings.Repeat("日本語", 5000)
	first := chatHistoryExcerpt(text, 0, 1000)
	if !utf8.ValidString(first.Content) || first.NextOffset == nil {
		t.Fatal("missing UTF-8-safe continuation")
	}
	var all strings.Builder
	all.WriteString(first.Content)
	next := first.NextOffset
	for next != nil {
		part := chatHistoryExcerpt(text, *next, 1000)
		all.WriteString(part.Content)
		next = part.NextOffset
	}
	if all.String() != text {
		t.Fatal("history pagination lost content")
	}
}

func historyCommandFixture(t *testing.T) (*gorm.DB, *InternalCommandService) {
	t.Helper()
	db, repo := chatHistoryFixture(t)
	for _, statement := range []string{
		`CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, dock_chat_id TEXT, triggered_by_user_id TEXT, external_runtime TEXT, external_runtime_id TEXT)`,
		`CREATE TABLE dock_chats (id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, visibility TEXT)`,
		`INSERT INTO agent_runs VALUES ('run', 'ws', 'chat', 'owner', '', '')`,
		`INSERT INTO dock_chats VALUES ('chat', 'ws', 'owner', 'private')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, &InternalCommandService{agentRunRepo: repository.NewAgentRunRepository(db), dockChatRepo: repository.NewDockChatRepository(db), agentService: &AgentService{runMessageRepo: repo}}
}

func TestChatHistoryCommandPaginationAndIsolation(t *testing.T) {
	db, service := historyCommandFixture(t)
	insertHistoryMessage(t, db, 1, "user", "user", "original question")
	insertHistoryMessage(t, db, 2, "assistant", "assistant_final", strings.Repeat("界", 9000))
	for i := 3; i < 100; i++ {
		insertHistoryMessage(t, db, i, "assistant", "assistant_progress", "noise")
	}
	if err := db.Exec(`INSERT INTO agent_run_messages (id,workspace_id,dock_chat_id,dock_chat_sequence,role,content,message_type) VALUES ('foreign','other-ws','chat',101,'user','secret','user'), ('other-chat','ws','other-chat',102,'user','private','user')`).Error; err != nil {
		t.Fatal(err)
	}
	meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "owner", RunID: "run"}
	output, err := service.executeReadChatHistory(context.Background(), meta, []byte(`{"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Messages   []chatHistoryMessage `json:"messages"`
		NextBefore *int64               `json:"next_before"`
	}
	if err := json.Unmarshal(output, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Sequence != 2 || page.NextBefore == nil || *page.NextBefore != 2 {
		t.Fatalf("wrong history page: %.300s", output)
	}
	if page.Messages[0].NextOffset == nil || *page.Messages[0].NextOffset != 2000 {
		t.Fatal("missing message continuation")
	}
	output, err = service.executeReadChatHistory(context.Background(), meta, []byte(`{"message_sequence":2,"offset":2000}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output, &page); err != nil {
		t.Fatal(err)
	}
	if page.Messages[0].Content != strings.Repeat("界", 7000) {
		t.Fatal("full message retrieval lost content")
	}
	output, err = service.executeReadChatHistory(context.Background(), meta, []byte(`{"before_sequence":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "original question") || strings.Contains(string(output), "secret") {
		t.Fatal("incorrect older page")
	}
	for _, seq := range []int{101, 102} {
		if _, err := service.executeReadChatHistory(context.Background(), meta, []byte(fmt.Sprintf(`{"message_sequence":%d}`, seq))); err == nil {
			t.Fatal("read foreign message")
		}
	}
}

func TestChatHistoryCommandRejectsUnauthorizedCallers(t *testing.T) {
	tests := []struct{ name, workspace, actor, run, sql string }{
		{name: "wrong actor", workspace: "ws", actor: "other", run: "run"},
		{name: "wrong workspace", workspace: "other", actor: "owner", run: "run"},
		{name: "missing run", workspace: "ws", actor: "owner", run: "missing"},
		{name: "non chat run", workspace: "ws", actor: "owner", run: "run", sql: `UPDATE agent_runs SET dock_chat_id=NULL`},
		{name: "revoked visibility", workspace: "ws", actor: "owner", run: "run", sql: `UPDATE dock_chats SET user_id='someone-else'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, service := historyCommandFixture(t)
			if tt.sql != "" {
				if err := db.Exec(tt.sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			_, err := service.executeReadChatHistory(context.Background(), model.InternalCommandContext{WorkspaceID: tt.workspace, ActorID: tt.actor, RunID: tt.run}, []byte(`{}`))
			if err == nil {
				t.Fatal("unauthorized history read succeeded")
			}
		})
	}
}

func TestChatHistoryRejectsInvalidPagination(t *testing.T) {
	_, service := historyCommandFixture(t)
	for _, input := range []string{`{"limit":21}`, `{"limit":-1}`, `{"offset":-1}`, `{"offset":1}`, `{"before_sequence":-1}`, `{"message_sequence":-1}`, `{`} {
		if _, err := service.executeReadChatHistory(context.Background(), model.InternalCommandContext{WorkspaceID: "ws", ActorID: "owner", RunID: "run"}, []byte(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}

func TestChatCarryForwardBudgetPreservesOriginalAndLatest(t *testing.T) {
	messages := []model.AgentRunMessage{{Role: "user", Content: "original request"}}
	for i := 0; i < 30; i++ {
		messages = append(messages, model.AgentRunMessage{Role: "assistant", MessageType: "assistant_final", Content: strings.Repeat("界", 10000)})
	}
	messages = append(messages, model.AgentRunMessage{Role: "user", Content: "latest decision"})
	result := renderChatCarryForward(messages)
	if len(result) > dockChatCarryForwardTotal || !utf8.ValidString(result) || !strings.Contains(result, "original request") || !strings.Contains(result, "latest decision") || !strings.Contains(result, "Excerpt truncated") {
		t.Fatal("handoff budget or retention failed")
	}
}

func TestChatCarryForwardKeepsFirstRequestBeyondRecentWindow(t *testing.T) {
	db, repo := chatHistoryFixture(t)
	insertHistoryMessage(t, db, 1, "user", "user", "unique original objective")
	for i := 2; i < 30; i++ {
		insertHistoryMessage(t, db, i, "assistant", "assistant_final", "useful later answer")
	}
	if err := db.Exec(`INSERT INTO agent_run_messages (id,workspace_id,run_id,dock_chat_id,dock_chat_sequence,role,content,message_type,delivery_status) VALUES ('failed','ws','other','chat',31,'user','unsent request','user','failed'), ('foreign','other','other','chat',32,'user','foreign secret','user','sent')`).Error; err != nil {
		t.Fatal(err)
	}
	chat := "chat"
	result := (&DockChatService{runMessageRepo: repo}).buildCarryForward(context.Background(), &model.AgentRun{ID: "latest", WorkspaceID: "ws", DockChatID: &chat})
	if !strings.Contains(result, "unique original objective") {
		t.Fatal("original request displaced")
	}
	if strings.Contains(result, "unsent request") || strings.Contains(result, "foreign secret") {
		t.Fatal("included ineligible history")
	}
}

func TestChatCarryForwardPreservesEarlierPlanAndArtifactReferences(t *testing.T) {
	db, repo := chatHistoryFixture(t)
	for _, sql := range []string{
		`CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, dock_chat_id TEXT)`,
		`INSERT INTO agent_runs VALUES ('research','ws','chat'),('successor','ws','chat'),('foreign','other','chat')`,
		`CREATE TABLE coding_session_state_snapshots (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT, snapshot_payload TEXT, created_at DATETIME)`,
		`INSERT INTO coding_session_state_snapshots VALUES ('plan','ws','research','{"current_plan":{"plan":[{"step":"Create Engineering tasks","status":"pending"}]}}','2026-09-21 09:00:00'),('empty','ws','successor','{}','2026-09-21 10:00:00'),('foreign','other','foreign','{"current_plan":{"note":"foreign secret"}}','2026-09-21 11:00:00')`,
		`UPDATE coding_session_state_snapshots SET snapshot_payload = CAST(snapshot_payload AS BLOB)`,
		`CREATE TABLE agent_run_artifacts (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT, artifact_type TEXT, storage_mode TEXT, object_key TEXT, created_at DATETIME, sequence_no INTEGER)`,
		`INSERT INTO agent_run_artifacts VALUES ('artifact','ws','research','report','object','private/storage/key','2026-09-21 09:00:00',1)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := &DockChatService{runMessageRepo: repo, artifactRepo: repository.NewAgentRunArtifactRepository(db), agentService: &AgentService{sessionSnapshotRepo: repository.NewCodingSessionStateSnapshotRepository(db)}}
	chat := "chat"
	result := service.buildCarryForward(context.Background(), &model.AgentRun{ID: "successor", WorkspaceID: "ws", DockChatID: &chat})
	if !strings.Contains(result, "Create Engineering tasks") || !strings.Contains(result, "artifact artifact, run research") {
		t.Fatalf("missing retained state: %s", result)
	}
	if strings.Contains(result, "private/storage/key") || strings.Contains(result, "foreign secret") {
		t.Fatal("leaked private storage or foreign plan")
	}
}
