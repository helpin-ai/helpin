package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestProductToolCommandDefinitionsExposeNativeAliases(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	tests := []struct {
		command  string
		alias    string
		category string
		mutating bool
	}{
		{command: "support.list_conversation_messages", alias: "list_conversation_messages", category: "Support", mutating: false},
		{command: "support.draft_reply", alias: "draft_support_reply", category: "Support", mutating: true},
		{command: "support.update_conversation_status", alias: "update_conversation_status", category: "Support", mutating: true},
		{command: "crm.list_deals", alias: "list_deals", category: "CRM", mutating: false},
		{command: "crm.list_contacts", alias: "list_contacts", category: "CRM", mutating: false},
		{command: "crm.list_buyer_signals", alias: "list_buyer_signals", category: "CRM", mutating: false},
		{command: "crm.create_deal", alias: "create_crm_deal", category: "CRM / Operations", mutating: true},
		{command: "docs.search_documents", alias: "search_documents", category: "Docs", mutating: false},
		{command: "docs.insert_document_artifact", alias: "insert_document_artifact", category: "Docs", mutating: true},
		{command: "docs.insert_document_image", alias: "insert_document_image", category: "Docs", mutating: true},
		{command: "release.get_release_context", alias: "get_release_context", category: "Release", mutating: false},
		{command: "release.find_tasks_for_git_changes", alias: "find_tasks_for_git_changes", category: "Release", mutating: false},
		{command: "release.get_task_context", alias: "get_task_context", category: "Release", mutating: false},
		{command: "docs.publish_prd_draft", alias: "publish_prd_draft", category: "Docs", mutating: false},
		{command: "docs.publish_task_plan_doc", alias: "publish_task_plan_doc", category: "Docs", mutating: false},
		{command: "docs.publish_document_change_proposal", alias: "publish_document_change_proposal", category: "Docs", mutating: true},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			def, ok := svc.Definition(tt.command)
			if !ok {
				t.Fatalf("expected %s definition", tt.command)
			}
			if !def.ExposesTool() {
				t.Fatalf("expected %s to expose a runtime tool", tt.command)
			}
			if def.Tool.Alias != tt.alias {
				t.Fatalf("alias = %q, want %q", def.Tool.Alias, tt.alias)
			}
			if def.Tool.Category != tt.category {
				t.Fatalf("category = %q, want %q", def.Tool.Category, tt.category)
			}
			if def.Mutating != tt.mutating {
				t.Fatalf("mutating = %v, want %v", def.Mutating, tt.mutating)
			}
			if def.Tool.InputSchema == nil {
				t.Fatal("expected an input schema")
			}
		})
	}
}

func seedProductToolConversation(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	mustExec(t, db, `INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, priority, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"conv-1", "ws-1", 1, "Widget is broken", "open", "medium", "widget", "internal", now, now)
	mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-1", "ws-1", "conv-1", "customer", "The widget will not load.", false, now, now)
	mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-2", "ws-1", "conv-1", "user", "Investigating now.", true, now.Add(time.Minute), now.Add(time.Minute))
}

func TestListConversationMessagesCommandReturnsConversationMessages(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "support_conversation",
		TargetID:    "conv-1",
	}, "support.list_conversation_messages", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("support.list_conversation_messages returned error: %v", err)
	}
	var result struct {
		Messages []struct {
			SenderType string `json:"sender_type"`
			Content    string `json:"content"`
			IsInternal bool   `json:"is_internal"`
			CreatedAt  string `json:"created_at"`
		} `json:"messages"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, string(output))
	}
	if result.Total != 2 || len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %#v", result)
	}
	if result.Messages[0].SenderType != "customer" || result.Messages[0].Content != "The widget will not load." {
		t.Fatalf("unexpected first message %#v", result.Messages[0])
	}
	if !result.Messages[1].IsInternal {
		t.Fatalf("expected internal note to be included, got %#v", result.Messages[1])
	}
}

