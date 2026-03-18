# Help Center Routing & Redirects Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish canonical `/:collectionSlug/:articleSlug` public URLs for the help center, with unified redirect support for imported legacy URLs, slug changes, and manual redirects.

**Architecture:** Unified `docs_redirects` table replaces `docs_slug_aliases`. New public API routes resolve articles by collection+article slug (spaces invisible in public URLs). Backend auto-creates redirects on slug changes. Import service populates legacy redirects. Admin UI for managing redirects in help center settings. Public help center frontend is a separate deployment (not in this repo) — this plan covers backend API + admin UI only.

**Tech Stack:** Go 1.24, Chi router, GORM/PostgreSQL, React/TypeScript frontend (admin UI)

**Spec:** `docs/superpowers/specs/2026-03-18-helpcenter-routing-redirects-design.md`

---

## File Structure

### Backend — New Files

| File | Responsibility |
|------|----------------|
| `server/internal/model/docs_redirect.go` | `DocsRedirect` GORM model + DTOs + constants |
| `server/internal/repository/docs_redirect.go` | Redirect CRUD (create, bulk create, lookup, list, delete) |
| `server/migrations/051_docs_redirects.sql` | Create `docs_redirects` table, migrate slug aliases, update indexes |

### Backend — Modified Files

| File | Change |
|------|--------|
| `server/internal/router/router.go` | Add new canonical public routes + admin redirect routes |
| `server/internal/handler/docs.go` | Add canonical article/collection handlers + redirect resolver + admin CRUD handlers |
| `server/internal/service/docs_helpcenter.go` | Add canonical resolution, auto-redirect on slug change, redirect CRUD |
| `server/internal/repository/docs_helpcenter.go` | Add `GetPublicArticleByCollectionSlug` query |
| `server/internal/service/docs_import.go` | Create legacy redirects during import |
| `server/cmd/api/main.go` | Wire `DocsRedirectRepository` into services |

### Frontend — New Files

| File | Responsibility |
|------|----------------|
| `frontend/src/components/settings/RedirectsTab.tsx` | Redirects management page (list, search, create, delete) |
| `frontend/src/lib/services/docsRedirectService.ts` | API service for redirects CRUD |

### Frontend — Modified Files

| File | Change |
|------|--------|
| `frontend/src/pages/Settings.tsx` | Add Redirects tab to help center settings group |

---

## Task 1: Schema Migration + Model

**Files:**
- Create: `server/migrations/051_docs_redirects.sql`
- Create: `server/internal/model/docs_redirect.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Write migration file**

Create `server/migrations/051_docs_redirects.sql`:
```sql
-- Unified redirects table (replaces docs_slug_aliases)
CREATE TABLE IF NOT EXISTS docs_redirects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  source_path TEXT NOT NULL,
  target_collection_slug TEXT NOT NULL,
  target_article_slug TEXT,
  type TEXT NOT NULL DEFAULT 'manual',
  source_system TEXT,
  source_object_type TEXT,
  source_object_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_redirects_ws_source
  ON docs_redirects (workspace_id, source_path);
CREATE INDEX IF NOT EXISTS idx_docs_redirects_ws
  ON docs_redirects (workspace_id);

-- Migrate existing slug aliases to redirects
INSERT INTO docs_redirects (workspace_id, source_path, target_collection_slug, target_article_slug, type)
  SELECT sa.workspace_id, sa.old_slug, COALESCE(c.slug, ''), ha.slug, 'slug_change'
  FROM docs_slug_aliases sa
  JOIN docs_helpcenter_articles ha ON ha.document_id = sa.document_id
  JOIN docs_documents d ON d.id = sa.document_id
  LEFT JOIN docs_collections c ON c.id = d.collection_id
  ON CONFLICT DO NOTHING;

-- Update collection slug uniqueness to workspace-level
DROP INDEX IF EXISTS idx_docs_collection_space_slug;
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collection_ws_slug
  ON docs_collections (workspace_id, slug) WHERE slug != '' AND deleted_at IS NULL;
