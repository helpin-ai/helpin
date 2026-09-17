# Help Center Performance Plan

**Date:** 2026-04-17
**Owner:** Frontend + Backend
**Scope:** `help-center/` (public docs site) + `server/` (Go API)
**Goal:** Eliminate cold-load slowness and the collection → articles loading spinner observed on `docs.usermaven.com`.

---

## 1. Problem Statement

1. First-visit load of the help center is slow even with no traffic — suggests cold DB wake-up + no edge/HTTP caching.
2. Clicking a collection shows a spinner for several seconds before articles render — classic client-side fetch waterfall after navigation.
3. No visible caching layer in front of the public `/api/hc/*` endpoints for most handlers.

---

## 2. Root-Cause Analysis

> **Revised 2026-04-17 after code review.** The initial diagnosis was partly stale. The true gaps, verified against current code, are below.

### Frontend (`help-center/`)

| Symptom | Evidence | Root cause |
|---|---|---|
| Spinner after clicking collection | Route loaders already exist at `help-center/src/routes/c/$collectionSlug.tsx:19` and `help-center/src/routes/articles/$articleKey.tsx:20`; both call into `help-center/src/lib/routeData.ts:30` which prefetches route + related nav | **Loaders exist, but navigations don't trigger them early** — router is set to `defaultPreload: 'intent'` in `help-center/src/router.tsx:76`, yet `DocsLink` (`help-center/src/components/DocsLink.tsx:24`) is a plain `<a>` + `navigate()` wrapper, **not a TanStack `Link`**. Intent-based preload never fires. |
| HTML shell fast, data slow on first visit | `serve.mjs` in-process HTML cache (500 × 120s) is per-pod | First hit per pod per path pays the full SSR + API cost |
| Query staleness is standard | `src/lib/queryClient.ts` — `staleTime: 300s`, `gcTime: 30min` | Reasonable; content-specific TTLs could go higher, but not the bottleneck |

### Backend (`server/`)

| Symptom | Evidence | Root cause |
|---|---|---|
| Cache headers partly inconsistent | `server/internal/handler/docs.go:1259` — cache headers already set on `PublicGetSpaces`, `PublicGetSpaceNavigation`, `PublicGetCollectionPage`, `PublicGetSpaceArticle`, canonical article, and search | **Policy, not coverage, is the gap**: TTLs differ across endpoints; no ETag/304 path; write handlers don't explicitly emit `no-store` |
| `PublicSearchArticles` is not purely idempotent | `server/internal/handler/docs.go:1488` records analytics/support events before responding | **Cannot blanket-cache search** — treating it as a cacheable GET would drop telemetry |
| 6+ sequential DB queries per nav request | `internal/service/docs_helpcenter.go:1293` `GetSpaceNavigation`: translations → fallback translations → collections → docs → public IDs, all serial | Query waterfall — this is real |
| Cold first-load | Neon Postgres serverless can take ~500ms–1s to wake | Cold start compounds with the serial-query pattern |
| No application-level cache | No Redis/LRU for public help-center reads | Every cache-miss request hits Postgres through the 6-query waterfall |

### Why "even first load with no traffic is slow"
`serve.mjs` HTML cache is cold per pod; the first SSR render calls the Go API, which runs 6 serial queries against a cold Neon; none of the intermediate results are cached between renders. **Plus** `DocsLink` doesn't trigger intent preload, so navigations to collection/article pages can't benefit from the loaders that already exist.

---

## 3. Caching approach for Helpin

Helpin's help center serves customer-editable content across multiple workspaces.
The proposed starting point is server-rendered pages backed by a shared data cache:

- Batch navigation queries to reduce serial database round-trips.
- Use an in-process cache for hot reads and Redis for reuse across API pods.
- Invalidate affected workspace, space, and article entries when content changes.
- Keep immutable static assets cacheable independently from page data.
- Consider edge caching and prewarming after measuring the remaining bottlenecks.

This keeps the initial work focused on the query and navigation gaps identified
above. The implementation phases below define the proposed cache policy.

---

## 3a. Current Kubernetes Infrastructure (reviewed 2026-04-17)

Understanding what already runs is necessary before picking a caching layer. `k8s/stage/` and `k8s/prod/` reveal:

- **Ingress**: NGINX Ingress Controller (`ingressClassName: pp-nginx`) — not Caddy, not Traefik. Files: `k8s/prod/ingress.yaml`, `k8s/prod/ingress-api-helpin.yaml`, `k8s/prod/ingress-client-helpin.yaml`.
- **Custom domains**: Wildcard `*.helpin.center` (plus `*.stage.helpin.center`) via a **static pre-issued wildcard cert** (secret `cert-prod-helpin-center-wildcard`). No on-demand TLS, no cert-manager CRD, no per-customer cert flow. Customer custom domains are subdomains of `helpin.center`, not arbitrary apex domains.
- **Help center pod** (`helpin-helpcenter`, 2 replicas prod): Node 3000, `serve.mjs` serves both HTML (in-process LRU, 500 entries × 120s TTL) and `/assets/*` with `public, max-age=31536000, immutable`. **Memory limit: 256 Mi (tight — already competing with in-proc cache).**
- **API pod** (`helpin-server`, 2 replicas prod): Go on 8080, 1 Gi memory limit. Help-center proxies API calls internally via `INTERNAL_API_URL=http://helpin-server-svc:8080/api`.
- **Caching infra today**: None. **No Redis, Memcached, Valkey, Dragonfly**. Only the in-process `serve.mjs` HTML cache.
- **Observability**: No Prometheus, no HPA. Static replica counts. Only liveness/readiness probes.
- **Static assets**: Served by the Node pod itself; no separate nginx/static layer or CDN.

