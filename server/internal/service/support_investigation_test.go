package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportInvestigationRequestEligibility(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws"}
	for _, tc := range []struct {
		name, kind, question, key string
		want                      bool
	}{
		{"normal handover", "", "", "", false},
		{"billing", "billing", "check billing", "billing", false},
		{"missing question", "suspected_product_bug", "", "bug", false},
		{"missing stable issue", "suspected_product_bug", "inspect export", "", false},
		{"bug", "suspected_product_bug", "inspect export", "export-error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := newSupportInvestigation(conv, tc.key, SupportHandoffBrief{InvestigationKind: tc.kind, InvestigationQuestion: tc.question, Issue: "Export fails"}, time.Now())
			if (got != nil) != tc.want {
				t.Fatalf("request = %#v", got)
			}
		})
	}
	a := newSupportInvestigation(conv, " export-error ", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "inspect export"}, time.Now())
	b := newSupportInvestigation(conv, "EXPORT-ERROR", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "inspect more"}, time.Now())
	if a.ID != b.ID || a.RunID != b.RunID {
		t.Fatal("same issue must deduplicate")
	}
}

func TestSupportInvestigationRunContract(t *testing.T) {
	tools := supportInvestigationTools()
	for _, tool := range tools {
		if !supportChildReadOnlyTools[tool] && tool != "read_skill" && tool != "find_skills" {
			t.Fatalf("unexpected tool %s", tool)
		}
	}
	payload, err := supportInvestigationRunInput(model.SupportInvestigation{ID: "job", RunID: "run", WorkspaceID: "ws", ConversationID: "conv", Question: "inspect", Brief: "facts"})
	if err != nil {
		t.Fatal(err)
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatal(err)
	}
	if input.Trigger == nil || input.Trigger.TriggerType != supportInvestigationTriggerType {
		t.Fatal("missing independent trigger")
	}
	if input.Target == nil || input.Target.TargetType != supportInvestigationTarget {
		t.Fatal("must not be a customer reply run")
	}
}

func TestSupportInvestigationAtomicHandoverAndCompletion(t *testing.T) {
	ctx := context.Background()
	svc, db, conv, _ := setupAIControlTest(t)
	if err := db.AutoMigrate(&model.SupportInvestigation{}); err != nil {
		t.Fatal(err)
	}
	job := newSupportInvestigation(conv, "export-error", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "Why does export fail?"}, time.Now().UTC())
	repo := repository.NewSupportInvestigationRepository(db)
	noChange := func(*model.SupportConversation) (map[string]any, *model.SupportMessage, error) { return nil, nil, nil }
	if _, changed, err := svc.conversationRepo.ChangeAIControlWithInvestigation(ctx, "ws", "conv", noChange, job); err != nil || changed {
		t.Fatalf("no-op %v %v", changed, err)
	}
	var count int64
	db.Model(&model.SupportInvestigation{}).Count(&count)
	if count != 0 {
		t.Fatal("queued without handover")
	}
	transition := func(*model.SupportConversation) (map[string]any, *model.SupportMessage, error) {
		return map[string]any{"ai_state": "escalated", "human_takeover": true}, nil, nil
	}
	badNote := &model.SupportMessage{ID: "bad", WorkspaceID: "other", ConversationID: "conv"}
	if _, _, err := svc.conversationRepo.ChangeAIControlWithInvestigation(ctx, "ws", "conv", transition, job, badNote); err == nil {
		t.Fatal("expected rollback")
	}
	db.Model(&model.SupportInvestigation{}).Count(&count)
	if count != 0 {
		t.Fatal("queued after rollback")
	}
	if derefString(readControlConversation(t, db).AIState) != "pending" {
		t.Fatal("failed transaction changed ownership")
	}
	for range 2 {
		if _, changed, err := svc.conversationRepo.ChangeAIControlWithInvestigation(ctx, "ws", "conv", transition, job); err != nil || !changed {
			t.Fatalf("handover %v %v", changed, err)
		}
	}
	db.Model(&model.SupportInvestigation{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate jobs: %d", count)
	}
	claimed, err := repo.Claim(ctx, time.Now().Add(time.Second))
	if err != nil || claimed == nil {
		t.Fatalf("claim %v %v", claimed, err)
	}
	if duplicate, err := repo.Claim(ctx, time.Now().Add(time.Second)); err != nil || duplicate != nil {
		t.Fatalf("duplicate lease %v %v", duplicate, err)
	}
	before := readControlConversation(t, db)
	chat := &SupportChatService{investigationRepo: repo}
	for range 2 {
		if err := chat.finishSupportInvestigation(ctx, claimed, "Observed facts and hypotheses"); err != nil {
			t.Fatal(err)
		}
	}
	var notes []model.SupportMessage
	if err := db.Where("id = ?", job.ID).Find(&notes).Error; err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || !notes[0].IsInternal || notes[0].MessageType != "note" {
		t.Fatalf("unexpected notes %#v", notes)
	}
	after := readControlConversation(t, db)
	if after.AIControlVersion != before.AIControlVersion || derefString(after.AIState) != "escalated" || after.AIActiveRunID != nil || (after.HumanTakeover == nil || !*after.HumanTakeover) {
		t.Fatal("research changed ownership")
	}
}

