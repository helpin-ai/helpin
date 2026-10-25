package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func coverageNote(id string, at time.Time, metadata string) model.SupportMessage {
	return model.SupportMessage{ID: id, WorkspaceID: "ws", ConversationID: "conv", SenderType: "agent", MessageType: "note", IsInternal: true, Metadata: metadata, CreatedAt: at}
}

func TestCoverageAIReportedMissingData(t *testing.T) {
	start := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	deleted := coverageNote("deleted", start.Add(5*time.Minute), `{"handoff":{"reason":"x","unavailable_data":["Shipping status"]}}`)
	deleted.DeletedAt = gorm.DeletedAt{Time: start, Valid: true}
	messages := []model.SupportMessage{
		coverageNote("before", start.Add(-time.Hour), `{"handoff":{"reason":"x","unavailable_data":["Old data"]}}`),
		coverageNote("handoff", start.Add(time.Minute), `{"ai_handoff_brief":true,"handoff":{"reason":"x","unavailable_data":["Payment history"]}}`),
		coverageNote("check", start.Add(2*time.Minute), `{"code_check":{"status":"done","unavailable_data":["Server error logs","payment history"]}}`),
		deleted,
		{ID: "public", SenderType: "customer", MessageType: "reply", Metadata: `{"handoff":{"unavailable_data":["Not a note"]}}`, CreatedAt: start.Add(3 * time.Minute)},
		coverageNote("after", end.Add(time.Minute), `{"code_check":{"status":"done","unavailable_data":["Later data"]}}`),
	}
	got := coverageAIReportedMissingData(messages, CoverageConversationSegment{StartAt: start, EndAt: &end})
	want := []string{"Payment history", "Server error logs"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("missing data = %q, want %q", got, want)
	}
}

func TestBuildCoverageAnalysisInputCarriesAIReportedMissingData(t *testing.T) {
	start := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	conversation := model.SupportConversation{ID: "conv", WorkspaceID: "ws", Status: model.SupportConversationStatusResolved}
	messages := []model.SupportMessage{
		{ID: "customer", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "Can I get a refund?", CreatedAt: start},
		{ID: "ai", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Let me connect you with a team member.", CreatedAt: start.Add(time.Minute)},
		coverageNote("handoff", start.Add(time.Minute+time.Millisecond), `{"ai_handoff_brief":true,"handoff":{"reason":"x","unavailable_data":["Payment history"]}}`),
		{ID: "human", WorkspaceID: "ws", ConversationID: "conv", SenderType: "user", MessageType: "reply", Content: "Refunded.", CreatedAt: start.Add(time.Hour)},
	}
	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.AIReportedMissingData) != 1 || input.AIReportedMissingData[0] != "Payment history" {
		t.Fatalf("missing data = %q", input.AIReportedMissingData)
	}
	for _, message := range input.Messages {
		if message.ID == "handoff" {
			t.Fatal("internal note reached the analyzer transcript")
		}
	}
}

func TestCoverageConnectedTools(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE external_mcp_servers(id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, server_name TEXT, enabled BOOLEAN, created_at DATETIME)`)
	mustExec(t, db, `CREATE TABLE external_mcp_tools(id TEXT PRIMARY KEY, workspace_id TEXT, server_id TEXT, remote_name TEXT, runtime_alias TEXT, description TEXT, access TEXT, enabled BOOLEAN)`)
	mustExec(t, db, `INSERT INTO external_mcp_servers VALUES ('billing','ws','Billing','billing',true,?),('off','ws','Old CRM','old_crm',false,?)`, time.Now(), time.Now())
	mustExec(t, db, `INSERT INTO external_mcp_tools VALUES ('t1','ws','billing','list_invoices','mcp__billing__list_invoices','List a customer''s invoices and payments.','read',true),('t2','ws','billing','delete_invoice','mcp__billing__delete_invoice','Delete an invoice.','write',false),('t3','ws','off','find_contact','mcp__old_crm__find_contact','Find a contact.','read',true)`)
	analyzer := &SupportCoverageDailyAnalyzer{conversationRepo: repository.NewSupportConversationRepository(db)}
	got := analyzer.coverageConnectedTools(context.Background(), "ws")
	if len(got) != 1 || got[0].Name != "Billing" || len(got[0].Tools) != 1 || got[0].Tools[0] != "list_invoices: List a customer's invoices and payments." {
		t.Fatalf("connected tools = %+v", got)
	}
}

func TestCoverageAnalysisPromptExplainsMissingData(t *testing.T) {
	prompt := coverageConversationAnalysisSystemPrompt()
	for _, want := range []string{"ai_reported_missing_data", "connected_tools", "gap_category=evaluation"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
