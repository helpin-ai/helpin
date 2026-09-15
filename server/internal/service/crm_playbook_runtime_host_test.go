package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type playbookLiveTestAuthority struct{ revoked bool }

func (a *playbookLiveTestAuthority) ResolveActor(context.Context, string, string) (*authorization.Actor, error) {
	if a.revoked {
		return nil, ErrCRMPlaybookForbidden
	}
	return f.Actor("admin"), nil
}
func (a *playbookLiveTestAuthority) Can(*authorization.Actor, authorization.Permission) bool {
	return !a.revoked
}
func (a *playbookLiveTestAuthority) CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error) {
	return !a.revoked, nil
}
func (a *playbookLiveTestAuthority) GetMembershipByID(context.Context, string, string) (*model.WorkspaceMember, error) {
	return &model.WorkspaceMember{ID: f.Sales, UserID: f.Ptr(f.SalesUser), Status: model.WorkspaceMemberStatusActive}, nil
}

func boundHostFixture(t *testing.T, index int) (*gorm.DB, *AgentRuntimeHostService, model.AutomationRunBinding, *playbookLiveTestAuthority) {
	t.Helper()
	return boundHostDefinitionFixture(t, index, playbookDefinition(index))
}

func boundHostDefinitionFixture(t *testing.T, index int, definition model.CRMPlaybookDefinition, profiles ...*AIProfileService) (*gorm.DB, *AgentRuntimeHostService, model.AutomationRunBinding, *playbookLiveTestAuthority) {
	t.Helper()
	db, store, pb, work, connection := executionDefinitionFixture(t, index, definition, profiles...)
	ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
	if _, err := store.Configure(ctx, f.Workspace, pb.ID, f.Sales, executionSettings(connection.ID), "enable", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Adopt(ctx, f.Workspace, work.ID, f.Sales, model.CRMPlaybookAutomationAdoption{CommandKey: uuid.NewString(), ExpectedRevision: work.Revision, ConnectionID: connection.ID, Enabled: true, Confirmed: true}, "start", now); err != nil {
		t.Fatal(err)
	}
	event, err := repository.NewAutomationScheduledEventRepository(db).ClaimNext(ctx, now, time.Minute)
	if err != nil || event == nil {
		t.Fatalf("claim: %v", err)
	}
	binding, err := store.ReserveRun(ctx, *event, now, func(source model.CRMPlaybookExecutionSource, id string) (model.AutomationRunBinding, error) {
		prepared, err := prepareCRMPlaybookExecution(source, "helpin")
		if err != nil {
			return model.AutomationRunBinding{}, err
		}
		return model.AutomationRunBinding{AgentID: prepared.helpinAgentID, RuntimeProfileID: prepared.agent.ID, Input: prepared.input}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(binding.Input)
	if err != nil {
		t.Fatal(err)
	}
	run := model.AgentRun{ID: binding.RunID, WorkspaceID: f.Workspace, AgentID: binding.AgentID, TargetType: "crm_company", TargetID: f.Company,
		RuntimeKind: "native_sdk", Status: model.AgentRunStatusRunning, Input: input, ExternalRuntime: f.Ptr(agentRuntimeName), ExternalRuntimeID: f.Ptr("remote-" + binding.RunID)}
	if _, _, err := store.CreateBoundRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	authority := &playbookLiveTestAuthority{}
	execution := NewCRMPlaybookExecutionService(store, nil, nil, authority, authority)
	host := &AgentRuntimeHostService{appID: "helpin", runRepo: repository.NewAgentRunRepository(db), agentRepo: repository.NewAgentRepository(db), playbookExecution: execution,
		commandService: &InternalCommandService{definitions: map[string]InternalCommandDefinition{}}}
	return db, host, *binding, authority
}

func playbookSkillRequest(binding model.AutomationRunBinding) AgentRuntimeSkillLookupRequest {
	return AgentRuntimeSkillLookupRequest{AppID: "helpin", RunID: "remote-" + binding.RunID, AgentID: binding.RuntimeProfileID,
		Target: agentruntime.TargetRef{Type: "crm_company", ID: f.Company}, Key: "crm_record_operations"}
}

func TestCRMPlaybookHostServesExactCapturedSkillsForAllJourneys(t *testing.T) {
	for index, name := range []string{"buying_intent", "sales_handoff", "renewal_recovery"} {
		t.Run(name, func(t *testing.T) {
			_, host, binding, _ := boundHostFixture(t, index)
			ctx := context.Background()
			_, source, err := host.playbookExecution.AuthorizeRun(ctx, f.Workspace, binding.RunID)
			if err != nil {
				t.Fatal(err)
			}
			for _, captured := range source.Connection.Snapshot.Specialization.Skills {
				req := playbookSkillRequest(binding)
				req.Key = captured.Key
				skill, err := host.ResolveActiveSkillByKey(ctx, req)
				if err != nil {
					t.Fatal(err)
				}
				if skill.VersionKey != captured.Version {
					t.Fatal("current catalogue replaced frozen skill")
				}
				want, checksum, err := agentcontract.BuildCRMPlaybookSkillArchive(captured)
				if err != nil {
					t.Fatal(err)
				}
				got, err := host.GetSkillPackageObject(ctx, skill.PackageObjectKey)
				if err != nil || !bytes.Equal(want, got) || checksum != skill.PackageChecksum {
					t.Fatalf("package differs: %v", err)
				}
				req.SkillID, req.Key = skill.ID, ""
				byID, err := host.ResolveSkillByID(ctx, req)
				if err != nil || byID.PackageObjectKey != skill.PackageObjectKey {
					t.Fatalf("captured lookup by ID: %v", err)
				}
			}
		})
	}
}

func TestCRMPlaybookHostCannotDowngradeOrBorrowIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*AgentRuntimeSkillLookupRequest)
	}{
		{"wrong Agent", func(r *AgentRuntimeSkillLookupRequest) { r.AgentID = uuid.NewString() }},
		{"wrong target", func(r *AgentRuntimeSkillLookupRequest) { r.Target.ID = uuid.NewString() }},
		{"foreign workspace", func(r *AgentRuntimeSkillLookupRequest) {
			r.Metadata = map[string]interface{}{"workspace_id": f.ForeignWorkspace}
		}},
		{"wrong connection", func(r *AgentRuntimeSkillLookupRequest) {
			r.Metadata = map[string]interface{}{"crm_playbook_connection_id": uuid.NewString()}
		}},
		{"conflicting early hint", func(r *AgentRuntimeSkillLookupRequest) {
			r.Metadata = map[string]interface{}{"helpin_run_id": uuid.NewString()}
		}},
		{"unsupported skill", func(r *AgentRuntimeSkillLookupRequest) { r.Key = "support_reply" }},
		{"stripped early run identity", func(r *AgentRuntimeSkillLookupRequest) { r.RunID = "unmapped-remote-run"; r.Metadata = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, host, binding, _ := boundHostFixture(t, 0)
			req := playbookSkillRequest(binding)
			test.change(&req)
			if _, err := host.ResolveActiveSkillByKey(context.Background(), req); err == nil {
				t.Fatal("invalid identity obtained skill")
			}
		})
	}
}

