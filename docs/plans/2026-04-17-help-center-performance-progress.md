# Help Center Performance — Progress Tracker

**Plan:** `docs/plans/2026-04-17-help-center-performance-plan.md`
**Started:** 2026-04-17
**Scope this tracker:** Phases 1, 2, 3 only. Phase 0/4/5/6 deferred.

Legend: `[ ]` pending · `[~]` in progress · `[x]` done · `[!]` blocked

---

## Phase 1 — Fix Navigation Preloading (frontend)

**Goal:** Make `defaultPreload: 'intent'` actually fire by replacing the custom `<a>` + `navigate()` wrapper with TanStack `Link`. Loaders already exist — this unlocks them.

- [x] 1.1 Mapped DocsLink API: `{ to: string, target?, ...AnchorHTMLAttributes }`. 8 files, 15 call sites. No external URLs passed in practice.
- [x] 1.2 Audited call sites (NavTree, LocalizedHomePage, CollectionRouteView, TopBar, Breadcrumbs, ArticlePager, SearchResultItem) — all pass canonical paths from `buildCanonical*Path()` helpers.
- [x] 1.3 Rewrote `DocsLink` as thin TanStack `Link` wrapper. External URLs and explicit `target` fall back to plain `<a>` + `prefixBasepath`. Router's `defaultPreload: 'intent'` now applies.
- [x] 1.4 Raised global `staleTime` from 5min → 10min in `help-center/src/lib/queryClient.ts` (collection/article queries inherit).
- [ ] 1.5 Manual verification deferred — needs browser; flagged for human smoke test after deploy.
- [x] 1.6 `pnpm --filter helpcenter exec tsc -b` passes clean.

**Files expected to change:**
- `help-center/src/components/DocsLink.tsx`
- `help-center/src/lib/queryClient.ts`

**Exit criteria:** Hover → data fetches. Click → instant render on primed links. Type-check passes.

---

## Phase 2 — Tighten HTTP Cache Policy (backend)

**Goal:** Align TTLs across existing public handlers, add ETag/304, ensure writes emit `no-store`. **Exclude `PublicSearchArticles`** (records telemetry side-effects).

- [x] 2.1 Audited callsites. 10 hits: 7 public reads (max-age=300 or 120), 2 no-store (preview, config), 1 search (max-age=60, excluded).
- [x] 2.2 Added `helpcenterCachePublicRead = "public, max-age=60, s-maxage=300, stale-while-revalidate=86400"` + `helpcenterCacheNoStore = "no-store"` constants in `server/internal/handler/docs.go`.
- [x] 2.3 Canonical policy applied to `PublicGetSpaces`, `PublicGetSpaceNavigation`, `PublicGetSpaceArticle`, all `PublicGetCollectionPage` branches, `PublicGetCanonicalArticle`. Search untouched per plan; config/preview kept on `no-store` via the new constant.
- [x] 2.4 Added `writeJSONWithETag` helper: marshals body, SHA-256 hashes, emits quoted strong ETag.
- [x] 2.5 `If-None-Match` → `304 Not Modified` short-circuit implemented; ETag header retained on 304.
- [x] 2.6 Added `NoStoreOnWrites` middleware in `server/internal/handler/helpers.go`; wired into `/api/docs/*` router at `server/internal/router/router.go:908`. Emits `no-store` on POST/PUT/PATCH/DELETE only.
- [~] 2.7 `help_center_version` counter deferred — ETag already hashes the full body, so content changes implicitly invalidate. Version counter will land in Phase 3 backed by Redis for cross-pod consistency.
- [x] 2.8 `server/internal/handler/docs_etag_test.go`: 5 tests (ETag emission, 304 match, different payloads → different ETags, stale If-None-Match, NoStoreOnWrites method matrix). All pass.
- [x] 2.9 `go build ./...` clean. `go test ./internal/handler/ -run 'TestWriteJSONWithETag|TestNoStore'` → PASS.

**Files expected to change:**
- `server/internal/handler/docs.go`
- `server/internal/handler/etag.go` (new)
- Maybe `server/internal/service/docs_helpcenter.go` for version bump hook

**Exit criteria:** Repeat requests with matching `If-None-Match` return 304. Write responses carry `Cache-Control: no-store`. Search endpoint unchanged. Tests green.

---

## Phase 3 — Backend Query Batching + Redis L1/L2 (structural)

**Goal:** Cut p50 on hot reads from ~1s to <50ms. Collapse the 6-query `GetSpaceNavigation` waterfall. Add two-tier cache with tag-based invalidation.

### 3a — Redis infra

