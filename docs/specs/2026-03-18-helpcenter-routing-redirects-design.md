# Help Center Routing & Redirects

> Source review, 2026-09-17

Historical March routing design. The public app now lives in this repository at
[help-center](../../help-center/package.json) and uses TanStack Start SSR.
Canonical collection/article keys now incorporate PublicIDs; consult
[locale route helpers](../../help-center/src/lib/locale.ts),
[collection keys](../../help-center/src/lib/collectionKey.ts), and
[article keys](../../help-center/src/lib/articleKey.ts). The slug-only URL model
below is not the current canonical contract.

## Overview

Establish a canonical public URL model for Helpin's help center, support 301 redirects for imported legacy URLs and native slug changes, and add a Redirects management page for admins.

## Canonical Public URL Model

**Browser URLs** (clean, no internal prefixes):
```
/:collectionSlug                → Collection landing page
/:collectionSlug/:articleSlug   → Article page
```

Examples:
- `/account-management` → lists articles in the Account Management collection
- `/account-management/team-members` → shows the Team Members article

**Rules:**
- `collectionSlug` globally unique within a workspace/help center
- `articleSlug` unique within its collection
- Spaces are internal architecture only — never in public URLs
- No `/hc/` prefix in browser URLs
- Collections are real public landing pages, not just sidebar groups

**Backend API routes** (called by the frontend app, not exposed in browser):
```
GET /api/hc/{subdomain}/config
GET /api/hc/{subdomain}/navigation
GET /api/hc/{subdomain}/c/{collectionSlug}
GET /api/hc/{subdomain}/c/{collectionSlug}/{articleSlug}
GET /api/hc/{subdomain}/search
GET /api/hc/{subdomain}/resolve/{path...}
```

---

## Redirect Architecture

### Three Redirect Types

| Type | Trigger | Example |
|------|---------|---------|
| `imported` | HelpScout/Zendesk import | `/article/267-team-members` → `/account-management/team-members` |
| `slug_change` | User renames collection/article slug | `/old-collection/old-article` → `/new-collection/new-article` |
| `manual` | Admin creates custom redirect | `/legacy-page` → `/new-page` |

All produce HTTP 301 permanent redirects.

### Unified `docs_redirects` Table

One table for all redirect types. Simpler to query, search, manage.

```sql
CREATE TABLE docs_redirects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  source_path TEXT NOT NULL,
  target_collection_slug TEXT NOT NULL,
  target_article_slug TEXT,
  type TEXT NOT NULL DEFAULT 'manual',
  source_system TEXT,
  source_object_type TEXT,
  source_object_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(workspace_id, source_path)
);
CREATE INDEX idx_docs_redirects_ws_path ON docs_redirects (workspace_id, source_path);
```

**Fields:**
- `source_path`: The path to match (e.g., `/article/267-team-members` or `/old-slug/old-article`)
- `target_collection_slug`: Canonical collection slug to redirect to
- `target_article_slug`: Canonical article slug (null for collection redirects)
- `type`: `imported`, `slug_change`, `manual`
- `source_system`: `helpscout`, `zendesk`, etc. (null for non-imports)
- `source_object_type`: `article`, `category` (null for non-imports)
- `source_object_id`: Original ID from source system (null for non-imports)

**Replaces** `docs_slug_aliases` table. Existing data migrated with `type = 'slug_change'`.

### Resolution Flow

On every public help center request:
1. Parse path as `/{segment1}` or `/{segment1}/{segment2}`
2. Try direct match: collection by slug → article by slug within collection
3. If no match: lookup `docs_redirects` where `source_path = request_path`
4. If redirect found: return 301 to `/{target_collection_slug}` or `/{target_collection_slug}/{target_article_slug}`
5. If nothing: 404

---

## Data Model Changes

### Schema Migration

```sql
-- 1. Create docs_redirects table
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

-- 2. Migrate existing slug aliases
INSERT INTO docs_redirects (workspace_id, source_path, target_collection_slug, target_article_slug, type)
  SELECT sa.workspace_id, sa.old_slug, '', ha.slug, 'slug_change'
  FROM docs_slug_aliases sa
  JOIN docs_helpcenter_articles ha ON ha.document_id = sa.document_id
  ON CONFLICT DO NOTHING;

-- 3. Change collection slug uniqueness to workspace-level
DROP INDEX IF EXISTS idx_docs_collection_space_slug;
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collection_ws_slug
  ON docs_collections (workspace_id, slug) WHERE slug != '' AND deleted_at IS NULL;

-- 4. Add collection_id to helpcenter article for slug uniqueness within collection
-- (collection_id already exists on docs_documents, helpcenter article references it via document)
-- Enforce article slug uniqueness within collection via application logic (not DB constraint,
-- since collection_id lives on docs_documents not docs_helpcenter_articles)
```

### New Model: `DocsRedirect`

```go
type DocsRedirect struct {
    ID                   string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID          string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
    SourcePath           string    `json:"source_path" gorm:"not null;uniqueIndex:idx_docs_redirects_ws_source,priority:2"`
    TargetCollectionSlug string    `json:"target_collection_slug" gorm:"not null"`
    TargetArticleSlug    *string   `json:"target_article_slug"`
    Type                 string    `json:"type" gorm:"not null;default:'manual'"`
    SourceSystem         *string   `json:"source_system"`
    SourceObjectType     *string   `json:"source_object_type"`
    SourceObjectID       *string   `json:"source_object_id"`
    CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
}
```

Constants: `RedirectTypeImported`, `RedirectTypeSlugChange`, `RedirectTypeManual`

