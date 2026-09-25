package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthorizeWidgetOrigin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, statement := range []string{
		`CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT, widget_key TEXT, allowed_origins TEXT, active BOOLEAN)`,
		`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, session_token TEXT, expires_at DATETIME, revoked_at DATETIME, last_active_at DATETIME)`,
		`INSERT INTO support_widget_installations VALUES ('one','workspace-one','key-one','{"https://site.example","tauri://localhost"}',true), ('two','workspace-two','key-two','{"https://site.example"}',true), ('off','workspace-off','key-off','{"https://site.example"}',false), ('empty','workspace-empty','key-empty','{}',true)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	for _, row := range []struct {
		token, workspace string
		expires          time.Time
		revoked          *time.Time
	}{
		{"valid", "workspace-one", future, nil}, {"other", "workspace-two", future, nil}, {"expired", "workspace-one", past, nil}, {"revoked", "workspace-one", future, &past}, {"inactive", "workspace-off", future, nil},
	} {
		if err := db.Exec(`INSERT INTO support_widget_sessions(id,workspace_id,session_token,expires_at,revoked_at) VALUES (?,?,?,?,?)`, row.token, row.workspace, row.token, row.expires, row.revoked).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &SupportInboxService{installationRepo: repository.NewSupportInboxInstallationRepository(db), sessionRepo: repository.NewSupportInboxSessionRepository(db)}
	for _, tc := range []struct {
		name, origin string
		ref          widgetorigin.Reference
		allowed      bool
	}{
		{"tauri key", "tauri://localhost", widgetorigin.Reference{WidgetKey: "key-one"}, true},
		{"tauri restored session", "tauri://localhost", widgetorigin.Reference{SessionToken: "valid"}, true},
		{"tauri unconfigured", "tauri://localhost", widgetorigin.Reference{WidgetKey: "key-two"}, false},
		{"key", "https://site.example", widgetorigin.Reference{WidgetKey: "key-one"}, true},
		{"id", "https://site.example", widgetorigin.Reference{InstallationID: "one"}, true},
		{"session", "https://site.example", widgetorigin.Reference{SessionToken: "valid"}, true},
		{"matching pair", "https://site.example", widgetorigin.Reference{WidgetKey: "key-one", SessionToken: "valid"}, true},
		{"cross workspace pair", "https://site.example", widgetorigin.Reference{WidgetKey: "key-one", SessionToken: "other"}, false},
		{"cross installation", "https://site.example", widgetorigin.Reference{WidgetKey: "key-one", InstallationID: "two"}, false},
		{"third origin", "https://third.example", widgetorigin.Reference{SessionToken: "valid"}, false},
		{"expired", "https://site.example", widgetorigin.Reference{SessionToken: "expired"}, false},
		{"revoked", "https://site.example", widgetorigin.Reference{SessionToken: "revoked"}, false},
		{"inactive key", "https://site.example", widgetorigin.Reference{WidgetKey: "key-off"}, false},
		{"inactive token", "https://site.example", widgetorigin.Reference{SessionToken: "inactive"}, false},
		{"empty allowlist", "https://site.example", widgetorigin.Reference{WidgetKey: "key-empty"}, false},
		{"unknown", "https://site.example", widgetorigin.Reference{WidgetKey: "unknown"}, false},
		{"missing references", "https://site.example", widgetorigin.Reference{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.AuthorizeWidgetOrigin(context.Background(), tc.origin, tc.ref); (err == nil) != tc.allowed {
				t.Fatalf("authorize: %v", err)
			}
		})
	}
	err = svc.AuthorizeWidgetOrigin(context.Background(), "tauri://localhost", widgetorigin.Reference{WidgetKey: "key-two"})
	var denied *widgetorigin.DeniedError
	if !errors.As(err, &denied) || denied.InstallationID != "two" {
		t.Fatalf("missing safe installation context: %v", err)
	}
	var touched int64
	if err := db.Table("support_widget_sessions").Where("last_active_at IS NOT NULL").Count(&touched).Error; err != nil {
		t.Fatal(err)
	}
	if touched != 0 {
		t.Fatal("authorization touched sessions")
	}
	if err := db.Exec(`UPDATE support_widget_installations SET allowed_origins = '{"*"}' WHERE id = 'one'`).Error; err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "null", "https://other.example"} {
		if err := svc.AuthorizeWidgetOrigin(context.Background(), origin, widgetorigin.Reference{WidgetKey: "key-one", SessionToken: "valid"}); err != nil {
			t.Fatalf("allow-all rejected %q: %v", origin, err)
		}
		for _, token := range []string{"expired", "revoked", "other"} {
			if err := svc.AuthorizeWidgetOrigin(context.Background(), origin, widgetorigin.Reference{WidgetKey: "key-one", SessionToken: token}); err == nil {
				t.Fatalf("allow-all bypassed credentials for %s", token)
			}
		}
	}
	if err := db.Exec(`UPDATE support_widget_installations SET allowed_origins = '{}' WHERE id = 'one'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.AuthorizeWidgetOrigin(context.Background(), "https://site.example", widgetorigin.Reference{SessionToken: "valid"}); err == nil {
		t.Fatal("existing token bypassed changed policy")
	}
}
