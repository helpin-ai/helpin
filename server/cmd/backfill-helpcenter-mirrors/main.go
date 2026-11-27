// Command backfill-helpcenter-mirrors writes missing default-locale help center
// mirror rows (space, collection and article translations). Public reads used
// to backfill these on every cold request; they no longer do, so run this once
// before deploying that change and after any script that writes docs through
// repositories instead of services. With REDIS_URL set, it also clears the
// help center cache of every workspace it fixes, so no pod keeps serving a
// navigation cached while the mirrors were missing.
//
// Usage:
//
//	go run ./cmd/backfill-helpcenter-mirrors [--apply] [--workspace-id=UUID]
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// missingMirrors counts default-locale mirror rows a workspace lacks.
type missingMirrors struct {
	WorkspaceID string
	Spaces      int64
	Collections int64
	Articles    int64
}

func (m missingMirrors) total() int64 { return m.Spaces + m.Collections + m.Articles }

// missingMirrorsSQL lists workspaces whose published help center content has no
// default-locale mirror. It mirrors the eligibility rules in
// EnsureDefaultLocaleMirrorsForSpace.
const missingMirrorsSQL = `
WITH cfg AS (
	SELECT workspace_id, COALESCE(NULLIF(default_locale, ''), 'en') AS locale
	FROM docs_helpcenter_configs
	WHERE (? = '' OR workspace_id::text = ?)
), spaces AS (
	SELECT s.id, s.workspace_id, cfg.locale
	FROM docs_spaces s JOIN cfg ON cfg.workspace_id = s.workspace_id
	WHERE s.type = 'external_capable' AND s.deleted_at IS NULL
)
SELECT cfg.workspace_id,
	(SELECT COUNT(*) FROM spaces sp
	 WHERE sp.workspace_id = cfg.workspace_id AND NOT EXISTS (
		SELECT 1 FROM docs_helpcenter_space_translations t
		WHERE t.space_id = sp.id AND t.locale = sp.locale)) AS spaces,
	(SELECT COUNT(*) FROM docs_collections c JOIN spaces sp ON sp.id = c.space_id
	 WHERE sp.workspace_id = cfg.workspace_id AND c.deleted_at IS NULL AND NOT EXISTS (
		SELECT 1 FROM docs_helpcenter_collection_translations t
		WHERE t.collection_id = c.id AND t.locale = sp.locale)) AS collections,
	(SELECT COUNT(*) FROM docs_documents d
	 JOIN spaces sp ON sp.id = d.space_id
	 JOIN docs_helpcenter_articles a ON a.document_id = d.id
	 WHERE sp.workspace_id = cfg.workspace_id AND d.deleted_at IS NULL
		AND d.status = 'published' AND a.slug <> '' AND a.public_published_at IS NOT NULL
		AND NOT EXISTS (
			SELECT 1 FROM docs_helpcenter_article_translations t
			WHERE t.document_id = d.id AND t.locale = sp.locale)) AS articles
FROM cfg
ORDER BY cfg.workspace_id`

func main() {
	apply := flag.Bool("apply", false, "write missing mirrors (default is dry-run)")
	workspaceID := flag.String("workspace-id", "", "process one workspace (empty = all)")
	flag.Parse()

	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	var rows []missingMirrors
	if err := db.WithContext(ctx).Raw(missingMirrorsSQL, *workspaceID, *workspaceID).Scan(&rows).Error; err != nil {
		log.Fatalf("count missing mirrors: %v", err)
	}

	useSortKey, _ := strconv.ParseBool(os.Getenv("DOCS_ORDERING_USE_SORT_KEY"))
	translationSvc := newTranslationService(db, useSortKey)
	invalidator := newCacheInvalidator(db, useSortKey, *apply)

	var affected, failed int
	for _, row := range rows {
		if row.total() == 0 {
			continue
		}
		affected++
		log.Printf("workspace=%s missing spaces=%d collections=%d articles=%d",
			row.WorkspaceID, row.Spaces, row.Collections, row.Articles)
		if !*apply {
			continue
		}
		if err := translationSvc.EnsureDefaultLocaleMirrorsForWorkspace(ctx, row.WorkspaceID); err != nil {
			failed++
			log.Printf("workspace=%s backfill failed: %v", row.WorkspaceID, err)
			continue
		}
		if invalidator != nil {
			invalidator.InvalidateHelpcenterCacheForWorkspace(ctx, row.WorkspaceID)
		}
	}

	mode := "DRY-RUN"
	if *apply {
		mode = "APPLY"
	}
	log.Printf("[%s] complete: %d of %d help centers missing mirrors, %d failed",
		mode, affected, len(rows), failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func newTranslationService(db *gorm.DB, useSortKey bool) *service.DocsHelpcenterTranslationService {
	return service.NewDocsHelpcenterTranslationService(
		repository.NewDocsHelpcenterTranslationRepository(db),
		repository.NewDocsHelpcenterRepository(db, useSortKey),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsRedirectRepository(db),
		repository.NewDocsDocumentRepository(db, useSortKey),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, useSortKey),
		nil,
	)
}

// newCacheInvalidator returns a help center service wired to the shared cache,
// or nil when there is nothing to clear (dry run or no REDIS_URL). Clearing
// through the service uses the same tags and pub/sub channel as the API, so
// both Redis entries and every pod's in-process copies are dropped.
func newCacheInvalidator(db *gorm.DB, useSortKey, apply bool) *service.DocsHelpcenterService {
	redisURL := os.Getenv("REDIS_URL")
	if !apply || redisURL == "" {
		return nil
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("parse REDIS_URL: %v", err)
	}
	client := redis.NewClient(opts)
	svc := service.NewDocsHelpcenterService(
		repository.NewDocsHelpcenterRepository(db, useSortKey),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsDocumentRepository(db, useSortKey),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, useSortKey),
		repository.NewDocsRedirectRepository(db),
		nil,
		nil,
	)
	svc.SetHelpcenterCache(cache.NewTiered(cache.TieredConfig{
		L1:      cache.NewLRU(1),
		L2:      cache.NewRedis(client, "hc"),
		Redis:   client,
		Channel: "cache:hc:invalidate",
		PodID:   "backfill-helpcenter-mirrors",
	}))
	return svc
}
