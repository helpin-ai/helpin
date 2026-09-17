# HelpScout Help Center Import

## Overview

Import help center articles from HelpScout into Helpin's Docs module. One-time import triggered from Settings → Import / Export, with progress tracking and retry for failures.

## Goals

- Import HelpScout collections (categories) and articles into Helpin spaces/collections/documents
- Preserve article slugs for SEO continuity
- Re-upload images to S3 (no dependency on HelpScout CDN after import)
- Let users choose import status: draft, published, or match source
- Show real-time progress with ability to retry failures

## Non-Goals

- Ongoing sync with HelpScout (one-time import only)
- Importing HelpScout Beacon/chat data
- Automatic URL redirect setup (we provide a redirect map, user configures DNS/CDN)

---

## HelpScout Docs API

- **Auth**: HTTP Basic (API key as username, empty password)
- **Base URL**: `https://docsapi.helpscout.net/v1/`
- **Rate limit**: 2000 requests per 10 minutes. Read `X-RateLimit-Remaining` header; on HTTP 429, back off and retry after `Retry-After` seconds.
- **Key endpoints**:
  - `GET /v1/collections` — list all collections (sites)
  - `GET /v1/collections/{id}/categories` — list categories within a collection
  - `GET /v1/collections/{id}/articles` — list article metadata (no body)
  - `GET /v1/articles/{id}` — get full article with HTML body
- **Content format**: HTML only (`text` field). May include inline CSS, custom `<div>` classes for callouts, `<iframe>` embeds, and images hosted on HelpScout CDN.
- **Article fields**: `id`, `number`, `slug`, `name` (title), `text` (HTML body), `status` (published/notpublished), `categories[]`, `hasDraft`, `createdAt`, `updatedAt`
- **Pagination**: `page` and `pages` in response for paginated endpoints
- **Draft access**: Append `?draft=true` to get draft version of an article

## Data Mapping

| HelpScout | Helpin | Notes |
|-----------|--------|-------|
| Collection | DocsSpace | One HelpScout collection → one Helpin space |
| Category | DocsCollection | Category name + slug preserved |
| Article | DocsDocument + DocsContent + DocsHelpcenterArticle | Title, content, slug, SEO metadata |
| Article slug | DocsHelpcenterArticle.slug | Stored as-is from source (e.g., `480-remove-a-workspace`) |
| Category slug | DocsCollection.slug (new field) | Requires migration to add slug column |
| Article status | DocsDocument.status | Mapped per user choice (draft/published/match source) |
| Article HTML | DocsContent.content | Converted to Markdown, saved via existing `SaveDocsMarkdownRequest` path |
| Images | S3/MinIO | Downloaded from HelpScout CDN, re-uploaded, URLs replaced |

---

## Schema Changes

### Add `slug` field to `DocsCollection`

Migration (`server/migrations/033_docs_collection_slug.sql`):

```sql
ALTER TABLE docs_collections ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collection_space_slug
  ON docs_collections (space_id, slug) WHERE slug != '';
```

GORM model update (`DocsCollection`): add `Slug string` field.
DTO updates: add `Slug` to `CreateDocsCollectionRequest` and collection responses.
Service update: `DocsCollectionService.Create()` accepts optional slug.

### Import job persistence table

Migration (`server/migrations/034_docs_import_jobs.sql`):

```sql
CREATE TABLE IF NOT EXISTS docs_import_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id),
  space_id UUID REFERENCES docs_spaces(id),
  source TEXT NOT NULL DEFAULT 'helpscout',
  status TEXT NOT NULL DEFAULT 'pending',
  total INT NOT NULL DEFAULT 0,
  completed INT NOT NULL DEFAULT 0,
  failed INT NOT NULL DEFAULT 0,
  failures JSONB NOT NULL DEFAULT '[]',
  config JSONB NOT NULL DEFAULT '{}',
  redirect_map JSONB,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_docs_import_jobs_ws ON docs_import_jobs (workspace_id);
```

### Add `docs.import` permission

Add `PermDocsImport` constant to `authorization/permissions.go`. Grant to `admin` and `owner` roles in the RBAC matrix.

---

## API Key Handling

The HelpScout API key is **ephemeral** — used only during the active import session and never persisted to the database. It is:
- Passed in request bodies to preview and start endpoints
- Held in memory only for the duration of the import job
- Never logged (handlers and service must sanitize)
- Cleared from the `ImportJob` struct after the job completes

If future features need stored API keys (e.g., ongoing sync), use `internal/crypto/` AES-256-GCM encryption as done for CRM OAuth tokens.

---

## Backend Architecture

### New Files