**Implications:**
- Adding a new edge-cache proxy (Caddy/Souin, Varnish) = new deployment + learning curve. Not justified when NGINX Ingress is already present and caching can live in the Go API.
- NGINX Ingress's `proxy_cache` is per-pod (not shared) and fiddly via annotations — skip it.
- Redis needs to be introduced regardless of approach; it is the single biggest infra addition in this plan.
- Custom-domain TLS is solved for subdomains of `helpin.center`. Supporting arbitrary apex domains (`docs.customer.com`) is out of scope for this plan.

---

## 4. Implementation Plan

> **Revised ordering 2026-04-17.** Measurement comes first — without it, every downstream optimization is guess-work. The frontend gap is navigation preloading (not loader wiring). The HTTP-cache work is policy tightening (not header coverage). Structural backend work lands last.

### Phase 0 — Minimal Measurement First (~1 day, blocks everything else)

You cannot tune what you cannot see. This is deliberately lightweight — not full `kube-prometheus-stack` yet.

- [ ] Add `prometheus/client_golang` to the Go API; expose `/metrics` on a non-public port or behind an auth middleware.
- [ ] Instrument the hot public handlers with a histogram: `help_center_request_duration_seconds{handler,outcome}`.
- [ ] Emit `Server-Timing` response headers from the public help-center handlers: `db;dur=X, render;dur=Y, total;dur=Z`. Visible in browser DevTools immediately.
- [ ] Add counters that will be populated by later phases: `help_center_cache_hits_total{tier}`, `help_center_cache_misses_total`.
- [ ] Log the first-query-after-idle duration per pod as a rough proxy for Neon wake-ups until real RUM is in place.

**Expected impact:** Nothing user-facing yet. Creates a baseline so Phases 1–3 can prove their impact.

### Phase 1 — Fix Navigation Preloading (the real frontend win, ~1–2 days)

Route loaders already exist. The gap is that `DocsLink` short-circuits TanStack Router's `defaultPreload: 'intent'` pathway, so loaders never pre-fire on hover/focus.

- [ ] Replace the hand-rolled `<a>` + `navigate()` in `help-center/src/components/DocsLink.tsx:24` with the TanStack `Link` component — keep the existing prop surface so callers don't change.
- [ ] Verify: hovering a collection link shows a `fetch` in DevTools to the collection endpoint before the click.
- [ ] If any DocsLink usage needs truly external navigation (different origin), branch to a plain `<a>` inside the component rather than dropping `Link` for the common case.
- [ ] Re-measure with Phase 0 metrics. Expected: the click → content time on a primed link collapses toward zero.
- [ ] Optional follow-up: raise `staleTime` to 10 minutes for collection/article queries in `help-center/src/lib/queryClient.ts` — content changes infrequently, so longer staleness is safe.

**Expected impact:** The spinner on collection click disappears for any user who paused/hovered the link before clicking. Cold-click (keyboard navigation, deep link from external) still hits the loader, which Phase 3 then speeds up.

### Phase 2 — Tighten HTTP Cache Policy Where It's Inconsistent (~1 day)

Public handlers already emit cache headers (`server/internal/handler/docs.go:1259`). This phase is surgical, not a sweep.

- [ ] Audit the existing `setHelpcenterCacheHeader` callsites and align on a consistent policy:
  - Read endpoints (spaces, navigation, collection page, article, canonical article): `public, max-age=60, s-maxage=300, stale-while-revalidate=86400`
  - **`PublicSearchArticles` is excluded** — it records support events (`docs.go:1488`), so caching it would drop telemetry. Split telemetry off into an async path if we later want to cache search responses.
- [ ] Add `ETag` (strong hash of canonical response body) + `If-None-Match` handling so repeat visits can 304 even when browser freshness expires.
- [ ] On write handlers (create/update/delete/publish of spaces, collections, articles, translations), explicitly set `Cache-Control: no-store` to prevent any downstream proxy caching the write-response accidentally.
- [ ] **Do not** use a workspace-scoped version **cookie** for cache keying — cookies are hostile to shared caching. Instead, include a workspace-level version stamp (monotonic counter bumped on any HC write) inside the ETag value so invalidation is implicit in the hash.

**Expected impact:** Repeat visits downgrade from full API round-trips to 304s. Gives a real CDN (Phase 6) something to work with if we add one later.

### Phase 3 — Backend Query Batching + Two-Tier Cache (structural, ~3–5 days)

