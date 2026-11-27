//go:build integration

package service

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
)

// TestHelpcenterColdNavigationPostgres seeds a help center the size of a real
// import (40 collections, 210 published articles) and reads its navigation
// with no cache. Cold reads used to rewrite every mirror row first, which took
// many seconds; they must now be a handful of read-only queries.
func TestHelpcenterColdNavigationPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()

	const (
		workspaceID = "11111111-1111-4111-8111-111111111111"
		userID      = "22222222-2222-4222-8222-222222222222"
		spaceID     = "33333333-3333-4333-8333-333333333333"
	)
	// Seed rows without their unrelated parents (workspace, user).
	coldReadExec(t, db, `SET LOCAL session_replication_role = replica`)
	coldReadExec(t, db, `INSERT INTO docs_helpcenter_configs (workspace_id, subdomain, brand_name, default_locale, is_published)
		VALUES (?, 'cold-read', 'Cold Read', 'en', true)`, workspaceID)
	coldReadExec(t, db, `INSERT INTO docs_spaces (id, workspace_id, name, slug, type, created_by)
		VALUES (?, ?, 'Help Center', 'help-center', 'external_capable', ?)`, spaceID, workspaceID, userID)
	coldReadExec(t, db, `INSERT INTO docs_collections (id, space_id, workspace_id, name, slug, public_id, position, created_by)
		SELECT md5('c' || g)::uuid, ?, ?, 'Collection ' || g, 'collection-' || g, substr(md5('cp' || g), 1, 8), g, ?
		FROM generate_series(1, 40) g`, spaceID, workspaceID, userID)
	coldReadExec(t, db, `INSERT INTO docs_documents (id, workspace_id, space_id, collection_id, title, status, position, published_at, created_by)
		SELECT md5('d' || g)::uuid, ?, ?, md5('c' || (1 + g % 40))::uuid, 'Article ' || g, 'published', g, now(), ?
		FROM generate_series(1, 210) g`, workspaceID, spaceID, userID)
	coldReadExec(t, db, `INSERT INTO docs_contents (document_id, content, content_text)
		SELECT md5('d' || g)::uuid, '{"type":"doc","content":[]}'::jsonb, 'Body ' || g
		FROM generate_series(1, 210) g`)
	coldReadExec(t, db, `INSERT INTO docs_helpcenter_articles (document_id, public_id, slug, public_published_at)
		SELECT md5('d' || g)::uuid, substr(md5('ap' || g), 1, 8), 'article-' || g, now()
		FROM generate_series(1, 210) g`)

	svc := newDocsHelpcenterPublicServiceForTest(db)

	// The backfill (startup and command) finds the gaps with PostgreSQL SQL,
	// repairs them, and reports each repaired workspace for cache clearing.
	dryRun, err := svc.translationSvc.BackfillMissingDefaultLocaleMirrors(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("backfill dry run: %v", err)
	}
	if len(dryRun.Gaps) != 1 || dryRun.Gaps[0].Spaces != 1 || dryRun.Gaps[0].Collections != 40 || dryRun.Gaps[0].Articles != 210 {
		t.Fatalf("dry run gaps = %+v, want 1 space, 40 collections, 210 articles", dryRun.Gaps)
	}
	var repaired []string
	if _, err := svc.translationSvc.BackfillMissingDefaultLocaleMirrors(ctx, "", true, func(_ context.Context, id string) {
		repaired = append(repaired, id)
	}); err != nil {
		t.Fatalf("backfill apply: %v", err)
	}
	if len(repaired) != 1 || repaired[0] != workspaceID {
		t.Fatalf("repaired workspaces = %v, want [%s]", repaired, workspaceID)
	}
	after, err := svc.translationSvc.BackfillMissingDefaultLocaleMirrors(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("backfill recheck: %v", err)
	}
	if len(after.Gaps) != 0 {
		t.Fatalf("gaps after backfill = %+v, want none", after.Gaps)
	}

	var statements, writes atomic.Int64
	countStatement := func(tx *gorm.DB) {
		statements.Add(1)
		sql := strings.TrimSpace(strings.ToUpper(tx.Statement.SQL.String()))
		if strings.HasPrefix(sql, "INSERT") || strings.HasPrefix(sql, "UPDATE") || strings.HasPrefix(sql, "DELETE") {
			writes.Add(1)
		}
	}
	cb := db.Callback()
	_ = cb.Query().After("gorm:query").Register("test:count_query", countStatement)
	_ = cb.Row().After("gorm:row").Register("test:count_row", countStatement)
	_ = cb.Raw().After("gorm:raw").Register("test:count_raw", countStatement)
	_ = cb.Create().After("gorm:create").Register("test:count_create", countStatement)
	_ = cb.Update().After("gorm:update").Register("test:count_update", countStatement)

	started := time.Now()
	nav, err := svc.GetSpaceNavigation(ctx, workspaceID, "en", "help-center")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("GetSpaceNavigation: %v", err)
	}
	articles := 0
	for _, collection := range nav {
		articles += len(collection.Articles)
	}
	t.Logf("cold navigation: %s, %d statements, %d writes, %d collections, %d articles",
		elapsed, statements.Load(), writes.Load(), len(nav), articles)

	if len(nav) != 40 || articles != 210 {
		t.Fatalf("navigation = %d collections / %d articles, want 40 / 210", len(nav), articles)
	}
	if writes.Load() != 0 {
		t.Fatalf("cold navigation issued %d writes, want 0", writes.Load())
	}
	if statements.Load() > 20 {
		t.Fatalf("cold navigation issued %d statements, want at most 20", statements.Load())
	}
}

func coldReadExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", strings.Fields(sql)[0], err)
	}
}