### Updated Models

**DocsCollection**: Slug uniqueness index changed from `(space_id, slug)` to `(workspace_id, slug)`.

**DocsHelpcenterArticle**: Article slug uniqueness enforced per collection at application layer (service validates before save).

---

## Backend Changes

### New Repository: `docs_redirect.go`

```go
type DocsRedirectRepository struct { db *gorm.DB }

func (r *DocsRedirectRepository) Create(ctx, redirect *DocsRedirect) error
func (r *DocsRedirectRepository) BulkCreate(ctx, redirects []DocsRedirect) error
func (r *DocsRedirectRepository) GetBySourcePath(ctx, workspaceID, sourcePath string) (*DocsRedirect, error)
func (r *DocsRedirectRepository) List(ctx, workspaceID string, filter RedirectFilter) ([]DocsRedirect, int64, error)
func (r *DocsRedirectRepository) Delete(ctx, id string) error
```

`RedirectFilter`: search (source_path contains), type filter, pagination.

### Service Changes

**DocsHelpcenterService:**
- `ResolvePublicPath(ctx, workspaceID, path string)` → returns article data OR redirect target OR 404
- Update `PublishExternally()` / slug change handlers → auto-create redirect with `type = 'slug_change'` when slug changes
- Validate article slug uniqueness within collection before save
- New CRUD methods for manual redirects

**DocsImportService:**
- During import, create `docs_redirects` entries:
  - Per article: `source_path: "/article/{number}-{slug}"`, targets canonical slugs, `type: "imported"`, `source_system: "helpscout"`
  - Per category: `source_path: "/category/{number}-{slug}"`, targets collection slug, `type: "imported"`

### Route Changes

**New public routes** (under `/api/hc/{subdomain}/`):
```
GET /c/{collectionSlug}                → Collection page data (articles list)
GET /c/{collectionSlug}/{articleSlug}   → Article data
GET /resolve/{path...}                 → Redirect resolver (returns 301 target or 404)
```

**New admin routes** (under `/api/docs/`):
```
GET    /redirects                → List redirects (search, filter, paginate)
POST   /redirects                → Create manual redirect
DELETE /redirects/{id}           → Delete redirect
```

Admin routes require `docs.admin` permission.

---

## Frontend Changes

### Public Help Center App

Update routes from current space-based pattern to canonical:
- `/:collectionSlug` → Collection page component (lists articles)
- `/:collectionSlug/:articleSlug` → Article page component

The frontend app calls backend API at `/api/hc/{subdomain}/c/...`. The browser URL stays clean.

On 301 response from `/resolve/` endpoint, the frontend redirects the browser.

### Settings — Redirects Management Page

**Location**: Help Center settings → new "Redirects" tab/section

**UI:**
- Table with columns: Source Path, Target, Type (badge), Source System, Created
- Type badges: `Imported` (blue), `Slug Change` (amber), `Manual` (gray)
- Search input: filter by source path
- Type filter: dropdown to filter by type
- "Add Redirect" button → inline form or dialog:
  - Source path input (e.g., `/old-page`)
  - Target: collection selector + optional article selector
  - Save button
- Delete button per row (with confirmation)

**Frontend service:**
```typescript
export const docsRedirectService = {
  list: (wsId, params?) => api.get('/docs/redirects', { params }),
  create: (wsId, data) => api.post('/docs/redirects', data),
  delete: (wsId, id) => api.del(`/docs/redirects/${id}`),
};
```

---

## Import Integration

During HelpScout import, for each imported article:
```go
redirect := DocsRedirect{
    WorkspaceID:          workspaceID,
    SourcePath:           fmt.Sprintf("/article/%s-%s", article.Number, article.Slug),
    TargetCollectionSlug: collectionSlug,
    TargetArticleSlug:    &articleSlug,
    Type:                 RedirectTypeImported,
    SourceSystem:         stringPtr("helpscout"),
    SourceObjectType:     stringPtr("article"),
    SourceObjectID:       stringPtr(article.ID),
}
```

For each imported category:
```go
redirect := DocsRedirect{
    WorkspaceID:          workspaceID,
    SourcePath:           fmt.Sprintf("/category/%s-%s", category.Number, category.Slug),
    TargetCollectionSlug: collectionSlug,
    Type:                 RedirectTypeImported,
    SourceSystem:         stringPtr("helpscout"),
    SourceObjectType:     stringPtr("category"),
    SourceObjectID:       stringPtr(category.ID),
}
```

---

## Implementation Phases

**Phase 1: Schema + Model + Repository**
- Create `docs_redirects` table (migration)
- Migrate `docs_slug_aliases` data
- Update collection slug uniqueness index
- Create `DocsRedirect` model + repository
- Register in AutoMigrate

**Phase 2: Backend canonical routes + resolution**
- New public routes: `/c/{collectionSlug}`, `/c/{collectionSlug}/{articleSlug}`
- Redirect resolver endpoint: `/resolve/{path...}`
- Auto-create redirects on slug change
- Article slug uniqueness validation per collection

**Phase 3: Import creates redirects**
- Update `DocsImportService.importArticle()` to create redirect entries
- Store HelpScout source metadata (number, ID, type)

**Phase 4: Admin Redirects CRUD**
- Backend: List/Create/Delete endpoints for manual redirects
- Frontend: Redirects management page in help center settings

**Phase 5: Public help center frontend routing**
- Update routes to `/:collectionSlug/:articleSlug`
- Remove space from public URLs
- Handle redirect responses
