package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupDocsHelpcenterConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-helpcenter-config-%s?mode=memory&cache=shared&_busy_timeout=5000", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.Exec(`CREATE TABLE docs_helpcenter_configs (
 demo_mode BOOLEAN NOT NULL DEFAULT 0,
 search_indexing_enabled BOOLEAN NOT NULL DEFAULT 1,
 official_docs_url TEXT NOT NULL DEFAULT '',
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL UNIQUE,
		subdomain TEXT NOT NULL DEFAULT '',
		custom_domain TEXT,
		custom_domain_status TEXT,
		custom_domain_token TEXT,
		custom_domain_verified_at DATETIME,
		custom_domain_checked_at DATETIME,
		custom_domain_last_error TEXT,
		custom_domain_failing_since DATETIME,
		custom_domain_alerted_status TEXT,
		public_url_mode TEXT NOT NULL DEFAULT 'hosted_subdomain',
		reverse_proxy_host TEXT,
		reverse_proxy_base_path TEXT,
		brand_name TEXT NOT NULL DEFAULT '',
		brand_logo_url TEXT,
		brand_logo_dark_url TEXT,
		brand_color TEXT NOT NULL DEFAULT '#000000',
		favicon_url TEXT,
		theme_mode TEXT NOT NULL DEFAULT 'system',
		header_links BLOB DEFAULT x'5b5d',
		footer_config BLOB DEFAULT x'7b7d',
		homepage_config BLOB DEFAULT x'7b7d',
		space_nav_config BLOB DEFAULT x'7b7d',
		search_placeholder TEXT,
		default_locale TEXT NOT NULL DEFAULT 'en',
		enabled_locales TEXT,
		protected_terms TEXT,
		show_language_switcher BOOLEAN NOT NULL DEFAULT 0,
		fallback_to_default_locale BOOLEAN NOT NULL DEFAULT 1,
		is_published BOOLEAN NOT NULL DEFAULT 0,
		chat_widget_enabled BOOLEAN NOT NULL DEFAULT 1,
		ai_answers_enabled BOOLEAN NOT NULL DEFAULT 1,
		seo_title TEXT,
		seo_description TEXT,
		og_title TEXT,
		og_description TEXT,
		og_image_url TEXT,
		og_image_alt TEXT,
		support_email TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create helpcenter config table: %v", err)
	}

	return db
}

func newDocsHelpcenterConfigServiceForTest(db *gorm.DB) *DocsHelpcenterService {
	return NewDocsHelpcenterService(
		repository.NewDocsHelpcenterRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestDocsHelpcenterConfigChatWidgetEnabledDefaultsOnAndCanBeDisabled(t *testing.T) {
	db := setupDocsHelpcenterConfigTestDB(t)
	svc := newDocsHelpcenterConfigServiceForTest(db)
	ctx := context.Background()

	cfg, err := svc.UpsertConfig(ctx, "ws-chat-widget", model.UpdateDocsHelpcenterConfigRequest{
		Subdomain: stringPtr("chat-widget-docs"),
		BrandName: stringPtr("Chat Widget Docs"),
	})
	if err != nil {
		t.Fatalf("create helpcenter config: %v", err)
	}
	assertHelpcenterConfigJSONBool(t, cfg, "chat_widget_enabled", true)

	var req model.UpdateDocsHelpcenterConfigRequest
	if err := json.Unmarshal([]byte(`{"chat_widget_enabled":false}`), &req); err != nil {
		t.Fatalf("decode update request: %v", err)
	}
	cfg, err = svc.UpsertConfig(ctx, "ws-chat-widget", req)
	if err != nil {
		t.Fatalf("disable helpcenter chat widget: %v", err)
	}
	assertHelpcenterConfigJSONBool(t, cfg, "chat_widget_enabled", false)
}

func assertHelpcenterConfigJSONBool(t *testing.T, cfg *model.DocsHelpcenterConfig, key string, want bool) {
	t.Helper()

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal helpcenter config: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode helpcenter config json: %v", err)
	}
	got, ok := payload[key].(bool)
	if !ok {
		t.Fatalf("expected %s to be a boolean in helpcenter config JSON, got %#v", key, payload[key])
	}
	if got != want {
		t.Fatalf("expected %s=%v, got %v", key, want, got)
	}
}

func TestDocsHelpcenterDemoIndexingLifecycle(t *testing.T) {
	svc := newDocsHelpcenterConfigServiceForTest(setupDocsHelpcenterConfigTestDB(t))
	ctx := context.Background()
	on, off := true, false
	cfg, err := svc.UpsertConfig(ctx, "demo", model.UpdateDocsHelpcenterConfigRequest{Subdomain: stringPtr("demo"), BrandName: stringPtr("Example")})
	if err != nil || !cfg.SearchIndexingEnabled || cfg.DemoMode {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	cfg, err = svc.UpsertConfig(ctx, "demo", model.UpdateDocsHelpcenterConfigRequest{DemoMode: &on, SearchIndexingEnabled: &on})
	if err != nil || !cfg.DemoMode || cfg.SearchIndexingEnabled {
		t.Fatalf("demo must force noindex: %+v %v", cfg, err)
	}
	cfg, err = svc.UpsertConfig(ctx, "demo", model.UpdateDocsHelpcenterConfigRequest{DemoMode: &off})
	if err != nil || cfg.SearchIndexingEnabled {
		t.Fatalf("leaving demo must not enable indexing: %+v %v", cfg, err)
	}
	cfg, err = svc.UpsertConfig(ctx, "demo", model.UpdateDocsHelpcenterConfigRequest{SearchIndexingEnabled: &on})
	if err != nil || !cfg.SearchIndexingEnabled {
		t.Fatalf("explicit enable: %+v %v", cfg, err)
	}
	for _, raw := range []string{"javascript:alert(1)", "https://user:secret@example.com", "http://example.com"} {
		if _, err := svc.UpsertConfig(ctx, "demo", model.UpdateDocsHelpcenterConfigRequest{OfficialDocsURL: &raw}); err == nil {
			t.Errorf("accepted unsafe URL %q", raw)
		}
	}
}

func TestDocsHelpcenterCreatedAsDemo(t *testing.T) {
	svc := newDocsHelpcenterConfigServiceForTest(setupDocsHelpcenterConfigTestDB(t))
	on := true
	cfg, err := svc.UpsertConfig(context.Background(), "new-demo", model.UpdateDocsHelpcenterConfigRequest{Subdomain: stringPtr("new-demo"), BrandName: stringPtr("Example"), DemoMode: &on, OfficialDocsURL: stringPtr("https://docs.example.com")})
	if err != nil || !cfg.DemoMode || cfg.SearchIndexingEnabled || cfg.OfficialDocsURL != "https://docs.example.com" {
		t.Fatalf("initial demo: %+v %v", cfg, err)
	}
	saved, err := svc.GetConfig(context.Background(), "new-demo")
	if err != nil || saved.SearchIndexingEnabled {
		t.Fatalf("persisted demo: %+v %v", saved, err)
	}
}