```

- [ ] **Step 2: Create DocsRedirect model**

Create `server/internal/model/docs_redirect.go`:
- `DocsRedirect` GORM struct matching the table
- `TableName()` → `"docs_redirects"`
- Constants: `RedirectTypeImported = "imported"`, `RedirectTypeSlugChange = "slug_change"`, `RedirectTypeManual = "manual"`
- DTOs: `CreateDocsRedirectRequest` (source_path, target_collection_slug, target_article_slug)
- `DocsRedirectFilter` struct: Search string, Type string, Page int, PerPage int
- `DocsRedirectListResponse`: Items []DocsRedirect, Total int64

- [ ] **Step 3: Register model in AutoMigrate**

Add `&model.DocsRedirect{}` to AutoMigrate in `server/cmd/api/main.go`.

- [ ] **Step 4: Build and verify**

Run: `cd /root/teampulse/server && go build ./...`

- [ ] **Step 5: Commit**

```
feat: add docs_redirects table and model
```

---

## Task 2: Redirect Repository

**Files:**
- Create: `server/internal/repository/docs_redirect.go`

- [ ] **Step 1: Create repository**

```go
type DocsRedirectRepository struct { db *gorm.DB }
func NewDocsRedirectRepository(db *gorm.DB) *DocsRedirectRepository
```

Methods:
- `Create(ctx, redirect *model.DocsRedirect) error`
- `BulkCreate(ctx, redirects []model.DocsRedirect) error` — uses `Clauses(clause.OnConflict{DoNothing: true})` to skip duplicates
- `GetBySourcePath(ctx, workspaceID, sourcePath string) (*model.DocsRedirect, error)` — returns nil if not found
- `List(ctx, workspaceID string, filter model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error)` — with search (ILIKE on source_path), type filter, pagination
- `Delete(ctx, id string) error`

- [ ] **Step 2: Build and verify**

- [ ] **Step 3: Commit**

```
feat: add docs redirect repository
```

---

## Task 3: Canonical Public Routes + Resolution

**Files:**
- Modify: `server/internal/repository/docs_helpcenter.go`
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Add repository method for collection-slug-based article lookup**

In `server/internal/repository/docs_helpcenter.go`, add:

```go
// GetPublicArticleByCollectionSlug looks up a published article by collection slug + article slug.
func (r *DocsHelpcenterRepository) GetPublicArticleByCollectionSlug(
    ctx context.Context, workspaceID, collectionSlug, articleSlug string,
) (*model.DocsDocument, *model.DocsHelpcenterArticle, *model.DocsContent, error)
```

Query: JOIN docs_documents → docs_collections → docs_helpcenter_articles → docs_content
WHERE collections.slug = collectionSlug AND helpcenter_articles.slug = articleSlug
AND documents.status = 'published' AND helpcenter_articles.public_published_at IS NOT NULL
AND documents.workspace_id = workspaceID AND documents.deleted_at IS NULL

Also add:
```go
// GetPublicCollectionBySlug returns a collection and its published articles by collection slug.
func (r *DocsHelpcenterRepository) GetPublicCollectionBySlug(
    ctx context.Context, workspaceID, collectionSlug string,
) (*model.DocsCollection, []model.PublicNavArticle, error)
```

- [ ] **Step 2: Add service methods for canonical resolution**

In `server/internal/service/docs_helpcenter.go`, add:

```go
// GetPublicArticleByCanonicalPath resolves /:collectionSlug/:articleSlug to a full article response.
func (s *DocsHelpcenterService) GetPublicArticleByCanonicalPath(
    ctx context.Context, workspaceID, collectionSlug, articleSlug string,
) (*model.PublicArticleResponse, error)
```

Flow: call repo.GetPublicArticleByCollectionSlug, render HTML from TipTap content, increment view count, return PublicArticleResponse.

```go
// GetPublicCollection resolves /:collectionSlug to a collection page with articles.
func (s *DocsHelpcenterService) GetPublicCollection(
    ctx context.Context, workspaceID, collectionSlug string,
) (*model.DocsCollection, []model.PublicNavArticle, error)
```

```go
// ResolvePublicPath checks if a path matches a redirect. Returns redirect target or nil.
func (s *DocsHelpcenterService) ResolvePublicPath(
    ctx context.Context, workspaceID, path string,
) (*model.DocsRedirect, error)
```

Flow: call redirectRepo.GetBySourcePath. Return redirect or nil.

- [ ] **Step 3: Wire DocsRedirectRepository into services**

In `server/cmd/api/main.go`:
- Create `docsRedirectRepo := repository.NewDocsRedirectRepository(db)`
- Pass to `DocsHelpcenterService` (update constructor)
- Pass to `DocsImportService` (update constructor)

Update `DocsHelpcenterService` struct to include `redirectRepo *repository.DocsRedirectRepository`.

- [ ] **Step 4: Add handler methods**

In `server/internal/handler/docs.go`, add:

```go
// PublicGetCollectionPage handles GET /hc/{subdomain}/c/{collectionSlug}
func (h *DocsHandler) PublicGetCollectionPage(w http.ResponseWriter, r *http.Request)

// PublicGetCanonicalArticle handles GET /hc/{subdomain}/c/{collectionSlug}/{articleSlug}
func (h *DocsHandler) PublicGetCanonicalArticle(w http.ResponseWriter, r *http.Request)