| File | Purpose |
|------|---------|
| `server/internal/helpscout/client.go` | HelpScout Docs API HTTP client |
| `server/internal/helpscout/convert.go` | HTML → Markdown converter with edge case handling |
| `server/internal/model/docs_import.go` | DocsImportJob GORM model + DTOs |
| `server/internal/repository/docs_import.go` | Import job CRUD |
| `server/internal/service/docs_import.go` | Import orchestration, background job lifecycle |
| `server/internal/handler/docs_import.go` | HTTP handlers for preview, start, status, retry, redirect map |

### HelpScout Client (`helpscout/client.go`)

```go
type Client struct {
    apiKey     string
    httpClient *http.Client
    logger     *slog.Logger
}

func NewClient(apiKey string) *Client
func (c *Client) ListCollections(ctx context.Context) ([]Collection, error)
func (c *Client) ListCategories(ctx context.Context, collectionID string) ([]Category, error)
func (c *Client) ListArticles(ctx context.Context, collectionID string) ([]ArticleRef, error)
func (c *Client) GetArticle(ctx context.Context, articleID string) (*Article, error)
```

- HTTP Basic auth (apiKey as username, empty password)
- Reads `X-RateLimit-Remaining` header; sleeps when approaching limit
- Handles HTTP 429 with exponential backoff using `Retry-After` header
- Handles pagination automatically (follows `page`/`pages`)
- Returns typed Go structs
- Shared `*http.Client` (reused, not per-request)

### HTML Converter (`helpscout/convert.go`)

Converts HelpScout HTML to Markdown using a Go library (`html-to-markdown`).

**Edge case handling:**
- **Tables**: Converted to Markdown pipe tables. Complex tables (colspan/rowspan) wrapped in code blocks.
- **Callouts**: `<div class="callout callout-info">` → `> **Note:** ...` blockquotes
- **Iframes/videos**: Extract src URL, convert to plain link
- **Images with captions**: `<figure>` → image + italic caption text
- **Inline CSS**: Stripped entirely
- **Internal links**: HelpScout article URLs detected and stored in a link map for redirect generation

**Image processing** (separate step, runs before conversion):
1. Parse HTML, find all `<img src="...">` tags
2. Download image from HelpScout CDN
3. Upload to S3 via existing `storage.S3Client`
4. Replace `src` URL in HTML with new S3 public URL
5. Then convert modified HTML to Markdown

### Import Job Model (`model/docs_import.go`)

```go
type DocsImportJob struct {
    ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
    SpaceID     *string         `json:"space_id" gorm:"type:uuid"`
    Source      string          `json:"source" gorm:"not null;default:'helpscout'"`
    Status      string          `json:"status" gorm:"not null;default:'pending'"`
    Total       int             `json:"total" gorm:"not null;default:0"`
    Completed   int             `json:"completed" gorm:"not null;default:0"`
    Failed      int             `json:"failed" gorm:"not null;default:0"`
    Failures    json.RawMessage `json:"failures" gorm:"type:jsonb;not null;default:'[]'"`
    Config      json.RawMessage `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
    RedirectMap json.RawMessage `json:"redirect_map,omitempty" gorm:"type:jsonb"`
    StartedAt   *time.Time      `json:"started_at"`
    CompletedAt *time.Time      `json:"completed_at"`
    CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsImportJob) TableName() string { return "docs_import_jobs" }
```

### Import Service (`service/docs_import.go`)

```go
type DocsImportService struct {
    importRepo    *repository.DocsImportRepository
    spaceSvc      *DocsSpaceService
    collectionSvc *DocsCollectionService
    documentSvc   *DocsDocumentService
    contentSvc    *DocsContentService
    helpcenterSvc *DocsHelpcenterService
    s3Client      *storage.S3Client
    logger        *slog.Logger
}
```

**Methods:**
- `Preview(ctx, apiKey)` → validates key, fetches collections/categories/article counts
- `Start(ctx, req)` → creates job in DB, launches managed goroutine via errgroup, returns job ID
- `GetStatus(ctx, jobID)` → reads job from DB
- `Retry(ctx, jobID)` → re-processes failed articles
- `GetRedirectMap(ctx, jobID)` → returns redirect map CSV

**Background job management:**
- Uses `errgroup.Group` wired into the server's graceful shutdown (not fire-and-forget)
- Job progress updated in DB periodically (batched, not per-article to reduce writes)
- On server shutdown, running jobs are marked as `interrupted` and can be retried

**Import sequence (managed goroutine):**
1. Fetch all categories for the HelpScout collection
2. Create DocsCollection for each category (with slug preserved)
3. Fetch article list, then fetch each article body (respecting rate limits)
4. For each article:
   a. Download and re-upload images to S3
   b. Replace image URLs in HTML
   c. Convert HTML → Markdown
   d. Create DocsDocument (title, space_id, collection_id)
   e. Save content via existing `SaveDocsMarkdownRequest` path (wraps in `_markdown_source` envelope)
   f. Create DocsHelpcenterArticle with original slug and SEO metadata
   g. Set status per user choice (draft/published/match source)
   h. Update job progress in DB
5. Build redirect map: `old HelpScout URL → new Helpin slug` for each article
6. Store redirect map in job record, mark job as `done`

### API Endpoints

All nested under the workspace route group (`/api/workspaces/{id}/...`):

```
POST /api/docs/import/helpscout/preview
  Body: { api_key: string }
  Response: { collections: [{ id, name, slug, category_count, article_count }] }

