package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportReplyBlocksActionOmittedFromClaimMap(t *testing.T) {
	for _, kind := range []string{"answer", "conversational", "clarify", "confirmation"} {
		t.Run(kind, func(t *testing.T) {
			_, commands, db, _, _, meta := setupSupportProgressTest(t)
			provider := &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"allowed":false,"unsupported_claims":["I have passed your suggestions on."]}`}}
			commands.supportAIService.llmProvider = provider
			evidence := model.SupportRunEvidence{WorkspaceID: "ws", RunID: "run", EvidenceID: "upload-guide", SourceType: "docs", Content: "Choose Manual Upload to review the settings before publishing.", VectorScore: .95}
			if err := commands.supportRunEvidenceRepo.UpsertBatch(context.Background(), []model.SupportRunEvidence{evidence}); err != nil {
				t.Fatal(err)
			}
			input, err := json.Marshal(map[string]any{
				"content":    "I have passed your suggestions on. Choose Manual Upload to review the settings before publishing.",
				"reply_kind": kind, "confidence": .95, "source_doc_ids": []string{"upload-guide"},
				"claims": []AIResponseClaim{{Text: "Choose Manual Upload to review the settings before publishing.", EvidenceIDs: []string{"upload-guide"}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			output, err := commands.executeSupportSendReply(context.Background(), meta, input)
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ Status, Reason string }
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "rewrite_required" || result.Reason != "unsupported_action" {
				t.Fatalf("unsupported action accepted: %s", output)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("sender_type = 'ai'").Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("rejected reply was published: count=%d err=%v", count, err)
			}
			if len(provider.lastReq.Messages) != 1 || !strings.Contains(provider.lastReq.Messages[0].Content, "I have passed your suggestions on.") {
				t.Fatal("review did not receive the full outgoing reply")
			}
		})
	}
}

func TestSupportHandoffKeepsGroundedPublicWorkaround(t *testing.T) {
	_, commands, _, conv, _, meta := setupSupportProgressTest(t)
	commands.supportAIService.llmProvider = &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"customer_scoped":true,"supported":true,"safe":true,"confidence":0.95,"allowed":true,"unsupported_claims":[]}`}}
	content := "Choose Manual Upload to review the settings before publishing."
	evidence := model.SupportRunEvidence{WorkspaceID: "ws", RunID: "run", EvidenceID: "upload-guide", SourceType: "docs", Content: content, VectorScore: .95}
	if err := commands.supportRunEvidenceRepo.UpsertBatch(context.Background(), []model.SupportRunEvidence{evidence}); err != nil {
		t.Fatal(err)
	}
	got := commands.checkedSupportHandoffUpdate(context.Background(), meta, conv, "source", content, []AIResponseClaim{{Text: content, EvidenceIDs: []string{"upload-guide"}}})
	if got != content {
		t.Fatalf("grounded public workaround lost during handoff: %q", got)
	}
}

func TestSupportActionReviewRequiresExplicitVerdict(t *testing.T) {
	for _, response := range []string{`{}`, `null`, `{"allowed":true,"unsupported_claims":["bad"]}`, `{"allowed":false,"unsupported_claims":[]}`, `not json`} {
		t.Run(response, func(t *testing.T) {
			provider := &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: response}}
			ai := &SupportAIService{llmProvider: provider}
			if _, err := ai.reviewSupportReplyActions(context.Background(), "ws", "conv", "Thanks for the suggestion.", nil); err == nil {
				t.Fatal("invalid assessment accepted")
			}
		})
	}
}