func TestCRMPlaybookHostRechecksPausePermissionsAndLateCallbacks(t *testing.T) {
	for _, condition := range []string{"paused", "closed", "revision", "permissions", "terminal", "agent access"} {
		t.Run(condition, func(t *testing.T) {
			db, host, binding, authority := boundHostFixture(t, 0)
			ctx, req := context.Background(), playbookSkillRequest(binding)
			skill, err := host.ResolveActiveSkillByKey(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			switch condition {
			case "paused":
				f.Exec(t, db, "UPDATE crm_situations SET lifecycle = ? WHERE id = ?", condition, binding.SituationID)
			case "closed":
				f.Exec(t, db, "UPDATE crm_situations SET lifecycle = 'closed', outcome_kind = 'achieved', outcome_summary = 'Confirmed by owner', closed_at = ? WHERE id = ?", time.Now().UTC(), binding.SituationID)
			case "revision":
				f.Exec(t, db, "UPDATE crm_situations SET revision = revision + 1 WHERE id = ?", binding.SituationID)
			case "permissions":
				authority.revoked = true
			case "terminal":
				f.Exec(t, db, "UPDATE agent_runs SET status = 'completed' WHERE id = ?", binding.RunID)
			case "agent access":
				f.Exec(t, db, "UPDATE agents SET allowed_tools = ? WHERE id = ?", []byte(`[]`), binding.AgentID)
			}
			if _, err := host.ResolveActiveSkillByKey(ctx, req); !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("continued access: %v", err)
			}
			if _, err := host.GetSkillPackageObject(ctx, skill.PackageObjectKey); !errors.Is(err, ErrAgentRuntimeHostForbidden) {
				t.Fatalf("package remained accessible: %v", err)
			}
		})
	}
}

func TestCRMPlaybookHostAllowsOnlyBoundCommands(t *testing.T) {
	_, host, binding, _ := boundHostFixture(t, 0)
	calls := 0
	host.commandService.definitions["crm.get_playbook_context"] = InternalCommandDefinition{Name: "crm.get_playbook_context", Module: "crm", Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
		calls++
		if meta.RunID != binding.RunID || meta.AgentID != binding.AgentID || meta.ActorID != f.SalesUser {
			t.Fatal("callback identity was not replaced with trusted host scope")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}
	req := agentruntime.CommandExecutionRequest{Meta: agentruntime.CommandExecutionContext{AppID: "helpin", RunID: "remote-" + binding.RunID, AgentID: binding.RuntimeProfileID, Target: agentruntime.TargetRef{Type: "crm_company", ID: f.Company}}, CommandName: "crm.get_playbook_context"}
	if result, err := host.ExecuteCommand(context.Background(), req); err != nil || result.Error != "" || calls != 1 {
		t.Fatalf("bound command: %v %#v", err, result)
	}
	for _, command := range []string{"crm.update_company", "pm.create_task", "crm.get_company"} {
		req.CommandName = command
		if _, err := host.ExecuteCommand(context.Background(), req); !errors.Is(err, ErrAgentRuntimeHostForbidden) {
			t.Fatalf("unguarded command %s: %v", command, err)
		}
	}
	if calls != 1 {
		t.Fatal("denied command reached executor")
	}
}