func TestSupportInvestigationDoesNotHideForegroundRun(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newDelegatedSupportRunService(t, db, runtimeClient)
	foreground := model.AgentRun{ID: "foreground", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "workspace", TargetID: "ws-1", Status: "running", Input: json.RawMessage(`{}`), CreatedAt: now.Add(-time.Minute), UpdatedAt: now}
	background := model.AgentRun{ID: "background", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: supportInvestigationTarget, TargetID: "background", Status: "running", Input: json.RawMessage(`{"trigger":{"source":"system","trigger_type":"support_background_investigation"}}`), CreatedAt: now, UpdatedAt: now}
	for _, run := range []*model.AgentRun{&foreground, &background} {
		if err := svc.runRepo.Create(context.Background(), run); err != nil {
			t.Fatal(err)
		}
	}
	agent, err := svc.agentRepo.GetByID(context.Background(), "ws-1", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.createRun(context.Background(), createRunParams{workspaceID: "ws-1", agent: agent, targetType: "workspace", targetID: "ws-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != foreground.ID || len(runtimeClient.startRunCalls) != 0 {
		t.Fatal("ordinary launch reused investigation")
	}
}

func TestSupportInvestigationLaunchIsIndependentAndReadOnly(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newDelegatedSupportRunService(t, db, runtimeClient)
	agent, err := svc.agentRepo.GetByID(context.Background(), "ws-1", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	agent.PresetKey = model.AgentPresetCommandAgent
	agent.AllowedTools, _ = json.Marshal(append(supportInvestigationTools(), "send_support_reply", "write_file", "start_agent_run"))
	agent.AllowedTargets = json.RawMessage(`["workspace"]`)
	// Include skill discovery in the registered managed command-agent surface.
	agent.PresetVersionKey = "command_agent_default"
	if err := svc.agentRepo.Update(context.Background(), agent); err != nil {
		t.Fatal(err)
	}
	foreground := &model.AgentRun{ID: "live", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "workspace", TargetID: "ws-1", Status: "running", Input: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := svc.runRepo.Create(context.Background(), foreground); err != nil {
		t.Fatal(err)
	}
	job := model.SupportInvestigation{ID: "job", RunID: "reserved-research", WorkspaceID: "ws-1", ConversationID: "conv-1", Question: "Check export"}
	input, err := supportInvestigationRunInput(job)
	if err != nil {
		t.Fatal(err)
	}
	var payload model.AgentRunInputPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		t.Fatal(err)
	}
	run, err := svc.createRun(context.Background(), createRunParams{runID: job.RunID, workspaceID: job.WorkspaceID, agent: agent, targetType: supportInvestigationTarget, targetID: job.RunID, input: input, trigger: payload.Trigger, invocationMode: model.InvocationModeAutonomous})
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != job.RunID || run.ParentRunID != nil || run.ConversationID != nil {
		t.Fatalf("not independent: %+v", run)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatal("missing launch")
	}
	start := runtimeClient.startRunCalls[0]
	if start.TurnPolicy.Mode != "complete_on_finish" {
		t.Fatalf("wrong turn policy %+v", start.TurnPolicy)
	}
	if len(start.AllowedTools) != len(supportInvestigationTools()) {
		t.Fatalf("wrong allowed tools %v", start.AllowedTools)
	}
	for _, tool := range start.AllowedTools {
		if strings.Contains(tool, "reply") || strings.Contains(tool, "write") || strings.Contains(tool, "start_agent") {
			t.Fatalf("unsafe tool %s", tool)
		}
	}
	if len(runtimeClient.cancelCalls) != 0 || len(runtimeClient.resumeCalls) != 0 {
		t.Fatal("research affected foreground")
	}
}

func TestSupportInvestigationLeaseRecoveryAndPrivateNotes(t *testing.T) {
	for _, mode := range []string{"public", "wrong_workspace", "stale_lease", "anonymized", "merged"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, db, conv, _ := setupAIControlTest(t)
			if err := db.AutoMigrate(&model.SupportInvestigation{}); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			job := newSupportInvestigation(conv, "bug", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "inspect"}, now)
			if err := db.Create(job).Error; err != nil {
				t.Fatal(err)
			}
			repo := repository.NewSupportInvestigationRepository(db)
			first, err := repo.Claim(ctx, now.Add(time.Second))
			if err != nil || first == nil {
				t.Fatalf("claim %v", err)
			}
			second, err := repo.Claim(ctx, now.Add(3*time.Minute))
			if err != nil || second == nil {
				t.Fatalf("recover lease %v", err)
			}
			note := &model.SupportMessage{ID: job.ID, WorkspaceID: "ws", ConversationID: "conv", MessageType: "note", SenderType: "agent", Content: "findings", IsInternal: true}
			chosen := second
			wantError := false
			switch mode {
			case "public":
				note.IsInternal = false
				wantError = true
			case "wrong_workspace":
				note.WorkspaceID = "other"
				wantError = true
			case "stale_lease":
				chosen = first
			case "anonymized":
				if err := svc.conversationRepo.UpdateFields(ctx, "ws", "conv", map[string]any{"anonymized_at": now}); err != nil {
					t.Fatal(err)
				}
			case "merged":
				if err := svc.conversationRepo.UpdateFields(ctx, "ws", "conv", map[string]any{"merged_into_id": "another"}); err != nil {
					t.Fatal(err)
				}
			}
			inserted, err := repo.Finish(ctx, chosen, note)
			if (err != nil) != wantError || inserted {
				t.Fatalf("Finish inserted=%v err=%v", inserted, err)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("id = ?", job.ID).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("invalid note was persisted")
			}
			var saved model.SupportInvestigation
			if err := db.First(&saved, "id = ?", job.ID).Error; err != nil {
				t.Fatal(err)
			}
			if (wantError || mode == "stale_lease") && saved.Status != "pending" {
				t.Fatal("failed completion lost pending job")
			}
		})
	}
}

