package service

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportMCPReplyGateUsesReviewedConfidence(t *testing.T) {
	evidence := []KnowledgeSearchResult{{ID: "mcp__logs__lookup", SourceType: "external_mcp", IsInternal: true, Content: "Import failed because of duplicate headers"}}
	contract := &AIResponseContract{Content: "Your import failed because of duplicate headers.", CanAnswer: true, Confidence: .95, Claims: []AIResponseClaim{{Text: "Your import failed because of duplicate headers.", EvidenceIDs: []string{"mcp__logs__lookup"}}}}
	for _, tc := range []struct {
		name       string
		assessment supportMCPReplyAssessment
		want       bool
	}{
		{"supported", supportMCPReplyAssessment{CustomerScoped: true, Supported: true, Safe: true, Confidence: .9}, true},
		{"wrong customer", supportMCPReplyAssessment{Supported: true, Safe: true, Confidence: .99}, false},
		{"unsupported", supportMCPReplyAssessment{CustomerScoped: true, Safe: true, Confidence: .99}, false},
		{"secret disclosure", supportMCPReplyAssessment{CustomerScoped: true, Supported: true, Confidence: .99}, false},
		{"uncertain", supportMCPReplyAssessment{CustomerScoped: true, Supported: true, Safe: true, Confidence: .4}, false},
		{"invalid confidence", supportMCPReplyAssessment{CustomerScoped: true, Supported: true, Safe: true, Confidence: math.NaN()}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: "answer", Contract: contract, Evidence: evidence, Threshold: .8, MCPAssessment: &tc.assessment})
			if gate.OK != tc.want {
				t.Fatalf("gate = %+v", gate)
			}
			if tc.want && gate.Confidence != .9 {
				t.Fatalf("confidence = %v, want reviewed .9", gate.Confidence)
			}
		})
	}
}

func TestSupportMCPReadUsesLatestCurrentTurnResult(t *testing.T) {
	now := time.Now().UTC()
	good := AgentRuntimeToolCall{ID: "call", RunID: "runtime", ToolName: "mcp__logs__lookup", CreatedAt: now, Input: json.RawMessage(`{"email":"customer@example.com"}`), Output: json.RawMessage(`{"email":"customer@example.com","error":"duplicate headers"}`)}
	for _, tc := range []struct {
		name   string
		change func(*AgentRuntimeToolCall)
	}{
		{"old turn", func(c *AgentRuntimeToolCall) { c.CreatedAt = now.Add(-time.Minute) }},
		{"other run", func(c *AgentRuntimeToolCall) { c.RunID = "other" }},
		{"write", func(c *AgentRuntimeToolCall) { c.Mutating = true }},
		{"failed", func(c *AgentRuntimeToolCall) { c.Error = "failed" }},
		{"MCP error", func(c *AgentRuntimeToolCall) { c.Output = json.RawMessage(`{"isError":true}`) }},
		{"empty", func(c *AgentRuntimeToolCall) { c.Output = json.RawMessage(`null`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := good
			tc.change(&bad)
			if _, err := supportMCPReadEvidence([]AgentRuntimeToolCall{bad}, "runtime", good.ToolName, now); err == nil {
				t.Fatal("invalid result accepted")
			}
		})
	}
	evidence, err := supportMCPReadEvidence([]AgentRuntimeToolCall{good}, "runtime", good.ToolName, now)
	if err != nil || evidence.ID != good.ToolName || !evidence.IsInternal || evidence.VectorScore != 0 {
		t.Fatalf("evidence = %+v, err = %v", evidence, err)
	}
	failed := good
	failed.CreatedAt = now.Add(time.Second)
	failed.Error = "failed"
	if _, err := supportMCPReadEvidence([]AgentRuntimeToolCall{good, failed}, "runtime", good.ToolName, now); err == nil {
		t.Fatal("used older successful result after latest lookup failed")
	}
}

func TestSupportMCPCustomerRequiresVerifiedWidgetEmail(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws", Channel: "widget", CustomerEmail: strPtr("customer@example.com")}
	session := &model.SupportWidgetSession{WorkspaceID: "ws", ConversationID: strPtr("conv"), CustomerEmail: conv.CustomerEmail, IdentityTrust: model.IdentityTrustVerified, IdentityMethod: model.IdentityMethodSignedWidget}
	if _, err := supportMCPVerifiedCustomerEmail(conv, session); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*model.SupportWidgetSession)
	}{
		{"unverified", func(s *model.SupportWidgetSession) { s.IdentityTrust = model.IdentityTrustUntrusted }},
		{"different customer", func(s *model.SupportWidgetSession) { s.CustomerEmail = strPtr("other@example.com") }},
		{"different workspace", func(s *model.SupportWidgetSession) { s.WorkspaceID = "other" }},
		{"revoked", func(s *model.SupportWidgetSession) { now := time.Now(); s.RevokedAt = &now }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := *session
			tc.change(&changed)
			if _, err := supportMCPVerifiedCustomerEmail(conv, &changed); err == nil {
				t.Fatal("unverified identity accepted")
			}
		})
	}
}

