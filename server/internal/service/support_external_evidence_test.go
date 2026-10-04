package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportWebEvidenceComesFromFetchedPages(t *testing.T) {
	chat, commands, db, _, _, _ := setupSupportProgressTest(t)
	chat.evidenceRepo = commands.supportRunEvidenceRepo
	chat.runRepo = commands.agentRunRepo
	chat.agentService = &AgentService{}
	// The plan finalizer can deliver before the child status projection lands.
	mustExec(t, db, `INSERT INTO agent_runs(id,workspace_id,agent_id,target_type,target_id,status,external_runtime,external_runtime_id) VALUES ('child','ws','agent','workspace','ws','running','agent-runtime','runtime-child')`)
	mustExec(t, db, `UPDATE agent_runs SET input=?, output_summary=? WHERE id='child'`, []byte(`{}`), []byte(`{}`))
	chat.agentService.agentRuntimeClient = &fakeAgentRuntimeSignalClient{toolCalls: map[string][]AgentRuntimeToolCall{
		"runtime-child": {
			{ID: "search", RunID: "runtime-child", ToolName: "web_search", Output: json.RawMessage(`{"results":[{"url":"https://provider.example/old","snippet":"Unsupported claim"}]}`)},
			{ID: "page-one", RunID: "runtime-child", ToolName: "fetch_url", Input: json.RawMessage(`{"url":"https://provider.example/token"}`), Output: json.RawMessage(`{"url":"https://provider.example/token","final_url":"https://docs.provider.example/token","status":200,"title":"Token expiry","text":"Provider token expired means request a new token."}`)},
			{ID: "page-two", RunID: "runtime-child", ToolName: "fetch_url", Input: json.RawMessage(`{"url":"https://provider.example/token-lifetime"}`), Output: json.RawMessage(`{"url":"https://provider.example/token-lifetime","status":200,"title":"Token lifetime","text":"Provider tokens expire after 1 hour."}`)},
			{ID: "missing", RunID: "runtime-child", ToolName: "fetch_url", Input: json.RawMessage(`{"url":"https://provider.example/missing"}`), Output: json.RawMessage(`{"url":"https://provider.example/missing","status":404,"text":"Not found"}`)},
		},
	}}
	// Agent Runtime stores the SDK tool-result envelope in its audit log.
	client := chat.agentService.agentRuntimeClient.(*fakeAgentRuntimeSignalClient)
	for i := range client.toolCalls["runtime-child"] {
		call := &client.toolCalls["runtime-child"][i]
		wrapped, err := json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": string(call.Output)}}})
		if err != nil {
			t.Fatal(err)
		}
		call.Output = wrapped
	}
	steps, err := json.Marshal([]model.CommandBarPlanStep{{AllowedTools: []string{"web_search", "fetch_url"}}})
	if err != nil {
		t.Fatal(err)
	}
	// A one-shot launch stores its handoff instructions in the plan prompt.
	plan := &model.CommandBarPlanRecord{ID: "plan", WorkspaceID: "ws", Status: model.CommandBarPlanStatusCompleted, Prompt: withSupportChildHandoffInstruction("Provider token expired"), Steps: steps, RunIDsByStep: json.RawMessage(`{"0":"child"}`)}
	block := dockChildResultOpenTag + `{"plan_id":"plan","runs":[{"result_available":true,"summary":"Invented claim https://provider.example/wrong-source"}]}` + dockChildResultCloseTag
	encoded, err := json.Marshal(chat.prepareSupportChildEvidence(context.Background(), plan, block))
	if err != nil {
		t.Fatal(err)
	}
	var rows []model.SupportRunEvidence
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("evidence = %s, want the two successful fetched pages", encoded)
	}
	if rows[0].URL != "https://docs.provider.example/token" || rows[1].URL != "https://provider.example/token-lifetime" {
		t.Fatalf("wrong citation URLs: %+v", rows)
	}
	if rows[0].EvidenceID == rows[1].EvidenceID || rows[0].Title != "Token expiry" || !strings.Contains(rows[0].Content, "request a new token") {
		t.Fatalf("page provenance lost: %+v", rows)
	}
	for _, row := range rows {
		if row.SourceType != "support_external_web" || row.IsInternal || strings.Contains(row.Content, "Invented claim") || row.VectorScore != 0 {
			t.Fatalf("invalid page evidence: %+v", row)
		}
	}
	if !chat.persistSupportChildEvidence(context.Background(), "run", rows) {
		t.Fatal("could not persist page evidence for the parent")
	}
	saved, err := chat.evidenceRepo.ListByRun(context.Background(), "ws", "run")
	if err != nil || len(saved) != 2 {
		t.Fatalf("parent evidence = %+v, err = %v", saved, err)
	}
	handoff, err := addSupportEvidenceToChildRunResultBlock(block, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(handoff, rows[0].EvidenceID) || !strings.Contains(handoff, rows[0].Content) || !strings.Contains(handoff, rows[1].URL) {
		t.Fatalf("actual page evidence missing from parent handoff: %s", handoff)
	}
	contract := &AIResponseContract{Content: rows[0].Content, CanAnswer: true, Confidence: .95,
		SourceDocIDs: []string{rows[0].EvidenceID}, Claims: []AIResponseClaim{{Text: rows[0].Content, EvidenceIDs: []string{rows[0].EvidenceID}}}}
	results := supportEvidenceFromRows(saved)
	if gate := evaluateSupportReplyGate(supportReplyGateInput{Kind: "answer", Contract: contract, Evidence: results, Threshold: .7}); !gate.OK {
		t.Fatalf("grounded external answer was rejected: %+v", gate)
	}
	sources := buildAISources(contract.SourceDocIDs, results)
	if len(sources) != 1 || sources[0].URL != rows[0].URL {
		t.Fatalf("customer citation = %+v, want the cited page's final URL", sources)
	}
}

