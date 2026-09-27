package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func previewFixture(t *testing.T) (*gorm.DB, *SupportPreviewService, *InternalCommandService, *fakeAgentRuntimeSignalClient, context.Context) {
	t.Helper()
	db := setupAgentRuntimeSupportRunTestDB(t)
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeInteractive, time.Now())
	mustExec(t, db, `UPDATE agents SET workspace_id='workspace', allowed_tools=?, allowed_targets=? WHERE id='agent-1'`, []byte(`["search_knowledge","send_support_reply","escalate_to_human","list_conversation_messages","get_support_conversation","start_agent_run","mcp__external__write"]`), []byte(`["support_conversation"]`))
	mustExec(t, db, `CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT, settings TEXT, active BOOLEAN)`)
	mustExec(t, db, `INSERT INTO support_widget_installations VALUES ('installation','workspace','{"ai_agent_id":"agent-1","ai_confidence_threshold":0.7}',true)`)
	// Use the evidence table contract from the tool tests; SQLite is sufficient
	// for this orchestration test (no vector search or Postgres-specific locks).
	source := setupSupportKnowledgeTestDB(t)
	var schema string
	if err := source.Raw("SELECT sql FROM sqlite_master WHERE name='support_run_evidence'").Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, schema)
	runtime := &fakeAgentRuntimeSignalClient{}
	agents := newDelegatedSupportRunService(t, db, runtime)
	agents.artifactRepo = repository.NewAgentRunArtifactRepository(db)
	ai := &SupportAIService{agentRepo: agents.agentRepo, installationRepo: repository.NewSupportInboxInstallationRepository(db), conversationRepo: repository.NewSupportConversationRepository(db), messageRepo: repository.NewSupportMessageRepository(db)}
	evidence := repository.NewSupportRunEvidenceRepository(db)
	preview := NewSupportPreviewService(agents, ai, evidence)
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commands.SetAgentRunDependencies(agents.runRepo, agents.artifactRepo)
	commands.SetSupportKnowledgeDependencies(ai, evidence)
	commands.SetSupportReplyDependencies(ai, nil, nil)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{WorkspaceID: "workspace", UserID: "owner", Role: model.RoleOwner})
	return db, preview, commands, runtime, ctx
}

func TestSupportPreviewUsesResolvedProfileAndIsolatedRuntimeTarget(t *testing.T) {
	db, preview, _, runtime, ctx := previewFixture(t)
	profiles, primary, _ := setupAIProfileTest(t)
	profile, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Support", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	if err = profiles.SetDefault(ctx, "workspace", "owner", profile.ID); err != nil {
		t.Fatal(err)
	}
	preview.agents.SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
	result, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "How do I install?", History: []model.SupportAIPreviewHistoryTurn{{SenderType: "customer", Content: "Earlier question"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.startRunCalls) != 1 {
		t.Fatal("runtime was not called")
	}
	req := runtime.startRunCalls[0]
	if req.Model == nil || req.Model.Model != "custom-unpriced-model" || req.ModelCredential == nil || req.ModelCredential.APIKey != "Primary-key" {
		t.Fatal("preview bypassed profile credentials")
	}
	if req.Target.Type != supportPreviewTarget || req.Target.ID != result.RunID || len(req.MCPServers) != 0 || req.TurnPolicy.Mode != agentRuntimeTurnCompleteOnFinish {
		t.Fatalf("unsafe launch: %+v", req.Target)
	}
	for _, tool := range req.AllowedTools {
		if !slices.Contains(supportPreviewTools, tool) {
			t.Fatalf("unsafe tool %s", tool)
		}
	}
	run, err := preview.agents.runRepo.GetByID(ctx, "workspace", result.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(run.Input), "Primary-key") || run.ConversationID != nil || runInputTriggerType(run) != supportPreviewTarget {
		t.Fatal("unsafe persisted preview")
	}
	if result.ProfileID != profile.ID || result.Model != "custom-unpriced-model" {
		t.Fatalf("missing resolved route: %+v", result)
	}
	var count int64
	if err = db.Table("support_conversations").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("preview created a conversation: %d %v", count, err)
	}
	host := NewAgentRuntimeHostService("helpin", preview.agents.runRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	contextResult, err := host.ResolveTargetContext(ctx, sdk.TargetContextRequest{AppID: "helpin", AgentID: "agent-1", RunID: "run_runtime_1", Target: req.Target})
	if err != nil {
		t.Fatal(err)
	}
	if len(contextResult.Data["messages"].([]model.SupportMessage)) != 2 {
		t.Fatal("history missing")
	}
	_, err = host.ResolveTargetContext(ctx, sdk.TargetContextRequest{AppID: "helpin", AgentID: "agent-1", RunID: "run_runtime_1", Target: sdk.TargetRef{Type: "support_conversation", ID: "live"}})
	if err == nil {
		t.Fatal("preview accessed live target")
	}
}

