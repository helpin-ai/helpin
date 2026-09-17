# Help center SSR migration

**Author:** Engineering  
**Date:** 2026-03-31  
**Status:** Draft  
**Priority:** High  

---

## 1. Problem Statement

The public help center is a client-side SPA (React + Vite). Every page visit requires:

1. Download ~200KB JS bundle
2. Parse and execute JavaScript
3. Fetch config API (sequential)
4. Fetch spaces API (sequential, depends on config)
5. Fetch route-specific data (article, collection, navigation)
6. Render content

This produces a **2-3 second blank screen** before any content appears. Competitors (Intercom, Zendesk, Notion) serve pre-rendered HTML that appears in <500ms.

Additionally:
- **Zero SEO**: Crawlers see `<div id="root"></div>` — no indexable content
- **No social previews**: Missing Open Graph / Twitter Card meta tags
- **No structured data**: No JSON-LD for rich search results
- **Excessive API calls**: 60s staleTime causes unnecessary refetches for rarely-changing content
- **No HTTP caching**: API responses lack cache-control headers

## 2. Goals & Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| First Contentful Paint | ~2-3s | <500ms |
| Time to Interactive | ~3-4s | <1s |
| Lighthouse Performance | ~40-50 | >90 |
| Lighthouse SEO | ~30 | >95 |
| SEO indexability | 0% (empty HTML) | 100% (full server-rendered HTML) |
| Article update latency | Instant (no cache) | Immediate (no SSR cache) or <2 min (with LRU cache) |

## 3. Solution Overview

Migrate from a client-side SPA to **server-side rendering (SSR)** using **TanStack Start** — the SSR framework built on TanStack Router, which the app already uses.

**Why TanStack Start (not Next.js or Remix):**
- Already on TanStack Router v1.163 with file-based routing — minimal migration
- Route files keep their structure; we add `loader` functions for server data fetching
- Native React Query dehydration/hydration — loaders pre-fill the cache, `useQuery` hooks find data on first render (see Hydration Contract below)
- Built on Vinxi (Vite-based), so Tailwind/PostCSS/aliases all carry over
- Streaming SSR support out of the box

**How it works:**
1. Browser requests a URL (e.g., `/en/getting-started/welcome`)
2. Node.js server resolves the route, runs loaders (fetches article + navigation from Go backend)
3. Server renders full HTML with content, meta tags, structured data
4. HTML streams to browser — content visible immediately
5. React hydrates for interactivity (search, navigation, feedback)
6. Subsequent navigations are client-side (SPA behavior preserved)

**Article updates:** Without caching, SSR renders on every request by calling the Go backend — updates are immediate. To reduce backend load and improve response times, we add an **explicit server-side cache** (in-memory LRU in the Node.js process, keyed by URL + locale, TTL 120s). This means updates appear within ~2 minutes. HTTP `Cache-Control` headers on Go API responses serve a different purpose: they enable **browser-side caching** for client-side navigations after hydration, reducing redundant fetches during a user's session.

**Caching ownership summary:**
| Layer | What it caches | TTL | Purpose |
|-------|---------------|-----|---------|
| SSR server (Node.js LRU) | Rendered HTML responses | 120s | Avoid re-rendering + re-fetching for identical requests |
| Go API `Cache-Control` headers | API JSON responses | 120-300s | Browser caching for client-side navigations |
| React Query (client) | Query results in memory | 5min staleTime | Avoid refetches during SPA navigation |

Without the SSR LRU cache, every request still hits Go and updates remain immediate. The LRU cache is an optimization that trades ~2min staleness for faster responses and lower backend load. It can be disabled or bypassed for preview routes.

## 4. Scope

### In Scope
- SSR migration using TanStack Start
- Route-level server loaders for data prefetching
- Full SEO: meta description, OG tags, canonical URLs, hreflang, JSON-LD
- HTTP cache headers on `/hc/` API responses
- React Query tuning (5min staleTime, 30min gcTime)
- Infrastructure changes (Node.js runtime, K8s, Docker, CI/CD)
- Streaming SSR for progressive rendering
- Server-side redirects (301s) replacing client-side `useEffect` + `navigate()`

### Out of Scope
- URL structure changes (all existing URLs preserved 1:1)
- Backend API redesign (minimal additions only: sitemap, robots.txt)
- Static site generation / ISR (future enhancement)
- CDN edge caching (future enhancement)
- Help center UI redesign

## 5. Technical Architecture

### Current Architecture
```
Browser → CDN/Ingress → nginx (static files) → Browser renders SPA → API calls → Content visible
         |                                                              |
         ~100ms                                                         ~2-3s total
```