POST /api/docs/import/helpscout/start
  Body: {
    api_key: string,
    helpscout_collection_id: string,
    target_space_id?: string,
    new_space_name?: string,
    import_status: "draft" | "published" | "match_source"
  }
  Response: { job_id: string }

GET  /api/docs/import/{jobId}/status
  Response: {
    id: string,
    status: "pending" | "running" | "done" | "failed" | "interrupted",
    total: 47,
    completed: 12,
    failed: 0,
    failures: [{ article_id, title, error }],
    redirect_map_available: boolean
  }

POST /api/docs/import/{jobId}/retry
  Response: { status: "running" }

GET  /api/docs/import/{jobId}/redirect-map
  Response: CSV download (old_url, new_slug, new_url)
```

All endpoints require `docs.import` permission (admin/owner only).

---

## Frontend UI

### Location

Settings → Import / Export tab → new "Help Center" section below existing PM import. The `ImportTab` component should be refactored into categorized sections with headers: "Project Management" and "Help Center".

### UI Flow

**Step 1 — Connect**
- Section header: "Help Center"
- Source dropdown: "HelpScout" (extensible for future sources)
- API key input + "Connect" button
- Loading state while validating
- On success: shows preview card — "Found 8 categories, 47 articles in [Collection Name]"
- If multiple HelpScout collections: dropdown to pick which one

**Step 2 — Configure**
- Target space: select existing or "Create new space" with name input
- Import status: radio group — "Import as draft" / "Import as published" / "Match source status"
- "Start Import" button

**Step 3 — Progress**
- Progress bar: "Importing... 12 of 47 articles"
- Frontend polls `GET /status` every 2 seconds
- On completion: "Import complete. 45 succeeded, 2 failed."
- Failed items listed with article title + error
- "Retry failed" button
- "Download Redirect Map" link (CSV)
- "Go to Space" link

### Component

New `HelpCenterImportSection` component added within existing `ImportTab`. No new route or settings tab needed.

---

## Slug & SEO Handling

- Article slugs stored as-is in `DocsHelpcenterArticle.slug` — no format assumptions
- Collection slugs stored in new `DocsCollection.slug` field
- Source-agnostic: works for any import source (HelpScout, Zendesk, Intercom, etc.)
- Redirect map generated on import completion and stored in the job record
- Available as CSV download: `old_url, new_slug, new_url`
- Users configure redirects at their DNS/CDN layer (Cloudflare, Vercel, etc.)

---

## Error Handling

- **Invalid API key**: Preview returns 401 → toast "Invalid API key"
- **Rate limit hit**: Client reads `X-RateLimit-Remaining`; on 429, backs off using `Retry-After` header
- **Image download failure**: Log warning, keep original URL as fallback, continue import
- **HTML conversion failure**: Save raw HTML wrapped in markdown code block, log warning
- **Article fetch failure**: Mark as failed in job, continue with next article. Retry up to 3 times before marking failed.
- **Network errors**: Exponential backoff, 3 retries per request
- **Server restart during import**: Job marked as `interrupted` in DB, user can retry

---

## Draft Article Handling

HelpScout articles can have both a published and draft version (`hasDraft` boolean):
- **"Import as published"**: Import only the published version. Skip unpublished articles.
- **"Import as draft"**: Import all articles as draft, including published ones.
- **"Match source status"**: Published articles → published, unpublished → draft. If `hasDraft` is true, import the draft version using `?draft=true` param.

---

## Future Extensibility

- **New sources**: Add `zendesk/client.go`, `intercom/client.go` following same pattern. The import job model is source-agnostic (`source` field).
- **Export**: Add export-to-markdown or export-to-HTML from Helpin docs
- **Ongoing sync**: Could add Temporal workflow if periodic re-import is needed later