func TestSupportPreviewCommandsCaptureOutcomeAndFailClosed(t *testing.T) {
	for _, test := range []struct{ name, input, decision string }{
		{"greeting", `{"content":"Hello! How can I help?","reply_kind":"conversational","confidence":1}`, "answer"},
		{"ungrounded", `{"content":"The plan costs $99.","reply_kind":"answer","confidence":1,"claims":[{"text":"The plan costs $99.","evidence_ids":["fake"]}]}`, "handoff"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, preview, commands, _, ctx := previewFixture(t)
			result, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "Hello"})
			if err != nil {
				t.Fatal(err)
			}
			meta := model.InternalCommandContext{WorkspaceID: "workspace", AgentID: "agent-1", RunID: "run_runtime_1", TargetType: supportPreviewTarget, TargetID: result.RunID}
			for _, name := range []string{"support.update_status", "support.assign_conversation", "agents.start_run"} {
				if _, err = commands.Execute(ctx, meta, name, json.RawMessage(`{}`)); err == nil {
					t.Fatalf("mutation permitted: %s", name)
				}
			}
			forged := meta
			forged.TargetType = "support_conversation"
			forged.TargetID = "live"
			if _, err = commands.Execute(ctx, forged, "support.send_reply", json.RawMessage(test.input)); err == nil {
				t.Fatal("forged target permitted")
			}
			if _, err = commands.Execute(ctx, meta, "support.list_conversation_messages", json.RawMessage(`{"conversation_id":"live"}`)); err == nil {
				t.Fatal("cross-conversation read permitted")
			}
			if _, err = commands.Execute(ctx, meta, "support.search_knowledge", json.RawMessage(`{"queries":["Hello"]}`)); err != nil {
				t.Fatal(err)
			}
			if _, err = commands.Execute(ctx, meta, "support.send_reply", json.RawMessage(test.input)); err != nil {
				t.Fatal(err)
			}
			// A repeated or competing callback must not overwrite the first outcome.
			if _, err = commands.Execute(ctx, meta, "support.escalate_to_human", json.RawMessage(`{"reason":"overwrite"}`)); err != nil {
				t.Fatal(err)
			}
			got, err := preview.Get(ctx, "workspace", "agent-1", result.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if got.FinalDecision != test.decision || got.FinalReason == "overwrite" {
				t.Fatalf("bad outcome %+v", got)
			}
			for _, table := range []string{"support_conversations", "support_messages"} {
				var count int64
				if err = db.Table(table).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("preview wrote %s: %d %v", table, count, err)
				}
			}
			wrongActor := authorization.WithActor(ctx, &authorization.Actor{WorkspaceID: "workspace", UserID: "other", Role: model.RoleOwner})
			if _, err = preview.Get(wrongActor, "workspace", "agent-1", result.RunID); err == nil {
				t.Fatal("another actor read preview")
			}
			mustExec(t, db, `UPDATE agent_runs SET status='completed' WHERE id=?`, result.RunID)
			if _, err = commands.Execute(ctx, meta, "support.send_reply", json.RawMessage(test.input)); err == nil {
				t.Fatal("late callback accepted")
			}
		})
	}
}

