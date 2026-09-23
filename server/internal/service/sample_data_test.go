package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// fakeSampleSeeder inserts minimal rows so the orchestration (module gating,
// tracking, idempotency, rollback, removal) can be tested on SQLite. The real
// module seeders are exercised against PostgreSQL in sample_data_postgres_test.go.
type fakeSampleSeeder struct {
	module model.ModuleID
	fail   bool
	calls  int
}

func (f *fakeSampleSeeder) Module() model.ModuleID { return f.module }

func (f *fakeSampleSeeder) Seed(ctx context.Context, env *SampleDataEnv) error {
	f.calls++
	switch f.module {
	case model.ModulePM:
		id := uuid.NewString()
		if err := env.Tx.Exec(`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id) VALUES (?, ?, 1, 'Sample task', 'wf', 'state')`, id, env.WorkspaceID).Error; err != nil {
			return err
		}
		env.Track(model.SampleEntityPMTask, id)
	case model.ModuleCRM:
		id := uuid.NewString()
		if err := env.Tx.Exec(`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name) VALUES (?, ?, 1, 'Maya')`, id, env.WorkspaceID).Error; err != nil {
			return err
		}
		env.Track(model.SampleEntityCRMContact, id)
	}
	if f.fail {
		return errors.New("seed failed")
	}
	return nil
}

// serviceSampleDataItemsSchema mirrors migration 202609220041; newTestDB creates it.
const serviceSampleDataItemsSchema = `CREATE TABLE sample_data_items (
	id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	workspace_id TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	entity_id TEXT NOT NULL UNIQUE,
	created_by TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`