### Target Architecture
```
Browser → CDN/Ingress → Node.js SSR server → Go backend API → HTML streamed to browser
         |                                                      |
         ~100ms                                                  ~300-500ms total
```

### Infrastructure Changes

| Component | Current | Target |
|-----------|---------|--------|
| Runtime | nginx-alpine (static) | node:22-alpine (SSR server) |
| Port | 8080 | 3000 |
| CPU request/limit | 50m/200m (stage) | 100m/300m (stage) |
| Memory request/limit | 64Mi/128Mi (stage) | 128Mi/256Mi (stage) |
| Health check | None | `GET /healthz` |
| API communication | Browser → Go API | Server → Go API (internal K8s DNS) |
| Static assets | nginx serves from /dist | Vinxi serves from /.output/public |
| Env vars | `VITE_API_URL` (build-time) | `INTERNAL_API_URL` (runtime) |

### Data Flow (SSR)
```
1. Request arrives at Node.js server
2. Resolve subdomain from Host header
3. Route matching (TanStack Router)
4. Run route loaders:
   a. Root loader: fetch config first (needed to resolve locale + multilingualEnabled), then fetch spaces
   b. Route loader: fetch article/collection + navigation (parallel, both depend on locale from root)
5. Render React tree to HTML stream
6. Inject <head> tags: title, meta, OG, canonical, hreflang, JSON-LD
7. Stream HTML to browser
8. Browser displays content immediately
9. React hydrates (attaches event listeners)
10. Subsequent navigations: client-side SPA (no full page reload)
```

### SSR → Client Hydration Contract

A critical detail: how does server-fetched data reach client components without double-fetching?

**Mechanism:** TanStack Start integrates with React Query's dehydration/hydration protocol:

1. **Server:** Route loaders call `queryClient.ensureQueryData(queryOptions)` — this populates the server-side QueryClient cache.
2. **Server render:** TanStack Start serializes (dehydrates) the QueryClient state into a `<script>` tag in the HTML: `<script>window.__TANSTACK_DEHYDRATED__ = {...}</script>`.
3. **Client hydration:** The client-side QueryClient rehydrates from this embedded state. All query keys that were fetched on the server are now pre-filled in the client cache.
4. **Components unchanged:** `useQuery()` hooks in `ArticleRouteView`, `CollectionRouteView`, etc. continue to work as-is. On initial hydration, they find data already in the cache (staleTime prevents refetch). No loading spinner, no double-fetch.
5. **Subsequent navigations:** Client-side SPA behavior. `useQuery` fetches fresh data only if the cache is stale.

**What this means for components:**
- Components **keep using `useQuery()`** — they do not switch to reading loader data directly.
- Route `loader` functions are the **only new code** — they pre-warm the QueryClient for the specific queries each route needs.
- The `LoadingState` gates in `__root.tsx` (lines 100-102) and `ArticleRouteView` (line 71) become **no-ops on SSR** because data is already in the cache when the component renders. They still function as fallbacks for client-side navigations to new routes.

**Streaming SSR interaction:** When using streaming, the shell (TopBar, Sidebar skeleton) renders immediately. Suspense boundaries wrap data-dependent content (article body, collection list). As loaders resolve, React streams the completed HTML chunks. The dehydrated QueryClient state is included in the final chunk.

## 6. Implementation Phases

### Phase 0: SSR-Compatible Refactors (2-3 days)
Make existing code isomorphic without changing behavior. Ships as SPA — validates nothing breaks.

**Full SSR audit findings (3 blockers, 1 caution):**

| File | Line | Issue | Severity |
|------|------|-------|----------|
| `src/main.tsx` | 10 | `resolveSubdomain()` called at module top-level — executes during import, crashes on server | BLOCKER |
| `src/lib/utils.ts` | 18, 22 | `window.location.search` and `window.location.hostname` accessed unguarded in function body | BLOCKER |
| `src/hooks/useTheme.ts` | 8 | `localStorage.getItem()` in `useState` initializer — crashes on server | BLOCKER |
| `src/lib/toc.ts` | 8 | `document.createElement('template')` called at function invocation time (used by `extractTocFromHtml` during render in `ArticleRouteView`) | CAUTION |

All other `window`/`document` usage (AppShell, MobileNav, ArticleContent, __root.tsx brand color/favicon, useScrollSpy, useDocumentTitle) is safely inside `useEffect` or event handlers.

**Fixes:**

