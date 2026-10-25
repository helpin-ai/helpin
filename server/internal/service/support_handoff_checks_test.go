package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func setupSupportHandoffCheckTest(t *testing.T, calls ...AgentRuntimeToolCall) (*InternalCommandService, *gorm.DB, model.InternalCommandContext) {
	t.Helper()
	chat, commands, db, _, _, meta := setupSupportProgressTest(t)
	chat.supportAIService.handoffRepo = repository.NewAgentHandoffRepository(db)
	mustExec(t, db, `CREATE TABLE external_mcp_servers(id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, server_name TEXT, enabled BOOLEAN, created_at DATETIME)`)
	mustExec(t, db, `CREATE TABLE external_mcp_tools(id TEXT PRIMARY KEY, workspace_id TEXT, server_id TEXT, remote_name TEXT, runtime_alias TEXT, access TEXT, enabled BOOLEAN)`)
	mustExec(t, db, `INSERT INTO external_mcp_servers VALUES ('server','ws','Post logs','logs',true,?)`, time.Now())
	mustExec(t, db, `INSERT INTO external_mcp_tools VALUES ('tool','ws','server','lookup','mcp__logs__lookup','read',true)`)
	for i := range calls {
		calls[i].RunID, calls[i].CreatedAt = "runtime-run", time.Now()
	}
	runtime := &fakeAgentRuntimeSignalClient{toolCalls: map[string][]AgentRuntimeToolCall{"runtime-run": calls}}
	commands.agentService = &AgentService{agentRuntimeClient: runtime, externalMCPService: &ExternalMCPService{repo: repository.NewExternalMCPRepository(db)}}
	return commands, db, meta
}

func TestSupportHandoffChecksUseSourcesTheRunUsed(t *testing.T) {
	commands, _, meta := setupSupportHandoffCheckTest(t,
		AgentRuntimeToolCall{ID: "search", ToolName: "mcp__helpin__search_knowledge", Output: json.RawMessage(`{}`)},
		AgentRuntimeToolCall{ID: "logs", ToolName: "mcp__logs__lookup", Output: json.RawMessage(`{}`)},
		AgentRuntimeToolCall{ID: "tasks", ToolName: "mcp__helpin__list_tasks", Error: "timeout"},
	)
	findings := []supportHandoffFinding{
		{Tool: "search_knowledge", Finding: "Nothing about approving drafts."},
		{Tool: "list_tasks", Finding: "A task already covers this."},
		{Finding: "Suggested opening the post again."},
	}
	checks, used, known := commands.supportHandoffChecks(context.Background(), meta, findings)
	want := []SupportHandoffCheck{
		{Source: "Help articles", Finding: "Nothing about approving drafts."},
		{Finding: "Suggested opening the post again."},
		{Source: "Post logs"},
	}
	if !known || !used["search_knowledge"] || used["list_tasks"] || len(checks) != len(want) {
		t.Fatalf("checks = %+v used = %v known = %v", checks, used, known)
	}
	for i := range want {
		if checks[i] != want[i] {
			t.Errorf("check %d = %+v, want %+v", i, checks[i], want[i])
		}
	}
}

func TestSupportHandoffChecksWithoutToolCallsKeepFindingsUnsourced(t *testing.T) {
	_, commands, _, _, _, meta := setupSupportProgressTest(t)
	checks, used, known := commands.supportHandoffChecks(context.Background(), meta, []supportHandoffFinding{{Tool: "search_knowledge", Finding: "Nothing about exports."}})
	if known || len(used) != 0 || len(checks) != 1 || checks[0] != (SupportHandoffCheck{Finding: "Nothing about exports."}) {
		t.Fatalf("checks = %+v used = %v known = %v", checks, used, known)
	}
}

func savedHandoffCard(t *testing.T, db *gorm.DB) supportHandoffCard {
	t.Helper()
	var note model.SupportMessage
	if err := db.Where("message_type = 'note' AND sender_display_name = 'AI control'").First(&note).Error; err != nil {
		t.Fatalf("handoff note: %v", err)
	}
	return handoffCardOf(t, &note)
}