func TestSupportPreviewRetrievalOnlyAndHardHandoffAvoidModel(t *testing.T) {
	_, preview, _, runtime, ctx := previewFixture(t)
	no := false
	for _, req := range []model.SupportAIPreviewRequest{{Message: "pricing", IncludeAnswer: &no}, {Message: "I want to talk to a human"}} {
		result, err := preview.Start(ctx, "workspace", "agent-1", req)
		if err != nil {
			t.Fatal(err)
		}
		if result.FinalDecision == "pending" || len(runtime.startRunCalls) != 0 {
			t.Fatal("unnecessary model execution")
		}
	}
}

func TestSupportPreviewGroundedReplySurvivesEvidenceCleanup(t *testing.T) {
	db, preview, commands, _, ctx := previewFixture(t)
	started, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "How much is Pro?"})
	if err != nil {
		t.Fatal(err)
	}
	evidence := supportGateEvidence()[0]
	commands.persistSupportRunEvidence(ctx, model.InternalCommandContext{WorkspaceID: "workspace", RunID: "run_runtime_1"}, []KnowledgeSearchResult{evidence})
	contract := &AIResponseContract{Content: "The Pro plan costs $49 per month and includes 10 seats.", CanAnswer: true, Confidence: 1, SourceDocIDs: []string{evidence.ID}, Claims: []AIResponseClaim{{Text: "The Pro plan costs $49 per month and includes 10 seats.", EvidenceIDs: []string{evidence.ID}}}}
	expected := evaluateSupportReplyGate(supportReplyGateInput{Kind: "answer", Contract: contract, Evidence: []KnowledgeSearchResult{evidence}, Threshold: 0.7})
	if !expected.OK {
		t.Fatalf("fixture should pass live gate: %+v", expected)
	}
	input, _ := json.Marshal(map[string]any{"content": contract.Content, "reply_kind": "answer", "confidence": contract.Confidence, "source_doc_ids": contract.SourceDocIDs, "claims": contract.Claims})
	meta := model.InternalCommandContext{WorkspaceID: "workspace", AgentID: "agent-1", RunID: "run_runtime_1", TargetType: supportPreviewTarget, TargetID: started.RunID}
	if _, err = commands.Execute(ctx, meta, "support.send_reply", input); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `DELETE FROM support_run_evidence WHERE run_id=?`, started.RunID)
	mustExec(t, db, `UPDATE agent_runs SET status='completed' WHERE id=?`, started.RunID)
	got, err := preview.Get(ctx, "workspace", "agent-1", started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FinalDecision != "answer" || got.Answer == nil || got.Answer.GroundedConfidence != expected.Confidence || len(got.Retrieval.Results) != 1 {
		t.Fatalf("lost gate result/evidence: %+v", got)
	}
}

func TestSupportPreviewFailureExpiryAndCancellation(t *testing.T) {
	t.Run("runtime failure has no legacy fallback", func(t *testing.T) {
		db, preview, _, runtime, ctx := previewFixture(t)
		runtime.startRunErr = fmt.Errorf("provider unavailable")
		if _, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "Question"}); err == nil {
			t.Fatal("failure hidden")
		}
		var runs []model.AgentRun
		if err := db.Find(&runs).Error; err != nil {
			t.Fatal(err)
		}
		if len(runs) != 1 || runs[0].Status != "failed" || len(runtime.startRunCalls) < 1 || len(runtime.startRunCalls) > 2 {
			t.Fatalf("failed admission left active/fallback run: %+v calls=%d", runs, len(runtime.startRunCalls))
		}
	})
	t.Run("expiry works without browser polling", func(t *testing.T) {
		db, preview, commands, runtime, ctx := previewFixture(t)
		started, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "Question"})
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, db, `UPDATE agent_runs SET created_at=?,updated_at=? WHERE id=?`, time.Now().UTC().Add(-5*time.Minute), time.Now().UTC(), started.RunID)
		meta := model.InternalCommandContext{WorkspaceID: "workspace", AgentID: "agent-1", RunID: "run_runtime_1", TargetType: supportPreviewTarget, TargetID: started.RunID}
		if _, err = commands.Execute(ctx, meta, "support.escalate_to_human", json.RawMessage(`{"reason":"late"}`)); err == nil {
			t.Fatal("expired tool call accepted")
		}
		projection := NewAgentRuntimeProjectionService(preview.agents.runRepo).SetOverageDependencies(nil, nil, runtime)
		if err = projection.ReconcileMappedRuns(ctx, 2*time.Minute, 50); err != nil {
			t.Fatal(err)
		}
		if len(runtime.cancelCalls) != 1 {
			t.Fatal("abandoned but recently updated preview was not cancelled")
		}
	})
	t.Run("explicit stop checks ownership", func(t *testing.T) {
		_, preview, _, runtime, ctx := previewFixture(t)
		started, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "Question"})
		if err != nil {
			t.Fatal(err)
		}
		outsider := authorization.WithActor(ctx, &authorization.Actor{WorkspaceID: "workspace", UserID: "other", Role: model.RoleOwner})
		if err = preview.Cancel(outsider, "workspace", "agent-1", started.RunID); err == nil {
			t.Fatal("another user cancelled test")
		}
		if err = preview.Cancel(ctx, "workspace", "agent-1", started.RunID); err != nil {
			t.Fatal(err)
		}
		if len(runtime.cancelCalls) != 1 {
			t.Fatal("stop did not reach runtime")
		}
	})
}

