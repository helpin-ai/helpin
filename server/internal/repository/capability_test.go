package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupCapabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:capability_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, organization_id TEXT)`,
		`CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT 1)`,
		`CREATE TABLE docs_chunks (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE support_email_logs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, direction TEXT NOT NULL)`,
		`CREATE TABLE git_integrations (id TEXT PRIMARY KEY, organization_id TEXT, provider TEXT, active BOOLEAN, deleted_at DATETIME, installation_id TEXT)`,
		`CREATE TABLE git_repositories (id TEXT PRIMARY KEY, workspace_id TEXT, active BOOLEAN, selected BOOLEAN, deleted_at DATETIME)`,
		`CREATE TABLE ai_connections (id TEXT PRIMARY KEY, scope TEXT, status TEXT, superseded_by TEXT, last_verified_at DATETIME, last_verification_error TEXT)`,
		`CREATE TABLE instance_capability_checks (key TEXT PRIMARY KEY, ok BOOLEAN NOT NULL, error TEXT, config_fingerprint TEXT NOT NULL, checked_by TEXT, checked_at DATETIME NOT NULL)`,
		`CREATE TABLE crm_meeting_provider_events (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, provider TEXT NOT NULL)`,
		`CREATE TABLE crm_email_accounts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, provider TEXT NOT NULL, status TEXT NOT NULL, is_active BOOLEAN NOT NULL, refresh_token_encrypted TEXT)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestCapabilityRepositoryEvidence(t *testing.T) {
	db := setupCapabilityTestDB(t)
	repo := NewCapabilityRepository(db)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO workspaces VALUES ('ws', 'org'), ('other', NULL)`,
		`INSERT INTO support_widget_installations VALUES ('i1', 'ws', 1), ('i2', 'ws', 0)`,
		`INSERT INTO docs_chunks VALUES ('c1', 'ws')`,
		`INSERT INTO support_email_logs VALUES ('l1', 'ws', 'inbound'), ('l2', 'other', 'outbound')`,
		`INSERT INTO git_integrations VALUES ('g1', 'org', 'github', 1, NULL, '42'), ('g2', 'org2', 'github', 1, NULL, '')`,
		`INSERT INTO git_repositories VALUES ('r1', 'ws', 1, 1, NULL), ('r2', 'ws', 1, 0, NULL)`,
		`INSERT INTO ai_connections VALUES ('a1', 'workspace', 'connected', NULL, CURRENT_TIMESTAMP, NULL), ('a2', 'workspace', 'connected', NULL, CURRENT_TIMESTAMP, 'failed'), ('a3', 'personal', 'connected', NULL, CURRENT_TIMESTAMP, NULL)`,
		`INSERT INTO crm_meeting_provider_events VALUES ('e1', 'ws', 'recall')`,
		`INSERT INTO crm_email_accounts VALUES ('m1', 'ws', 'gmail', 'connected', 1, 'enc'), ('m2', 'other', 'gmail', 'pending_oauth', 1, NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if org, err := repo.WorkspaceOrganizationID(ctx, "ws"); err != nil || org != "org" {
		t.Fatal(org, err)
	}
	if org, err := repo.WorkspaceOrganizationID(ctx, "other"); err != nil || org != "" {
		t.Fatal(org, err)
	}
	if n, err := repo.ActiveWidgetInstallationCount(ctx, "ws"); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	for _, tc := range []struct {
		name string
		got  func() (bool, error)
		want bool
	}{
		{"workspace chunks", func() (bool, error) { return repo.HasEmbeddedChunks(ctx, "ws") }, true},
		{"other workspace chunks", func() (bool, error) { return repo.HasEmbeddedChunks(ctx, "other") }, false},
		{"instance chunks", func() (bool, error) { return repo.HasEmbeddedChunks(ctx, "") }, true},
		{"inbound email", func() (bool, error) { return repo.HasInboundSupportEmail(ctx, "ws") }, true},
		{"outbound only", func() (bool, error) { return repo.HasInboundSupportEmail(ctx, "other") }, false},
		{"github installation", func() (bool, error) { return repo.HasGitHubInstallation(ctx, "org") }, true},
		{"github without installation id", func() (bool, error) { return repo.HasGitHubInstallation(ctx, "org2") }, false},
		{"no organization", func() (bool, error) { return repo.HasGitHubInstallation(ctx, "") }, false},
		{"workspace meeting event", func() (bool, error) { return repo.HasMeetingProviderEvent(ctx, "ws", "recall") }, true},
		{"other provider meeting event", func() (bool, error) { return repo.HasMeetingProviderEvent(ctx, "ws", "vexa") }, false},
		{"other workspace meeting event", func() (bool, error) { return repo.HasMeetingProviderEvent(ctx, "other", "recall") }, false},
		{"instance meeting event", func() (bool, error) { return repo.HasMeetingProviderEvent(ctx, "", "recall") }, true},
		{"connected google account", func() (bool, error) { return repo.HasConnectedGoogleAccount(ctx, "ws") }, true},
		{"pending google account", func() (bool, error) { return repo.HasConnectedGoogleAccount(ctx, "other") }, false},
		{"instance google account", func() (bool, error) { return repo.HasConnectedGoogleAccount(ctx, "") }, true},
	} {
		got, err := tc.got()
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %v err %v", tc.name, got, err)
		}
	}
	if n, err := repo.ConnectedRepositoryCount(ctx, "ws"); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	summary, err := repo.SharedAIConnectionSummary(ctx)
	if err != nil || summary.Connected != 2 || summary.Verified != 1 {
		t.Fatalf("%+v %v", summary, err)
	}
}

func TestCapabilityRepositoryRecordCheckReplacesPrevious(t *testing.T) {
	repo := NewCapabilityRepository(setupCapabilityTestDB(t))
	ctx := context.Background()
	if check, err := repo.GetCheck(ctx, "email_outbound"); err != nil || check != nil {
		t.Fatal(check, err)
	}
	failure := "failed"
	if err := repo.RecordCheck(ctx, &model.InstanceCapabilityCheck{Key: "email_outbound", OK: false, Error: &failure, ConfigFingerprint: "a", CheckedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCheck(ctx, &model.InstanceCapabilityCheck{Key: "email_outbound", OK: true, ConfigFingerprint: "b", CheckedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	check, err := repo.GetCheck(ctx, "email_outbound")
	if err != nil || check == nil || !check.OK || check.Error != nil || check.ConfigFingerprint != "b" {
		t.Fatalf("%+v %v", check, err)
	}
}
