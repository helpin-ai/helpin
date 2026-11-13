//go:build ee

package repository

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openDemoPostgres opens a transaction on a migrated disposable database and
// rolls it back when the test ends.
func openDemoPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("SUPPORT_OPS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SUPPORT_OPS_TEST_DATABASE_URL to a disposable PostgreSQL database migrated with the ee build to run this test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		tx.Rollback()
		_ = sqlDB.Close()
	})
	return tx
}

func seedDemoHomeWorkspace(t *testing.T, db *gorm.DB) (workspaceID, userID string) {
	t.Helper()
	userID, workspaceID = uuid.NewString(), uuid.NewString()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, email, password_hash, full_name) VALUES (?, ?, 'x', 'Demo Owner')`, []any{userID, userID + "@example.com"}},
		{`INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, 'Home', ?, ?, ?)`,
			[]any{workspaceID, "home-" + workspaceID[:8], strings.ToUpper(strings.NewReplacer("0", "A", "1", "B", "2", "C", "3", "D", "4", "E", "5", "F", "6", "G", "7", "H", "8", "I", "9", "J").Replace(workspaceID[:5])), userID}},
	} {
		if err := db.Exec(stmt.sql, stmt.args...).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return workspaceID, userID
}

func TestDemoRepositoryPostgres(t *testing.T) {
	db := openDemoPostgres(t)
	ctx := context.Background()
	home, user := seedDemoHomeWorkspace(t, db)
	otherHome, _ := seedDemoHomeWorkspace(t, db)
	repo := NewDemoRepository(db)
	newDemo := func(homeWorkspace, companyID string) *DemoWorkspace {
		return &DemoWorkspace{HomeWorkspaceID: homeWorkspace, CompanyID: companyID, CompanyName: "Acme", Domain: "acme.com",
			HelpCenterURL: "https://help.acme.com/en/", Status: DemoStatusQueued, Step: "organization", Report: json.RawMessage(`{}`), RequestedBy: user}
	}
	companyA, companyB := uuid.NewString(), uuid.NewString()

	first, created, err := repo.CreateOrGet(ctx, newDemo(home, companyA))
	if err != nil || !created || first.ID == "" {
		t.Fatalf("CreateOrGet = %+v, %v, %v", first, created, err)
	}
	// The same company again returns the stored build, not a second one.
	again, created, err := repo.CreateOrGet(ctx, newDemo(home, companyA))
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("second CreateOrGet = %+v, %v, %v", again, created, err)
	}
	second, _, err := repo.CreateOrGet(ctx, newDemo(home, companyB))
	if err != nil {
		t.Fatal(err)
	}
	// The same company in another home workspace is a separate build.
	foreign, created, err := repo.CreateOrGet(ctx, newDemo(otherHome, companyA))
	if err != nil || !created || foreign.ID == first.ID {
		t.Fatalf("other workspace CreateOrGet = %+v, %v, %v", foreign, created, err)
	}

	// Lookups never cross home workspaces.
	if got, err := repo.GetByID(ctx, otherHome, first.ID); err != nil || got != nil {
		t.Fatalf("GetByID from another workspace = %+v, %v", got, err)
	}
	if got, err := repo.GetByCompany(ctx, home, companyA); err != nil || got == nil || got.ID != first.ID {
		t.Fatalf("GetByCompany = %+v, %v", got, err)
	}
	if count, err := repo.CountUnfinished(ctx, home); err != nil || count != 2 {
		t.Fatalf("CountUnfinished = %d, %v; want 2", count, err)
	}

	// Each unfinished build is leased once, oldest first; a leased build is
	// not handed out again until its lease ends.
	now := time.Now()
	claimedIDs := map[string]bool{}
	for range 3 {
		claimed, err := repo.ClaimNext(ctx, now, time.Minute)
		if err != nil || claimed == nil {
			t.Fatalf("ClaimNext = %+v, %v", claimed, err)
		}
		if claimedIDs[claimed.ID] {
			t.Fatalf("build %s was leased twice", claimed.ID)
		}
		claimedIDs[claimed.ID] = true
	}
	if claimed, err := repo.ClaimNext(ctx, now, time.Minute); err != nil || claimed != nil {
		t.Fatalf("ClaimNext with everything leased = %+v, %v", claimed, err)
	}
	if claimed, err := repo.ClaimNext(ctx, now.Add(2*time.Minute), time.Minute); err != nil || claimed == nil {
		t.Fatalf("ClaimNext after the lease ended = %+v, %v", claimed, err)
	}

	// A finished build is never claimed, and a failed one can be requeued.
	message, done := "The help center import failed.", time.Now()
	first.Status, first.Step, first.Error, first.CompletedAt, first.LeaseUntil, first.Attempts = DemoStatusFailed, "import", &message, &done, nil, 4
	workspaceID := uuid.NewString()
	first.WorkspaceID = &workspaceID
	first.Report = json.RawMessage(`{"articles_imported": 3, "notes": ["check the logo"]}`)
	if err := repo.Save(ctx, first); err != nil {
		t.Fatal(err)
	}
	second.Status = DemoStatusReady
	if err := repo.Save(ctx, second); err != nil {
		t.Fatal(err)
	}
	if claimed, err := repo.ClaimNext(ctx, now.Add(time.Hour), time.Minute); err != nil || claimed == nil || claimed.ID != foreign.ID {
		t.Fatalf("ClaimNext with two finished builds = %+v, %v; want only the other workspace's build", claimed, err)
	}
	// Requeue is limited to the build's own home workspace.
	if err := repo.Requeue(ctx, otherHome, first.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetByID(ctx, home, first.ID); got.Status != DemoStatusFailed {
		t.Fatalf("another workspace requeued the build: status %s", got.Status)
	}
	if err := repo.Requeue(ctx, home, first.ID); err != nil {
		t.Fatal(err)
	}
	requeued, err := repo.GetByID(ctx, home, first.ID)
	if err != nil || requeued.Status != DemoStatusQueued || requeued.Error != nil || requeued.Attempts != 0 || requeued.CompletedAt != nil ||
		requeued.Step != "import" || derefTestString(requeued.WorkspaceID) != workspaceID || !strings.Contains(string(requeued.Report), "check the logo") {
		t.Fatalf("requeued build = %+v, %v; want it queued at the step it stopped, with its progress kept", requeued, err)
	}
}

func derefTestString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
