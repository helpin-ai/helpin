package service

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type pmTriageTestProvider struct {
	err    error
	calls  int
	states []string
	during func()
}

func TestPMTriageDecisionMetrics(t *testing.T) {
	svc, _, _, _ := setupPMTriageService(t)
	metrics := observability.NewMetrics()
	svc.SetMetrics(metrics)
	for range 2 {
		if _, err := svc.Analyze(pmTriageMemberContext(), "workspace", "task", "source"); err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for _, outcome := range []string{"ready", "cached"} {
		want := `helpin_ai_decisions_total{operation="pm_triage",outcome="` + outcome + `"} 1`
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("missing metric %s", want)
		}
	}
}

func TestPMTriageMetricsReportFailedShadowCooldown(t *testing.T) {
	svc, provider, _, _ := setupPMTriageService(t)
	svc.config.Mode = "shadow"
	provider.err = errors.New("provider unavailable")
	metrics := observability.NewMetrics()
	svc.SetMetrics(metrics)
	if _, err := svc.Analyze(pmTriageMemberContext(), "workspace", "task", "source"); err == nil {
		t.Fatal("expected provider error")
	}
	if _, err := svc.Analyze(pmTriageMemberContext(), "workspace", "task", "source"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for _, outcome := range []string{"error", "failed"} {
		want := `helpin_ai_decisions_total{operation="pm_triage",outcome="` + outcome + `"} 1`
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("missing metric %s", want)
		}
	}
}

func (p *pmTriageTestProvider) DecideMany(_ context.Context, state string, questions map[string]decision.Question) (*decision.Result, error) {
	p.calls++
	p.states = append(p.states, state)
	if p.err != nil {
		return nil, p.err
	}
	if p.during != nil {
		p.during()
	}
	result := &decision.Result{Model: decision.Model, InputTokens: 100, OutputTokens: 10, Answers: map[string]decision.Answer{}}
	for key, q := range questions {
		selected := "yes"
		switch key {
		case "task_type":
			selected = "bug"
		case "team":
			selected = "mine"
		}
		if strings.HasPrefix(key, "candidate_") {
			selected = "duplicates"
		}
		probabilities := map[string]float64{}
		for id := range q.Choices {
			probabilities[id] = .01 / float64(len(q.Choices)-1)
		}
		probabilities[selected] = .99
		result.Answers[key] = decision.Answer{Choice: selected, Probabilities: probabilities}
	}
	return result, nil
}

type pmTriageTestUsage struct {
	entries []model.AIExecutionUsage
	err     error
}

func (u *pmTriageTestUsage) RecordExecutionUsage(_ context.Context, entry model.AIExecutionUsage, _ model.JSONBlob) error {
	u.entries = append(u.entries, entry)
	if u.err != nil {
		return u.err
	}
	return nil
}

