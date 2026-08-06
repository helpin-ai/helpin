package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDockExecutionGuardTest(t *testing.T) (*InternalCommandService, *gorm.DB, model.InternalCommandContext) {
	t.Helper()
	dbName := fmt.Sprintf("file:dock_execution_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_id TEXT NOT NULL, task_id TEXT, conversation_id TEXT,
		target_type TEXT NOT NULL, target_id TEXT NOT NULL, runtime_kind TEXT NOT NULL, invocation_mode TEXT NOT NULL,
		parent_run_id TEXT, dock_chat_id TEXT, handoff_state TEXT, approval_state TEXT NOT NULL DEFAULT 'not_required',
		pause_reason TEXT NOT NULL DEFAULT 'none', triggered_by_user_id TEXT, status TEXT NOT NULL, workflow_id TEXT,
		workflow_run_id TEXT, external_runtime TEXT, external_runtime_id TEXT, task_queue TEXT, runner_pool TEXT,
		agent_version_id TEXT, repository_id TEXT, repo_full_name TEXT, base_branch TEXT, working_branch TEXT,
		delivery_target_id TEXT, execution_stage TEXT, last_heartbeat_at DATETIME, input TEXT NOT NULL DEFAULT '{}',
		output_summary TEXT NOT NULL DEFAULT '{}', cached_input_tokens INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0, tokens_used INTEGER NOT NULL DEFAULT 0,
		error_message TEXT, started_at DATETIME, completed_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent_runs: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dock_action_proposals (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, dock_chat_run_id TEXT NOT NULL, actor_id TEXT NOT NULL,
		kind TEXT NOT NULL, status TEXT NOT NULL, summary TEXT NOT NULL, spec TEXT NOT NULL DEFAULT '{}',
		usage TEXT NOT NULL DEFAULT '{}', approval_interaction_id TEXT, expires_at DATETIME NOT NULL,
		activated_at DATETIME, completed_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create dock_action_proposals: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_run_interactions (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, run_id TEXT NOT NULL, runtime_kind TEXT NOT NULL,
		interaction_kind TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending', request_schema_version TEXT NOT NULL,
		response_schema_version TEXT, request_id TEXT, thread_id TEXT, turn_id TEXT, item_id TEXT, approval_id TEXT,
		assistant_message_sequence_no INTEGER, title TEXT, summary TEXT, request_payload TEXT NOT NULL DEFAULT '{}',
		response_payload TEXT, runtime_metadata TEXT NOT NULL DEFAULT '{}', resolved_by TEXT, resolved_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent_run_interactions: %v", err)
	}
	dockID := "dock-1"
	run := &model.AgentRun{
		ID: "run-1", WorkspaceID: "ws-1", AgentID: "ask-agent", TargetType: "workspace", TargetID: "ws-1",
		RuntimeKind: "native_sdk", InvocationMode: "interactive", Status: "running", DockChatID: &dockID,
		Input: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{}`),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create dock run: %v", err)
	}
	svc := &InternalCommandService{
		definitions:             map[string]InternalCommandDefinition{},
		agentRunRepo:            repository.NewAgentRunRepository(db),
		agentRunInteractionRepo: repository.NewAgentRunInteractionRepository(db),
		dockActionProposalRepo:  repository.NewDockActionProposalRepository(db),
	}
	svc.register(InternalCommandDefinition{
		Name: "docs.test_mutation", Module: "docs", Mutating: true,
		Tool: &commandtools.RuntimeToolMetadata{CommandName: "docs.test_mutation", Alias: "test_document_mutation"},
		Execute: func(_ context.Context, _ model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return input, nil
		},
	})
	return svc, db, model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "user-1", RunID: run.ID, TargetType: "workspace", TargetID: "ws-1"}
}

func TestDockMutationRequiresMatchingBoundedGrant(t *testing.T) {
	svc, db, meta := setupDockExecutionGuardTest(t)
	ctx := context.Background()

	if _, err := svc.Execute(ctx, meta, "docs.test_mutation", json.RawMessage(`{"document_id":"doc-1"}`)); err == nil || !strings.Contains(err.Error(), "dock_execution_approval_required") {
		t.Fatalf("mutation without grant error = %v", err)
	}

	now := time.Now().UTC()
	spec, _ := json.Marshal(dockExecutionSpec{Operations: []dockExecutionOperation{
		{ToolName: "test_document_mutation", MaxCalls: 1, Constraints: map[string]interface{}{"document_id": "doc-1"}},
		{ToolName: "test_document_mutation", MaxCalls: 1, Constraints: map[string]interface{}{"document_id": "doc-2"}},
	}})
	proposal := &model.DockActionProposal{
		ID: "proposal-1", WorkspaceID: meta.WorkspaceID, DockChatRunID: meta.RunID, ActorID: meta.ActorID,
		Kind: model.DockActionProposalKindExecution, Status: model.DockActionProposalStatusActive,
		Summary: "Update two documents", Spec: spec, Usage: json.RawMessage(`{}`), ExpiresAt: now.Add(time.Hour), ActivatedAt: &now,
	}
	if err := db.Create(proposal).Error; err != nil {
		t.Fatalf("create active proposal: %v", err)
	}

	if _, err := svc.Execute(ctx, meta, "docs.test_mutation", json.RawMessage(`{"document_id":"doc-other"}`)); err == nil {
		t.Fatal("mutation outside approved constraints succeeded")
	}
	for _, documentID := range []string{"doc-1", "doc-2"} {
		input, _ := json.Marshal(map[string]string{"document_id": documentID})
		if _, err := svc.Execute(ctx, meta, "docs.test_mutation", input); err != nil {
			t.Fatalf("approved mutation for %s failed: %v", documentID, err)
		}
	}
	if _, err := svc.Execute(ctx, meta, "docs.test_mutation", json.RawMessage(`{"document_id":"doc-1"}`)); err == nil {
		t.Fatal("mutation exceeded approved max_calls")
	}

	stored, err := svc.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, proposal.ID)
	if err != nil {
		t.Fatalf("get proposal: %v", err)
	}
	usage := map[string]int{}
	if err := json.Unmarshal(stored.Usage, &usage); err != nil {
		t.Fatalf("decode usage: %v", err)
	}
	if usage["0:test_document_mutation"] != 1 || usage["1:test_document_mutation"] != 1 {
		t.Fatalf("usage = %#v, want one call for each scoped operation", usage)
	}
}

func TestRiskBasedDockMutationExecutesWithoutLegacyApprovalFailure(t *testing.T) {
	svc, db, meta := setupDockExecutionGuardTest(t)
	if err := db.Exec(`CREATE TABLE agents (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, is_system BOOLEAN NOT NULL,
		preset_key TEXT, preset_version_key TEXT, source_preset_key TEXT, source_preset_version_key TEXT,
		status TEXT, runtime_kind TEXT, approval_mode TEXT, allowed_tools BLOB, allowed_commands BLOB,
		allowed_targets BLOB, skills BLOB, execution_config BLOB, default_invocation_mode TEXT,
		created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agents: %v", err)
	}
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, preset_key, preset_version_key, status, runtime_kind, approval_mode,
		allowed_tools, allowed_commands, allowed_targets, skills, execution_config, default_invocation_mode
	) VALUES (?, ?, true, ?, ?, 'active', 'native_sdk', 'risk_based', CAST('[]' AS BLOB), CAST('[]' AS BLOB), CAST('["workspace"]' AS BLOB), CAST('[]' AS BLOB), CAST('{}' AS BLOB), 'interactive')`,
		"ask-agent", meta.WorkspaceID, model.AgentPresetAskAgent,
		defaultPresetVersionKeyForPresetKey(model.AgentPresetAskAgent)).Error; err != nil {
		t.Fatalf("create Ask Agent: %v", err)
	}
	svc.agentService = &AgentService{agentRepo: repository.NewAgentRepository(db)}
	meta.AgentID = "ask-agent"
	input := json.RawMessage(`{"document_id":"doc-1"}`)

	output, err := svc.Execute(context.Background(), meta, "docs.test_mutation", input)
	if err != nil {
		t.Fatalf("risk-based routine mutation returned a legacy approval failure: %v", err)
	}
	if string(output) != string(input) {
		t.Fatalf("output = %s, want %s", output, input)
	}
}

func TestInternalCommandRegistrationDefaultsUnclassifiedMutationToSensitive(t *testing.T) {
	svc := &InternalCommandService{definitions: map[string]InternalCommandDefinition{}}
	svc.register(InternalCommandDefinition{
		Name: "docs.unclassified", Mutating: true,
		Tool: &commandtools.RuntimeToolMetadata{CommandName: "docs.unclassified", Alias: "unclassified"},
	})
	def, ok := svc.Definition("docs.unclassified")
	if !ok || def.Tool == nil || def.Tool.RiskLevel != commandtools.RiskLevelSensitive {
		t.Fatalf("unclassified mutation did not default to sensitive: %#v", def)
	}
}

func TestDockMutationEarlyCallReturnsCompactApprovalAndRetryAutoActivates(t *testing.T) {
	svc, db, meta := setupDockExecutionGuardTest(t)
	ctx := context.Background()
	input := json.RawMessage(`{"space_id":"space-1","title":"Kafka Producer Architecture","content":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"large prepared findings"}]}]}}`)

	_, firstErr := svc.Execute(ctx, meta, "docs.test_mutation", input)
	if firstErr == nil {
		t.Fatal("mutation without approval unexpectedly succeeded")
	}
	var required struct {
		Code       string `json:"code"`
		ProposalID string `json:"proposal_id"`
		NextTool   string `json:"next_tool"`
		NextInput  struct {
			Phase  string `json:"phase"`
			Action struct {
				ProposalID string `json:"proposal_id"`
			} `json:"action"`
		} `json:"next_input"`
	}
	if err := json.Unmarshal([]byte(firstErr.Error()), &required); err != nil {
		t.Fatalf("approval-required error is not structured JSON: %v (%v)", err, firstErr)
	}
	if required.Code != "dock_execution_approval_required" || required.ProposalID == "" || required.NextTool != "request_approval" || required.NextInput.Phase != dockExecutionApprovalPhase || required.NextInput.Action.ProposalID != required.ProposalID {
		t.Fatalf("unexpected approval-required contract: %#v", required)
	}
	if strings.Contains(firstErr.Error(), "large prepared findings") {
		t.Fatalf("approval-required error echoed large mutation input: %s", firstErr)
	}

	now := time.Now().UTC()
	interaction := &model.AgentRunInteraction{
		ID: "interaction-single-mutation", WorkspaceID: meta.WorkspaceID, RunID: meta.RunID,
		RuntimeKind: "native_sdk", InteractionKind: model.AgentRunInteractionKindApprovalRequest,
		Status: model.AgentRunInteractionStatusResolved, RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:  json.RawMessage(fmt.Sprintf(`{"phase":%q,"action":{"proposal_id":%q}}`, dockExecutionApprovalPhase, required.ProposalID)),
		ResponsePayload: json.RawMessage(`{"decision":"approve"}`), ResolvedAt: &now,
		RuntimeMetadata: json.RawMessage(`{}`),
	}
	if err := db.Create(interaction).Error; err != nil {
		t.Fatalf("create resolved approval: %v", err)
	}
	widenedInput := json.RawMessage(`{"space_id":"space-1","title":"Kafka Producer Architecture","content":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"large prepared findings"}]}]},"icon":"book"}`)
	if _, err := svc.Execute(ctx, meta, "docs.test_mutation", widenedInput); err == nil || !strings.Contains(err.Error(), "dock_execution_approval_required") {
		t.Fatalf("mutation with an unapproved added field was not rejected: %v", err)
	}

	output, err := svc.Execute(ctx, meta, "docs.test_mutation", input)
	if err != nil {
		t.Fatalf("identical mutation retry after approval failed: %v", err)
	}
	if string(output) != string(input) {
		t.Fatalf("mutation output = %s, want %s", output, input)
	}
	proposal, err := svc.dockActionProposalRepo.GetByID(ctx, meta.WorkspaceID, required.ProposalID)
	if err != nil || proposal == nil {
		t.Fatalf("load auto-activated proposal: proposal=%#v err=%v", proposal, err)
	}
	if proposal.Status != model.DockActionProposalStatusActive || proposal.ApprovalInteractionID == nil || *proposal.ApprovalInteractionID != interaction.ID {
		t.Fatalf("proposal was not lazily activated: %#v", proposal)
	}
	usage := map[string]int{}
	if err := json.Unmarshal(proposal.Usage, &usage); err != nil || usage["0:test_document_mutation"] != 1 {
		t.Fatalf("approved retry did not consume exactly one call: usage=%#v err=%v", usage, err)
	}
}