func setupSampleDataService(t *testing.T, seeders ...SampleDataSeeder) (*SampleDataService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	seedUser(t, db, "user-1", "owner@example.com", "Olivia Owner", "hash")
	seedWorkspace(t, db, "ws-1", "Northwind", "northwind", "user-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "user-1", "owner@example.com", "Olivia Owner", "owner")
	return NewSampleDataServiceWithSeeders(db, seeders...), db
}

func countRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

var allSampleModules = map[model.ModuleID]bool{model.ModulePM: true, model.ModuleCRM: true, model.ModuleDocs: true, model.ModuleSupport: true}

func TestSampleDataLoadSeedsEnabledModulesOnly(t *testing.T) {
	pm := &fakeSampleSeeder{module: model.ModulePM}
	crm := &fakeSampleSeeder{module: model.ModuleCRM}
	svc, db := setupSampleDataService(t, pm, crm)

	status, err := svc.Load(context.Background(), "ws-1", "user-1", map[model.ModuleID]bool{model.ModulePM: true})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if pm.calls != 1 || crm.calls != 0 {
		t.Fatalf("seeder calls pm=%d crm=%d, want only enabled module seeded", pm.calls, crm.calls)
	}
	if !status.Loaded || status.LoadedAt == nil || status.Counts[model.SampleEntityPMTask] != 1 || len(status.Counts) != 1 {
		t.Fatalf("status = %+v", status)
	}
	if len(status.Modules) != 1 || status.Modules[0] != model.ModulePM {
		t.Fatalf("modules = %v", status.Modules)
	}
	if got := countRows(t, db, "crm_contacts"); got != 0 {
		t.Fatalf("crm contacts = %d, want 0 for a disabled module", got)
	}
}

func TestSampleDataLoadIsNotRepeated(t *testing.T) {
	pm := &fakeSampleSeeder{module: model.ModulePM}
	svc, db := setupSampleDataService(t, pm)
	ctx := context.Background()
	if _, err := svc.Load(ctx, "ws-1", "user-1", allSampleModules); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if _, err := svc.Load(ctx, "ws-1", "user-1", allSampleModules); !errors.Is(err, model.ErrSampleDataAlreadyLoaded) {
		t.Fatalf("second load err = %v, want ErrSampleDataAlreadyLoaded", err)
	}
	if got := countRows(t, db, "pm_tasks"); got != 1 {
		t.Fatalf("tasks = %d, want the first load only", got)
	}
}

func TestSampleDataLoadRollsBackOnSeederFailure(t *testing.T) {
	pm := &fakeSampleSeeder{module: model.ModulePM}
	crm := &fakeSampleSeeder{module: model.ModuleCRM, fail: true}
	svc, db := setupSampleDataService(t, pm, crm)
	if _, err := svc.Load(context.Background(), "ws-1", "user-1", allSampleModules); err == nil {
		t.Fatal("expected load to fail")
	}
	for _, table := range []string{"pm_tasks", "crm_contacts", "sample_data_items"} {
		if got := countRows(t, db, table); got != 0 {
			t.Errorf("%s rows = %d after a failed load, want rollback", table, got)
		}
	}
}

func TestSampleDataLoadRequiresASeedableModule(t *testing.T) {
	svc, _ := setupSampleDataService(t, &fakeSampleSeeder{module: model.ModulePM})
	_, err := svc.Load(context.Background(), "ws-1", "user-1", map[model.ModuleID]bool{model.ModuleAutomation: true})
	if !errors.Is(err, model.ErrSampleDataNoModules) {
		t.Fatalf("err = %v, want ErrSampleDataNoModules", err)
	}
}

func TestSampleDataLoadRequiresMembership(t *testing.T) {
	svc, db := setupSampleDataService(t, &fakeSampleSeeder{module: model.ModulePM})
	_, err := svc.Load(context.Background(), "ws-1", "stranger", allSampleModules)
	var forbidden *model.ErrForbidden
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if got := countRows(t, db, "pm_tasks"); got != 0 {
		t.Fatalf("tasks = %d", got)
	}
}

func TestSampleDataRemoveDeletesTrackedRecordsOnly(t *testing.T) {
	svc, db := setupSampleDataService(t, &fakeSampleSeeder{module: model.ModulePM}, &fakeSampleSeeder{module: model.ModuleCRM})
	ctx := context.Background()
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id) VALUES ('real-task', 'ws-1', 2, 'Real task', 'wf', 'state')`)
	if _, err := svc.Load(ctx, "ws-1", "user-1", allSampleModules); err != nil {
		t.Fatalf("load: %v", err)
	}
	status, err := svc.Remove(ctx, "ws-1", "user-1", allSampleModules)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if status.Loaded || len(status.Counts) != 0 {
		t.Fatalf("status after remove = %+v", status)
	}
	if got := countRows(t, db, "crm_contacts"); got != 0 {
		t.Errorf("crm contacts = %d, want sample contact removed", got)
	}
	if got := countRows(t, db, "sample_data_items"); got != 0 {
		t.Errorf("tracking rows = %d, want 0", got)
	}
	var remaining []string
	if err := db.Table("pm_tasks").Pluck("id", &remaining).Error; err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0] != "real-task" {
		t.Fatalf("remaining tasks = %v, want only the real task", remaining)
	}
	reloaded, err := svc.Load(ctx, "ws-1", "user-1", allSampleModules)
	if err != nil || !reloaded.Loaded {
		t.Fatalf("reload after removal: status=%+v err=%v", reloaded, err)
	}
}

func TestSampleDataStatusBeforeLoad(t *testing.T) {
	svc, _ := setupSampleDataService(t, &fakeSampleSeeder{module: model.ModulePM}, &fakeSampleSeeder{module: model.ModuleCRM})
	status, err := svc.Status(context.Background(), "ws-1", map[model.ModuleID]bool{model.ModuleCRM: true})
	if err != nil {
		t.Fatal(err)
	}
	if status.Loaded || status.LoadedAt != nil || len(status.Counts) != 0 {
		t.Fatalf("status = %+v", status)
	}
	if fmt.Sprint(status.Modules) != "[crm]" {
		t.Fatalf("modules = %v", status.Modules)
	}
}

func TestDefaultSampleDataSeedersCoverEveryContentModule(t *testing.T) {
	want := []model.ModuleID{model.ModuleCRM, model.ModulePM, model.ModuleDocs, model.ModuleSupport}
	seeders := DefaultSampleDataSeeders(false)
	if len(seeders) != len(want) {
		t.Fatalf("seeders = %d, want %d", len(seeders), len(want))
	}
	for i, seeder := range seeders {
		if seeder.Module() != want[i] {
			t.Fatalf("seeder %d module = %s, want %s (CRM and PM must run before support)", i, seeder.Module(), want[i])
		}
	}
}

func TestSampleFixturesUseReservedExampleDomains(t *testing.T) {
	for _, contact := range sampleContacts {
		if !hasExampleDomain(contact.email) {
			t.Errorf("contact email %q is not on example.com", contact.email)
		}
	}
	for _, conversation := range sampleConversations {
		if !hasExampleDomain(conversation.customerEmail) {
			t.Errorf("conversation email %q is not on example.com", conversation.customerEmail)
		}
	}
	for _, company := range sampleCompanies {
		if !hasExampleDomain("x@" + company.domain) {
			t.Errorf("company domain %q is not under example.com", company.domain)
		}
	}
}

func hasExampleDomain(email string) bool {
	for i := len(email) - 1; i >= 0; i-- {
		if email[i] == '@' {
			domain := email[i+1:]
			return domain == "example.com" || len(domain) > len(".example.com") && domain[len(domain)-len(".example.com"):] == ".example.com"
		}
	}
	return false
}