func TestListConversationMessagesCommandDefaultsToNewestTwentyInChronologicalOrder(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	now := time.Now().Add(2 * time.Minute)
	for i := 3; i <= 25; i++ {
		mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("msg-%02d", i), "ws-1", "conv-1", "customer", fmt.Sprintf("message %02d", i), false, now.Add(time.Duration(i)*time.Minute), now.Add(time.Duration(i)*time.Minute))
	}

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1", TargetType: "workspace", TargetID: "ws-1",
	}, "support.list_conversation_messages", json.RawMessage(`{"conversation_id":"conv-1"}`))
	if err != nil {
		t.Fatalf("list newest messages: %v", err)
	}
	var result struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
		Limit      int  `json:"limit"`
		NextOffset *int `json:"next_offset"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(result.Messages) != 20 || result.Limit != 20 {
		t.Fatalf("expected newest 20 messages, got %#v", result)
	}
	if result.Messages[0].Content != "message 06" || result.Messages[19].Content != "message 25" {
		t.Fatalf("expected chronological newest window, got first=%q last=%q", result.Messages[0].Content, result.Messages[19].Content)
	}
	if result.NextOffset == nil || *result.NextOffset != 20 {
		t.Fatalf("expected next_offset 20, got %#v", result.NextOffset)
	}
}

func TestListConversationMessagesCommandReturnsOlderPagesAndSafeAttachments(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	mustExec(t, db, `CREATE TABLE support_attachments (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, message_id TEXT, file_name TEXT NOT NULL, file_size INTEGER NOT NULL, content_type TEXT NOT NULL, storage_key TEXT NOT NULL, public_url TEXT NOT NULL, is_uploaded BOOLEAN NOT NULL, uploaded_by_type TEXT NOT NULL, created_at DATETIME)`)
	mustExec(t, db, `INSERT INTO support_attachments (id, workspace_id, conversation_id, message_id, file_name, file_size, content_type, storage_key, public_url, is_uploaded, uploaded_by_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"att-1", "ws-1", "conv-1", "msg-1", "error.png", 2048, "image/png", "private/ws-1/error.png", "https://files.example/error.png", true, "customer", time.Now())

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc.SetSupportAttachmentRepository(repository.NewSupportAttachmentRepository(db))

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1", TargetType: "workspace", TargetID: "ws-1",
	}, "support.list_conversation_messages", json.RawMessage(`{"conversation_id":"conv-1","limit":1,"offset":1}`))
	if err != nil {
		t.Fatalf("list older messages: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	messages := decoded["messages"].([]any)
	message := messages[0].(map[string]any)
	if message["content"] != "The widget will not load." {
		t.Fatalf("expected older message, got %#v", message)
	}
	attachments := message["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	if attachment["file_name"] != "error.png" || attachment["file_type"] != "image/png" || attachment["url"] != "https://files.example/error.png" {
		t.Fatalf("unexpected attachment %#v", attachment)
	}
	if _, exposed := attachment["file_key"]; exposed {
		t.Fatalf("private storage key must not be exposed: %#v", attachment)
	}
}

func TestListConversationMessagesCommandRequiresConversationTarget(t *testing.T) {
	db := newTestDB(t)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)

	def, ok := svc.Definition("support.list_conversation_messages")
	if !ok {
		t.Fatal("expected support.list_conversation_messages definition")
	}
	_, err := def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "no support conversation") {
		t.Fatalf("expected missing conversation error, got %v", err)
	}
}

func TestListConversationMessagesCommandDefaultsToSupportContextAttachedToAskChat(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	createProductToolAgentRunTables(t, db)
	mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN dock_chat_id TEXT`)
	mustExec(t, db, `CREATE TABLE dock_chats (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		title TEXT, visibility TEXT NOT NULL DEFAULT 'private', module_id TEXT,
		support_conversation_id TEXT, active_run_id TEXT, last_message_at DATETIME,
		archived_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`)
	mustExec(t, db, `INSERT INTO dock_chats (id, workspace_id, user_id, support_conversation_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, "chat-1", "ws-1", "user-1", "conv-1", time.Now(), time.Now())
	mustExec(t, db, `UPDATE agent_runs SET target_type = ?, target_id = ?, dock_chat_id = ?, external_runtime_id = ? WHERE id = ?`,
		"workspace", "ws-1", "chat-1", "rt-chat-1", "run-1")

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc.SetAgentRunDependencies(repository.NewAgentRunRepository(db), nil)
	svc.SetDockChatRepository(repository.NewDockChatRepository(db))

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1", RunID: "rt-chat-1", TargetType: "workspace", TargetID: "ws-1",
	}, "support.list_conversation_messages", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list conversation messages from attached Ask context: %v", err)
	}
	var result struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2", result.Total)
	}
}