func TestSupportInvestigationHostContextScope(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	repo := repository.NewAgentRunRepository(db)
	job := model.SupportInvestigation{ID: "job", RunID: "investigation", WorkspaceID: "ws-1", ConversationID: "conv-1"}
	input, err := supportInvestigationRunInput(job)
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{ID: job.RunID, WorkspaceID: job.WorkspaceID, AgentID: "agent-1", TargetType: supportInvestigationTarget, TargetID: job.RunID, Input: input, Status: "running"}
	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	host := &AgentRuntimeHostService{appID: "helpin", runRepo: repo}
	for _, tc := range []struct {
		name, workspace, agent, target string
		want                           bool
	}{
		{"owned", "ws-1", "agent-1", job.RunID, true},
		{"cross workspace", "other", "agent-1", job.RunID, false},
		{"wrong agent", "ws-1", "other", job.RunID, false},
		{"unknown job", "ws-1", "agent-1", "missing", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{AppID: "helpin", AgentID: tc.agent, Target: agentruntime.TargetRef{Type: supportInvestigationTarget, ID: tc.target}, Metadata: map[string]interface{}{"workspace_id": tc.workspace}})
			if (err == nil) != tc.want {
				t.Fatalf("context=%+v err=%v", result, err)
			}
		})
	}
}

func TestSupportInvestigationRejectsExpandedToolContract(t *testing.T) {
	job := model.SupportInvestigation{ID: "job", RunID: "research", WorkspaceID: "ws"}
	input, err := supportInvestigationRunInput(job)
	if err != nil {
		t.Fatal(err)
	}
	var payload model.AgentRunInputPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		t.Fatal(err)
	}
	for _, tools := range [][]string{nil, {"send_support_reply"}, {"repository_search", "write_file"}} {
		payload.AllowedTools = tools
		raw, _ := json.Marshal(payload)
		run := &model.AgentRun{ID: job.RunID, TargetType: supportInvestigationTarget, TargetID: job.RunID, Input: raw}
		if _, err := runtimeStartRunRequest(run, nil, AgentRuntimeAgent{AllowedTools: []string{"send_support_reply", "write_file", "repository_search"}}); err == nil {
			t.Fatalf("accepted unsafe tools %v", tools)
		}
	}
}

