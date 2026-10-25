package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// TestCRMBehaviorProfileSyncEndToEnd runs the sync against a migrated
// PostgreSQL database and a ClickHouse instance built from the production
// helpin.events DDL, then filters CRM lists on the synced fields. Both DSNs
// must point at disposable instances; the test creates and drops data.
func TestCRMBehaviorProfileSyncEndToEnd(t *testing.T) {
	pgDSN := os.Getenv("CRM_BEHAVIOR_TEST_DATABASE_URL")
	chDSN := os.Getenv("CRM_BEHAVIOR_TEST_CLICKHOUSE_DSN")
	if pgDSN == "" || chDSN == "" {
		t.Skip("set CRM_BEHAVIOR_TEST_DATABASE_URL (migrated) and CRM_BEHAVIOR_TEST_CLICKHOUSE_DSN to disposable instances")
	}
	ctx := context.Background()
	db, err := gorm.Open(postgres.Open(pgDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	events := openBehaviorTestClickHouse(t, chDSN)

	workspaceID, otherWorkspaceID := uuid.NewString(), uuid.NewString()
	ada, bob, eve, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	acme, globex := uuid.NewString(), uuid.NewString()
	seedBehaviorTestCRM(t, db, workspaceID, otherWorkspaceID, ada, bob, eve, outsider, acme, globex)
	t.Cleanup(func() {
		for _, table := range []string{"crm_contacts", "crm_companies", "crm_identity_links", "crm_behavior_profiles"} {
			db.Exec("DELETE FROM "+table+" WHERE workspace_id IN (?, ?)", workspaceID, otherWorkspaceID)
		}
	})

	now := time.Now().UTC().Truncate(time.Second)
	day := func(days int) time.Time { return now.AddDate(0, 0, days) }
	insertBehaviorTestEvents(t, events, []map[string]any{
		// Ada is identified by email only, across two sessions.
		{"project_id": workspaceID, "timestamp": day(-45), "session_id": "a1", "user_email": "ADA@example.com "},
		{"project_id": workspaceID, "timestamp": day(-3), "session_id": "a2", "user_email": "ada@example.com"},
		// Bob was anonymous, then identified; his link ties both to him and Acme.
		{"project_id": workspaceID, "timestamp": day(-60), "session_id": "b0", "user_anonymous_id": "anon-bob"},
		{"project_id": workspaceID, "timestamp": day(-1), "session_id": "b1", "user_id": "user-bob", "company_id": "acct-acme"},
		{"project_id": workspaceID, "timestamp": day(-1), "session_id": "b2", "user_id": "user-bob", "company_id": "acct-acme"},
		// A bot hit for Eve must be ignored.
		{"project_id": workspaceID, "timestamp": day(-2), "session_id": "e1", "user_email": "eve@example.com", "parsed_ua_bot": 1},
		// Another workspace's event for the same email must not leak in.
		{"project_id": otherWorkspaceID, "timestamp": day(-1), "session_id": "x1", "user_email": "ada@example.com"},
		// Globex shows up by company id equal to the CRM record id.
		{"project_id": workspaceID, "timestamp": day(-10), "session_id": "g1", "company_id": globex},
	})

	profiles := repository.NewCRMBehaviorProfileRepository(db)
	signals := repository.NewCRMSignalRepository(db)
	projects := repository.NewEventProjectRepository(db)
	sync := NewCRMBehaviorProfileSynchronizer(profiles, signals, projects, events)
	result, err := sync.SyncWorkspace(ctx, workspaceID, now)
	if err != nil {
		t.Fatalf("SyncWorkspace: %v", err)
	}
	if result.Contacts != 2 || result.Companies != 2 || result.EmailOnlyContacts != 1 {
		t.Fatalf("sync result = %+v, want Ada and Bob, Acme and Globex", result)
	}

	summaries, err := profiles.SummariesByID(ctx, workspaceID, model.CRMFieldObjectContact, []string{ada, bob, eve})
	if err != nil {
		t.Fatal(err)
	}
	if got := summaries[ada]; got.Sessions30d != 1 || !got.FirstSeenAt.Equal(day(-45)) || !got.LastSeenAt.Equal(day(-3)) ||
		strings.Join(got.MatchMethods, ",") != model.CRMBehaviorMatchEmail {
		t.Fatalf("ada = %+v", got)
	}
	if got := summaries[bob]; got.Sessions30d != 2 || !got.FirstSeenAt.Equal(day(-60)) ||
		strings.Join(got.MatchMethods, ",") != model.CRMBehaviorMatchIdentityLink {
		t.Fatalf("bob = %+v", got)
	}
	if _, ok := summaries[eve]; ok {
		t.Fatal("bot traffic must not create a profile")
	}

	contactRepo := repository.NewCRMContactRepository(db)
	contacts := NewCRMContactService(contactRepo).SetBehaviorProfiles(profiles)
	companies := NewCRMCompanyService(repository.NewCRMCompanyRepository(db)).SetBehaviorProfiles(profiles)
	since := day(-7).Format("2006-01-02")
	listed, total, err := contacts.List(ctx, workspaceID, model.CRMContactListFilters{Query: &model.QueryFilterGroup{
		Logic: model.QueryFilterLogicAnd,
		Rules: []model.QueryFilterRule{{Field: "last_seen_at", Operator: model.QueryFilterOpOnOrAfter, Value: &since}},
	}}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || listed[0].Behavior == nil || listed[1].Behavior == nil {
		t.Fatalf("contacts seen in the last week = %d %+v", total, listed)
	}
	two := "2"
	listedCompanies, total, err := companies.List(ctx, workspaceID, model.CRMCompanyListFilters{Query: &model.QueryFilterGroup{
		Rules: []model.QueryFilterRule{{Field: "sessions_30d", Operator: model.QueryFilterOpGTE, Value: &two}},
	}}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || listedCompanies[0].ID != acme {
		t.Fatalf("companies with 2+ sessions = %d %+v", total, listedCompanies)
	}
	unseen, total, err := contacts.List(ctx, workspaceID, model.CRMContactListFilters{Query: &model.QueryFilterGroup{
		Rules: []model.QueryFilterRule{{Field: "last_seen_at", Operator: model.QueryFilterOpIsEmpty}},
	}}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || unseen[0].ID != eve {
		t.Fatalf("contacts never seen = %d %+v", total, unseen)
	}

	catalog := NewCRMFieldCatalogService(repository.NewCRMCompanyRepository(db), contacts, profiles)
	described, err := catalog.Catalog(ctx, workspaceID, model.CRMFieldObjectContact)
	if err != nil {
		t.Fatal(err)
	}
	if described.Behavior.RecordsWithBehavior != 2 || described.Behavior.TotalRecords != 3 || described.Behavior.LastComputedAt == nil {
		t.Fatalf("coverage = %+v", described.Behavior)
	}
	encoded, _ := json.Marshal(described.CustomProperties)
	if !strings.Contains(string(encoded), `"key":"plan_tier"`) || strings.Contains(string(encoded), "secret") {
		t.Fatalf("contact custom properties = %s", encoded)
	}

	// Upsert on Postgres: existing keys are not reported, new ones are.
	upserter := NewCRMRecordUpsertService(repository.NewCRMCompanyRepository(db), contactRepo, companies, contacts, nil)
	upserted, err := upserter.Upsert(ctx, model.CRMRecordUpsertRequest{
		WorkspaceID: workspaceID, ObjectType: model.CRMUpsertObjectContact, Mode: model.CRMUpsertModeUpdateOnly,
		Records: []model.CRMRecordUpsertInput{{Fields: map[string]any{"email": "ada@example.com"}, CustomProperties: map[string]any{"plan_tier": "scale", "seats": float64(3)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(upserted.NewCustomPropertyKeys) != "[seats]" {
		t.Fatalf("upsert = %+v", upserted)
	}

	// Concurrent replacements of one workspace serialize instead of colliding.
	profileSet, _, err := sync.ComputeWorkspace(ctx, workspaceID, now)
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 4)
	for range 4 {
		go func() { errs <- profiles.ReplaceWorkspaceProfiles(ctx, workspaceID, profileSet) }()
	}
	for range 4 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent replace: %v", err)
		}
	}

	// Profiles of a workspace that stopped collecting events are removed.
	stale := []model.CRMBehaviorProfile{{EntityType: model.CRMFieldObjectContact, EntityID: outsider, WorkspaceID: otherWorkspaceID,
		FirstSeenAt: now, LastSeenAt: now, ComputedAt: now}}
	if err := profiles.ReplaceWorkspaceProfiles(ctx, otherWorkspaceID, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.DeleteProfilesOutsideWorkspaces(ctx, []string{workspaceID}); err != nil {
		t.Fatal(err)
	}
	if other, _ := profiles.Coverage(ctx, otherWorkspaceID, model.CRMFieldObjectContact); other.RecordsWithBehavior != 0 {
		t.Fatalf("stale workspace profiles survived: %+v", other)
	}
	if kept, _ := profiles.Coverage(ctx, workspaceID, model.CRMFieldObjectContact); kept.RecordsWithBehavior != 2 {
		t.Fatalf("active workspace profiles were removed: %+v", kept)
	}

	// A sync whose events aged out drops the profile instead of keeping it.
	if _, err := sync.SyncWorkspace(ctx, workspaceID, now.AddDate(0, 0, 400)); err != nil {
		t.Fatal(err)
	}
	coverage, err := profiles.Coverage(ctx, workspaceID, model.CRMFieldObjectContact)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.RecordsWithBehavior != 0 {
		t.Fatalf("stale profiles survived: %+v", coverage)
	}

	// The lease skips a workspace another replica synced within the interval.
	sync.runWorkspaceAndLog(ctx, workspaceID, now)
	first, _ := profiles.Coverage(ctx, workspaceID, model.CRMFieldObjectContact)
	db.Exec("DELETE FROM crm_behavior_profiles WHERE workspace_id = ?", workspaceID)
	NewCRMBehaviorProfileSynchronizer(profiles, signals, projects, events).runWorkspaceAndLog(ctx, workspaceID, now.Add(10*time.Minute))
	second, _ := profiles.Coverage(ctx, workspaceID, model.CRMFieldObjectContact)
	if first.RecordsWithBehavior != 2 || second.RecordsWithBehavior != 0 {
		t.Fatalf("lease: first run = %+v, recent rerun = %+v; want 2 then a skipped run", first, second)
	}
	db.Exec("DELETE FROM crm_signal_evaluator_watermarks WHERE cadence = ?", "crm_behavior_profile_sync:"+workspaceID)
}

func openBehaviorTestClickHouse(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP DATABASE IF EXISTS helpin SYNC`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE DATABASE helpin`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), `DROP DATABASE IF EXISTS helpin SYNC`)
		db.Close()
	})
	for _, name := range []string{"202608250001_create_helpin_events.sql", "202608250002_event_compatibility.sql", "202608270001_ai_bot_classification.sql"} {
		contents, err := os.ReadFile(filepath.Join("..", "chmigrate", "sql", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range localClickHouseStatements(string(contents)) {
			if _, err := db.ExecContext(ctx, statement); err != nil {
				t.Fatalf("%s: %v\n%s", name, err, statement)
			}
		}
	}
	return db
}

var (
	_clickHouseSettingsBlock = regexp.MustCompile(`(?s)\nSETTINGS\n.*$`)
	_clickHouseOnCluster     = regexp.MustCompile(`\s+ON CLUSTER helpin`)
)

// localClickHouseStatements adapts production DDL to a single local server:
// no Replicated database or engine and no deployment storage policy.
func localClickHouseStatements(contents string) []string {
	var statements []string
	for _, statement := range strings.Split(contents, ";\n") {
		statement = strings.TrimSpace(statement)
		var lines []string
		for _, line := range strings.Split(statement, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		statement = strings.TrimSpace(strings.Join(lines, "\n"))
		statement = strings.TrimSuffix(statement, ";")
		if statement == "" || strings.HasPrefix(statement, "CREATE DATABASE") {
			continue
		}
		statement = _clickHouseOnCluster.ReplaceAllString(statement, "")
		statement = strings.ReplaceAll(statement, "ReplicatedReplacingMergeTree(", "ReplacingMergeTree(")
		statement = _clickHouseSettingsBlock.ReplaceAllString(statement, "")
		statements = append(statements, statement)
	}
	return statements
}

func insertBehaviorTestEvents(t *testing.T, db *sql.DB, events []map[string]any) {
	t.Helper()
	for index, event := range events {
		event["event_id"] = fmt.Sprintf("event-%d", index)
		event["event_type"] = "pageview"
		event["timestamp"] = event["timestamp"].(time.Time).Format("2006-01-02T15:04:05.000Z")
		event["event_received_at"] = event["timestamp"]
		if _, ok := event["parsed_ua_bot"]; !ok {
			event["parsed_ua_bot"] = 0
		}
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(context.Background(),
			`INSERT INTO helpin.events (raw_event, _ingest_version, _written_at) VALUES (?, 1, now())`, string(raw)); err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}
}

func seedBehaviorTestCRM(t *testing.T, db *gorm.DB, workspaceID, otherWorkspaceID, ada, bob, eve, outsider, acme, globex string) {
	t.Helper()
	email := func(value string) *string { return &value }
	externalID, userID := "acct-acme", "user-bob"
	records := []any{
		&model.CRMContact{ID: ada, WorkspaceID: workspaceID, DisplayID: "C-1", FirstName: "Ada", Email: email("ada@example.com"), CustomProperties: model.JSONB{"plan_tier": "pro"}},
		&model.CRMContact{ID: bob, WorkspaceID: workspaceID, DisplayID: "C-2", FirstName: "Bob", Email: email("bob@example.com")},
		&model.CRMContact{ID: eve, WorkspaceID: workspaceID, DisplayID: "C-3", FirstName: "Eve", Email: email("eve@example.com")},
		&model.CRMContact{ID: outsider, WorkspaceID: otherWorkspaceID, DisplayID: "C-1", FirstName: "Ada elsewhere", Email: email("ada@example.com"), CustomProperties: model.JSONB{"secret": "x"}},
		&model.CRMCompany{ID: acme, WorkspaceID: workspaceID, DisplayID: "CO-1", Name: "Acme", ExternalID: &externalID},
		&model.CRMCompany{ID: globex, WorkspaceID: workspaceID, DisplayID: "CO-2", Name: "Globex"},
		&model.CRMIdentityLink{WorkspaceID: workspaceID, AnonymousID: "anon-bob", ExternalUserID: &userID, ContactID: &bob,
			IdentityMethod: "browser_hmac", IdentityTrust: "verified"},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed %T: %v", record, err)
		}
	}
}