func TestListConversationMessagesCommandDefaultsToPersistedAttachedContext(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	createProductToolAgentRunTables(t, db)
	mustExec(t, db, `UPDATE agent_runs SET target_type = ?, target_id = ?, external_runtime_id = ?, input = ? WHERE id = ?`,
		"workspace", "ws-1", "rt-attached-1", []byte(`{"trigger":{"context":{"dock_chat_id":"chat-1","attached_contexts":[{"entity_type":"support_conversation","entity_id":"conv-1"}]}}}`), "run-1")

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc.SetAgentRunDependencies(repository.NewAgentRunRepository(db), nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1", RunID: "rt-attached-1", TargetType: "workspace", TargetID: "ws-1",
	}, "support.list_conversation_messages", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list conversation messages from persisted attached context: %v", err)
	}
	var result struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2", result.Total)
	}
}

func TestUpdateConversationStatusCommandUpdatesStatus(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "support_conversation",
		TargetID:    "conv-1",
	}, "support.update_conversation_status", json.RawMessage(`{"status":"resolved"}`))
	if err != nil {
		t.Fatalf("support.update_conversation_status returned error: %v", err)
	}
	var result struct {
		ConversationID string `json:"conversation_id"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.ConversationID != "conv-1" || result.Status != "resolved" {
		t.Fatalf("unexpected result %#v", result)
	}
	var status string
	if err := db.Raw(`SELECT status FROM support_conversations WHERE id = ?`, "conv-1").Scan(&status).Error; err != nil {
		t.Fatalf("load conversation status: %v", err)
	}
	if status != "resolved" {
		t.Fatalf("conversation status = %q, want resolved", status)
	}
}

func createProductToolAgentRunTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			model_tier TEXT NOT NULL DEFAULT '',
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			status TEXT NOT NULL DEFAULT 'queued',
			external_runtime TEXT,
			external_runtime_id TEXT,
			input BLOB NOT NULL DEFAULT '{}',
			output_summary BLOB NOT NULL DEFAULT '{}',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL DEFAULT 'text',
			storage_mode TEXT NOT NULL DEFAULT 'inline',
			inline_content TEXT,
			object_key TEXT,
			metadata BLOB NOT NULL DEFAULT '{}',
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create agent run tables: %v", err)
		}
	}
	now := time.Now()
	mustExec(t, db, `INSERT INTO agent_runs (id, workspace_id, agent_id, target_type, target_id, status, external_runtime, external_runtime_id, input, output_summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-1", "ws-1", "agent-1", "support_conversation", "conv-1", "running", "agent-runtime", "rt-run-1", []byte(`{}`), []byte(`{"result":"partial"}`), now, now)
}

func TestDraftSupportReplyCommandStagesDraftOnRunOutputSummary(t *testing.T) {
	db := newTestDB(t)
	createProductToolAgentRunTables(t, db)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetAgentRunDependencies(repository.NewAgentRunRepository(db), repository.NewAgentRunArtifactRepository(db))

	// RunID carries the external runtime run ID, as sent by the runtime host executor.
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "rt-run-1",
		TargetType:  "support_conversation",
		TargetID:    "conv-1",
	}, "support.draft_reply", json.RawMessage(`{"content":"  Hi, we are on it!  ","is_internal":false,"sender_display_name":"Echo"}`))
	if err != nil {
		t.Fatalf("support.draft_reply returned error: %v", err)
	}
	var result struct {
		Status string `json:"status"`
		RunID  string `json:"run_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.Status != "drafted" || result.RunID != "run-1" {
		t.Fatalf("unexpected result %#v", result)
	}

	var rawSummary string
	if err := db.Raw(`SELECT output_summary FROM agent_runs WHERE id = ?`, "run-1").Scan(&rawSummary).Error; err != nil {
		t.Fatalf("load run output summary: %v", err)
	}
	var summary struct {
		Result     string `json:"result"`
		DraftReply *struct {
			Content           string `json:"content"`
			IsInternal        bool   `json:"is_internal"`
			SenderDisplayName string `json:"sender_display_name"`
			ApprovalRequired  bool   `json:"approval_required"`
		} `json:"draft_reply"`
	}
	if err := json.Unmarshal([]byte(rawSummary), &summary); err != nil {
		t.Fatalf("unmarshal run output summary: %v\n%s", err, rawSummary)
	}
	if summary.Result != "partial" {
		t.Fatalf("expected existing summary keys to be preserved, got %s", rawSummary)
	}
	if summary.DraftReply == nil || summary.DraftReply.Content != "Hi, we are on it!" || !summary.DraftReply.ApprovalRequired {
		t.Fatalf("unexpected staged draft %s", rawSummary)
	}
	if summary.DraftReply.SenderDisplayName != "Echo" || summary.DraftReply.IsInternal {
		t.Fatalf("unexpected draft attributes %s", rawSummary)
	}
}

func TestDraftSupportReplyCommandRequiresContent(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	def, ok := svc.Definition("support.draft_reply")
	if !ok {
		t.Fatal("expected support.draft_reply definition")
	}
	_, err := def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1", RunID: "run-1"}, json.RawMessage(`{"content":"  "}`))
	if err == nil || !strings.Contains(err.Error(), "content is required") {
		t.Fatalf("expected content required error, got %v", err)
	}
}

func createProductToolCRMTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE crm_pipelines (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_pipeline_stages (
			id TEXT PRIMARY KEY,
			pipeline_id TEXT NOT NULL,
			name TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_deals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			pipeline_id TEXT NOT NULL,
			stage_id TEXT NOT NULL,
			amount REAL,
			currency TEXT NOT NULL DEFAULT 'USD',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_buyer_signals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			company_id TEXT,
			signal_type TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'manual',
			source_id TEXT,
			source_thread_id TEXT,
			summary TEXT NOT NULL,
			evidence_excerpt TEXT,
			metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			confidence REAL NOT NULL DEFAULT 0,
			detected_at DATETIME NOT NULL,
			detector_kind TEXT NOT NULL DEFAULT 'direct',
			signal_domain TEXT NOT NULL DEFAULT 'conversation',
			polarity TEXT NOT NULL DEFAULT 'neutral',
			rule_key TEXT,
			rule_version TEXT,
			window_started_at DATETIME,
			window_ended_at DATETIME,
			evidence_identity_method TEXT NOT NULL DEFAULT 'unknown',
			evidence_identity_trust TEXT NOT NULL DEFAULT 'untrusted',
			evidence_fingerprint TEXT NOT NULL DEFAULT '',
			dismissed_at DATETIME,
			dismissed_by_member_id TEXT,
			dismissal_reason TEXT,
			reviewed_at DATETIME,
			acted_at DATETIME,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create crm tables: %v", err)
		}
	}
	now := time.Now()
	mustExec(t, db, `INSERT INTO crm_pipelines (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"pipe-1", "ws-1", "Sales", now, now)
	mustExec(t, db, `INSERT INTO crm_pipeline_stages (id, pipeline_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"stage-1", "pipe-1", "Qualified", now, now)
	mustExec(t, db, `INSERT INTO crm_deals (id, workspace_id, name, pipeline_id, stage_id, amount, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"deal-1", "ws-1", "Acme expansion", "pipe-1", "stage-1", 4200.0, now, now)
	mustExec(t, db, `INSERT INTO crm_buyer_signals (id, workspace_id, deal_id, signal_type, summary, confidence, detected_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"signal-1", "ws-1", "deal-1", "buying_intent", "Asked for pricing", 0.92, now, now)
	mustExec(t, db, `INSERT INTO crm_buyer_signals (id, workspace_id, deal_id, signal_type, summary, confidence, detected_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"signal-2", "ws-1", "deal-2", "risk_signal", "Went quiet", 0.71, now, now)
}

func TestCRMListDealsCommandReturnsDealSummaries(t *testing.T) {
	db := newTestDB(t)
	createProductToolCRMTables(t, db)

	dealService := NewCRMDealService(repository.NewCRMDealRepository(db), repository.NewCRMAssociationRepository(db))
	svc := NewInternalCommandService(nil, nil, dealService, nil, nil, nil, nil, nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "crm.list_deals", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("crm.list_deals returned error: %v", err)
	}
	var result struct {
		Deals []struct {
			ID           string   `json:"id"`
			MarkdownLink string   `json:"markdown_link"`
			Name         string   `json:"name"`
			Stage        string   `json:"stage"`
			Amount       *float64 `json:"amount"`
		} `json:"deals"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, string(output))
	}
	if len(result.Deals) != 1 || result.Deals[0].ID != "deal-1" || result.Deals[0].Stage != "Qualified" {
		t.Fatalf("unexpected deals %#v", result.Deals)
	}
	if result.Deals[0].MarkdownLink != "[Acme expansion](helpin://deals/deal-1)" {
		t.Fatalf("deal markdown_link = %q", result.Deals[0].MarkdownLink)
	}
	if result.Deals[0].Amount == nil || *result.Deals[0].Amount != 4200.0 {
		t.Fatalf("unexpected deal amount %#v", result.Deals[0])
	}
	filtered, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1", TargetType: "workspace", TargetID: "ws-1"}, "crm.list_deals", json.RawMessage(`{"query":"no-such-deal"}`))
	if err != nil {
		t.Fatalf("query crm.list_deals returned error: %v", err)
	}
	var filteredResult struct {
		Deals []struct {
			ID string `json:"id"`
		} `json:"deals"`
	}
	if err := json.Unmarshal(filtered, &filteredResult); err != nil {
		t.Fatalf("unmarshal filtered deals: %v", err)
	}
	if len(filteredResult.Deals) != 0 {
		t.Fatalf("unexpected filtered deals %#v", filteredResult.Deals)
	}
}

func TestCRMListContactsCommandReturnsContactSummaries(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	mustExec(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, last_name, email, job_title, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"contact-1", "ws-1", "C-1", "Ada", "Lovelace", "ada@example.com", "CTO", now, now)
	mustExec(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, last_name, email, job_title, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"contact-2", "ws-1", "C-2", "Grace", "Hopper", "grace@example.com", "Admiral", now.Add(time.Second), now.Add(time.Second))

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetCRMReadServices(NewCRMContactService(repository.NewCRMContactRepository(db)), nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "crm.list_contacts", json.RawMessage(`{"query":"ada","limit":5}`))
	if err != nil {
		t.Fatalf("crm.list_contacts returned error: %v", err)
	}
	var result struct {
		Contacts []struct {
			ID           string  `json:"id"`
			MarkdownLink string  `json:"markdown_link"`
			FirstName    string  `json:"first_name"`
			LastName     *string `json:"last_name"`
			Email        *string `json:"email"`
			JobTitle     *string `json:"job_title"`
		} `json:"contacts"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, string(output))
	}
	if len(result.Contacts) != 1 || result.Contacts[0].FirstName != "Ada" || result.Contacts[0].Email == nil || *result.Contacts[0].Email != "ada@example.com" {
		t.Fatalf("unexpected contacts %#v", result.Contacts)
	}
	if result.Contacts[0].MarkdownLink != "[Ada Lovelace](helpin://contacts/contact-1)" {
		t.Fatalf("contact markdown_link = %q", result.Contacts[0].MarkdownLink)
	}
}

func TestCRMListBuyerSignalsCommandFiltersByDeal(t *testing.T) {
	db := newTestDB(t)
	createProductToolCRMTables(t, db)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetCRMReadServices(nil, NewCRMSignalService(repository.NewCRMSignalRepository(db), nil))

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "crm.list_buyer_signals", json.RawMessage(`{"deal_id":"deal-1"}`))
	if err != nil {
		t.Fatalf("crm.list_buyer_signals returned error: %v", err)
	}
	var result struct {
		Signals []struct {
			ID         string  `json:"id"`
			SignalType string  `json:"signal_type"`
			Summary    string  `json:"summary"`
			Confidence float64 `json:"confidence"`
			DealID     *string `json:"deal_id"`
		} `json:"buyer_signals"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, string(output))
	}
	if len(result.Signals) != 1 || result.Signals[0].ID != "signal-1" || result.Signals[0].SignalType != "buying_intent" || result.Signals[0].Confidence != 0.92 {
		t.Fatalf("unexpected signals %#v", result.Signals)
	}
}

func TestSearchDocumentsCommandRequiresQuery(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	def, ok := svc.Definition("docs.search_documents")
	if !ok {
		t.Fatal("expected docs.search_documents definition")
	}
	_, err := def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{"query":"  "}`))
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query required error, got %v", err)
	}
	_, err = def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{"query":"runbook"}`))
	if err == nil || !strings.Contains(err.Error(), "docs search is not available") {
		t.Fatalf("expected unavailable search error when repo missing, got %v", err)
	}
}

type fakeReleaseFactsProvider struct {
	releaseReq  *model.GetReleaseContextRequest
	findReq     *model.FindTasksForGitChangesRequest
	taskReq     *model.GetTaskContextRequest
	workspaceID string
}

func (f *fakeReleaseFactsProvider) GetReleaseContext(_ context.Context, workspaceID string, req model.GetReleaseContextRequest) (*model.ReleaseContextResult, error) {
	f.workspaceID = workspaceID
	f.releaseReq = &req
	return &model.ReleaseContextResult{RepoFullName: req.RepoFullName, CurrentRelease: model.ReleaseSummary{TagName: req.TagName}}, nil
}

func (f *fakeReleaseFactsProvider) FindTasksForGitChanges(_ context.Context, workspaceID string, req model.FindTasksForGitChangesRequest) (*model.FindTasksForGitChangesResult, error) {
	f.workspaceID = workspaceID
	f.findReq = &req
	return &model.FindTasksForGitChangesResult{Matches: []model.GitChangeTaskMatch{{TaskID: "task-1", Confidence: "high"}}}, nil
}

func (f *fakeReleaseFactsProvider) GetTaskContext(_ context.Context, workspaceID string, req model.GetTaskContextRequest) (*model.GetTaskContextResult, error) {
	f.workspaceID = workspaceID
	f.taskReq = &req
	return &model.GetTaskContextResult{}, nil
}

func TestReleaseGetReleaseContextCommandDefaultsRepositoryFromTarget(t *testing.T) {
	provider := &fakeReleaseFactsProvider{}
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetReleaseFactsProvider(provider)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "repository",
		TargetID:    "repo-1",
	}, "release.get_release_context", json.RawMessage(`{"tag_name":"v1.2.0"}`))
	if err != nil {
		t.Fatalf("release.get_release_context returned error: %v", err)
	}
	if provider.releaseReq == nil || provider.releaseReq.RepositoryID != "repo-1" || provider.releaseReq.TagName != "v1.2.0" {
		t.Fatalf("unexpected provider request %#v", provider.releaseReq)
	}
	if provider.workspaceID != "ws-1" {
		t.Fatalf("workspace = %q, want ws-1", provider.workspaceID)
	}
	var result struct {
		CurrentRelease struct {
			TagName string `json:"tag_name"`
		} `json:"current_release"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.CurrentRelease.TagName != "v1.2.0" {
		t.Fatalf("unexpected output %s", string(output))
	}
}

