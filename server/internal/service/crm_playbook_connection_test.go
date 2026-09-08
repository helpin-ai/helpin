package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookConnectionPublication(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(playbookDefinition(i).Journey, func(t *testing.T) {
			db, svc, ctx := playbookFixture(t)
			flow, agent := f.PlaybookConnectionStorage(t, db)
			pb := readyPlaybook(t, svc, ctx, i)
			selection := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
			review, err := svc.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
			if err != nil {
				t.Fatal(err)
			}
			if review.ExecutionEnabled || len(review.Skills) != 2 {
				t.Fatalf("invalid review: %#v", review)
			}
			req := model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: selection.FlowID, AgentID: selection.AgentID, CommandKey: uuid.NewString(), ReviewFingerprint: review.ReviewFingerprint}
			result, err := svc.PublishConnection(ctx, f.Workspace, pb.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			conn := result.Connection
			if conn.ExecutionEnabled || conn.Version != 1 || result.Replayed {
				t.Fatalf("invalid receipt: %#v", result)
			}
			stored, err := repository.NewCRMPlaybookRepository(db).Connection(ctx, f.Workspace, pb.ID, conn.ID)
			if err != nil || stored == nil {
				t.Fatalf("not persisted: %v", err)
			}
			if stored.Snapshot.Specialization.PresetPrompt != "Workspace-reviewed Beacon instructions" || stored.Snapshot.Agent.MonthlyTokenBudget == nil || *stored.Snapshot.Agent.MonthlyTokenBudget != 1000 {
				t.Fatal("effective instructions or host limits missing")
			}
			var runtime AgentRuntimeAgent
			if err := json.Unmarshal(stored.Snapshot.RuntimeAgent, &runtime); err != nil {
				t.Fatal(err)
			}
			if runtime.SystemPrompt != stored.Snapshot.Specialization.PresetPrompt || runtime.ID != agent {
				t.Fatal("runtime projection differs")
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "Workspace-reviewed") || strings.Contains(string(encoded), "runtime_agent") || strings.Contains(string(encoded), "command_fingerprint") {
				t.Fatal("private snapshot leaked")
			}
			f.Exec(t, db, "UPDATE agents SET system_prompt = 'Unreviewed edit' WHERE id = ?", agent)
			pb.Draft.Name = "Later draft"
			mustPlaybookCommand(t, svc, ctx, pb, "update_draft", &pb.Draft, nil)
			retry, err := svc.PublishConnection(ctx, f.Workspace, pb.ID, req)
			if err != nil || !retry.Replayed || retry.Connection.ID != conn.ID || retry.Connection.Fingerprint != conn.Fingerprint || !retry.Connection.PublishedAt.Equal(conn.PublishedAt) {
				t.Fatalf("retry lost original receipt: %v", err)
			}
			req.ReviewFingerprint = strings.Repeat("0", 64)
			if _, err := svc.PublishConnection(ctx, f.Workspace, pb.ID, req); !errors.Is(err, repository.ErrCRMPlaybookConflict) {
				t.Fatalf("reused key: %v", err)
			}
			var enabled bool
			if err := db.Table("automation_rules").Select("enabled").Where("id = ?", flow).Scan(&enabled).Error; err != nil || enabled {
				t.Fatal("Flow was enabled")
			}
			var count int64
			if err := db.Model(&model.CRMSituation{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("publication enrolled customers")
			}
			if err := db.Model(&model.CRMPlaybookConnection{}).Where("id = ?", conn.ID).Update("execution_enabled", true).Error; err == nil {
				t.Fatal("database allowed activation")
			}
		})
	}
}

func TestCRMPlaybookConnectionConcurrentPublicationAndHistory(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	flow, agent := f.PlaybookConnectionStorage(t, db)
	pb := readyPlaybook(t, svc, ctx, 0)
	selection := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
	review, err := svc.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
	if err != nil {
		t.Fatal(err)
	}
	const attempts = 6
	results := make([]*model.CRMPlaybookConnectionResult, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.PublishConnection(ctx, f.Workspace, pb.ID, model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: selection.FlowID, AgentID: selection.AgentID, CommandKey: uuid.NewString(), ReviewFingerprint: review.ReviewFingerprint})
		}(i)
	}
	wg.Wait()
	winners := 0
	for i, err := range errs {
		if err == nil {
			winners++
			if results[i] == nil {
				t.Fatal("empty receipt")
			}
		} else if !errors.Is(err, repository.ErrCRMPlaybookStale) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent publications: %d", winners)
	}
	// Live usage is not editable configuration and must not force a new review.
	review, err = svc.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE agents SET tokens_used_this_month = 999, status = 'running' WHERE id = ?", agent)
	second, err := svc.PublishConnection(ctx, f.Workspace, pb.ID, model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: selection.FlowID, AgentID: selection.AgentID, CommandKey: uuid.NewString(), ExpectedConnectionVersion: review.ConnectionVersion, ReviewFingerprint: review.ReviewFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	if second.Connection.Version != 2 {
		t.Fatal("missing publication version")
	}
	history, err := svc.Connections(ctx, f.Workspace, pb.ID, 0, 1)
	if err != nil || len(history.Data) != 1 || history.Data[0].Version != 2 || history.NextBeforeVersion == nil {
		t.Fatalf("history: %#v %v", history, err)
	}
	older, err := svc.Connections(ctx, f.Workspace, pb.ID, *history.NextBeforeVersion, 1)
	if err != nil || len(older.Data) != 1 || older.Data[0].Version != 1 || older.NextBeforeVersion != nil {
		t.Fatalf("older history: %#v %v", older, err)
	}
}