func TestSupportMCPReviewReceivesActualResultAndFullReply(t *testing.T) {
	provider := &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"customer_scoped":true,"supported":true,"safe":true,"confidence":0.9}`}}
	svc := &SupportAIService{llmProvider: provider}
	contract := &AIResponseContract{Content: "Your import failed.", Confidence: .95, Claims: []AIResponseClaim{{Text: "Your import failed.", EvidenceIDs: []string{"mcp__logs__lookup"}}}}
	evidence := []KnowledgeSearchResult{{ID: "mcp__logs__lookup", SourceType: "external_mcp", Content: `{"email":"customer@example.com","status":"failed"}`}}
	assessment, err := svc.reviewSupportMCPReply(context.Background(), "ws", "conv", "customer@example.com", "Why did my import fail?", contract, evidence)
	if err != nil || assessment == nil || !assessment.Supported {
		t.Fatalf("assessment = %+v, err = %v", assessment, err)
	}
	data, _ := json.Marshal(provider.lastReq.Messages)
	for _, want := range []string{"customer@example.com", "Your import failed.", "Why did my import fail?", "failed"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("review missing %q", want)
		}
	}
	provider.response.Content = `{}`
	assessment, err = svc.reviewSupportMCPReply(context.Background(), "ws", "conv", "customer@example.com", "Why?", contract, evidence)
	if err == nil && assessment != nil && assessment.Supported {
		t.Fatal("empty review accepted")
	}
}

func TestSupportMCPReplyPublishesFromAuditedEvidence(t *testing.T) {
	for _, verified := range []bool{true, false} {
		t.Run(map[bool]string{true: "verified", false: "unverified"}[verified], func(t *testing.T) {
			_, commands, db, _, _, meta := setupSupportProgressTest(t)
			mustExec(t, db, `UPDATE support_conversations SET customer_email='customer@example.com' WHERE id='conv'`)
			mustExec(t, db, `UPDATE support_messages SET content='Why did my import fail?' WHERE id='source'`)
			trust := model.IdentityTrustUntrusted
			if verified {
				trust = model.IdentityTrustVerified
			}
			mustExec(t, db, `INSERT INTO support_widget_sessions(id,workspace_id,conversation_id,session_token,customer_email,identity_method,identity_trust,expires_at,created_at) VALUES ('session','ws','conv','token','customer@example.com','signed_widget',?,?,?)`, trust, time.Now().Add(time.Hour), time.Now())
			mustExec(t, db, `CREATE TABLE external_mcp_servers(id TEXT PRIMARY KEY, workspace_id TEXT, enabled BOOLEAN)`)
			mustExec(t, db, `CREATE TABLE external_mcp_tools(id TEXT PRIMARY KEY, workspace_id TEXT, server_id TEXT, runtime_alias TEXT, access TEXT, enabled BOOLEAN)`)
			mustExec(t, db, `INSERT INTO external_mcp_servers VALUES ('server','ws',true)`)
			mustExec(t, db, `INSERT INTO external_mcp_tools VALUES ('tool','ws','server','mcp__logs__lookup','read',true)`)
			runtime := &fakeAgentRuntimeSignalClient{toolCalls: map[string][]AgentRuntimeToolCall{"runtime-run": {{ID: "call", RunID: "runtime-run", ToolName: "mcp__logs__lookup", CreatedAt: time.Now(), Input: json.RawMessage(`{"email":"customer@example.com"}`), Output: json.RawMessage(`{"email":"customer@example.com","status":"failed","cause":"duplicate headers","token":"private-secret"}`)}}}}
			commands.agentService = &AgentService{agentRuntimeClient: runtime, externalMCPService: &ExternalMCPService{repo: repository.NewExternalMCPRepository(db)}}
			provider := &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"customer_scoped":true,"supported":true,"safe":true,"confidence":0.96}`}}
			commands.supportAIService.llmProvider = provider
			input := json.RawMessage(`{"content":"Your import failed because of duplicate headers.","reply_kind":"answer","confidence":0.95,"source_doc_ids":["mcp__logs__lookup"],"claims":[{"text":"Your import failed because of duplicate headers.","evidence_ids":["mcp__logs__lookup"]}]}`)
			output, err := commands.executeSupportSendReply(context.Background(), meta, input)
			if err != nil {
				t.Fatal(err)
			}
			var outcome struct {
				Status     string  `json:"status"`
				MessageID  string  `json:"message_id"`
				Confidence float64 `json:"confidence"`
			}
			if err := json.Unmarshal(output, &outcome); err != nil {
				t.Fatal(err)
			}
			if !verified {
				if outcome.Status != "evidence_unavailable" || len(provider.lastReq.Messages) != 0 {
					t.Fatalf("unverified identity reached review/delivery: %s", output)
				}
				var count int64
				if err := db.Model(&model.SupportMessage{}).Where("sender_type = 'ai'").Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("unverified account data was published")
				}
				return
			}
			if outcome.Status != "sent" || outcome.Confidence != .95 {
				t.Fatalf("outcome = %s", output)
			}
			var message model.SupportMessage
			if err := db.First(&message, "id = ?", outcome.MessageID).Error; err != nil {
				t.Fatal(err)
			}
			if strings.Contains(message.Content+message.Metadata, "private-secret") || strings.Contains(message.Metadata, "mcp__logs__lookup") {
				t.Fatalf("private data leaked: %+v", message)
			}
			var count int64
			if err := db.Model(&model.SupportRunEvidence{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("raw MCP result was persisted as knowledge")
			}
		})
	}
}