func TestReleaseGetReleaseContextCommandRequiresTagAndRepository(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetReleaseFactsProvider(&fakeReleaseFactsProvider{})
	def, _ := svc.Definition("release.get_release_context")

	_, err := def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{"repo_full_name":"acme/app"}`))
	if err == nil || !strings.Contains(err.Error(), "tag_name is required") {
		t.Fatalf("expected tag_name error, got %v", err)
	}
	_, err = def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{"tag_name":"v1.0.0"}`))
	if err == nil || !strings.Contains(err.Error(), "repository_id or repo_full_name is required") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestReleaseFindTasksForGitChangesCommandRequiresEvidence(t *testing.T) {
	provider := &fakeReleaseFactsProvider{}
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetReleaseFactsProvider(provider)
	def, _ := svc.Definition("release.find_tasks_for_git_changes")

	_, err := def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{"repo_full_name":"acme/app"}`))
	if err == nil || !strings.Contains(err.Error(), "at least one evidence array is required") {
		t.Fatalf("expected evidence error, got %v", err)
	}

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "repository",
		TargetID:    "repo-1",
	}, "release.find_tasks_for_git_changes", json.RawMessage(`{"pr_numbers":[41]}`))
	if err != nil {
		t.Fatalf("release.find_tasks_for_git_changes returned error: %v", err)
	}
	if provider.findReq == nil || provider.findReq.RepositoryID != "repo-1" || len(provider.findReq.PRNumbers) != 1 {
		t.Fatalf("unexpected provider request %#v", provider.findReq)
	}
	if !strings.Contains(string(output), `"task-1"`) {
		t.Fatalf("unexpected output %s", string(output))
	}
}

func TestReleaseGetTaskContextCommandRequiresTaskIDs(t *testing.T) {
	provider := &fakeReleaseFactsProvider{}
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetReleaseFactsProvider(provider)
	def, _ := svc.Definition("release.get_task_context")
	properties := def.Tool.InputSchema["properties"].(map[string]any)
	taskIDs := properties["task_ids"].(map[string]any)
	if taskIDs["maxItems"] != 50 {
		t.Fatalf("task_ids maxItems = %#v, want 50", taskIDs["maxItems"])
	}
	for _, field := range def.Tool.InputSchema["required"].([]string) {
		if field == "task_ids" {
			t.Fatal("task_ids must be optional so a task target can supply the default")
		}
	}

	_, err := def.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1"}, json.RawMessage(`{"task_ids":[" "]}`))
	if err == nil || !strings.Contains(err.Error(), "task_ids or task_keys is required") {
		t.Fatalf("expected task_ids error, got %v", err)
	}
	if _, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "release.get_task_context", json.RawMessage(`{"task_ids":["task-9"],"include_comments":true}`)); err != nil {
		t.Fatalf("release.get_task_context returned error: %v", err)
	}
	if provider.taskReq == nil || len(provider.taskReq.TaskIDs) != 1 || provider.taskReq.TaskIDs[0] != "task-9" || !provider.taskReq.IncludeComments {
		t.Fatalf("unexpected provider request %#v", provider.taskReq)
	}
}

func TestPublishPRDDraftCommandPersistsRunPreviewArtifact(t *testing.T) {
	db := newTestDB(t)
	createProductToolAgentRunTables(t, db)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetAgentRunDependencies(repository.NewAgentRunRepository(db), repository.NewAgentRunArtifactRepository(db))

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "rt-run-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
	}, "docs.publish_prd_draft", json.RawMessage(`{"content":"# PRD\n\nGoal."}`))
	if err != nil {
		t.Fatalf("docs.publish_prd_draft returned error: %v", err)
	}
	var result struct {
		Status   string `json:"status"`
		PanelKey string `json:"panel_key"`
		Format   string `json:"format"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.Status != "published" || result.PanelKey != "prd_draft" || result.Format != "markdown" {
		t.Fatalf("unexpected result %#v", result)
	}

	var artifact struct {
		ArtifactType  string
		Format        string
		InlineContent string
		SequenceNo    int
	}
	if err := db.Raw(`SELECT artifact_type, format, inline_content, sequence_no FROM agent_run_artifacts WHERE run_id = ?`, "run-1").Scan(&artifact).Error; err != nil {
		t.Fatalf("load run artifact: %v", err)
	}
	if artifact.ArtifactType != "run_preview" || artifact.Format != "json" || artifact.SequenceNo != 1 {
		t.Fatalf("unexpected artifact %#v", artifact)
	}
	if !strings.Contains(artifact.InlineContent, `"panel_key":"prd_draft"`) || !strings.Contains(artifact.InlineContent, "# PRD") {
		t.Fatalf("unexpected artifact content %s", artifact.InlineContent)
	}
}