1. **Isomorphic subdomain resolver** (`src/lib/utils.ts`): Accept optional `hostname` param, guard all `window` access with `typeof window !== 'undefined'`
2. **Module-level init** (`src/main.tsx`): Move `resolveSubdomain()` call inside the render function or behind a lazy init — this file is replaced entirely in Phase 1, but must be safe for Phase 0 SPA validation
3. **Theme hook** (`src/hooks/useTheme.ts`): Guard `localStorage.getItem()` with `typeof window !== 'undefined'`, default to `'system'` on server
4. **TOC parser** (`src/lib/toc.ts`): Replace `document.createElement('template')` with a DOMParser guard or an isomorphic HTML parser (e.g., a regex-based heading extractor for server, DOM for client)
5. **Isomorphic API client** (`src/lib/api.ts`): Server uses `INTERNAL_API_URL` (K8s internal DNS), client uses `/api`

### Phase 1: TanStack Start Migration (3-5 days)
Core framework migration.

1. **Dependencies**: Add `@tanstack/react-start`, `vinxi`; remove `@vitejs/plugin-react`
2. **Config**: New `app.config.ts` replaces `vite.config.ts`
3. **Entry points**: New `client.tsx` (hydration) + `ssr.tsx` (server render) replace `main.tsx`
4. **Router factory**: New `router.tsx` — shared between client and server
5. **Route loaders**: Add `beforeLoad`/`loader` to root, article, and collection routes
6. **Server redirects**: Convert all client-side redirect surfaces to server-side `throw redirect()` (301s). Full inventory:

   **Route-level redirects (6 files, `useEffect` + `navigate()`):**
   - `src/routes/index.tsx` — root → locale home
   - `src/routes/search.tsx` — `/search` → `/$locale/search`
   - `src/routes/$spaceSlug.tsx` — legacy space → locale space
   - `src/routes/$spaceSlug/$articleSlug.tsx` — legacy article → locale article
   - `src/routes/$spaceSlug/index.tsx` — legacy space index → locale
   - `src/routes/$locale/$spaceSlug/index.tsx` — space index → first collection

   **Component-level redirects (3 instances, `window.location.replace/assign()`):**
   - `src/components/routes/ArticleRouteView.tsx:56` — invalid article → collection path (`window.location.replace`)
   - `src/components/routes/CollectionRouteView.tsx:61` — empty space → first collection (`window.location.replace`)
   - `src/components/layout/LocaleSwitcher.tsx:80` — locale switch (`window.location.assign`)

   The component-level redirects are harder to move to `beforeLoad` because they depend on fetched data (article existence, space navigation). Strategy: keep these as client-side redirects initially (they're in `useEffect`, so SSR-safe), and convert to server-side in a follow-up when loaders provide the needed data.

### Phase 2: SEO (2-3 days)
Full search engine optimization.

1. **Meta tags**: Per-route `head()` functions returning title, description, canonical URL
2. **Open Graph**: og:title, og:description, og:type, og:url for articles and collections
3. **Structured data**: JSON-LD `Article` schema on article pages, `WebSite` on home
4. **Hreflang**: `<link rel="alternate" hreflang="xx">` for each enabled locale
5. **Sitemap** (`GET /sitemap.xml`): Served by the Node.js SSR server at the origin root. Each tenant has their own custom domain, so the server resolves the workspace from the `Host` header and generates a per-tenant sitemap dynamically. Includes all published article URLs with `<lastmod>` dates, collection URLs, and the home page (following Help Scout's pattern). The SSR server queries the Go backend's existing article list endpoints to build the XML. Cached in the SSR LRU cache (TTL 1 hour) since sitemap changes are infrequent.
6. **Robots.txt** (`GET /robots.txt`): Minimal, same template for all tenants (following Intercom's pattern). Served by the Node.js SSR server. Content:
   ```
   User-agent: *
   Disallow: /preview/
   Crawl-delay: 1
   Sitemap: https://{host}/sitemap.xml
   ```
   The only dynamic part is the `Sitemap` URL which uses the request `Host` header. Blocks `/preview/` (token-authenticated, not for crawlers). No per-tenant customization needed.

### Phase 3: Caching (1-2 days)
Minimize API calls and maximize cache hits.

1. **HTTP headers on Go backend**:
   - Config/Spaces/Navigation: `Cache-Control: public, max-age=300, stale-while-revalidate=60`
   - Articles: `Cache-Control: public, max-age=120, stale-while-revalidate=60`
   - Search: `Cache-Control: public, max-age=60`
   - Preview: `Cache-Control: no-store`
2. **React Query tuning**: staleTime 5min (from 60s), gcTime 30min (from 5min)

### Phase 4: Infrastructure (2-3 days)
Deploy SSR server to K8s.

1. **Dockerfile**: Multi-stage Node.js build → node:22-alpine runtime
2. **K8s manifests**: Updated container port, probes, resource limits, env vars
3. **CI/CD**: Updated GitHub Actions workflows
4. **Health check**: `GET /healthz` endpoint

### Phase 5: Optimization (1-2 days)
Final performance polish.

1. **Streaming SSR**: Shell streams immediately, content streams when data ready
2. **Font preloading**: `<link rel="preload">` for Inter font
3. **Static asset caching**: `/assets/*` with `immutable, max-age=31536000`

### Phase 6: Testing & Validation (2-3 days)
1. All existing routes render correctly with SSR
2. Redirects return 301 status codes
3. Preview mode works
4. Multi-tenant + multilingual routing works
5. Lighthouse CI: Performance >90, SEO >95
6. Google Rich Results Test validation
7. Cross-browser + mobile testing

**Total timeline: ~2-3 weeks** (Phases 2-5 parallelizable after Phase 1)

## 7. Files Changed

### Modified
| File | Change |
|------|--------|
| `help-center/package.json` | TanStack Start deps, updated scripts |
| `help-center/src/lib/utils.ts` | Isomorphic subdomain resolver |
| `help-center/src/lib/api.ts` | Isomorphic API client |
| `help-center/src/lib/queryClient.ts` | Increased staleTime/gcTime |
| `help-center/src/routes/__root.tsx` | Server loader + head meta |
| `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/$articleSlug.tsx` | Server loader + head |
| `help-center/src/routes/$locale/$spaceSlug/$collectionSlug/index.tsx` | Server loader + head |
| `help-center/src/routes/$locale/index.tsx` | Head meta |
| `help-center/src/routes/$spaceSlug.tsx` | Server-side redirect |
| `help-center/src/routes/$spaceSlug/$articleSlug.tsx` | Server-side redirect |
| `help-center/src/routes/search.tsx` | Server-side redirect |
| `help-center/src/routes/index.tsx` | Server-side redirect |
| `help-center/Dockerfile` | Node.js runtime |
| `server/internal/handler/docs.go` | Cache-control headers |
| `k8s/stage/helpcenter.yaml` | Node.js config, probes, env vars |
| `k8s/prod/helpcenter.yaml` | Node.js config, probes, env vars |
| `.github/workflows/deploy-staging.yml` | Updated build args |
| `.github/workflows/deploy-prod.yml` | Updated build args |

### New
| File | Purpose |
|------|---------|
| `help-center/app.config.ts` | TanStack Start configuration |
| `help-center/src/client.tsx` | Client hydration entry point |
| `help-center/src/ssr.tsx` | Server rendering entry point |
| `help-center/src/router.tsx` | Shared router factory |

### Removed
| File | Reason |
|------|--------|
| `help-center/src/main.tsx` | Replaced by client.tsx + ssr.tsx |
| `help-center/vite.config.ts` | Replaced by app.config.ts |
| `help-center/nginx.conf` | No longer needed (Node.js serves directly) |

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| TanStack Start is newer than Next.js | Medium | Already on TanStack Router; Start is v1 stable; community growing |
| Node.js uses more resources than nginx | Low | Help center is low-traffic; 256Mi is plenty; scale replicas if needed |
| SSR server failure blocks all visitors | High | K8s readiness probes + instant rollback to previous nginx image |
| Hydration mismatches | Medium | Phase 0 fixes all 3 SSR blockers + 1 caution; React 19 has better hydration error reporting |
| Internal API URL changes | Low | Use K8s service DNS (stable); configurable via env var |

## 9. Rollback Plan

The previous nginx-based Docker image remains tagged in GHCR. To rollback:

```bash
# Instant rollback — no code changes needed
kubectl set image deployment/helpcenter helpcenter=ghcr.io/helpin-ai/helpin/helpcenter:v0.93.3
```

## 10. Future Enhancements (Out of Scope)

1. **Edge caching / CDN**: Cache SSR HTML responses at CDN edge (e.g., Cloudflare, Fastly) for <100ms global loads — requires cache key design per tenant + locale
2. **On-demand cache invalidation**: Backend webhook purges SSR LRU cache and/or CDN cache when article is published — eliminates the ~2min staleness window
3. **Incremental Static Regeneration**: Pre-build popular articles as static HTML, revalidate on publish
4. **Image optimization**: Resize/compress article images server-side
5. **Search SSR**: Server-render search results for SEO (low priority — search pages shouldn't be indexed)