func setupPMTriageService(t *testing.T) (*PMTriageService, *pmTriageTestProvider, *pmTriageTestUsage, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pm_triage_service_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces (id) VALUES ('workspace')`,
		`CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT, team_id TEXT, display_id INTEGER, name TEXT, description TEXT, updated_at DATETIME, archived BOOLEAN)`,
		`INSERT INTO pm_tasks VALUES ('source','workspace','mine',1,'CSV export crash','Invoice export fails','2026-09-17 10:00:00',false), ('match','workspace','mine',2,'CSV export fails','Invoice export fails','2026-09-17 10:00:00',false), ('secret','workspace','other',3,'SECRET CSV export','Private','2026-09-17 10:00:00',false)`,
		`CREATE TABLE workspace_teams (id TEXT PRIMARY KEY,workspace_id TEXT,name TEXT,description TEXT)`,
		`INSERT INTO workspace_teams VALUES ('mine','workspace','Payments','Invoice exports'), ('other','workspace','SECRET TEAM','Private')`,
		`CREATE TABLE pm_labels (id TEXT PRIMARY KEY,workspace_id TEXT,team_id TEXT,name TEXT,description TEXT,archived BOOLEAN)`,
		`INSERT INTO pm_labels VALUES ('export','workspace','mine','Export','Export functionality',false), ('secret','workspace','other','SECRET LABEL','Private',false)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&model.PMTriageAssessment{}, &model.PMTriageLabelSuppression{}, &model.PMTaskLabel{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE pm_activity_log (id TEXT PRIMARY KEY,workspace_id TEXT,entity_type TEXT,entity_id TEXT,actor_id TEXT,event_type TEXT,action TEXT,field_name TEXT,old_value TEXT,new_value TEXT,metadata TEXT,created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	provider := &pmTriageTestProvider{}
	usage := &pmTriageTestUsage{}
	service, err := NewPMTriageService(PMTriageConfig{Mode: "primary", Threshold: .95, DailyLimit: 10}, provider, repository.NewPMTriageRepository(db), usage, repository.NewPMTaskRepository(db), repository.NewWorkspaceRepository(db), repository.NewPMLabelRepository(db), &SupportInboxService{})
	if err != nil {
		t.Fatal(err)
	}
	return service, provider, usage, db
}
func pmTriageMemberContext() context.Context {
	return authorization.WithActor(context.Background(), &authorization.Actor{UserID: "actor", WorkspaceID: "workspace", Role: model.RoleMember, TeamMemberships: []authorization.TeamRole{{TeamID: "mine", Role: "member"}}})
}

func TestPMTriageTaskUsesAuthorizedEvidence(t *testing.T) {
	service, provider, usage, _ := setupPMTriageService(t)
	view, err := service.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "ready" || view.Assessment == nil || len(view.Assessment.Matches) != 1 || view.Assessment.Matches[0].TaskID != "match" {
		t.Fatalf("unexpected result: %+v", view)
	}
	if provider.calls != 1 || strings.Contains(provider.states[0], "SECRET") {
		t.Fatalf("unauthorized context sent: calls=%d", provider.calls)
	}
	if len(usage.entries) != 1 || usage.entries[0].FeatureKey != "pm_triage" || usage.entries[0].InputTokens != 100 {
		t.Fatalf("missing usage: %+v", usage.entries)
	}
	cached, err := service.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	if cached.ID != view.ID || cached.Assessment == nil || provider.calls != 1 || len(usage.entries) != 1 {
		t.Fatal("cached assessment caused another provider call or lost suggestions")
	}
}
func TestPMTriageRejectsUnauthorizedSource(t *testing.T) {
	service, provider, _, _ := setupPMTriageService(t)
	for _, tc := range []struct {
		name              string
		ctx               context.Context
		workspace, source string
	}{
		{"no actor", context.Background(), "workspace", "source"},
		{"wrong workspace", pmTriageMemberContext(), "other", "source"},
		{"other team", pmTriageMemberContext(), "workspace", "secret"},
		{"missing source", pmTriageMemberContext(), "workspace", "missing"},
		{"viewer", authorization.WithActor(context.Background(), &authorization.Actor{UserID: "viewer", WorkspaceID: "workspace", Role: model.RoleViewer}), "workspace", "source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := service.Analyze(tc.ctx, tc.workspace, "task", tc.source); err == nil {
				t.Fatal("unauthorized source accepted")
			}
		})
	}
	if provider.calls != 0 {
		t.Fatal("unauthorized source reached provider")
	}
}
func TestPMTriageModes(t *testing.T) {
	for _, mode := range []string{"off", "shadow"} {
		t.Run(mode, func(t *testing.T) {
			service, provider, _, _ := setupPMTriageService(t)
			service.config.Mode = mode
			view, err := service.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
			if err != nil {
				t.Fatal(err)
			}
			if view.Assessment != nil {
				t.Fatal("non-primary mode exposed actionable suggestions")
			}
			if mode == "off" && (view.Status != "disabled" || provider.calls != 0) {
				t.Fatal("off mode invoked provider")
			}
			if mode == "shadow" && (view.Status != "shadow" || provider.calls != 1) {
				t.Fatal("shadow did not evaluate")
			}
		})
	}
}
func TestPMTriagePermissionChangeInvalidatesCandidates(t *testing.T) {
	service, provider, _, db := setupPMTriageService(t)
	if _, err := service.Analyze(pmTriageMemberContext(), "workspace", "task", "source"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE pm_tasks SET team_id = ? WHERE id = ?", "other", "match").Error; err != nil {
		t.Fatal(err)
	}
	view, err := service.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Candidates) != 0 || len(view.Assessment.Matches) != 0 || provider.calls != 2 {
		t.Fatal("reused inaccessible cached candidates")
	}
}