func TestSupportActionReceiptsRequireSuccessfulScopedCurrentWrites(t *testing.T) {
	now := time.Now()
	good := AgentRuntimeToolCall{ID: "note", RunID: "run", ToolName: "add_support_conversation_note", Status: "completed", Mutating: true, CreatedAt: now, Input: json.RawMessage(`{"content":"Please consider saved upload settings."}`), Output: json.RawMessage(`{"status":"created","conversation_id":"conv","message_id":"note","is_internal":true}`)}
	for _, tc := range []struct {
		name   string
		change func(*AgentRuntimeToolCall)
	}{
		{"wrong run", func(c *AgentRuntimeToolCall) { c.RunID = "other" }},
		{"previous turn", func(c *AgentRuntimeToolCall) { c.CreatedAt = now.Add(-time.Second) }},
		{"no timestamp", func(c *AgentRuntimeToolCall) { c.CreatedAt = time.Time{} }},
		{"read only", func(c *AgentRuntimeToolCall) { c.Mutating = false }},
		{"failed", func(c *AgentRuntimeToolCall) { c.Error = "failed" }},
		{"pending", func(c *AgentRuntimeToolCall) { c.Status = "pending" }},
		{"suppressed", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"conversation_id":"conv","status":"suppressed"}`)
		}},
		{"wrong conversation", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"status":"created","conversation_id":"other","message_id":"note","is_internal":true}`)
		}},
		{"missing receipt", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"status":"created","conversation_id":"conv","is_internal":true}`)
		}},
		{"tool error", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"isError":true,"status":"created","conversation_id":"conv","message_id":"note","is_internal":true}`)
		}},
		{"search", func(c *AgentRuntimeToolCall) { c.ToolName = "search_knowledge" }},
		{"external write", func(c *AgentRuntimeToolCall) { c.ToolName = "mcp__other__write" }},
		{"oversized", func(c *AgentRuntimeToolCall) {
			c.Input = json.RawMessage(`{"content":"` + strings.Repeat("a", 9000) + `"}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := good
			tc.change(&bad)
			if got := supportActionReceipts([]AgentRuntimeToolCall{bad}, "run", "conv", now); len(got) != 0 {
				t.Fatalf("invalid action accepted: %+v", got)
			}
		})
	}
	got := supportActionReceipts([]AgentRuntimeToolCall{good}, "run", "conv", now)
	if len(got) != 1 || got[0].Tool != "add_support_conversation_note" {
		t.Fatalf("missing successful note: %+v", got)
	}
	linked := good
	linked.ToolName = "link_support_conversation_task"
	linked.Output = json.RawMessage(`{"conversation_id":"conv","task_id":"task"}`)
	cleared := linked
	cleared.CreatedAt = now.Add(time.Second)
	cleared.Output = json.RawMessage(`{"conversation_id":"conv","task_id":null}`)
	got = supportActionReceipts([]AgentRuntimeToolCall{linked, cleared}, "run", "conv", now)
	if len(got) != 2 || !got[1].PerformedAt.After(got[0].PerformedAt) || !strings.Contains(string(got[1].Result), `"task_id":null`) {
		t.Fatalf("successful undo was omitted: %+v", got)
	}
}

func TestSupportActionCorrectionPreservesConversationalReply(t *testing.T) {
	_, commands, db, _, _, meta := setupSupportProgressTest(t)
	mustExec(t, db, `UPDATE command_bar_plans SET status='completed',parent_notified_at=CURRENT_TIMESTAMP`)
	provider := &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"allowed":false,"unsupported_claims":["The team has been notified."]}`}}
	commands.supportAIService.llmProvider = provider
	input := json.RawMessage(`{"content":"The team has been notified.","reply_kind":"conversational","confidence":1}`)
	out, err := commands.executeSupportSendReply(context.Background(), meta, input)
	if err != nil || !strings.Contains(string(out), `"status":"rewrite_required"`) {
		t.Fatalf("first correction = %s, %v", out, err)
	}
	var turn model.AIMessageProcessing
	if err := db.First(&turn, "source_message_id='source'").Error; err != nil {
		t.Fatal(err)
	}
	if !turn.ActionValidationRepairUsed || turn.ValidationRepairUsed || turn.Status != "processing" {
		t.Fatalf("correction changed factual gate or settled the turn: %+v", turn)
	}
	provider.response.Content = `{"allowed":true,"unsupported_claims":[]}`
	input = json.RawMessage(`{"content":"Thanks for the suggestion.","reply_kind":"conversational","confidence":1}`)
	out, err = commands.executeSupportSendReply(context.Background(), meta, input)
	if err != nil || !strings.Contains(string(out), `"status":"sent"`) {
		t.Fatalf("honest acknowledgment rejected: %s, %v", out, err)
	}
	out, err = commands.executeSupportSendReply(context.Background(), meta, input)
	if err != nil || !strings.Contains(string(out), `"status":"suppressed"`) {
		t.Fatalf("retry was not suppressed: %s, %v", out, err)
	}
}