Collapse the serial queries and add a two-tier cache (L1 in-process LRU, L2 Redis). Use tag-based invalidation to evict entries affected by content changes.

**K8s additions:**
- [ ] New Redis StatefulSet in the `helpin` namespace: `k8s/prod/redis.yaml`, `k8s/stage/redis.yaml`. ~50–100 Mi footprint, one replica with persistent volume claim (cache loss is fine — warm back from Postgres). Secret `REDIS_URL` via Doppler.
- [ ] ClusterIP service `helpin-redis-svc:6379` — internal-only, no Ingress.

**Go API changes (`server/internal/`):**
- [ ] Add a `cache` package with interface `Get/Set/InvalidateTag`. Provide two implementations:
  - **L1** — `hashicorp/golang-lru/v2` in-process, ~10 Mi budget, 30–60 s TTL
  - **L2** — Redis client (`go-redis/v9`), 5–15 min TTL, shared across Go pods
  - Reads: L1 → L2 → DB. Writes: populate both.
- [ ] Refactor `GetSpaceNavigation` in `internal/service/docs_helpcenter.go` to issue **one joined query** (collections + articles + translations + public IDs) using GORM Preload/raw join.
- [ ] Apply caching to the hot public reads: `GetSpaceNavigation`, `PublicGetCollectionPage`, `PublicGetSpaceArticle`.
- [ ] **Tag-based invalidation (Redis SETs as surrogate keys):**
  - On cache set: `SADD tag:ws:{wsId} <cacheKey>`, `SADD tag:space:{spaceId} <cacheKey>`, `SADD tag:article:{articleId} <cacheKey>` with matching TTL.
  - On write (create/update/delete): `SMEMBERS tag:...` → `DEL` all members in a pipeline, then `DEL` the tag set itself.
  - Publish invalidation over Redis pub/sub so each Go pod also drops its L1 entries immediately.
- [ ] Use the workspace-level version stamp from Phase 2 as part of the cache key so writes implicitly orphan old entries; tag-set `DEL`s are the fast path, the version bump is the safety net.

**Expected impact:** p50 API response drops from ~500–1500 ms to <50 ms on L1 hits, <20 ms on L2 hits. Neon cold-start cost paid only on the first miss per TTL window. Amplified under SSR — every server-side render benefits.

### Phase 4 — Full Observability + Autoscaling (~2 days)

Phase 0 gave us the bare minimum. This is the proper deployment.

- [ ] Deploy `kube-prometheus-stack` (or equivalent) in the cluster — Grafana dashboard tracking cache hit rate per tier, p50/p95/p99 per handler, Neon wake proxy.
- [ ] Add alert rules for cache hit rate falling under 80% and p95 over 500 ms.
- [ ] `HorizontalPodAutoscaler` on `helpin-helpcenter` (CPU 70%, min 2, max 6) and `helpin-server` (CPU 60%, min 2, max 8).
- [ ] Wire Grafana dashboards into the team's on-call view.

### Phase 5 — SEO polish

- [ ] Emit JSON-LD (`Article`, `BreadcrumbList`) on article pages for search indexing.
- [ ] Generate per-workspace `sitemap.xml` from the Go API with `Cache-Control: public, max-age=3600`.
- [ ] Emit `llms.txt` / `llms-full.txt` per space — useful for AI retrieval.

### Phase 6 — (Optional) External CDN (defer until traffic demands it)

Only if metrics from Phase 4 show the Go API is still a bottleneck, front `helpin.center` + `*.helpin.center` with a CDN (Cloudflare proxy mode works cleanly with NGINX Ingress backends):

- [ ] Flip DNS for `*.helpin.center` to Cloudflare proxy; keep origin cert on NGINX Ingress.
- [ ] Cloudflare respects the `s-maxage` and `stale-while-revalidate` from Phase 1 automatically.
- [ ] Purge via Cloudflare API on write paths, keyed by the same surrogate tags from Phase 3.

Not needed if Phase 1–3 hit the success criteria.

### Out of scope

- Arbitrary apex custom domains (`docs.customer.com`) — requires cert-manager DNS-01 or an on-demand TLS layer (e.g. Caddy sidecar with the admin API). Separate project.
- Replacing NGINX Ingress with another controller.

---

## 5. Success Criteria

| Metric | Current (observed) | Target |
|---|---|---|
| First-visit LCP (cold) | ~4–6s | <1.5s |
| Repeat-visit LCP | ~1–2s | <500ms |
| Collection-click spinner time | ~2–4s | 0 (data inlined via loader) |
| API p50 for `/api/hc/.../navigation` | ~500–1500ms | <50ms (cache hit) |
| Cache hit rate for public HC reads | 0% | >90% |

---

## 6. References

- Codebase investigation (2026-04-17): `help-center/src/lib/rootLoader.ts`, `help-center/src/lib/services.ts`, `help-center/src/lib/queryClient.ts`, `help-center/serve.mjs`, `server/internal/handler/docs.go`, `server/internal/service/docs_helpcenter.go`