func TestSupportPreviewGenericRunSurfacesDoNotLeakText(t *testing.T) {
	_, preview, _, _, ctx := previewFixture(t)
	started, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "Private test text"})
	if err != nil {
		t.Fatal(err)
	}
	outsider := authorization.WithActor(ctx, &authorization.Actor{WorkspaceID: "workspace", UserID: "other", Role: model.RoleOwner})
	if _, err = preview.agents.GetAgentRun(outsider, "workspace", started.RunID); err == nil {
		t.Fatal("generic detail leaked preview")
	}
	if _, err = preview.agents.ListRunMessages(outsider, "workspace", started.RunID); err == nil {
		t.Fatal("generic transcript leaked preview")
	}
	if _, err = preview.agents.ListRunArtifacts(outsider, "workspace", started.RunID); err == nil {
		t.Fatal("generic artifacts leaked preview")
	}
	run, err := preview.agents.runRepo.GetByID(ctx, "workspace", started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	rows := preview.agents.normalizeRunCollection([]model.AgentRun{*run})
	if len(rows[0].Input) != 0 || len(rows[0].OutputSummary) != 0 {
		t.Fatal("run lists contain preview text")
	}
	publisher := &fakeAgentRuntimeProjectionPublisher{}
	projection := &AgentRuntimeProjectionService{wsPublisher: publisher}
	projection.publishRuntimeCodingSessionEvent(run, AgentRuntimeEventEnvelope{Type: "assistant.message", Data: map[string]any{"content": "private"}})
	projection.publishRuntimeInteractionEvent(run, model.AgentRunInteraction{})
	if len(publisher.events) != 0 {
		t.Fatal("preview broadcast to workspace")
	}
}

func TestSupportPreviewSkipReplyHasNoLiveEffects(t *testing.T) {
	db, preview, commands, _, ctx := previewFixture(t)
	result, err := preview.Start(ctx, "workspace", "agent-1", model.SupportAIPreviewRequest{Message: "A notification with no question"})
	if err != nil {
		t.Fatal(err)
	}
	meta := model.InternalCommandContext{WorkspaceID: "workspace", AgentID: "agent-1", RunID: "run_runtime_1", TargetType: supportPreviewTarget, TargetID: result.RunID}
	if _, err = commands.Execute(ctx, meta, "support.skip_reply", json.RawMessage(`{"reason":"spam","summary":"Impersonation with unrelated link"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := preview.Get(ctx, "workspace", "agent-1", result.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FinalDecision != "no_reply" || got.Answer != nil {
		t.Fatalf("unexpected outcome: %+v", got)
	}
	for _, table := range []string{"support_conversations", "support_messages"} {
		var count int64
		if err = db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("preview modified %s: %d %v", table, count, err)
		}
	}
}
