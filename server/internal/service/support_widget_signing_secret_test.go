package service

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func newSigningSecretTestService(t *testing.T) (*SupportInboxService, *gorm.DB, *model.SupportWidgetInstallation) {
	t.Helper()
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS workspace_event_project_aliases (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		project_id TEXT NOT NULL UNIQUE,
		source TEXT NOT NULL,
		valid_from DATETIME,
		valid_to DATETIME,
		created_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS support_credential_rotation_audits (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		installation_id TEXT NOT NULL,
		actor_user_id TEXT NOT NULL,
		rotation_kind TEXT NOT NULL,
		created_at DATETIME
	)`)
	seedWorkspace(t, db, "ws-signing", "Signing", "signing", "user-owner")
	svc := &SupportInboxService{installationRepo: repository.NewSupportInboxInstallationRepository(db)}
	inst, _, err := svc.GetInstallation(context.Background(), "ws-signing")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(inst.SecretKey) == "" {
		t.Fatal("new installation has no signing secret")
	}
	return svc, db, inst
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func auditKinds(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var kinds []string
	if err := db.Raw(`SELECT rotation_kind FROM support_credential_rotation_audits WHERE workspace_id = ? AND actor_user_id = ? ORDER BY rowid`, "ws-signing", "user-admin").Scan(&kinds).Error; err != nil {
		t.Fatal(err)
	}
	return kinds
}

func TestRevealWidgetSecretReturnsCurrentSecretAndAudits(t *testing.T) {
	svc, db, inst := newSigningSecretTestService(t)
	logs := captureSlog(t)
	ctx := context.Background()

	if _, err := svc.RevealWidgetSecret(ctx, "ws-signing", ""); err == nil {
		t.Fatal("reveal without an authenticated actor succeeded")
	}
	if _, err := svc.RevealWidgetSecret(ctx, "ws-missing", "user-admin"); err == nil {
		t.Fatal("reveal for a workspace without an installation succeeded")
	}

	secret, err := svc.RevealWidgetSecret(ctx, "ws-signing", "user-admin")
	if err != nil {
		t.Fatal(err)
	}
	if secret != inst.SecretKey {
		t.Fatal("reveal returned a different secret than the stored one")
	}
	if got := auditKinds(t, db); len(got) != 1 || got[0] != model.CredentialAuditSigningSecretRevealed {
		t.Fatalf("audit kinds = %v", got)
	}
	if !strings.Contains(logs.String(), "revealed support widget signing secret") {
		t.Fatal("reveal was not logged")
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatal("signing secret leaked into logs")
	}
}

func TestRotateWidgetSecretReplacesSecretWithoutLoggingIt(t *testing.T) {
	svc, db, inst := newSigningSecretTestService(t)
	logs := captureSlog(t)
	ctx := context.Background()

	rotated, err := svc.RotateWidgetSecret(ctx, "ws-signing", "user-admin")
	if err != nil {
		t.Fatal(err)
	}
	if rotated == "" || rotated == inst.SecretKey {
		t.Fatal("rotation did not produce a new secret")
	}
	current, err := svc.RevealWidgetSecret(ctx, "ws-signing", "user-admin")
	if err != nil {
		t.Fatal(err)
	}
	if current != rotated {
		t.Fatal("stored secret does not match the rotated secret")
	}
	after, _, err := svc.GetInstallation(ctx, "ws-signing")
	if err != nil {
		t.Fatal(err)
	}
	if after.WidgetKey != inst.WidgetKey {
		t.Fatal("rotating the signing secret changed the public widget key")
	}
	want := []string{model.CredentialAuditSigningSecretRotated, model.CredentialAuditSigningSecretRevealed}
	if got := auditKinds(t, db); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("audit kinds = %v, want %v", got, want)
	}
	for _, secret := range []string{inst.SecretKey, rotated} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("signing secret leaked into logs")
		}
	}
}

func TestRegenerateWidgetKeyAuditsSigningSecretRotation(t *testing.T) {
	svc, db, inst := newSigningSecretTestService(t)
	logs := captureSlog(t)
	regenerated, _, err := svc.RegenerateWidgetKey(context.Background(), "ws-signing", "user-admin")
	if err != nil {
		t.Fatal(err)
	}
	if regenerated.SecretKey == inst.SecretKey || regenerated.WidgetKey == inst.WidgetKey {
		t.Fatal("regeneration did not replace both keys")
	}
	if got := auditKinds(t, db); len(got) != 1 || got[0] != model.CredentialAuditSigningSecretRotated {
		t.Fatalf("audit kinds = %v", got)
	}
	if strings.Contains(logs.String(), regenerated.SecretKey) || strings.Contains(logs.String(), inst.SecretKey) {
		t.Fatal("signing secret leaked into logs")
	}
}