func TestSupportRepeatedUnsupportedActionHandsOffOnce(t *testing.T) {
	commands, db, meta := setupSupportHandoffCheckTest(t)
	commands.supportAIService.llmProvider = &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"allowed":false,"unsupported_claims":["The team has been notified."]}`}}
	input := json.RawMessage(`{"content":"The team has been notified.","reply_kind":"conversational","confidence":1}`)
	for _, want := range []string{"rewrite_required", "escalated", "suppressed"} {
		out, err := commands.executeSupportSendReply(context.Background(), meta, input)
		if err != nil || !strings.Contains(string(out), `"status":"`+want+`"`) {
			t.Fatalf("outcome = %s, %v; want %s", out, err, want)
		}
	}
	conv := readControlConversation(t, db)
	if derefString(conv.AIState) != "escalated" || !model.SupportAIConversationBlocked(conv) {
		t.Fatal("no real handoff occurred")
	}
	card := savedHandoffCard(t, db)
	if card.HeldReply != "The team has been notified." {
		t.Fatal("rejected draft not preserved for teammate")
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai' AND is_internal=false").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("expected only one server handoff message: %d %v", count, err)
	}
}

func TestSupportActionReviewFailureDoesNotSendOrSettle(t *testing.T) {
	_, commands, db, _, _, meta := setupSupportProgressTest(t)
	commands.supportAIService.llmProvider = &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{}`}}
	out, err := commands.executeSupportSendReply(context.Background(), meta, json.RawMessage(`{"content":"Thanks for your feedback.","reply_kind":"conversational","confidence":1}`))
	if err != nil || !strings.Contains(string(out), `"status":"review_unavailable"`) {
		t.Fatalf("review failure = %s %v", out, err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("unreviewed reply sent: %d %v", count, err)
	}
	var turn model.AIMessageProcessing
	if err := db.First(&turn, "source_message_id='source'").Error; err != nil || turn.Status != "processing" || turn.ActionValidationRepairUsed {
		t.Fatalf("review failure settled/consumed correction: %+v %v", turn, err)
	}
}