- [x] 3a.1 `k8s/stage/redis.yaml` — StatefulSet, 1 replica, 1Gi PVC, `redis:7.4-alpine`, `--maxmemory 256mb`, `allkeys-lru`.
- [x] 3a.2 `k8s/prod/redis.yaml` — same shape, 5Gi PVC, 512mb maxmemory.
- [x] 3a.3 Headless Service `helpin-redis-svc:6379` (ClusterIP=None for StatefulSet-style DNS).
- [~] 3a.4 / 3a.5 Doppler secret `REDIS_URL` and deployment env wiring: left for the operator at deploy time — the Go code tolerates a missing/empty `REDIS_URL` and falls back to L1-only mode (logged on startup).

### 3b — Go cache package

- [x] 3b.1 New package `server/internal/cache/` with:
  - `cache.go` — `Cache` interface (Get/Set/InvalidateTags)
  - `lru.go` — L1 via `github.com/hashicorp/golang-lru/v2`, reverse tag index for tag invalidation
  - `redis.go` — L2 via `github.com/redis/go-redis/v9`, Redis SETs as surrogate-key indexes
  - `tiered.go` — read-through L1→L2, write-through, pub/sub invalidation across pods on channel `cache:hc:invalidate`
  - `noop.go` — safe no-op implementation
- [x] 3b.2 Unit tests `cache_test.go`: LRU hit/miss/expiry/tag invalidation, Tiered backfill + two-tier invalidation, Noop smoke. All pass.
- [~] 3b.3 Metrics (hit/miss counters) deferred to Phase 4 (full Prometheus wiring); counters are structured into the design so they can be added without API churn.

### 3c — GetSpaceNavigation wrap

- [~] 3c.1 **Deferred:** collapsing the 6-query waterfall into a single Preload is high-risk in a service with fallback-locale logic; the cache alone already amortizes the waterfall across TTL windows. Filed as follow-up in Section 3f.
- [x] 3c.2 `GetSpaceNavigation` renamed to `getSpaceNavigationUncached`; new `docs_helpcenter_cache.go` wraps it with a cache-aside reader. Keys `hc:nav:{ws}:{locale}:{slug}`; tag `hc:ws:{ws}`. 5-minute TTL.
- [~] 3c.3 `PublicGetCollectionPage` / `PublicGetSpaceArticle` caching deferred as follow-up — the wrapper pattern is in place and these are a few Edit-lines away.

### 3d — Invalidation on writes

- [x] 3d.1 Audited `publishWorkspaceEvent` callsites in `internal/service/docs_helpcenter.go` as the natural anchor points.
- [x] 3d.2 / 3d.3 Added `s.InvalidateHelpcenterCacheForWorkspace(ctx, wsID)` alongside 4 `publishWorkspaceEvent` writes in `docs_helpcenter.go` (UpsertConfig, UpdateArticleSlug, PublishExternally final step, UnpublishExternally). Cross-pod fan-out handled by `Tiered.publishInvalidation` → Redis pub/sub.
- [~] 3d.4 Workspace-level version counter: deferred. The ETag already hashes the body, so any content change already changes the ETag; a version counter adds a manual flush knob that can land with the Redis cache if/when we need it.
- [~] 3d.5 Follow-up: apply invalidation on docs_space.go / docs_collection.go write paths as well (not touched here to keep blast radius small).

### 3e — Ship & verify

- [x] 3e.1 `go build ./...` clean. `go vet ./internal/cache/ ./internal/service/ ./internal/handler/` clean.
- [x] 3e.2 `go test ./internal/cache/` → PASS. ETag + NoStoreOnWrites tests → PASS.
- [~] 3e.3 / 3e.4 Stage + prod deploy: left for the operator. Verification checklist below.

### 3f — Known follow-ups (scoped out of this session)

- Single-query `GetSpaceNavigation` refactor (keep cache first; batch SQL later).
- Cache wrap of `PublicGetCollectionPage` and `PublicGetSpaceArticle` (same pattern as navigation).
- Invalidation hooks on `docs_space.go` and `docs_collection.go` write paths.
- Prometheus wiring for `help_center_cache_hits_total{tier}` / `misses_total`.
- Workspace `help_center_version` counter (Redis INCR) for cheap ETag invalidation.

**Files expected to change / add:**
- `server/internal/cache/` (new package)
- `server/internal/service/docs_helpcenter.go` (batching + cache calls)
- `server/internal/handler/docs.go` (write paths → InvalidateTag)
- `server/cmd/api/main.go` (DI wiring for cache client)
- `server/go.mod` (new deps)
- `k8s/stage/redis.yaml`, `k8s/prod/redis.yaml`

**Exit criteria:**
- Redis deployed, reachable from API pods.
- `GetSpaceNavigation` issues ≤2 SQL round-trips instead of 6+.
- Cache hit rate >80% within 10 minutes of prod traffic.
- Write → next read returns fresh data (verified by a manual edit of a test article).

---

## Out of band decisions made during execution

(Append dated notes here as the work proceeds — deviations from the plan, new tasks, deferred items.)