func TestPublishTaskPlanDocCommandRequiresContent(t *testing.T) {
	db := newTestDB(t)
	createProductToolAgentRunTables(t, db)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetAgentRunDependencies(repository.NewAgentRunRepository(db), repository.NewAgentRunArtifactRepository(db))
	def, _ := svc.Definition("docs.publish_task_plan_doc")

	_, err := def.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		RunID:       "rt-run-1",
	}, json.RawMessage(`{"title":"Plan only"}`))
	if err == nil || !strings.Contains(err.Error(), "missing content") {
		t.Fatalf("expected missing content error, got %v", err)
	}
}

func TestPublishDocumentChangeProposalCommandCreatesPendingProposal(t *testing.T) {
	db := newTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'published',
			deleted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_change_proposals (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			block_id TEXT,
			agent_id TEXT,
			agent_run_id TEXT,
			scope TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			revision INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL,
			content_markdown TEXT NOT NULL,
			base_markdown TEXT NOT NULL DEFAULT '',
			content BLOB NOT NULL,
			sources BLOB NOT NULL DEFAULT '[]',
			created_by TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs proposal tables: %v", err)
		}
	}
	now := time.Now()
	mustExec(t, db, `INSERT INTO docs_documents (id, workspace_id, title, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"doc-1", "ws-1", "Runbook", "published", now, now)

	proposalService := NewDocsChangeProposalService(
		repository.NewDocsChangeProposalRepository(db),
		repository.NewDocsDocumentRepository(db, false),
		nil,
		nil,
		nil,
		nil,
	)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetDocsChangeProposalService(proposalService)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		TargetType:  "document",
		TargetID:    "doc-1",
	}, "docs.publish_document_change_proposal", json.RawMessage(`{"scope":"document","document_id":"doc-1","content":"# Runbook v2\n\nUpdated steps.","summary":"Refresh runbook"}`))
	if err != nil {
		t.Fatalf("docs.publish_document_change_proposal returned error: %v", err)
	}
	var result struct {
		Status     string `json:"status"`
		ProposalID string `json:"proposal_id"`
		Scope      string `json:"scope"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.Status != "submitted" || result.ProposalID == "" || result.Scope != "document" {
		t.Fatalf("unexpected result %#v", result)
	}

	var row struct {
		Status          string
		Scope           string
		Summary         string
		ContentMarkdown string
		AgentID         string
		CreatedBy       string
	}
	if err := db.Raw(`SELECT status, scope, summary, content_markdown, agent_id, created_by FROM docs_change_proposals WHERE id = ?`, result.ProposalID).Scan(&row).Error; err != nil {
		t.Fatalf("load proposal: %v", err)
	}
	if row.Status != "pending" || row.Scope != "document" || row.Summary != "Refresh runbook" {
		t.Fatalf("unexpected proposal row %#v", row)
	}
	if row.AgentID != "agent-1" || row.CreatedBy != "agent-1" {
		t.Fatalf("unexpected proposal attribution %#v", row)
	}
	if !strings.Contains(row.ContentMarkdown, "# Runbook v2") {
		t.Fatalf("unexpected proposal markdown %q", row.ContentMarkdown)
	}
}

func TestPublishDocumentChangeProposalCommandRejectsMismatchedTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetDocsChangeProposalService(&DocsChangeProposalService{})
	def, _ := svc.Definition("docs.publish_document_change_proposal")

	_, err := def.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "document",
		TargetID:    "doc-1",
	}, json.RawMessage(`{"scope":"document","document_id":"doc-2","content":"body","summary":"s"}`))
	if err == nil || !strings.Contains(err.Error(), "does not match this run target") {
		t.Fatalf("expected target mismatch error, got %v", err)
	}
}