func TestSupportEscalateKeepsDocsGapOnlyAfterSearch(t *testing.T) {
	tests := []struct {
		name    string
		calls   []AgentRuntimeToolCall
		wantGap string
	}{
		{name: "searched help articles", calls: []AgentRuntimeToolCall{{ID: "search", ToolName: "mcp__helpin__search_knowledge", Output: json.RawMessage(`{}`)}}, wantGap: "How to export reports to CSV"},
		{name: "never searched", wantGap: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, db, meta := setupSupportHandoffCheckTest(t, tt.calls...)
			input := json.RawMessage(`{"reason":"cannot_answer","issue_summary":"Wants to export reports to CSV.","checked":[{"tool":"search_knowledge","finding":"Nothing about exporting reports."}],"help_article_gap":"How to export reports to CSV","unavailable_data":["Report export history"]}`)
			if _, err := commands.executeSupportEscalate(context.Background(), meta, input); err != nil {
				t.Fatal(err)
			}
			card := savedHandoffCard(t, db)
			if card.HelpArticleGap != tt.wantGap || card.Issue != "Wants to export reports to CSV." || len(card.UnavailableData) != 1 {
				t.Fatalf("card = %+v", card)
			}
			if tt.wantGap == "" && (len(card.Checked) != 0 || !card.NothingChecked) {
				t.Fatalf("unbacked finding kept: %+v", card.Checked)
			}
		})
	}
}

func TestSupportReplyCheckFailureHoldsOnlySafeReplies(t *testing.T) {
	tests := []struct {
		name     string
		review   string
		wantHeld string
	}{
		{name: "unsupported but about this customer", review: `{"customer_scoped":true,"supported":false,"safe":true,"confidence":0.96}`, wantHeld: "Hi,\n\nYour import failed because of duplicate headers."},
		{name: "about someone else", review: `{"customer_scoped":false,"supported":true,"safe":true,"confidence":0.96}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, db, meta := setupSupportHandoffCheckTest(t, AgentRuntimeToolCall{ID: "logs", ToolName: "mcp__logs__lookup", Input: json.RawMessage(`{"email":"customer@example.com"}`), Output: json.RawMessage(`{"email":"customer@example.com","status":"failed"}`)})
			mustExec(t, db, `UPDATE support_conversations SET customer_email='customer@example.com' WHERE id='conv'`)
			mustExec(t, db, `INSERT INTO support_widget_sessions(id,workspace_id,conversation_id,session_token,customer_email,identity_method,identity_trust,expires_at,created_at) VALUES ('session','ws','conv','token','customer@example.com','signed_widget',?,?,?)`, model.IdentityTrustVerified, time.Now().Add(time.Hour), time.Now())
			commands.supportAIService.llmProvider = &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: tt.review}}
			input := json.RawMessage(`{"content":"Hi,\nYour import failed because of duplicate headers.","reply_kind":"answer","confidence":0.95,"source_doc_ids":["mcp__logs__lookup"],"claims":[{"text":"Your import failed because of duplicate headers.","evidence_ids":["mcp__logs__lookup"]}]}`)
			output, err := commands.executeSupportSendReply(context.Background(), meta, input)
			if err != nil {
				t.Fatal(err)
			}
			var outcome struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(output, &outcome) != nil || outcome.Status != "escalated" {
				t.Fatalf("outcome = %s", output)
			}
			card := savedHandoffCard(t, db)
			if card.HeldReply != tt.wantHeld {
				t.Fatalf("held reply = %q, want %q", card.HeldReply, tt.wantHeld)
			}
			if len(card.Checked) != 1 || card.Checked[0] != (SupportHandoffCheck{Source: "Post logs"}) {
				t.Fatalf("checked = %+v", card.Checked)
			}
		})
	}
}