func TestSupportInvestigationCompletionDoesNotTriggerAutomation(t *testing.T) {
	rules := &fakeFinalizerRuleEvaluator{}
	messages := &fakeFinalizerSupportMessageRepo{}
	finalizer := &AgentRunFinalizerService{ruleEngine: rules, supportMessageRepo: messages}
	job := model.SupportInvestigation{ID: "job", RunID: "run", WorkspaceID: "ws"}
	input, err := supportInvestigationRunInput(job)
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{ID: "run", TargetType: supportInvestigationTarget, TargetID: "run", Status: model.AgentRunStatusCompleted, Input: input, OutputSummary: json.RawMessage(`{"draft_reply":{"content":"Do not send"}}`)}
	finalizer.FinalizeTerminalRun(context.Background(), run, true)
	if len(rules.events) != 0 || messages.creates != 0 {
		t.Fatal("investigation triggered public side effects")
	}
}

func TestSupportInvestigationRestartDoesNotRelaunch(t *testing.T) {
	ctx := context.Background()
	svc, db, conv, _ := setupAIControlTest(t)
	mustExec(t, db, `CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, target_type TEXT, target_id TEXT, status TEXT, input TEXT, created_at DATETIME)`)
	job := newSupportInvestigation(conv, "bug", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "inspect"}, time.Now().UTC())
	now := time.Now().UTC()
	job.StartedAt = &now
	input, err := supportInvestigationRunInput(*job)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO agent_runs (id,workspace_id,target_type,target_id,status,input) VALUES (?,?,?,?,?,?)`, job.RunID, "ws", supportInvestigationTarget, job.RunID, "running", input)
	// No runtime client is needed: recovery must observe the durable run and
	// never submit a new launch, even after a worker restart loses local state.
	chat := &SupportChatService{conversationRepo: svc.conversationRepo, runRepo: repository.NewAgentRunRepository(db), agentService: &AgentService{}}
	for range 2 {
		if err := chat.processSupportInvestigation(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
}

type investigationRunCloser struct {
	calls int
	err   error
}

func (c *investigationRunCloser) CancelRun(_ context.Context, _, id, _ string) (*model.AgentRun, error) {
	c.calls++
	return &model.AgentRun{ID: id, Status: "cancelled"}, c.err
}

func TestSupportInvestigationTimeoutRequiresCancellation(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			ctx := context.Background()
			svc, db, conv, _ := setupAIControlTest(t)
			if err := db.AutoMigrate(&model.SupportInvestigation{}); err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, `CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, target_type TEXT, target_id TEXT, status TEXT, input BLOB)`)
			past := time.Now().UTC().Add(-15 * time.Minute)
			job := newSupportInvestigation(conv, "bug", SupportHandoffBrief{InvestigationKind: "suspected_product_bug", InvestigationQuestion: "inspect"}, past)
			job.StartedAt = &past
			if err := db.Create(job).Error; err != nil {
				t.Fatal(err)
			}
			repo := repository.NewSupportInvestigationRepository(db)
			claimed, err := repo.Claim(ctx, time.Now().UTC())
			if err != nil || claimed == nil {
				t.Fatalf("claim %v", err)
			}
			input, err := supportInvestigationRunInput(*job)
			if err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, `INSERT INTO agent_runs(id,workspace_id,target_type,target_id,status,input) VALUES (?,?,?,?,?,?)`, job.RunID, "ws", supportInvestigationTarget, job.RunID, "running", input)
			closer := &investigationRunCloser{}
			if fail {
				closer.err = fmt.Errorf("runtime unavailable")
			}
			chat := &SupportChatService{conversationRepo: svc.conversationRepo, runRepo: repository.NewAgentRunRepository(db), investigationRepo: repo, runCloser: closer}
			err = chat.processSupportInvestigation(ctx, claimed)
			if (err != nil) != fail || closer.calls != 1 {
				t.Fatalf("process err=%v cancellation calls=%d", err, closer.calls)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("id = ? AND is_internal = true", job.ID).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			want := int64(1)
			if fail {
				want = 0
			}
			if count != want {
				t.Fatalf("notes=%d want=%d", count, want)
			}
		})
	}
}
