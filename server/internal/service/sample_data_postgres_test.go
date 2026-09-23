//go:build integration

package service

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Runs the real module seeders against a migrated PostgreSQL database:
//
//	SAMPLE_DATA_TEST_DATABASE_URL=postgres://... go test -tags integration -run SampleDataPostgres ./internal/service/
//
// Point it only at a disposable database; it creates and deletes workspaces.
func openSampleDataPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("SAMPLE_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAMPLE_DATA_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return db
}

type sampleWorkspaceFixture struct {
	workspaceID, userID, memberID string
}

func createSampleWorkspace(t *testing.T, db *gorm.DB) sampleWorkspaceFixture {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	user := model.User{Email: "owner-" + suffix + "@example.com", PasswordHash: "x", FullName: "Olivia Owner"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	letters := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	key := make([]byte, 5)
	for i := range key {
		key[i] = letters[rand.Intn(len(letters))]
	}
	workspace := model.Workspace{Name: "Sample " + suffix, Slug: "sample-" + suffix, WorkspaceKey: string(key), OwnerID: user.ID, Timezone: "UTC"}
	if err := db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	member := model.WorkspaceMember{WorkspaceID: workspace.ID, UserID: &user.ID, Email: user.Email, DisplayName: user.FullName, Role: model.RoleOwner, Status: model.WorkspaceMemberStatusActive}
	if err := db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	return sampleWorkspaceFixture{workspaceID: workspace.ID, userID: user.ID, memberID: member.ID}
}

func workspaceRowCount(t *testing.T, db *gorm.DB, table, workspaceID string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func TestSampleDataPostgresLoadAndRemove(t *testing.T) {
	db := openSampleDataPostgres(t)
	ctx := context.Background()
	fixture := createSampleWorkspace(t, db)
	svc := NewSampleDataService(db, true)

	status, err := svc.Load(ctx, fixture.workspaceID, fixture.userID, allSampleModules)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := map[string]int64{
		model.SampleEntityCRMCompany: 3, model.SampleEntityCRMContact: 5, model.SampleEntityCRMDeal: 2,
		model.SampleEntityCRMPipeline: 1, model.SampleEntityWorkspaceTeam: 1, model.SampleEntityPMEpic: 1,
		model.SampleEntityPMTask: 6, model.SampleEntityDocsSpace: 1, model.SampleEntityDocsDocument: 3,
		model.SampleEntitySupportConversation: 4, model.SampleEntityAutomationRule: 2,
	}
	for entity, count := range want {
		if status.Counts[entity] != count {
			t.Errorf("count[%s] = %d, want %d", entity, status.Counts[entity], count)
		}
	}
	if _, err := svc.Load(ctx, fixture.workspaceID, fixture.userID, allSampleModules); !errors.Is(err, model.ErrSampleDataAlreadyLoaded) {
		t.Fatalf("second load err = %v", err)
	}

	// Seeded content is coherent and visible to the product surfaces.
	var displayIDs []int
	db.Table("pm_tasks").Where("workspace_id = ?", fixture.workspaceID).Order("display_id").Pluck("display_id", &displayIDs)
	if len(displayIDs) != 6 || displayIDs[0] != 1 || displayIDs[5] != 6 {
		t.Errorf("task display ids = %v", displayIDs)
	}
	if n := workspaceRowCount(t, db, "pm_tasks", fixture.workspaceID); n != 6 {
		t.Errorf("tasks = %d", n)
	}
	var completed int64
	db.Table("pm_tasks").Where("workspace_id = ? AND completed = true", fixture.workspaceID).Count(&completed)
	if completed != 1 {
		t.Errorf("completed tasks = %d, want 1", completed)
	}
	var previews int64
	db.Table("support_conversations").Where("workspace_id = ? AND list_last_message_preview IS NOT NULL", fixture.workspaceID).Count(&previews)
	if previews != 4 {
		t.Errorf("conversations with inbox previews = %d, want 4", previews)
	}
	var linked int64
	db.Table("support_conversations").Where("workspace_id = ? AND linked_task_id IS NOT NULL AND crm_contact_id IS NULL", fixture.workspaceID).Count(&linked)
	if linked != 1 {
		t.Errorf("conversations linked to the checkout bug = %d", linked)
	}
	if n := workspaceRowCount(t, db, "support_messages", fixture.workspaceID); n != 9 {
		t.Errorf("messages = %d, want 9", n)
	}
	if n := workspaceRowCount(t, db, "support_live_messages", fixture.workspaceID); n != 0 {
		t.Errorf("queued live translations = %d, want 0 (no AI work for sample data)", n)
	}
	var words int64
	db.Table("docs_contents").Joins("JOIN docs_documents d ON d.id = docs_contents.document_id").Where("d.workspace_id = ? AND docs_contents.word_count > 0", fixture.workspaceID).Count(&words)
	if words != 3 {
		t.Errorf("articles with content = %d, want 3", words)
	}
	var associations int64
	db.Table("crm_associations").Where("workspace_id = ? AND from_object_type = 'contact' AND to_object_type = 'company'", fixture.workspaceID).Count(&associations)
	if associations < 4 {
		t.Errorf("contact-company associations = %d, want at least 4", associations)
	}
	if n := workspaceRowCount(t, db, "notifications", fixture.workspaceID); n != 0 {
		t.Errorf("notifications = %d, want none for sample data", n)
	}

	evidence, err := repository.NewSetupRepository(db).GetEvidence(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("setup evidence: %v", err)
	}
	for name, count := range map[string]int64{
		"team": evidence.TeamCount, "initial work": evidence.InitialWorkCount, "completed": evidence.CompletedTaskCount,
		"planned project": evidence.PlannedProjectCount, "resolved": evidence.ResolvedConversationCount,
		"validated support": evidence.ValidatedSupportCount, "linked support": evidence.LinkedSupportTaskCount,
		"help space": evidence.HelpCenterSpaceCount, "help content": evidence.HelpCenterContentCount,
		"contacts": evidence.CRMContactCount, "companies": evidence.CRMCompanyCount, "enabled flows": evidence.EnabledAutomationCount,
		"pipelines": evidence.CRMPipelineCount, "deals": evidence.CRMActionableDealCount,
	} {
		if count != 0 {
			t.Errorf("setup evidence %s = %d, want sample data ignored", name, count)
		}
	}

	removed, err := svc.Remove(ctx, fixture.workspaceID, fixture.userID, allSampleModules)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removed.Loaded || len(removed.Retained) != 0 {
		t.Fatalf("status after remove = %+v", removed)
	}
	for _, table := range []string{"pm_tasks", "pm_epics", "workspace_teams", "docs_spaces", "docs_documents", "crm_companies", "crm_contacts", "crm_deals", "crm_pipelines", "crm_associations", "support_conversations", "support_messages", "pm_activity_log", "automation_rules", "sample_data_items"} {
		if n := workspaceRowCount(t, db, table, fixture.workspaceID); n != 0 {
			t.Errorf("%s rows after removal = %d", table, n)
		}
	}
	if _, err := svc.Load(ctx, fixture.workspaceID, fixture.userID, allSampleModules); err != nil {
		t.Fatalf("reload after removal: %v", err)
	}
	if _, err := svc.Remove(ctx, fixture.workspaceID, fixture.userID, allSampleModules); err != nil {
		t.Fatalf("second removal: %v", err)
	}
}

func TestSampleDataPostgresReusesExistingTeamAndPipeline(t *testing.T) {
	db := openSampleDataPostgres(t)
	ctx := context.Background()
	fixture := createSampleWorkspace(t, db)
	team := model.WorkspaceTeam{WorkspaceID: fixture.workspaceID, Name: "Product", TeamType: "engineering", DefaultTaskType: "feature", SprintsEnabled: true}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TeamWorkspaceMembership{TeamID: team.ID, WorkspaceMemberID: fixture.memberID, Role: "member"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.NewCRMDealRepository(db).SeedDefaultPipeline(ctx, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	svc := NewSampleDataService(db, false)
	status, err := svc.Load(ctx, fixture.workspaceID, fixture.userID, allSampleModules)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if status.Counts[model.SampleEntityWorkspaceTeam] != 0 || status.Counts[model.SampleEntityCRMPipeline] != 0 {
		t.Fatalf("existing team/pipeline were tracked as sample: %+v", status.Counts)
	}
	var teamTasks int64
	db.Table("pm_tasks").Where("team_id = ?", team.ID).Count(&teamTasks)
	if teamTasks != 6 {
		t.Fatalf("tasks on the member's team = %d, want 6", teamTasks)
	}
	// A record the user adds to the sample space keeps the space on removal.
	var spaceID string
	db.Table("docs_spaces").Where("workspace_id = ?", fixture.workspaceID).Pluck("id", &spaceID)
	userDoc := model.DocsDocument{WorkspaceID: fixture.workspaceID, SpaceID: spaceID, Title: "My own article", Status: model.DocStatusDraft, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: fixture.userID}
	if err := db.Create(&userDoc).Error; err != nil {
		t.Fatal(err)
	}
	removed, err := svc.Remove(ctx, fixture.workspaceID, fixture.userID, allSampleModules)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removed.Retained[model.SampleEntityDocsSpace] != 1 {
		t.Fatalf("retained = %+v, want the space kept", removed.Retained)
	}
	if n := workspaceRowCount(t, db, "workspace_teams", fixture.workspaceID); n != 1 {
		t.Errorf("teams = %d, want the existing team kept", n)
	}
	if n := workspaceRowCount(t, db, "crm_pipelines", fixture.workspaceID); n != 1 {
		t.Errorf("pipelines = %d, want the existing pipeline kept", n)
	}
	if n := workspaceRowCount(t, db, "docs_documents", fixture.workspaceID); n != 1 {
		t.Errorf("documents = %d, want only the user's document", n)
	}
	if n := workspaceRowCount(t, db, "pm_tasks", fixture.workspaceID); n != 0 {
		t.Errorf("tasks = %d, want 0", n)
	}
}
