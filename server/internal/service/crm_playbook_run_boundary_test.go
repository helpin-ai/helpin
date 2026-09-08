package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func playbookRunContextJSON(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(model.AgentRunInputPayload{CRMPlaybook: &model.CRMPlaybookRunContext{
		SchemaVersion: 1, WorkspaceID: f.Workspace, PlaybookID: uuid.NewString(), PlaybookVersionID: uuid.NewString(),
		SituationID: uuid.NewString(), SituationRevision: 1, ConnectionID: uuid.NewString(), ConnectionVersion: 1,
		ConnectionFingerprint: "published", SpecializationVersion: "skills", Target: model.AgentRunTargetContext{TargetType: "crm_company", TargetID: f.Company},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func playbookRunBoundaryFixture(t *testing.T) (*gorm.DB, *repository.AgentRunRepository, model.AgentRun) {
	t.Helper()
	db := f.Open(t)
	f.Exec(t, db, `CREATE TABLE agent_runs (id uuid PRIMARY KEY,workspace_id uuid,agent_id uuid,target_type text,target_id uuid,
		input jsonb,status text,pause_reason text,approval_state text,external_runtime text,external_runtime_id text,
		created_at timestamp,updated_at timestamp)`)
	run := model.AgentRun{ID: uuid.NewString(), WorkspaceID: f.Workspace, AgentID: uuid.NewString(), TargetType: "crm_company", TargetID: f.Company,
		Input: playbookRunContextJSON(t), Status: "paused", PauseReason: model.AgentRunPauseReasonHumanApproval, ApprovalState: "pending",
		ExternalRuntime: f.Ptr(agentRuntimeName), ExternalRuntimeID: f.Ptr("runtime-run"), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	f.Exec(t, db, `INSERT INTO agent_runs (id,workspace_id,agent_id,target_type,target_id,input,status,pause_reason,approval_state,external_runtime,external_runtime_id,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, run.ID, run.WorkspaceID, run.AgentID, run.TargetType, run.TargetID, []byte(run.Input), run.Status, run.PauseReason, run.ApprovalState, *run.ExternalRuntime, *run.ExternalRuntimeID, run.CreatedAt, run.UpdatedAt)
	return db, repository.NewAgentRunRepository(db), run
}

func TestCRMPlaybookRunRejectsUnclaimedLaunchBeforeDependencies(t *testing.T) {
	for _, input := range []json.RawMessage{playbookRunContextJSON(t), json.RawMessage(`{"crm_playbook":{}}`), json.RawMessage(`{"CRM_PLAYBOOK":{}}`)} {
		svc := &AgentService{}
		if _, err := svc.createRun(context.Background(), createRunParams{input: input}); !errors.Is(err, ErrCRMPlaybookExecutionNotReady) {
			t.Fatalf("generic launch did not reject reserved context: %v", err)
		}
		if _, err := runtimeStartRunRequest(&model.AgentRun{Input: input}, nil, AgentRuntimeAgent{}); !errors.Is(err, ErrCRMPlaybookExecutionNotReady) {
			t.Fatalf("runtime request bypass: %v", err)
		}
	}
}

func TestCRMPlaybookRunScopesDoNotMergeCustomerObjectives(t *testing.T) {
	left := playbookRunContextJSON(t)
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(left, &input); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*model.CRMPlaybookRunContext)
	}{
		{name: "another objective", mutate: func(c *model.CRMPlaybookRunContext) { c.SituationID = uuid.NewString() }},
		{name: "another publication", mutate: func(c *model.CRMPlaybookRunContext) { c.ConnectionID = uuid.NewString() }},
		{name: "new facts", mutate: func(c *model.CRMPlaybookRunContext) { c.SituationRevision++ }},
		{name: "different policy", mutate: func(c *model.CRMPlaybookRunContext) { c.PlaybookVersionID = uuid.NewString() }},
		{name: "different workspace", mutate: func(c *model.CRMPlaybookRunContext) { c.WorkspaceID = f.ForeignWorkspace }},
		{name: "different customer", mutate: func(c *model.CRMPlaybookRunContext) { c.Target.TargetID = f.ForeignCompany }},
		{name: "different skills", mutate: func(c *model.CRMPlaybookRunContext) { c.SpecializationVersion = "changed" }},
		{name: "different connection content", mutate: func(c *model.CRMPlaybookRunContext) { c.ConnectionFingerprint = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := *input.CRMPlaybook
			tc.mutate(&c)
			right, err := json.Marshal(model.AgentRunInputPayload{CRMPlaybook: &c})
			if err != nil {
				t.Fatal(err)
			}
			if matches, err := crmPlaybookRunScopesMatch(left, right); err != nil || matches {
				t.Fatalf("merged scope: %v", err)
			}
		})
	}
	if matches, err := crmPlaybookRunScopesMatch(left, left); err != nil || !matches {
		t.Fatalf("identical scope: %v", err)
	}
	if matches, err := crmPlaybookRunScopesMatch(left, json.RawMessage(`{}`)); err != nil || matches {
		t.Fatalf("standalone borrowed Playbook: %v", err)
	}
	if matches, err := crmPlaybookRunScopesMatch(nil, json.RawMessage(`{"target":{"target_type":"crm_company"}}`)); err != nil || !matches {
		t.Fatalf("ordinary reuse changed: %v", err)
	}
}

func TestCRMPlaybookRunCannotBeReusedByStandaloneBeacon(t *testing.T) {
	_, repo, run := playbookRunBoundaryFixture(t)
	svc := &AgentService{runRepo: repo}
	_, err := svc.createRun(context.Background(), createRunParams{workspaceID: run.WorkspaceID, agent: &model.Agent{ID: run.AgentID}, targetType: run.TargetType, targetID: run.TargetID, input: []byte(`{}`)})
	if !errors.Is(err, ErrCRMPlaybookRunScopeConflict) {
		t.Fatalf("standalone run reused Playbook execution: %v", err)
	}
}

func TestCRMPlaybookRunBoundaryPreservesOrdinaryBeaconReuse(t *testing.T) {
	db, repo, run := playbookRunBoundaryFixture(t)
	f.Exec(t, db, "UPDATE agent_runs SET input = ? WHERE id = ?", []byte(`{"target":{"target_type":"crm_company"}}`), run.ID)
	svc := &AgentService{runRepo: repo}
	result, err := svc.createRun(context.Background(), createRunParams{workspaceID: run.WorkspaceID, agent: &model.Agent{ID: run.AgentID}, targetType: run.TargetType, targetID: run.TargetID, input: []byte(`{}`)})
	if err != nil || result == nil || result.ID != run.ID || result.Status != run.Status {
		t.Fatalf("ordinary active-run reuse changed: result=%v error=%v", result, err)
	}
}

func TestCRMPlaybookRunGenericApprovalsCannotAuthorizeActions(t *testing.T) {
	for _, intent := range []string{model.AgentRunResumeIntentApprove, model.AgentRunResumeIntentReply, model.AgentRunResumeIntentRequestChanges} {
		t.Run(intent, func(t *testing.T) {
			_, repo, run := playbookRunBoundaryFixture(t)
			svc := &AgentService{runRepo: repo}
			if _, err := svc.ResumeRun(context.Background(), run.WorkspaceID, run.ID, f.SalesUser, model.ResumeAgentRunRequest{Intent: intent, Content: "approved"}); !errors.Is(err, ErrCRMPlaybookExecutionNotReady) {
				t.Fatalf("generic decision bypassed action policy: %v", err)
			}
			saved, err := repo.GetByID(context.Background(), run.WorkspaceID, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Status != run.Status || saved.ApprovalState != "pending" {
				t.Fatal("decision changed run state")
			}
		})
	}
}

func TestCRMPlaybookRuntimeCallbacksCannotDropTheirStoredScope(t *testing.T) {
	for _, mapped := range []bool{true, false} {
		t.Run(map[bool]string{true: "mapped run", false: "early callback"}[mapped], func(t *testing.T) {
			db, repo, run := playbookRunBoundaryFixture(t)
			metadata := map[string]interface{}{}
			if !mapped {
				f.Exec(t, db, "UPDATE agent_runs SET external_runtime_id = NULL WHERE id = ?", run.ID)
				metadata["helpin_run_id"] = run.ID
			}
			called := false
			commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
			commands.register(InternalCommandDefinition{Name: "test.write", Mutating: true, Execute: func(context.Context, model.InternalCommandContext, json.RawMessage) (json.RawMessage, error) {
				called = true
				return json.RawMessage(`{}`), nil
			}})
			host := NewAgentRuntimeHostService("helpin", repo, nil, nil, nil, nil, nil, nil, nil, nil, commands, &GitService{})
			_, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{CommandName: "test.write", Meta: agentruntime.CommandExecutionContext{AppID: "helpin", RunID: *run.ExternalRuntimeID, WorkspaceID: run.WorkspaceID, AgentID: run.AgentID, RunInputMetadata: metadata}})
			if !errors.Is(err, ErrAgentRuntimeHostForbidden) || called {
				t.Fatalf("command escaped boundary: %v", err)
			}
			_, err = host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{AppID: "helpin", RunID: *run.ExternalRuntimeID, Key: "crm_record_operations", Metadata: metadata})
			if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("silently loaded latest core skill: %v", err)
			}
			_, err = host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{AppID: "helpin", RunID: *run.ExternalRuntimeID, SkillID: "helpin_builtin:crm_record_operations", Metadata: metadata})
			if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("by-ID lookup silently loaded latest core skill: %v", err)
			}
			_, err = host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{AppID: "helpin", RunID: *run.ExternalRuntimeID, Target: agentruntime.TargetRef{Type: run.TargetType, ID: run.TargetID}, Metadata: metadata})
			if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("target callback escaped boundary: %v", err)
			}
			_, err = host.ResolveRepositorySpec(context.Background(), agentruntime.PrepareWorkspaceRequest{AppID: "helpin", RunID: *run.ExternalRuntimeID, Metadata: metadata})
			if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("repository callback escaped boundary: %v", err)
			}
		})
	}
}

func TestCRMPlaybookRunCannotContinueHandoffOrResumeThroughAuthentication(t *testing.T) {
	for _, action := range []string{"continue", "handoff", "authentication", "runtime resume"} {
		t.Run(action, func(t *testing.T) {
			db, repo, run := playbookRunBoundaryFixture(t)
			if action == "continue" {
				run.Status = model.AgentRunStatusFailed
				f.Exec(t, db, "UPDATE agent_runs SET status = ? WHERE id = ?", run.Status, run.ID)
			}
			svc := &AgentService{runRepo: repo}
			ctx := context.Background()
			var err error
			switch action {
			case "continue":
				_, err = svc.ContinueTerminalRun(ctx, run.WorkspaceID, run.ID, f.SalesUser, model.ContinueAgentRunRequest{})
			case "handoff":
				_, err = svc.HandoffRun(ctx, run.WorkspaceID, run.ID, f.SalesUser, model.HandoffAgentRunRequest{Reason: "Please finish this"})
			case "authentication":
				_, _, err = svc.loadRunAndAgentForCodexAuth(ctx, run.WorkspaceID, run.ID)
			case "runtime resume":
				_, _, err = svc.resumeAgentRuntimeRunWithIntent(ctx, run.WorkspaceID, &run, *run.ExternalRuntimeID, f.SalesUser, model.ResumeAgentRunRequest{}, model.AgentRunResumeIntentAuthCompleted)
			}
			if !errors.Is(err, ErrCRMPlaybookExecutionNotReady) {
				t.Fatalf("unguarded continuation: %v", err)
			}
			saved, err := repo.GetByID(ctx, run.WorkspaceID, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Status != run.Status || saved.ApprovalState != run.ApprovalState {
				t.Fatal("rejected continuation changed run state")
			}
		})
	}
}

func TestCRMPlaybookRuntimeBoundaryPreservesOrdinarySkillLookup(t *testing.T) {
	db, repo, run := playbookRunBoundaryFixture(t)
	f.Exec(t, db, "UPDATE agent_runs SET input = ? WHERE id = ?", []byte(`{}`), run.ID)
	host := NewAgentRuntimeHostService("helpin", repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	metadata := map[string]interface{}{"helpin_run_id": run.ID}
	byKey, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{AppID: "helpin", RunID: *run.ExternalRuntimeID, Key: "crm_record_operations", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	byID, err := host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{AppID: "helpin", RunID: *run.ExternalRuntimeID, SkillID: byKey.ID, Metadata: metadata})
	if err != nil || byID == nil || byID.PackageChecksum != byKey.PackageChecksum {
		t.Fatalf("ordinary skill lookup changed: %v", err)
	}
}

func TestCRMPlaybookRuntimeMetadataCannotAuthorizeUnclaimedWork(t *testing.T) {
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, key := range []string{"crm_playbook", "crm_playbook_connection_id", "crm_playbook_situation_id"} {
		for _, value := range []interface{}{nil, "", map[string]interface{}{"execution_enabled": true, "approved": true}} {
			_, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{AppID: "helpin", Key: "crm_record_operations", Target: agentruntime.TargetRef{Metadata: map[string]interface{}{key: value}}})
			if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("metadata %s authorized work: %v", key, err)
			}
		}
	}
}

func TestCRMPlaybookRuntimeProfileDoesNotReconfigureSharedBeacon(t *testing.T) {
	connection := model.CRMPlaybookConnection{ID: uuid.NewString(), WorkspaceID: f.Workspace, Fingerprint: "one-publication"}
	first := crmPlaybookRuntimeProfileID(connection)
	if first != crmPlaybookRuntimeProfileID(connection) {
		t.Fatal("published profile is unstable")
	}
	connection.Fingerprint = "another-publication"
	if first == crmPlaybookRuntimeProfileID(connection) {
		t.Fatal("later settings overwrite approved profile")
	}
	connection.Fingerprint = "one-publication"
	connection.WorkspaceID = f.ForeignWorkspace
	if first == crmPlaybookRuntimeProfileID(connection) {
		t.Fatal("workspace profiles collide")
	}
}