func TestSupportHandoffDoesNotReportSuccessForStaleTurn(t *testing.T) {
	commands, db, meta := setupSupportHandoffCheckTest(t)
	mustExec(t, db, `UPDATE support_conversations SET ai_resumed_at=? WHERE id='conv'`, time.Now().Add(time.Second))
	out, err := commands.executeSupportEscalate(context.Background(), meta, json.RawMessage(`{"reason":"action_unavailable"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"status":"escalated"`) {
		t.Fatalf("no-op handoff reported success: %s", out)
	}
	var turn model.AIMessageProcessing
	if err := db.First(&turn, "source_message_id='source'").Error; err != nil || turn.Status != "processing" {
		t.Fatalf("no-op handoff settled the turn: %+v %v", turn, err)
	}
}

func TestSupportActionReviewPreservesHumanTakeover(t *testing.T) {
	_, commands, db, _, _, meta := setupSupportProgressTest(t)
	commands.supportAIService.llmProvider = &greetingProvider{during: func() {
		mustExec(t, db, `UPDATE support_conversations SET human_takeover=true,ai_control_version=2 WHERE id='conv'`)
	}}
	out, err := commands.executeSupportSendReply(context.Background(), meta, json.RawMessage(`{"content":"Thanks for the suggestion.","reply_kind":"conversational","confidence":1}`))
	if err != nil || !strings.Contains(string(out), `"status":"suppressed"`) {
		t.Fatalf("takeover during review = %s %v", out, err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("review bypassed ownership fence: %d %v", count, err)
	}
}

func TestSupportRejectedActionDoesNotHandoffNewerReply(t *testing.T) {
	commands, db, meta := setupSupportHandoffCheckTest(t)
	mustExec(t, db, `UPDATE ai_message_processing SET action_validation_repair_used=true WHERE source_message_id='source'`)
	commands.supportAIService.llmProvider = &greetingProvider{
		response: &llm.ChatResponse{Content: `{"allowed":false,"unsupported_claims":["I notified the team."]}`},
		during: func() {
			mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='newer-reply' WHERE id='conv'`)
			mustExec(t, db, `UPDATE ai_message_processing SET status='completed',reply_message_id='newer-reply' WHERE source_message_id='source'`)
		},
	}
	out, err := commands.executeSupportSendReply(context.Background(), meta, json.RawMessage(`{"content":"I notified the team.","reply_kind":"conversational","confidence":1}`))
	if err != nil || !strings.Contains(string(out), `"status":"suppressed"`) {
		t.Fatalf("stale rejection = %s %v", out, err)
	}
	if derefString(readControlConversation(t, db).AIState) == "escalated" {
		t.Fatal("stale rejected draft escalated a newer reply")
	}
}

func TestSupportPublicWorkaroundAndHandoffCommitTogether(t *testing.T) {
	commands, db, meta := setupSupportHandoffCheckTest(t)
	commands.supportAIService.llmProvider = &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"customer_scoped":true,"supported":true,"safe":true,"confidence":0.95,"allowed":true,"unsupported_claims":[]}`}}
	content := "Choose Manual Upload to review the settings before publishing."
	if err := commands.supportRunEvidenceRepo.UpsertBatch(context.Background(), []model.SupportRunEvidence{{WorkspaceID: "ws", RunID: "run", EvidenceID: "upload-guide", SourceType: "docs", Content: content, VectorScore: .95}}); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(map[string]any{"reason": "action_unavailable", "issue_summary": "Customer wants saved upload defaults.", "customer_update": content, "claims": []AIResponseClaim{{Text: content, EvidenceIDs: []string{"upload-guide"}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"escalated", "suppressed"} {
		out, err := commands.executeSupportEscalate(context.Background(), meta, input)
		if err != nil || !strings.Contains(string(out), `"status":"`+want+`"`) {
			t.Fatalf("handoff = %s %v; want %s", out, err, want)
		}
	}
	conv := readControlConversation(t, db)
	if derefString(conv.AIState) != "escalated" || conv.AIEscalatedAt == nil || conv.AIActiveRunID != nil {
		t.Fatal("answer was not paired with durable handoff")
	}
	card := savedHandoffCard(t, db)
	if card.Issue != "Customer wants saved upload defaults." {
		t.Fatalf("lost team brief: %+v", card)
	}
	messages, err := commands.supportAIService.messageRepo.ListByConversation(context.Background(), "ws", "conv", false)
	if err != nil || len(messages) != 2 || !strings.HasPrefix(messages[1].Content, content) || len(messages[1].Content) <= len(content) {
		t.Fatalf("wanted one workaround + handoff reply, got %+v %v", messages, err)
	}
}

func TestSupportActionReviewReceivesAuditedNoteReceipt(t *testing.T) {
	commands, _, meta := setupSupportHandoffCheckTest(t, AgentRuntimeToolCall{
		ID: "note-call", ToolName: "add_support_conversation_note", Mutating: true, Status: "completed",
		Input:  json.RawMessage(`{"content":"Customer suggested saved upload defaults."}`),
		Output: json.RawMessage(`{"status":"created","conversation_id":"conv","message_id":"saved-note","is_internal":true}`),
	})
	provider := &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"allowed":true,"unsupported_claims":[]}`}}
	commands.supportAIService.llmProvider = provider
	out, err := commands.executeSupportSendReply(context.Background(), meta, json.RawMessage(`{"content":"I recorded your suggestion in a note on this conversation.","reply_kind":"conversational","confidence":1}`))
	if err != nil || strings.Contains(string(out), "rewrite_required") || len(provider.lastReq.Messages) != 1 {
		t.Fatalf("verified note rejected: %s %v", out, err)
	}
	var payload struct {
		Actions []supportActionReceipt `json:"completed_actions"`
	}
	if err := json.Unmarshal([]byte(provider.lastReq.Messages[0].Content), &payload); err != nil || len(payload.Actions) != 1 || payload.Actions[0].Tool != "add_support_conversation_note" {
		t.Fatalf("review missing actual note receipt: %+v %v", payload, err)
	}
}

func TestSupportPreviewRejectsUnperformedAction(t *testing.T) {
	db, preview, commands, _, ctx := previewFixture(t)
	commands.supportAIService.llmProvider = &scriptedSupportRewriteLLM{response: llm.ChatResponse{Content: `{"allowed":false,"unsupported_claims":["I notified the team."]}`}}
	started, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "Please pass along my suggestion."})
	if err != nil {
		t.Fatal(err)
	}
	meta := model.InternalCommandContext{WorkspaceID: "workspace", AgentID: "agent-1", RunID: "run_runtime_1", TargetType: supportPreviewTarget, TargetID: started.RunID}
	_, err = commands.Execute(ctx, meta, "support.send_reply", json.RawMessage(`{"content":"I notified the team.","reply_kind":"conversational","confidence":1}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := preview.Get(ctx, "workspace", "agent-1", started.RunID)
	if err != nil || result.FinalDecision != "handoff" || result.FinalReason != "unsupported_action" {
		t.Fatalf("preview accepted false action: %+v %v", result, err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("preview contacted customer: %d %v", count, err)
	}
}
