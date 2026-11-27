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

	useSortKey, _ := strconv.ParseBool(os.Getenv("DOCS_ORDERING_USE_SORT_KEY"))
	translationSvc := newTranslationService(db, useSortKey)
	var onFixed func(context.Context, string)
	if invalidator := newCacheInvalidator(db, useSortKey, *apply); invalidator != nil {
		onFixed = invalidator.InvalidateHelpcenterCacheForWorkspace
	}

	report, err := translationSvc.BackfillMissingDefaultLocaleMirrors(ctx, *workspaceID, *apply, onFixed)
	if err != nil {
		log.Fatalf("backfill mirrors: %v", err)
	}
	for _, gap := range report.Gaps {
		log.Printf("workspace=%s missing spaces=%d collections=%d articles=%d",
			gap.WorkspaceID, gap.Spaces, gap.Collections, gap.Articles)
	}

	mode := "DRY-RUN"
	if *apply {
		mode = "APPLY"
	}
	log.Printf("[%s] complete: %d of %d help centers missing mirrors, %d failed",
		mode, len(report.Gaps), report.HelpCenters, report.Failed)
	if report.Failed > 0 {
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