func TestPMTriageAutomaticLabelsRespectManualRemoval(t *testing.T) {
	for _, removal := range []string{"endpoint", "form"} {
		t.Run(removal, func(t *testing.T) {
			service, provider, _, db := setupPMTriageService(t)
			ctx := pmTriageMemberContext()
			added, err := service.automaticTask(ctx, "workspace", "source")
			if err != nil {
				t.Fatal(err)
			}
			if len(added) != 1 || added[0] != "export" {
				t.Fatalf("label not applied: %v", added)
			}
			var events int64
			if err := db.Model(&model.PMActivityLog{}).Where("action = ?", "label_added").Count(&events).Error; err != nil {
				t.Fatal(err)
			}
			if events != 1 {
				t.Fatal("automatic label missing audit")
			}
			switch removal {
			case "endpoint":
				err = service.tasks.RemoveLabelWithTriageSuppression(ctx, "workspace", "source", "export")
			case "form":
				err = service.tasks.ReplaceLabelsWithTriageSuppression(ctx, "workspace", "source", []string{})
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("UPDATE pm_tasks SET name = ?, updated_at = ? WHERE id = ?", "CSV export crash persists", time.Now().UTC(), "source").Error; err != nil {
				t.Fatal(err)
			}
			added, err = service.automaticTask(ctx, "workspace", "source")
			if err != nil {
				t.Fatal(err)
			}
			if len(added) != 0 || provider.calls != 2 {
				t.Fatal("automatic triage overrode manual removal")
			}
			var count int64
			if err := db.Model(&model.PMTaskLabel{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("removed label reappeared")
			}
		})
	}
}
func TestPMTriageAutomaticLabelsRejectEditedSource(t *testing.T) {
	service, provider, _, db := setupPMTriageService(t)
	provider.during = func() {
		if err := db.Exec("UPDATE pm_tasks SET name = ?, updated_at = ? WHERE id = ?", "Actually fix login", time.Now().UTC(), "source").Error; err != nil {
			t.Fatal(err)
		}
	}
	added, err := service.automaticTask(pmTriageMemberContext(), "workspace", "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 {
		t.Fatal("labels applied from stale source evidence")
	}
}
func TestPMTriageAutomaticLabelsAreIdempotent(t *testing.T) {
	service, provider, _, db := setupPMTriageService(t)
	if _, err := service.automaticTask(pmTriageMemberContext(), "workspace", "source"); err != nil {
		t.Fatal(err)
	}
	added, err := service.automaticTask(pmTriageMemberContext(), "workspace", "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 || provider.calls != 1 {
		t.Fatal("repeated triage duplicated work")
	}
	var count int64
	if err := db.Model(&model.PMActivityLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("repeated triage duplicated activity")
	}
}

func TestPMTriageFailedProviderOrUsageDoesNotExposeSuggestions(t *testing.T) {
	for _, failure := range []string{"provider", "usage"} {
		t.Run(failure, func(t *testing.T) {
			triage, provider, usage, db := setupPMTriageService(t)
			if failure == "provider" {
				provider.err = errors.New("provider unavailable")
			} else {
				usage.err = errors.New("usage unavailable")
			}
			view, err := triage.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
			if err == nil || view != nil {
				t.Fatal("failed evaluation exposed actionable suggestions")
			}
			var attempts []model.PMTriageAssessment
			if err := db.Find(&attempts).Error; err != nil {
				t.Fatal(err)
			}
			if len(attempts) != 1 || attempts[0].Status != "failed" {
				t.Fatal("failed attempt was not finalized")
			}
		})
	}
}
func TestPMTriageDraftDoesNotCreateTask(t *testing.T) {
	triage, provider, _, db := setupPMTriageService(t)
	team := "mine"
	view, err := triage.AnalyzeDraft(pmTriageMemberContext(), "workspace", model.PMTriageDraftRequest{DraftID: "draft", Name: "CSV export fails", Description: "Invoice CSV export crashes", TeamID: &team})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "ready" || view.SourceKind != "task_draft" || len(view.Assessment.Matches) == 0 || provider.calls != 1 {
		t.Fatalf("missing draft suggestions %+v", view)
	}
	var count int64
	if err := db.Model(&model.PMTask{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("preview changed task count: %d", count)
	}
}
func TestPMTriageProviderFailureDoesNotFailTaskCreation(t *testing.T) {
	env := newTaskTestEnv(t)
	if err := env.db.AutoMigrate(&model.PMTriageAssessment{}); err != nil {
		t.Fatal(err)
	}
	triage, err := NewPMTriageService(PMTriageConfig{Mode: "primary", Threshold: .95, DailyLimit: 10}, &pmTriageTestProvider{err: errors.New("provider down")}, repository.NewPMTriageRepository(env.db), &pmTriageTestUsage{}, env.svc.taskRepo, env.svc.workspaceRepo, env.svc.labelRepo, &SupportInboxService{})
	if err != nil {
		t.Fatal(err)
	}
	env.svc.SetTriageService(triage)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{UserID: env.userID, WorkspaceID: env.wsID, Role: model.RoleAdmin})
	created, err := env.svc.Create(ctx, model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Fix CSV export", TaskType: "bug", TeamID: &env.teamID, WorkflowID: env.wfID, WorkflowStateID: env.stTodo}, env.userID)
	if err != nil || created == nil {
		t.Fatalf("provider failure prevented task creation: %v", err)
	}
	var attempts []model.PMTriageAssessment
	if err := env.db.Find(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Status != "failed" {
		t.Fatalf("missing failed provider audit: %+v", attempts)
	}
}

func TestPMTriageBackfillsBoundedCandidateContext(t *testing.T) {
	triage, provider, _, db := setupPMTriageService(t)
	for i := 0; i < 25; i++ {
		description := "Invoice export fails"
		if i < 12 {
			description = strings.Repeat("Invoice export fails ", 1000)
		}
		if err := db.Exec("INSERT INTO pm_tasks(id,workspace_id,team_id,display_id,name,description,updated_at,archived) VALUES(?,?,?,?,?,?,?,false)", fmt.Sprintf("candidate-%02d", i), "workspace", "mine", i+10, "CSV export crash", description, time.Now()).Error; err != nil {
			t.Fatal(err)
		}
	}
	view, err := triage.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Candidates) != 10 || view.Assessment.CandidatesChecked != 10 {
		t.Fatalf("expected ten fitting candidates: %+v", view)
	}
	if provider.calls != 1 || len(provider.states[0]) > 16000 {
		t.Fatal("provider calls or context grew beyond bounds")
	}
	for _, candidate := range view.Candidates {
		for i := 0; i < 12; i++ {
			if candidate.ID == fmt.Sprintf("candidate-%02d", i) {
				t.Fatal("oversized candidate included")
			}
		}
	}
}
