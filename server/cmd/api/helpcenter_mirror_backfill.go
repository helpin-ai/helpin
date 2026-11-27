package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/service"
)

const (
	_helpcenterMirrorBackfillLockKey = "hc:mirror-backfill:lock"
	_helpcenterMirrorBackfillTimeout = 30 * time.Minute
)

// runStartupHelpcenterMirrorBackfill writes default-locale help center mirrors
// that are missing, then clears each repaired help center's cache. Public
// reads stopped writing mirrors, so without this a help center whose mirrors
// were never written would show an empty navigation after a deploy. When
// nothing is missing it costs one query. With Redis, one pod runs it at a time.
func runStartupHelpcenterMirrorBackfill(
	translationSvc *service.DocsHelpcenterTranslationService,
	helpcenterSvc *service.DocsHelpcenterService,
	rdb *redis.Client,
	podID string,
) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("startup: helpcenter mirror backfill panicked", "panic", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), _helpcenterMirrorBackfillTimeout)
	defer cancel()

	if rdb != nil {
		acquired, err := rdb.SetNX(ctx, _helpcenterMirrorBackfillLockKey, podID, _helpcenterMirrorBackfillTimeout).Result()
		if err != nil {
			slog.WarnContext(ctx, "startup: helpcenter mirror backfill lock failed", "error", err)
			return
		}
		if !acquired {
			return
		}
		defer func() {
			if owner, err := rdb.Get(context.Background(), _helpcenterMirrorBackfillLockKey).Result(); err == nil && owner == podID {
				rdb.Del(context.Background(), _helpcenterMirrorBackfillLockKey)
			}
		}()
	}

	report, err := translationSvc.BackfillMissingDefaultLocaleMirrors(ctx, "", true, helpcenterSvc.InvalidateHelpcenterCacheForWorkspace)
	if err != nil {
		slog.ErrorContext(ctx, "startup: helpcenter mirror backfill failed", "error", err)
		return
	}
	if len(report.Gaps) > 0 {
		slog.InfoContext(ctx, "startup: helpcenter mirror backfill complete",
			"repaired", len(report.Gaps)-report.Failed, "failed", report.Failed, "help_centers", report.HelpCenters)
	}
}