func TestSupportWebResultWithoutFetchedPageHasNoEvidence(t *testing.T) {
	chat, commands, db, _, _, _ := setupSupportProgressTest(t)
	chat.evidenceRepo = commands.supportRunEvidenceRepo
	chat.runRepo = commands.agentRunRepo
	chat.agentService = &AgentService{}
	mustExec(t, db, `INSERT INTO agent_runs(id,workspace_id,agent_id,target_type,target_id,status,external_runtime,external_runtime_id) VALUES ('child','ws','agent','workspace','ws','completed','agent-runtime','runtime-child')`)
	mustExec(t, db, `UPDATE agent_runs SET input=?, output_summary=? WHERE id='child'`, []byte(`{}`), []byte(`{}`))
	chat.agentService.agentRuntimeClient = &fakeAgentRuntimeSignalClient{toolCalls: map[string][]AgentRuntimeToolCall{
		"runtime-child": {{ID: "search", RunID: "runtime-child", ToolName: "web_search", Output: json.RawMessage(`{"results":[{"url":"https://provider.example","snippet":"An unverified fact"}]}`)}},
	}}
	plan := &model.CommandBarPlanRecord{ID: "plan", WorkspaceID: "ws", Status: model.CommandBarPlanStatusCompleted, Steps: json.RawMessage(`[{"allowed_tools":["web_search"]}]`), RunIDsByStep: json.RawMessage(`{"0":"child"}`)}
	block := dockChildResultOpenTag + `{"runs":[{"result_available":true,"summary":"An unverified fact https://provider.example"}]}` + dockChildResultCloseTag
	if evidence := chat.prepareSupportChildEvidence(context.Background(), plan, block); evidence != nil {
		t.Fatalf("search snippet became evidence: %+v", evidence)
	}
}

func TestSupportFetchedPageEvidenceRejectsInvalidSources(t *testing.T) {
	good := AgentRuntimeToolCall{ID: "call", RunID: "runtime", ToolName: "fetch_url",
		Input:  json.RawMessage(`{"url":"https://provider.example/token"}`),
		Output: json.RawMessage(`{"url":"https://provider.example/token","status":200,"text":"Provider token expired."}`)}
	plan := &model.CommandBarPlanRecord{WorkspaceID: "ws", Prompt: "Provider token expired"}
	if _, ok := supportFetchedPageEvidence(plan, "child", "runtime", good); !ok {
		t.Fatal("valid direct fetch output was rejected")
	}
	for _, tc := range []struct {
		name   string
		change func(*AgentRuntimeToolCall)
	}{
		{"different run", func(c *AgentRuntimeToolCall) { c.RunID = "other" }},
		{"search snippet", func(c *AgentRuntimeToolCall) { c.ToolName = "web_search" }},
		{"failed call", func(c *AgentRuntimeToolCall) { c.Error = "network error" }},
		{"wrong page", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"url":"https://provider.example/other","status":200,"text":"Provider token expired."}`)
		}},
		{"404", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"url":"https://provider.example/token","status":404,"text":"Not found"}`)
		}},
		{"empty text", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"url":"https://provider.example/token","status":200,"text":""}`)
		}},
		{"credentialed redirect", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"url":"https://provider.example/token","final_url":"https://user:password@provider.example/token","status":200,"text":"Provider token expired."}`)
		}},
		{"tool error envelope", func(c *AgentRuntimeToolCall) {
			c.Output = json.RawMessage(`{"is_error":true,"structured_content":{"url":"https://provider.example/token","status":200,"text":"Provider token expired."}}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := good
			tc.change(&call)
			if _, ok := supportFetchedPageEvidence(plan, "child", "runtime", call); ok {
				t.Fatal("invalid source became support evidence")
			}
		})
	}
}