// PublicResolvePath handles GET /hc/{subdomain}/resolve/{path...}
// Returns 301 redirect JSON or 404
func (h *DocsHandler) PublicResolvePath(w http.ResponseWriter, r *http.Request)
```

For `PublicResolvePath`: extract `{path...}` (catch-all), call `ResolvePublicPath`, if found return:
```json
{"redirect": true, "target": "/account-management/team-members", "status": 301}
```
If not found: 404.

- [ ] **Step 5: Register routes**

In `server/internal/router/router.go`, add to the `/hc/{subdomain}` group:

```go
// Canonical collection + article routes
r.Get("/c/{collectionSlug}", h.Docs.PublicGetCollectionPage)
r.Get("/c/{collectionSlug}/{articleSlug}", h.Docs.PublicGetCanonicalArticle)

// Legacy/redirect resolver (catch-all)
r.Get("/resolve/*", h.Docs.PublicResolvePath)
```

Keep existing `/spaces/` routes for backward compatibility.

- [ ] **Step 6: Build and verify**

Run: `cd /root/teampulse/server && go build ./...`

- [ ] **Step 7: Commit**

```
feat: add canonical collection/article public routes with redirect resolution
```

---

## Task 4: Auto-Redirect on Slug Change

**Files:**
- Modify: `server/internal/service/docs_helpcenter.go`

- [ ] **Step 1: Update PublishExternally to create redirects on slug change**

In `PublishExternally()`, find where `DocsSlugAlias` is created when slug changes. Replace with creating a `DocsRedirect`:

```go
// When slug changes, create redirect from old canonical path to new
if oldSlug != newSlug {
    redirect := &model.DocsRedirect{
        WorkspaceID:          workspaceID,
        SourcePath:           fmt.Sprintf("/%s/%s", collectionSlug, oldSlug),
        TargetCollectionSlug: collectionSlug,
        TargetArticleSlug:    &newSlug,
        Type:                 model.RedirectTypeSlugChange,
    }
    _ = s.redirectRepo.Create(ctx, redirect)
}
```

Also handle collection slug changes — when a collection's slug is updated, create redirects for all articles in that collection pointing from old collection slug path to new.

- [ ] **Step 2: Build and verify**

- [ ] **Step 3: Commit**

```
feat: auto-create redirects on article/collection slug change
```

---

## Task 5: Import Creates Legacy Redirects

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/helpscout/types.go`

- [ ] **Step 1: Ensure HelpScout types include Number field**

Verify `ArticleRef` and `Category` structs in `server/internal/helpscout/types.go` have `Number` field (int) for building legacy URLs like `/article/267-team-members`.

- [ ] **Step 2: Update import service to create redirects**

In `server/internal/service/docs_import.go`, update `importArticle()`:

After creating the document + helpcenter article, create a legacy redirect:
```go
redirect := &model.DocsRedirect{
    WorkspaceID:          workspaceID,
    SourcePath:           fmt.Sprintf("/article/%d-%s", article.Number, article.Slug),
    TargetCollectionSlug: collectionSlug,
    TargetArticleSlug:    &articleSlug,
    Type:                 model.RedirectTypeImported,
    SourceSystem:         stringPtr("helpscout"),
    SourceObjectType:     stringPtr("article"),
    SourceObjectID:       stringPtr(article.ID),
}
```

Also after creating each collection, create a category redirect:
```go
redirect := &model.DocsRedirect{
    WorkspaceID:          workspaceID,
    SourcePath:           fmt.Sprintf("/category/%d-%s", category.Number, category.Slug),
    TargetCollectionSlug: collectionSlug,
    Type:                 model.RedirectTypeImported,
    SourceSystem:         stringPtr("helpscout"),
    SourceObjectType:     stringPtr("category"),
    SourceObjectID:       stringPtr(category.ID),
}
```

Wire `DocsRedirectRepository` into `DocsImportService` (update constructor + struct).

- [ ] **Step 3: Build and verify**

- [ ] **Step 4: Commit**

```
feat: create legacy redirects during HelpScout import
```

---

## Task 6: Admin Redirect CRUD — Backend

