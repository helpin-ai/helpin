package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportMCPResultBecomesPrivateCitableEvidence(t *testing.T) {
	db := setupSupportKnowledgeTestDB(t)
	for _, ddl := range []string{
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, customer_email TEXT, customer_phone TEXT, crm_contact_id TEXT)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, sender_type TEXT NOT NULL, message_type TEXT NOT NULL, is_internal BOOLEAN NOT NULL, created_at DATETIME)`,
		`CREATE TABLE external_mcp_servers (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, enabled BOOLEAN NOT NULL, status TEXT NOT NULL)`,
		`CREATE TABLE external_mcp_tools (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, server_id TEXT NOT NULL, runtime_alias TEXT NOT NULL, access TEXT NOT NULL, enabled BOOLEAN NOT NULL)`,
		`CREATE TABLE agent_run_external_mcp_bindings (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_run_id TEXT NOT NULL, runtime_run_id TEXT NOT NULL, server_id TEXT NOT NULL, server_name TEXT NOT NULL, tool_aliases TEXT NOT NULL, created_at DATETIME)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO support_conversations (id, workspace_id, customer_email) VALUES ('conv-1', 'ws-1', 'alex@example.com')`,
		`INSERT INTO external_mcp_servers (id, workspace_id, enabled, status) VALUES ('server-1', 'ws-1', true, 'connected')`,
		`INSERT INTO external_mcp_tools (id, workspace_id, server_id, runtime_alias, access, enabled) VALUES ('tool-1', 'ws-1', 'server-1', 'mcp__logs__find_events', 'read', true)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO agent_runs (id, workspace_id, agent_id, target_type, target_id, status, external_runtime, external_runtime_id, input, output_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "run-1", "ws-1", "agent-1", "support_conversation", "conv-1", "running", "agent-runtime", "rt-run-1", []byte(`{}`), []byte(`{}`)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO agent_run_external_mcp_bindings (id, workspace_id, agent_run_id, runtime_run_id, server_id, server_name, tool_aliases) VALUES (?, ?, ?, ?, ?, ?, ?)`, "binding-1", "ws-1", "run-1", "rt-run-1", "server-1", "logs", []byte(`["mcp__logs__find_events"]`)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, is_internal, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "customer-1", "ws-1", "conv-1", "customer", "reply", false, time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	client := &fakeAgentRuntimeSignalClient{toolCalls: map[string][]AgentRuntimeToolCall{"rt-run-1": {{ID: "call-1", RunID: "rt-run-1", ToolName: "mcp__logs__find_events", Input: json.RawMessage(`{"email":"alex@example.com"}`), Output: json.RawMessage(`{"events":[{"message":"Import failed because CSV headers are duplicated"}]}`), CreatedAt: time.Now()}}}}
	agentSvc := &AgentService{agentRuntimeClient: client, externalMCPService: &ExternalMCPService{repo: repository.NewExternalMCPRepository(db)}}
	svc := NewInternalCommandService(agentSvc, nil, nil, nil, nil, nil, nil, nil)
	svc.agentRunRepo = repository.NewAgentRunRepository(db)
	svc.supportRunEvidenceRepo = repository.NewSupportRunEvidenceRepository(db)
	svc.supportAIService = &SupportAIService{conversationRepo: repository.NewSupportConversationRepository(db)}
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", RunID: "rt-run-1", TargetType: "support_conversation", TargetID: "conv-1"}
	output, err := svc.Execute(context.Background(), meta, "support.register_mcp_evidence", json.RawMessage(`{"tool_name":"mcp__logs__find_events"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		EvidenceID string `json:"evidence_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.EvidenceID, "mcp-result:") {
		t.Fatalf("missing server-issued evidence ID: %s", output)
	}
	rows, err := svc.supportRunEvidenceRepo.ListByRun(context.Background(), "ws-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].IsInternal || rows[0].SourceType != "external_mcp" {
		t.Fatalf("private evidence missing: %+v", rows)
	}
	evidence := supportEvidenceFromRows(rows)
	contract := &AIResponseContract{Content: "Your import failed because the CSV headers are duplicated.", CanAnswer: true, Confidence: 0.95, SourceDocIDs: []string{result.EvidenceID}, Claims: []AIResponseClaim{{Text: "The CSV headers are duplicated.", EvidenceIDs: []string{result.EvidenceID}}}}
	if gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: "answer", Contract: contract, Evidence: evidence, Threshold: 0.7}); !gate.OK {
		t.Fatalf("private evidence did not validate: %+v", gate)
	}
	if sources := buildAISources([]string{rows[0].ReferenceID}, evidence); len(sources) != 0 {
		t.Fatalf("private MCP evidence became visible: %+v", sources)
	}
	if err := db.Exec(`UPDATE external_mcp_tools SET access = 'write' WHERE id = 'tool-1'`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Execute(context.Background(), meta, "support.register_mcp_evidence", json.RawMessage(`{"tool_name":"mcp__logs__find_events"}`)); err == nil {
		t.Fatal("write-enabled MCP tool was accepted as private read evidence")
	}
	if err := db.Exec(`UPDATE external_mcp_tools SET access = 'read' WHERE id = 'tool-1'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, is_internal, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "customer-2", "ws-1", "conv-1", "customer", "reply", false, time.Now().Add(time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Execute(context.Background(), meta, "support.register_mcp_evidence", json.RawMessage(`{"tool_name":"mcp__logs__find_events"}`)); err == nil {
		t.Fatal("stale MCP result from a previous customer turn was accepted")
	}
}

func TestMCPRegistrationRejectsUnmatchedOrFailedCalls(t *testing.T) {
	email := "alex@example.com"
	conv := &model.SupportConversation{CustomerEmail: &email}
	base := AgentRuntimeToolCall{ID: "call", RunID: "rt-run", ToolName: "mcp__logs__find_events", Input: json.RawMessage(`{"email":"alex@example.com"}`), Output: json.RawMessage(`{"message":"Import failed"}`), CreatedAt: time.Now()}
	for name, mutate := range map[string]func(*AgentRuntimeToolCall){
		"wrong customer": func(call *AgentRuntimeToolCall) { call.Input = json.RawMessage(`{"email":"other@example.com"}`) },
		"customer only in unrelated field": func(call *AgentRuntimeToolCall) {
			call.Input = json.RawMessage(`{"email":"other@example.com","note":"alex@example.com"}`)
		},
		"write call":   func(call *AgentRuntimeToolCall) { call.Mutating = true },
		"failed call":  func(call *AgentRuntimeToolCall) { call.Error = "failed" },
		"error result": func(call *AgentRuntimeToolCall) { call.Output = json.RawMessage(`{"isError":true}`) },
		"empty result": func(call *AgentRuntimeToolCall) { call.Output = json.RawMessage(`null`) },
		"wrong run":    func(call *AgentRuntimeToolCall) { call.RunID = "other-run" },
		"oversized result": func(call *AgentRuntimeToolCall) {
			call.Output = json.RawMessage(`{"data":"` + strings.Repeat("x", supportMCPResultMaxBytes) + `"}`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			call := base
			mutate(&call)
			if got := latestSupportMCPReadCall([]AgentRuntimeToolCall{call}, "rt-run", base.ToolName, time.Time{}, conv); got != nil {
				t.Fatalf("unsafe call accepted: %+v", got)
			}
		})
	}
}

func TestMCPRegistrationDoesNotFallBackAfterLatestCallFails(t *testing.T) {
	email := "alex@example.com"
	conv := &model.SupportConversation{CustomerEmail: &email}
	first := AgentRuntimeToolCall{ID: "first", RunID: "rt-run", ToolName: "mcp__logs__find_events", Input: json.RawMessage(`{"email":"alex@example.com"}`), Output: json.RawMessage(`{"message":"Earlier result"}`), CreatedAt: time.Now()}
	failed := first
	failed.ID, failed.Error, failed.CreatedAt = "second", "lookup failed", first.CreatedAt.Add(time.Second)
	if got := latestSupportMCPReadCall([]AgentRuntimeToolCall{first, failed}, "rt-run", first.ToolName, time.Time{}, conv); got != nil {
		t.Fatalf("older result reused after latest lookup failed: %+v", got)
	}
	failed.Error = ""
	failed.Input = json.RawMessage(`{"email":"other@example.com"}`)
	if got := latestSupportMCPReadCall([]AgentRuntimeToolCall{first, failed}, "rt-run", first.ToolName, time.Time{}, conv); got != nil {
		t.Fatalf("older result reused after another customer's lookup: %+v", got)
	}
}

func TestOldMCPResultCannotSupportLaterCustomerTurn(t *testing.T) {
	cutoff := time.Now()
	rows := []model.SupportRunEvidence{
		{EvidenceID: "old-mcp", SourceType: "external_mcp", CreatedAt: cutoff.Add(-time.Minute)},
		{EvidenceID: "current-mcp", SourceType: "external_mcp", CreatedAt: cutoff.Add(time.Second)},
		{EvidenceID: "public-doc", SourceType: "content", CreatedAt: cutoff.Add(-time.Minute)},
	}
	filtered := supportEvidenceRowsForTurn(rows, cutoff)
	if len(filtered) != 2 || filtered[0].EvidenceID != "current-mcp" || filtered[1].EvidenceID != "public-doc" {
		t.Fatalf("wrong evidence retained for current turn: %+v", filtered)
	}
}