func TestCRMPlaybookConnectionFingerprintSurvivesJSONBOrdering(t *testing.T) {
	first, err := connectionContentFingerprint(json.RawMessage(`{"z":{"limit":9007199254740993,"steps":12},"a":true}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := connectionContentFingerprint(json.RawMessage(`{"a": true, "z": {"steps":12,"limit":9007199254740993}}`))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("JSON object order changed review identity")
	}
	changed, err := connectionContentFingerprint(json.RawMessage(`{"a":true,"z":{"steps":12,"limit":9007199254740992}}`))
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("configuration precision was lost")
	}
}

type connectionDeniedAuthorizer struct {
	*authorization.AuthzService
	permission authorization.Permission
	module     model.ModuleID
}

func (a connectionDeniedAuthorizer) Can(actor *authorization.Actor, p authorization.Permission) bool {
	return p != a.permission && a.AuthzService.Can(actor, p)
}

func (a connectionDeniedAuthorizer) CanAccessModule(ctx context.Context, actor *authorization.Actor, m model.ModuleID) (bool, error) {
	if m == a.module {
		return false, nil
	}
	return a.AuthzService.CanAccessModule(ctx, actor, m)
}

func TestCRMPlaybookConnectionRequiresBothModulesAndPermissions(t *testing.T) {
	for _, denied := range []connectionDeniedAuthorizer{
		{permission: authorization.PermCRMAdmin}, {permission: authorization.PermPMAdminAutomations},
		{module: model.ModuleCRM}, {module: model.ModuleAutomation},
	} {
		db, svc, ctx := playbookFixture(t)
		denied.AuthzService = authorization.NewAuthzService(db, nil, nil)
		svc.authz = denied
		if _, err := svc.ReviewConnection(ctx, f.Workspace, uuid.NewString(), model.CRMPlaybookConnectionSelection{}); !errors.Is(err, ErrCRMPlaybookForbidden) {
			t.Fatalf("review bypassed auth: %v", err)
		}
		if _, err := svc.PublishConnection(ctx, f.Workspace, uuid.NewString(), model.PublishCRMPlaybookConnectionRequest{}); !errors.Is(err, ErrCRMPlaybookForbidden) {
			t.Fatalf("publish bypassed auth: %v", err)
		}
	}
}

func TestCRMPlaybookConnectionRejectsDriftAndUnauthorizedSelection(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		args      []any
	}{
		{"prompt", "UPDATE agents SET system_prompt = 'Changed'", nil},
		{"tools", "UPDATE agents SET allowed_tools = ?", []any{[]byte(`[]`)}},
		{"budget", "UPDATE agents SET monthly_token_budget = 2", nil},
		{"team", "INSERT INTO agent_team_access (agent_id,team_id) SELECT id,'c0000000-0000-4000-8000-000000000001' FROM agents", nil},
		{"flow", "UPDATE automation_rules SET trigger_config = ?", []any{[]byte(`{"preset":"hourly"}`)}},
		{"enabled", "UPDATE automation_rules SET enabled = true", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, svc, ctx := playbookFixture(t)
			flow, agent := f.PlaybookConnectionStorage(t, db)
			pb := readyPlaybook(t, svc, ctx, 0)
			s := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
			review, err := svc.ReviewConnection(ctx, f.Workspace, pb.ID, s)
			if err != nil {
				t.Fatal(err)
			}
			f.Exec(t, db, tc.sql, tc.args...)
			_, err = svc.PublishConnection(ctx, f.Workspace, pb.ID, model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: s.PlaybookVersionID, ExpectedRevision: s.ExpectedRevision, FlowID: s.FlowID, AgentID: s.AgentID, CommandKey: uuid.NewString(), ReviewFingerprint: review.ReviewFingerprint})
			if !errors.Is(err, repository.ErrCRMPlaybookStale) && !errors.Is(err, ErrCRMPlaybookConnectionUnsupported) {
				t.Fatalf("unreviewed configuration result: %v", err)
			}
			var count int64
			if err := db.Model(&model.CRMPlaybookConnection{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("failed publish left partial state")
			}
		})
	}
	db, svc, ctx := playbookFixture(t)
	flow, agent := f.PlaybookConnectionStorage(t, db)
	pb := readyPlaybook(t, svc, ctx, 0)
	s := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
	for _, role := range []string{"member", "viewer"} {
		actorCtx := authorization.WithActor(context.Background(), f.Actor(role))
		if _, err := svc.ReviewConnection(actorCtx, f.Workspace, pb.ID, s); !errors.Is(err, ErrCRMPlaybookForbidden) {
			t.Fatalf("%s could review settings: %v", role, err)
		}
	}
	f.Exec(t, db, "UPDATE agents SET workspace_id = ?", f.ForeignWorkspace)
	if _, err := svc.ReviewConnection(ctx, f.Workspace, pb.ID, s); !errors.Is(err, ErrCRMPlaybookNotFound) {
		t.Fatalf("foreign Agent: %v", err)
	}
}