**Files:**
- Modify: `server/internal/service/docs_helpcenter.go`
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/router/router.go`

- [ ] **Step 1: Add service methods for redirect management**

In `docs_helpcenter.go`:
```go
func (s *DocsHelpcenterService) ListRedirects(ctx, workspaceID string, filter model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error)
func (s *DocsHelpcenterService) CreateRedirect(ctx, workspaceID string, req model.CreateDocsRedirectRequest) (*model.DocsRedirect, error)
func (s *DocsHelpcenterService) DeleteRedirect(ctx, id string) error
```

`CreateRedirect` validates source_path format (must start with `/`), sets type to `manual`.

- [ ] **Step 2: Add handler methods**

In `docs.go`:
```go
func (h *DocsHandler) ListRedirects(w http.ResponseWriter, r *http.Request)
func (h *DocsHandler) CreateRedirect(w http.ResponseWriter, r *http.Request)
func (h *DocsHandler) DeleteRedirect(w http.ResponseWriter, r *http.Request)
```

`ListRedirects`: parse query params (search, type, page, per_page), call service, return JSON with items + total.
`CreateRedirect`: decode request body, call service, return 201.
`DeleteRedirect`: extract `{id}` URL param, call service, return 200.

- [ ] **Step 3: Register routes**

In router.go, under the authenticated docs group:
```go
// Redirect management (docs.admin)
r.With(requirePerm(authorization.PermDocsAdmin)).Get("/redirects", h.Docs.ListRedirects)
r.With(requirePerm(authorization.PermDocsAdmin)).Post("/redirects", h.Docs.CreateRedirect)
r.With(requirePerm(authorization.PermDocsAdmin)).Delete("/redirects/{id}", h.Docs.DeleteRedirect)
```

- [ ] **Step 4: Build and verify**

- [ ] **Step 5: Commit**

```
feat: add redirect management endpoints for admin
```

---

## Task 7: Frontend — Redirects API Service

**Files:**
- Create: `frontend/src/lib/services/docsRedirectService.ts`

- [ ] **Step 1: Create the API service**

```typescript
import { api } from '../api';

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`;

export interface DocsRedirect {
  id: string;
  workspace_id: string;
  source_path: string;
  target_collection_slug: string;
  target_article_slug?: string;
  type: 'imported' | 'slug_change' | 'manual';
  source_system?: string;
  source_object_type?: string;
  source_object_id?: string;
  created_at: string;
}

export interface RedirectListResponse {
  items: DocsRedirect[];
  total: number;
}

export const docsRedirectService = {
  list: (wsId: string, params?: { search?: string; type?: string; page?: number; per_page?: number }) => {
    const searchParams = new URLSearchParams({ workspace_id: wsId });
    if (params?.search) searchParams.set('search', params.search);
    if (params?.type) searchParams.set('type', params.type);
    if (params?.page) searchParams.set('page', String(params.page));
    if (params?.per_page) searchParams.set('per_page', String(params.per_page));
    return api.get<RedirectListResponse>(`/docs/redirects?${searchParams}`);
  },
  create: (wsId: string, data: { source_path: string; target_collection_slug: string; target_article_slug?: string }) =>
    api.post<DocsRedirect>(`/docs/redirects${qs(wsId)}`, data),
  delete: (wsId: string, id: string) =>
    api.del(`/docs/redirects/${id}${qs(wsId)}`),
};
```

- [ ] **Step 2: Commit**

```
feat: add docs redirect frontend API service
```

---

## Task 8: Frontend — Redirects Management Page

**Files:**
- Create: `frontend/src/components/settings/RedirectsTab.tsx`
- Modify: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: Create RedirectsTab component**

Props: `{ workspaceId: string; editable: boolean }`

Features:
- Table: Source Path | Target | Type (badge) | Source | Created | Actions (delete)
- Type badges: `Imported` (blue), `Slug Change` (amber), `Manual` (gray)
- Search input filtering by source path
- Type filter dropdown (All, Imported, Slug Change, Manual)
- Pagination (if many redirects)
- "Add Redirect" button → dialog with:
  - Source path input (must start with `/`)
  - Target collection slug input
  - Target article slug input (optional — empty for collection redirect)
  - Save button
- Delete button per row with confirmation
- Empty state: "No redirects yet"

Use existing UI patterns from the codebase:
- Table from shadcn (`Table, TableBody, TableCell, TableHead, TableHeader, TableRow`)
- `Badge` for type labels
- `Dialog` for add form
- `Input` for search
- `Select` for type filter
- `Button` for actions
- `toast` from sonner for feedback

- [ ] **Step 2: Add Redirects tab to Settings**

In `frontend/src/pages/Settings.tsx`:
- Import `RedirectsTab`
- Add to the settings sections array in the "Support & Docs" group:
  ```
  { id: 'redirects', label: 'Redirects', description: 'Manage URL redirects for the public help center.', icon: ..., group: 'Support & Docs' }
  ```
- Add case in the render switch:
  ```
  case 'redirects': return <RedirectsTab workspaceId={workspaceId} editable={canManageSettings} />;
  ```

- [ ] **Step 3: Verify no TS errors**

Run: `npx tsc --noEmit`

- [ ] **Step 4: Commit**

```
feat: add Redirects management page to help center settings
```
